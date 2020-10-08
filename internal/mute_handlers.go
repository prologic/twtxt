package internal

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

// MuteHandler ...
func (s *Server) MuteHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		nick := strings.TrimSpace(r.FormValue("nick"))
		url := NormalizeURL(r.FormValue("url"))

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

		user.Mute(nick, url)

		if err := s.db.SetUser(ctx.Username, user); err != nil {
			ctx.Error = true
			ctx.Message = fmt.Sprintf("Error muting feed %s: %s", nick, url)
			s.render("error", w, ctx)
			return
		}

		ctx.Error = false
		ctx.Message = fmt.Sprintf("Successfully muted %s: %s", nick, url)
		s.render("error", w, ctx)
		return
	}
}

// UnmuteHandler ...
func (s *Server) UnmuteHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		nick := strings.TrimSpace(r.FormValue("nick"))

		if nick == "" {
			ctx.Error = true
			ctx.Message = "No nick specified to unmute"
			s.render("error", w, ctx)
			return
		}

		user := ctx.User
		if user == nil {
			log.Fatalf("user not found in context")
		}

		user.Unmute(nick)

		if err := s.db.SetUser(ctx.Username, user); err != nil {
			ctx.Error = true
			ctx.Message = fmt.Sprintf("Error unmuting feed %s", nick)
			s.render("error", w, ctx)
			return
		}

		ctx.Error = false
		ctx.Message = fmt.Sprintf("Successfully unmuted %s", nick)
		s.render("error", w, ctx)
		return
	}
}
