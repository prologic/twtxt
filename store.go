package twtxt

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidStore   = errors.New("error: invalid store")
	ErrUserNotFound   = errors.New("error: user not found")
	ErrFeedNotFound   = errors.New("error: feed not found")
	ErrInvalidSession = errors.New("error: invalid session")
)

type Store interface {
	Close() error
	Sync() error

	DelFeed(name string) error
	HasFeed(name string) bool
	GetFeed(name string) (*Feed, error)
	SetFeed(name string, user *Feed) error

	DelUser(username string) error
	HasUser(username string) bool
	GetUser(username string) (*User, error)
	SetUser(username string, user *User) error

	GetAllFeeds() ([]*Feed, error)
	GetAllUsers() ([]*User, error)
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
