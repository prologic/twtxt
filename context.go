package twtxt

import (
	"net/http"

	"github.com/prologic/twtxt/session"
	log "github.com/sirupsen/logrus"
	"github.com/vcraescu/go-paginator"
)

type Context struct {
	InstanceName            string
	SoftwareVersion         string
	RegisterDisabled        bool
	RegisterDisabledMessage string

	Username      string
	User          *User
	Authenticated bool

	Error   bool
	Message string

	Tweeter Tweeter
	Tweets  Tweets
	Pager   paginator.Paginator
}

func NewContext(conf *Config, db Store, req *http.Request) *Context {
	ctx := &Context{
		InstanceName:    conf.Name,
		SoftwareVersion: FullVersion(),

		RegisterDisabled: !conf.Register,
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
		user.Following["me"] = user.URL

		ctx.User = user
	}

	return ctx
}
