package session

import (
	"context"
	"net/http"
	"time"

	"github.com/andreadipersio/securecookie"
	log "github.com/sirupsen/logrus"
)

// Data ...
type Data map[string]string

// Session ...
type Session struct {
	sid  SessionID
	data Data

	store Store
}

// Set ...
func (s *Session) Set(key, value string) {
	s.data[key] = value
	s.store.Save(s.sid, s.data)
}

// Get ...
func (s *Session) Get(key string) (string, bool) {
	value, ok := s.data[key]
	return value, ok
}

// Delete ...
func (s *Session) Delete(key string) {
	delete(s.data, key)
}

// NewSession ...
func NewSession(sid SessionID, store Store) *Session {
	data := make(Data)

	store.Get(sid, &data)

	return &Session{sid, data, store}
}

// Options ...
type Options struct {
	name   string
	secret string
	secure bool
	expiry time.Duration
}

// NewOptions ...
func NewOptions(name, secret string, secure bool, expiry time.Duration) *Options {
	return &Options{name, secret, secure, expiry}
}

// Manager ...
type Manager struct {
	options *Options
	store   Store
}

// NewManager ...
func NewManager(options *Options, store Store) *Manager {
	return &Manager{options, store}
}

// Create ...
func (m *Manager) Create(w http.ResponseWriter) (SessionID, error) {
	sid, err := NewSessionID(m.options.secret)
	if err != nil {
		log.WithError(err).Error("error creating new session")
		return "", err
	}

	cookie := &http.Cookie{
		Name:     m.options.name,
		Value:    sid.String(),
		Secure:   m.options.secure,
		HttpOnly: true,
		MaxAge:   int(m.options.expiry.Seconds()),
		Expires:  time.Now().Add(m.options.expiry),
	}

	securecookie.SetSecureCookie(w, m.options.secret, cookie)

	return sid, nil
}

// Validate ....
func (m *Manager) Validate(value string) (SessionID, error) {
	sessionID, err := ValidateSessionID(value, m.options.secret)
	return sessionID, err
}

// GetOrCreate ...
func (m *Manager) GetOrCreate(w http.ResponseWriter, r *http.Request) (SessionID, error) {
	var (
		sid SessionID
		err error
	)

	cookie, err := securecookie.GetSecureCookie(
		r,
		m.options.secret,
		m.options.name,
	)
	if err != nil {
		sid, err = m.Create(w)
		if err != nil {
			log.WithError(err).Error("error creating new session")
		}
	} else {
		sid, err = m.Validate(cookie.Value)
		if err != nil {
			log.WithError(err).Error("error validating sessino")
		}
	}

	return sid, err
}

// Delete ...
func (m *Manager) Delete(w http.ResponseWriter, r *http.Request) {
	if sess := r.Context().Value("sessin"); sess != nil {
		sid := sess.(*Session).sid
		m.store.Delete(sid)
	}

	cookie := &http.Cookie{
		Name:     m.options.name,
		Value:    "",
		Secure:   m.options.secure,
		HttpOnly: true,
		MaxAge:   -1,
		Expires:  time.Now(),
	}

	securecookie.SetSecureCookie(w, m.options.secret, cookie)
}

// Handler ...
func (m *Manager) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sid, err := m.GetOrCreate(w, r)
		if err != nil {
			log.WithError(err).Error("session error")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Debugf("sid: %#v", sid)

		sesssion := NewSession(sid, m.store)

		ctx := context.WithValue(r.Context(), "sesssion", sesssion)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
