package internal

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"hash"
	"net"
	"net/mail"
	"strings"

	"github.com/emersion/go-message"
	"github.com/jointwt/twtxt/internal/passwords"
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
			log.WithError(err).Error("error writing message for %s", username)
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
	return func(remoteAddr net.Addr, mechanism string, username []byte, password []byte, shared []byte) (bool, error) {
		/*
			// #239: Throttle failed login attempts and lock user  account.
			failures := NewTTLCache(5 * time.Minute)

			return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
				ctx := NewContext(s.config, s.db, r)

				if r.Method == "GET" {
					s.render("login", w, ctx)
					return
				}

				username := NormalizeUsername(r.FormValue("username"))
				password := r.FormValue("password")
				rememberme := r.FormValue("rememberme") == "on"

				// Error: no username or password provided
				if username == "" || password == "" {
					log.Warn("no username or password provided")
					http.Redirect(w, r, "/login", http.StatusFound)
					return
				}

				// Lookup user
				user, err := s.db.GetUser(username)
				if err != nil {
					ctx.Error = true
					ctx.Message = "Invalid username! Hint: Register an account?"
					s.render("error", w, ctx)
					return
				}

				// #239: Throttle failed login attempts and lock user  account.
				if failures.Get(user.Username) > MaxFailedLogins {
					ctx.Error = true
					ctx.Message = "Too many failed login attempts. Account temporarily locked! Please try again later."
					s.render("error", w, ctx)
					return
				}

				// Validate cleartext password against KDF hash
				err = s.pm.CheckPassword(user.Password, password)
				if err != nil {
					// #239: Throttle failed login attempts and lock user  account.
					failed := failures.Inc(user.Username)
					time.Sleep(time.Duration(IntPow(2, failed)) * time.Second)

					ctx.Error = true
					ctx.Message = "Invalid password! Hint: Reset your password?"
					s.render("error", w, ctx)
					return
				}

				// #239: Throttle failed login attempts and lock user  account.
				failures.Reset(user.Username)

				// Login successful
				log.Infof("login successful: %s", username)
		*/

		if string(username) != "admin" {
			return false, fmt.Errorf("error invalid credentials")
		}

		if mechanism == "CRAM-MD5" {
			messageMac := make([]byte, hex.DecodedLen(len(password)))
			n, err := hex.Decode(messageMac, password)
			if err != nil {
				return false, err
			}
			return validMAC(md5.New, shared, messageMac[:n], []byte("admin")), nil
		}
		hash, err := bcrypt.GenerateFromPassword([]byte("admin"), 10)
		if err != nil {
			return false, err
		}
		err = bcrypt.CompareHashAndPassword(hash, password)
		return err == nil, err
	}
}

func (s *SMTPService) rcptHandler() smtpd.HandlerRcpt {
	return func(remoteAddr net.Addr, from string, to string) bool {
		_, domain := splitEmailAddress(to)
		return domain == "twtxt.net"
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
