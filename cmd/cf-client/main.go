package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jingweno/codeface"
	"github.com/pkg/browser"
	log "github.com/sirupsen/logrus"
)

func main() {
	accessToken := os.Getenv("HEROKU_API_TOKEN")
	repo := os.Getenv("GIT_REPO")

	if accessToken == "" || repo == "" {
		log.Fatalf("Provide HEROKU_API_TOKEN and GIT_REPO")
	}

	deployer := codeface.NewDeployer(accessToken)
	url, err := deployer.DeployEditorAppAndWait(context.Background(), repo, os.Stderr)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Open your browser at %s\n", url)
	browser.OpenURL(url)
}
