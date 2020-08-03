package twtxt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"github.com/vcraescu/go-paginator"
	"github.com/vcraescu/go-paginator/adapter"

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
	Current     int `json:"current_page"`
	MaxPages    int `json:"max_pages"`
	TotalTweets int `json:"total_tweets"`
}

// TimelineResponse ...
type TimelineResponse struct {
	Tweets []Tweet `json:"tweets"`
	Pager  PagerResponse
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

	router.GET("/ping", a.PingEndpoint())
	router.POST("/auth", a.AuthEndpoint())
	router.POST("/post", a.isAuthorized(a.PostEndpoint()))
	router.POST("/timeline", a.isAuthorized(a.TimelineEndpoint()))
	router.POST("/discover", a.TimelineEndpoint())
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

// PingEndpoint ...
func (a *API) PingEndpoint() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
		return
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

// TimelineEndpoint ...
func (a *API) TimelineEndpoint() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		token := r.Context().Value(TokenContextKey).(*jwt.Token)
		claims := token.Claims.(jwt.MapClaims)

		req, err := NewTimelineRequest(r.Body)
		if err != nil {
			log.WithError(err).Error("error parsing post request")
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

		cache, err := LoadCache(a.config.Data)
		if err != nil {
			log.WithError(err).Error("error loading cache")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		var tweets Tweets

		for _, url := range user.Following {
			tweets = append(tweets, cache.GetByURL(url)...)
		}

		sort.Sort(sort.Reverse(tweets))

		var pagedTweets Tweets

		pager := paginator.New(adapter.NewSliceAdapter(tweets), a.config.TweetsPerPage)
		pager.SetPage(req.Page)

		if err = pager.Results(&pagedTweets); err != nil {
			log.WithError(err).Error("error loading timeline")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		res := TimelineResponse{
			Tweets: pagedTweets,
			Pager: PagerResponse{
				Current:     pager.Page(),
				MaxPages:    pager.PageNums(),
				TotalTweets: pager.Nums(),
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

		tweets, err := GetAllTweets(a.config)
		if err != nil {
			log.WithError(err).Error("error loading local tweets")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		sort.Sort(sort.Reverse(tweets))

		var pagedTweets Tweets

		pager := paginator.New(adapter.NewSliceAdapter(tweets), a.config.TweetsPerPage)
		pager.SetPage(req.Page)

		if err = pager.Results(&pagedTweets); err != nil {
			log.WithError(err).Error("error loading discover")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		res := TimelineResponse{
			Tweets: pagedTweets,
			Pager: PagerResponse{
				Current:     pager.Page(),
				MaxPages:    pager.PageNums(),
				TotalTweets: pager.Nums(),
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

// FollowEndpoing ...
func (a *API) FollowEndpoing() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		token := r.Context().Value(TokenContextKey).(*jwt.Token)
		claims := token.Claims.(jwt.MapClaims)

		req, err := NewFollowRequest(r.Body)
		if err != nil {
			log.WithError(err).Error("error parsing post request")
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
