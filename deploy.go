package codeface

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

	log "github.com/sirupsen/logrus"

	heroku "github.com/heroku/heroku-go/v5"
)

var (
	containerStack = "container"
)

func NewDeployer(accessToken string) *Deployer {
	client := &http.Client{
		Transport: &heroku.Transport{
			BearerToken: accessToken,
		},
	}

	return &Deployer{
		heroku:      heroku.NewService(client),
		logger:      log.New(),
		accessToken: accessToken,
	}
}

type Deployer struct {
	heroku      *heroku.Service
	logger      log.FieldLogger
	accessToken string
}

func (d *Deployer) BuildInfo(ctx context.Context, appName, buildID string) (*heroku.Build, error) {
	return d.heroku.BuildInfo(ctx, appName, buildID)
}

func (d *Deployer) DeployEditorApp(ctx context.Context, repo string) (*heroku.Build, error) {
	logger := d.logger.WithField("repo", repo)

	logger.Infof("Getting account")
	acct, err := d.account(ctx)
	if err != nil {
		return nil, err
	}

	logger.Infof("Creating cf app")
	cfApp, err := d.createCFApp(ctx, d.accessToken, acct, repo)
	if err != nil {
		return nil, err
	}

	logger = logger.WithField("cf-app", cfApp.Name)

	logger.Infof("Uploading source")
	src, err := d.uploadSource(ctx, "./template", map[string]string{})
	if err != nil {
		return nil, err
	}

	logger.Infof("Creating build")
	return d.createBuild(ctx, cfApp, src)
}

func (d *Deployer) DeployEditorAppAndWait(ctx context.Context, repo string, buildOutput io.Writer) (string, error) {
	build, err := d.DeployEditorApp(ctx, repo)
	if err != nil {
		return "", err
	}

	if err := d.streamBuildLog(ctx, build, buildOutput); err != nil {
		return "", err
	}

	logger := d.logger.WithFields(log.Fields{"app": build.App.ID, "build": build.ID})
	if err := d.waitForRelease(ctx, build, logger); err != nil {
		return "", err
	}

	return d.cfAppURL(ctx, build.App.ID)
}

func (d *Deployer) account(ctx context.Context) (*heroku.Account, error) {
	acct, err := d.heroku.AccountInfo(ctx)
	if err != nil {
		return nil, err
	}

	if acct.Email == "" {
		return nil, fmt.Errorf("error getting account email")
	}

	return acct, nil
}

func (d *Deployer) app(ctx context.Context, appName string) (*heroku.App, error) {
	app, err := d.heroku.AppInfo(ctx, appName)
	if err != nil {
		return nil, err
	}
	if app.Name != appName {
		return nil, fmt.Errorf("error getting app %s", appName)
	}

	return app, nil
}

func (d *Deployer) createCFApp(ctx context.Context, accessToken string, acct *heroku.Account, repo string) (*heroku.App, error) {
	region := "us"
	cfApp, err := d.heroku.AppCreate(ctx, heroku.AppCreateOpts{
		Region: &region,
		Stack:  &containerStack,
	})
	if err != nil {
		return nil, err
	}

	if _, err := d.heroku.ConfigVarUpdate(ctx, cfApp.Name, map[string]*string{
		"GIT_REPO": &repo,
	}); err != nil {
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

		return nil, fmt.Errorf("error uploading source: status=%d body=%s", resp.StatusCode, b)
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
			URL: &src.SourceBlob.GetURL,
			// TODO: add checksum and version
		},
	})
}

func (d *Deployer) streamBuildLog(ctx context.Context, build *heroku.Build, buildOutput io.Writer) error {
	resp, err := http.Get(build.OutputStreamURL)
	if err != nil {
		return err
	}

	_, err = io.Copy(buildOutput, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func (d *Deployer) waitForRelease(ctx context.Context, build *heroku.Build, logger log.FieldLogger) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	logger = logger.WithField("build", build.ID)
	for {
		logger.Info("Waiting for release")

		select {
		case <-ticker.C:
			b, err := d.BuildInfo(ctx, build.App.ID, build.ID)
			if err == nil {
				if b.Status == "failed" {
					return fmt.Errorf("build failed")
				}

				if b.Release != nil {
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
		return "", fmt.Errorf("no domain is found for app %s", appID)
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
