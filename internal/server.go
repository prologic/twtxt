package internal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	rice "github.com/GeertJohan/go.rice"
	"github.com/NYTimes/gziphandler"
	"github.com/andyleap/microformats"
	humanize "github.com/dustin/go-humanize"
	"github.com/prologic/observe"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
	"github.com/unrolled/logger"

	"github.com/prologic/twtxt"
	"github.com/prologic/twtxt/internal/auth"
	"github.com/prologic/twtxt/internal/passwords"
	"github.com/prologic/twtxt/internal/session"
	"github.com/prologic/twtxt/internal/webmention"
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
	config    *config
	templates *Templates
	router    *Router
	server    *http.Server

	// Blogs Cache
	blogs *BlogsCache

	// Feed Cache
	cache *Cache

	// Feed Archiver
	archive Archiver

	// Data Store
	db Store

	// Scheduler
	cron *cron.Cron

	// Auth
	am *auth.Manager

	// Sessions
	sc *SessionStore
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

func (s *Server) setupMetrics() {
	ctime := time.Now()

	// server uptime counter
	metrics.NewCounterFunc(
		"server", "uptime",
		"Number of nanoseconds the server has been running",
		func() float64 {
			return float64(time.Since(ctime).Nanoseconds())
		},
	)

	// sessions
	metrics.NewGaugeFunc(
		"server", "sessions",
		"Number of active in-memory sessions (non-persistent)",
		func() float64 {
			return float64(s.sc.Count())
		},
	)

	// database keys
	metrics.NewGaugeFunc(
		"db", "feeds",
		"Number of database /feeds keys",
		func() float64 {
			return float64(s.db.LenFeeds())
		},
	)
	metrics.NewGaugeFunc(
		"db", "sessions",
		"Number of database /sessions keys",
		func() float64 {
			return float64(s.db.LenSessions())
		},
	)
	metrics.NewGaugeFunc(
		"db", "users",
		"Number of database /users keys",
		func() float64 {
			return float64(s.db.LenUsers())
		},
	)
	metrics.NewGaugeFunc(
		"db", "tokens",
		"Number of database /tokens keys",
		func() float64 {
			return float64(s.db.LenTokens())
		},
	)

	// feed cache sources
	metrics.NewGauge(
		"cache", "sources",
		"Number of feed sources being fetched by the global feed cache",
	)

	// feed cache size
	metrics.NewGauge(
		"cache", "feeds",
		"Number of unique feeds in the global feed cache",
	)

	// feed cache size
	metrics.NewGauge(
		"cache", "twts",
		"Number of active twts in the global feed cache",
	)

	// blogs cache size
	metrics.NewGaugeFunc(
		"cache", "blogs",
		"Number of blogs in the blogs cache",
		func() float64 {
			return float64(s.blogs.Count())
		},
	)

	// feed cache processing time
	metrics.NewGauge(
		"cache", "last_processed_seconds",
		"Number of seconds for a feed cache cycle",
	)

	// archive size
	metrics.NewCounter(
		"archive", "size",
		"Number of items inserted into the global feed archive",
	)

	// server info
	metrics.NewGaugeVec(
		"server", "info",
		"Server information",
		[]string{"full_version", "version", "commit"},
	)
	metrics.GaugeVec("server", "info").
		With(map[string]string{
			"full_version": twtxt.FullVersion(),
			"version":      twtxt.Version,
			"commit":       twtxt.Commit,
		}).Set(1)

	// old avatars
	metrics.NewCounter(
		"media", "old_avatar",
		"Count of old Avtar (PNG) conversions",
	)
	// old media
	metrics.NewCounter(
		"media", "old_media",
		"Count of old Media (PNG) served",
	)

	s.AddRoute("GET", "/metrics", metrics.Handler())
}

func (s *Server) processWebMention(source, target *url.URL, sourceData *microformats.Data) error {
	log.
		WithField("source", source).
		WithField("target", target).
		Infof("received webmention from %s to %s", source.String(), target.String())

	getEntry := func(data *microformats.Data) (*microformats.MicroFormat, error) {
		if data != nil {
			for _, item := range sourceData.Items {
				if HasString(item.Type, "h-entry") {
					return item, nil
				}
			}
		}
		return nil, errors.New("error: no entry found")
	}

	getAuthor := func(entry *microformats.MicroFormat) (*microformats.MicroFormat, error) {
		if entry != nil {
			authors := entry.Properties["author"]
			if len(authors) > 0 {
				if v, ok := authors[0].(*microformats.MicroFormat); ok {
					return v, nil
				}
			}
		}
		return nil, errors.New("error: no author found")
	}

	parseSourceData := func(data *microformats.Data) (string, string, error) {
		if data == nil {
			log.Warn("no source data to parse")
			return "", "", nil
		}

		entry, err := getEntry(data)
		if err != nil {
			log.WithError(err).Error("error getting entry")
			return "", "", err
		}

		author, err := getAuthor(entry)
		if err != nil {
			log.WithError(err).Error("error getting author")
			return "", "", err
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

		return authorName, sourceFeed, nil
	}

	user, err := GetUserFromURL(s.config, s.db, target.String())
	if err != nil {
		log.WithError(err).WithField("target", target.String()).Warn("unable to get used from webmention target")
		return err
	}

	authorName, sourceFeed, err := parseSourceData(sourceData)
	if err != nil {
		log.WithError(err).Warnf("error parsing mf2 source data from %s", source)
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
				"WEBMENTION: @<%s %s> on %s",
				user.Username, user.URL,
				source.String(),
			),
		); err != nil {
			log.WithError(err).Warnf("error appending special MENTION post")
			return err
		}
	}

	return nil
}

func (s *Server) setupWebMentions() {
	webmentions = webmention.New()
	webmentions.Mention = s.processWebMention
}

func (s *Server) setupCronJobs() error {
	for name, jobSpec := range Jobs {
		job := jobSpec.Factory(s.config, s.blogs, s.cache, s.archive, s.db)
		if err := s.cron.AddJob(jobSpec.Schedule, job); err != nil {
			return err
		}
		log.Infof("Started background job %s (%s)", name, jobSpec.Schedule)
	}

	return nil
}

func (s *Server) runStartupJobs() {
	time.Sleep(time.Second * 5)

	log.Info("running startup jobs")
	for name, jobSpec := range StartupJobs {
		job := jobSpec.Factory(s.config, s.blogs, s.cache, s.archive, s.db)
		log.Infof("running %s now...", name)
		job.Run()
	}

	// Merge store
	if err := s.db.Merge(); err != nil {
		log.WithError(err).Error("error merging store")
	}
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

	s.router.GET("/robots.txt", s.RobotsHandler())
	s.router.HEAD("/robots.txt", s.RobotsHandler())

	s.router.GET("/discover", s.am.MustAuth(s.DiscoverHandler()))
	s.router.GET("/mentions", s.am.MustAuth(s.MentionsHandler()))
	s.router.GET("/search", s.SearchHandler())

	s.router.HEAD("/twt/:hash", s.PermalinkHandler())
	s.router.GET("/twt/:hash", s.PermalinkHandler())

	s.router.HEAD("/conv/:hash", s.ConversationHandler())
	s.router.GET("/conv/:hash", s.ConversationHandler())

	s.router.GET("/feeds", s.am.MustAuth(s.FeedsHandler()))
	s.router.POST("/feed", s.am.MustAuth(s.FeedHandler()))

	s.router.POST("/post", s.am.MustAuth(s.PostHandler()))
	s.router.PATCH("/post", s.am.MustAuth(s.PostHandler()))
	s.router.DELETE("/post", s.am.MustAuth(s.PostHandler()))

	s.router.POST("/blog", s.am.MustAuth(s.PublishBlogHandler()))
	s.router.GET("/blogs/:author", s.BlogsHandler())
	s.router.GET("/blog/:author/:year/:month/:date/:slug", s.BlogHandler())
	s.router.HEAD("/blog/:author/:year/:month/:date/:slug", s.BlogHandler())
	s.router.GET("/blog/:author/:year/:month/:date/:slug/edit", s.EditBlogHandler())
	s.router.GET("/blog/:author/:year/:month/:date/:slug/delete", s.DeleteBlogHandler())

	// Redirect old URIs (twtxt <= v0.0.8) of the form /u/<nick> -> /user/<nick>/twtxt.txt
	// TODO: Remove this after v1
	s.router.GET("/u/:nick", s.OldTwtxtHandler())
	s.router.HEAD("/u/:nick", s.OldTwtxtHandler())

	// Redirect old URIs (twtxt <= v0.1.0) of the form /user/<nick>/avatar.png -> /user/<nick>/avatar
	// TODO: Remove this after v1
	s.router.GET("/user/:nick/avatar.png", s.OldAvatarHandler())
	s.router.HEAD("/user/:nick/avatar.png", s.OldAvatarHandler())

	if s.config.OpenProfiles {
		s.router.GET("/user/:nick", s.ProfileHandler())
		s.router.GET("/user/:nick/config.yaml", s.UserConfigHandler())
	} else {
		s.router.GET("/user/:nick", s.am.MustAuth(s.ProfileHandler()))
		s.router.GET("/user/:nick/config.yaml", s.am.MustAuth(s.UserConfigHandler()))
	}
	s.router.GET("/user/:nick/avatar", s.AvatarHandler())
	s.router.HEAD("/user/:nick/avatar", s.AvatarHandler())
	s.router.HEAD("/user/:nick/twtxt.txt", s.TwtxtHandler())
	s.router.GET("/user/:nick/twtxt.txt", s.TwtxtHandler())
	s.router.GET("/user/:nick/followers", s.FollowersHandler())
	s.router.GET("/user/:nick/following", s.FollowingHandler())

	// WebMentions
	s.router.POST("/user/:nick/webmention", s.WebMentionHandler())

	// External Feeds
	s.router.GET("/external/:slug/:nick", s.ExternalHandler())
	s.router.GET("/external/:slug/:nick/avatar", s.ExternalAvatarHandler())
	s.router.HEAD("/external/:slug/:nick/avatar", s.ExternalAvatarHandler())

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

	s.router.GET("/config", s.ConfigHandler())

	// Reset Password
	s.router.GET("/resetPassword", s.ResetPasswordHandler())
	s.router.POST("/resetPassword", s.ResetPasswordHandler())
	s.router.GET("/newPassword", s.ResetPasswordMagicLinkHandler())
	s.router.POST("/newPassword", s.NewPasswordHandler())

	// Media Handling
	s.router.GET("/media/:name", s.MediaHandler())
	s.router.HEAD("/media/:name", s.MediaHandler())
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

	s.router.POST("/token/delete/:signature", s.am.MustAuth(s.DeleteTokenHandler()))

	s.router.GET("/deleteFeeds", s.DeleteAccountHandler())
	s.router.POST("/delete", s.am.MustAuth(s.DeleteAllHandler()))
}

// NewServer ...
func NewServer(bind string, options ...Option) (*Server, error) {
	config := NewConfig()

	for _, opt := range options {
		if err := opt(config); err != nil {
			return nil, err
		}
	}

	blogs, err := LoadBlogsCache(config.data)
	if err != nil {
		log.WithError(err).Error("error loading blogs cache (re-creating)")
		blogs = NewBlogsCache()
		log.Info("updating blogs cache")
		blogs.UpdateBlogs(config)
	}
	if len(blogs.Blogs) == 0 {
		log.Info("empty blogs cache, updating...")
		blogs.UpdateBlogs(config)
	}

	cache, err := LoadCache(config.data)
	if err != nil {
		log.WithError(err).Error("error loading feed cache")
		return nil, err
	}

	archive, err := NewDiskArchiver(filepath.Join(config.data, archiveDir))
	if err != nil {
		log.WithError(err).Error("error creating feed archiver")
		return nil, err
	}

	db, err := NewStore(config.store)
	if err != nil {
		log.WithError(err).Error("error creating store")
		return nil, err
	}

	if err := db.Merge(); err != nil {
		log.WithError(err).Error("error merging store")
		return nil, err
	}

	templates, err := NewTemplates(config, blogs, cache)
	if err != nil {
		log.WithError(err).Error("error loading templates")
		return nil, err
	}

	router := NewRouter()

	am := auth.NewManager(auth.NewOptions("/login", "/register"))

	pm := passwords.NewScryptPasswords(nil)

	sc := NewSessionStore(db, config.sessionCacheTTL)

	sm := session.NewManager(
		session.NewOptions(
			config.PodName,
			config.cookieSecret,
			strings.HasPrefix(config.baseURL, "https"),
			config.sessionExpiry,
		),
		sc,
	)

	api := NewAPI(router, config, cache, archive, db, pm)

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

		// Blogs Cache
		blogs: blogs,

		// Feed Cache
		cache: cache,

		// Feed Archiver
		archive: archive,

		// Data Store
		db: db,

		// Schedular
		cron: cron.New(),

		// Auth Manager
		am: am,

		// Session Manager
		sc: sc,
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

	server.setupWebMentions()
	log.Infof("started webmentions processor")

	server.setupMetrics()
	log.Infof("serving metrics endpoint at %s/metrics", server.config.BaseURL)

	// Log interesting configuration options
	log.Infof("Instance Name: %s", server.config.PodName)
	log.Infof("Base URL: %s", server.config.baseURL)
	log.Infof("Admin User: %s", server.config.adminUser)
	log.Infof("Admin Name: %s", server.config.adminName)
	log.Infof("Admin Email: %s", server.config.adminEmail)
	log.Infof("Max Twts per Page: %d", server.config.twtsPerPage)
	log.Infof("Max Cache TTL: %s", server.config.MaxCacheTTL)
	log.Infof("Max Cache Items: %d", server.config.MaxCacheItems)
	log.Infof("Maximum length of Posts: %d", server.config.MaxTwtLength)
	log.Infof("Open User Profiles: %t", server.config.OpenProfiles)
	log.Infof("Open Registrations: %t", server.config.OpenRegistrations)
	log.Infof("SMTP Host: %s", server.config.smtpHost)
	log.Infof("SMTP Port: %d", server.config.smtpPort)
	log.Infof("SMTP User: %s", server.config.smtpUser)
	log.Infof("SMTP From: %s", server.config.smtpFrom)
	log.Infof("Max Fetch Limit: %s", humanize.Bytes(uint64(server.config.MaxFetchLimit)))
	log.Infof("Max Upload Size: %s", humanize.Bytes(uint64(server.config.MaxUploadSize)))
	log.Infof("API Session Time: %s", server.config.apiSessionTime)

	// Warn about user registration being disabled.
	if !server.config.OpenRegistrations {
		log.Warn("Open Registrations are disabled as per configuration (no -R/--open-registrations)")
	}

	server.initRoutes()
	api.initRoutes()

	go server.runStartupJobs()

	return server, nil
}
