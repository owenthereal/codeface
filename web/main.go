package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"syscall/js"
	"time"

	"github.com/gopherjs/vecty"
	"github.com/gopherjs/vecty/elem"
	"github.com/gopherjs/vecty/event"
	"github.com/gopherjs/vecty/prop"
	"github.com/jingweno/codeface/model"
)

func main() {
	vecty.SetTitle("Codeface")
	vecty.AddStylesheet("/assets/style.css")
	pv := &PageView{
		GitHubRepoURL: gitHubRepoFromURL(),
	}
	// if repo exists from url param ?repo=...
	if pv.GitHubRepoURL != "" {
		go pv.claimEditor(pv.GitHubRepoURL)
	}

	vecty.RenderBody(pv)
}

type PageView struct {
	vecty.Core
	GitHubRepoURL   string
	ValidFeedback   string
	InvalidFeedback string
	IsWorking       bool

	input *vecty.HTML
}

func (p *PageView) Render() vecty.ComponentOrHTML {
	p.input = elem.Input(
		vecty.Markup(
			prop.Type(prop.TypeURL),
			prop.ID("inputRepo"),
			vecty.Class("form-control"),
			prop.Placeholder("GitHub repository URL"),
			prop.Autofocus(true),
			vecty.Property("required", true),
			prop.Value(p.GitHubRepoURL),
			event.Input(p.onInput),
			vecty.MarkupIf(
				p.ValidFeedback != "",
				vecty.Class("is-valid"),
			),
			vecty.MarkupIf(
				p.InvalidFeedback != "",
				vecty.Class("is-invalid"),
			),
		),
	)

	return elem.Body(
		elem.Form(
			vecty.Markup(
				vecty.Class("form-signin"),
				event.Submit(p.onEnter).PreventDefault(),
			),
			vecty.Tag("fieldset",
				vecty.Markup(
					prop.Disabled(p.IsWorking),
				),
				elem.Div(
					vecty.Markup(
						vecty.Class("text-center"),
						vecty.Class("mb-4"),
					),
					elem.Heading1(
						vecty.Markup(
							vecty.Class("h3"),
							vecty.Class("mb-3"),
							vecty.Class("font-weight-normal"),
						),
						vecty.Text("Codeface"),
					),
					elem.Paragraph(vecty.Text("Run VS Code on Heroku")),
				),
				elem.Div(
					vecty.Markup(
						vecty.Class("form-label-group"),
						vecty.Class("mb-3"),
					),
					p.input,
					elem.Small(
						vecty.Markup(
							vecty.Class("form-text"),
							vecty.Class("text-muted"),
						),
						vecty.Text("The GitHub repository URL must be a valid public repository URL, e.g., https://github.com/jingweno/upterm."),
					),
					vecty.If(
						p.ValidFeedback != "",
						elem.Div(
							vecty.Markup(
								vecty.Class("valid-feedback"),
							),
							vecty.Text(p.ValidFeedback),
						),
					),
					vecty.If(
						p.InvalidFeedback != "",
						elem.Div(
							vecty.Markup(
								vecty.Class("invalid-feedback"),
							),
							vecty.Text(p.InvalidFeedback),
						),
					),
					elem.Label(
						vecty.Markup(
							prop.For("inputRepo"),
						),
						vecty.Text("GitHub repository URL"),
					),
				),
				elem.Button(
					vecty.Markup(
						vecty.Class("btn"),
						//vecty.Class("btn-lg"),
						vecty.Class("btn-primary"),
						vecty.Class("btn-block"),
						prop.Type(prop.TypeSubmit),
						event.Click(p.onEnter).PreventDefault(),
					),
					vecty.If(
						p.IsWorking,
						elem.Span(
							vecty.Markup(
								vecty.Class("spinner-border"),
								vecty.Class("spinner-border-sm"),
							),
						),
					),
					vecty.Text("\nRun\n"),
				),
			),
		),
	)
}

func (p *PageView) onInput(event *vecty.Event) {
	p.GitHubRepoURL = event.Target.Get("value").String()
	if p.GitHubRepoURL == "" {
		p.resetFields()
	}
	vecty.Rerender(p)
}

func (p *PageView) resetFields() {
	p.ValidFeedback = ""
	p.InvalidFeedback = ""
	p.IsWorking = false
}

func (p *PageView) onEnter(event *vecty.Event) {
	p.resetFields()
	vecty.Rerender(p)
	go p.claimEditor(p.GitHubRepoURL)
}

func (p *PageView) claimEditor(repo string) {
	p.IsWorking = true // mark as working
	vecty.Rerender(p)

	url, err := claimEditor(repo)
	if err == nil {
		p.ValidFeedback = fmt.Sprintf("Please wait, redirecting to %s", url)
		p.IsWorking = true
		vecty.Rerender(p)
		redirectTo(url)
	} else {
		p.InvalidFeedback = err.Error()
		p.IsWorking = false
		vecty.Rerender(p)
		// FIXME: hack, Rerender appears to be async
		time.Sleep(200 * time.Millisecond)
		p.input.Node().Call("focus")
	}
}

func claimEditor(url string) (string, error) {
	u, err := model.ParseGitHubRepoURL(url)
	if err != nil {
		return "", err
	}

	req := model.EditorRequest{
		GitRepo: u,
	}

	b, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	resp, err := http.Post("/editor", "application/json", bytes.NewBuffer(b))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var errResp model.ErrorResponse
		dec := json.NewDecoder(resp.Body)
		if err := dec.Decode(&errResp); err != nil {
			return "", err
		}

		return "", fmt.Errorf(errResp.Error)
	}

	var editorResp model.EditorResponse
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&editorResp); err != nil {
		return "", err
	}

	return editorResp.URL, nil
}

func redirectTo(url string) {
	loc := js.Global().Get("window").Get("location")
	loc.Set("href", url)
}

func gitHubRepoFromURL() string {
	loc := js.Global().Get("window").Get("location")
	u, err := url.ParseRequestURI(loc.Get("href").String())
	if err != nil {
		return ""
	}

	return u.Query().Get("repo")
}
