package internal

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/prologic/twtxt/types"
	log "github.com/sirupsen/logrus"
)

func init() {
}

// WhoFollowsHandler ...
func (s *Server) WhoFollowsHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		uri := r.URL.Query().Get("uri")
		nick := r.URL.Query().Get("nick")

		if uri == "" {
			ctx.Error = true
			ctx.Message = "No URI supplied"
			s.render("error", w, ctx)
			return
		}

		if nick == "" {
			log.Warn("no nick given to whoFollows request")
			nick = "unknown"
		}

		followers := make(map[string]string)

		users, err := s.db.GetAllUsers()
		if err != nil {
			log.WithError(err).Error("unable to get all users from database")
			ctx.Error = true
			ctx.Message = "Error computing followers list"
			s.render("error", w, ctx)
			return
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

		if r.Header.Get("Accept") == "application/json" {
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
