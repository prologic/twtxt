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

// Sync ...
func (bs *BitcaskStore) Sync() error {
	return bs.db.Sync()
}

// Close ...
func (bs *BitcaskStore) Close() error {
	log.Info("syncing store ...")
	if err := bs.db.Sync(); err != nil {
		log.WithError(err).Error("error syncing store")
		return err
	}

	log.Info("closing store ...")
	if err := bs.db.Close(); err != nil {
		log.WithError(err).Error("error closing store")
		return err
	}

	return nil
}

func (bs *BitcaskStore) HasFeed(name string) bool {
	return bs.db.Has([]byte(fmt.Sprintf("/feeds/%s", name)))
}

func (bs *BitcaskStore) DelFeed(name string) error {
	return bs.db.Delete([]byte(fmt.Sprintf("/feeds/%s", name)))
}

func (bs *BitcaskStore) GetFeed(name string) (*Feed, error) {
	data, err := bs.db.Get([]byte(fmt.Sprintf("/feeds/%s", name)))
	if err == bitcask.ErrKeyNotFound {
		return nil, ErrFeedNotFound
	}
	return LoadFeed(data)
}

func (bs *BitcaskStore) SetFeed(name string, feed *Feed) error {
	data, err := feed.Bytes()
	if err != nil {
		return err
	}

	if err := bs.db.Put([]byte(fmt.Sprintf("/feeds/%s", name)), data); err != nil {
		return err
	}
	return nil
}

func (bs *BitcaskStore) GetAllFeeds() ([]*Feed, error) {
	var feeds []*Feed

	err := bs.db.Scan([]byte("/feeds"), func(key []byte) error {
		data, err := bs.db.Get(key)
		if err != nil {
			return err
		}

		feed, err := LoadFeed(data)
		if err != nil {
			return err
		}
		feeds = append(feeds, feed)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return feeds, nil
}

func (bs *BitcaskStore) HasUser(username string) bool {
	return bs.db.Has([]byte(fmt.Sprintf("/users/%s", username)))
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
