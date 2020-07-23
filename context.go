package twtxt

import (
	"net/http"
	"strings"

	"github.com/prologic/twtxt/session"
	log "github.com/sirupsen/logrus"
	"github.com/vcraescu/go-paginator"
)

type Context struct {
	InstanceName            string
	SoftwareVersion         string
	MaxTweetLength          int
	RegisterDisabled        bool
	RegisterDisabledMessage string

	Username      string
	User          *User
	Authenticated bool

	Error   bool
	Message string
	Theme   string

	Tweeter Tweeter
	Tweets  Tweets
	Feeds   Feeds
	Pager   paginator.Paginator
}

func NewContext(conf *Config, db Store, req *http.Request) *Context {
	ctx := &Context{
		InstanceName:     conf.Name,
		SoftwareVersion:  FullVersion(),
		MaxTweetLength:   conf.MaxTweetLength,
		RegisterDisabled: !conf.Register,

		Theme: conf.Theme,
	}

	// Set the theme based on user-defined perfernece via Cookies
	// XXX: This is what cookies were meant for :D (not tracking!)
	if cookie, err := req.Cookie("theme"); err == nil {
		log.Debugf("%#v", cookie)
		name := strings.ToLower(cookie.Value)
		switch name {
		case "light", "dark":
			log.Debugf("setting theme to %s", name)
			ctx.Theme = name
		default:
			log.WithField("name", name).Warn("invalid theme found in user cookie")
		}
	}

	if sess := req.Context().Value("sesssion"); sess != nil {
		if username, ok := sess.(*session.Session).Get("username"); ok {
			ctx.Authenticated = true
			ctx.Username = username
		}
	}

	if ctx.Authenticated && ctx.Username != "" {
		user, err := db.GetUser(ctx.Username)
		if err != nil {
			log.WithError(err).Warnf("error loading user object for %s", ctx.Username)
		}

		ctx.Tweeter = Tweeter{
			Nick: user.Username,
			URL:  user.URL,
		}

		// Every registered new user follows themselves
		// TODO: Make  this configurable server behaviour?
		if user.Following == nil {
			user.Following = make(map[string]string)
		}
		user.Following[user.Username] = user.URL

		ctx.User = user
	} else {
		ctx.User = &User{}
		ctx.Tweeter = Tweeter{}
	}

	return ctx
}
