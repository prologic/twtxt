package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	rice "github.com/GeertJohan/go.rice"
	"github.com/NYTimes/gziphandler"
	"github.com/andyleap/microformats"
	humanize "github.com/dustin/go-humanize"
	"github.com/prologic/observe"
	"github.com/prologic/webmention"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
	"github.com/unrolled/logger"

	"github.com/prologic/twtxt/internal/auth"
	"github.com/prologic/twtxt/internal/passwords"
	"github.com/prologic/twtxt/internal/session"
)

var (
	metrics     *observe.Metrics
	webmentions *webmention.WebMention
)

func init() {
	metrics = observe.NewMetrics("twtd")
}

// Server ...
type Server struct {
	bind      string
	config    *Config
	templates *Templates
	router    *Router
	server    *http.Server

	// Feed Cache
	cache Cache

	// Data Store
	db Store

	// Scheduler
	cron *cron.Cron

	// Auth
	am *auth.Manager

	// Sessions
	sm *session.Manager

	// API
	api *API

	// Passwords
	pm passwords.Passwords
}

func (s *Server) render(name string, w http.ResponseWriter, ctx *Context) {
	buf, err := s.templates.Exec(name, ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = buf.WriteTo(w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// AddRouter ...
func (s *Server) AddRoute(method, path string, handler http.Handler) {
	s.router.Handler(method, path, handler)
}

// AddShutdownHook ...
func (s *Server) AddShutdownHook(f func()) {
	s.server.RegisterOnShutdown(f)
}

// Shutdown ...
func (s *Server) Shutdown(ctx context.Context) error {
	s.cron.Stop()

	if err := s.server.Shutdown(ctx); err != nil {
		log.WithError(err).Error("error shutting down server")
		return err
	}

	if err := s.db.Close(); err != nil {
		log.WithError(err).Error("error closing store")
		return err
	}

	return nil
}

// Run ...
func (s *Server) Run() (err error) {
	idleConnsClosed := make(chan struct{})
	go func() {
		sigch := make(chan os.Signal, 1)
		signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigch
		log.Infof("Recieved signal %s", sig)

		log.Info("Shutting down...")

		// We received an interrupt signal, shut down.
		if err = s.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			log.WithError(err).Fatal("Error shutting down HTTP server")
		}
		close(idleConnsClosed)
	}()

	if err = s.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		log.WithError(err).Fatal("HTTP server ListenAndServe")
	}

	<-idleConnsClosed

	return
}

// ListenAndServe ...
func (s *Server) ListenAndServe() error {
	return s.server.ListenAndServe()
}

// AddCronJob ...
func (s *Server) AddCronJob(spec string, job cron.Job) error {
	return s.cron.AddJob(spec, job)
}

func (s *Server) setupMetrics() error {
	ctime := time.Now()

	// server uptime counter
	metrics.NewCounterFunc(
		"server", "uptime",
		"Number of nanoseconds the server has been running",
		func() float64 {
			return float64(time.Since(ctime).Nanoseconds())
		},
	)

	// feed cache size
	metrics.NewCounterFunc(
		"server", "feed_cache_size",
		"Number of items in the global feed cache",
		func() float64 {
			return float64(len(s.cache.GetAll()))
		},
	)

	// feed cache processing time
	metrics.NewGauge(
		"server", "feed_cache_last_processing_time_seconds",
		"Number of seconds for a feed cache cycle",
	)

	s.AddRoute("GET", "/metrics", metrics.Handler())

	return nil
}

func (s *Server) processWebMention(source, target *url.URL, sourceData *microformats.Data) error {
	log.
		WithField("source", source).
		WithField("target", target).
		Infof("received webmention from %s to %s", source.String(), target.String())

	var hEntry *microformats.MicroFormat

	for _, item := range sourceData.Items {
		if HasString(item.Type, "h-entry") {
			hEntry = item
		}
	}

	var author *microformats.MicroFormat

	authors := hEntry.Properties["author"]
	if len(authors) > 0 {
		if v, ok := authors[0].(*microformats.MicroFormat); ok {
			author = v
		}
	}

	var authorName string

	if author != nil {
		authorName = strings.TrimSpace(author.Value)
	}

	var sourceFeed string

	for _, alternate := range sourceData.Alternates {
		if alternate.Type == "text/plain" {
			sourceFeed = alternate.URL
		}
	}

	user, err := GetUserFromURL(s.config, s.db, target.String())
	if err != nil {
		log.WithError(err).WithField("target", target.String()).Warn("unable to get used from webmention target")
		return err
	}

	if authorName != "" && sourceFeed != "" {
		if _, err := AppendSpecial(
			s.config, s.db,
			twtxtBot,
			fmt.Sprintf(
				"MENTION: @<%s %s> from @<%s %s> on %s",
				user.Username, user.URL, authorName, sourceFeed,
				source.String(),
			),
		); err != nil {
			log.WithError(err).Warnf("error appending special MENTION post")
			return err
		}

	} else {
		if _, err := AppendSpecial(
			s.config, s.db,
			twtxtBot,
			fmt.Sprintf(
				"WEBMENTION: @<%s %s> from %s on %s",
				user.Username, user.URL,
				source.String(), target.String(),
			),
		); err != nil {
			log.WithError(err).Warnf("error appending special MENTION post")
			return err
		}
	}

	return nil
}

func (s *Server) setupWebMentions() error {
	webmentions = webmention.New()
	webmentions.Mention = s.processWebMention

	return nil
}

func (s *Server) setupCronJobs() error {
	for name, jobSpec := range Jobs {
		job := jobSpec.Factory(s.config, s.cache, s.db)
		if err := s.cron.AddJob(jobSpec.Schedule, job); err != nil {
			return err
		}
		log.Infof("Started background job %s (%s)", name, jobSpec.Schedule)
	}

	log.Info("running FixUserAccountsJob now...")
	NewFixUserAccountsJob(s.config, s.cache, s.db).Run()

	return nil
}

func (s *Server) initRoutes() {
	s.router.ServeFilesWithCacheControl(
		"/css/:commit/*filepath",
		rice.MustFindBox("static/css").HTTPBox(),
	)

	s.router.ServeFilesWithCacheControl(
		"/img/:commit/*filepath",
		rice.MustFindBox("static/img").HTTPBox(),
	)

	s.router.ServeFilesWithCacheControl(
		"/js/:commit/*filepath",
		rice.MustFindBox("static/js").HTTPBox(),
	)

	s.router.NotFound = http.HandlerFunc(s.NotFoundHandler)

	s.router.GET("/about", s.PageHandler("about"))
	s.router.GET("/help", s.PageHandler("help"))
	s.router.GET("/privacy", s.PageHandler("privacy"))

	s.router.GET("/support", s.SupportHandler())
	s.router.POST("/support", s.SupportHandler())
	s.router.GET("/_captcha", s.CaptchaHandler())

	s.router.GET("/", s.TimelineHandler())
	s.router.HEAD("/", s.TimelineHandler())

	s.router.GET("/discover", s.am.MustAuth(s.DiscoverHandler()))
	s.router.GET("/mentions", s.am.MustAuth(s.MentionsHandler()))
	s.router.GET("/search", s.SearchHandler())

	s.router.HEAD("/twt/:hash", s.PermalinkHandler())
	s.router.GET("/twt/:hash", s.PermalinkHandler())

	s.router.GET("/feeds", s.am.MustAuth(s.FeedsHandler()))
	s.router.POST("/feed", s.am.MustAuth(s.FeedHandler()))

	s.router.POST("/post", s.am.MustAuth(s.PostHandler()))
	s.router.PATCH("/post", s.am.MustAuth(s.PostHandler()))
	s.router.DELETE("/post", s.am.MustAuth(s.PostHandler()))

	// Redirect old URIs (twtxt <= v0.0.8) of the form /u/<nick> -> /user/<nick>/twtxt.txt
	// TODO: Remove this after v1
	s.router.GET("/u/:nick", s.OldTwtxtHandler())
	s.router.HEAD("/u/:nick", s.OldTwtxtHandler())

	if s.config.OpenProfiles {
		s.router.GET("/user/:nick", s.ProfileHandler())
	} else {
		s.router.GET("/user/:nick", s.am.MustAuth(s.ProfileHandler()))
	}
	s.router.GET("/user/:nick/avatar.png", s.AvatarHandler())
	s.router.HEAD("/user/:nick/twtxt.txt", s.TwtxtHandler())
	s.router.GET("/user/:nick/twtxt.txt", s.TwtxtHandler())
	s.router.GET("/user/:nick/followers", s.FollowersHandler())
	s.router.GET("/user/:nick/following", s.FollowingHandler())

	// WebMentions
	s.router.POST("/user/:nick/webmention", s.WebMentionHandler())

	// External Feeds
	s.router.GET("/external", s.ExternalHandler())

	// Syndication Formats (RSS, Atom, JSON Feed)
	s.router.HEAD("/atom.xml", s.SyndicationHandler())
	s.router.HEAD("/user/:nick/atom.xml", s.SyndicationHandler())
	s.router.GET("/atom.xml", s.SyndicationHandler())
	s.router.GET("/user/:nick/atom.xml", s.SyndicationHandler())

	s.router.GET("/feed/:name/manage", s.am.MustAuth(s.ManageFeedHandler()))
	s.router.POST("/feed/:name/manage", s.am.MustAuth(s.ManageFeedHandler()))
	s.router.POST("/feed/:name/archive", s.am.MustAuth(s.ArchiveFeedHandler()))

	s.router.GET("/login", s.LoginHandler())
	s.router.POST("/login", s.LoginHandler())

	s.router.GET("/logout", s.LogoutHandler())
	s.router.POST("/logout", s.LogoutHandler())

	s.router.GET("/register", s.RegisterHandler())
	s.router.POST("/register", s.RegisterHandler())

	// Reset Password
	s.router.GET("/resetPassword", s.ResetPasswordHandler())
	s.router.POST("/resetPassword", s.ResetPasswordHandler())
	s.router.GET("/newPassword", s.ResetPasswordMagicLinkHandler())
	s.router.POST("/newPassword", s.NewPasswordHandler())

	// Media Handling
	s.router.GET("/media/:name", s.MediaHandler())
	s.router.POST("/upload", s.am.MustAuth(s.UploadMediaHandler()))

	// User/Feed Lookups
	s.router.GET("/lookup", s.am.MustAuth(s.LookupHandler()))

	s.router.GET("/follow", s.am.MustAuth(s.FollowHandler()))
	s.router.POST("/follow", s.am.MustAuth(s.FollowHandler()))

	s.router.GET("/import", s.am.MustAuth(s.ImportHandler()))
	s.router.POST("/import", s.am.MustAuth(s.ImportHandler()))

	s.router.GET("/unfollow", s.am.MustAuth(s.UnfollowHandler()))
	s.router.POST("/unfollow", s.am.MustAuth(s.UnfollowHandler()))

	s.router.GET("/settings", s.am.MustAuth(s.SettingsHandler()))
	s.router.POST("/settings", s.am.MustAuth(s.SettingsHandler()))

	s.router.POST("/delete", s.am.MustAuth(s.DeleteHandler()))

	s.router.GET("/manage", s.am.MustAuth(s.ManageHandler()))
	s.router.POST("/manage", s.am.MustAuth(s.ManageHandler()))
}

// NewServer ...
func NewServer(bind string, options ...Option) (*Server, error) {
	config := NewConfig()

	for _, opt := range options {
		if err := opt(config); err != nil {
			return nil, err
		}
	}

	cache, err := LoadCache(config.Data)
	if err != nil {
		log.WithError(err).Error("error loading feed cache")
		return nil, err
	}

	db, err := NewStore(config.Store)
	if err != nil {
		log.WithError(err).Error("error creating store")
		return nil, err
	}

	templates, err := NewTemplates(config)
	if err != nil {
		log.WithError(err).Error("error loading templates")
		return nil, err
	}

	router := NewRouter()

	am := auth.NewManager(auth.NewOptions("/login", "/register"))

	pm := passwords.NewScryptPasswords(nil)

	sm := session.NewManager(
		session.NewOptions(
			config.Name,
			config.CookieSecret,
			strings.HasPrefix(config.BaseURL, "https"),
			config.SessionExpiry,
		),
		db,
	)

	api := NewAPI(router, config, cache, db, pm)

	server := &Server{
		bind:      bind,
		config:    config,
		router:    router,
		templates: templates,

		server: &http.Server{
			Addr: bind,
			Handler: logger.New(logger.Options{
				Prefix:               "twtxt",
				RemoteAddressHeaders: []string{"X-Forwarded-For"},
			}).Handler(
				gziphandler.GzipHandler(
					sm.Handler(router),
				),
			),
		},

		// API
		api: api,

		// Feed Cache

		cache: cache,

		// Data Store
		db: db,

		// Schedular
		cron: cron.New(),

		// Auth Manager
		am: am,

		// Session Manager
		sm: sm,

		// Password Manager
		pm: pm,
	}

	if err := server.setupCronJobs(); err != nil {
		log.WithError(err).Error("error setting up background jobs")
		return nil, err
	}
	server.cron.Start()
	log.Infof("started background jobs")

	if err := server.setupWebMentions(); err != nil {
		log.WithError(err).Error("error setting up webmentions processor")
		return nil, err
	}
	log.Infof("started webmentions processor")

	if err := server.setupMetrics(); err != nil {
		log.WithError(err).Error("error setting up metrics")
		return nil, err
	}
	log.Infof("serving metrics endpoint at %s/metrics", server.config.BaseURL)

	// Log interesting configuration options
	log.Infof("Instance Name: %s", server.config.Name)
	log.Infof("Base URL: %s", server.config.BaseURL)
	log.Infof("Admin User: %s", server.config.AdminUser)
	log.Infof("Admin Name: %s", server.config.AdminName)
	log.Infof("Admin Email: %s", server.config.AdminEmail)
	log.Infof("Max Twts per Page: %d", server.config.TwtsPerPage)
	log.Infof("Maximum length of Posts: %d", server.config.MaxTwtLength)
	log.Infof("Open User Profiles: %t", server.config.OpenProfiles)
	log.Infof("Open Registrations: %t", server.config.OpenRegistrations)
	log.Infof("SMTP Host: %s", server.config.SMTPHost)
	log.Infof("SMTP Port: %d", server.config.SMTPPort)
	log.Infof("SMTP User: %s", server.config.SMTPUser)
	log.Infof("SMTP From: %s", server.config.SMTPFrom)
	log.Infof("Max Fetch Limit: %s", humanize.Bytes(uint64(server.config.MaxFetchLimit)))
	log.Infof("Max Upload Size: %s", humanize.Bytes(uint64(server.config.MaxUploadSize)))
	log.Infof("API Session Time: %s", (server.config.APISessionTime))

	// Warn about user registration being disabled.
	if !server.config.OpenRegistrations {
		log.Warn("Open Registrations are disabled as per configuration (no -R/--open-registrations)")
	}

	server.initRoutes()
	api.initRoutes()

	return server, nil
}
