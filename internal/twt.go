// -*- tab-width: 4; -*-

package internal

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	read_file_last_line "github.com/prologic/read-file-last-line"
	log "github.com/sirupsen/logrus"

	"github.com/jointwt/twtxt/types"
)

const (
	feedsDir = "feeds"
)

// ExpandMentions turns "@nick" into "@<nick URL>" if we're following the user or feed
// or if they exist on the local pod. Also turns @user@domain into
// @<user URL> as a convenient way to mention users across pods.
func ExpandMentions(conf *Config, db Store, user *User, text string) string {
	re := regexp.MustCompile(`@([a-zA-Z0-9][a-zA-Z0-9_-]+)(?:@)?((?:[_a-z0-9](?:[_a-z0-9-]{0,61}[a-z0-9]\.)|(?:[0-9]+/[0-9]{2})\.)+(?:[a-z](?:[a-z0-9-]{0,61}[a-z0-9])?)?)?`)
	return re.ReplaceAllStringFunc(text, func(match string) string {
		parts := re.FindStringSubmatch(match)
		mentionedNick := parts[1]
		mentionedDomain := parts[2]

		if mentionedNick != "" && mentionedDomain != "" {
			// TODO: Validate the remote end for a valid Twtxt pod?
			// XXX: Should we always assume https:// ?
			return fmt.Sprintf(
				"@<%s https://%s/user/%s/twtxt.txt>",
				mentionedNick, mentionedDomain, mentionedNick,
			)
		}

		for followedNick, followedURL := range user.Following {
			if mentionedNick == followedNick {
				return fmt.Sprintf("@<%s %s>", followedNick, followedURL)
			}
		}

		username := NormalizeUsername(mentionedNick)
		if db.HasUser(username) || db.HasFeed(username) {
			return fmt.Sprintf("@<%s %s>", username, URLForUser(conf, username))
		}

		// Not expanding if we're not following, not a local user/feed
		return match
	})
}

// Turns #tag into "@<tag URL>"
func ExpandTag(conf *Config, db Store, user *User, text string) string {
	re := regexp.MustCompile(`#([-\w]+)`)
	return re.ReplaceAllStringFunc(text, func(match string) string {
		parts := re.FindStringSubmatch(match)
		tag := parts[1]

		return fmt.Sprintf("#<%s %s>", tag, URLForTag(conf.BaseURL, tag))
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

func AppendSpecial(conf *Config, db Store, specialUsername, text string, args ...interface{}) (types.Twt, error) {
	user := &User{Username: specialUsername}
	user.Following = make(map[string]string)
	return AppendTwt(conf, db, user, text, args)
}

func AppendTwt(conf *Config, db Store, user *User, text string, args ...interface{}) (types.Twt, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return types.NilTwt, fmt.Errorf("cowardly refusing to twt empty text, or only spaces")
	}

	p := filepath.Join(conf.Data, feedsDir)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating feeds directory")
		return types.NilTwt, err
	}

	fn := filepath.Join(p, user.Username)

	f, err := os.OpenFile(fn, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return types.NilTwt, err
	}
	defer f.Close()

	// Support replacing/editing an existing Twt whilst preserving Created Timestamp
	now := time.Now()
	if len(args) == 1 {
		if t, ok := args[0].(time.Time); ok {
			now = t
		}
	}

	line := fmt.Sprintf(
		"%s\t%s\n",
		now.Format(time.RFC3339),
		ExpandTag(conf, db, user, ExpandMentions(conf, db, user, text)),
	)

	if _, err = f.WriteString(line); err != nil {
		return types.NilTwt, err
	}

	twt, err := types.ParseLine(strings.TrimSpace(line), user.Twter())
	if err != nil {
		return types.NilTwt, err
	}

	return twt, nil
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

func GetLastTwt(conf *Config, user *User) (twt types.Twt, offset int, err error) {
	twt = types.NilTwt

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

	twt, err = types.ParseLine(string(data), user.Twter())

	return
}

func GetAllFeeds(conf *Config) ([]string, error) {
	p := filepath.Join(conf.Data, feedsDir)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating feeds directory")
		return nil, err
	}

	files, err := ioutil.ReadDir(p)
	if err != nil {
		log.WithError(err).Error("error reading feeds directory")
		return nil, err
	}

	fns := []string{}
	for _, fileInfo := range files {
		fns = append(fns, filepath.Base(fileInfo.Name()))
	}
	return fns, nil
}

func GetFeedCount(conf *Config, name string) (int, error) {
	p := filepath.Join(conf.Data, feedsDir)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating feeds directory")
		return 0, err
	}

	fn := filepath.Join(p, name)

	f, err := os.Open(fn)
	if err != nil {
		log.WithError(err).Error("error opening feed file")
		return 0, err
	}
	defer f.Close()

	return LineCount(f)
}

func GetAllTwts(conf *Config, name string) (types.Twts, error) {
	p := filepath.Join(conf.Data, feedsDir)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating feeds directory")
		return nil, err
	}

	var twts types.Twts

	twter := types.Twter{
		Nick: name,
		URL:  URLForUser(conf, name),
	}
	fn := filepath.Join(p, name)
	f, err := os.Open(fn)
	if err != nil {
		log.WithError(err).Warnf("error opening feed: %s", fn)
		return nil, err
	}
	t, err := types.ParseFile(f, twter)
	if err != nil {
		log.WithError(err).Errorf("error processing feed %s", fn)
		return nil, err
	}
	twts = append(twts, t.Twts()...)
	f.Close()

	return twts, nil
}
