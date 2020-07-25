package twtxt

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxUserFeeds = 5 // 5 is < 7 and humans can only really handle ~7 things
)

var (
	ErrFeedAlreadyExists = errors.New("error: feed already exists by that name")
	ErrTooManyFeeds      = errors.New("error: you have too many feeds")
)

type User struct {
	Username  string
	Password  string
	Tagline   string
	Email     string
	URL       string
	CreatedAt time.Time

	Feeds []string

	Followers map[string]string
	Following map[string]string

	remotes map[string]string
	sources map[string]string
}

func LoadUser(data []byte) (user *User, err error) {
	if err = json.Unmarshal(data, &user); err != nil {
		return nil, err
	}

	if user.Followers == nil {
		user.Followers = make(map[string]string)
	}
	if user.Following == nil {
		user.Following = make(map[string]string)
	}

	user.remotes = make(map[string]string)
	for n, u := range user.Followers {
		if u = NormalizeURL(u); u == "" {
			continue
		}
		user.remotes[u] = n
	}

	user.sources = make(map[string]string)
	for n, u := range user.Following {
		if u = NormalizeURL(u); u == "" {
			continue
		}
		user.sources[u] = n
	}

	return
}

func (u *User) OwnsFeed(name string) bool {
	name = strings.ToLower(name)
	for _, feed := range u.Feeds {
		if strings.ToLower(feed) == name {
			return true
		}
	}
	return false
}

func (u *User) CreateFeed(path, name string) error {
	if len(u.Feeds) > maxUserFeeds {
		return ErrTooManyFeeds
	}

	p := filepath.Join(path, feedsDir, name)
	if _, err := os.Stat(p); err == nil {
		return ErrFeedAlreadyExists
	}

	if err := ioutil.WriteFile(p, []byte{}, 0644); err != nil {
		return err
	}

	u.Feeds = append(u.Feeds, name)

	return nil
}

func (u *User) Is(username string) bool {
	return NormalizeUsername(u.Username) == NormalizeUsername(username)
}

func (u *User) FollowedBy(url string) bool {
	_, ok := u.remotes[NormalizeURL(url)]
	return ok
}

func (u *User) Follow(nick, url string) {
	if !u.Follows(url) {
		u.Following[nick] = url
		u.sources[url] = nick
	}
}

func (u *User) Follows(url string) bool {
	_, ok := u.sources[NormalizeURL(url)]
	return ok
}

func (u *User) Sources() map[string]string {
	return u.sources
}

func (u *User) Bytes() ([]byte, error) {
	data, err := json.Marshal(u)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Session ...
type Session struct {
	ID       int
	User     int
	Hash     string
	ExpireAt time.Time
}

func LoadSession(data []byte) (session *Session, err error) {
	if err = json.Unmarshal(data, session); err != nil {
		return nil, err
	}
	return
}

func (s *Session) Bytes() ([]byte, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return data, nil
}
