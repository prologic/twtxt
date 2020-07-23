package twtxt

import (
	"fmt"

	"github.com/prologic/bitcask"
	log "github.com/sirupsen/logrus"
)

// BitcaskStore ...
type BitcaskStore struct {
	db *bitcask.Bitcask
}

func newBitcaskStore(path string) (*BitcaskStore, error) {
	db, err := bitcask.Open(path)
	if err != nil {
		return nil, err
	}

	return &BitcaskStore{db: db}, nil
}

// Close ...
func (bs *BitcaskStore) Close() error {
	if err := bs.db.Sync(); err != nil {
		log.WithError(err).Error("error syncing store")
		return err
	}

	if err := bs.db.Close(); err != nil {
		log.WithError(err).Error("error closing store")
		return err
	}

	return nil
}

func (bs *BitcaskStore) DelUser(username string) error {
	return bs.db.Delete([]byte(fmt.Sprintf("/users/%s", username)))
}

func (bs *BitcaskStore) GetUser(username string) (*User, error) {
	data, err := bs.db.Get([]byte(fmt.Sprintf("/users/%s", username)))
	if err == bitcask.ErrKeyNotFound {
		return nil, ErrUserNotFound
	}
	return LoadUser(data)
}

func (bs *BitcaskStore) SetUser(username string, user *User) error {
	data, err := user.Bytes()
	if err != nil {
		return err
	}

	if err := bs.db.Put([]byte(fmt.Sprintf("/users/%s", username)), data); err != nil {
		return err
	}
	return nil
}

func (bs *BitcaskStore) GetAllUsers() ([]*User, error) {
	var users []*User

	err := bs.db.Scan([]byte("/users"), func(key []byte) error {
		data, err := bs.db.Get(key)
		if err != nil {
			return err
		}

		user, err := LoadUser(data)
		if err != nil {
			return err
		}
		users = append(users, user)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return users, nil
}

func (bs *BitcaskStore) GetSession(sid string) (*Session, error) {
	data, err := bs.db.Get([]byte(fmt.Sprintf("/sessions/%s", sid)))
	if err == bitcask.ErrKeyNotFound {
		return nil, ErrInvalidSession
	}
	return LoadSession(data)
}

func (bs *BitcaskStore) SetSession(sid string, session *Session) error {
	data, err := session.Bytes()
	if err != nil {
		return err
	}

	if err := bs.db.Put([]byte(fmt.Sprintf("/sessions/%s", sid)), data); err != nil {
		return err
	}
	return nil
}
