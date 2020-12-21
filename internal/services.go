package internal

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"net"
	"net/mail"
	"path/filepath"
	"strings"
	"time"

	"github.com/emersion/go-message"
	"github.com/jointwt/twtxt/internal/passwords"
	"github.com/marcinwyszynski/popart"
	"github.com/prologic/smtpd"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"

	"github.com/jointwt/twtxt"
)

func parseAddresses(addrs []string) ([]*mail.Address, error) {
	var addresses []*mail.Address

	for _, addr := range addrs {
		address, err := mail.ParseAddress(addr)
		if err != nil {
			log.WithError(err).Error("error parsing address")
			return nil, fmt.Errorf("error parsing address %s: %w", addr, err)
		}
		addresses = append(addresses, address)
	}

	return addresses, nil
}

func storeMessage(conf *Config, msg *message.Entity, to []string) error {
	addresses, err := parseAddresses(to)
	if err != nil {
		log.WithError(err).Error("error parsing `To` address list")
		return fmt.Errorf("error parsing `To` address list: %w", err)
	}

	for _, address := range addresses {
		username, _ := splitEmailAddress(address.Address)
		if err := writeMessage(conf, msg, username); err != nil {
			log.WithError(err).Errorf("error writing message for %s", username)
			return fmt.Errorf("error writing message for %s: %w", username, err)
		}
	}

	return nil
}

func splitEmailAddress(email string) (string, string) {
	components := strings.Split(email, "@")
	username, domain := components[0], components[1]
	return username, domain
}

func validMAC(fn func() hash.Hash, message, messageMAC, key []byte) bool {
	mac := hmac.New(fn, key)
	mac.Write(message)
	expectedMAC := mac.Sum(nil)
	return hmac.Equal(messageMAC, expectedMAC)
}

type mboxHandler struct {
	config *Config
	db     Store
	pm     passwords.Passwords
	tasks  *Dispatcher

	mboxFile string
	username string
}

func NewMboxHandler(config *Config, db Store, pm passwords.Passwords, tasks *Dispatcher) popart.Handler {
	return &mboxHandler{config, db, pm, tasks, "", ""}
}

func (m *mboxHandler) AuthenticatePASS(username, password string) error {
	if !m.db.HasUser(username) {
		return fmt.Errorf("error: invalid credentials")
	}

	user, err := m.db.GetUser(username)
	if err != nil {
		log.WithError(err).Error("error loading user object")
		return fmt.Errorf("error loading user  object: %w", err)
	}

	if password != user.POP3Token {
		return fmt.Errorf("error: invalid credentials")
	}

	log.Debugf("Logged in with username %q and password %q", username, password)
	m.username = username
	m.mboxFile = filepath.Join(msgsDir, username)
	return nil
}

func (m *mboxHandler) AuthenticateAPOP(username, hexdigest string) error {
	return nil
}

func (m *mboxHandler) DeleteMessages(numbers []uint64) error {
	strNums := make([]string, len(numbers), len(numbers))
	for i, number := range numbers {
		strNums[i] = fmt.Sprintf("%d", number)
	}
	log.Debugf(
		"Following messages would be deleted: %q",
		strings.Join(strNums, ", "),
	)
	return nil
}

func (m *mboxHandler) GetMessageReader(number uint64) (io.ReadCloser, error) {
	return ioutil.NopCloser(strings.NewReader("Hello World!")), nil
}

func (m *mboxHandler) GetMessageCount() (uint64, error) {
	return uint64(4), nil
}

func (m *mboxHandler) GetMessageID(number uint64) (string, error) {
	return "foo", nil
}

func (m *mboxHandler) GetMessageSize(number uint64) (uint64, error) {
	return uint64(12), nil
}

func (m *mboxHandler) HandleSessionError(err error) {
	log.WithError(err).Error("error occurred handling session")
}

func (m *mboxHandler) LockMaildrop() error {
	log.Debugf("Mailbox for user %q would be locked", m.username)
	return nil
}

func (m *mboxHandler) SetBanner(banner string) error {
	log.Debugf("Banner would be set to %q", banner)
	return nil
}

func (m *mboxHandler) UnlockMaildrop() error {
	log.Debugf("Mailbox for user %q would be unlocked", m.username)
	return nil
}

type POP3Service struct {
	config *Config
	db     Store
	pm     passwords.Passwords
	tasks  *Dispatcher
}

// NewPOP3Service ...
func NewPOP3Service(config *Config, db Store, pm passwords.Passwords, tasks *Dispatcher) *POP3Service {
	svc := &POP3Service{config, db, pm, tasks}

	return svc
}

func (s *POP3Service) getHandler() func(peer net.Addr) popart.Handler {
	return func(peer net.Addr) popart.Handler {
		log.Infof("Incoming connection from %q", peer)
		return NewMboxHandler(s.config, s.db, s.pm, s.tasks)
	}
}

func (s *POP3Service) Start() {
	go func() {
		if err := s.ListenAndServe(); err != nil {
			log.WithError(err).Error("error running POP3 service")
		}
	}()
}

func (s *POP3Service) Stop() {}

func (s *POP3Service) ListenAndServe() error {
	listener, err := net.Listen("tcp", s.config.POP3Bind)
	if err != nil {
		log.WithError(err).Error("error creating listener")
		return fmt.Errorf("error creating listener: %w", err)
	}

	svr := &popart.Server{
		Hostname:        HostnameFromURL(s.config.BaseURL),
		OnNewConnection: s.getHandler(),
		Timeout:         10 * time.Minute,
		APOP:            false,
	}

	return svr.Serve(listener)
}

type SMTPService struct {
	config *Config
	db     Store
	pm     passwords.Passwords
	tasks  *Dispatcher
}

// NewSMTPService ...
func NewSMTPService(config *Config, db Store, pm passwords.Passwords, tasks *Dispatcher) *SMTPService {
	svc := &SMTPService{config, db, pm, tasks}

	return svc
}

func (s *SMTPService) authHandler() smtpd.AuthHandler {
	failures := NewTTLCache(5 * time.Minute)
	return func(remoteAddr net.Addr, mechanism string, username []byte, password []byte, shared []byte) (bool, error) {
		// Error: no username or password provided
		if username == nil || password == nil {
			log.Warn("no username or password provided")
			return false, nil
		}

		// Lookup user
		user, err := s.db.GetUser(string(username))
		if err != nil {
			return false, err
		}

		if failures.Get(user.Username) > MaxFailedLogins {
			return false, err
		}

		failures.Reset(user.Username)

		if mechanism == "CRAM-MD5" {
			messageMac := make([]byte, hex.DecodedLen(len(password)))
			n, err := hex.Decode(messageMac, password)
			if err != nil {
				return false, err
			}
			if !validMAC(md5.New, shared, messageMac[:n], []byte(user.SMTPToken)) {
				return false, nil
			}
			log.Infof("SMTP login successful: %s", username)
			return true, nil
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(user.SMTPToken), 10)
		if err != nil {
			return false, err
		}
		if err := bcrypt.CompareHashAndPassword(hash, password); err != nil {
			return false, err
		}

		log.Infof("SMTP login successful: %s", username)

		return true, nil
	}
}

func (s *SMTPService) rcptHandler() smtpd.HandlerRcpt {
	localDomain := HostnameFromURL(s.config.BaseURL)

	return func(remoteAddr net.Addr, from string, to string) bool {
		username, domain := splitEmailAddress(to)

		if domain == localDomain {
			return s.db.HasUser(username)
		}
		log.Warn("cross-pod messaging not implemented (yet)")
		return false
	}
}

func (s *SMTPService) mailHandler() smtpd.Handler {
	return func(origin net.Addr, from string, to []string, data []byte) error {
		msg, err := message.Read(bytes.NewReader(data))
		if message.IsUnknownCharset(err) {
			log.WithError(err).Warn("unknown encoding")
		} else if err != nil {
			log.WithError(err).Error("error parsing message")
			return fmt.Errorf("error parsing message: %w", err)
		}

		conf := &Config{Data: "./"}

		if err := storeMessage(conf, msg, to); err != nil {
			log.WithError(err).Error("error storing message")
			return fmt.Errorf("error storing message: %w", err)
		}

		return nil
	}
}

func (s *SMTPService) Start() {
	go func() {
		if err := s.ListenAndServe(); err != nil {
			log.WithError(err).Error("error running SMTP service")
		}
	}()
}

func (s *SMTPService) Stop() {}

func (s *SMTPService) ListenAndServe() error {
	authMechs := map[string]bool{"PLAIN": true, "LOGIN": true}

	srv := &smtpd.Server{
		Addr:         s.config.SMTPBind,
		Handler:      s.mailHandler(),
		HandlerRcpt:  s.rcptHandler(),
		Appname:      fmt.Sprintf("%x SMTP v%s", s.config.Name, twtxt.Version),
		Hostname:     HostnameFromURL(s.config.BaseURL),
		AuthMechs:    authMechs,
		AuthHandler:  s.authHandler(),
		AuthRequired: true,
	}

	return srv.ListenAndServe()
}
