package editor

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"text/template"
	"time"

	heroku "github.com/heroku/heroku-go/v5"
	log "github.com/sirupsen/logrus"
)

var (
	containerStack = "container"
	version        = "0.0.2" // TODO load from env var
)

func NewDeployer(accessToken, templateDir string) *Deployer {
	client := &http.Client{
		Transport: &heroku.Transport{
			BearerToken: accessToken,
		},
	}

	return &Deployer{
		templateDir: templateDir,
		heroku:      heroku.NewService(client),
		logger:      log.New().WithField("com", "deployer"),
	}
}

type Deployer struct {
	templateDir string
	heroku      *heroku.Service
	logger      log.FieldLogger
}

func (d *Deployer) buildInfo(ctx context.Context, appName, buildID string) (*heroku.Build, error) {
	return d.heroku.BuildInfo(ctx, appName, buildID)
}

func (d *Deployer) DeployEditorAndScaleDown(ctx context.Context) (*heroku.App, error) {
	d.logger.Infof("Getting account")
	acct, err := Account(ctx, d.heroku)
	if err != nil {
		return nil, err
	}

	d.logger.Infof("Creating cf app")
	cfApp, err := d.createCFApp(ctx, acct)
	if err != nil {
		return nil, err
	}

	logger := d.logger.WithField("app", cfApp.Name)

	defer func() {
		if r := recover(); r != nil {
			if cfApp != nil {
				logger.Info("Panic deploying app, cleaning up")
				DeleteApp(d.heroku, cfApp, d.logger)
			}

			// re-panic
			panic(r)
		}
	}()

	// make sure failed app is cleaned up if there is any error
	defer func() {
		if err != nil && cfApp != nil {
			logger.Info("Error deploying app, cleaning up")
			DeleteApp(d.heroku, cfApp, d.logger)
		}
	}()

	err = d.buildAndScaleDown(ctx, cfApp, logger)
	if err != nil {
		return cfApp, err
	}

	logger.Infof("Marking app as idled")
	cfApp, err = d.markAppAsIdled(ctx, cfApp)

	return cfApp, err
}

func (d *Deployer) markAppAsIdled(ctx context.Context, app *heroku.App) (*heroku.App, error) {
	if buildingAppCurrentVersionRegexp.MatchString(app.Name) {
		cfID := buildingAppCurrentVersionRegexp.FindStringSubmatch(app.Name)
		newIdentity := buildIdleAppName(cfID[1])
		newApp, err := d.heroku.AppUpdate(ctx, app.Name, heroku.AppUpdateOpts{
			Name: &newIdentity,
		})

		if newApp == nil {
			newApp = app
		}

		return newApp, err
	}

	return app, nil
}

func (d *Deployer) buildAndScaleDown(ctx context.Context, cfApp *heroku.App, logger *log.Entry) error {
	logger.Infof("Uploading source")
	src, err := d.uploadSource(ctx, d.templateDir, map[string]string{})
	if err != nil {
		return err
	}

	logger.Infof("Creating build")
	build, err := d.createBuild(ctx, cfApp, src)
	if err != nil {
		return err
	}

	logger = logger.WithField("build", build.ID)

	logger.Infof("Building")
	if err := d.streamBuildLog(ctx, build, logger.Writer()); err != nil {
		return err
	}

	if err := d.waitForRelease(ctx, build, logger); err != nil {
		return err
	}

	logger.Infof("Scaling down app")
	return d.scaleDownApp(ctx, cfApp.Name)
}

func (d *Deployer) scaleDownApp(ctx context.Context, appIdentity string) error {
	qty := 0
	_, err := d.heroku.FormationUpdate(ctx, appIdentity, "web", heroku.FormationUpdateOpts{
		Quantity: &qty,
	})
	return err
}

func (d *Deployer) app(ctx context.Context, appName string) (*heroku.App, error) {
	app, err := d.heroku.AppInfo(ctx, appName)
	if err != nil {
		return nil, err
	}
	if app.Name != appName {
		return nil, fmt.Errorf("error: fail to get app %s", appName)
	}

	return app, nil
}

func (d *Deployer) createCFApp(ctx context.Context, acct *heroku.Account) (*heroku.App, error) {
	region := "us"
	name := genBuildingAppName()
	cfApp, err := d.heroku.AppCreate(ctx, heroku.AppCreateOpts{
		Name:   &name,
		Region: &region,
		Stack:  &containerStack,
	})
	if err != nil {
		return nil, err
	}
	return cfApp, nil
}

func (d *Deployer) uploadSource(ctx context.Context, dir string, tmplData map[string]string) (*heroku.Source, error) {
	src, err := d.heroku.SourceCreate(ctx)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(nil)
	if err := compress("./template", buf, tmplData); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPut, src.SourceBlob.PutURL, buf)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		return nil, fmt.Errorf("error: fail to upload source status=%d body=%s", resp.StatusCode, b)
	}

	return src, nil
}

func (d *Deployer) createBuild(ctx context.Context, cfApp *heroku.App, src *heroku.Source) (*heroku.Build, error) {
	return d.heroku.BuildCreate(ctx, cfApp.Name, heroku.BuildCreateOpts{
		SourceBlob: struct {
			Checksum *string `json:"checksum,omitempty" url:"checksum,omitempty,key"`
			URL      *string `json:"url,omitempty" url:"url,omitempty,key"`
			Version  *string `json:"version,omitempty" url:"version,omitempty,key"`
		}{
			URL:     &src.SourceBlob.GetURL,
			Version: &version,
			// TODO: add checksum and version
		},
	})
}

func (d *Deployer) streamBuildLog(ctx context.Context, build *heroku.Build, buildOutput io.Writer) error {
	errCh := make(chan error, 1)

	go func(url string) {
		resp, err := http.Get(url)
		if err != nil {
			errCh <- err
			return
		}

		_, err = io.Copy(buildOutput, resp.Body)
		if err != nil {
			errCh <- err
		}

		errCh <- nil
	}(build.OutputStreamURL)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *Deployer) waitForRelease(ctx context.Context, build *heroku.Build, logger log.FieldLogger) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var err error
	for {
		select {
		case <-ticker.C:
			build, err = d.buildInfo(ctx, build.App.ID, build.ID)
			if err == nil {
				logger.WithField("build-status", build.Status).Info("Waiting for release")

				if build.Status == "failed" {
					return fmt.Errorf("error: fail to build")
				}

				if build.Release != nil {
					return nil
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (d *Deployer) cfAppURL(ctx context.Context, appID string) (string, error) {
	domains, err := d.heroku.DomainList(ctx, appID, nil)
	if err != nil {
		return "", err
	}

	if len(domains) == 0 {
		return "", fmt.Errorf("error: no domain is found for app %s", appID)
	}

	u, err := url.Parse("https://" + domains[0].Hostname)
	if err != nil {
		return "", err
	}

	val := u.Query()
	val.Set("folder", "/home/dyno/project") // default to the project folder
	u.RawQuery = val.Encode()

	return u.String(), nil
}

func compress(src string, buf io.Writer, tmplData map[string]string) error {
	// tar > gzip > buf
	zr := gzip.NewWriter(buf)
	tw := tar.NewWriter(zr)

	// walk through every file in the folder
	err := filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
		path := filepath.ToSlash(file)

		if !fi.IsDir() {
			dir, err := ioutil.TempDir("", "tmp")
			if err != nil {
				return err
			}
			tmpf, err := os.OpenFile(filepath.Join(dir, fi.Name()), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				return err
			}
			defer os.Remove(dir)

			t := template.Must(template.New(filepath.Base(file)).ParseFiles(file))
			if err := t.Execute(tmpf, tmplData); err != nil {
				fmt.Println(err)
				return err
			}

			fi, err = tmpf.Stat()
			if err != nil {
				return err
			}

			if err := tmpf.Close(); err != nil {
				return err
			}

			file = tmpf.Name()
		}

		// generate tar header
		header, err := tar.FileInfoHeader(fi, file)
		if err != nil {
			return err
		}

		// tar.FileInfoHeader only keeps base name of a file
		header.Name = path

		// write header
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// if not a dir, write file content
		if !fi.IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}
			if _, err := io.Copy(tw, data); err != nil {
				return err
			}
			if err := data.Close(); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	// produce tar
	if err := tw.Close(); err != nil {
		return err
	}
	// produce gzip
	if err := zr.Close(); err != nil {
		return err
	}

	return nil
}
