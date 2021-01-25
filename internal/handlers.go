package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"image/png"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	rice "github.com/GeertJohan/go.rice"
	"github.com/chai2010/webp"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/dgrijalva/jwt-go"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/gorilla/feeds"
	"github.com/james4k/fmatter"
	"github.com/julienschmidt/httprouter"
	"github.com/rickb777/accept"
	"github.com/securisec/go-keywords"
	log "github.com/sirupsen/logrus"
	"github.com/vcraescu/go-paginator"
	"github.com/vcraescu/go-paginator/adapter"
	"gopkg.in/yaml.v2"

	"github.com/jointwt/twtxt/internal/session"
	"github.com/jointwt/twtxt/types"
)

const (
	MediaResolution  = 720 // 720x576
	AvatarResolution = 360 // 360x360
	AsyncTaskLimit   = 5
	MaxFailedLogins  = 3 // By default 3 failed login attempts per 5 minutes
)

var (
	ErrFeedImposter = errors.New("error: imposter detected, you do not own this feed")
)

func (s *Server) NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, "Endpoint Not Found", http.StatusNotFound)
		return
	}

	ctx := NewContext(s.config, s.db, r)
	ctx.Title = "Page Not Found"
	w.WriteHeader(http.StatusNotFound)
	s.render("404", w, ctx)
}

type FrontMatter struct {
	Title string
}

// PageHandler ...
func (s *Server) PageHandler(name string) httprouter.Handle {
	box := rice.MustFindBox("pages")
	mdTpl := box.MustString(fmt.Sprintf("%s.md", name))

	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		md, err := RenderString(mdTpl, ctx)
		if err != nil {
			log.WithError(err).Errorf("error rendering page %s", name)
			ctx.Error = true
			ctx.Message = "Error loading help page! Please contact support."
			s.render("error", w, ctx)
			return
		}

		var frontmatter FrontMatter
		content, err := fmatter.Read([]byte(md), &frontmatter)
		if err != nil {
			log.WithError(err).Error("error parsing front matter")
			ctx.Error = true
			ctx.Message = "Error loading page! Please contact support."
			s.render("error", w, ctx)
			return
		}

		extensions := parser.CommonExtensions | parser.AutoHeadingIDs
		p := parser.NewWithExtensions(extensions)

		htmlFlags := html.CommonFlags
		opts := html.RendererOptions{
			Flags:     htmlFlags,
			Generator: "",
		}
		renderer := html.NewRenderer(opts)

		html := markdown.ToHTML(content, p, renderer)

		var title string

		if frontmatter.Title != "" {
			title = frontmatter.Title
		} else {
			title = strings.Title(name)
		}
		ctx.Title = title

		ctx.Page = name
		ctx.Content = template.HTML(html)

		s.render("page", w, ctx)
	}
}

// UserConfigHandler ...
func (s *Server) UserConfigHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		nick := NormalizeUsername(p.ByName("nick"))
		if nick == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		nick = NormalizeUsername(nick)

		var (
			url       string
			following map[string]string
			bookmarks map[string]string
		)

		if s.db.HasUser(nick) {
			user, err := s.db.GetUser(nick)
			if err != nil {
				log.WithError(err).Errorf("error loading user object for %s", nick)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			url = user.URL
			if ctx.Authenticated || user.IsFollowingPubliclyVisible {
				following = user.Following
			}
			if ctx.Authenticated || user.IsBookmarksPubliclyVisible {
				bookmarks = user.Bookmarks
			}
		} else if s.db.HasFeed(nick) {
			feed, err := s.db.GetFeed(nick)
			if err != nil {
				log.WithError(err).Errorf("error loading feed object for %s", nick)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			url = feed.URL
		} else {
			http.Error(w, "User or Feed not found", http.StatusNotFound)
			return
		}

		config := struct {
			Nick      string            `json:"nick"`
			URL       string            `json:"url"`
			Following map[string]string `json:"following"`
			Bookmarks map[string]string `json:"bookmarks"`
		}{
			Nick:      nick,
			URL:       url,
			Following: following,
			Bookmarks: bookmarks,
		}

		data, err := yaml.Marshal(config)
		if err != nil {
			log.WithError(err).Errorf("error exporting user/feed config")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/yaml")
		if r.Method == http.MethodHead {
			return
		}

		_, _ = w.Write(data)
	}
}

// ProfileHandler ...
func (s *Server) ProfileHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		log.Debugf("in ProfileHandler()...")

		ctx := NewContext(s.config, s.db, r)

		nick := NormalizeUsername(p.ByName("nick"))
		if nick == "" {
			ctx.Error = true
			ctx.Message = "No user specified"
			s.render("error", w, ctx)
			return
		}

		log.Debugf("nick: %s", nick)

		var profile types.Profile

		if s.db.HasUser(nick) {
			user, err := s.db.GetUser(nick)
			if err != nil {
				log.WithError(err).Errorf("error loading user object for %s", nick)
				ctx.Error = true
				ctx.Message = "Error loading profile"
				s.render("error", w, ctx)
				return
			}
			profile = user.Profile(s.config.BaseURL, ctx.User)
		} else if s.db.HasFeed(nick) {
			feed, err := s.db.GetFeed(nick)
			if err != nil {
				log.WithError(err).Errorf("error loading feed object for %s", nick)
				ctx.Error = true
				ctx.Message = "Error loading profile"
				s.render("error", w, ctx)
				return
			}
			profile = feed.Profile(s.config.BaseURL, ctx.User)
		} else {
			ctx.Error = true
			ctx.Message = "User or Feed Not Found"
			s.render("404", w, ctx)
			return
		}

		ctx.Profile = profile

		ctx.Links = append(ctx.Links, types.Link{
			Href: fmt.Sprintf("%s/webmention", UserURL(profile.URL)),
			Rel:  "webmention",
		})

		ctx.Alternatives = append(ctx.Alternatives, types.Alternatives{
			types.Alternative{
				Type:  "text/plain",
				Title: fmt.Sprintf("%s's Twtxt Feed", profile.Username),
				URL:   profile.URL,
			},
			types.Alternative{
				Type:  "application/atom+xml",
				Title: fmt.Sprintf("%s's Atom Feed", profile.Username),
				URL:   fmt.Sprintf("%s/atom.xml", UserURL(profile.URL)),
			},
		}...)

		twts := s.cache.GetByURL(profile.URL)

		var pagedTwts types.Twts

		page := SafeParseInt(r.FormValue("p"), 1)
		pager := paginator.New(adapter.NewSliceAdapter(twts), s.config.TwtsPerPage)
		pager.SetPage(page)

		if err := pager.Results(&pagedTwts); err != nil {
			log.WithError(err).Error("error sorting and paging twts")
			ctx.Error = true
			ctx.Message = "An error occurred while loading the timeline"
			s.render("error", w, ctx)
			return
		}

		ctx.Title = fmt.Sprintf("%s's Profile: %s", profile.Username, profile.Tagline)
		ctx.Twts = FilterTwts(ctx.User, pagedTwts)
		ctx.Pager = &pager

		s.render("profile", w, ctx)
	}
}

// ManageFeedHandler...
func (s *Server) ManageFeedHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)
		feedName := NormalizeFeedName(p.ByName("name"))

		if feedName == "" {
			ctx.Error = true
			ctx.Message = "No feed specified"
			s.render("error", w, ctx)
			return
		}

		feed, err := s.db.GetFeed(feedName)
		if err != nil {
			log.WithError(err).Errorf("error loading feed object for %s", feedName)
			ctx.Error = true
			if err == ErrFeedNotFound {
				ctx.Message = "Feed not found"
				s.render("404", w, ctx)
			}

			ctx.Message = "Error loading feed"
			s.render("error", w, ctx)
			return
		}

		if !ctx.User.OwnsFeed(feed.Name) {
			ctx.Error = true
			s.render("401", w, ctx)
			return
		}

		switch r.Method {
		case http.MethodGet:
			ctx.Profile = feed.Profile(s.config.BaseURL, ctx.User)
			ctx.Title = fmt.Sprintf("Manage feed %s", feed.Name)
			s.render("manageFeed", w, ctx)
			return
		case http.MethodPost:
			description := r.FormValue("description")
			feed.Description = description

			avatarFile, _, err := r.FormFile("avatar_file")
			if err != nil && err != http.ErrMissingFile {
				log.WithError(err).Error("error parsing form file")
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if avatarFile != nil {
				opts := &ImageOptions{
					Resize: true,
					Width:  AvatarResolution,
					Height: AvatarResolution,
				}
				_, err = StoreUploadedImage(
					s.config, avatarFile,
					avatarsDir, feedName,
					opts,
				)
				if err != nil {
					ctx.Error = true
					ctx.Message = fmt.Sprintf("Error updating user: %s", err)
					s.render("error", w, ctx)
					return
				}
			}

			if err := s.db.SetFeed(feed.Name, feed); err != nil {
				log.WithError(err).Warnf("error updating user object for followee %s", feed.Name)

				ctx.Error = true
				ctx.Message = "Error updating feed"
				s.render("error", w, ctx)
				return
			}

			ctx.Error = false
			ctx.Message = "Successfully updated feed"
			s.render("error", w, ctx)
			return
		}

		ctx.Error = true
		ctx.Message = "Not found"
		s.render("404", w, ctx)
	}
}

// ArchiveFeedHandler...
func (s *Server) ArchiveFeedHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)
		feedName := NormalizeFeedName(p.ByName("name"))

		if feedName == "" {
			ctx.Error = true
			ctx.Message = "No feed specified"
			s.render("error", w, ctx)
			return
		}

		feed, err := s.db.GetFeed(feedName)
		if err != nil {
			log.WithError(err).Errorf("error loading feed object for %s", feedName)
			ctx.Error = true
			if err == ErrFeedNotFound {
				ctx.Message = "Feed not found"
				s.render("404", w, ctx)
			}

			ctx.Message = "Error loading feed"
			s.render("error", w, ctx)
			return
		}

		if !ctx.User.OwnsFeed(feed.Name) {
			ctx.Error = true
			s.render("401", w, ctx)
			return
		}

		if err := DetachFeedFromOwner(s.db, ctx.User, feed); err != nil {
			log.WithError(err).Warnf("Error detaching feed owner %s from feed %s", ctx.User.Username, feed.Name)
			ctx.Error = true
			ctx.Message = "Error archiving feed"
			s.render("error", w, ctx)
			return
		}

		ctx.Error = false
		ctx.Message = "Successfully archived feed"
		s.render("error", w, ctx)
	}
}

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

// OldAvatarHandler ...
// Redirect old URIs (twtxt <= v0.1.0) of the form /user/<nick>/avatar.png -> /user/<nick>/avatar
// TODO: Remove this after v1
func (s *Server) OldAvatarHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		nick := NormalizeUsername(p.ByName("nick"))
		if nick == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		newURI := fmt.Sprintf(
			"%s/user/%s/avatar",
			strings.TrimSuffix(s.config.BaseURL, "/"),
			nick,
		)

		http.Redirect(w, r, newURI, http.StatusMovedPermanently)
	}
}

// AvatarHandler ...
func (s *Server) AvatarHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		w.Header().Set("Cache-Control", "public, no-cache, must-revalidate")

		nick := NormalizeUsername(p.ByName("nick"))
		if nick == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		if !s.db.HasUser(nick) && !FeedExists(s.config, nick) {
			http.Error(w, "User or Feed Not Found", http.StatusNotFound)
			return
		}

		preferredContentType := accept.PreferredContentTypeLike(r.Header, "image/webp")

		// Apple iOS 14.3 is lying. It claims it can support WebP and sends
		// an Accept: image/webp,... however it doesn't render the WebP
		// correctly at all.
		// XXX: https://github.com/jointwt/twtxt/issues/337 for details
		if preferredContentType == "image/webp" && strings.Contains(r.UserAgent(), "iPhone OS 14_3") {
			preferredContentType = "image/png"
		}

		var fn string

		if preferredContentType == "image/webp" {
			fn = filepath.Join(s.config.Data, avatarsDir, fmt.Sprintf("%s.webp", nick))
			w.Header().Set("Content-Type", "image/webp")
		} else {
			// Support older browsers like IE11 that don't support WebP :/
			metrics.Counter("media", "old_avatar").Inc()
			fn = filepath.Join(s.config.Data, avatarsDir, fmt.Sprintf("%s.png", nick))
			w.Header().Set("Content-Type", "image/png")
		}

		if fileInfo, err := os.Stat(fn); err == nil {
			etag := fmt.Sprintf("W/\"%s-%s\"", r.RequestURI, fileInfo.ModTime().Format(time.RFC3339))

			if match := r.Header.Get("If-None-Match"); match != "" {
				if strings.Contains(match, etag) {
					w.WriteHeader(http.StatusNotModified)
					return
				}
			}

			w.Header().Set("Etag", etag)
			if r.Method == http.MethodHead {
				return
			}

			f, err := os.Open(fn)
			if err != nil {
				log.WithError(err).Error("error opening avatar file")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			defer f.Close()

			if _, err := io.Copy(w, f); err != nil {
				log.WithError(err).Error("error writing avatar response")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			return
		}

		etag := fmt.Sprintf("W/\"%s\"", r.RequestURI)

		if match := r.Header.Get("If-None-Match"); match != "" {
			if strings.Contains(match, etag) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}

		w.Header().Set("Etag", etag)
		if r.Method == http.MethodHead {
			return
		}

		img, err := GenerateAvatar(s.config, nick)
		if err != nil {
			log.WithError(err).Errorf("error generating avatar for %s", nick)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if preferredContentType == "image/webp" {
			w.Header().Set("Content-Type", "image/webp")
			if r.Method == http.MethodHead {
				return
			}
			if err := webp.Encode(w, img, &webp.Options{Lossless: true}); err != nil {
				log.WithError(err).Error("error encoding auto generated avatar")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			return
		}

		if r.Method == http.MethodHead {
			return
		}

		// Support older browsers like IE11 that don't support WebP :/
		metrics.Counter("media", "old_avatar").Inc()
		w.Header().Set("Content-Type", "image/png")
		if err := png.Encode(w, img); err != nil {
			log.WithError(err).Error("error encoding auto generated avatar")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}
}

// TwtxtHandler ...
func (s *Server) TwtxtHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
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

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
		w.Header().Set("Link", fmt.Sprintf(`<%s/user/%s/webmention>; rel="webmention"`, s.config.BaseURL, nick))
		w.Header().Set("Last-Modified", fileInfo.ModTime().UTC().Format(http.TimeFormat))

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

		f, err := os.Open(fn)
		if err != nil {
			log.WithError(err).Error("error opening feed")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		defer f.Close()

		if r.Method == http.MethodHead {
			return
		}

		http.ServeContent(w, r, filepath.Base(fn), fileInfo.ModTime(), f)
	}
}

// PostHandler ...
func (s *Server) PostHandler() httprouter.Handle {
	isLocalURL := IsLocalURLFactory(s.config)
	isExternalFeed := IsExternalFeedFactory(s.config)
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		postas := strings.ToLower(strings.TrimSpace(r.FormValue("postas")))

		// TODO: Support deleting/patching last feed (`postas`) twt too.
		if r.Method == http.MethodDelete || r.Method == http.MethodPatch {
			if err := DeleteLastTwt(s.config, ctx.User); err != nil {
				ctx.Error = true
				ctx.Message = "Error deleting last twt"
				s.render("error", w, ctx)
			}

			// Update user's own timeline with their own new post.
			s.cache.FetchTwts(s.config, s.archive, ctx.User.Source(), nil)

			// Re-populate/Warm cache with local twts for this pod
			s.cache.GetByPrefix(s.config.BaseURL, true)

			if r.Method != http.MethodDelete {
				return
			}
		}

		hash := r.FormValue("hash")
		lastTwt, _, err := GetLastTwt(s.config, ctx.User)
		if err != nil {
			ctx.Error = true
			ctx.Message = "Error deleting last twt"
			s.render("error", w, ctx)
			return
		}

		if hash != "" && lastTwt.Hash() == hash {
			if err := DeleteLastTwt(s.config, ctx.User); err != nil {
				ctx.Error = true
				ctx.Message = "Error deleting last twt"
				s.render("error", w, ctx)
			}
		} else {
			log.Warnf("hash mismatch %s != %s", lastTwt.Hash(), hash)
		}

		text := CleanTwt(r.FormValue("text"))

		if text == "" {
			ctx.Error = true
			ctx.Message = "No post content provided!"
			s.render("error", w, ctx)
			return
		}

		reply := strings.TrimSpace(r.FormValue("reply"))
		if reply != "" {
			re := regexp.MustCompile(`^(@<.*>[, ]*)*(\(.*?\))(.*)`)
			match := re.FindStringSubmatch(text)
			if match == nil {
				text = fmt.Sprintf("(%s) %s", reply, text)
			}
		}

		user, err := s.db.GetUser(ctx.Username)
		if err != nil {
			log.WithError(err).Errorf("error loading user object for %s", ctx.Username)
			ctx.Error = true
			ctx.Message = "Error posting twt"
			s.render("error", w, ctx)
			return
		}

		var twt types.Twt = types.NilTwt

		switch postas {
		case "", user.Username:
			if hash != "" && lastTwt.Hash() == hash {
				twt, err = AppendTwt(s.config, s.db, user, text, lastTwt.Created)
			} else {
				twt, err = AppendTwt(s.config, s.db, user, text)
			}
		default:
			if user.OwnsFeed(postas) {
				if hash != "" && lastTwt.Hash() == hash {
					twt, err = AppendSpecial(s.config, s.db, postas, text, lastTwt.Created)
				} else {
					twt, err = AppendSpecial(s.config, s.db, postas, text)
				}
			} else {
				err = ErrFeedImposter
			}
		}

		if err != nil {
			log.WithError(err).Error("error posting twt")
			ctx.Error = true
			ctx.Message = "Error posting twt"
			s.render("error", w, ctx)
			return
		}

		// Update user's own timeline with their own new post.
		s.cache.FetchTwts(s.config, s.archive, user.Source(), nil)

		// Re-populate/Warm cache with local twts for this pod
		s.cache.GetByPrefix(s.config.BaseURL, true)

		// WebMentions ...
		for _, m := range twt.Mentions() {
			twter := m.Twter()
			if !isLocalURL(twter.URL) || isExternalFeed(twter.URL) {
				if err := WebMention(twter.URL, URLForTwt(s.config.BaseURL, twt.Hash())); err != nil {
					log.WithError(err).Warnf("error sending webmention to %s", twter.URL)
				}
			}
		}

		http.Redirect(w, r, "/", http.StatusFound)
	}
}

// TimelineHandler ...
func (s *Server) TimelineHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.Method == http.MethodHead {
			defer r.Body.Close()
			return
		}

		ctx := NewContext(s.config, s.db, r)

		var twts types.Twts

		if !ctx.Authenticated {
			twts = s.cache.GetByPrefix(s.config.BaseURL, false)
			ctx.Title = "Local timeline"
		} else {
			ctx.Title = "Timeline"
			user := ctx.User
			if user != nil {
				for feed := range user.Sources() {
					twts = append(twts, s.cache.GetByURL(feed.URL)...)
				}
			}
			sort.Sort(twts)
		}

		var pagedTwts types.Twts

		page := SafeParseInt(r.FormValue("p"), 1)
		pager := paginator.New(adapter.NewSliceAdapter(twts), s.config.TwtsPerPage)
		pager.SetPage(page)

		if err := pager.Results(&pagedTwts); err != nil {
			log.WithError(err).Error("error sorting and paging twts")
			ctx.Error = true
			ctx.Message = "An error occurred while loading the timeline"
			s.render("error", w, ctx)
			return
		}

		if ctx.Authenticated {
			lastTwt, _, err := GetLastTwt(s.config, ctx.User)
			if err != nil {
				log.WithError(err).Error("error getting user last twt")
				ctx.Error = true
				ctx.Message = "An error occurred while loading the timeline"
				s.render("error", w, ctx)
				return
			}
			ctx.LastTwt = lastTwt
		}

		ctx.Twts = FilterTwts(ctx.User, pagedTwts)
		ctx.Pager = &pager

		s.render("timeline", w, ctx)
	}
}

// WebMentionHandler ...
func (s *Server) WebMentionHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		defer r.Body.Close()
		webmentions.WebMentionEndpoint(w, r)
	}
}

// PermalinkHandler ...
func (s *Server) PermalinkHandler() httprouter.Handle {
	isLocal := IsLocalURLFactory(s.config)

	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		hash := p.ByName("hash")
		if hash == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		var err error

		twt, ok := s.cache.Lookup(hash)
		if !ok {
			// If the twt is not in the cache look for it in the archive
			if s.archive.Has(hash) {
				twt, err = s.archive.Get(hash)
				if err != nil {
					ctx.Error = true
					ctx.Message = "Error loading twt from archive, please try again"
					s.render("error", w, ctx)
					return
				}
			}
		}

		if twt.IsZero() {
			ctx.Error = true
			ctx.Message = "No matching twt found!"
			s.render("404", w, ctx)
			return
		}

		var (
			who   string
			image string
		)

		if isLocal(twt.Twter().URL) {
			who = fmt.Sprintf("%s@%s", twt.Twter().Nick, s.config.baseURL.Hostname())
			image = URLForAvatar(s.config, twt.Twter().Nick)
		} else {
			who = fmt.Sprintf("@<%s %s>", twt.Twter().Nick, twt.Twter().URL)
			image = URLForExternalAvatar(s.config, twt.Twter().URL)
		}

		when := twt.Created().Format(time.RFC3339)
		what := twt.FormatText(types.TextFmt, s.config)

		var ks []string
		if ks, err = keywords.Extract(what); err != nil {
			log.WithError(err).Warn("error extracting keywords")
		}

		for _, m := range twt.Mentions() {
			ks = append(ks, m.Twter().Nick)
		}
		var tags types.TagList = twt.Tags()
		ks = append(ks, tags.Tags()...)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Last-Modified", twt.Created().Format(http.TimeFormat))
		if strings.HasPrefix(twt.Twter().URL, s.config.BaseURL) {
			w.Header().Set(
				"Link",
				fmt.Sprintf(
					`<%s/user/%s/webmention>; rel="webmention"`,
					s.config.BaseURL, twt.Twter().Nick,
				),
			)
		}

		if r.Method == http.MethodHead {
			defer r.Body.Close()
			return
		}

		title := fmt.Sprintf("%s \"%s\"", who, what)

		ctx.Title = title
		ctx.Meta = Meta{
			Title:       fmt.Sprintf("Twt #%s", twt.Hash()),
			Description: what,
			UpdatedAt:   when,
			Author:      who,
			Image:       image,
			URL:         URLForTwt(s.config.BaseURL, hash),
			Keywords:    strings.Join(ks, ", "),
		}
		if strings.HasPrefix(twt.Twter().URL, s.config.BaseURL) {
			ctx.Links = append(ctx.Links, types.Link{
				Href: fmt.Sprintf("%s/webmention", UserURL(twt.Twter().URL)),
				Rel:  "webmention",
			})
			ctx.Alternatives = append(ctx.Alternatives, types.Alternatives{
				types.Alternative{
					Type:  "text/plain",
					Title: fmt.Sprintf("%s's Twtxt Feed", twt.Twter().Nick),
					URL:   twt.Twter().URL,
				},
				types.Alternative{
					Type:  "application/atom+xml",
					Title: fmt.Sprintf("%s's Atom Feed", twt.Twter().Nick),
					URL:   fmt.Sprintf("%s/atom.xml", UserURL(twt.Twter().URL)),
				},
			}...)
		}

		ctx.Twts = FilterTwts(ctx.User, types.Twts{twt})
		s.render("permalink", w, ctx)

	}
}

// DiscoverHandler ...
func (s *Server) DiscoverHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		localTwts := s.cache.GetByPrefix(s.config.BaseURL, false)

		var pagedTwts types.Twts

		page := SafeParseInt(r.FormValue("p"), 1)
		pager := paginator.New(adapter.NewSliceAdapter(localTwts), s.config.TwtsPerPage)
		pager.SetPage(page)

		if err := pager.Results(&pagedTwts); err != nil {
			log.WithError(err).Error("error sorting and paging twts")
			ctx.Error = true
			ctx.Message = "An error occurred while loading the timeline"
			s.render("error", w, ctx)
			return
		}

		if ctx.Authenticated {
			lastTwt, _, err := GetLastTwt(s.config, ctx.User)
			if err != nil {
				log.WithError(err).Error("error getting user last twt")
				ctx.Error = true
				ctx.Message = "An error occurred while loading the timeline"
				s.render("error", w, ctx)
				return
			}
			ctx.LastTwt = lastTwt
		}

		ctx.Title = "Local timeline"
		ctx.Twts = FilterTwts(ctx.User, pagedTwts)
		ctx.Pager = &pager

		s.render("timeline", w, ctx)
	}
}

// MentionsHandler ...
func (s *Server) MentionsHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		twts := s.cache.GetMentions(ctx.User)
		sort.Sort(twts)

		var pagedTwts types.Twts

		page := SafeParseInt(r.FormValue("p"), 1)
		pager := paginator.New(adapter.NewSliceAdapter(twts), s.config.TwtsPerPage)
		pager.SetPage(page)

		if err := pager.Results(&pagedTwts); err != nil {
			ctx.Error = true
			ctx.Message = "An error occurred while loading mentions"
			s.render("error", w, ctx)
			return
		}

		ctx.Title = "Mentions"
		ctx.Twts = FilterTwts(ctx.User, pagedTwts)
		ctx.Pager = &pager
		s.render("timeline", w, ctx)
	}
}

// SearchHandler ...
func (s *Server) SearchHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		var twts types.Twts

		tag := r.URL.Query().Get("tag")

		if tag == "" {
			ctx.Error = true
			ctx.Message = "At least search query is required"
			s.render("error", w, ctx)
		}

		getTweetsByTag := func() types.Twts {
			var result types.Twts
			seen := make(map[string]bool)
			// TODO: Improve this by making this an O(1) lookup on the tag
			for _, twt := range s.cache.GetAll() {
				var tags types.TagList = twt.Tags()
				if HasString(UniqStrings(tags.Tags()), tag) && !seen[twt.Hash()] {
					result = append(result, twt)
					seen[twt.Hash()] = true
				}
			}
			return result
		}

		twts = getTweetsByTag()
		sort.Sort(twts)

		var pagedTwts types.Twts

		page := SafeParseInt(r.FormValue("p"), 1)
		pager := paginator.New(adapter.NewSliceAdapter(twts), s.config.TwtsPerPage)
		pager.SetPage(page)

		if err := pager.Results(&pagedTwts); err != nil {
			ctx.Error = true
			ctx.Message = "An error occurred while loading search results"
			s.render("error", w, ctx)
			return
		}

		ctx.Twts = FilterTwts(ctx.User, pagedTwts)
		ctx.Pager = &pager

		s.render("timeline", w, ctx)
	}
}

// FeedHandler ...
func (s *Server) FeedHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		name := NormalizeFeedName(r.FormValue("name"))

		if err := ValidateFeedName(s.config.Data, name); err != nil {
			ctx.Error = true
			ctx.Message = fmt.Sprintf("Invalid feed name: %s", err.Error())
			s.render("error", w, ctx)
			return
		}

		if err := CreateFeed(s.config, s.db, ctx.User, name, false); err != nil {
			ctx.Error = true
			ctx.Message = fmt.Sprintf("Error creating: %s", err.Error())
			s.render("error", w, ctx)
			return
		}

		ctx.User.Follow(name, URLForUser(s.config, name))

		if err := s.db.SetUser(ctx.Username, ctx.User); err != nil {
			ctx.Error = true
			ctx.Message = fmt.Sprintf("Error creating feed: %s", err.Error())
			s.render("error", w, ctx)
			return
		}

		if _, err := AppendSpecial(
			s.config, s.db,
			twtxtBot,
			fmt.Sprintf(
				"FEED: @<%s %s> from @<%s %s>",
				name, URLForUser(s.config, name),
				ctx.User.Username, URLForUser(s.config, ctx.User.Username),
			),
		); err != nil {
			log.WithError(err).Warnf("error appending special FOLLOW post")
		}

		ctx.Error = false
		ctx.Message = fmt.Sprintf("Successfully created feed: %s", name)
		s.render("error", w, ctx)

	}
}

// FeedsHandler ...
func (s *Server) FeedsHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		feeds, err := s.db.GetAllFeeds()
		if err != nil {
			ctx.Error = true
			ctx.Message = "An error occurred while loading feeds"
			s.render("error", w, ctx)
			return
		}

		feedsources, err := LoadFeedSources(s.config.Data)
		if err != nil {
			ctx.Error = true
			ctx.Message = "An error occurred while loading feeds"
			s.render("error", w, ctx)
			return
		}

		ctx.Title = "Feeds"
		ctx.Feeds = feeds
		ctx.FeedSources = feedsources.Sources

		s.render("feeds", w, ctx)
	}
}

// LoginHandler ...
func (s *Server) LoginHandler() httprouter.Handle {
	// #239: Throttle failed login attempts and lock user  account.
	failures := NewTTLCache(5 * time.Minute)

	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		if r.Method == "GET" {
			s.render("login", w, ctx)
			return
		}

		username := NormalizeUsername(r.FormValue("username"))
		password := r.FormValue("password")
		rememberme := r.FormValue("rememberme") == "on"

		// Error: no username or password provided
		if username == "" || password == "" {
			log.Warn("no username or password provided")
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Lookup user
		user, err := s.db.GetUser(username)
		if err != nil {
			ctx.Error = true
			ctx.Message = "Invalid username! Hint: Register an account?"
			s.render("error", w, ctx)
			return
		}

		// #239: Throttle failed login attempts and lock user  account.
		if failures.Get(user.Username) > MaxFailedLogins {
			ctx.Error = true
			ctx.Message = "Too many failed login attempts. Account temporarily locked! Please try again later."
			s.render("error", w, ctx)
			return
		}

		// Validate cleartext password against KDF hash
		err = s.pm.CheckPassword(user.Password, password)
		if err != nil {
			// #239: Throttle failed login attempts and lock user  account.
			failed := failures.Inc(user.Username)
			time.Sleep(time.Duration(IntPow(2, failed)) * time.Second)

			ctx.Error = true
			ctx.Message = "Invalid password! Hint: Reset your password?"
			s.render("error", w, ctx)
			return
		}

		// #239: Throttle failed login attempts and lock user  account.
		failures.Reset(user.Username)

		// Login successful
		log.Infof("login successful: %s", username)

		// Lookup session
		sess := r.Context().Value(session.SessionKey)
		if sess == nil {
			log.Warn("no session found")
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Authorize session
		_ = sess.(*session.Session).Set("username", username)

		// Persist session?
		if rememberme {
			_ = sess.(*session.Session).Set("persist", "1")
		}

		http.Redirect(w, r, RedirectRefererURL(r, s.config, "/"), http.StatusFound)
	}
}

// LogoutHandler ...
func (s *Server) LogoutHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		s.sm.Delete(w, r)
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

// RegisterHandler ...
func (s *Server) RegisterHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		if r.Method == "GET" {
			if s.config.OpenRegistrations {
				s.render("register", w, ctx)
			} else {
				message := s.config.RegisterMessage

				if message == "" {
					message = "Open Registrations are disabled on this pod. Please contact the pod operator."
				}

				ctx.Error = true
				ctx.Message = message
				s.render("error", w, ctx)
			}

			return
		}

		username := NormalizeUsername(r.FormValue("username"))
		password := r.FormValue("password")
		// XXX: We DO NOT store this! (EVER)
		email := strings.TrimSpace(r.FormValue("email"))

		if err := ValidateUsername(username); err != nil {
			ctx.Error = true
			ctx.Message = fmt.Sprintf("Username validation failed: %s", err.Error())
			s.render("error", w, ctx)
			return
		}

		if s.db.HasUser(username) || s.db.HasFeed(username) {
			ctx.Error = true
			ctx.Message = "User or Feed with that name already exists! Please pick another!"
			s.render("error", w, ctx)
			return
		}

		p := filepath.Join(s.config.Data, feedsDir)
		if err := os.MkdirAll(p, 0755); err != nil {
			log.WithError(err).Error("error creating feeds directory")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		fn := filepath.Join(p, username)
		if _, err := os.Stat(fn); err == nil {
			ctx.Error = true
			ctx.Message = "Deleted user with that username already exists! Please pick another!"
			s.render("error", w, ctx)
			return
		}

		if err := ioutil.WriteFile(fn, []byte{}, 0644); err != nil {
			log.WithError(err).Error("error creating new user feed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		hash, err := s.pm.CreatePassword(password)
		if err != nil {
			log.WithError(err).Error("error creating password hash")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		recoveryHash := fmt.Sprintf("email:%s", FastHash(email))

		user := NewUser()
		user.Username = username
		user.Password = hash
		user.Recovery = recoveryHash
		user.URL = URLForUser(s.config, username)
		user.CreatedAt = time.Now()

		if err := s.db.SetUser(username, user); err != nil {
			log.WithError(err).Error("error saving user object for new user")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Infof("user registered: %v", user)
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

// LookupHandler ...
func (s *Server) LookupHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		prefix := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("prefix")))

		feeds := s.db.SearchFeeds(prefix)

		user := ctx.User

		var following []string
		if len(prefix) > 0 {
			for nick := range user.Following {
				if strings.HasPrefix(strings.ToLower(nick), prefix) {
					following = append(following, nick)
				}
			}
		} else {
			following = append(following, StringKeys(user.Following)...)
		}

		var matches []string

		matches = append(matches, feeds...)
		matches = append(matches, following...)

		matches = UniqStrings(matches)

		data, err := json.Marshal(matches)
		if err != nil {
			log.WithError(err).Error("error serializing lookup response")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}
}

// SettingsHandler ...
func (s *Server) SettingsHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		if r.Method == "GET" {
			ctx.Title = "Settings"
			s.render("settings", w, ctx)
			return
		}

		// Limit request body to to abuse
		r.Body = http.MaxBytesReader(w, r.Body, s.config.MaxUploadSize)
		defer r.Body.Close()

		// XXX: We DO NOT store this! (EVER)
		email := strings.TrimSpace(r.FormValue("email"))
		tagline := strings.TrimSpace(r.FormValue("tagline"))
		password := r.FormValue("password")

		theme := r.FormValue("theme")
		displayDatesInTimezone := r.FormValue("displayDatesInTimezone")
		isFollowersPubliclyVisible := r.FormValue("isFollowersPubliclyVisible") == "on"
		isFollowingPubliclyVisible := r.FormValue("isFollowingPubliclyVisible") == "on"
		isBookmarksPubliclyVisible := r.FormValue("isBookmarksPubliclyVisible") == "on"

		avatarFile, _, err := r.FormFile("avatar_file")
		if err != nil && err != http.ErrMissingFile {
			log.WithError(err).Error("error parsing form file")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		user := ctx.User
		if user == nil {
			log.Fatalf("user not found in context")
		}

		if password != "" {
			hash, err := s.pm.CreatePassword(password)
			if err != nil {
				log.WithError(err).Error("error creating password hash")
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			user.Password = hash
		}

		if avatarFile != nil {
			opts := &ImageOptions{
				Resize: true,
				Width:  AvatarResolution,
				Height: AvatarResolution,
			}
			_, err = StoreUploadedImage(
				s.config, avatarFile,
				avatarsDir, ctx.Username,
				opts,
			)
			if err != nil {
				ctx.Error = true
				ctx.Message = fmt.Sprintf("Error updating user: %s", err)
				s.render("error", w, ctx)
				return
			}
		}

		recoveryHash := fmt.Sprintf("email:%s", FastHash(email))

		user.Recovery = recoveryHash
		user.Tagline = tagline

		user.Theme = theme
		user.DisplayDatesInTimezone = displayDatesInTimezone
		user.IsFollowersPubliclyVisible = isFollowersPubliclyVisible
		user.IsFollowingPubliclyVisible = isFollowingPubliclyVisible
		user.IsBookmarksPubliclyVisible = isBookmarksPubliclyVisible

		if err := s.db.SetUser(ctx.Username, user); err != nil {
			ctx.Error = true
			ctx.Message = "Error updating user"
			s.render("error", w, ctx)
			return
		}

		ctx.Error = false
		ctx.Message = "Successfully updated settings"
		s.render("error", w, ctx)

	}
}

// DeleteTokenHandler ...
func (s *Server) DeleteTokenHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		signature := p.ByName("signature")

		if err := s.db.DelToken(signature); err != nil {
			ctx.Error = true
			ctx.Message = "Error deleting token"
			s.render("error", w, ctx)
			return
		}

		ctx.Error = false
		ctx.Message = "Successfully deleted token"

		http.Redirect(w, r, "/settings", http.StatusFound)

	}
}

// FollowersHandler ...
func (s *Server) FollowersHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		nick := NormalizeUsername(p.ByName("nick"))

		if s.db.HasUser(nick) {
			user, err := s.db.GetUser(nick)
			if err != nil {
				log.WithError(err).Errorf("error loading user object for %s", nick)
				ctx.Error = true
				ctx.Message = "Error loading profile"
				s.render("error", w, ctx)
				return
			}

			if !user.IsFollowersPubliclyVisible && !ctx.User.Is(user.URL) {
				s.render("401", w, ctx)
				return
			}
			ctx.Profile = user.Profile(s.config.BaseURL, ctx.User)
		} else if s.db.HasFeed(nick) {
			feed, err := s.db.GetFeed(nick)
			if err != nil {
				log.WithError(err).Errorf("error loading feed object for %s", nick)
				ctx.Error = true
				ctx.Message = "Error loading profile"
				s.render("error", w, ctx)
				return
			}
			ctx.Profile = feed.Profile(s.config.BaseURL, ctx.User)
		} else {
			ctx.Error = true
			ctx.Message = "User or Feed Not Found"
			s.render("404", w, ctx)
			return
		}

		if r.Header.Get("Accept") == "application/json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			if err := json.NewEncoder(w).Encode(ctx.Profile.Followers); err != nil {
				log.WithError(err).Error("error encoding user for display")
				http.Error(w, "Bad Request", http.StatusBadRequest)
			}

			return
		}

		ctx.Title = fmt.Sprintf("Followers for %s", nick)
		s.render("followers", w, ctx)
	}
}

// FollowingHandler ...
func (s *Server) FollowingHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		nick := NormalizeUsername(p.ByName("nick"))

		if s.db.HasUser(nick) {
			user, err := s.db.GetUser(nick)
			if err != nil {
				log.WithError(err).Errorf("error loading user object for %s", nick)
				ctx.Error = true
				ctx.Message = "Error loading profile"
				s.render("error", w, ctx)
				return
			}

			if !user.IsFollowingPubliclyVisible && !ctx.User.Is(user.URL) {
				s.render("401", w, ctx)
				return
			}
			ctx.Profile = user.Profile(s.config.BaseURL, ctx.User)
		} else {
			ctx.Error = true
			ctx.Message = "User Not Found"
			s.render("404", w, ctx)
			return
		}

		if r.Header.Get("Accept") == "application/json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			if err := json.NewEncoder(w).Encode(ctx.Profile.Followers); err != nil {
				log.WithError(err).Error("error encoding user for display")
				http.Error(w, "Bad Request", http.StatusBadRequest)
			}

			return
		}

		ctx.Title = fmt.Sprintf("Users following %s", nick)
		s.render("following", w, ctx)
	}
}

// ExternalHandler ...
func (s *Server) ExternalHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		uri := r.URL.Query().Get("uri")
		nick := r.URL.Query().Get("nick")

		if uri == "" {
			ctx.Error = true
			ctx.Message = "Cannot find external feed"
			s.render("error", w, ctx)
			return
		}

		if nick == "" {
			log.Warn("no nick given to external profile request")
			nick = "unknown"
		}

		if !s.cache.IsCached(uri) {
			sources := make(types.Feeds)
			sources[types.Feed{Nick: nick, URL: uri}] = true
			s.cache.FetchTwts(s.config, s.archive, sources, nil)
		}

		twts := s.cache.GetByURL(uri)

		var pagedTwts types.Twts

		page := SafeParseInt(r.FormValue("p"), 1)
		pager := paginator.New(adapter.NewSliceAdapter(twts), s.config.TwtsPerPage)
		pager.SetPage(page)

		if err := pager.Results(&pagedTwts); err != nil {
			log.WithError(err).Error("error sorting and paging twts")
			ctx.Error = true
			ctx.Message = "An error occurred while loading the timeline"
			s.render("error", w, ctx)
			return
		}

		ctx.Twts = FilterTwts(ctx.User, pagedTwts)
		ctx.Pager = &pager

		if len(ctx.Twts) > 0 {
			ctx.Twter = ctx.Twts[0].Twter()
		} else {
			ctx.Twter = types.Twter{Nick: nick, URL: uri}
			avatar := GetExternalAvatar(s.config, nick, uri)
			if avatar != "" {
				ctx.Twter.Avatar = URLForExternalAvatar(s.config, uri)
			}
		}

		ctx.Profile = types.Profile{
			Username: nick,
			TwtURL:   uri,
			URL:      URLForExternalProfile(s.config, nick, uri),

			Follows:    ctx.User.Follows(uri),
			FollowedBy: ctx.User.FollowedBy(uri),
			Muted:      ctx.User.HasMuted(uri),
		}

		ctx.Title = fmt.Sprintf("External profile for @<%s %s>", nick, uri)
		s.render("externalProfile", w, ctx)
	}
}

// ExternalAvatarHandler ...
func (s *Server) ExternalAvatarHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		w.Header().Set("Cache-Control", "public, no-cache, must-revalidate")

		uri := r.URL.Query().Get("uri")

		if uri == "" {
			log.Warn("no uri provided for external avatar")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		slug := Slugify(uri)

		var fn string

		preferredContentType := accept.PreferredContentTypeLike(r.Header, "image/webp")

		// Apple iOS 14.3 is lying. It claims it can support WebP and sends
		// an Accept: image/webp,... however it doesn't render the WebP
		// correctly at all.
		// XXX: https://github.com/jointwt/twtxt/issues/337 for details
		if preferredContentType == "image/webp" && strings.Contains(r.UserAgent(), "iPhone OS 14_3") {
			preferredContentType = "image/png"
		}

		if preferredContentType == "image/webp" {
			fn = filepath.Join(s.config.Data, externalDir, fmt.Sprintf("%s.webp", slug))
			w.Header().Set("Content-Type", "image/webp")
		} else {
			// Support older browsers like IE11 that don't support WebP :/
			metrics.Counter("media", "old_avatar").Inc()
			fn = filepath.Join(s.config.Data, externalDir, fmt.Sprintf("%s.png", slug))
			w.Header().Set("Content-Type", "image/png")
		}

		if !FileExists(fn) {
			log.Warnf("no external avatar found for %s", slug)
			http.Error(w, "External avatar not found", http.StatusNotFound)
			return
		}

		fileInfo, err := os.Stat(fn)
		if err != nil {
			log.WithError(err).Error("os.Stat() error")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		etag := fmt.Sprintf("W/\"%s-%s\"", r.RequestURI, fileInfo.ModTime().Format(time.RFC3339))

		if match := r.Header.Get("If-None-Match"); match != "" {
			if strings.Contains(match, etag) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}

		w.Header().Set("Etag", etag)
		if r.Method == http.MethodHead {
			return
		}

		if r.Method == http.MethodHead {
			return
		}

		f, err := os.Open(fn)
		if err != nil {
			log.WithError(err).Error("error opening avatar file")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		defer f.Close()

		if _, err := io.Copy(w, f); err != nil {
			log.WithError(err).Error("error writing avatar response")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}
}

// ResetPasswordHandler ...
func (s *Server) ResetPasswordHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		if r.Method == "GET" {
			ctx.Title = "Reset password"
			s.render("resetPassword", w, ctx)
			return
		}

		username := NormalizeUsername(r.FormValue("username"))
		email := strings.TrimSpace(r.FormValue("email"))
		recovery := fmt.Sprintf("email:%s", FastHash(email))

		if err := ValidateUsername(username); err != nil {
			ctx.Error = true
			ctx.Message = fmt.Sprintf("Username validation failed: %s", err.Error())
			s.render("error", w, ctx)
			return
		}

		// Check if user exist
		if !s.db.HasUser(username) {
			ctx.Error = true
			ctx.Message = "User not found!"
			s.render("error", w, ctx)
			return
		}

		// Get user object from DB
		user, err := s.db.GetUser(username)
		if err != nil {
			ctx.Error = true
			ctx.Message = "Error loading user"
			s.render("error", w, ctx)
			return
		}

		if recovery != user.Recovery {
			ctx.Error = true
			ctx.Message = "Error! The email address you supplied does not match what you registered with :/"
			s.render("error", w, ctx)
			return
		}

		// Create magic link expiry time
		now := time.Now()
		secs := now.Unix()
		expiresAfterSeconds := int64(600) // Link expires after 10 minutes

		expiryTime := secs + expiresAfterSeconds

		// Create magic link
		token := jwt.NewWithClaims(
			jwt.SigningMethodHS256,
			jwt.MapClaims{"username": username, "expiresAt": expiryTime},
		)
		tokenString, err := token.SignedString([]byte(s.config.MagicLinkSecret))
		if err != nil {
			ctx.Error = true
			ctx.Message = err.Error()
			s.render("error", w, ctx)
			return
		}

		if err := SendPasswordResetEmail(s.config, user, email, tokenString); err != nil {
			log.WithError(err).Errorf("unable to send reset password email to %s", user.Username)
			ctx.Error = true
			ctx.Message = err.Error()
			s.render("error", w, ctx)
			return
		}

		log.Infof("reset password email sent for %s", user.Username)

		// Show success msg
		ctx.Error = false
		ctx.Message = "Password request request sent! Please check your email and follow the instructions"
		s.render("error", w, ctx)
	}
}

// ResetPasswordMagicLinkHandler ...
func (s *Server) ResetPasswordMagicLinkHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		// Get token from query string
		tokens, ok := r.URL.Query()["token"]

		// Check if valid token
		if !ok || len(tokens[0]) < 1 {
			ctx.Error = true
			ctx.Message = "Invalid token"
			s.render("error", w, ctx)
			return
		}

		tokenEmail := tokens[0]
		ctx.PasswordResetToken = tokenEmail

		// Show newPassword page
		s.render("newPassword", w, ctx)
	}
}

// NewPasswordHandler ...
func (s *Server) NewPasswordHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		if r.Method == "GET" {
			return
		}

		password := r.FormValue("password")
		tokenEmail := r.FormValue("token")

		// Check if token is valid
		token, err := jwt.Parse(tokenEmail, func(token *jwt.Token) (interface{}, error) {

			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
			}

			return []byte(s.config.MagicLinkSecret), nil
		})

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {

			var username = fmt.Sprintf("%v", claims["username"])
			var expiresAt int = int(claims["expiresAt"].(float64))

			now := time.Now()
			secs := now.Unix()

			// Check token expiry
			if secs > int64(expiresAt) {
				ctx.Error = true
				ctx.Message = "Token expires"
				s.render("error", w, ctx)
				return
			}

			user, err := s.db.GetUser(username)
			if err != nil {
				ctx.Error = true
				ctx.Message = "Error loading user"
				s.render("error", w, ctx)
				return
			}

			// Reset password
			if password != "" {
				hash, err := s.pm.CreatePassword(password)
				if err != nil {
					ctx.Error = true
					ctx.Message = "Error loading user"
					s.render("error", w, ctx)
					return
				}

				user.Password = hash

				// Save user
				if err := s.db.SetUser(username, user); err != nil {
					ctx.Error = true
					ctx.Message = "Error loading user"
					s.render("error", w, ctx)
					return
				}
			}

			log.Infof("password changed: %v", user)

			// Show success msg
			ctx.Error = false
			ctx.Message = "Password reset successfully."
			s.render("error", w, ctx)
		} else {
			ctx.Error = true
			ctx.Message = err.Error()
			s.render("error", w, ctx)
			return
		}
	}
}

// TaskHandler ...
func (s *Server) TaskHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		uuid := p.ByName("uuid")

		if uuid == "" {
			log.Warn("no task uuid provided")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		t, ok := s.tasks.Lookup(uuid)
		if !ok {
			log.Warnf("no task found by uuid: %s", uuid)
			http.Error(w, "Task Not Found", http.StatusNotFound)
			return
		}

		data, err := json.Marshal(t.Result())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)

	}
}

// SyndicationHandler ...
func (s *Server) SyndicationHandler() httprouter.Handle {
	formatTwt := FormatTwtFactory(s.config)

	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		var (
			twts    types.Twts
			profile types.Profile
			err     error
		)

		nick := NormalizeUsername(p.ByName("nick"))
		if nick != "" {
			if s.db.HasUser(nick) {
				if user, err := s.db.GetUser(nick); err == nil {
					profile = user.Profile(s.config.BaseURL, nil)
					twts = s.cache.GetByURL(profile.URL)
				} else {
					log.WithError(err).Error("error loading user object")
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
			} else if s.db.HasFeed(nick) {
				if feed, err := s.db.GetFeed(nick); err == nil {
					profile = feed.Profile(s.config.BaseURL, nil)
					twts = s.cache.GetByURL(profile.URL)
				} else {
					log.WithError(err).Error("error loading user object")
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
			} else {
				http.Error(w, "Feed Not Found", http.StatusNotFound)
				return
			}
		} else {
			twts = s.cache.GetByPrefix(s.config.BaseURL, false)

			profile = types.Profile{
				Type:     "Local",
				Username: s.config.Name,
				Tagline:  "", // TODO: Maybe Twtxt Pods should have a configurable description?
				URL:      s.config.BaseURL,
			}
		}

		if err != nil {
			log.WithError(err).Errorf("errorloading feeds for %s", nick)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if r.Method == http.MethodHead {
			defer r.Body.Close()
			w.Header().Set(
				"Last-Modified",
				twts[len(twts)].Created().Format(http.TimeFormat),
			)
			return
		}

		now := time.Now()

		feed := &feeds.Feed{
			Title:       fmt.Sprintf("%s Twtxt Atom Feed", profile.Username),
			Link:        &feeds.Link{Href: profile.URL},
			Description: profile.Tagline,
			Author:      &feeds.Author{Name: profile.Username},
			Created:     now,
		}

		var items []*feeds.Item

		for _, twt := range twts {
			items = append(items, &feeds.Item{
				Id:          twt.Hash(),
				Title:       string(formatTwt(twt)),
				Link:        &feeds.Link{Href: URLForTwt(s.config.BaseURL, twt.Hash())},
				Author:      &feeds.Author{Name: twt.Twter().Nick},
				Description: string(formatTwt(twt)),
				Created:     twt.Created(),
			},
			)
		}
		feed.Items = items

		w.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")
		data, err := feed.ToAtom()
		if err != nil {
			log.WithError(err).Error("error serializing feed")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		_, _ = w.Write([]byte(data))
	}
}

// PodConfigHandler ...
func (s *Server) PodConfigHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		data, err := json.Marshal(s.config.Settings())
		if err != nil {
			log.WithError(err).Error("error serializing pod config response")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}
}

// TransferFeedHandler...
func (s *Server) TransferFeedHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)
		feedName := NormalizeFeedName(p.ByName("name"))
		transferToName := NormalizeFeedName(p.ByName("transferTo"))

		if feedName == "" {
			ctx.Error = true
			ctx.Message = "No feed specified"
			s.render("error", w, ctx)
			return
		}

		if transferToName == "" {
			// Get feed followers list
			if s.db.HasFeed(feedName) {
				feed, err := s.db.GetFeed(feedName)
				if err != nil {
					log.WithError(err).Errorf("Error loading feed object for %s", feedName)
					ctx.Error = true
					ctx.Message = "Error loading profile"
					s.render("error", w, ctx)
					return
				}

				ctx.Profile = feed.Profile(s.config.BaseURL, ctx.User)
				s.render("transferFeed", w, ctx)
				return
			}
		}

		// Get feed
		if s.db.HasFeed(feedName) {

			feed, err := s.db.GetFeed(feedName)
			if err != nil {
				log.WithError(err).Errorf("Error loading feed object for %s", feedName)
				ctx.Error = true
				ctx.Message = "Error loading profile"
				s.render("error", w, ctx)
				return
			}

			// Get FromUser
			fromUser, err := s.db.GetUser(ctx.User.Username)
			if err != nil {
				log.WithError(err).Errorf("Error loading user")
				ctx.Error = true
				ctx.Message = "Error loading user"
				s.render("error", w, ctx)
				return
			}

			// Get ToUser
			toUser, err := s.db.GetUser(transferToName)
			if err != nil {
				log.WithError(err).Errorf("Error loading user")
				ctx.Error = true
				ctx.Message = "Error loading user"
				s.render("error", w, ctx)
				return
			}

			// Transfer ownerships
			_ = RemoveFeedOwnership(s.db, fromUser, feed)
			_ = AddFeedOwnership(s.db, toUser, feed)

			ctx.Error = false
			ctx.Message = "Feed ownership changed successfully."
			s.render("error", w, ctx)
		}
	}
}

// DeleteAllHandler ...
func (s *Server) DeleteAllHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		// Get all user feeds
		feeds, err := s.db.GetAllFeeds()
		if err != nil {
			ctx.Error = true
			ctx.Message = "An error occured whilst deleting your account"
			s.render("error", w, ctx)
			return
		}

		for _, feed := range feeds {
			// Get user's owned feeds
			if ctx.User.OwnsFeed(feed.Name) {
				// Get twts in a feed
				nick := feed.Name
				if nick != "" {
					if s.db.HasFeed(nick) {
						// Fetch feed twts
						twts, err := GetAllTwts(s.config, nick)
						if err != nil {
							ctx.Error = true
							ctx.Message = "An error occured whilst deleting your account"
							s.render("error", w, ctx)
							return
						}

						// Parse twts to search and remove uploaded media
						for _, twt := range twts {
							// Delete archived twts
							if err := s.archive.Del(twt.Hash()); err != nil {
								ctx.Error = true
								ctx.Message = "An error occured whilst deleting your account"
								s.render("error", w, ctx)
								return
							}

							mediaPaths := GetMediaNamesFromText(fmt.Sprintf("%t", twt))

							// Remove all uploaded media in a twt
							for _, mediaPath := range mediaPaths {
								// Delete .png
								fn := filepath.Join(s.config.Data, mediaDir, fmt.Sprintf("%s.png", mediaPath))
								if FileExists(fn) {
									if err := os.Remove(fn); err != nil {
										ctx.Error = true
										ctx.Message = "An error occured whilst deleting your account"
										s.render("error", w, ctx)
										return
									}
								}

								// Delete .webp
								fn = filepath.Join(s.config.Data, mediaDir, fmt.Sprintf("%s.webp", mediaPath))
								if FileExists(fn) {
									if err := os.Remove(fn); err != nil {
										ctx.Error = true
										ctx.Message = "An error occured whilst deleting your account"
										s.render("error", w, ctx)
										return
									}
								}
							}
						}
					}
				}

				// Delete feed
				if err := s.db.DelFeed(nick); err != nil {
					ctx.Error = true
					ctx.Message = "An error occured whilst deleting your account"
					s.render("error", w, ctx)
					return
				}

				// Delete feeds's twtxt.txt
				fn := filepath.Join(s.config.Data, feedsDir, nick)
				if FileExists(fn) {
					if err := os.Remove(fn); err != nil {
						log.WithError(err).Error("error removing feed")
						ctx.Error = true
						ctx.Message = "An error occured whilst deleting your account"
						s.render("error", w, ctx)
					}
				}

				// Delete feed from cache
				s.cache.Delete(feed.Source())
			}
		}

		// Get user's primary feed twts
		twts, err := GetAllTwts(s.config, ctx.User.Username)
		if err != nil {
			ctx.Error = true
			ctx.Message = "An error occured whilst deleting your account"
			s.render("error", w, ctx)
			return
		}

		// Parse twts to search and remove primary feed uploaded media
		for _, twt := range twts {
			// Delete archived twts
			if err := s.archive.Del(twt.Hash()); err != nil {
				ctx.Error = true
				ctx.Message = "An error occured whilst deleting your account"
				s.render("error", w, ctx)
				return
			}

			mediaPaths := GetMediaNamesFromText(fmt.Sprintf("%t", twt))

			// Remove all uploaded media in a twt
			for _, mediaPath := range mediaPaths {
				// Delete .png
				fn := filepath.Join(s.config.Data, mediaDir, fmt.Sprintf("%s.png", mediaPath))
				if FileExists(fn) {
					if err := os.Remove(fn); err != nil {
						log.WithError(err).Error("error removing media")
						ctx.Error = true
						ctx.Message = "An error occured whilst deleting your account"
						s.render("error", w, ctx)
					}
				}

				// Delete .webp
				fn = filepath.Join(s.config.Data, mediaDir, fmt.Sprintf("%s.webp", mediaPath))
				if FileExists(fn) {
					if err := os.Remove(fn); err != nil {
						log.WithError(err).Error("error removing media")
						ctx.Error = true
						ctx.Message = "An error occured whilst deleting your account"
						s.render("error", w, ctx)
					}
				}
			}
		}

		// Delete user's primary feed
		if err := s.db.DelFeed(ctx.User.Username); err != nil {
			ctx.Error = true
			ctx.Message = "An error occured whilst deleting your account"
			s.render("error", w, ctx)
			return
		}

		// Delete user's twtxt.txt
		fn := filepath.Join(s.config.Data, feedsDir, ctx.User.Username)
		if FileExists(fn) {
			if err := os.Remove(fn); err != nil {
				log.WithError(err).Error("error removing user's feed")
				ctx.Error = true
				ctx.Message = "An error occured whilst deleting your account"
				s.render("error", w, ctx)
			}
		}

		// Delete user
		if err := s.db.DelUser(ctx.Username); err != nil {
			ctx.Error = true
			ctx.Message = "An error occured whilst deleting your account"
			s.render("error", w, ctx)
			return
		}

		// Delete user's feed from cache
		s.cache.Delete(ctx.User.Source())

		// Re-populate/Warm cache with local twts for this pod
		s.cache.GetByPrefix(s.config.BaseURL, true)

		s.sm.Delete(w, r)
		ctx.Authenticated = false

		ctx.Error = false
		ctx.Message = "Successfully deleted account"
		s.render("error", w, ctx)
	}
}

// DeleteAccountHandler ...
func (s *Server) DeleteAccountHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		feeds, err := s.db.GetAllFeeds()
		if err != nil {
			ctx.Error = true
			ctx.Message = "An error occurred while loading feeds"
			s.render("error", w, ctx)
			return
		}

		ctx.Feeds = feeds
		s.render("deleteAccount", w, ctx)
	}
}
