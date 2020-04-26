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
	"time"

	"github.com/rs/xid"
	log "github.com/sirupsen/logrus"

	heroku "github.com/heroku/heroku-go/v5"
)

var (
	containerStack = "container"
)

func NewDeployer(accessToken string) *Deployer {
	heroku.DefaultTransport.BearerToken = accessToken
	svc := heroku.NewService(heroku.DefaultClient)

	return &Deployer{
		Heroku:      svc,
		Logger:      log.New(),
		accessToken: accessToken,
	}
}

type Deployer struct {
	Heroku      *heroku.Service
	Logger      log.FieldLogger
	accessToken string
}

func (d *Deployer) Deploy(ctx context.Context, appName string, buildOutput io.Writer) (string, error) {
	logger := d.Logger

	logger.Infof("Getting account")
	acct, err := d.account(ctx)
	if err != nil {
		return "", err
	}

	logger.Infof("Getting app")
	app, err := d.app(ctx, appName)
	if err != nil {
		return "", err
	}

	logger.Infof("Creating cf app")
	cfApp, err := d.createCFApp(ctx, d.accessToken, acct, app)
	if err != nil {
		return "", err
	}

	logger = logger.WithField("cf-app", cfApp.Name)

	logger.Infof("Uploading source")
	src, err := d.uploadSource(ctx, "./template")
	if err != nil {
		return "", err
	}

	logger.Infof("Creating build")
	build, err := d.createBuild(ctx, cfApp, src, buildOutput)
	if err != nil {
		return "", err
	}

	if err := d.waitForRelease(ctx, cfApp, build, logger); err != nil {
		return "", err
	}

	return d.cfAppURL(ctx, cfApp)
}

func (d *Deployer) account(ctx context.Context) (*heroku.Account, error) {
	acct, err := d.Heroku.AccountInfo(ctx)
	if err != nil {
		return nil, err
	}

	if acct.Email == "" {
		return nil, fmt.Errorf("error getting account email")
	}

	return acct, nil
}

func (d *Deployer) app(ctx context.Context, appName string) (*heroku.App, error) {
	app, err := d.Heroku.AppInfo(ctx, appName)
	if err != nil {
		return nil, err
	}
	if app.Name != appName {
		return nil, fmt.Errorf("error getting app %s", appName)
	}

	return app, nil
}

func (d *Deployer) createCFApp(ctx context.Context, accessToken string, acct *heroku.Account, app *heroku.App) (*heroku.App, error) {
	cfAppName := app.Name + "-cf-" + xid.New().String()
	cfApp, err := d.Heroku.AppCreate(ctx, heroku.AppCreateOpts{
		Name:   &cfAppName,
		Region: &app.Region.ID,
		Stack:  &containerStack,
	})
	if err != nil {
		return nil, err
	}

	if _, err := d.Heroku.ConfigVarUpdate(ctx, cfApp.Name, map[string]*string{
		"HEROKU_USER":      &acct.Email,
		"HEROKU_USER_NAME": &acct.Email,
		"HEROKU_API_KEY":   &accessToken,
		"HEROKU_APP":       &app.Name,
	}); err != nil {
		return nil, err
	}

	return cfApp, nil
}

func (d *Deployer) uploadSource(ctx context.Context, dir string) (*heroku.Source, error) {
	src, err := d.Heroku.SourceCreate(ctx)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(nil)
	if err := compress("./template", buf); err != nil {
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

func (d *Deployer) createBuild(ctx context.Context, cfApp *heroku.App, src *heroku.Source, buildOutput io.Writer) (*heroku.Build, error) {
	build, err := d.Heroku.BuildCreate(ctx, cfApp.Name, heroku.BuildCreateOpts{
		SourceBlob: struct {
			Checksum *string `json:"checksum,omitempty" url:"checksum,omitempty,key"`
			URL      *string `json:"url,omitempty" url:"url,omitempty,key"`
			Version  *string `json:"version,omitempty" url:"version,omitempty,key"`
		}{
			URL: &src.SourceBlob.GetURL,
			// TODO: add checksum and version
		},
	})
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(build.OutputStreamURL)
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(buildOutput, resp.Body)
	if err != nil {
		return nil, err
	}

	return build, nil
}

func (d *Deployer) waitForRelease(ctx context.Context, cfApp *heroku.App, build *heroku.Build, logger log.FieldLogger) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	logger = logger.WithField("build", build.ID)
	for {
		logger.Info("Waiting for build")

		select {
		case <-ticker.C:
			b, err := d.Heroku.BuildInfo(ctx, cfApp.Name, build.ID)
			if err == nil && b.Release != nil {
				return nil
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (d *Deployer) cfAppURL(ctx context.Context, cfApp *heroku.App) (string, error) {
	domains, err := d.Heroku.DomainList(ctx, cfApp.Name, nil)
	if err != nil {
		return "", err
	}

	if len(domains) == 0 {
		return "", fmt.Errorf("no domain is found for app %s", cfApp.Name)
	}

	u, err := url.Parse("https://" + domains[0].Hostname)
	if err != nil {
		return "", err
	}

	val := u.Query()
	val.Set("folder", "/home/coder/project") // default to the project folder
	u.RawQuery = val.Encode()

	return u.String(), nil
}

func compress(src string, buf io.Writer) error {
	// tar > gzip > buf
	zr := gzip.NewWriter(buf)
	tw := tar.NewWriter(zr)

	// walk through every file in the folder
	filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
		// generate tar header
		header, err := tar.FileInfoHeader(fi, file)
		if err != nil {
			return err
		}

		// must provide real name
		// (see https://golang.org/src/archive/tar/common.go?#L626)
		header.Name = filepath.ToSlash(file)

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
		}

		return nil
	})

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
