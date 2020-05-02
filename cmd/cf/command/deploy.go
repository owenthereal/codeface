package command

import (
	"context"
	"fmt"

	"github.com/jingweno/codeface/editor"
	"github.com/spf13/cobra"
)

func deployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy a Codeface editor to Heroku",
		RunE:  deployRunE,
	}

	cmd.PersistentFlags().StringVarP(&herokuAPIToken, "token", "t", "", "Heroku API token (required)")
	cmd.PersistentFlags().StringVarP(&templateDir, "template", "", "./template", "deployment template directory")

	return cmd
}

func deployRunE(c *cobra.Command, args []string) error {
	if herokuAPIToken == "" {
		return fmt.Errorf("missing required flags")
	}

	d := editor.NewDeployer(herokuAPIToken, templateDir)
	app, err := d.DeployEditorAndScaleDown(context.Background())
	if err != nil {
		return err
	}

	fmt.Printf("Deployed Codeface app: %s\n", app.Name)

	return nil
}
