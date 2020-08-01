package twtxt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"

	"github.com/prologic/twtxt/passwords"
)

// ContextKey ...
type ContextKey int

const (
	TokenContextKey ContextKey = iota
)

var (
	// ErrInvalidCredentials is returned for invalid credentials against /auth
	ErrInvalidCredentials = errors.New("error: invalid credentials")

	// ErrInvalidToken is returned for expired or invalid tokens used in Authorizeation headers
	ErrInvalidToken = errors.New("error: invalid token")
)

// AuthRequest ...
type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// NewAuthRequest ...
func NewAuthRequest(r io.Reader) (req AuthRequest, err error) {
	body, err := ioutil.ReadAll(r)
	if err != nil {
		return
	}
	err = json.Unmarshal(body, &req)
	return
}

// AuthResponse ...
type AuthResponse struct {
	Token string `json:"token"`
}

// Bytes ...
func (res AuthResponse) Bytes() ([]byte, error) {
	body, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// PostRequest ...
type PostRequest struct {
	PostAs string `json:"post_as"`
	Text   string `json:"text"`
}

// NewPostRequest ...
func NewPostRequest(r io.Reader) (req PostRequest, err error) {
	body, err := ioutil.ReadAll(r)
	if err != nil {
		return
	}
	err = json.Unmarshal(body, &req)
	return
}

// PostResponse ...
type PostResponse struct {
}

// Bytes ...
func (res PostResponse) Bytes() ([]byte, error) {
	body, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// API ...
type API struct {
	router *Router
	config *Config
	db     Store
	pm     passwords.Passwords
}

// NewAPI ...
func NewAPI(router *Router, config *Config, db Store, pm passwords.Passwords) *API {
	api := &API{router, config, db, pm}

	api.initRoutes()

	return api
}

func (a *API) initRoutes() {
	router := a.router.Group("/api/v1")

	router.GET("/ping", a.PingHandler())
	router.POST("/auth", a.AuthHandler())
	router.POST("/post", a.isAuthorized(a.PostHandler()))
}

// CreateToken ...
func (a *API) CreateToken(user *User) (string, error) {
	claims := jwt.MapClaims{}
	claims["authorized"] = true
	claims["username"] = user.Username
	claims["feeds"] = user.Feeds
	claims["expiery"] = time.Now().Add(a.config.APISessionTime).Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(a.config.APISigningKey)
	if err != nil {
		log.WithError(err).Error("error creating signed token")
		return "", err
	}
	return tokenString, nil
}

func (a *API) isAuthorized(endpoint httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		if r.Header["Token"] != nil {
			token, err := jwt.Parse(r.Header["Token"][0], func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("There was an error")
				}
				return a.config.APISigningKey, nil
			})
			if err != nil {
				log.WithError(err).Error("error parsing token")
				return
			}

			if token.Valid {
				ctx := context.WithValue(r.Context(), TokenContextKey, token)
				endpoint(w, r.WithContext(ctx), p)
			}
		} else {
			fmt.Fprintf(w, "Not Authorized")
		}
	}
}

// PingHandler ...
func (a *API) PingHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		w.Header().Set("Content-Type", "application/json")
		return
	}
}

// AuthHandler ...
func (a *API) AuthHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		req, err := NewAuthRequest(r.Body)
		if err != nil {
			log.WithError(err).Error("error parsing auth request")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		username := NormalizeUsername(req.Username)
		password := req.Password

		// Error: no username or password provided
		if username == "" || password == "" {
			log.Warn("no username or password provided")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		// Lookup user
		user, err := a.db.GetUser(username)
		if err != nil {
			log.WithField("username", username).Warn("login attempt from non-existent user")
			http.Error(w, "Invalid Credentials", http.StatusUnauthorized)
			return
		}

		// Validate cleartext password against KDF hash
		err = a.pm.CheckPassword(user.Password, password)
		if err != nil {
			log.WithField("username", username).Warn("login attempt with invalid credentials")
			http.Error(w, "Invalid Credentials", http.StatusUnauthorized)
			return
		}

		// Login successful
		log.WithField("username", username).Info("login successful")

		token, err := a.CreateToken(user)
		if err != nil {
			log.WithError(err).Error("error creating token")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		user.AddToken(token)
		if err := a.db.SetUser(user.Username, user); err != nil {
			log.WithError(err).Error("error saving user object")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		res := AuthResponse{Token: token}

		body, err := res.Bytes()
		if err != nil {
			log.WithError(err).Error("error serializing response")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}
}

// PostHandler ...
func (a *API) PostHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		token := r.Context().Value(TokenContextKey).(*jwt.Token)
		claims := token.Claims.(jwt.MapClaims)

		req, err := NewPostRequest(r.Body)
		if err != nil {
			log.WithError(err).Error("error parsing post request")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		text := CleanTweet(req.Text)
		if text == "" {
			log.Warn("no text provided for post")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		username := claims["username"].(string)

		user, err := a.db.GetUser(username)
		if err != nil {
			log.WithError(err).Error("error loading user object")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		switch req.PostAs {
		case "", "me":
			err = AppendTweet(a.config, a.db, user, text)
		default:
			if user.OwnsFeed(req.PostAs) {
				err = AppendSpecial(a.config, a.db, req.PostAs, text)
			} else {
				err = ErrFeedImposter
			}
		}

		if err != nil {
			log.WithError(err).Error("error posting twt")
			if err == ErrFeedImposter {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			} else {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
			return
		}

		// Update user's own timeline with their own new post.
		sources := map[string]string{
			user.Username: user.URL,
		}

		if err := func() error {
			cache, err := LoadCache(a.config.Data)
			if err != nil {
				log.WithError(err).Warn("error loading feed cache")
				return err
			}

			cache.FetchTweets(a.config, sources)

			if err := cache.Store(a.config.Data); err != nil {
				log.WithError(err).Warn("error saving feed cache")
				return err
			}
			return nil
		}(); err != nil {
			log.WithError(err).Error("error updating feed cache")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		res := PostResponse{}

		body, err := res.Bytes()
		if err != nil {
			log.WithError(err).Error("error serializing response")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}
}
