package internal

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/emersion/go-mbox"
	"github.com/emersion/go-message"
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

func createMessage(from, to, subject string, body io.Reader) (*message.Entity, error) {
	var headers message.Header

	now := time.Now()

	headers.Set(headerKeyFrom, from)
	headers.Set(headerKeyTo, to)
	headers.Set(headerKeySubject, subject)
	headers.Set(headerKeyDate, now.Format(rfc2822))

	msg, err := message.New(headers, body)
	if err != nil {
		log.WithError(err).Error("error creating entity")
		return nil, fmt.Errorf("error creating entity: %w", err)
	}

	return msg, nil
}

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
