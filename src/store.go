package twtxt

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidStore   = errors.New("error: invalid store")
	ErrUserNotFound   = errors.New("error: user not found")
	ErrInvalidSession = errors.New("error: invalid session")
)

type Store interface {
	Close() error

	DelUser(username string) error
	GetUser(username string) (*User, error)
	SetUser(username string, user *User) error

	GetAllUsers() ([]*User, error)

	GetSession(sid string) (*Session, error)
	SetSession(sid string, session *Session) error
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
