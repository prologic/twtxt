package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
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

	"github.com/prologic/twtxt"
	"github.com/prologic/twtxt/internal/passwords"
	"github.com/prologic/twtxt/types"
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

// API ...
type API struct {
	router  *Router
	config  *Config
	cache   *Cache
	archive Archiver
	db      Store
	pm      passwords.Passwords
}

// NewAPI ...
func NewAPI(router *Router, config *Config, cache *Cache, archive Archiver, db Store, pm passwords.Passwords) *API {
	api := &API{router, config, cache, archive, db, pm}

	api.initRoutes()

	return api
}

func (a *API) initRoutes() {
	router := a.router.Group("/api/v1")

	router.GET("/ping", a.PingEndpoint())
	router.POST("/auth", a.AuthEndpoint())
	router.POST("/register", a.RegisterEndpoint())

	router.POST("/post", a.isAuthorized(a.PostEndpoint()))
	router.POST("/upload", a.isAuthorized(a.UploadMediaEndpoint()))

	router.POST("/follow", a.isAuthorized(a.FollowEndpoint()))
	router.POST("/unfollow", a.isAuthorized(a.UnfollowEndpoint()))

	router.POST("/timeline", a.isAuthorized(a.TimelineEndpoint()))
	router.POST("/discover", a.DiscoverEndpoint())

	router.GET("/profile/:nick", a.ProfileEndpoint())
	router.POST("/fetch-twts", a.FetchTwtsEndpoint())

	router.GET("/external/:slug/:nick", a.ExternalProfileEndpoint())

	router.POST("/mentions", a.isAuthorized(a.MentionsEndpoint()))
}

// CreateToken ...
func (a *API) CreateToken(user *User, r *http.Request) (*Token, error) {
	claims := jwt.MapClaims{}
	claims["username"] = user.Username
	createdAt := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(a.config.APISigningKey))
	if err != nil {
		log.WithError(err).Error("error creating signed token")
		return nil, err
	}

	signedToken, err := jwt.Parse(tokenString, a.jwtKeyFunc)
	if err != nil {
		log.WithError(err).Error("error creating signed token")
		return nil, err
	}

	tkn := &Token{
		Signature: signedToken.Signature,
		Value:     tokenString,
		UserAgent: r.UserAgent(),
		CreatedAt: createdAt,
	}

	return tkn, nil
}

func (a *API) formatTwtText(twts types.Twts) types.Twts {
	res := make(types.Twts, 0)

	for _, twt := range twts {
		res = append(res, types.Twt{
			Twter:   twt.Twter,
			Text:    FormatMentionsAndTags(a.config, twt.Text, MarkdownFmt),
			Created: twt.Created,
		})
	}

	return res
}

func (a *API) jwtKeyFunc(token *jwt.Token) (interface{}, error) {
	if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, fmt.Errorf("There was an error")
	}
	return []byte(a.config.APISigningKey), nil
}

func (a *API) getLoggedInUser(r *http.Request) *User {
	token, err := jwt.Parse(r.Header.Get("Token"), a.jwtKeyFunc)
	if err != nil {
		return nil
	}

	if !token.Valid {
		return nil
	}

	claims := token.Claims.(jwt.MapClaims)

	username := claims["username"].(string)

	user, err := a.db.GetUser(username)
	if err != nil {
		log.WithError(err).Error("error loading user object")
		return nil
	}

	// Every registered new user follows themselves
	// TODO: Make  this configurable server behaviour?
	if user.Following == nil {
		user.Following = make(map[string]string)
	}

	user.Following[user.Username] = user.URL

	return user

}

func (a *API) isAuthorized(endpoint httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		if r.Header.Get("Token") == "" {
			http.Error(w, "No Token Provided", http.StatusUnauthorized)
			return
		}

		token, err := jwt.Parse(r.Header.Get("Token"), a.jwtKeyFunc)
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
		req, err := types.NewRegisterRequest(r.Body)
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
			URL:       URLForUser(a.config, username),
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
		req, err := types.NewAuthRequest(r.Body)
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

		token, err := a.CreateToken(user, r)
		if err != nil {
			log.WithError(err).Error("error creating token")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		user.AddToken(token)
		if err := a.db.SetToken(token.Signature, token); err != nil {
			log.WithError(err).Error("error saving token object")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if err := a.db.SetUser(user.Username, user); err != nil {
			log.WithError(err).Error("error saving user object")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		res := types.AuthResponse{Token: token.Value}

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

		req, err := types.NewPostRequest(r.Body)
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
			_, err = AppendTwt(a.config, a.db, user, text)
		default:
			if user.OwnsFeed(req.PostAs) {
				_, err = AppendSpecial(a.config, a.db, req.PostAs, text)
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
		a.cache.FetchTwts(a.config, a.archive, user.Source())

		// Re-populate/Warm cache with local twts for this pod
		a.cache.GetByPrefix(a.config.BaseURL, true)

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

		req, err := types.NewPagedRequest(r.Body)
		if err != nil {
			log.WithError(err).Error("error parsing post request")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		var twts types.Twts

		for feed := range user.Sources() {
			twts = append(twts, a.cache.GetByURL(feed.URL)...)
		}

		sort.Sort(twts)

		var pagedTwts types.Twts

		pager := paginator.New(adapter.NewSliceAdapter(twts), a.config.TwtsPerPage)
		pager.SetPage(req.Page)

		if err = pager.Results(&pagedTwts); err != nil {
			log.WithError(err).Error("error loading timeline")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		res := types.PagedResponse{
			Twts: a.formatTwtText(pagedTwts),
			Pager: types.PagerResponse{
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
		req, err := types.NewPagedRequest(r.Body)
		if err != nil {
			log.WithError(err).Error("error parsing post request")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		twts := a.cache.GetByPrefix(a.config.BaseURL, false)

		sort.Sort(twts)

		var pagedTwts types.Twts

		pager := paginator.New(adapter.NewSliceAdapter(twts), a.config.TwtsPerPage)
		pager.SetPage(req.Page)

		if err = pager.Results(&pagedTwts); err != nil {
			log.WithError(err).Error("error loading discover")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		res := types.PagedResponse{
			Twts: a.formatTwtText(pagedTwts),
			Pager: types.PagerResponse{
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

// MentionsEndpoint ...
func (a *API) MentionsEndpoint() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		req, err := types.NewPagedRequest(r.Body)
		if err != nil {
			log.WithError(err).Error("error parsing post request")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		user := r.Context().Value(UserContextKey).(*User)

		var twts types.Twts

		seen := make(map[string]bool)

		// Search for @mentions on feeds user is following
		for feed := range user.Sources() {
			for _, twt := range a.cache.GetByURL(feed.URL) {
				for _, twter := range twt.Mentions() {
					if user.Is(twter.URL) && !seen[twt.Hash()] {
						twts = append(twts, twt)
						seen[twt.Hash()] = true
					}
				}
			}
		}

		// Search for @mentions in local twts too (i.e: /discover)
		for _, twt := range a.cache.GetByPrefix(a.config.BaseURL, false) {
			for _, twter := range twt.Mentions() {
				if user.Is(twter.URL) && !seen[twt.Hash()] {
					twts = append(twts, twt)
					seen[twt.Hash()] = true
				}
			}
		}

		sort.Sort(twts)

		var pagedTwts types.Twts

		pager := paginator.New(adapter.NewSliceAdapter(twts), a.config.TwtsPerPage)
		pager.SetPage(req.Page)

		if err = pager.Results(&pagedTwts); err != nil {
			log.WithError(err).Error("error loading discover")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		res := types.PagedResponse{
			Twts: a.formatTwtText(pagedTwts),
			Pager: types.PagerResponse{
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

		req, err := types.NewFollowRequest(r.Body)
		if err != nil {
			log.WithError(err).Error("error parsing follow request")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		nick := strings.TrimSpace(req.Nick)
		url := NormalizeURL(req.URL)

		if nick == "" || url == "" {
			log.Warn("no nick or url provided")
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

				if _, err := AppendSpecial(
					a.config, a.db,
					twtxtBot,
					fmt.Sprintf(
						"FOLLOW: @<%s %s> from @<%s %s> using %s/%s",
						followee.Username, URLForUser(a.config, followee.Username),
						user.Username, URLForUser(a.config, user.Username),
						"twtxt", twtxt.FullVersion(),
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

				if _, err := AppendSpecial(
					a.config, a.db,
					twtxtBot,
					fmt.Sprintf(
						"FOLLOW: @<%s %s> from @<%s %s> using %s/%s",
						feed.Name, URLForUser(a.config, feed.Name),
						user.Username, URLForUser(a.config, user.Username),
						"twtxt", twtxt.FullVersion(),
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

// UnfollowEndpoint ...
func (a *API) UnfollowEndpoint() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		req, err := types.NewUnfollowRequest(r.Body)

		if err != nil {
			log.WithError(err).Error("error parsing follow request")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		nick := req.Nick

		if nick == "" {
			log.Error("no nick provided")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		user := r.Context().Value(UserContextKey).(*User)

		if user == nil {
			log.Fatalf("user not found in context")
		}

		url, ok := user.Following[nick]
		if !ok {
			log.Errorf("user %s is not following %s", nick, url)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		delete(user.Following, nick)

		if err := a.db.SetUser(user.Username, user); err != nil {
			log.WithError(err).Warnf("error updating user object for user  %s", user.Username)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		if strings.HasPrefix(url, a.config.BaseURL) {
			url = UserURL(url)
			nick := NormalizeUsername(filepath.Base(url))
			followee, err := a.db.GetUser(nick)
			if err != nil {
				log.WithError(err).Warnf("error loading user object for followee %s", nick)
			} else {
				if followee.Followers != nil {
					delete(followee.Followers, user.Username)
					if err := a.db.SetUser(followee.Username, followee); err != nil {
						log.WithError(err).Warnf("error updating user object for followee %s", followee.Username)
					}
				}
				if _, err := AppendSpecial(
					a.config, a.db,
					twtxtBot,
					fmt.Sprintf(
						"UNFOLLOW: @<%s %s> from @<%s %s> using %s/%s",
						followee.Username, URLForUser(a.config, followee.Username),
						user.Username, URLForUser(a.config, user.Username),
						"twtxt", twtxt.FullVersion(),
					),
				); err != nil {
					log.WithError(err).Warnf("error appending special FOLLOW post")
				}
			}
		}

		// No real response
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}
}

// UploadMediaEndpoint ...
func (a *API) UploadMediaEndpoint() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		// Limit request body to to abuse
		r.Body = http.MaxBytesReader(w, r.Body, a.config.MaxUploadSize)

		mediaFile, _, err := r.FormFile("media_file")
		if err != nil && err != http.ErrMissingFile {
			log.WithError(err).Error("error parsing form file")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var mediaURI string

		if mediaFile != nil {
			opts := &ImageOptions{Resize: true, ResizeW: MediaResolution, ResizeH: 0}
			mediaURI, err = StoreUploadedImage(
				a.config, mediaFile,
				mediaDir, "",
				opts,
			)

			if err != nil {
				log.WithError(err).Error("error storing the file")
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		uri := URI{"mediaURI", mediaURI}
		data, err := json.Marshal(uri)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)

		return
	}
}

// ProfileEndpoint ...
func (a *API) ProfileEndpoint() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		loggedInUser := a.getLoggedInUser(r)
		nick := NormalizeUsername(p.ByName("nick"))
		if nick == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		nick = NormalizeUsername(nick)

		var profile types.Profile

		if a.db.HasUser(nick) {
			user, err := a.db.GetUser(nick)
			if err != nil {
				log.WithError(err).Errorf("error loading user object for %s", nick)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			profile = user.Profile(a.config.BaseURL)

			if loggedInUser == nil {
				if !user.IsFollowersPubliclyVisible {
					profile.Followers = map[string]string{}
				}
				if !user.IsFollowingPubliclyVisible {
					profile.Following = map[string]string{}
				}
			}
		} else if a.db.HasFeed(nick) {
			feed, err := a.db.GetFeed(nick)
			if err != nil {
				log.WithError(err).Errorf("error loading feed object for %s", nick)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			profile = feed.Profile(a.config.BaseURL)
		} else {
			http.Error(w, "User/Feed not found", http.StatusNotFound)
			return
		}

		profileResponse := types.ProfileResponse{}

		profileResponse.Profile = profile

		profileResponse.Links = types.Links{types.Link{
			Href: fmt.Sprintf("%s/webmention", UserURL(profile.URL)),
			Rel:  "webmention",
		}}

		profileResponse.Alternatives = types.Alternatives{
			types.Alternative{
				Type:  "application/atom+xml",
				Title: fmt.Sprintf("%s local feed", a.config.Name),
				URL:   fmt.Sprintf("%s/atom.xml", a.config.BaseURL),
			},
			types.Alternative{
				Type:  "text/plain",
				Title: fmt.Sprintf("%s's Twtxt Feed", profile.Username),
				URL:   profile.URL,
			},
			types.Alternative{
				Type:  "application/atom+xml",
				Title: fmt.Sprintf("%s's Atom Feed", profile.Username),
				URL:   fmt.Sprintf("%s/atom.xml", UserURL(profile.URL)),
			},
		}

		profileResponse.Twter = types.Twter{
			Nick:   profile.Username,
			Avatar: URLForAvatar(a.config, profile.Username),
			URL:    URLForUser(a.config, profile.Username),
		}

		data, err := json.Marshal(profileResponse)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}

// ProfileTwtsEndpoint ...
func (a *API) FetchTwtsEndpoint() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		req, err := types.NewFetchTwtsRequest(r.Body)
		nick := NormalizeUsername(req.Nick)
		if nick == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		var profile types.Profile
		var twts types.Twts

		if a.db.HasUser(nick) {
			log.Debug("has user")
			user, err := a.db.GetUser(nick)
			if err != nil {
				log.WithError(err).Errorf("error loading user object for %s", nick)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			profile = user.Profile(a.config.BaseURL)
			twts = a.cache.GetByURL(profile.URL)
		} else if a.db.HasFeed(nick) {
			log.Debug("has feed")
			feed, err := a.db.GetFeed(nick)
			if err != nil {
				log.WithError(err).Errorf("error loading feed object for %s", nick)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			profile = feed.Profile(a.config.BaseURL)

			twts = a.cache.GetByURL(profile.URL)
		} else if req.Slug != "" {
			v, ok := slugs.Load(req.Slug)
			if !ok {
				http.Error(w, "User/Feed not found", http.StatusNotFound)
				return
			}

			log.Debug("slug %s", v)
			u := v.(*url.URL)
			if !a.cache.IsCached(u.String()) {
				log.Debug("is cached %s", u.String())
				sources := make(types.Feeds)
				sources[types.Feed{Nick: nick, URL: u.String()}] = true
				a.cache.FetchTwts(a.config, a.archive, sources)
			}

			twts = a.cache.GetByURL(u.String())
		} else {
			http.Error(w, "User/Feed not found", http.StatusNotFound)
			return
		}

		sort.Sort(twts)

		var pagedTwts types.Twts

		pager := paginator.New(adapter.NewSliceAdapter(twts), a.config.TwtsPerPage)
		pager.SetPage(req.Page)

		if err = pager.Results(&pagedTwts); err != nil {
			log.WithError(err).Error("error loading twts")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		res := types.PagedResponse{
			Twts: a.formatTwtText(pagedTwts),
			Pager: types.PagerResponse{
				Current:   pager.Page(),
				MaxPages:  pager.PageNums(),
				TotalTwts: pager.Nums(),
			},
		}

		data, err := json.Marshal(res)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}

// ExternalProfileEndpoint ...
func (a *API) ExternalProfileEndpoint() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		slug := p.ByName("slug")
		nick := p.ByName("nick")

		if slug == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		v, ok := slugs.Load(slug)
		if !ok {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		u := v.(*url.URL)

		if nick == "" {
			log.Warn("no nick given to external profile request")
			nick = "unknown"
		}

		url := u.String()

		profileResponse := types.ProfileResponse{}

		profileResponse.Profile = types.Profile{
			Username: nick,
			TwtURL:   url,
			URL:      URLForExternalProfile(a.config, nick, url),
		}

		profileResponse.Twter = types.Twter{
			Nick:   nick,
			Avatar: URLForExternalAvatar(a.config, nick, url),
			URL:    URLForExternalProfile(a.config, nick, url),
		}

		data, err := json.Marshal(profileResponse)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}
