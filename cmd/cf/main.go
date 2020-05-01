package main

import (
	"github.com/jingweno/codeface/cmd/cf/command"
	log "github.com/sirupsen/logrus"
)

func main() {
	rootCmd := command.Root()
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
