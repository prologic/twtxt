package internal

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/prologic/twtxt/types"
	"github.com/vcraescu/go-paginator"
	"github.com/vcraescu/go-paginator/adapter"
)

// MessagesHandler ...
func (s *Server) MessagesHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		twts := types.Twts{}

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

		if r.Method == http.MethodHead {
			defer r.Body.Close()
			return
		}

		title := "Messages ..."

		ctx.Title = title
		ctx.Twts = FilterTwts(ctx.User, pagedTwts)
		ctx.Pager = &pager
		s.render("messages", w, ctx)
		return
	}
}
