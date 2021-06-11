package internal

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/andyleap/microformats"
	humanize "github.com/dustin/go-humanize"
	"github.com/gabstv/merger"
	"github.com/justinas/nosurf"
	"github.com/prologic/observe"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
	"github.com/unrolled/logger"

	"github.com/jointwt/twtxt"
	"github.com/jointwt/twtxt/internal/auth"
	"github.com/jointwt/twtxt/internal/passwords"
	"github.com/jointwt/twtxt/internal/session"
	"github.com/jointwt/twtxt/internal/webmention"
)

var (
	metrics     *observe.Metrics
	webmentions *webmention.WebMention

	//go:embed static/css
	staticCSS embed.FS

	//go:embed static/js
	staticJS embed.FS

	//go:embed static/img
	staticIMG embed.FS
)

func init() {
	metrics = observe.NewMetrics("twtd")
}

// Server ...
type Server struct {
	bind    string
	config  *Config
	tmplman *TemplateManager
	router  *Router
	server  *http.Server

	// Blogs Cache
	blogs *BlogsCache

	// Messages Cache
	msgs *MessagesCache

	// Feed Cache
	cache *Cache

	// Feed Archiver
	archive Archiver

	// Data Store
	db Store

	// Scheduler
	cron *cron.Cron

	// Dispatcher
	tasks *Dispatcher

	// Auth
	am *auth.Manager

	// Sessions
	sc *SessionStore
	sm *session.Manager

	// API
	api *API

	// POP3 Service
	pop3Service *POP3Service

	// SMTP Service
	smtpService *SMTPService

	// Passwords
	pm passwords.Passwords

	// Translator
	translator *Translator
}

func (s *Server) render(name string, w http.ResponseWriter, ctx *Context) {
	if ctx.Authenticated && ctx.Username != "" {
		ctx.NewMessages = s.msgs.Get(ctx.User.Username)
	}

	buf, err := s.tmplman.Exec(name, ctx)
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
	s.tasks.Stop()
	s.smtpService.Stop()

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
		if err = s.ListenAndServe(); err != http.ErrServerClosed {
			// Error starting or closing listener:
			log.WithError(err).Fatal("HTTP server ListenAndServe")
		}
	}()

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigch
	log.Infof("Received signal %s", sig)

	log.Info("Shutting down...")

	// We received an interrupt signal, shut down.
	if err = s.Shutdown(context.Background()); err != nil {
		// Error from closing listeners, or context timeout:
		log.WithError(err).Fatal("Error shutting down HTTP server")
	}
	close(idleConnsClosed)

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

	// feed cache limited fetch (feed exceeded MaxFetchLImit or unknown size)
	metrics.NewCounter(
		"cache", "limited",
		"Number of feed cache fetches affected by MaxFetchLimit",
	)

	// archive size
	metrics.NewCounter(
		"archive", "size",
		"Number of items inserted into the global feed archive",
	)

	// archive errors
	metrics.NewCounter(
		"archive", "error",
		"Number of items errored inserting into the global feed archive",
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
		if jobSpec.Schedule == "" {
			continue
		}

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
	if s.config.Debug {
		s.router.ServeFiles("/css/*filepath", http.Dir("./internal/static/css"))
		s.router.ServeFiles("/img/*filepath", http.Dir("./internal/static/img"))
		s.router.ServeFiles("/js/*filepath", http.Dir("./internal/static/js"))
	} else {
		cssFS, err := fs.Sub(staticCSS, "static/css")
		if err != nil {
			log.Fatal("error getting SubFS for static/css")
		}

		jsFS, err := fs.Sub(staticJS, "static/js")
		if err != nil {
			log.Fatal("error getting SubFS for static/js")
		}

		imgFS, err := fs.Sub(staticIMG, "static/img")
		if err != nil {
			log.Fatal("error getting SubFS for static/img")
		}

		s.router.ServeFilesWithCacheControl("/css/:commit/*filepath", cssFS)
		s.router.ServeFilesWithCacheControl("/img/:commit/*filepath", imgFS)
		s.router.ServeFilesWithCacheControl("/js/:commit/*filepath", jsFS)
	}

	s.router.NotFound = http.HandlerFunc(s.NotFoundHandler)

	s.router.GET("/about", s.PageHandler("about"))
	s.router.GET("/help", s.PageHandler("help"))
	s.router.GET("/privacy", s.PageHandler("privacy"))
	s.router.GET("/abuse", s.PageHandler("abuse"))

	s.router.GET("/", s.TimelineHandler())
	s.router.HEAD("/", s.TimelineHandler())

	s.router.GET("/robots.txt", s.RobotsHandler())
	s.router.HEAD("/robots.txt", s.RobotsHandler())

	s.router.GET("/discover", s.am.MustAuth(s.DiscoverHandler()))
	s.router.GET("/mentions", s.am.MustAuth(s.MentionsHandler()))
	s.router.GET("/search", s.SearchHandler())

	s.router.HEAD("/twt/:hash", s.PermalinkHandler())
	s.router.GET("/twt/:hash", s.PermalinkHandler())

	s.router.GET("/bookmark/:hash", s.BookmarkHandler())
	s.router.POST("/bookmark/:hash", s.BookmarkHandler())

	s.router.HEAD("/conv/:hash", s.ConversationHandler())
	s.router.GET("/conv/:hash", s.ConversationHandler())

	s.router.GET("/feeds", s.am.MustAuth(s.FeedsHandler()))
	s.router.POST("/feed", s.am.MustAuth(s.FeedHandler()))

	s.router.POST("/post", s.am.MustAuth(s.PostHandler()))
	s.router.PATCH("/post", s.am.MustAuth(s.PostHandler()))
	s.router.DELETE("/post", s.am.MustAuth(s.PostHandler()))

	// Private Messages
	s.router.GET("/messages", s.am.MustAuth(s.ListMessagesHandler()))
	s.router.GET("/messages/:msgid", s.am.MustAuth(s.ViewMessageHandler()))
	s.router.POST("/messages/send", s.am.MustAuth(s.SendMessageHandler()))
	s.router.POST("/messages/delete", s.am.MustAuth(s.DeleteMessagesHandler()))

	s.router.POST("/blog", s.am.MustAuth(s.CreateOrUpdateBlogHandler()))
	s.router.GET("/blogs/:author", s.ListBlogsHandler())
	s.router.GET("/blog/:author/:year/:month/:date/:slug", s.ViewBlogHandler())
	s.router.HEAD("/blog/:author/:year/:month/:date/:slug", s.ViewBlogHandler())
	s.router.GET("/blog/:author/:year/:month/:date/:slug/edit", s.EditBlogHandler())
	s.router.GET("/blog/:author/:year/:month/:date/:slug/delete", s.DeleteBlogHandler())
	s.router.GET("/blog/:author/:year/:month/:date/:slug/publish", s.PublishBlogHandler())

	// Redirect old URIs (twtxt <= v0.0.8) of the form /u/<nick> -> /user/<nick>/twtxt.txt
	// TODO: Remove this after v1
	s.router.GET("/u/:nick", s.OldTwtxtHandler())
	s.router.HEAD("/u/:nick", s.OldTwtxtHandler())

	// Redirect old URIs (twtxt <= v0.1.0) of the form /user/<nick>/avatar.png -> /user/<nick>/avatar
	// TODO: Remove this after v1
	s.router.GET("/user/:nick/avatar.png", s.OldAvatarHandler())
	s.router.HEAD("/user/:nick/avatar.png", s.OldAvatarHandler())

	if s.config.OpenProfiles {
		s.router.GET("/user/:nick/", s.ProfileHandler())
		s.router.GET("/user/:nick/config.yaml", s.UserConfigHandler())
	} else {
		s.router.GET("/user/:nick/", s.am.MustAuth(s.ProfileHandler()))
		s.router.GET("/user/:nick/config.yaml", s.am.MustAuth(s.UserConfigHandler()))
	}
	s.router.GET("/user/:nick/avatar", s.AvatarHandler())
	s.router.HEAD("/user/:nick/avatar", s.AvatarHandler())
	s.router.HEAD("/user/:nick/twtxt.txt", s.TwtxtHandler())
	s.router.GET("/user/:nick/twtxt.txt", s.TwtxtHandler())
	s.router.GET("/user/:nick/followers", s.FollowersHandler())
	s.router.GET("/user/:nick/following", s.FollowingHandler())
	s.router.GET("/user/:nick/bookmarks", s.BookmarksHandler())

	// WebMentions
	s.router.POST("/user/:nick/webmention", s.WebMentionHandler())

	// External Feeds
	s.router.GET("/external", s.ExternalHandler())
	s.router.GET("/externalAvatar", s.ExternalAvatarHandler())
	s.router.HEAD("/externalAvatar", s.ExternalAvatarHandler())

	// External Queries (protected by a short-lived token)
	s.router.GET("/whoFollows", s.WhoFollowsHandler())

	// Syndication Formats (RSS, Atom, JSON Feed)
	s.router.HEAD("/atom.xml", s.SyndicationHandler())
	s.router.HEAD("/user/:nick/atom.xml", s.SyndicationHandler())
	s.router.GET("/atom.xml", s.SyndicationHandler())
	s.router.GET("/user/:nick/atom.xml", s.SyndicationHandler())

	s.router.GET("/feed/:name/manage", s.am.MustAuth(s.ManageFeedHandler()))
	s.router.POST("/feed/:name/manage", s.am.MustAuth(s.ManageFeedHandler()))
	s.router.POST("/feed/:name/archive", s.am.MustAuth(s.ArchiveFeedHandler()))

	s.router.GET("/login", s.am.HasAuth(s.LoginHandler()))
	s.router.POST("/login", s.LoginHandler())

	s.router.GET("/logout", s.LogoutHandler())
	s.router.POST("/logout", s.LogoutHandler())

	s.router.GET("/register", s.am.HasAuth(s.RegisterHandler()))
	s.router.POST("/register", s.RegisterHandler())

	// Reset Password
	s.router.GET("/resetPassword", s.ResetPasswordHandler())
	s.router.POST("/resetPassword", s.ResetPasswordHandler())
	s.router.GET("/newPassword", s.ResetPasswordMagicLinkHandler())
	s.router.POST("/newPassword", s.NewPasswordHandler())

	// Media Handling
	s.router.GET("/media/:name", s.MediaHandler())
	s.router.HEAD("/media/:name", s.MediaHandler())
	s.router.POST("/upload", s.am.MustAuth(s.UploadMediaHandler()))

	// Task State
	s.router.GET("/task/:uuid", s.TaskHandler())

	// User/Feed Lookups
	s.router.GET("/lookup", s.am.MustAuth(s.LookupHandler()))

	s.router.GET("/follow", s.am.MustAuth(s.FollowHandler()))
	s.router.POST("/follow", s.am.MustAuth(s.FollowHandler()))

	s.router.GET("/import", s.am.MustAuth(s.ImportHandler()))
	s.router.POST("/import", s.am.MustAuth(s.ImportHandler()))

	s.router.GET("/unfollow", s.am.MustAuth(s.UnfollowHandler()))
	s.router.POST("/unfollow", s.am.MustAuth(s.UnfollowHandler()))

	s.router.GET("/mute", s.am.MustAuth(s.MuteHandler()))
	s.router.POST("/mute", s.am.MustAuth(s.MuteHandler()))
	s.router.GET("/unmute", s.am.MustAuth(s.UnmuteHandler()))
	s.router.POST("/unmute", s.am.MustAuth(s.UnmuteHandler()))

	s.router.GET("/transferFeed/:name", s.TransferFeedHandler())
	s.router.GET("/transferFeed/:name/:transferTo", s.TransferFeedHandler())

	s.router.GET("/settings", s.am.MustAuth(s.SettingsHandler()))
	s.router.POST("/settings", s.am.MustAuth(s.SettingsHandler()))
	s.router.POST("/token/delete/:signature", s.am.MustAuth(s.DeleteTokenHandler()))

	s.router.GET("/config", s.am.MustAuth(s.PodConfigHandler()))
	s.router.GET("/manage/pod", s.ManagePodHandler())
	s.router.POST("/manage/pod", s.ManagePodHandler())

	s.router.GET("/manage/users", s.ManageUsersHandler())
	s.router.POST("/manage/adduser", s.AddUserHandler())
	s.router.POST("/manage/deluser", s.DelUserHandler())

	s.router.GET("/deleteFeeds", s.DeleteAccountHandler())
	s.router.POST("/delete", s.am.MustAuth(s.DeleteAllHandler()))

	// Support / Report Abuse handlers

	s.router.GET("/support", s.SupportHandler())
	s.router.POST("/support", s.SupportHandler())
	s.router.GET("/_captcha", s.CaptchaHandler())

	s.router.GET("/report", s.ReportHandler())
	s.router.POST("/report", s.ReportHandler())
}

// NewServer ...
func NewServer(bind string, options ...Option) (*Server, error) {
	config := NewConfig()

	for _, opt := range options {
		if err := opt(config); err != nil {
			return nil, err
		}
	}

	settings, err := LoadSettings(filepath.Join(config.Data, "settings.yaml"))
	if err != nil {
		log.Warnf("error loading pod settings: %s", err)
	} else {
		if err := merger.MergeOverwrite(config, settings); err != nil {
			log.WithError(err).Error("error merging pod settings")
			return nil, err
		}
	}

	if err := config.Validate(); err != nil {
		log.WithError(err).Error("error validating config")
		return nil, fmt.Errorf("error validating config: %w", err)
	}

	blogs, err := LoadBlogsCache(config.Data)
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

	msgs, err := LoadMessagesCache(config.Data)
	if err != nil {
		log.WithError(err).Error("error loading messages cache (re-creating)")
		msgs = NewMessagesCache()
	}
	log.Info("updating messages cache")
	msgs.Refresh(config)

	cache, err := LoadCache(config.Data)
	if err != nil {
		log.WithError(err).Error("error loading feed cache")
		return nil, err
	}

	archive, err := NewDiskArchiver(filepath.Join(config.Data, archiveDir))
	if err != nil {
		log.WithError(err).Error("error creating feed archiver")
		return nil, err
	}

	db, err := NewStore(config.Store)
	if err != nil {
		log.WithError(err).Error("error creating store")
		return nil, err
	}

	if err := db.Merge(); err != nil {
		log.WithError(err).Error("error merging store")
		return nil, err
	}

	// translator
	translator, err := NewTranslator()
	if err != nil {
		log.WithError(err).Error("error loading translator")
		return nil, err
	}

	tmplman, err := NewTemplateManager(config, translator, blogs, cache, archive)
	if err != nil {
		log.WithError(err).Error("error creating template manager")
		return nil, err
	}

	router := NewRouter()

	am := auth.NewManager(auth.NewOptions("/login", "/register"))

	tasks := NewDispatcher(10, 100) // TODO: Make this configurable?

	pm := passwords.NewScryptPasswords(nil)

	sc := NewSessionStore(db, config.SessionCacheTTL)

	sm := session.NewManager(
		session.NewOptions(
			config.Name,
			config.CookieSecret,
			config.LocalURL().Scheme == "https",
			config.SessionExpiry,
		),
		sc,
	)

	api := NewAPI(router, config, cache, archive, db, pm, tasks)

	pop3Service := NewPOP3Service(config, db, pm, msgs, tasks)

	smtpService := NewSMTPService(config, db, pm, msgs, tasks)

	csrfHandler := nosurf.New(router)
	csrfHandler.ExemptGlob("/api/v1/*")

	server := &Server{
		bind:    bind,
		config:  config,
		router:  router,
		tmplman: tmplman,

		server: &http.Server{
			Addr: bind,
			Handler: logger.New(logger.Options{
				Prefix:               "twtxt",
				RemoteAddressHeaders: []string{"X-Forwarded-For"},
			}).Handler(
				gziphandler.GzipHandler(
					sm.Handler(csrfHandler),
				),
			),
		},

		// API
		api: api,

		// POP3 Servicee
		pop3Service: pop3Service,

		// SMTP Servicee
		smtpService: smtpService,

		// Blogs Cache
		blogs: blogs,

		// Messages Cache
		msgs: msgs,

		// Feed Cache
		cache: cache,

		// Feed Archiver
		archive: archive,

		// Data Store
		db: db,

		// Schedular
		cron: cron.New(),

		// Dispatcher
		tasks: tasks,

		// Auth Manager
		am: am,

		// Session Manager
		sc: sc,
		sm: sm,

		// Password Manager
		pm: pm,

		// Translator
		translator: translator,
	}

	if err := server.setupCronJobs(); err != nil {
		log.WithError(err).Error("error setting up background jobs")
		return nil, err
	}
	server.cron.Start()
	log.Info("started background jobs")

	server.tasks.Start()
	log.Info("started task dispatcher")

	server.pop3Service.Start()
	log.Info("started POP3 service")

	server.smtpService.Start()
	log.Info("started SMTP service")

	server.setupWebMentions()
	log.Infof("started webmentions processor")

	server.setupMetrics()
	log.Infof("serving metrics endpoint at %s/metrics", server.config.BaseURL)

	// Log interesting configuration options
	log.Infof("Debug: %t", server.config.Debug)
	log.Infof("Parser: %s", server.config.Parser)
	log.Infof("Instance Name: %s", server.config.Name)
	log.Infof("Base URL: %s", server.config.BaseURL)
	log.Infof("Admin User: %s", server.config.AdminUser)
	log.Infof("Admin Name: %s", server.config.AdminName)
	log.Infof("Admin Email: %s", server.config.AdminEmail)
	log.Infof("Max Twts per Page: %d", server.config.TwtsPerPage)
	log.Infof("Max Cache TTL: %s", server.config.MaxCacheTTL)
	log.Infof("Fetch Interval: %s", server.config.FetchInterval)
	log.Infof("Max Cache Items: %d", server.config.MaxCacheItems)
	log.Infof("Maximum length of Posts: %d", server.config.MaxTwtLength)
	log.Infof("Open User Profiles: %t", server.config.OpenProfiles)
	log.Infof("Open Registrations: %t", server.config.OpenRegistrations)
	log.Infof("SMTP Host: %s", server.config.SMTPHost)
	log.Infof("SMTP Port: %d", server.config.SMTPPort)
	log.Infof("SMTP User: %s", server.config.SMTPUser)
	log.Infof("SMTP From: %s", server.config.SMTPFrom)
	log.Infof("Max Fetch Limit: %s", humanize.Bytes(uint64(server.config.MaxFetchLimit)))
	log.Infof("Max Upload Size: %s", humanize.Bytes(uint64(server.config.MaxUploadSize)))
	log.Infof("API Session Time: %s", server.config.APISessionTime)

	// Warn about user registration being disabled.
	if !server.config.OpenRegistrations {
		log.Warn("Open Registrations are disabled as per configuration (no -R/--open-registrations)")
	}

	server.initRoutes()
	api.initRoutes()

	go server.runStartupJobs()

	return server, nil
}

func (s *Server) tr(ctx *Context, msgID string, data ...interface{}) string {
	return s.translator.Translate(ctx, msgID, data...)
}
