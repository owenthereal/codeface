package command

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jingweno/codeface/worker"
	"github.com/joeshaw/envdecode"
	"github.com/spf13/cobra"
)

func workerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Start the worker",
		RunE:  workerRunE,
	}

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

	worker := worker.New(cfg)
	return worker.Start(ctx)
}
