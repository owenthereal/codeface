package command

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/jingweno/codeface/worker"
	"github.com/joeshaw/envdecode"
	"github.com/spf13/cobra"
)

var (
	templateDir string
)

func workerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Start the worker",
		RunE:  workerRunE,
	}

	pwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	cmd.PersistentFlags().StringVarP(&templateDir, "template", "", filepath.Join(pwd, "template"), "deployment template directory")

	return cmd
}

func workerRunE(c *cobra.Command, args []string) error {
	var cfg worker.Config
	if err := envdecode.StrictDecode(&cfg); err != nil {
		return err
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-sigs
		cancel()
	}()

	cfg.TemplateDir = templateDir

	worker := worker.New(cfg)
	return worker.Start(ctx)
}
