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
	headerKeyStatus  = "Status"
)

type Message struct {
	Id      int
	From    string
	Sent    time.Time
	Subject string
	Status  string

	body string
}

func (m *Message) Text() string {
	return m.body
}

type Messages []*Message

func (msgs Messages) Len() int {
	return len(msgs)
}
func (msgs Messages) Less(i, j int) bool {
	return msgs[i].Sent.After(msgs[j].Sent)
}
func (msgs Messages) Swap(i, j int) {
	msgs[i], msgs[j] = msgs[j], msgs[i]
}

func getMessages(conf *Config, username string) (Messages, error) {
	var msgs Messages

	path := filepath.Join(conf.Data, msgsDir)
	if err := os.MkdirAll(path, 0755); err != nil {
		log.WithError(err).Error("error creating msgs directory")
		return nil, err
	}

	fn := filepath.Join(path, username)

	f, err := os.OpenFile(fn, os.O_CREATE|os.O_RDONLY, 0666)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	mr := mbox.NewReader(f)

	id := 1

	for {
		r, err := mr.NextMessage()
		if err == io.EOF {
			break
		} else if err != nil {
			log.WithError(err).Error("error getting next message reader")
			return nil, err
		}
		e, err := message.Read(r)
		if err != nil {
			log.WithError(err).Error("error reading next message")
			return nil, err
		}

		d, err := time.Parse(rfc2822, e.Header.Get(headerKeyDate))
		if err != nil {
			log.WithError(err).Error("error parsing message date")
			return nil, fmt.Errorf("error parsing message date: %w", err)
		}

		id++

		msg := &Message{
			Id:      id,
			From:    e.Header.Get(headerKeyFrom),
			Sent:    d,
			Subject: e.Header.Get(headerKeySubject),
			Status:  e.Header.Get(headerKeyStatus),
		}

		msgs = append(msgs, msg)
	}

	return msgs, nil
}

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
