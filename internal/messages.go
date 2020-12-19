package internal

import (
	"fmt"
	"io"
	"io/ioutil"
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

func (m Message) User() string {
	username, _ := splitEmailAddress(m.From)
	return username
}

func (m Message) Text() string {
	return m.body
}

type Messages []Message

func (msgs Messages) Len() int {
	return len(msgs)
}
func (msgs Messages) Less(i, j int) bool {
	return msgs[i].Sent.After(msgs[j].Sent)
}
func (msgs Messages) Swap(i, j int) {
	msgs[i], msgs[j] = msgs[j], msgs[i]
}

func getMessage(conf *Config, username string, msgId int) (msg Message, err error) {
	path := filepath.Join(conf.Data, msgsDir)
	if err := os.MkdirAll(path, 0755); err != nil {
		log.WithError(err).Error("error creating msgs directory")
		return msg, fmt.Errorf("error creating msgs directory: %w", err)
	}

	fn := filepath.Join(path, username)

	f, err := os.OpenFile(fn, os.O_CREATE|os.O_RDONLY, 0666)
	if err != nil {
		log.WithError(err).Error("error opening msgs file")
		return msg, fmt.Errorf("error opening msgs file: %w", err)
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
			return msg, fmt.Errorf("error getting next message reader: %w", err)
		}

		e, err := message.Read(r)
		if err != nil {
			log.WithError(err).Error("error reading next message")
			return msg, fmt.Errorf("error reading next message: %w", err)
		}

		if id == msgId {
			d, err := time.Parse(rfc2822, e.Header.Get(headerKeyDate))
			if err != nil {
				log.WithError(err).Error("error parsing message date")
				return msg, fmt.Errorf("error parsing message date: %w", err)
			}

			body, err := ioutil.ReadAll(e.Body)
			if err != nil {
				log.WithError(err).Error("error reading message body")
				return msg, fmt.Errorf("error reading message body: %w", err)
			}

			return Message{
				Id:      id,
				From:    e.Header.Get(headerKeyFrom),
				Sent:    d,
				Subject: e.Header.Get(headerKeySubject),
				Status:  e.Header.Get(headerKeyStatus),
				body:    string(body),
			}, nil
		}

		id++
	}

	return msg, fmt.Errorf("error message not found")
}

func getMessages(conf *Config, username string) (Messages, error) {
	var msgs Messages

	path := filepath.Join(conf.Data, msgsDir)
	if err := os.MkdirAll(path, 0755); err != nil {
		log.WithError(err).Error("error creating msgs directory")
		return nil, fmt.Errorf("error creating msgs directory: %w", err)
	}

	fn := filepath.Join(path, username)

	f, err := os.OpenFile(fn, os.O_CREATE|os.O_RDONLY, 0666)
	if err != nil {
		log.WithError(err).Error("error opening msgs file")
		return nil, fmt.Errorf("error opening msgs file: %w", err)
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
			return nil, fmt.Errorf("error getting next message reader: %w", err)
		}
		e, err := message.Read(r)
		if err != nil {
			log.WithError(err).Error("error reading next message")
			return nil, fmt.Errorf("error reading next message: %w", err)
		}

		d, err := time.Parse(rfc2822, e.Header.Get(headerKeyDate))
		if err != nil {
			log.WithError(err).Error("error parsing message date")
			return nil, fmt.Errorf("error parsing message date: %w", err)
		}

		msg := Message{
			Id:      id,
			From:    e.Header.Get(headerKeyFrom),
			Sent:    d,
			Subject: e.Header.Get(headerKeySubject),
			Status:  e.Header.Get(headerKeyStatus),
		}

		id++

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
