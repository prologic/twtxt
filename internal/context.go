package internal

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/vcraescu/go-paginator"

	"github.com/prologic/twtxt"
	"github.com/prologic/twtxt/internal/session"
	"github.com/prologic/twtxt/types"
	"github.com/theplant-retired/timezones"
)

type Meta struct {
	Title       string
	Author      string
	Keywords    string
	Description string
}

type Context struct {
	Config string

	BaseURL                 string
	InstanceName            string
	SoftwareVersion         string
	TwtsPerPage             int
	TwtPrompt               string
	MaxTwtLength            int
	RegisterDisabled        bool
	OpenProfiles			bool
	RegisterDisabledMessage string

	Timezones []*timezones.Zoneinfo

	Reply         string
	Username      string
	User          *User
	Tokens        []*Token
	LastTwt       types.Twt
	Profile       types.Profile
	Authenticated bool
	IsAdmin 	  bool

	Error   bool
	Message string
	Theme   string
	Commit  string

	Page    string
	Content template.HTML

	Title        string
	Meta         Meta
	Links        types.Links
	Alternatives types.Alternatives

	Twter       types.Twter
	Twts        types.Twts
	BlogPost    *BlogPost
	BlogPosts   BlogPosts
	Feeds       []*Feed
	FeedSources FeedSourceMap
	Pager       *paginator.Paginator

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
		OpenProfiles:	  conf.OpenProfiles,

		Commit: twtxt.Commit,
		Theme:  conf.Theme,

		Timezones: timezones.AllZones,

		Title: "",
		Meta: Meta{
			Title:       DefaultMetaTitle,
			Author:      DefaultMetaAuthor,
			Keywords:    DefaultMetaKeywords,
			Description: conf.Description,
		},

		Alternatives: types.Alternatives{
			types.Alternative{
				Type:  "application/atom+xml",
				Title: fmt.Sprintf("%s local feed", conf.Name),
				URL:   fmt.Sprintf("%s/atom.xml", conf.BaseURL),
			},
		},
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
			URL:  URLForUser(conf, user.Username),
		}

		ctx.User = user

		tokens, err := db.GetUserTokens(user)
		if err != nil {
			log.WithError(err).Warnf("error loading tokens for %s", ctx.Username)
		}
		ctx.Tokens = tokens

	} else {
		ctx.User = &User{}
		ctx.Twter = types.Twter{}
	}

	if ctx.Username == conf.AdminUser {
		ctx.IsAdmin = true
	}

	// Set the theme based on user preferences
	theme := strings.ToLower(ctx.User.Theme)
	switch theme {
	case "", "auto":
		ctx.Theme = ""
	case "light", "dark":
		ctx.Theme = theme
	default:
		log.WithField("name", theme).Warn("invalid theme found")
	}

	return ctx
}
