package internal

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

const defaultPreambleTemplate = `# Twtxt is an open, distributed microblogging platform that
# uses human-readable text files, common transport protocols,
# and free software.
#
# Learn more about twtxt at  https://github.com/buckket/twtxt
#
# nick        = {{ .Profile.Username }}
# url         = {{ .Profile.URL }}
# avatar      = {{ .Profile.AvatarURL }}
# description = {{ .Profile.Tagline }}
#
{{- if .Profile.ShowFollowing }}
{{ range $nick, $url := .Profile.Following -}}
# follow = {{ $nick }} {{ $url }}
{{ end -}}
#
{{ end }}
`

// OldTwtxtHandler ...
// Redirect old URIs (twtxt <= v0.0.8) of the form /u/<nick> -> /user/<nick>/twtxt.txt
// TODO: Remove this after v1
func (s *Server) OldTwtxtHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		nick := NormalizeUsername(p.ByName("nick"))
		if nick == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		newURI := fmt.Sprintf(
			"%s/user/%s/twtxt.txt",
			strings.TrimSuffix(s.config.BaseURL, "/"),
			nick,
		)

		http.Redirect(w, r, newURI, http.StatusMovedPermanently)
	}
}

// TwtxtHandler ...
func (s *Server) TwtxtHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		nick := NormalizeUsername(p.ByName("nick"))
		if nick == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		fn, err := securejoin.SecureJoin(filepath.Join(s.config.Data, "feeds"), nick)
		if err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		fileInfo, err := os.Stat(fn)
		if err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "Feed Not Found", http.StatusNotFound)
				return
			}

			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		followerClient, err := DetectFollowerFromUserAgent(r.UserAgent())
		if err != nil {
			log.WithError(err).Warnf("unable to detect twtxt client from %s", FormatRequest(r))
		} else {
			var (
				user       *User
				feed       *Feed
				err        error
				followedBy bool
			)

			if user, err = s.db.GetUser(nick); err == nil {
				followedBy = user.FollowedBy(followerClient.URL)
			} else if feed, err = s.db.GetFeed(nick); err == nil {
				followedBy = feed.FollowedBy(followerClient.URL)
			} else {
				log.WithError(err).Warnf("unable to load user or feed object for %s", nick)
			}

			if (user != nil) || (feed != nil) {
				if (s.config.Debug || followerClient.IsPublicURL()) && !followedBy {
					if _, err := AppendSpecial(
						s.config, s.db,
						twtxtBot,
						fmt.Sprintf(
							"FOLLOW: @<%s %s> from @<%s %s> using %s",
							nick, URLForUser(s.config, nick),
							followerClient.Nick, followerClient.URL,
							followerClient.Client,
						),
					); err != nil {
						log.WithError(err).Warnf("error appending special FOLLOW post")
					}

					if user != nil {
						user.AddFollower(followerClient.Nick, followerClient.URL)
						if err := s.db.SetUser(nick, user); err != nil {
							log.WithError(err).Warnf("error updating user object for %s", nick)
						}
					} else if feed != nil {
						feed.AddFollower(followerClient.Nick, followerClient.URL)
						if err := s.db.SetFeed(nick, feed); err != nil {
							log.WithError(err).Warnf("error updating feed object for %s", nick)
						}
					} else {
						panic("should not be reached")
						// Should not be reached
					}
				}
			}
		}

		// XXX: This is stupid doing this twice
		// TODO: Refactor all of this :/

		if user, err := s.db.GetUser(nick); err == nil {
			ctx.Profile = user.Profile(s.config.BaseURL, ctx.User)
		} else if feed, err := s.db.GetFeed(nick); err == nil {
			ctx.Profile = feed.Profile(s.config.BaseURL, ctx.User)
		} else {
			log.WithError(err).Warnf("unable to load user or feed profile for %s", nick)
		}

		var preampleTemplate string

		preampleCustomTemplateFn := filepath.Join(s.config.Data, feedsDir, fmt.Sprintf("%s.tpl", nick))
		if FileExists(preampleCustomTemplateFn) {
			if data, err := ioutil.ReadFile(preampleCustomTemplateFn); err == nil {
				preampleTemplate = string(data)
			} else {
				log.WithError(err).Warnf("error loading custom preamble template for %s")
				preampleTemplate = defaultPreambleTemplate
			}
		} else {
			preampleTemplate = defaultPreambleTemplate
		}

		preamble, err := RenderString(preampleTemplate, ctx)
		if err != nil {
			log.WithError(err).Warn("error rendering twtxt preamble")
		}

		f, err := os.Open(fn)
		if err != nil {
			log.WithError(err).Error("error opening feed")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		defer f.Close()

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", int64(len(preamble))+fileInfo.Size()))
		w.Header().Set("Link", fmt.Sprintf(`<%s/user/%s/webmention>; rel="webmention"`, s.config.BaseURL, nick))
		w.Header().Set("Last-Modified", fileInfo.ModTime().UTC().Format(http.TimeFormat))

		if r.Method == http.MethodHead {
			return
		}

		if _, err = w.Write([]byte(preamble)); err != nil {
			log.WithError(err).Warn("error writing twtxt preamble")
		}
		_, _ = io.Copy(w, f)
	}
}
