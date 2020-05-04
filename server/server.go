package server

import (
	"context"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/heroku"

	hkclient "github.com/heroku/heroku-go/v5"
	"github.com/jingweno/codeface/editor"
	"github.com/jingweno/codeface/model"
	"github.com/shurcooL/httpgzip"
	log "github.com/sirupsen/logrus"
)

type contextKey int

const (
	accountKey contextKey = iota
)

func init() {
	// for cookie store
	gob.Register(&oauth2.Token{})
}

type Config struct {
	Port               string   `env:"PORT,required"`
	HerokuAPIKey       string   `env:"HEROKU_API_KEY,required"`
	HerokuClientID     string   `env:"HEROKU_CLIENT_ID,required"`
	HerokuClientSecret string   `env:"HEROKU_CLIENT_SECRET,required"`
	WhitelistUsers     []string `env:"WHITELIST_USERS"`
	// cat /dev/urandom | base64 | head -c 64
	SessionKey string `env:"SESSION_KEY,required"`
}

func New(cfg Config) *Server {
	return &Server{
		cfg:    cfg,
		logger: log.New().WithField("com", "server"),
	}
}

type Server struct {
	cfg    Config
	logger log.FieldLogger
}

func (s *Server) Serve() error {
	h := handlers{
		herokuAPIKey:   s.cfg.HerokuAPIKey,
		whitelistUsers: s.cfg.WhitelistUsers,
		store:          sessions.NewCookieStore([]byte(s.cfg.SessionKey)),
		oauthConf: &oauth2.Config{
			ClientID:     s.cfg.HerokuClientID,
			ClientSecret: s.cfg.HerokuClientSecret,
			Scopes:       []string{"identity"},
			Endpoint:     heroku.Endpoint,
		},
		logger: s.logger,
	}

	r := mux.NewRouter()

	r.Use(mux.CORSMethodMiddleware(r))
	r.Use(h.AuthMiddleware)

	r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", httpgzip.FileServer(
		AssetFile(),
		httpgzip.FileServerOptions{},
	)))
	r.Path("/").Handler(http.FileServer(AssetFile())) // for index.html

	r.Methods("POST").Path("/editor").HandlerFunc(h.HandleEditor)
	r.Methods("GET").Path("/login").HandlerFunc(h.HandleLogin)
	r.Methods("GET").Path("/callback").HandlerFunc(h.HandleCallback)
	r.Methods("GET").Path("/health").HandlerFunc(h.HandleHealth)

	http.Handle("/", r)

	s.logger.Infof("Starting server on %s", s.cfg.Port)

	return http.ListenAndServe(":"+s.cfg.Port, nil)
}

type handlers struct {
	herokuAPIKey   string
	whitelistUsers []string
	store          sessions.Store
	oauthConf      *oauth2.Config
	logger         log.FieldLogger
}

func (h *handlers) HandleHome(w http.ResponseWriter, r *http.Request) {
	acct := r.Context().Value(accountKey)
	fmt.Println(acct)
}

func (h *handlers) HandleEditor(w http.ResponseWriter, r *http.Request) {
	acct := r.Context().Value(accountKey).(*hkclient.Account)

	var opt model.EditorRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&opt); err != nil {
		jsonResp(w, http.StatusUnprocessableEntity, model.ErrorResponse{err.Error()})
		return
	}

	fmt.Println(opt.GitRepo)
	url, err := model.ParseGitHubRepoURL(opt.GitRepo)
	if err != nil {
		jsonResp(w, http.StatusUnprocessableEntity, model.ErrorResponse{err.Error()})
		return
	}

	c := editor.NewClaimer(h.herokuAPIKey)
	app, err := c.Claim(r.Context(), "", acct.Email, url)
	if err != nil {
		h.logger.WithError(err).Info("error: fail to claim an app")
		jsonResp(w, http.StatusUnprocessableEntity, model.ErrorResponse{err.Error()})
		return
	}

	jsonResp(w, http.StatusCreated, model.EditorResponse{
		URL: editor.EditorAppURL(app),
	})
}

func (h *handlers) heroku(token string) *hkclient.Service {
	client := &http.Client{
		Transport: &hkclient.Transport{
			BearerToken: token,
		},
	}

	return hkclient.NewService(client)
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

func (h *handlers) HandleHealth(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "hello owen")
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

		acct, err := editor.Account(r.Context(), h.heroku(tok.AccessToken))
		if err != nil {
			delete(session.Values, "token") // delete session and retry
			if err := session.Save(r, w); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			return
		}

		allowed := len(h.whitelistUsers) == 0
		for _, u := range h.whitelistUsers {
			if strings.Contains(acct.Email, u) {
				allowed = true
				break
			}
		}

		if allowed {
			ctx := context.WithValue(r.Context(), accountKey, acct)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		} else {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
	})
}

func jsonResp(w http.ResponseWriter, status int, i interface{}) {
	w.WriteHeader(status)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	enc := json.NewEncoder(w)
	if err := enc.Encode(i); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
