package internal

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/vcraescu/go-paginator"
	"github.com/vcraescu/go-paginator/adapter"
)

// ListMessagesHandler ...
func (s *Server) ListMessagesHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		var msgs Messages

		m, err := getMessages(s.config, ctx.User.Username)
		if err != nil {
			ctx.Error = true
			ctx.Message = "Error getting messages"
			s.render("error", w, ctx)
			return
		}

		for i := 0; i < len(m); i++ {
			d, err := time.Parse(rfc2822, m[i].Header.Get(headerKeyDate))
			if err != nil {
				ctx.Error = true
				ctx.Message = "Error parsing message date"
				s.render("error", w, ctx)
				return
			}
			msg := &Message{
				From:    m[i].Header.Get(headerKeyFrom),
				Sent:    d,
				Subject: m[i].Header.Get(headerKeySubject),
				Status:  m[i].Header.Get(headerKeyStatus),
			}
			msgs = append(msgs, msg)
		}

		var pagedMsgs Messages

		page := SafeParseInt(r.FormValue("p"), 1)
		pager := paginator.New(adapter.NewSliceAdapter(msgs), s.config.MsgsPerPage)
		pager.SetPage(page)

		if err := pager.Results(&pagedMsgs); err != nil {
			log.WithError(err).Error("error sorting and paging messages")
			ctx.Error = true
			ctx.Message = "An error occurred while loading messages"
			s.render("error", w, ctx)
			return
		}

		ctx.Title = "Private Messages"

		ctx.Messages = pagedMsgs
		ctx.Pager = &pager

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
