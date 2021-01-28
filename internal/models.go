package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/creasty/defaults"
	"github.com/jointwt/twtxt/types"
	log "github.com/sirupsen/logrus"
)

const (
	maxUserFeeds = 5 // 5 is < 7 and humans can only really handle ~7 things
)

var (
	ErrFeedAlreadyExists = errors.New("error: feed already exists by that name")
	ErrAlreadyFollows    = errors.New("error: you already follow this feed")
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
	Email     string // DEPRECATED: In favor of storing a Hashed Email
	URL       string
	CreatedAt time.Time

	Theme                      string `default:"auto"`
	Recovery                   string `default:"auto"`
	DisplayDatesInTimezone     string `default:"UTC"`
	IsFollowersPubliclyVisible bool   `default:"true"`
	IsFollowingPubliclyVisible bool   `default:"true"`
	IsBookmarksPubliclyVisible bool   `default:"true"`

	Feeds  []string `default:"[]"`
	Tokens []string `default:"[]"`

	SMTPToken string `default:""`
	POP3Token string `default:""`

	Bookmarks map[string]string `default:"{}"`
	Followers map[string]string `default:"{}"`
	Following map[string]string `default:"{}"`
	Muted     map[string]string `default:"{}"`

	muted   map[string]string
	remotes map[string]string
	sources map[string]string
}

// Token ...
type Token struct {
	Signature string
	Value     string
	UserAgent string
	CreatedAt time.Time
	ExpiresAt time.Time
}

func LoadToken(data []byte) (token *Token, err error) {
	token = &Token{}
	if err := defaults.Set(token); err != nil {
		return nil, err
	}

	if err = json.Unmarshal(data, &token); err != nil {
		return nil, err
	}

	return
}

func (t *Token) Bytes() ([]byte, error) {
	data, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}
	return data, nil
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
	feed.URL = URLForUser(conf, name)
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

func RemoveFeedOwnership(db Store, user *User, feed *Feed) (err error) {
	user.Feeds = RemoveString(user.Feeds, feed.Name)
	if err = db.SetUser(user.Username, user); err != nil {
		return
	}

	return nil
}

func AddFeedOwnership(db Store, user *User, feed *Feed) (err error) {
	user.Feeds = append(user.Feeds, feed.Name)
	if err = db.SetUser(user.Username, user); err != nil {
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

	if user.SMTPToken == "" {
		user.SMTPToken = GenerateRandomToken()
	}
	if user.POP3Token == "" {
		user.POP3Token = GenerateRandomToken()
	}

	if user.Bookmarks == nil {
		user.Bookmarks = make(map[string]string)
	}
	if user.Followers == nil {
		user.Followers = make(map[string]string)
	}
	if user.Following == nil {
		user.Following = make(map[string]string)
	}

	user.muted = make(map[string]string)
	for n, u := range user.Muted {
		if u = NormalizeURL(u); u == "" {
			continue
		}
		user.muted[u] = n
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

func (f *Feed) AddFollower(nick, url string) {
	url = NormalizeURL(url)
	f.Followers[nick] = url
	f.remotes[url] = nick
}

func (f *Feed) FollowedBy(url string) bool {
	_, ok := f.remotes[NormalizeURL(url)]
	return ok
}

func (f *Feed) Source() types.Feeds {
	feeds := make(types.Feeds)
	feeds[types.Feed{Nick: f.Name, URL: f.URL}] = true
	return feeds
}

func (f *Feed) Profile(baseURL string, viewer *User) types.Profile {
	var (
		follows    bool
		followedBy bool
		muted      bool
	)

	if viewer != nil {
		follows = viewer.Follows(f.URL)
		followedBy = viewer.FollowedBy(f.URL)
		muted = viewer.HasMuted(f.URL)
	}

	return types.Profile{
		Type: "Feed",

		Username: f.Name,
		Tagline:  f.Description,
		URL:      f.URL,
		BlogsURL: URLForBlogs(baseURL, f.Name),

		Follows:    follows,
		FollowedBy: followedBy,
		Muted:      muted,

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

func (u *User) String() string {
	url, err := url.Parse(u.URL)
	if err != nil {
		log.WithError(err).Warn("error parsing user url")
		return u.Username
	}
	return fmt.Sprintf("%s@%s", u.Username, url.Hostname())
}

// HasToken will add a token to a user if it doesn't exist already
func (u *User) AddToken(token *Token) {
	if !u.HasToken(token.Signature) {
		u.Tokens = append(u.Tokens, token.Signature)
	}
}

// HasToken will compare a token value with stored tokens
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

func (u *User) Bookmark(hash string) {
	if _, ok := u.Bookmarks[hash]; !ok {
		u.Bookmarks[hash] = ""
	} else {
		delete(u.Bookmarks, hash)
	}
}

func (u *User) Bookmarked(hash string) bool {
	_, ok := u.Bookmarks[hash]
	return ok
}

func (u *User) AddFollower(nick, url string) {
	url = NormalizeURL(url)
	u.Followers[nick] = url
	u.remotes[url] = nick
}

func (u *User) FollowedBy(url string) bool {
	_, ok := u.remotes[NormalizeURL(url)]
	return ok
}

func (u *User) Mute(nick, url string) {
	if !u.HasMuted(url) {
		u.Muted[nick] = url
		u.muted[url] = nick
	}
}

func (u *User) Unmute(nick string) {
	url, ok := u.Muted[nick]
	if ok {
		delete(u.Muted, nick)
		delete(u.muted, url)
	}
}

func (u *User) Follow(nick, url string) {
	if !u.Follows(url) {
		u.Following[nick] = url
		u.sources[url] = nick
	}
}

func (u *User) FollowAndValidate(conf *Config, nick, url string) error {
	if err := ValidateFeed(conf, nick, url); err != nil {
		return err
	}

	if u.Follows(url) {
		return ErrAlreadyFollows
	}

	u.Following[nick] = url
	u.sources[url] = nick

	return nil
}

func (u *User) Follows(url string) bool {
	_, ok := u.sources[NormalizeURL(url)]
	return ok
}

func (u *User) HasMuted(url string) bool {
	_, ok := u.muted[NormalizeURL(url)]
	return ok
}

func (u *User) Source() types.Feeds {
	feeds := make(types.Feeds)
	feeds[types.Feed{Nick: u.Username, URL: u.URL}] = true
	return feeds
}

func (u *User) Sources() types.Feeds {
	// Ensure we fetch the user's own posts in the cache
	feeds := u.Source()
	for url, nick := range u.sources {
		feeds[types.Feed{Nick: nick, URL: url}] = true
	}
	return feeds
}

func (u *User) Profile(baseURL string, viewer *User) types.Profile {
	var (
		follows    bool
		followedBy bool
		muted      bool
	)

	if viewer != nil {
		if viewer.Is(u.URL) {
			follows = true
			followedBy = true
		} else {
			follows = viewer.Follows(u.URL)
			followedBy = viewer.FollowedBy(u.URL)
		}

		muted = viewer.HasMuted(u.URL)
	}

	return types.Profile{
		Type: "User",

		Username: u.Username,
		Tagline:  u.Tagline,
		URL:      u.URL,
		BlogsURL: URLForBlogs(baseURL, u.Username),

		Follows:    follows,
		FollowedBy: followedBy,
		Muted:      muted,

		Followers: u.Followers,
		Following: u.Following,
		Bookmarks: u.Bookmarks,
	}
}

func (u *User) Twter() types.Twter {
	return types.Twter{Nick: u.Username, URL: u.URL}
}

func (u *User) Filter(twts []types.Twt) (filtered []types.Twt) {
	// fast-path
	if len(u.muted) == 0 {
		return twts
	}

	for _, twt := range twts {
		if u.HasMuted(twt.Twter().URL) {
			continue
		}
		filtered = append(filtered, twt)
	}
	return
}

func (u *User) Reply(twt types.Twt) string {
	mentionsSet := make(map[string]bool)
	for _, m := range twt.Mentions() {
		twter := m.Twter()
		if _, ok := mentionsSet[twter.Nick]; !ok && twter.Nick != u.Username {
			mentionsSet[twter.Nick] = true
		}
	}

	mentions := []string{fmt.Sprintf("@%s", twt.Twter().Nick)}
	for nick := range mentionsSet {
		mentions = append(mentions, fmt.Sprintf("@%s", nick))
	}

	mentions = UniqStrings(mentions)

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
