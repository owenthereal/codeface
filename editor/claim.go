package editor

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	heroku "github.com/heroku/heroku-go/v5"
	log "github.com/sirupsen/logrus"
)

func NewClaimer(accessToken string) *Claimer {
	client := &http.Client{
		Transport: &heroku.Transport{
			BearerToken: accessToken,
		},
	}

	return &Claimer{
		heroku:      heroku.NewService(client),
		logger:      log.New().WithField("com", "claimer"),
		accessToken: accessToken,
	}
}

type Claimer struct {
	heroku      *heroku.Service
	logger      log.FieldLogger
	accessToken string
}

func (t *Claimer) Claim(ctx context.Context, appIdentity, recipient, gitRepo string) (*heroku.App, error) {
	logger := t.logger.WithFields(log.Fields{"app": appIdentity, "recipient": recipient})

	var (
		app *heroku.App
		err error
	)

	if appIdentity == "" {
		logger.Info("Taking one app from the pool")
		app, err = t.findOneCFApp(ctx)
		if err != nil {
			return app, err
		}
	} else {
		logger.Info("Getting app")
		app, err = t.app(ctx, appIdentity)
		if err != nil {
			return app, err
		}
	}

	logger.WithField("app", app.Name).Infof("Updating idle app name")
	app, err = t.updateIdleAppName(ctx, app)
	if err != nil {
		return app, err
	}

	logger = logger.WithField("app", app.Name)

	logger.Infof("Adding Git repo")
	if err := t.addGitRepo(ctx, app.Name, gitRepo); err != nil {
		return app, err
	}

	logger.Infof("Scaling up app")
	if err := t.scaleUpApp(ctx, app.Name); err != nil {
		return app, err
	}

	// the app is already owned by the recipient
	if app.Owner.Email == recipient || app.Owner.ID == recipient {
		return app, nil
	}

	logger.Infof("Adding collaborator")
	if err := t.addCollaborator(ctx, app.Name, recipient); err != nil {
		if !strings.Contains(err.Error(), "User is already a collaborator on app") {
			return app, err
		}
	}

	logger.Infof("Transferring app")
	tr, err := t.transferApp(ctx, app.Name, recipient)
	if err != nil {
		return app, err
	}

	logger = logger.WithField("transfer", tr.ID)
	logger.Infof("Accepting transfer")
	if err := t.acceptTransfer(ctx, tr.ID); err != nil {
		return app, err
	}

	logger.Infof("Removing owner")
	return app, t.removeOwner(ctx, app.Name, tr.Owner.ID)
}

func (t *Claimer) findOneCFApp(ctx context.Context) (*heroku.App, error) {
	apps, err := AllCFApps(ctx, t.heroku)
	if err != nil {
		return nil, err
	}

	if len(apps) == 0 {
		return nil, fmt.Errorf("error: no qualified app is found in the pool")
	}

	return &apps[0], nil
}

func (t *Claimer) app(ctx context.Context, appIdentity string) (*heroku.App, error) {
	return t.heroku.AppInfo(ctx, appIdentity)
}

func (t *Claimer) updateIdleAppName(ctx context.Context, app *heroku.App) (*heroku.App, error) {
	if cfIdleAppRegexp.MatchString(app.Name) {
		cfID := cfIdleAppRegexp.FindStringSubmatch(app.Name)
		newIdentity := "cf-" + cfID[1]
		return t.heroku.AppUpdate(ctx, app.Name, heroku.AppUpdateOpts{
			Name: &newIdentity,
		})
	}

	return app, nil
}

func (t *Claimer) addGitRepo(ctx context.Context, appIdentity, gitRepo string) error {
	_, err := t.heroku.ConfigVarUpdate(ctx, appIdentity, map[string]*string{
		"GIT_REPO": &gitRepo,
	})
	return err
}

func (t *Claimer) scaleUpApp(ctx context.Context, appIdentity string) error {
	qty := 1
	_, err := t.heroku.FormationUpdate(ctx, appIdentity, "web", heroku.FormationUpdateOpts{
		Quantity: &qty,
	})
	return err
}

func (t *Claimer) addCollaborator(ctx context.Context, appIdentity, recipient string) error {
	silent := true
	_, err := t.heroku.CollaboratorCreate(ctx, appIdentity, heroku.CollaboratorCreateOpts{
		Silent: &silent,
		User:   recipient,
	})
	return err
}

func (t *Claimer) removeOwner(ctx context.Context, appIdentity, owner string) error {
	_, err := t.heroku.CollaboratorDelete(ctx, appIdentity, owner)
	return err
}

func (t *Claimer) transferApp(ctx context.Context, appIdentity, recipient string) (*heroku.AppTransfer, error) {
	silent := true
	return t.heroku.AppTransferCreate(ctx, heroku.AppTransferCreateOpts{
		App:       appIdentity,
		Recipient: recipient,
		Silent:    &silent,
	})
}

func (t *Claimer) acceptTransfer(ctx context.Context, transferID string) error {
	_, err := t.heroku.AppTransferUpdate(ctx, transferID, heroku.AppTransferUpdateOpts{
		State: "auto-accepted",
	})
	return err
}

func EditorAppURL(app *heroku.App) string {
	return fmt.Sprintf("https://%s.herokuapp.com/?folder=/home/dyno/project", app.Name)
}
