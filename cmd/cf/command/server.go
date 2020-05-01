package command

import (
	"github.com/jingweno/codeface/server"
	"github.com/joeshaw/envdecode"
	"github.com/spf13/cobra"
)

func serverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start the server",
		RunE:  serverRunE,
	}

	return cmd
}

func serverRunE(c *cobra.Command, args []string) error {
	var cfg server.Config
	if err := envdecode.StrictDecode(&cfg); err != nil {
		return err
	}

	s := server.New(cfg)
	return s.Serve()
}
