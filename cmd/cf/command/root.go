package command

import (
	"github.com/spf13/cobra"
)

var (
	herokuAPIToken string
)

func Root() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "cf",
		Short: "Codeface",
	}

	rootCmd.AddCommand(claimCmd())
	rootCmd.AddCommand(deployCmd())
	rootCmd.AddCommand(workerCmd())
	rootCmd.AddCommand(serverCmd())

	return rootCmd
}
