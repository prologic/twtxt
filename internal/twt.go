// -*- tab-width: 4; -*-

package twtxt

import (
	"bufio"
	"encoding/base32"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	read_file_last_line "github.com/prologic/read-file-last-line"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/blake2b"
)

const (
	feedsDir = "feeds"
)

var (
	ErrInvalidTwtLine = errors.New("error: invalid twt line parsed")
)

type Twter struct {
	Nick string
	URL  string
}

type Twt struct {
	Twter   Twter
	Text    string
	Created time.Time

	hash string
}

func (twt Twt) Mentions() []string {
	var mentions []string

	re := regexp.MustCompile(`@<(.*?) .*?>`)
	matches := re.FindAllStringSubmatch(twt.Text, -1)
	for _, match := range matches {
		mentions = append(mentions, match[1])
	}

	return mentions
}

func (twt Twt) Tags() []string {
	var mentions []string

	re := regexp.MustCompile(`#<(.*?) .*?>`)
	matches := re.FindAllStringSubmatch(twt.Text, -1)
	for _, match := range matches {
		mentions = append(mentions, match[1])
	}

	return mentions
}

func (twt Twt) Subject() string {
	re := regexp.MustCompile(`^(@<.*>[, ]*)*(\(.*?\))(.*)`)
	match := re.FindStringSubmatch(twt.Text)
	if match != nil {
		return match[2]
	}
	return ""
}

func (twt Twt) Hash() string {
	if twt.hash != "" {
		return twt.hash
	}

	payload := twt.Created.String() + "\n" + twt.Text
	sum := blake2b.Sum256([]byte(payload))

	// Base32 is URL-safe, unlike Base64, and shorter than hex.
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	twt.hash = strings.ToLower(encoding.EncodeToString(sum[:]))

	return twt.hash
}

// typedef to be able to attach sort methods
type Twts []Twt

func (twts Twts) Len() int {
	return len(twts)
}
func (twts Twts) Less(i, j int) bool {
	return twts[i].Created.Before(twts[j].Created)
}
func (twts Twts) Swap(i, j int) {
	twts[i], twts[j] = twts[j], twts[i]
}

func (twts Twts) Tags() map[string]int {
	tags := make(map[string]int)
	re := regexp.MustCompile(`#[-\w]+`)
	for _, twt := range twts {
		for _, tag := range re.FindAllString(twt.Text, -1) {
			tags[strings.TrimLeft(tag, "#")]++
		}
	}
	return tags
}

// Turns "@nick" into "@<nick URL>" if we're following nick.
func ExpandMentions(conf *Config, db Store, user *User, text string) string {
	re := regexp.MustCompile(`@([_a-zA-Z0-9]+)`)
	return re.ReplaceAllStringFunc(text, func(match string) string {
		parts := re.FindStringSubmatch(match)
		mentionedNick := parts[1]

		for followedNick, followedURL := range user.Following {
			if mentionedNick == followedNick {
				return fmt.Sprintf("@<%s %s>", followedNick, followedURL)
			}
		}

		username := NormalizeUsername(mentionedNick)
		if db.HasUser(username) || db.HasFeed(username) {
			return fmt.Sprintf("@<%s %s>", username, URLForUser(conf.BaseURL, username))
		} else {
			// Not expanding if we're not following
			return match
		}
	})
}

// Turns #tag into "@<tag URL>"
func ExpandTag(conf *Config, db Store, user *User, text string) string {
	re := regexp.MustCompile(`#([-\w]+)`)
	return re.ReplaceAllStringFunc(text, func(match string) string {
		parts := re.FindStringSubmatch(match)
		mentionedTag := parts[1]

		return fmt.Sprintf("#<%s %s>", mentionedTag, URLForTag(conf.BaseURL, mentionedTag))
	})
}

func DeleteLastTwt(conf *Config, user *User) error {
	p := filepath.Join(conf.Data, feedsDir)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating feeds directory")
		return err
	}

	fn := filepath.Join(p, user.Username)

	_, n, err := GetLastTwt(conf, user)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(fn, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	return f.Truncate(int64(n))
}

func AppendSpecial(conf *Config, db Store, specialUsername, text string) error {
	user := &User{Username: specialUsername}
	user.Following = make(map[string]string)
	return AppendTwt(conf, db, user, text)
}

func AppendTwt(conf *Config, db Store, user *User, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("cowardly refusing to twt empty text, or only spaces")
	}

	p := filepath.Join(conf.Data, feedsDir)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating feeds directory")
		return err
	}

	fn := filepath.Join(p, user.Username)

	f, err := os.OpenFile(fn, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err = f.WriteString(
		fmt.Sprintf("%s\t%s\n", time.Now().Format(time.RFC3339),
			ExpandTag(conf, db, user, ExpandMentions(conf, db, user, text))),
	); err != nil {
		return err
	}

	return nil
}

func FeedExists(conf *Config, username string) bool {
	fn := filepath.Join(conf.Data, feedsDir, NormalizeUsername(username))
	if _, err := os.Stat(fn); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}

	return true
}

func GetLastTwt(conf *Config, user *User) (twt Twt, offset int, err error) {
	p := filepath.Join(conf.Data, feedsDir)
	if err = os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating feeds directory")
		return
	}

	fn := filepath.Join(p, user.Username)

	var data []byte
	data, offset, err = read_file_last_line.ReadLastLine(fn)
	if err != nil {
		return
	}

	twt, err = ParseLine(string(data), user.Twter())
	return
}

func GetUserTwts(conf *Config, username string) (Twts, error) {
	p := filepath.Join(conf.Data, feedsDir)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating feeds directory")
		return nil, err
	}

	username = NormalizeUsername(username)

	var twts Twts

	twter := Twter{
		Nick: username,
		URL:  URLForUser(conf.BaseURL, username),
	}
	fn := filepath.Join(p, username)
	f, err := os.Open(fn)
	if err != nil {
		log.WithError(err).Warnf("error opening feed: %s", fn)
		return nil, err
	}
	s := bufio.NewScanner(f)
	t, err := ParseFile(s, twter)
	if err != nil {
		log.WithError(err).Errorf("error processing feed %s", fn)
		return nil, err
	}
	twts = append(twts, t...)
	f.Close()

	return twts, nil
}

func GetAllTwts(conf *Config) (Twts, error) {
	p := filepath.Join(conf.Data, feedsDir)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating feeds directory")
		return nil, err
	}

	files, err := ioutil.ReadDir(p)
	if err != nil {
		log.WithError(err).Error("error listing feeds")
		return nil, err
	}

	var twts Twts

	for _, info := range files {
		twter := Twter{
			Nick: info.Name(),
			URL:  URLForUser(conf.BaseURL, info.Name()),
		}
		fn := filepath.Join(p, info.Name())
		f, err := os.Open(fn)
		if err != nil {
			log.WithError(err).Warnf("error opening feed: %s", fn)
			continue
		}
		s := bufio.NewScanner(f)
		t, err := ParseFile(s, twter)
		if err != nil {
			log.WithError(err).Errorf("error processing feed %s", fn)
			continue
		}
		twts = append(twts, t...)
		f.Close()
	}

	return twts, nil
}

func ParseLine(line string, twter Twter) (twt Twt, err error) {
	if line == "" {
		return
	}
	if strings.HasPrefix(line, "#") {
		return
	}

	re := regexp.MustCompile(`^(.+?)(\s+)(.+)$`) // .+? is ungreedy
	parts := re.FindStringSubmatch(line)
	// "Submatch 0 is the match of the entire expression, submatch 1 the
	// match of the first parenthesized subexpression, and so on."
	if len(parts) != 4 {
		err = ErrInvalidTwtLine
		return
	}

	twt = Twt{
		Twter:   twter,
		Created: ParseTime(parts[1]),
		Text:    parts[3],
	}

	return
}

func ParseFile(scanner *bufio.Scanner, twter Twter) (Twts, error) {
	var twts Twts

	for scanner.Scan() {
		line := scanner.Text()
		twt, err := ParseLine(line, twter)
		if err != nil {
			log.Warnf("could not parse: '%s' (source:%s)\n", line, twter.URL)
			continue
		}

		twts = append(twts, twt)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return twts, nil
}

func ParseTime(timestr string) time.Time {
	var tm time.Time
	var err error
	// Twtxt clients generally uses basically time.RFC3339Nano, but sometimes
	// there's a colon in the timezone, or no timezone at all.
	for _, layout := range []string{
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02T15:04:05.999999999Z0700",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04.999999999Z07:00",
		"2006-01-02T15:04.999999999Z0700",
		"2006-01-02T15:04.999999999",
	} {
		tm, err = time.Parse(layout, strings.ToUpper(timestr))
		if err != nil {
			continue
		} else {
			break
		}
	}
	if err != nil {
		return time.Unix(0, 0)
	}
	return tm
}
