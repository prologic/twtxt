package internal

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// MessagesHandler ...
func (s *Server) MessagesHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		ctx.Title = "Private Messages"
		s.render("messages", w, ctx)
		return
	}
}
