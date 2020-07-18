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
		nick := p.ByName("nick")
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
			ctx := &Context{
				Error:   true,
				Message: "No post content provided!",
			}
			s.render("error", w, ctx)
			return
		}

		user, err := s.db.GetUser(ctx.Username)
		if err != nil {
			log.WithError(err).Errorf("error loading user object for %s", ctx.Username)
			ctx := &Context{
				Error:   true,
				Message: "Error posting tweet",
			}
			s.render("error", w, ctx)
			return
		}

		if err := AppendTweet(s.config.Data, text, user); err != nil {
			ctx := &Context{
				Error:   true,
				Message: "Error posting tweet",
			}
			s.render("error", w, ctx)
			return
		}

		http.Redirect(w, r, "/", http.StatusFound)
	}
}

// TimelineHandler ...
func (s *Server) TimelineHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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
			ctx := &Context{
				Error:   true,
				Message: "An error occurred while loading the  timeline",
			}
			s.render("error", w, ctx)
			return
		}

		sort.Sort(sort.Reverse(tweets))

		ctx.Tweets = tweets

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

		username := r.FormValue("username")
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

				ctx := &Context{
					Error:   true,
					Message: message,
				}
				s.render("error", w, ctx)
			}

			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")
		email := r.FormValue("email")

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

		if r.Method == "GET" {
			s.render("follow", w, ctx)
			return
		}

		nick := strings.TrimSpace(r.FormValue("nick"))
		url := strings.TrimSpace(r.FormValue("url"))

		if nick == "" || url == "" {
			ctx := &Context{
				Error:   true,
				Message: "Both nick and url must be specified",
			}
			s.render("error", w, ctx)
			return
		}

		user := ctx.User
		if user == nil {
			log.Fatalf("user not found in context")
		}

		user.Following[nick] = url

		if err := s.db.SetUser(ctx.Username, user); err != nil {
			ctx := &Context{
				Error:   true,
				Message: fmt.Sprintf("Error following feed %s: %s", nick, url),
			}
			s.render("error", w, ctx)
			return
		}

		ctx = &Context{
			Error:   false,
			Message: fmt.Sprintf("Successfully started following %s: %s", nick, url),
		}
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
			ctx := &Context{
				Error:   true,
				Message: "Nothing to import!",
			}
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
			if len(matches) >= 2 {
				nick := matches[0]
				url := NormalizeURL(matches[1])
				if nick != "" || url != "" {
					user.Following[nick] = url
					imported++
				}
			}
		}
		if err := scanner.Err(); err != nil {
			log.WithError(err).Error("error scanning feeds for import")
			ctx := &Context{
				Error:   true,
				Message: "Error importing feeds",
			}
			s.render("error", w, ctx)
		}

		if err := s.db.SetUser(ctx.Username, user); err != nil {
			ctx := &Context{
				Error:   true,
				Message: "Error importing feeds",
			}
			s.render("error", w, ctx)
			return
		}

		ctx = &Context{
			Error:   false,
			Message: fmt.Sprintf("Successfully imported %d feeds", imported),
		}
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
			ctx := &Context{
				Error:   true,
				Message: "No nick specified to unfollow",
			}
			s.render("error", w, ctx)
			return
		}

		user := ctx.User
		if user == nil {
			log.Fatalf("user not found in context")
		}

		url, ok := user.Following[nick]
		if !ok {
			ctx := &Context{
				Error:   true,
				Message: fmt.Sprintf("No feed found by the nick %s", nick),
			}
			s.render("error", w, ctx)
			return
		}

		delete(user.Following, nick)

		if err := s.db.SetUser(ctx.Username, user); err != nil {
			ctx := &Context{
				Error:   true,
				Message: fmt.Sprintf("Error unfollowing feed %s: %s", nick, url),
			}
			s.render("error", w, ctx)
			return
		}

		ctx = &Context{
			Error:   false,
			Message: fmt.Sprintf("Successfully stopped following %s: %s", nick, url),
		}
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
			ctx := &Context{
				Error:   true,
				Message: "Error updating user",
			}
			s.render("error", w, ctx)
			return
		}

		ctx = &Context{
			Error:   false,
			Message: "Successfully updated settings",
		}
		s.render("error", w, ctx)
		return
	}
}
