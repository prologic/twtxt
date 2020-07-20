package twtxt

import (
	"bufio"
	"fmt"
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

		text := r.FormValue("text")
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

		if err := AppendTweet(s.config.Data, text, user); err != nil {
			ctx.Error = true
			ctx.Message = "Error posting tweet"
			s.render("error", w, ctx)
			return
		}

		// Update user's own timeline with their own new post.
		sources := map[string]string{
			"me": user.URL,
		}

		if err := func() error {
			cache, err := LoadCache(s.config.Data)
			if err != nil {
				log.WithError(err).Warn("error loading feed cache")
				return err
			}

			cache.FetchTweets(sources)

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
			log.WithError(err).Errorf("error looking up user %s", username)
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Validate cleartext password against KDF hash
		err = s.pm.Check(user.Password, password)
		if err != nil {
			log.WithError(err).Errorf("password mismatch for %s", username)
			http.Redirect(w, r, "/login", http.StatusFound)
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

		if _, err := s.db.GetUser(username); err == nil {
			ctx.Error = true
			ctx.Message = "User with that username already exists! Please pick another!"
			s.render("error", w, ctx)
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

			URL: fmt.Sprintf(
				"%s/u/%s",
				strings.TrimSuffix(s.config.BaseURL, "/"),
				username,
			),
		}

		s.db.SetUser(username, user)

		log.Infof("user registered: %v", user)
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

// FollowHandler ...
func (s *Server) FollowHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		nick := strings.TrimSpace(r.FormValue("nick"))
		url := strings.TrimSpace(r.FormValue("url"))

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
