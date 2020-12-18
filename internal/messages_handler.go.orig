package internal

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
)

// ListMessagesHandler ...
func (s *Server) ListMessagesHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		ctx.Title = "Private Messages"

		ctx.Messages = Messages{
			&Message{
				From: "kate",
				Sent: time.Now(),
			},
			&Message{
				From: "admin",
				Sent: time.Now(),
			},
		}

		s.render("messages", w, ctx)
		return
	}
}

// SendMessagesHandler ...
func (s *Server) SendMessageHandler() httprouter.Handle {
	localDomain := HostnameFromURL(s.config.BaseURL)

	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		from := fmt.Sprintf("%s@%s", ctx.User.Username, localDomain)

		recipient := NormalizeUsername(strings.TrimSpace(r.FormValue("recipient")))
		if !s.db.HasUser(recipient) {
			ctx.Error = true
			ctx.Message = "No such user exists!"
			s.render("error", w, ctx)
			return
		}
		to := fmt.Sprintf("%s@%s", recipient, localDomain)

		subject := strings.TrimSpace(r.FormValue("subject"))
		body := strings.NewReader(strings.TrimSpace(r.FormValue("body")))

		msg, err := createMessage(from, to, subject, body)
		if err != nil {
			ctx.Error = true
			ctx.Message = "Error creating message"
			s.render("error", w, ctx)
			return
		}

		if err := writeMessage(s.config, msg, recipient); err != nil {
			ctx.Error = true
			ctx.Message = "Error sending message, please try again later!"
			s.render("error", w, ctx)
			return
		}

		ctx.Error = false
		ctx.Message = "Messages successfully sent"
		s.render("error", w, ctx)
		return
	}
}

// DeleteMessagesHandler ...
func (s *Server) DeleteMessagesHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		ctx.Error = false
		ctx.Message = "Messages successfully deleted"
		s.render("error", w, ctx)
		return
	}
}

// ViewMessageHandler ...
func (s *Server) ViewMessageHandler() httprouter.Handle {
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
