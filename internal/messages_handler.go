package internal

import (
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
)

type Message struct {
	From string
	Sent time.Time

	hash string
}

func (m *Message) Hash() string {
	if m.hash != "" {
		return m.hash
	}

	m.hash = "fa47b31"

	return m.hash
}

func (m *Message) Text() string {
	return "Hello there!"
}

type Messages []*Message

// MessagesHandler ...
func (s *Server) MessagesHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		ctx.Title = "Private Messages"
		s.render("messages", w, ctx)
		return
	}
}

// MessageHandler ...
func (s *Server) MessageHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		if r.Method == http.MethodPost {
			// TODO: Implement this :D
			return
		}

		ctx.Title = "Private Message from Kate: Hello"

		ctx.Messages = Messages{
			&Message{
				From: "kate",
				Sent: time.Now(),
			},
		}

		s.render("message", w, ctx)
		return
	}
}
