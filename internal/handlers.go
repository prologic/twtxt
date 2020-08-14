package internal

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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

	"github.com/aofei/cameron"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/feeds"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"github.com/vcraescu/go-paginator"
	"github.com/vcraescu/go-paginator/adapter"

	"github.com/prologic/twtxt"
	"github.com/prologic/twtxt/internal/session"
	"github.com/prologic/twtxt/types"
)

const (
	MediaResolution  = 640 // 640x480
	AvatarResolution = 60  // 60x60
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
	w.WriteHeader(http.StatusNotFound)
	s.render("404", w, ctx)
}

// PageHandler ...
func (s *Server) PageHandler(name string) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)
		s.render(name, w, ctx)
	}
}

// ProfileHandler ...
func (s *Server) ProfileHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		nick := NormalizeUsername(p.ByName("nick"))
		if nick == "" {
			ctx.Error = true
			ctx.Message = "No user specified"
			s.render("error", w, ctx)
			return
		}

		nick = NormalizeUsername(nick)

		var profile Profile

		if s.db.HasUser(nick) {
			user, err := s.db.GetUser(nick)
			if err != nil {
				log.WithError(err).Errorf("error loading user object for %s", nick)
				ctx.Error = true
				ctx.Message = "Error loading profile"
				s.render("error", w, ctx)
				return
			}
			profile = user.Profile()
		} else if s.db.HasFeed(nick) {
			feed, err := s.db.GetFeed(nick)
			if err != nil {
				log.WithError(err).Errorf("error loading feed object for %s", nick)
				ctx.Error = true
				ctx.Message = "Error loading profile"
				s.render("error", w, ctx)
				return
			}
			profile = feed.Profile()
		} else {
			ctx.Error = true
			ctx.Message = "User or Feed Not Found"
			s.render("404", w, ctx)
			return
		}

		ctx.Profile = profile

		ctx.Alternatives = append(ctx.Alternatives, Alternatives{
			Alternative{
				Type:  "text/plain",
				Title: fmt.Sprintf("%s's Twtxt Feed", profile.Username),
				URL:   profile.URL,
			},
			Alternative{
				Type:  "application/atom+xml",
				Title: fmt.Sprintf("%s's Atom Feed", profile.Username),
				URL:   fmt.Sprintf("%s/atom.xml", UserURL(profile.URL)),
			},
		}...)

		twts, err := GetUserTwts(s.config, profile.Username)
		if err != nil {
			log.WithError(err).Error("error loading twts")
			ctx.Error = true
			ctx.Message = "An error occurred while loading the profile"
			s.render("error", w, ctx)
			return
		}

		sort.Sort(sort.Reverse(twts))

		var pagedTwts types.Twts

		page := SafeParseInt(r.FormValue("page"), 1)
		pager := paginator.New(adapter.NewSliceAdapter(twts), s.config.TwtsPerPage)
		pager.SetPage(page)

		if err = pager.Results(&pagedTwts); err != nil {
			log.WithError(err).Error("error sorting and paging twts")
			ctx.Error = true
			ctx.Message = "An error occurred while loading the timeline"
			s.render("error", w, ctx)
			return
		}

		ctx.Twts = pagedTwts
		ctx.Pager = pager

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
			ctx.Profile = feed.Profile()
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
				uploadOptions := &UploadOptions{
					Resize:  true,
					ResizeW: AvatarResolution,
					ResizeH: AvatarResolution,
				}
				_, err = StoreUploadedImage(
					s.config, avatarFile,
					avatarsDir, feedName,
					uploadOptions,
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

// MediaHandler ...
func (s *Server) MediaHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		name := p.ByName("name")
		if name == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		fn := filepath.Join(s.config.Data, mediaDir, name)
		fileInfo, err := os.Stat(fn)
		if err != nil {
			log.WithError(err).Error("error reading media file info")
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

		f, err := os.Open(fn)
		if err != nil {
			log.WithError(err).Error("error opening media file")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		defer f.Close()

		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Etag", etag)
		if _, err := io.Copy(w, f); err != nil {
			log.WithError(err).Error("error writing media response")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}
}

// AvatarHandler ...
func (s *Server) AvatarHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		w.Header().Set("Content-Type", "image/png")

		nick := NormalizeUsername(p.ByName("nick"))
		if nick == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		if !s.db.HasUser(nick) && !FeedExists(s.config, nick) {
			http.Error(w, "User or Feed Not Found", http.StatusNotFound)
			return
		}

		fn := filepath.Join(s.config.Data, avatarsDir, fmt.Sprintf("%s.png", nick))
		if fileInfo, err := os.Stat(fn); err == nil {
			etag := fmt.Sprintf("W/\"%s-%s\"", r.RequestURI, fileInfo.ModTime().Format(time.RFC3339))

			if match := r.Header.Get("If-None-Match"); match != "" {
				if strings.Contains(match, etag) {
					w.WriteHeader(http.StatusNotModified)
					return
				}
			}

			f, err := os.Open(fn)
			if err != nil {
				log.WithError(err).Error("error opening avatar file")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			defer f.Close()

			w.Header().Set("Etag", etag)
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

		buf := bytes.Buffer{}
		img := cameron.Identicon([]byte(nick), 60, 12)
		png.Encode(&buf, img)

		w.Header().Set("Etag", etag)
		w.Write(buf.Bytes())
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

		path, err := securejoin.SecureJoin(filepath.Join(s.config.Data, "feeds"), nick)
		if err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		stat, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "Feed Not Found", http.StatusNotFound)
				return
			}

			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if r.Method == http.MethodHead {
			defer r.Body.Close()
			w.Header().Set("Content-Type", "text/plain")
			w.Header().Set(
				"Content-Length",
				fmt.Sprintf("%d", stat.Size()),
			)
			w.Header().Set(
				"Last-Modified",
				stat.ModTime().UTC().Format(http.TimeFormat),
			)
		} else if r.Method == http.MethodGet {
			followerClient, err := DetectFollowerFromUserAgent(r.UserAgent())
			if err != nil {
				log.WithError(err).Warnf("unable to detect twtxt client from %s", FormatRequest(r))
			} else {
				user, err := s.db.GetUser(nick)
				if err != nil {
					log.WithError(err).Warnf("error loading user object for %s", nick)
				} else {
					if !user.FollowedBy(followerClient.URL) {
						if err := AppendSpecial(
							s.config, s.db,
							twtxtBot,
							fmt.Sprintf(
								"FOLLOW: @<%s %s> from @<%s %s> using %s/%s",
								nick, URLForUser(s.config.BaseURL, nick),
								followerClient.Nick, followerClient.URL,
								followerClient.ClientName, followerClient.ClientVersion,
							),
						); err != nil {
							log.WithError(err).Warnf("error appending special FOLLOW post")
						}
						if user.Followers == nil {
							user.Followers = make(map[string]string)
						}
						user.Followers[followerClient.Nick] = followerClient.URL
						if err := s.db.SetUser(nick, user); err != nil {
							log.WithError(err).Warnf("error updating user object for %s", nick)
						}
					}
				}
			}
			http.ServeFile(w, r, path)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	}
}

// PostHandler ...
func (s *Server) PostHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		defer func() {
			// Update user's own timeline with their own new post.
			sources := map[string]string{
				ctx.User.Username: ctx.User.URL,
			}
			s.cache.FetchTwts(s.config, sources)
		}()

		postas := strings.ToLower(strings.TrimSpace(r.FormValue("postas")))

		// TODO: Support deleting/patching last feed (`postas`) twt too.
		if r.Method == http.MethodDelete || r.Method == http.MethodPatch {
			if err := DeleteLastTwt(s.config, ctx.User); err != nil {
				ctx.Error = true
				ctx.Message = "Error deleting last twt"
				s.render("error", w, ctx)
			}

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

		user, err := s.db.GetUser(ctx.Username)
		if err != nil {
			log.WithError(err).Errorf("error loading user object for %s", ctx.Username)
			ctx.Error = true
			ctx.Message = "Error posting twt"
			s.render("error", w, ctx)
			return
		}

		switch postas {
		case "", user.Username:
			if hash != "" && lastTwt.Hash() == hash {
				err = AppendTwt(s.config, s.db, user, text, lastTwt.Created)
			} else {
				err = AppendTwt(s.config, s.db, user, text)
			}
		default:
			if user.OwnsFeed(postas) {
				if hash != "" && lastTwt.Hash() == hash {
					err = AppendSpecial(s.config, s.db, postas, text, lastTwt.Created)
				} else {
					err = AppendSpecial(s.config, s.db, postas, text)
				}
			} else {
				err = ErrFeedImposter
			}
		}

		if err != nil {
			ctx.Error = true
			ctx.Message = "Error posting twt"
			s.render("error", w, ctx)
			return
		}

		http.Redirect(w, r, RedirectURL(r, s.config, "/"), http.StatusFound)
	}
}

// TimelineHandler ...
func (s *Server) TimelineHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		if r.Method == http.MethodHead {
			defer r.Body.Close()

			cacheLastModified, err := CacheLastModified(s.config.Data)
			if err != nil {
				log.WithError(err).Error("CacheLastModified() error")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			w.Header().Set(
				"Last-Modified",
				cacheLastModified.UTC().Format(http.TimeFormat),
			)
			return
		}

		ctx := NewContext(s.config, s.db, r)

		var (
			twts types.Twts
			err  error
		)

		if !ctx.Authenticated {
			twts, err = GetAllTwts(s.config)
		} else {
			user := ctx.User
			if user != nil {
				for _, url := range user.Following {
					twts = append(twts, s.cache.GetByURL(url)...)
				}
			}
		}

		if err != nil {
			log.WithError(err).Error("error loading twts")
			ctx.Error = true
			ctx.Message = "An error occurred while loading the timeline"
			s.render("error", w, ctx)
			return
		}

		sort.Sort(sort.Reverse(twts))

		var pagedTwts types.Twts

		page := SafeParseInt(r.FormValue("page"), 1)
		pager := paginator.New(adapter.NewSliceAdapter(twts), s.config.TwtsPerPage)
		pager.SetPage(page)

		if err = pager.Results(&pagedTwts); err != nil {
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

		ctx.Twts = pagedTwts
		ctx.Pager = pager

		s.render("timeline", w, ctx)
	}
}

// PermalinkHandler ...
func (s *Server) PermalinkHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		var (
			twts types.Twts
			err  error
		)

		hash := p.ByName("hash")
		if hash == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		localTwts, err := GetAllTwts(s.config)
		if err != nil {
			log.WithError(err).Error("error loading twts")
			ctx.Error = true
			ctx.Message = "An error occurred while loading permalink"
			s.render("error", w, ctx)
			return
		}

		cachedTwts := s.cache.GetAll()
		if err != nil {
			log.WithError(err).Error("error loading twts")
			ctx.Error = true
			ctx.Message = "An error occurred while loading permalink"
			s.render("error", w, ctx)
			return
		}

		for _, twt := range cachedTwts {
			if twt.Hash() == hash {
				twts = append(twts, twt)
			}
		}

		// If the twt is not in the cache look for it across all local users
		if len(twts) == 0 {
			for _, twt := range localTwts {
				if twt.Hash() == hash {
					twts = append(twts, twt)
				}
			}
		}

		if len(twts) == 0 {
			ctx.Error = true
			ctx.Message = "No matching twt found!"
			s.render("404", w, ctx)
			return
		}

		ctx.Twts = twts
		s.render("timeline", w, ctx)
		return
	}
}

// DiscoverHandler ...
func (s *Server) DiscoverHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		twts, err := GetAllTwts(s.config)
		if err != nil {
			log.WithError(err).Error("error loading local twts")
			ctx.Error = true
			ctx.Message = "An error occurred while loading the timeline"
			s.render("error", w, ctx)
			return
		}

		sort.Sort(sort.Reverse(twts))

		var pagedTwts types.Twts

		page := SafeParseInt(r.FormValue("page"), 1)
		pager := paginator.New(adapter.NewSliceAdapter(twts), s.config.TwtsPerPage)
		pager.SetPage(page)

		if err = pager.Results(&pagedTwts); err != nil {
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

		ctx.Twts = pagedTwts
		ctx.Pager = pager

		s.render("timeline", w, ctx)
	}
}

// MentionsHandler ...
func (s *Server) MentionsHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		var twts types.Twts

		for _, url := range ctx.User.Following {
			for _, twt := range s.cache.GetByURL(url) {
				if HasString(UniqStrings(twt.Mentions()), ctx.User.Username) {
					twts = append(twts, twt)
				}
			}
		}

		sort.Sort(sort.Reverse(twts))

		var pagedTwts types.Twts

		page := SafeParseInt(r.FormValue("page"), 1)
		pager := paginator.New(adapter.NewSliceAdapter(twts), s.config.TwtsPerPage)
		pager.SetPage(page)

		if err := pager.Results(&pagedTwts); err != nil {
			ctx.Error = true
			ctx.Message = "An error occurred while loading mentions"
			s.render("error", w, ctx)
			return
		}

		ctx.Twts = pagedTwts
		ctx.Pager = pager

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
			for _, twt := range s.cache.GetAll() {
				if HasString(UniqStrings(twt.Tags()), tag) {
					result = append(result, twt)
				}
			}
			return result
		}

		twts = getTweetsByTag()

		sort.Sort(sort.Reverse(twts))

		var pagedTwts types.Twts

		page := SafeParseInt(r.FormValue("page"), 1)
		pager := paginator.New(adapter.NewSliceAdapter(twts), s.config.TwtsPerPage)
		pager.SetPage(page)

		if err := pager.Results(&pagedTwts); err != nil {
			ctx.Error = true
			ctx.Message = "An error occurred while loading search results"
			s.render("error", w, ctx)
			return
		}

		ctx.Twts = pagedTwts
		ctx.Pager = pager

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

		ctx.User.Follow(name, URLForUser(s.config.BaseURL, name))

		if err := s.db.SetUser(ctx.Username, ctx.User); err != nil {
			ctx.Error = true
			ctx.Message = fmt.Sprintf("Error creating feed: %s", err.Error())
			s.render("error", w, ctx)
			return
		}

		if err := AppendSpecial(
			s.config, s.db,
			twtxtBot,
			fmt.Sprintf(
				"FEED: @<%s %s> from @<%s %s>",
				name, URLForUser(s.config.BaseURL, name),
				ctx.User.Username, URLForUser(s.config.BaseURL, ctx.User.Username),
			),
		); err != nil {
			log.WithError(err).Warnf("error appending special FOLLOW post")
		}

		ctx.Error = false
		ctx.Message = fmt.Sprintf("Successfully created feed: %s", name)
		s.render("error", w, ctx)
		return
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

		ctx.Feeds = feeds
		ctx.FeedSources = feedsources.Sources

		s.render("feeds", w, ctx)
	}
}

// LoginHandler ...
func (s *Server) LoginHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		if r.Method == "GET" {
			s.render("login", w, ctx)
			return
		}

		username := NormalizeUsername(r.FormValue("username"))
		password := r.FormValue("password")

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

		// Validate cleartext password against KDF hash
		err = s.pm.CheckPassword(user.Password, password)
		if err != nil {
			ctx.Error = true
			ctx.Message = "Invalid password! Hint: Reset your password?"
			s.render("error", w, ctx)
			return
		}

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
		sess.(*session.Session).Set("username", username)

		http.Redirect(w, r, "/", http.StatusFound)
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
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		if r.Method == "GET" {
			if s.config.Register {
				s.render("register", w, ctx)
			} else {
				message := s.config.RegisterMessage

				if message == "" {
					message = "Registrations are disabled on this instance. Please contact the operator."
				}

				ctx.Error = true
				ctx.Message = message
				s.render("error", w, ctx)
			}

			return
		}

		username := NormalizeUsername(r.FormValue("username"))
		password := r.FormValue("password")
		email := r.FormValue("email")

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

		fn := filepath.Join(s.config.Data, feedsDir, username)
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

		user := NewUser()
		user.Username = username
		user.Email = email
		user.Password = hash
		user.URL = URLForUser(s.config.BaseURL, username)
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

		prefix := r.URL.Query().Get("prefix")

		users := s.db.SearchUsers(prefix)
		feeds := s.db.SearchFeeds(prefix)

		user := ctx.User

		var nicks []string
		for nick := range user.Following {
			if strings.HasPrefix(nick, prefix) {
				nicks = append(nicks, nick)
			}
		}

		var matches []string

		matches = append(matches, users...)
		matches = append(matches, feeds...)
		matches = append(matches, nicks...)

		matches = UniqStrings(matches)

		data, err := json.Marshal(matches)
		if err != nil {
			log.WithError(err).Error("error serializing lookup response")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}

// FollowHandler ...
func (s *Server) FollowHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		nick := strings.TrimSpace(r.FormValue("nick"))
		url := NormalizeURL(r.FormValue("url"))

		if r.Method == "GET" && nick == "" && url == "" {
			s.render("follow", w, ctx)
			return
		}

		if nick == "" || url == "" {
			ctx.Error = true
			ctx.Message = "Both nick and url must be specified"
			s.render("error", w, ctx)
			return
		}

		user := ctx.User
		if user == nil {
			log.Fatalf("user not found in context")
			return
		}

		user.Following[nick] = url

		if err := s.db.SetUser(ctx.Username, user); err != nil {
			ctx.Error = true
			ctx.Message = fmt.Sprintf("Error following feed %s: %s", nick, url)
			s.render("error", w, ctx)
			return
		}

		if strings.HasPrefix(url, s.config.BaseURL) {
			url = UserURL(url)
			nick := NormalizeUsername(filepath.Base(url))

			if s.db.HasUser(nick) {
				followee, err := s.db.GetUser(nick)
				if err != nil {
					log.WithError(err).Errorf("error loading user object for %s", nick)
					ctx.Error = true
					ctx.Message = "Error following user"
					s.render("error", w, ctx)
					return
				}

				if followee.Followers == nil {
					followee.Followers = make(map[string]string)
				}

				followee.Followers[user.Username] = user.URL

				if err := s.db.SetUser(followee.Username, followee); err != nil {
					log.WithError(err).Warnf("error updating user object for followee %s", followee.Username)
					ctx.Error = true
					ctx.Message = "Error following user"
					s.render("error", w, ctx)
					return
				}

				if err := AppendSpecial(
					s.config, s.db,
					twtxtBot,
					fmt.Sprintf(
						"FOLLOW: @<%s %s> from @<%s %s> using %s/%s",
						followee.Username, URLForUser(s.config.BaseURL, followee.Username),
						user.Username, URLForUser(s.config.BaseURL, user.Username),
						"twtxt", twtxt.FullVersion(),
					),
				); err != nil {
					log.WithError(err).Warnf("error appending special FOLLOW post")
				}
			} else if s.db.HasFeed(nick) {
				feed, err := s.db.GetFeed(nick)
				if err != nil {
					log.WithError(err).Errorf("error loading feed object for %s", nick)
					ctx.Error = true
					ctx.Message = "Error following user"
					s.render("error", w, ctx)
					return
				}

				feed.Followers[user.Username] = user.URL

				if err := s.db.SetFeed(feed.Name, feed); err != nil {
					log.WithError(err).Warnf("error updating user object for followee %s", feed.Name)
					ctx.Error = true
					ctx.Message = "Error following feed"
					s.render("error", w, ctx)
					return
				}

				if err := AppendSpecial(
					s.config, s.db,
					twtxtBot,
					fmt.Sprintf(
						"FOLLOW: @<%s %s> from @<%s %s> using %s/%s",
						feed.Name, URLForUser(s.config.BaseURL, feed.Name),
						user.Username, URLForUser(s.config.BaseURL, user.Username),
						"twtxt", twtxt.FullVersion(),
					),
				); err != nil {
					log.WithError(err).Warnf("error appending special FOLLOW post")
				}
			}
		}

		ctx.Error = false
		ctx.Message = fmt.Sprintf("Successfully started following %s: %s", nick, url)
		s.render("error", w, ctx)
		return
	}
}

// ImportHandler ...
func (s *Server) ImportHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		if r.Method == "GET" {
			s.render("import", w, ctx)
			return
		}

		feeds := r.FormValue("feeds")

		if feeds == "" {
			ctx.Error = true
			ctx.Message = "Nothing to import!"
			s.render("error", w, ctx)
			return
		}

		user := ctx.User
		if user == nil {
			log.Fatalf("user not found in context")
		}

		re := regexp.MustCompile(`(?P<nick>.*?)[: ](?P<url>.*)`)

		imported := 0

		scanner := bufio.NewScanner(strings.NewReader(feeds))
		for scanner.Scan() {
			line := scanner.Text()
			matches := re.FindStringSubmatch(line)
			if len(matches) == 3 {
				nick := strings.TrimSpace(matches[1])
				url := NormalizeURL(strings.TrimSpace(matches[2]))
				if nick != "" && url != "" {
					user.Following[nick] = url
					imported++
				}
			}
		}
		if err := scanner.Err(); err != nil {
			log.WithError(err).Error("error scanning feeds for import")
			ctx.Error = true
			ctx.Message = "Error importing feeds"
			s.render("error", w, ctx)
		}

		if err := s.db.SetUser(ctx.Username, user); err != nil {
			ctx.Error = true
			ctx.Message = "Error importing feeds"
			s.render("error", w, ctx)
			return
		}

		ctx.Error = false
		ctx.Message = fmt.Sprintf("Successfully imported %d feeds", imported)
		s.render("error", w, ctx)
		return
	}
}

// UnfollowHandler ...
func (s *Server) UnfollowHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		nick := strings.TrimSpace(r.FormValue("nick"))

		if nick == "" {
			ctx.Error = true
			ctx.Message = "No nick specified to unfollow"
			s.render("error", w, ctx)
			return
		}

		user := ctx.User
		if user == nil {
			log.Fatalf("user not found in context")
		}

		url, ok := user.Following[nick]
		if !ok {
			ctx.Error = true
			ctx.Message = fmt.Sprintf("No feed found by the nick %s", nick)
			s.render("error", w, ctx)
			return
		}

		delete(user.Following, nick)

		if err := s.db.SetUser(ctx.Username, user); err != nil {
			ctx.Error = true
			ctx.Message = fmt.Sprintf("Error unfollowing feed %s: %s", nick, url)
			s.render("error", w, ctx)
			return
		}

		if strings.HasPrefix(url, s.config.BaseURL) {
			url = UserURL(url)
			nick := NormalizeUsername(filepath.Base(url))
			followee, err := s.db.GetUser(nick)
			if err != nil {
				log.WithError(err).Warnf("error loading user object for followee %s", nick)
			} else {
				if followee.Followers != nil {
					delete(followee.Followers, user.Username)
					if err := s.db.SetUser(followee.Username, followee); err != nil {
						log.WithError(err).Warnf("error updating user object for followee %s", followee.Username)
					}
				}
				if err := AppendSpecial(
					s.config, s.db,
					twtxtBot,
					fmt.Sprintf(
						"UNFOLLOW: @<%s %s> from @<%s %s> using %s/%s",
						followee.Username, URLForUser(s.config.BaseURL, followee.Username),
						user.Username, URLForUser(s.config.BaseURL, user.Username),
						"twtxt", twtxt.FullVersion(),
					),
				); err != nil {
					log.WithError(err).Warnf("error appending special FOLLOW post")
				}
			}
		}

		ctx.Error = false
		ctx.Message = fmt.Sprintf("Successfully stopped following %s: %s", nick, url)
		s.render("error", w, ctx)
		return
	}
}

// SettingsHandler ...
func (s *Server) SettingsHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		if r.Method == "GET" {
			s.render("settings", w, ctx)
			return
		}

		// Limit request body to to abuse
		r.Body = http.MaxBytesReader(w, r.Body, s.config.MaxUploadSize)

		email := strings.TrimSpace(r.FormValue("email"))
		tagline := strings.TrimSpace(r.FormValue("tagline"))
		password := r.FormValue("password")
		isFollowersPubliclyVisible := r.FormValue("isFollowersPubliclyVisible") == "on"
		isFollowingPubliclyVisible := r.FormValue("isFollowingPubliclyVisible") == "on"

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
			uploadOptions := &UploadOptions{
				Resize:  true,
				ResizeW: AvatarResolution,
				ResizeH: AvatarResolution,
			}
			_, err = StoreUploadedImage(
				s.config, avatarFile,
				avatarsDir, ctx.Username,
				uploadOptions,
			)
			if err != nil {
				ctx.Error = true
				ctx.Message = fmt.Sprintf("Error updating user: %s", err)
				s.render("error", w, ctx)
				return
			}
		}

		user.Email = email
		user.Tagline = tagline
		user.IsFollowersPubliclyVisible = isFollowersPubliclyVisible
		user.IsFollowingPubliclyVisible = isFollowingPubliclyVisible

		if err := s.db.SetUser(ctx.Username, user); err != nil {
			ctx.Error = true
			ctx.Message = "Error updating user"
			s.render("error", w, ctx)
			return
		}

		ctx.Error = false
		ctx.Message = "Successfully updated settings"
		s.render("error", w, ctx)
		return
	}
}

// DeleteHandler ...
func (s *Server) DeleteHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		user := ctx.User
		if user == nil {
			log.Fatalf("user not found in context")
		}

		if err := s.db.DelUser(ctx.Username); err != nil {
			ctx.Error = true
			ctx.Message = "Error deleting account"
			s.render("error", w, ctx)
			return
		}

		s.sm.Delete(w, r)
		ctx.Authenticated = false

		ctx.Error = false
		ctx.Message = "Successfully deleted account"
		s.render("error", w, ctx)
		return
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
			ctx.Profile = user.Profile()
		} else if s.db.HasFeed(nick) {
			feed, err := s.db.GetFeed(nick)
			if err != nil {
				log.WithError(err).Errorf("error loading feed object for %s", nick)
				ctx.Error = true
				ctx.Message = "Error loading profile"
				s.render("error", w, ctx)
				return
			}
			ctx.Profile = feed.Profile()
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
			ctx.Profile = user.Profile()
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

		s.render("following", w, ctx)
	}
}

// FollowingHandler ...
func (s *Server) ExternalHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)
		query := r.URL.Query()

		nick := query.Get("nick")
		if nick == "" {
			ctx.Error = true
			ctx.Message = "Cannot find external feed"
			s.render("error", w, ctx)
		}

		url := query.Get("url")
		if url == "" {
			ctx.Error = true
			ctx.Message = "Cannot find external feed"
			s.render("error", w, ctx)
		}

		twtxttURL := string(url)

		twts := s.cache.GetByURL(twtxttURL)

		sort.Sort(sort.Reverse(twts))

		var pagedTwts types.Twts

		page := SafeParseInt(r.FormValue("page"), 1)
		pager := paginator.New(adapter.NewSliceAdapter(twts), s.config.TwtsPerPage)
		pager.SetPage(page)

		if err := pager.Results(&pagedTwts); err != nil {
			log.WithError(err).Error("error sorting and paging twts")
			ctx.Error = true
			ctx.Message = "An error occurred while loading the timeline"
			s.render("error", w, ctx)
			return
		}

		ctx.Twts = pagedTwts
		ctx.Pager = pager
		ctx.Profile = Profile{
			Username: nick,
			TwtURL:   url,
			URL:      url,
		}

		s.render("externalProfile", w, ctx)
	}
}

// ResetPasswordHandler ...
func (s *Server) ResetPasswordHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		if r.Method == "GET" {
			return
		}

		username := NormalizeUsername(r.FormValue("username"))

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

		// Create magic link expiry time
		now := time.Now()
		secs := now.Unix()
		expiresAfterSeconds := int64(600) // Link expires after 10 minutes

		expiryTime := secs + expiresAfterSeconds

		// Create magic link
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"username": username, "expiresAt": expiryTime})
		tokenString, err := token.SignedString([]byte(s.config.MagicLinkSecret))
		if err != nil {
			ctx.Error = true
			ctx.Message = err.Error()
			s.render("error", w, ctx)
			return
		}

		magicLink := fmt.Sprintf("%s/newPassword?token=%v", s.config.BaseURL, tokenString)

		// Send email
		to := []string{user.Email}
		subject := "Reset Password - txttxt.net"
		body := magicLink

		if err := s.config.SendEmail(to, subject, body); err != nil {
			log.WithError(err).Errorf("unable to send reset password email to %s", user.Email)
			ctx.Error = true
			ctx.Message = err.Error()
			s.render("error", w, ctx)
			return
		}

		log.Infof("reset password email sent for %s", user.Username)

		// Show success msg
		ctx.Error = false
		ctx.Message = fmt.Sprintf("Magic Link successfully sent via email to %v", user.Email)
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

// UploadMediaHandler ...
func (s *Server) UploadMediaHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		// Limit request body to to abuse
		r.Body = http.MaxBytesReader(w, r.Body, s.config.MaxUploadSize)

		mediaFile, _, err := r.FormFile("media_file")
		if err != nil && err != http.ErrMissingFile {
			log.WithError(err).Error("error parsing form file")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var mediaURI string

		if mediaFile != nil {
			uploadOptions := &UploadOptions{Resize: true, ResizeW: MediaResolution, ResizeH: 0}
			mediaURI, err = StoreUploadedImage(
				s.config, mediaFile,
				mediaDir, "",
				uploadOptions,
			)
		}

		uri := URI{"mediaURI", mediaURI}
		data, err := json.Marshal(uri)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)

		return
	}
}

// SyndicationHandler ...
func (s *Server) SyndicationHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		var (
			twts    types.Twts
			profile Profile
			err     error
		)

		nick := NormalizeUsername(p.ByName("nick"))
		if nick != "" {
			if s.db.HasUser(nick) {
				twts, err = GetUserTwts(s.config, nick)
				if user, err := s.db.GetUser(nick); err == nil {
					profile = user.Profile()
				} else {
					log.WithError(err).Error("error loading user object")
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
			} else if s.db.HasFeed(nick) {
				twts, err = GetUserTwts(s.config, nick)
				if feed, err := s.db.GetFeed(nick); err == nil {
					profile = feed.Profile()
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
			twts, err = GetAllTwts(s.config)
			profile = Profile{
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

		sort.Sort(sort.Reverse(twts))

		if r.Method == http.MethodHead {
			defer r.Body.Close()
			w.Header().Set(
				"Last-Modified",
				twts[len(twts)].Created.Format(http.TimeFormat),
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
				Id:      twt.Hash(),
				Title:   twt.Text,
				Link:    &feeds.Link{Href: URLForTwt(s.config.BaseURL, twt.Hash())},
				Author:  &feeds.Author{Name: twt.Twter.Nick},
				Created: twt.Created,
			},
			)
		}
		feed.Items = items

		w.Header().Set("Content-Type", "application/atom+xml")
		data, err := feed.ToAtom()
		if err != nil {
			log.WithError(err).Error("error serializing feed")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Write([]byte(data))
	}
}
