package internal

import (
	"time"

	"github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"

	"github.com/jointwt/twtxt/internal/session"
)

// SessionStore ...
type SessionStore struct {
	store  Store
	cached *cache.Cache
}

func NewSessionStore(store Store, sessionCacheTTL time.Duration) *SessionStore {
	return &SessionStore{
		store:  store,
		cached: cache.New(sessionCacheTTL, time.Minute*5),
	}
}

func (s *SessionStore) Count() int {
	return s.cached.ItemCount()
}

func (s *SessionStore) GetSession(sid string) (*session.Session, error) {
	val, found := s.cached.Get(sid)
	if found {
		return val.(*session.Session), nil
	}

	return s.store.GetSession(sid)
}

func (s *SessionStore) SetSession(sid string, sess *session.Session) error {
	s.cached.Set(sid, sess, cache.DefaultExpiration)
	if persist, ok := sess.Get("persist"); !ok || persist != "1" {
		return nil
	}

	return s.store.SetSession(sid, sess)
}

func (s *SessionStore) HasSession(sid string) bool {
	_, ok := s.cached.Get(sid)
	if ok {
		return true
	}

	return s.store.HasSession(sid)
}

func (s *SessionStore) DelSession(sid string) error {
	if s.store.HasSession(sid) {
		if err := s.store.DelSession(sid); err != nil {
			log.WithError(err).Errorf("error deleting persistent session %s", sid)
			return err
		}
	}
	s.cached.Delete(sid)
	return nil
}

func (s *SessionStore) SyncSession(sess *session.Session) error {
	if persist, ok := sess.Get("persist"); ok && persist == "1" {
		if err := s.store.SetSession(sess.ID, sess); err != nil {
			log.WithError(err).Errorf("error persisting session %s", sess.ID)
			return err
		}
	}

	return s.SetSession(sess.ID, sess)
}

func (s *SessionStore) GetAllSessions() ([]*session.Session, error) {
	var sessions []*session.Session
	for _, item := range s.cached.Items() {
		sess := item.Object.(*session.Session)
		sessions = append(sessions, sess)
	}
	persistedSessions, err := s.store.GetAllSessions()
	if err != nil {
		log.WithError(err).Error("error getting all persisted sessions")
		return sessions, err
	}
	return append(sessions, persistedSessions...), nil
}
