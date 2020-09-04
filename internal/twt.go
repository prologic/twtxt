// -*- tab-width: 4; -*-

package internal

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	read_file_last_line "github.com/prologic/read-file-last-line"
	log "github.com/sirupsen/logrus"

	"github.com/prologic/twtxt/types"
)

const (
	feedsDir = "feeds"
)

var (
	ErrInvalidTwtLine = errors.New("error: invalid twt line parsed")
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
		return types.Twt{}, fmt.Errorf("cowardly refusing to twt empty text, or only spaces")
	}

	p := filepath.Join(conf.Data, feedsDir)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating feeds directory")
		return types.Twt{}, err
	}

	fn := filepath.Join(p, user.Username)

	f, err := os.OpenFile(fn, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return types.Twt{}, err
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
		return types.Twt{}, err
	}

	twt, err := ParseLine(strings.TrimSpace(line), user.Twter())
	if err != nil {
		return types.Twt{}, err
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

func ParseLine(line string, twter types.Twter) (twt types.Twt, err error) {
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

	twt = types.Twt{
		Twter:   twter,
		Created: ParseTime(parts[1]),
		Text:    parts[3],
	}

	return
}

func ParseFile(scanner *bufio.Scanner, twter types.Twter, ttl time.Duration, N int) (types.Twts, types.Twts, error) {
	var (
		twts types.Twts
		old  types.Twts
	)

	oldTime := time.Now().Add(-ttl)

	for scanner.Scan() {
		line := scanner.Text()
		twt, err := ParseLine(line, twter)
		if err != nil {
			log.Warnf("could not parse: '%s' (source:%s)\n", line, twter.URL)
			continue
		}
		if twt.IsZero() {
			continue
		}

		if ttl > 0 && twt.Created.Before(oldTime) {
			old = append(old, twt)
		} else {
			twts = append(twts, twt)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	// Sort by CreatedAt timestamp
	sort.Sort(twts)
	sort.Sort(old)

	// Further limit by Max Cache Items
	if N > 0 && len(twts) > N {
		if N > len(twts) {
			N = len(twts)
		}
		twts = twts[:N]
		old = append(old, twts[N:]...)
	}

	return twts, old, nil
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
