package twtxt

import (
	"fmt"
	"net/http"

	rice "github.com/GeertJohan/go.rice"
	"github.com/NYTimes/gziphandler"
	"github.com/julienschmidt/httprouter"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
	"github.com/unrolled/logger"

	"github.com/prologic/twtxt/auth"
	"github.com/prologic/twtxt/password"
	"github.com/prologic/twtxt/session"
)

// Router ...
type Router struct {
	httprouter.Router
}

// NewRouter ...
func NewRouter() *Router {
	return &Router{
		httprouter.Router{
			RedirectTrailingSlash:  true,
			RedirectFixedPath:      true,
			HandleMethodNotAllowed: false,
			HandleOPTIONS:          true,
		},
	}
}

// ServeFilesWithCacheControl ...
func (r *Router) ServeFilesWithCacheControl(path string, root http.FileSystem) {
	if len(path) < 10 || path[len(path)-10:] != "/*filepath" {
		panic("path must end with /*filepath in path '" + path + "'")
	}

	fileServer := http.FileServer(root)

	r.GET(path, func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		w.Header().Set("Vary", "Accept-Encoding")
		w.Header().Set("Cache-Control", "public, max-age=7776000")
		req.URL.Path = ps.ByName("filepath")
		fileServer.ServeHTTP(w, req)
	})
}

// Server ...
type Server struct {
	bind      string
	config    *Config
	templates *Templates
	router    *Router

	// Database
	db Store

	// Scheduler
	cron *cron.Cron

	// Auth
	am *auth.Manager

	// Sessions
	sm *session.Manager

	// Passwords
	pm *password.Manager
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

// ListenAndServe ...
func (s *Server) ListenAndServe() {

	log.Fatal(
		http.ListenAndServe(
			s.bind,
			logger.New(logger.Options{
				Prefix:               "twtxt",
				RemoteAddressHeaders: []string{"X-Forwarded-For"},
			}).Handler(
				gziphandler.GzipHandler(
					s.sm.Handler(
						s.router,
					),
				),
			),
		),
	)
}

func (s *Server) setupCronJobs() error {
	for spec, factory := range Jobs {
		job := factory(s.config, s.db)
		if err := s.cron.AddJob(spec, job); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) initRoutes() {
	s.router.ServeFilesWithCacheControl(
		"/css/*filepath",
		rice.MustFindBox("static/css").HTTPBox(),
	)

	s.router.ServeFilesWithCacheControl(
		"/img/*filepath",
		rice.MustFindBox("static/img").HTTPBox(),
	)

	s.router.ServeFilesWithCacheControl(
		"/js/*filepath",
		rice.MustFindBox("static/js").HTTPBox(),
	)

	s.router.GET("/favicon.ico", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		box, err := rice.FindBox("static")
		if err != nil {
			http.Error(w, "404 file not found", http.StatusNotFound)
			return
		}

		buf, err := box.Bytes("favicon.ico")
		if err != nil {
			msg := fmt.Sprintf("error reading favicon: %s", err)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "image/x-icon")
		w.Header().Set("Vary", "Accept-Encoding")
		w.Header().Set("Cache-Control", "public, max-age=7776000")

		n, err := w.Write(buf)
		if err != nil {
			log.Errorf("error writing response for favicon: %s", err)
		} else if n != len(buf) {
			log.Warnf(
				"not all bytes of favicon response were written: %d/%d",
				n, len(buf),
			)
		}
	})

	s.router.NotFound = http.HandlerFunc(s.NotFoundHandler)

	s.router.GET("/about", s.PageHandler("about"))
	s.router.GET("/privacy", s.PageHandler("privacy"))
	s.router.GET("/support", s.PageHandler("support"))

	s.router.GET("/", s.TimelineHandler())
	s.router.POST("/post", s.am.MustAuth(s.PostHandler()))
	s.router.HEAD("/u/:nick", s.TwtxtHandler())
	s.router.GET("/u/:nick", s.TwtxtHandler())

	s.router.GET("/login", s.LoginHandler())
	s.router.POST("/login", s.LoginHandler())

	s.router.GET("/logout", s.LogoutHandler())
	s.router.POST("/logout", s.LogoutHandler())

	s.router.GET("/register", s.RegisterHandler())
	s.router.POST("/register", s.RegisterHandler())

	s.router.GET("/follow", s.am.MustAuth(s.FollowHandler()))
	s.router.POST("/follow", s.am.MustAuth(s.FollowHandler()))

	s.router.GET("/import", s.am.MustAuth(s.ImportHandler()))
	s.router.POST("/import", s.am.MustAuth(s.ImportHandler()))

	s.router.GET("/unfollow", s.am.MustAuth(s.UnfollowHandler()))
	s.router.POST("/unfollow", s.am.MustAuth(s.UnfollowHandler()))

	s.router.GET("/settings", s.am.MustAuth(s.SettingsHandler()))
	s.router.POST("/settings", s.am.MustAuth(s.SettingsHandler()))
}

// NewServer ...
func NewServer(bind string, options ...Option) (*Server, error) {
	templates, err := NewTemplates()
	if err != nil {
		log.WithError(err).Error("error loading templates")
		return nil, err
	}

	config := NewConfig()

	router := NewRouter()

	server := &Server{
		bind:      bind,
		config:    config,
		router:    router,
		templates: templates,

		// Schedular
		cron: cron.New(),

		// Auth
		am: auth.NewManager(auth.NewOptions("/login", "/register")),

		// Sessions
		sm: session.NewManager(
			session.NewOptions("twtxt", "mysecret"),
			session.NewMemoryStore(-1),
		),

		// Passwords
		pm: password.NewManager(nil),
	}

	for _, opt := range options {
		if err := opt(server.config); err != nil {
			return nil, err
		}
	}

	db, err := NewStore(server.config.Store)
	if err != nil {
		log.WithError(err).Error("error creating store")
		return nil, err
	}
	server.db = db

	if err := server.setupCronJobs(); err != nil {
		log.WithError(err).Error("error settupt up background jobs")
		return nil, err
	}
	server.cron.Start()
	log.Infof("started background jobs")

	server.initRoutes()

	return server, nil
}
