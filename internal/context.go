package internal

import (
	"fmt"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/vcraescu/go-paginator"

	"github.com/prologic/twtxt"
	"github.com/prologic/twtxt/internal/session"
	"github.com/prologic/twtxt/types"
)

type Alternative struct {
	Type  string
	Title string
	URL   string
}

type Alternatives []Alternative

type Context struct {
	BaseURL                 string
	InstanceName            string
	SoftwareVersion         string
	TwtsPerPage             int
	TwtPrompt               string
	MaxTwtLength            int
	RegisterDisabled        bool
	RegisterDisabledMessage string

	Username      string
	User          *User
	LastTwt       types.Twt
	Profile       Profile
	Alternatives  Alternatives
	Authenticated bool

	Error   bool
	Message string
	Theme   string
	Commit  string

	Twter       types.Twter
	Twts        types.Twts
	Feeds       []*Feed
	FeedSources FeedSourceMap
	Pager       paginator.Paginator

	// Reset Password Token
	PasswordResetToken string
}

func NewContext(conf *Config, db Store, req *http.Request) *Context {
	ctx := &Context{
		BaseURL:          conf.BaseURL,
		InstanceName:     conf.Name,
		SoftwareVersion:  twtxt.FullVersion(),
		TwtsPerPage:      conf.TwtsPerPage,
		TwtPrompt:        conf.RandomTwtPrompt(),
		MaxTwtLength:     conf.MaxTwtLength,
		RegisterDisabled: !conf.Register,

		Commit: twtxt.Commit,
		Theme:  conf.Theme,

		Alternatives: Alternatives{
			Alternative{
				Type:  "application/atom+xml",
				Title: fmt.Sprintf("%s local feed", conf.Name),
				URL:   fmt.Sprintf("%s/atom.xml", conf.BaseURL),
			},
		},
	}

	// Set the theme based on user-defined perfernece via Cookies
	// XXX: This is what cookies were meant for :D (not tracking!)
	if cookie, err := req.Cookie("theme"); err == nil {
		name := strings.ToLower(cookie.Value)
		switch name {
		case "light", "dark":
			ctx.Theme = name
		default:
			log.WithField("name", name).Warn("invalid theme found in user cookie")
		}
	}

	if sess := req.Context().Value(session.SessionKey); sess != nil {
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

		ctx.Twter = types.Twter{
			Nick: user.Username,
			URL:  URLForUser(conf.BaseURL, user.Username),
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
		ctx.Twter = types.Twter{}
	}

	return ctx
}

func (ctx Context) IsLocal(url string) bool {
	if NormalizeURL(url) == "" {
		return false
	}
	return strings.HasPrefix(NormalizeURL(url), NormalizeURL(ctx.BaseURL))
}
