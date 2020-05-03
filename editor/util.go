package editor

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	heroku "github.com/heroku/heroku-go/v5"
	"github.com/rs/xid"
	log "github.com/sirupsen/logrus"
)

var (
	// cf building app name is in the format of cf-#{ID}-#{VERSION}b
	cfBuildingAppRegexp = regexp.MustCompile(fmt.Sprintf("cf-(.+)-%sb", dashizedVersion()))
	// cf idle app name is in the format of cf-#{ID}-#{VERSION}i
	cfIdleAppRegexp = regexp.MustCompile(fmt.Sprintf("cf-(.+)-%si", dashizedVersion()))
)

func buildClaimedAppName(id string) string {
	return fmt.Sprintf("cf-%s-%s", id, dashizedVersion())
}

func buildIdleAppName(id string) string {
	return fmt.Sprintf("cf-%s-%si", id, dashizedVersion())
}

func genBuildingAppName() string {
	return fmt.Sprintf("cf-%s-%sb", xid.New().String(), dashizedVersion())
}

func dashizedVersion() string {
	return strings.ReplaceAll(version, ".", "")
}

func AllIdledApps(ctx context.Context, client *heroku.Service) ([]heroku.App, error) {
	var result []heroku.App

	apps, err := client.AppListOwnedAndCollaborated(ctx, "~", &heroku.ListRange{
		Field: "name",
		Max:   1000, // FIXME: hardcode
	})
	if err != nil {
		return nil, err
	}

	for _, app := range apps {
		if cfIdleAppRegexp.MatchString(app.Name) {
			result = append(result, app)
		}
	}

	return result, nil
}

func Account(ctx context.Context, client *heroku.Service) (*heroku.Account, error) {
	acct, err := client.AccountInfo(ctx)
	if err != nil {
		return nil, err
	}

	if acct.Email == "" {
		return nil, fmt.Errorf("error: fail to get account email")
	}

	return acct, nil
}

func deleteFailedApp(client *heroku.Service, app *heroku.App, logger log.FieldLogger) {
	logger = logger.WithField("app", app.Name)

	logger.Info("Removing failed app")
	// use a new ctx to make sure it's detached
	_, err := client.AppDelete(context.Background(), app.Name)
	if err != nil {
		logger.WithError(err).Info("Fail to remove failed app")
	}
}
