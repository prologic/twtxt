package internal

import (
	"fmt"
	"strings"

	"github.com/prologic/bitcask"
	log "github.com/sirupsen/logrus"

	"github.com/prologic/twtxt/internal/session"
)

const (
	sessionsKeyPrefix = "/sessions"
)

// BitcaskStore ...
type BitcaskStore struct {
	db *bitcask.Bitcask
}

func newBitcaskStore(path string) (*BitcaskStore, error) {
	db, err := bitcask.Open(
		path,
		bitcask.WithMaxKeySize(256),
	)
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

// Merge ...
func (bs *BitcaskStore) Merge() error {
	log.Info("merging store ...")
	if err := bs.db.Merge(); err != nil {
		log.WithError(err).Error("error merging store")
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

func (bs *BitcaskStore) SearchFeeds(prefix string) []string {
	var keys []string

	bs.db.Scan([]byte("/feeds/"), func(key []byte) error {
		if strings.HasPrefix(strings.ToLower(string(key)), prefix) {
			keys = append(keys, strings.TrimPrefix(string(key), "/feeds/"))
		}
		return nil
	})

	return keys
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

func (bs *BitcaskStore) SearchUsers(prefix string) []string {
	var keys []string

	bs.db.Scan([]byte("/users/"), func(key []byte) error {
		if strings.HasPrefix(strings.ToLower(string(key)), prefix) {
			keys = append(keys, strings.TrimPrefix(string(key), "/users/"))
		}
		return nil
	})

	return keys
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

func (bs *BitcaskStore) GetSession(sid string) (*session.Session, error) {
	key := []byte(fmt.Sprintf("%s/%s", sessionsKeyPrefix, sid))
	data, err := bs.db.Get(key)
	if err != nil {
		if err == bitcask.ErrKeyNotFound {
			return nil, session.ErrSessionNotFound
		}
		return nil, err
	}
	return session.LoadSession(data)
}

func (bs *BitcaskStore) SetSession(sid string, sess *session.Session) error {
	key := []byte(fmt.Sprintf("%s/%s", sessionsKeyPrefix, sid))

	data, err := sess.Bytes()
	if err != nil {
		return err
	}

	return bs.db.Put(key, data)
}

func (bs *BitcaskStore) HasSession(sid string) bool {
	key := []byte(fmt.Sprintf("%s/%s", sessionsKeyPrefix, sid))
	return bs.db.Has(key)
}

func (bs *BitcaskStore) DelSession(sid string) error {
	key := []byte(fmt.Sprintf("%s/%s", sessionsKeyPrefix, sid))
	return bs.db.Delete(key)
}

func (bs *BitcaskStore) SyncSession(sess *session.Session) error {
	return bs.SetSession(sess.ID, sess)
}

func (bs *BitcaskStore) GetAllSessions() ([]*session.Session, error) {
	var sessions []*session.Session

	err := bs.db.Scan([]byte(sessionsKeyPrefix), func(key []byte) error {
		data, err := bs.db.Get(key)
		if err != nil {
			return err
		}

		session, err := session.LoadSession(data)
		if err != nil {
			return err
		}
		sessions = append(sessions, session)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return sessions, nil
}

func (bs *BitcaskStore) GetUserTokens(user *User) ([]*Token, error) {
	tokens := []*Token{}
	for _, signature := range user.Tokens {

		fmt.Printf("looking up %s\n", signature)
		data, err := bs.db.Get([]byte(fmt.Sprintf("/token/%s", signature)))
		if err == bitcask.ErrKeyNotFound {
			return nil, ErrTokenNotFound
		}
		tkn, err := LoadToken(data)

		if err != nil {
			return tokens, err
		}

		tokens = append(tokens, tkn)
	}

	return tokens, nil
}

func (bs *BitcaskStore) SetToken(signature string, tkn *Token) error {

	data, err := tkn.Bytes()
	if err != nil {
		return err
	}

	if err := bs.db.Put([]byte(fmt.Sprintf("/token/%s", signature)), data); err != nil {
		return err
	}
	return nil
}

func (bs *BitcaskStore) DelToken(signature string) error {
	return bs.db.Delete([]byte(fmt.Sprintf("/token/%s", signature)))
}
