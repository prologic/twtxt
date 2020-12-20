package internal

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/julienschmidt/httprouter"
	"github.com/vcraescu/go-paginator"
	"github.com/vcraescu/go-paginator/adapter"
)

// ListMessagesHandler ...
func (s *Server) ListMessagesHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		msgs, err := getMessages(s.config, ctx.User.Username)
		if err != nil {
			ctx.Error = true
			ctx.Message = "Error getting messages"
			s.render("error", w, ctx)
			return
		}
		sort.Sort(msgs)

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

		if r.FormValue("delete_all") != "" {
			if err := deleteAllMessages(s.config, ctx.Username); err != nil {
				ctx.Error = true
				ctx.Message = "Error deleting all messages! Please try again later"
				s.render("error", w, ctx)
				return
			}
			ctx.Error = false
			ctx.Message = "All messages successfully deleted!"
			s.render("error", w, ctx)
			return
		}

		msgId := r.Form["msgid"]
		log.Debugf("msgid: %v", msgId)

		ctx.Error = false
		ctx.Message = "Selected messages successfully deleted!"
		s.render("error", w, ctx)
		return
	}
}

// ViewMessageHandler ...
func (s *Server) ViewMessageHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s.config, s.db, r)

		msgId, err := strconv.Atoi(p.ByName("msgid"))
		if err != nil {
			ctx.Error = true
			ctx.Message = "Error invalid message id"
			s.render("error", w, ctx)
			return
		}

		msg, err := getMessage(s.config, ctx.Username, msgId)
		if err != nil {
			ctx.Error = true
			ctx.Message = "Error opening message, please try again later!"
			s.render("error", w, ctx)
			return
		}
		if err := markMessageAsRead(s.config, ctx.Username, msgId); err != nil {
			log.WithError(err).Warn("error marking message %d for %s as read", msgId, ctx.Username)
		}

		ctx.Title = fmt.Sprintf("Private Message from %s: %s", msg.From, msg.Subject)
		ctx.Messages = Messages{msg}
		s.render("message", w, ctx)
		return
	}
}
