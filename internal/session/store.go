package session

import (
	"errors"
	"time"
)

// DefaultSessionDuration is the default duration for
// saving session data in the store. Most Store implementations
// will automatically delete saved session data after this time.
const DefaultSessionDuration = time.Hour

var (
	ErrSessionNotFound = errors.New("session not found or expired")
	ErrSessionExpired  = errors.New("session expired")
)

// Store represents a session data store.
// This is an abstract interface that can be implemented
// against several different types of data stores. For example,
// session data could be stored in memory in a concurrent map,
// or more typically in a shared key/value server store like redis.
type Store interface {
	GetSession(sid string) (*Session, error)
	SetSession(sid string, sess *Session) error
	HasSession(sid string) bool
	DelSession(sid string) error

	SyncSession(sess *Session) error

	GetAllSessions() ([]*Session, error)
}
