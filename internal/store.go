package internal

import (
	"errors"
	"fmt"

	"github.com/jointwt/twtxt/internal/session"
)

var (
	ErrInvalidStore   = errors.New("error: invalid store")
	ErrUserNotFound   = errors.New("error: user not found")
	ErrTokenNotFound  = errors.New("error: token not found")
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
	LenFeeds() int64
	SearchFeeds(prefix string) []string
	GetAllFeeds() ([]*Feed, error)

	DelUser(username string) error
	HasUser(username string) bool
	GetUser(username string) (*User, error)
	SetUser(username string, user *User) error
	LenUsers() int64
	SearchUsers(prefix string) []string
	GetAllUsers() ([]*User, error)

	GetSession(sid string) (*session.Session, error)
	SetSession(sid string, sess *session.Session) error
	HasSession(sid string) bool
	DelSession(sid string) error
	SyncSession(sess *session.Session) error
	LenSessions() int64
	GetAllSessions() ([]*session.Session, error)

	GetUserTokens(user *User) ([]*Token, error)
	SetToken(signature string, token *Token) error
	DelToken(signature string) error
	LenTokens() int64
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
