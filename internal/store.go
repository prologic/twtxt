package internal

import (
	"errors"
	"fmt"

	"github.com/prologic/twtxt/internal/session"
)

var (
	ErrInvalidStore   = errors.New("error: invalid store")
	ErrUserNotFound   = errors.New("error: user not found")
	ErrFeedNotFound   = errors.New("error: feed not found")
	ErrInvalidSession = errors.New("error: invalid session")
)

type Store interface {
	Merge() error
	Close() error
	Sync() error

	DelFeed(name string) error
	HasFeed(name string) bool
	GetFeed(name string) (*Feed, error)
	SetFeed(name string, user *Feed) error
	SearchFeeds(prefix string) []string

	DelUser(username string) error
	HasUser(username string) bool
	GetUser(username string) (*User, error)
	SetUser(username string, user *User) error
	SearchUsers(prefix string) []string

	GetAllFeeds() ([]*Feed, error)
	GetAllUsers() ([]*User, error)

	GetSession(sid string) (*session.Session, error)
	SetSession(sid string, sess *session.Session) error
	HasSession(sid string) bool
	DelSession(sid string) error
	SyncSession(sess *session.Session) error
	GetAllSessions() ([]*session.Session, error)

	GetUserTokens(user *User) ([]*Token, error)
	SetToken(signature string, token *Token) error
}

func NewStore(store string) (Store, error) {
	u, err := ParseURI(store)
	if err != nil {
		return nil, fmt.Errorf("error parsing store uri: %s", err)
	}

	switch u.Type {
	case "bitcask":
		return newBitcaskStore(u.Path)
	default:
		return nil, ErrInvalidStore
	}
}
