package twtxt

import (
	"encoding/json"
	"time"
)

type User struct {
	Username  string
	Password  string
	Email     string
	URL       string
	CreatedAt time.Time

	Following map[string]string

	sources map[string]string
}

func LoadUser(data []byte) (user *User, err error) {
	if err = json.Unmarshal(data, &user); err != nil {
		return nil, err
	}

	if user.Following == nil {
		user.Following = make(map[string]string)
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
