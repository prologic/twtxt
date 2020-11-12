package internal

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/prologic/twtxt/types"
	log "github.com/sirupsen/logrus"
)

// WhoFollowsHandler ...
func (s *Server) WhoFollowsHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		ctype := "html"
		if r.Header.Get("Accept") == "application/json" {
			ctype = "json"
		}

		uri := r.URL.Query().Get("uri")
		nick := r.URL.Query().Get("nick")
		token := r.URL.Query().Get("token")

		if uri == "" {
			if ctype == "html" {
				ctx.Error = true
				ctx.Message = "No URI supplied"
				s.render("error", w, ctx)
			} else {
				http.Error(w, "Bad Request", http.StatusBadRequest)
			}
			return
		}

		if nick == "" {
			log.Warn("no nick given to whoFollows request")
			nick = "unknown"
		}

		if !ctx.Authenticated && tokenCache.Get(token) == 0 {
			log.Warn("unauthenticated or invalid token for whoFollows request")
			if ctype == "html" {
				ctx.Error = true
				ctx.Message = "You are not authorized to view this resource"
				s.render("401", w, ctx)
			} else {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			}
			return
		}

		followers := make(map[string]string)

		users, err := s.db.GetAllUsers()
		if err != nil {
			log.WithError(err).Error("unable to get all users from database")
			if ctype == "html" {
				ctx.Error = true
				ctx.Message = "Error computing followers list"
				s.render("error", w, ctx)
			} else {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
			return
		}

		for _, user := range users {
			if !user.IsFollowersPubliclyVisible && !ctx.User.Is(user.URL) {
				continue
			}

			if user.Follows(uri) {
				followers[user.Username] = user.URL
			}
		}

		ctx.Profile = types.Profile{
			Type: "External",

			Username: nick,
			Tagline:  "",
			URL:      uri,
			BlogsURL: "#",

			Follows:    true,
			FollowedBy: true,
			Muted:      false,

			Followers: followers,
		}

		if ctype == "json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			if err := json.NewEncoder(w).Encode(ctx.Profile.Followers); err != nil {
				log.WithError(err).Error("error encoding user for display")
				http.Error(w, "Bad Request", http.StatusBadRequest)
			}

			return
		}

		ctx.Title = fmt.Sprintf("Followers for @<%s %s>", nick, uri)
		s.render("followers", w, ctx)
	}
}
