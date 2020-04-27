package main

import (
	"context"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/jingweno/codeface"
	"github.com/joeshaw/envdecode"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/heroku"
)

type Config struct {
	Port               string `env:"PORT",required`
	HerokuClientID     string `env:"HEROKU_CLIENT_ID",required`
	HerokuClientSecret string `env:"HEROKU_CLIENT_SECRET",required`
	// cat /dev/urandom | base64 | head -c 64
	SessionKey string `env:"SESSION_KEY",required`
}

type contextKey int

const (
	tokenKey contextKey = iota
)

func init() {
	gob.Register(&oauth2.Token{})
}

func main() {
	var cfg Config
	if err := envdecode.Decode(&cfg); err != nil {
		log.Fatal(err)
	}

	h := handlers{
		store: sessions.NewCookieStore([]byte(cfg.SessionKey)),
		oauthConf: &oauth2.Config{
			ClientID:     cfg.HerokuClientID,
			ClientSecret: cfg.HerokuClientSecret,
			Scopes:       []string{"global"},
			Endpoint:     heroku.Endpoint,
		},
	}

	r := mux.NewRouter()

	r.Use(mux.CORSMethodMiddleware(r))
	r.Use(h.AuthMiddleware)

	r.Methods("GET").Path("/").HandlerFunc(h.HandleHome)
	r.Methods("POST").Path("/apps/{app}/actions/edit").HandlerFunc(h.HandleEdit)
	r.Methods("GET").Path("/apps/{app}/builds/{build}").HandlerFunc(h.HandleBuildInfo)
	r.Methods("GET").Path("/login").HandlerFunc(h.HandleLogin)
	r.Methods("GET").Path("/callback").HandlerFunc(h.HandleCallback)

	http.Handle("/", r)

	log.Infof("Starting server on %s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, nil))
}

type handlers struct {
	store     sessions.Store
	oauthConf *oauth2.Config
}

func (h *handlers) HandleHome(w http.ResponseWriter, r *http.Request) {
	tok := r.Context().Value(tokenKey)
	fmt.Println(tok)
}

func (h *handlers) HandleEdit(w http.ResponseWriter, r *http.Request) {
	tok := r.Context().Value(tokenKey).(*oauth2.Token)
	vars := mux.Vars(r)

	d := codeface.NewDeployer(tok.AccessToken)
	build, err := d.DeployEditorApp(r.Context(), vars["app"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	enc := json.NewEncoder(w)
	if err := enc.Encode(build); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
}

func (h *handlers) HandleBuildInfo(w http.ResponseWriter, r *http.Request) {
	tok := r.Context().Value(tokenKey).(*oauth2.Token)
	vars := mux.Vars(r)

	d := codeface.NewDeployer(tok.AccessToken)
	build, err := d.BuildInfo(r.Context(), vars["app"], vars["build"])

	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	enc := json.NewEncoder(w)
	if err := enc.Encode(build); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
}

func (h *handlers) HandleLogin(w http.ResponseWriter, r *http.Request) {
	session, err := h.store.Get(r, "session")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	state := base64.StdEncoding.EncodeToString(securecookie.GenerateRandomKey(32))
	session.AddFlash(state, "state")
	if err := session.Save(r, w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	url := h.oauthConf.AuthCodeURL(state, oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (h *handlers) HandleCallback(w http.ResponseWriter, r *http.Request) {
	session, err := h.store.Get(r, "session")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sessionState := session.Flashes("state")
	if len(sessionState) != 1 {
		http.Error(w, "tampered oauth state", http.StatusUnauthorized)
		return
	}

	query := r.URL.Query()
	if state := query.Get("state"); state != sessionState[0].(string) {
		http.Error(w, "error validating state", http.StatusUnauthorized)
		return
	}

	code := query.Get("code")
	if code == "" {
		http.Error(w, "error receiving oauth code", http.StatusUnauthorized)
		return
	}

	tok, err := h.oauthConf.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, "error exchanging oauth code", http.StatusUnauthorized)
		return
	}

	session.Values["token"] = tok

	redirect := "/"
	if uri := session.Flashes("redirect-uri"); len(uri) > 0 {
		redirect = uri[0].(string)
	}

	if err := session.Save(r, w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, redirect, http.StatusTemporaryRedirect)
}

func (h *handlers) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/login" || path == "/callback" {
			next.ServeHTTP(w, r)
			return
		}

		session, err := h.store.Get(r, "session")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tok, ok := session.Values["token"].(*oauth2.Token)
		// Redirect to login when no token in cookies or token expires
		if !ok || !tok.Valid() {
			// Store current uri after oauth callback for GET method
			if r.Method == "GET" {
				session.AddFlash(r.URL.String(), "redirect-uri")
				if err := session.Save(r, w); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}

			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			return
		}

		ctx := context.WithValue(r.Context(), tokenKey, tok)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}
