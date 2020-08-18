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
	"github.com/theplant-retired/timezones"
)

type Link struct {
	Href string
	Rel  string
}

type Alternative struct {
	Type  string
	Title string
	URL   string
}

type Alternatives []Alternative
type Links []Link

type Meta struct {
	Title       string
	Author      string
	Keywords    string
	Description string
}

type Context struct {
	BaseURL                 string
	InstanceName            string
	SoftwareVersion         string
	TwtsPerPage             int
	TwtPrompt               string
	MaxTwtLength            int
	RegisterDisabled        bool
	RegisterDisabledMessage string

	Timezones []*timezones.Zoneinfo

	Username      string
	User          *User
	LastTwt       types.Twt
	Profile       Profile
	Authenticated bool

	Error   bool
	Message string
	Theme   string
	Commit  string

	Title        string
	Meta         Meta
	Links        Links
	Alternatives Alternatives

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
		RegisterDisabled: !conf.OpenRegistrations,

		Commit: twtxt.Commit,
		Theme:  conf.Theme,

		Timezones: timezones.AllZones,

		Title: "",
		Meta: Meta{
			Title:       DefaultMetaTitle,
			Author:      DefaultMetaAuthor,
			Keywords:    DefaultMetaKeywords,
			Description: DefaultMetaDescription,
		},

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
