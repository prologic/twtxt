package twtxt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"github.com/vcraescu/go-paginator"
	"github.com/vcraescu/go-paginator/adapter"

	"github.com/prologic/twtxt/internal/passwords"
)

// ContextKey ...
type ContextKey int

const (
	TokenContextKey ContextKey = iota
	UserContextKey
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

// RegisterRequest ...
type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

// NewRegisterRequest ...
func NewRegisterRequest(r io.Reader) (req RegisterRequest, err error) {
	body, err := ioutil.ReadAll(r)
	if err != nil {
		return
	}
	err = json.Unmarshal(body, &req)
	return
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

// TimelineRequest ...
type TimelineRequest struct {
	Page int `json:"page"`
}

// NewTimelineRequest ...
func NewTimelineRequest(r io.Reader) (req TimelineRequest, err error) {
	body, err := ioutil.ReadAll(r)
	if err != nil {
		return
	}
	err = json.Unmarshal(body, &req)
	return
}

// PagerResponse ...
type PagerResponse struct {
	Current   int `json:"current_page"`
	MaxPages  int `json:"max_pages"`
	TotalTwts int `json:"total_twts"`
}

// TimelineResponse ...
type TimelineResponse struct {
	Twts  []Twt `json:"twts"`
	Pager PagerResponse
}

// Bytes ...
func (res TimelineResponse) Bytes() ([]byte, error) {
	body, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// FollowRequest ...
type FollowRequest struct {
	Nick string `json:"nick"`
	URL  string `json:"url"`
}

// NewFollowRequest ...
func NewFollowRequest(r io.Reader) (req FollowRequest, err error) {
	body, err := ioutil.ReadAll(r)
	if err != nil {
		return
	}
	err = json.Unmarshal(body, &req)
	return
}

// API ...
type API struct {
	router *Router
	config *Config
	cache  Cache
	db     Store
	pm     passwords.Passwords
}

// NewAPI ...
func NewAPI(router *Router, config *Config, cache Cache, db Store, pm passwords.Passwords) *API {
	api := &API{router, config, cache, db, pm}

	api.initRoutes()

	return api
}

func (a *API) initRoutes() {
	router := a.router.Group("/api/v1")

	router.GET("/ping", a.PingEndpoint())
	router.POST("/auth", a.AuthEndpoint())
	router.POST("/register", a.RegisterEndpoint())

	router.POST("/post", a.isAuthorized(a.PostEndpoint()))
	router.POST("/follow", a.isAuthorized(a.FollowEndpoint()))
	router.POST("/timeline", a.isAuthorized(a.TimelineEndpoint()))
	router.POST("/discover", a.DiscoverEndpoint())
}

// CreateToken ...
func (a *API) CreateToken(user *User) (string, error) {
	claims := jwt.MapClaims{}
	claims["username"] = user.Username
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
		if r.Header["Token"][0] == "" {
			http.Error(w, "No Token Provided", http.StatusUnauthorized)
			return
		}

		token, err := jwt.Parse(r.Header["Token"][0], func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("There was an error")
			}
			return a.config.APISigningKey, nil
		})
		if err != nil {
			log.WithError(err).Error("error parsing token")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		if token.Valid {
			claims := token.Claims.(jwt.MapClaims)

			username := claims["username"].(string)

			user, err := a.db.GetUser(username)
			if err != nil {
				log.WithError(err).Error("error loading user object")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			// Every registered new user follows themselves
			// TODO: Make  this configurable server behaviour?
			if user.Following == nil {
				user.Following = make(map[string]string)
			}
			user.Following[user.Username] = user.URL

			ctx := context.WithValue(r.Context(), TokenContextKey, token)
			ctx = context.WithValue(ctx, UserContextKey, user)

			endpoint(w, r.WithContext(ctx), p)
		} else {
			http.Error(w, "Invalid Token", http.StatusUnauthorized)
			return
		}
	}
}

// PingEndpoint ...
func (a *API) PingEndpoint() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
		return
	}
}

// RegisterEndpoint ...
func (a *API) RegisterEndpoint() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		req, err := NewRegisterRequest(r.Body)
		if err != nil {
			log.WithError(err).Error("error parsing register request")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		username := NormalizeUsername(req.Username)
		password := req.Password
		email := req.Email

		if err := ValidateUsername(username); err != nil {
			http.Error(w, "Bad Username", http.StatusBadRequest)
			return
		}

		if a.db.HasUser(username) || a.db.HasFeed(username) {
			http.Error(w, "Username Exists", http.StatusBadRequest)
			return
		}

		fn := filepath.Join(a.config.Data, feedsDir, username)
		if _, err := os.Stat(fn); err == nil {
			http.Error(w, "Feed Exists", http.StatusBadRequest)
			return
		}

		if err := ioutil.WriteFile(fn, []byte{}, 0644); err != nil {
			log.WithError(err).Error("error creating new user feed")
			http.Error(w, "Feed Creation Failed", http.StatusInternalServerError)
			return
		}

		hash, err := a.pm.CreatePassword(password)
		if err != nil {
			log.WithError(err).Error("error creating password hash")
			http.Error(w, "Passwrod Creation Failed", http.StatusInternalServerError)
			return
		}

		user := &User{
			Username:  username,
			Email:     email,
			Password:  hash,
			URL:       URLForUser(a.config.BaseURL, username),
			CreatedAt: time.Now(),
		}

		if err := a.db.SetUser(username, user); err != nil {
			log.WithError(err).Error("error saving user object for new user")
			http.Error(w, "User Creation Failed", http.StatusInternalServerError)
			return
		}

		log.Infof("user registered: %v", user)
	}
}

// AuthEndpoint ...
func (a *API) AuthEndpoint() httprouter.Handle {
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

// PostEndpoint ...
func (a *API) PostEndpoint() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		user := r.Context().Value(UserContextKey).(*User)

		defer func() {
			// Update user's own timeline with their own new post.
			sources := map[string]string{user.Username: user.URL}
			a.cache.FetchTwts(a.config, sources)
		}()

		req, err := NewPostRequest(r.Body)
		if err != nil {
			log.WithError(err).Error("error parsing post request")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		text := CleanTwt(req.Text)
		if text == "" {
			log.Warn("no text provided for post")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		switch req.PostAs {
		case "", me:
			err = AppendTwt(a.config, a.db, user, text)
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

		// No real response
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
		return
	}
}

// TimelineEndpoint ...
func (a *API) TimelineEndpoint() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		user := r.Context().Value(UserContextKey).(*User)

		req, err := NewTimelineRequest(r.Body)
		if err != nil {
			log.WithError(err).Error("error parsing post request")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		var twts Twts

		for _, url := range user.Following {
			twts = append(twts, a.cache.GetByURL(url)...)
		}

		sort.Sort(sort.Reverse(twts))

		var pagedTwts Twts

		pager := paginator.New(adapter.NewSliceAdapter(twts), a.config.TwtsPerPage)
		pager.SetPage(req.Page)

		if err = pager.Results(&pagedTwts); err != nil {
			log.WithError(err).Error("error loading timeline")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		res := TimelineResponse{
			Twts: pagedTwts,
			Pager: PagerResponse{
				Current:   pager.Page(),
				MaxPages:  pager.PageNums(),
				TotalTwts: pager.Nums(),
			},
		}

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

// DiscoverEndpoint ...
func (a *API) DiscoverEndpoint() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		req, err := NewTimelineRequest(r.Body)
		if err != nil {
			log.WithError(err).Error("error parsing post request")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		twts, err := GetAllTwts(a.config)
		if err != nil {
			log.WithError(err).Error("error loading local twts")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		sort.Sort(sort.Reverse(twts))

		var pagedTwts Twts

		pager := paginator.New(adapter.NewSliceAdapter(twts), a.config.TwtsPerPage)
		pager.SetPage(req.Page)

		if err = pager.Results(&pagedTwts); err != nil {
			log.WithError(err).Error("error loading discover")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		res := TimelineResponse{
			Twts: pagedTwts,
			Pager: PagerResponse{
				Current:   pager.Page(),
				MaxPages:  pager.PageNums(),
				TotalTwts: pager.Nums(),
			},
		}

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

// FollowEndpoint ...
func (a *API) FollowEndpoint() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		user := r.Context().Value(UserContextKey).(*User)

		req, err := NewFollowRequest(r.Body)
		if err != nil {
			log.WithError(err).Error("error parsing follow request")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		nick := strings.TrimSpace(req.Nick)
		url := NormalizeURL(req.URL)

		if nick == "" || url == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		user.Following[nick] = url

		if err := a.db.SetUser(user.Username, user); err != nil {
			log.WithError(err).Error("error saving user object")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if strings.HasPrefix(url, a.config.BaseURL) {
			url = UserURL(url)
			nick := NormalizeUsername(filepath.Base(url))

			if a.db.HasUser(nick) {
				followee, err := a.db.GetUser(nick)
				if err != nil {
					log.WithError(err).Errorf("error loading user object for %s", nick)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}

				if followee.Followers == nil {
					followee.Followers = make(map[string]string)
				}

				followee.Followers[user.Username] = user.URL

				if err := a.db.SetUser(followee.Username, followee); err != nil {
					log.WithError(err).Warnf("error updating user object for followee %s", followee.Username)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}

				if err := AppendSpecial(
					a.config, a.db,
					twtxtBot,
					fmt.Sprintf(
						"FOLLOW: @<%s %s> from @<%s %s> using %s/%s",
						followee.Username, URLForUser(a.config.BaseURL, followee.Username),
						user.Username, URLForUser(a.config.BaseURL, user.Username),
						"twtxt", FullVersion(),
					),
				); err != nil {
					log.WithError(err).Warnf("error appending special FOLLOW post")
				}
			} else if a.db.HasFeed(nick) {
				feed, err := a.db.GetFeed(nick)
				if err != nil {
					log.WithError(err).Errorf("error loading feed object for %s", nick)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}

				feed.Followers[user.Username] = user.URL

				if err := a.db.SetFeed(feed.Name, feed); err != nil {
					log.WithError(err).Warnf("error updating user object for followee %s", feed.Name)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}

				if err := AppendSpecial(
					a.config, a.db,
					twtxtBot,
					fmt.Sprintf(
						"FOLLOW: @<%s %s> from @<%s %s> using %s/%s",
						feed.Name, URLForUser(a.config.BaseURL, feed.Name),
						user.Username, URLForUser(a.config.BaseURL, user.Username),
						"twtxt", FullVersion(),
					),
				); err != nil {
					log.WithError(err).Warnf("error appending special FOLLOW post")
				}
			}
		}

		// No real response
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
		return
	}
}
