package internal

import (
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/vcraescu/go-paginator"
	"github.com/vcraescu/go-paginator/adapter"
)

type Message struct {
	User    string
	Created time.Time
	Updated time.Time

	hash string
}

func (m Message) Hash() string {
	if m.hash != "" {
		return m.hash
	}

	return FastHash(m.User + "\n" + m.Created.String())
}

func (m Message) Text() string {
	return "foo bar baz"
}

type Messages []Message

// MessagesHandler ...
func (s *Server) MessagesHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		messages := Messages{
			Message{
				User:    "admin",
				Created: time.Now(),
				Updated: time.Now(),
			},
		}

		var pagedMessages Messages

		page := SafeParseInt(r.FormValue("p"), 1)
		pager := paginator.New(adapter.NewSliceAdapter(messages), s.config.TwtsPerPage)
		pager.SetPage(page)

		if err := pager.Results(&pagedMessages); err != nil {
			ctx.Error = true
			ctx.Message = "An error occurred while loading search results"
			s.render("error", w, ctx)
			return
		}

		if r.Method == http.MethodHead {
			defer r.Body.Close()
			return
		}

		title := "Messages"

		ctx.Title = title
		// TODO: Filter out messages from blocked users?
		//ctx.Messages = FilterMessages(ctx.User, pagedMessages)
		ctx.Messages = pagedMessages
		ctx.Pager = &pager
		s.render("messages", w, ctx)
		return
	}
}
