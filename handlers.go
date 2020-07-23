package twtxt

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"github.com/vcraescu/go-paginator"
	"github.com/vcraescu/go-paginator/adapter"

	"github.com/prologic/twtxt/session"
)

var (
	ErrFeedImposter = errors.New("error: imposter detected, you do not own this feed")
)

func (s *Server) NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	ctx := NewContext(s.config, s.db, r)
	w.WriteHeader(http.StatusNotFound)
	s.render("404", w, ctx)
}

// PageHandler ...
func (s *Server) PageHandler(name string) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)
		s.render(name, w, ctx)
	}
}

// ProfileHandler ...
func (s *Server) ProfileHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		nick := NormalizeUsername(p.ByName("nick"))
		if nick == "" {
			ctx.Error = true
			ctx.Message = "No user specified"
			s.render("error", w, ctx)
			return
		}

		nick = NormalizeUsername(nick)

		userProfile, err := s.db.GetUser(nick)
		if err != nil {
			log.WithError(err).Errorf("error loading user object for %s", nick)
			ctx.Error = true
			ctx.Message = "Error loading profile"
			s.render("error", w, ctx)
			return
		}

		ctx.Profile = userProfile

		var (
			tweets Tweets
			cache  Cache
		)

		cache, err = LoadCache(s.config.Data)
		if err != nil {
			log.WithError(err).Error("error loading cache")
			ctx.Error = true
			ctx.Message = "An error occurred while loading the profile"
			s.render("error", w, ctx)
			return
		}

		tweets = append(tweets, cache.GetByURL(userProfile.URL)...)

		sort.Sort(sort.Reverse(tweets))

		var pagedTweets Tweets

		page := SafeParseInt(r.FormValue("page"), 1)
		pager := paginator.New(adapter.NewSliceAdapter(tweets), s.config.TweetsPerPage)
		pager.SetPage(page)

		if err = pager.Results(&pagedTweets); err != nil {
			ctx.Error = true
			ctx.Message = "An error occurred while loading the  timeline"
			s.render("error", w, ctx)
			return
		}

		ctx.Tweets = pagedTweets
		ctx.Pager = pager

		s.render("profile", w, ctx)
	}
}

// TwtxtHandler ...
func (s *Server) TwtxtHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		nick := NormalizeUsername(p.ByName("nick"))
		if nick == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		path, err := securejoin.SecureJoin(filepath.Join(s.config.Data, "feeds"), nick)
		if err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		stat, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "Feed Not Found", http.StatusNotFound)
				return
			}

			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if r.Method == http.MethodHead {
			defer r.Body.Close()
			w.Header().Set("Content-Type", "text/plain")
			w.Header().Set(
				"Content-Length",
				fmt.Sprintf("%d", stat.Size()),
			)
			w.Header().Set(
				"Last-Modified",
				stat.ModTime().UTC().Format(http.TimeFormat),
			)
		} else if r.Method == http.MethodGet {
			followerClient, err := DetectFollowerFromUserAgent(r.UserAgent())
			if err != nil {
				log.WithError(err).Warnf("unable to detect twtxt client from %s", FormatRequest(r))
			} else {
				user, err := s.db.GetUser(nick)
				if err != nil {
					log.WithError(err).Warnf("error loading user object for %s", nick)
				} else {
					if !user.FollowedBy(followerClient.URL) {
						if err := AppendSpecial(
							s.config.Data,
							"twtxt",
							fmt.Sprintf(
								"FOLLOW: @<%s %s> from @<%s %s> using %s/%s",
								nick, URLForUser(s.config.BaseURL, nick, true),
								followerClient.Nick, followerClient.URL,
								followerClient.ClientName, followerClient.ClientVersion,
							),
						); err != nil {
							log.WithError(err).Warnf("error appending special FOLLOW post")
						}
						if user.Followers == nil {
							user.Followers = make(map[string]string)
						}
						user.Followers[followerClient.Nick] = followerClient.URL
						if err := s.db.SetUser(nick, user); err != nil {
							log.WithError(err).Warnf("error updating user object for %s", nick)
						}
					}
				}
			}
			http.ServeFile(w, r, path)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	}
}

// PostHandler ...
func (s *Server) PostHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		postas := strings.ToLower(strings.TrimSpace(r.FormValue("postas")))

		text := CleanTweet(r.FormValue("text"))
		if text == "" {
			ctx.Error = true
			ctx.Message = "No post content provided!"
			s.render("error", w, ctx)
			return
		}

		user, err := s.db.GetUser(ctx.Username)
		if err != nil {
			log.WithError(err).Errorf("error loading user object for %s", ctx.Username)
			ctx.Error = true
			ctx.Message = "Error posting tweet"
			s.render("error", w, ctx)
			return
		}

		switch postas {
		case "", "me":
			err = AppendTweet(s.config.Data, text, user)
		default:
			if user.OwnsFeed(postas) {
				err = AppendSpecial(s.config.Data, postas, text)
			} else {
				err = ErrFeedImposter
			}
		}

		if err != nil {
			ctx.Error = true
			ctx.Message = "Error posting tweet"
			s.render("error", w, ctx)
			return
		}

		// Update user's own timeline with their own new post.
		sources := map[string]string{
			user.Username: user.URL,
		}

		if err := func() error {
			cache, err := LoadCache(s.config.Data)
			if err != nil {
				log.WithError(err).Warn("error loading feed cache")
				return err
			}

			cache.FetchTweets(s.config, sources)

			if err := cache.Store(s.config.Data); err != nil {
				log.WithError(err).Warn("error saving feed cache")
				return err
			}
			return nil
		}(); err != nil {
			log.WithError(err).Error("error updating feed cache")
			ctx.Error = true
			ctx.Message = "Error updating feed cache and timeline"
			s.render("error", w, ctx)
			return
		}

		http.Redirect(w, r, "/", http.StatusFound)
	}
}

// TimelineHandler ...
func (s *Server) TimelineHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		if r.Method == http.MethodHead {
			defer r.Body.Close()

			cacheLastModified, err := CacheLastModified(s.config.Data)
			if err != nil {
				log.WithError(err).Error("CacheLastModified() error")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			w.Header().Set(
				"Last-Modified",
				cacheLastModified.UTC().Format(http.TimeFormat),
			)
			return
		}

		ctx := NewContext(s.config, s.db, r)

		var (
			tweets Tweets
			cache  Cache
			err    error
		)

		if !ctx.Authenticated {
			tweets, err = GetAllTweets(s.config)
		} else {
			cache, err = LoadCache(s.config.Data)
			if err == nil {
				user := ctx.User
				if user != nil {
					for _, url := range user.Following {
						tweets = append(tweets, cache.GetByURL(url)...)
					}
				}
			}
		}

		if err != nil {
			ctx.Error = true
			ctx.Message = "An error occurred while loading the  timeline"
			s.render("error", w, ctx)
			return
		}

		sort.Sort(sort.Reverse(tweets))

		var pagedTweets Tweets

		page := SafeParseInt(r.FormValue("page"), 1)
		pager := paginator.New(adapter.NewSliceAdapter(tweets), s.config.TweetsPerPage)
		pager.SetPage(page)

		if err = pager.Results(&pagedTweets); err != nil {
			ctx.Error = true
			ctx.Message = "An error occurred while loading the  timeline"
			s.render("error", w, ctx)
			return
		}

		ctx.Tweets = pagedTweets
		ctx.Pager = pager

		s.render("timeline", w, ctx)
	}
}

// DiscoverHandler ...
func (s *Server) DiscoverHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		tweets, err := GetAllTweets(s.config)
		if err != nil {
			ctx.Error = true
			ctx.Message = "An error occurred while loading the timeline"
			s.render("error", w, ctx)
			return
		}

		sort.Sort(sort.Reverse(tweets))

		var pagedTweets Tweets

		page := SafeParseInt(r.FormValue("page"), 1)
		pager := paginator.New(adapter.NewSliceAdapter(tweets), s.config.TweetsPerPage)
		pager.SetPage(page)

		if err = pager.Results(&pagedTweets); err != nil {
			ctx.Error = true
			ctx.Message = "An error occurred while loading the  timeline"
			s.render("error", w, ctx)
			return
		}

		ctx.Tweets = pagedTweets
		ctx.Pager = pager

		s.render("timeline", w, ctx)
	}
}

// FeedHandler ...
func (s *Server) FeedHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		name := NormalizeFeedName(r.FormValue("name"))

		if err := ValidateFeedName(s.config.Data, name); err != nil {
			ctx.Error = true
			ctx.Message = fmt.Sprintf("Invalid feed name: %s", err.Error())
			s.render("error", w, ctx)
			return
		}

		if err := ctx.User.CreateFeed(s.config.Data, name); err != nil {
			ctx.Error = true
			ctx.Message = fmt.Sprintf("Error creating: %s", err.Error())
			s.render("error", w, ctx)
			return
		}

		ctx.User.Follow(name, URLForUser(s.config.BaseURL, name))

		if err := s.db.SetUser(ctx.Username, ctx.User); err != nil {
			ctx.Error = true
			ctx.Message = fmt.Sprintf("Error creating feed: %s", err.Error())
			s.render("error", w, ctx)
			return
		}

		if err := AppendSpecial(
			s.config.Data,
			"twtxt",
			fmt.Sprintf(
				"FEED: %s from @<%s %s>",
				name, ctx.User.Username, URLForUser(s.config.BaseURL, ctx.User.Username),
			),
		); err != nil {
			log.WithError(err).Warnf("error appending special FOLLOW post")
		}

		ctx.Error = false
		ctx.Message = fmt.Sprintf("Successfully created feed: %s", name)
		s.render("error", w, ctx)
		return
	}
}

// FeedsHandler ...
func (s *Server) FeedsHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		feeds, err := LoadFeeds(s.config.Data)
		if err != nil {
			ctx.Error = true
			ctx.Message = "An error occurred while loading feed "
			s.render("error", w, ctx)
			return
		}

		ctx.Feeds = feeds

		s.render("feeds", w, ctx)
	}
}

// LoginHandler ...
func (s *Server) LoginHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		if r.Method == "GET" {
			s.render("login", w, ctx)
			return
		}

		username := NormalizeUsername(r.FormValue("username"))
		password := r.FormValue("password")

		// Error: no username or password provided
		if username == "" || password == "" {
			log.Warn("no username or password provided")
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Lookup user
		user, err := s.db.GetUser(username)
		if err != nil {
			ctx.Error = true
			ctx.Message = "Invalid username! Hint: Register an account?"
			s.render("error", w, ctx)
			return
		}

		// Validate cleartext password against KDF hash
		err = s.pm.Check(user.Password, password)
		if err != nil {
			ctx.Error = true
			ctx.Message = "Invalid password! Hint: Reset your password?"
			s.render("error", w, ctx)
			return
		}

		// Login successful
		log.Infof("login successful: %s", username)

		// Lookup session
		sess := r.Context().Value("sesssion")
		if sess == nil {
			log.Warn("no session found")
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Authorize session
		sess.(*session.Session).Set("username", username)

		http.Redirect(w, r, "/", http.StatusFound)
	}
}

// LogoutHandler ...
func (s *Server) LogoutHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		s.sm.Delete(w, r)
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

// RegisterHandler ...
func (s *Server) RegisterHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		if r.Method == "GET" {
			if s.config.Register {
				s.render("register", w, ctx)
			} else {
				message := s.config.RegisterMessage

				if message == "" {
					message = "Registrations are disabled on this instance. Please contact the operator."
				}

				ctx.Error = true
				ctx.Message = message
				s.render("error", w, ctx)
			}

			return
		}

		username := NormalizeUsername(r.FormValue("username"))
		password := r.FormValue("password")
		email := r.FormValue("email")

		if err := ValidateUsername(username); err != nil {
			ctx.Error = true
			ctx.Message = fmt.Sprintf("Username validation failed: %s", err.Error())
			s.render("error", w, ctx)
			return
		}

		if _, err := s.db.GetUser(username); err == nil {
			ctx.Error = true
			ctx.Message = "User with that username already exists! Please pick another!"
			s.render("error", w, ctx)
			return
		}

		fn := filepath.Join(s.config.Data, feedsDir, username)
		if _, err := os.Stat(fn); err == nil {
			ctx.Error = true
			ctx.Message = "Deleted user with that username already exists! Please pick another!"
			s.render("error", w, ctx)
			return
		}

		if err := ioutil.WriteFile(fn, []byte{}, 0644); err != nil {
			log.WithError(err).Error("error creating new user feed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		hash, err := s.pm.NewPassword(password)
		if err != nil {
			log.WithError(err).Error("error creating password hash")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		user := &User{
			Username:  username,
			Email:     email,
			Password:  hash,
			CreatedAt: time.Now(),

			URL: URLForUser(s.config.BaseURL, username, true),
		}

		if err := s.db.SetUser(username, user); err != nil {
			log.WithError(err).Error("error saving user object for new user")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Infof("user registered: %v", user)
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

// FollowHandler ...
func (s *Server) FollowHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		nick := strings.TrimSpace(r.FormValue("nick"))
		url := NormalizeURL(r.FormValue("url"))

		if r.Method == "GET" && nick == "" && url == "" {
			s.render("follow", w, ctx)
			return
		}

		if nick == "" || url == "" {
			ctx.Error = true
			ctx.Message = "Both nick and url must be specified"
			s.render("error", w, ctx)
			return
		}

		user := ctx.User
		if user == nil {
			log.Fatalf("user not found in context")
		}

		user.Following[nick] = url

		if err := s.db.SetUser(ctx.Username, user); err != nil {
			ctx.Error = true
			ctx.Message = fmt.Sprintf("Error following feed %s: %s", nick, url)
			s.render("error", w, ctx)
			return
		}

		if strings.HasPrefix(url, s.config.BaseURL) {
			followee, err := s.db.GetUser(NormalizeUsername(filepath.Base(url)))
			if err != nil {
				log.WithError(err).Warnf("error loading user object for followee %s", NormalizeUsername(filepath.Base(url)))
			} else {
				if followee.Followers == nil {
					followee.Followers = make(map[string]string)
				}
				followee.Followers[user.Username] = user.URL
				if err := s.db.SetUser(followee.Username, followee); err != nil {
					log.WithError(err).Warnf("error updating user object for followee %s", followee.Username)
				}
				if err := AppendSpecial(
					s.config.Data,
					"twtxt",
					fmt.Sprintf(
						"FOLLOW: @<%s %s> from @<%s %s> using %s/%s",
						followee.Username, URLForUser(s.config.BaseURL, followee.Username),
						user.Username, URLForUser(s.config.BaseURL, user.Username),
						"twtxt", FullVersion(),
					),
				); err != nil {
					log.WithError(err).Warnf("error appending special FOLLOW post")
				}
			}
		}

		ctx.Error = false
		ctx.Message = fmt.Sprintf("Successfully started following %s: %s", nick, url)
		s.render("error", w, ctx)
		return
	}
}

// ImportHandler ...
func (s *Server) ImportHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		if r.Method == "GET" {
			s.render("import", w, ctx)
			return
		}

		feeds := r.FormValue("feeds")

		if feeds == "" {
			ctx.Error = true
			ctx.Message = "Nothing to import!"
			s.render("error", w, ctx)
			return
		}

		user := ctx.User
		if user == nil {
			log.Fatalf("user not found in context")
		}

		re := regexp.MustCompile(`(?P<nick>.*?)[: ](?P<url>.*)`)

		imported := 0

		scanner := bufio.NewScanner(strings.NewReader(feeds))
		for scanner.Scan() {
			line := scanner.Text()
			matches := re.FindStringSubmatch(line)
			if len(matches) == 3 {
				nick := strings.TrimSpace(matches[1])
				url := NormalizeURL(strings.TrimSpace(matches[2]))
				if nick != "" && url != "" {
					user.Following[nick] = url
					imported++
				}
			}
		}
		if err := scanner.Err(); err != nil {
			log.WithError(err).Error("error scanning feeds for import")
			ctx.Error = true
			ctx.Message = "Error importing feeds"
			s.render("error", w, ctx)
		}

		if err := s.db.SetUser(ctx.Username, user); err != nil {
			ctx.Error = true
			ctx.Message = "Error importing feeds"
			s.render("error", w, ctx)
			return
		}

		ctx.Error = false
		ctx.Message = fmt.Sprintf("Successfully imported %d feeds", imported)
		s.render("error", w, ctx)
		return
	}
}

// UnfollowHandler ...
func (s *Server) UnfollowHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		nick := strings.TrimSpace(r.FormValue("nick"))

		if nick == "" {
			ctx.Error = true
			ctx.Message = "No nick specified to unfollow"
			s.render("error", w, ctx)
			return
		}

		user := ctx.User
		if user == nil {
			log.Fatalf("user not found in context")
		}

		url, ok := user.Following[nick]
		if !ok {
			ctx.Error = true
			ctx.Message = fmt.Sprintf("No feed found by the nick %s", nick)
			s.render("error", w, ctx)
			return
		}

		delete(user.Following, nick)

		if err := s.db.SetUser(ctx.Username, user); err != nil {
			ctx.Error = true
			ctx.Message = fmt.Sprintf("Error unfollowing feed %s: %s", nick, url)
			s.render("error", w, ctx)
			return
		}

		if strings.HasPrefix(url, s.config.BaseURL) {
			followee, err := s.db.GetUser(NormalizeUsername(filepath.Base(url)))
			if err != nil {
				log.WithError(err).Warnf("error loading user object for followee %s", NormalizeUsername(filepath.Base(url)))
			} else {
				if followee.Followers != nil {
					delete(followee.Followers, user.Username)
					if err := s.db.SetUser(followee.Username, followee); err != nil {
						log.WithError(err).Warnf("error updating user object for followee %s", followee.Username)
					}
				}
				if err := AppendSpecial(
					s.config.Data,
					"twtxt",
					fmt.Sprintf(
						"UNFOLLOW: @<%s %s> from @<%s %s> using %s/%s",
						followee.Username, URLForUser(s.config.BaseURL, followee.Username),
						user.Username, URLForUser(s.config.BaseURL, user.Username),
						"twtxt", FullVersion(),
					),
				); err != nil {
					log.WithError(err).Warnf("error appending special FOLLOW post")
				}
			}
		}

		ctx.Error = false
		ctx.Message = fmt.Sprintf("Successfully stopped following %s: %s", nick, url)
		s.render("error", w, ctx)
		return
	}
}

// SettingsHandler ...
func (s *Server) SettingsHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		if r.Method == "GET" {
			s.render("settings", w, ctx)
			return
		}

		password := r.FormValue("password")

		user := ctx.User
		if user == nil {
			log.Fatalf("user not found in context")
		}

		if password != "" {
			hash, err := s.pm.NewPassword(password)
			if err != nil {
				log.WithError(err).Error("error creating password hash")
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			user.Password = hash
		}

		if err := s.db.SetUser(ctx.Username, user); err != nil {
			ctx.Error = true
			ctx.Message = "Error updating user"
			s.render("error", w, ctx)
			return
		}

		ctx.Error = false
		ctx.Message = "Successfully updated settings"
		s.render("error", w, ctx)
		return
	}
}

// DeleteHandler ...
func (s *Server) DeleteHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		user := ctx.User
		if user == nil {
			log.Fatalf("user not found in context")
		}

		if err := s.db.DelUser(ctx.Username); err != nil {
			ctx.Error = true
			ctx.Message = "Error deleting account"
			s.render("error", w, ctx)
			return
		}

		s.sm.Delete(w, r)
		ctx.Authenticated = false

		ctx.Error = false
		ctx.Message = "Successfully deleted account"
		s.render("error", w, ctx)
		return
	}
}
