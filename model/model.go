package model

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type EditorRequest struct {
	GitRepo string
}

func (r EditorRequest) Validate() error {
	u, err := url.ParseRequestURI(r.GitRepo)
	if err != nil {
		return err
	}

	if u.Scheme != "https" || u.Host != "github.com" {
		return fmt.Errorf("Please provide a GitHub repository URL")
	}

	resp, err := http.Get(fmt.Sprintf("https://api.github.com/repos/%s", strings.TrimLeft(u.Path, "/")))
	if err != nil {
		return err
	}

	if resp.StatusCode == 200 {
		return nil
	}

	return fmt.Errorf("GitHub repository is not found or accessible")
}

type EditorResponse struct {
	URL string
}

type ErrorResponse struct {
	Error string
}
