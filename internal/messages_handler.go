package internal

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/emersion/go-mbox"
	"github.com/emersion/go-message"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

const (
	msgsDir          = "msgs"
	rfc2822          = "Mon Jan 02 15:04:05 -0700 2006"
	headerKeyTo      = "To"
	headerKeyDate    = "Date"
	headerKeyFrom    = "From"
	headerKeySubject = "Subject"
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

func writeMessage(conf *Config, msg *message.Entity, username string) error {
	p := filepath.Join(conf.Data, msgsDir)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating msgs directory")
		return err
	}

	fn := filepath.Join(p, username)

	f, err := os.OpenFile(fn, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	from := msg.Header.Get(headerKeyFrom)
	if from == "" {
		return fmt.Errorf("error no `From` header found in message")
	}

	w := mbox.NewWriter(f)
	defer w.Close()

	mw, err := w.CreateMessage(from, time.Now())
	if err != nil {
		log.WithError(err).Error("error creating message writer")
		return fmt.Errorf("error creating message writer: %w", err)
	}

	if err := msg.WriteTo(mw); err != nil {
		log.WithError(err).Error("error writing message")
		return fmt.Errorf("error writing message: %w", err)
	}

	return nil
}

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

		var headers message.Header

		now := time.Now()

		headers.Set(headerKeyFrom, from)
		headers.Set(headerKeyTo, to)
		headers.Set(headerKeySubject, subject)
		headers.Set(headerKeyDate, now.Format(rfc2822))

		msg, err := message.New(headers, body)
		if err != nil {
			log.WithError(err).Error("error creating entity")
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
