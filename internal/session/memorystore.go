package session

import (
	"time"

	"github.com/patrickmn/go-cache"
)

// MemoryStore represents an in-memory session store.
// This should be used only for testing and prototyping.
// Production systems should use a shared server store like redis
type MemoryStore struct {
	entries *cache.Cache
}

// NewMemoryStore constructs and returns a new MemoryStore
func NewMemoryStore(sessionDuration time.Duration) *MemoryStore {
	if sessionDuration < 0 {
		sessionDuration = DefaultSessionDuration
	}
	return &MemoryStore{
		entries: cache.New(sessionDuration, time.Minute),
	}
}

// GetSession ...
func (s *MemoryStore) GetSession(sid string) (*Session, error) {
	val, found := s.entries.Get(sid)
	if !found {
		return nil, ErrSessionNotFound
	}
	sess := val.(*Session)
	return sess, nil
}

// SetSession ...
func (s *MemoryStore) SetSession(sid string, sess *Session) error {
	s.entries.Set(sid, sess, cache.DefaultExpiration)
	return nil
}

// HasSession ...
func (s *MemoryStore) HasSession(sid string) bool {
	_, ok := s.entries.Get(sid)
	return ok
}

// DelSession ...
func (s *MemoryStore) DelSession(sid string) error {
	s.entries.Delete(sid)
	return nil
}

// SyncSession ...
func (s *MemoryStore) SyncSession(sess *Session) error {
	return nil
}

// GetAllSessions ...
func (s *MemoryStore) GetAllSessions() ([]*Session, error) {
	var sessions []*Session
	for _, item := range s.entries.Items() {
		sess := item.Object.(*Session)
		sessions = append(sessions, sess)
	}
	return sessions, nil
}
