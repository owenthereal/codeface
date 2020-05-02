package command

import (
	"context"
	"fmt"

	"github.com/jingweno/codeface/editor"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

var (
	appIdentity string
	recipient   string
	gitRepo     string
)

func claimCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "claim",
		Short: "Claim a Codeface app",
		RunE:  claimRunE,
	}

	cmd.PersistentFlags().StringVarP(&herokuAPIToken, "token", "t", "", "Heroku API token (required)")
	cmd.PersistentFlags().StringVarP(&appIdentity, "app", "a", "", "Heroku app identity (optional)")
	cmd.PersistentFlags().StringVarP(&recipient, "recipient", "r", "", "recipient (required)")
	cmd.PersistentFlags().StringVarP(&gitRepo, "git", "g", "", "Git repository (required)")

	return cmd
}

func claimRunE(c *cobra.Command, args []string) error {
	if herokuAPIToken == "" || recipient == "" || gitRepo == "" {
		return fmt.Errorf("missing required flags")
	}

	t := editor.NewClaimer(herokuAPIToken)
	app, err := t.Claim(context.Background(), appIdentity, recipient, gitRepo)
	if err != nil {
		return err
	}

	url := editor.EditorAppURL(app)
	fmt.Printf("Visit %s\n", url)
	return browser.OpenURL(url)
}
