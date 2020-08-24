package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apex/log"
	"github.com/creasty/defaults"
	"github.com/prologic/twtxt/types"
)

const (
	maxUserFeeds = 5 // 5 is < 7 and humans can only really handle ~7 things
)

var (
	ErrFeedAlreadyExists = errors.New("error: feed already exists by that name")
	ErrTooManyFeeds      = errors.New("error: you have too many feeds")
)

// Feed ...
type Feed struct {
	Name        string
	Description string
	URL         string
	CreatedAt   time.Time

	Followers map[string]string `default:"{}"`

	remotes map[string]string
}

// User ...
type User struct {
	Username  string
	Password  string
	Tagline   string
	Email     string
	URL       string
	CreatedAt time.Time

	Theme                      string `default:"auto"`
	DisplayDatesInTimezone     string `default:"UTC"`
	IsFollowersPubliclyVisible bool   `default:"true"`
	IsFollowingPubliclyVisible bool   `default:"true"`

	Feeds  []string `default:"[]"`
	Tokens []string `default:"[]"`

	Followers map[string]string `default:"{}"`
	Following map[string]string `default:"{}"`

	remotes map[string]string
	sources map[string]string
}

// Token ...
type Token struct {
	Value     string    // -- The actual JWT token value which stored encrypted/signed data as given to the client using it.
	UserAgent string    // -- The name of the client and version number that initially created the token.
	CreatedAt time.Time // -- The date/time the token was created
	ExpiresAt time.Time // -- The date/time the token expires (if ever)

}

func CreateFeed(conf *Config, db Store, user *User, name string, force bool) error {
	if user != nil {
		if !force && len(user.Feeds) > maxUserFeeds {
			return ErrTooManyFeeds
		}
	}

	fn := filepath.Join(conf.Data, feedsDir, name)
	stat, err := os.Stat(fn)

	if err == nil && !force {
		return ErrFeedAlreadyExists
	}

	if stat == nil {
		if err := ioutil.WriteFile(fn, []byte{}, 0644); err != nil {
			return err
		}
	}

	if user != nil {
		if !user.OwnsFeed(name) {
			user.Feeds = append(user.Feeds, name)
		}
	}

	followers := make(map[string]string)
	if user != nil {
		followers[user.Username] = user.URL
	}

	feed := NewFeed()
	feed.Name = name
	feed.URL = URLForUser(conf.BaseURL, name)
	feed.Followers = followers
	feed.CreatedAt = time.Now()

	if err := db.SetFeed(name, feed); err != nil {
		return err
	}

	if user != nil {
		user.Follow(name, feed.URL)
	}

	return nil
}

func DetachFeedFromOwner(db Store, user *User, feed *Feed) (err error) {
	delete(user.Following, feed.Name)
	delete(user.sources, feed.URL)

	user.Feeds = RemoveString(user.Feeds, feed.Name)
	if err = db.SetUser(user.Username, user); err != nil {
		return
	}

	delete(feed.Followers, user.Username)
	if err = db.SetFeed(feed.Name, feed); err != nil {
		return
	}

	return nil
}

// NewFeed ...
func NewFeed() *Feed {
	feed := &Feed{}
	if err := defaults.Set(feed); err != nil {
		log.WithError(err).Error("error creating new feed object")
	}
	return feed
}

// LoadFeed ...
func LoadFeed(data []byte) (feed *Feed, err error) {
	feed = &Feed{}
	if err := defaults.Set(feed); err != nil {
		return nil, err
	}

	if err = json.Unmarshal(data, &feed); err != nil {
		return nil, err
	}

	if feed.Followers == nil {
		feed.Followers = make(map[string]string)
	}

	feed.remotes = make(map[string]string)
	for n, u := range feed.Followers {
		if u = NormalizeURL(u); u == "" {
			continue
		}
		feed.remotes[u] = n
	}

	return
}

// NewUser ...
func NewUser() *User {
	user := &User{}
	if err := defaults.Set(user); err != nil {
		log.WithError(err).Error("error creating new user object")
	}
	return user
}

func LoadUser(data []byte) (user *User, err error) {
	user = &User{}
	if err := defaults.Set(user); err != nil {
		return nil, err
	}

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

func (f *Feed) FollowedBy(url string) bool {
	_, ok := f.remotes[NormalizeURL(url)]
	return ok
}

func (f *Feed) Profile() Profile {
	return Profile{
		Type: "Feed",

		Username: f.Name,
		Tagline:  f.Description,
		URL:      f.URL,

		Followers: f.Followers,
	}
}

func (f *Feed) Bytes() ([]byte, error) {
	data, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (u *User) AddToken(token string) {
	if !u.HasToken(token) {
		u.Tokens = append(u.Tokens, token)
	}
}

func (u *User) HasToken(token string) bool {
	for _, t := range u.Tokens {
		if t == token {
			return true
		}
	}
	return false
}

func (u *User) OwnsFeed(name string) bool {
	name = NormalizeFeedName(name)
	for _, feed := range u.Feeds {
		if NormalizeFeedName(feed) == name {
			return true
		}
	}
	return false
}

func (u *User) Is(url string) bool {
	if NormalizeURL(url) == "" {
		return false
	}
	return u.URL == NormalizeURL(url)
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

func (u *User) Profile() Profile {
	return Profile{
		Type: "User",

		Username: u.Username,
		Tagline:  u.Tagline,
		URL:      u.URL,

		Followers: u.Followers,
		Following: u.Following,
	}
}

func (u *User) Twter() types.Twter {
	return types.Twter{Nick: u.Username, URL: u.URL}
}

func (u *User) Reply(twt types.Twt) string {
	mentionsSet := make(map[string]bool)
	for _, twter := range twt.Mentions() {
		if _, ok := mentionsSet[twter.Nick]; !ok && twter.Nick != u.Username {
			mentionsSet[twter.Nick] = true
		}
	}

	mentions := []string{fmt.Sprintf("@%s", twt.Twter.Nick)}
	for nick := range mentionsSet {
		mentions = append(mentions, fmt.Sprintf("@%s", nick))
	}

	subject := twt.Subject()

	if subject != "" {
		subject = FormatMentionsAndTagsForSubject(subject)
		return fmt.Sprintf("%s %s ", strings.Join(mentions, " "), subject)
	}
	return fmt.Sprintf("%s ", strings.Join(mentions, " "))
}

func (u *User) Bytes() ([]byte, error) {
	data, err := json.Marshal(u)
	if err != nil {
		return nil, err
	}
	return data, nil
}
