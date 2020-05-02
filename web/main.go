package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"syscall/js"

	"github.com/gopherjs/vecty"
	"github.com/gopherjs/vecty/elem"
	"github.com/gopherjs/vecty/event"
	"github.com/gopherjs/vecty/prop"
	"github.com/jingweno/codeface/model"
)

func main() {
	vecty.SetTitle("Codeface")
	vecty.RenderBody(&PageView{
		Build: "Hello",
	})
}

type PageView struct {
	vecty.Core
	Repo  string
	Build string
}

func (p *PageView) Render() vecty.ComponentOrHTML {
	return elem.Body(
		elem.Heading1(
			vecty.Text(p.Build),
		),
		elem.Form(
			vecty.Markup(
				event.Submit(p.onEnter).PreventDefault(),
			),
			elem.Input(
				vecty.Markup(
					prop.Placeholder("What is the git repository URL?"),
					prop.Autofocus(true),
					prop.Value(p.Repo),
					event.Input(p.onInput),
				),
			),
		),
	)
}

func (p *PageView) onInput(event *vecty.Event) {
	p.Repo = event.Target.Get("value").String()
	vecty.Rerender(p)
}

func (p *PageView) onEnter(event *vecty.Event) {
	go func(repo string) {
		url, err := p.claimEditor(repo)
		if err != nil {
			fmt.Println(err)
			p.Build = err.Error()
			vecty.Rerender(p)
			return
		}

		loc := js.Global().Get("window").Get("location")
		loc.Call("replace", url)
	}(p.Repo)

	p.Repo = ""
	vecty.Rerender(p)
}

func (p *PageView) claimEditor(repo string) (string, error) {
	req := model.EditorRequest{
		GitRepo: repo,
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

	if resp.StatusCode > 300 {
		var errResp model.ErrorResponse
		dec := json.NewDecoder(resp.Body)
		if err := dec.Decode(&errResp); err != nil {
			return "", err
		}

		return "", fmt.Errorf("error: fail to claim editor status=%d error=%s", resp.StatusCode, errResp.Error)
	}

	var editorResp model.EditorResponse
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&editorResp); err != nil {
		return "", err
	}

	return editorResp.URL, nil
}
