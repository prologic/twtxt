package session

import (
	"encoding/json"
	"time"
)

// Map  ...
type Map map[string]string

// Session ...
type Session struct {
	ID        string    `json:"id"`
	Data      Map       `json:"data"`
	CreatedAt time.Time `json:"created"`
	ExpiresAt time.Time `json:"expires"`
}

func LoadSession(data []byte) (sess *Session, err error) {
	if err = json.Unmarshal(data, &sess); err != nil {
		return nil, err
	}

	if sess.Data == nil {
		sess.Data = make(Map)
	}

	return
}

func (sess *Session) Set(key, val string) {
	sess.Data[key] = val
}

func (sess *Session) Get(key string) (val string, ok bool) {
	val, ok = sess.Data[key]
	return
}

func (sess *Session) Has(key string) bool {
	_, ok := sess.Data[key]
	return ok
}

func (sess *Session) Del(key string) {
	delete(sess.Data, key)
}

func (sess *Session) Bytes() ([]byte, error) {
	data, err := json.Marshal(sess)
	if err != nil {
		return nil, err
	}
	return data, nil
}
