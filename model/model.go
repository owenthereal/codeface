package model

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
)

type EditorRequest struct {
	GitRepo string
}

func ParseGitHubRepoURL(s string) (string, error) {
	u, err := url.ParseRequestURI(s)
	if err != nil {
		return "", err
	}

	if u.Scheme != "https" || u.Host != "github.com" {
		return "", fmt.Errorf("Please provide a GitHub repository URL")
	}

	split := strings.Split(strings.TrimLeft(path.Clean(u.Path), "/"), "/")
	if len(split) < 2 {
		return "", fmt.Errorf("Please provide a valid GitHub repository URL")
	}

	user := split[0]
	repo := split[1]

	resp, err := http.Get(fmt.Sprintf("https://api.github.com/repos/%s/%s", user, repo))
	if err != nil {
		return "", err
	}

	if resp.StatusCode == 200 {
		return fmt.Sprintf("https://github.com/%s/%s", user, repo), nil
	}

	return "", fmt.Errorf("GitHub repository is not found or accessible")
}

type EditorResponse struct {
	URL string
}

type ErrorResponse struct {
	Error string
}
