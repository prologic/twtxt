package session

import (
	"context"
	"net/http"
	"time"

	"github.com/andreadipersio/securecookie"
	log "github.com/sirupsen/logrus"
)

// Key ...
type Key int

const (
	SessionKey Key = iota
)

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
func (m *Manager) Create(w http.ResponseWriter) (*Session, error) {
	sid, err := NewSessionID(m.options.secret)
	if err != nil {
		log.WithError(err).Error("error creating new session")
		return nil, err
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

	return &Session{
		store: m.store,

		ID:        sid.String(),
		Data:      make(Map),
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(m.options.expiry),
	}, nil
}

// Validate ....
func (m *Manager) Validate(value string) (SessionID, error) {
	sessionID, err := ValidateSessionID(value, m.options.secret)
	return sessionID, err
}

// GetOrCreate ...
func (m *Manager) GetOrCreate(w http.ResponseWriter, r *http.Request) (*Session, error) {
	cookie, err := securecookie.GetSecureCookie(
		r,
		m.options.secret,
		m.options.name,
	)
	if err != nil {
		sess, err := m.Create(w)
		if err != nil {
			log.WithError(err).Error("error creating new session")
			return nil, err
		}
		if err = m.store.SetSession(sess.ID, sess); err != nil {
			log.WithError(err).Errorf("error creating new session for %s", sess.ID)
			return nil, err
		}
		return sess, nil
	}

	sid, err := m.Validate(cookie.Value)
	if err != nil {
		log.WithError(err).Error("error validating seesion")
		return nil, err
	}

	sess, err := m.store.GetSession(sid.String())
	if err != nil {
		if err == ErrSessionNotFound {
			log.WithError(err).Warnf("no session found for %s (creating new one)", sid)
			m.Delete(w, r)

			sess, err := m.Create(w)
			if err != nil {
				log.WithError(err).Error("error creating new session")
				return nil, err
			}
			if err = m.store.SetSession(sess.ID, sess); err != nil {
				log.WithError(err).Errorf("error creating new session for %s", sess.ID)
				return nil, err
			}
			return sess, nil
		}
		log.WithError(err).Errorf("error loading session for %s", sid)
		return nil, err
	}

	sess.store = m.store

	return sess, nil
}

// Delete ...
func (m *Manager) Delete(w http.ResponseWriter, r *http.Request) {
	if sess := r.Context().Value(SessionKey); sess != nil {
		sess := sess.(*Session)
		if err := m.store.DelSession(sess.ID); err != nil {
			log.WithError(err).Warnf("error deleting session %s", sess.ID)
		}
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
		sess, err := m.GetOrCreate(w, r)
		if err != nil {
			log.WithError(err).Error("session error")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		ctx := context.WithValue(r.Context(), SessionKey, sess)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
