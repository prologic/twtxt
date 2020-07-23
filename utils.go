package twtxt

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/goware/urlx"
	"github.com/microcosm-cc/bluemonday"
	log "github.com/sirupsen/logrus"
)

const (
	meSpecialUser      = "me"
	newsSpecialUser    = "news"
	helpSpecialUser    = "help"
	twtxtSpecialUser   = "twtxt"
	supportSpecialUser = "support"
)

var (
	reservedUsernames = []string{
		meSpecialUser,
		newsSpecialUser,
		helpSpecialUser,
		twtxtSpecialUser,
		supportSpecialUser,
	}

	validUsername  = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]+$`)
	userAgentRegex = regexp.MustCompile(`(.*?)\/(.*?) ?\(\+(https?://.*); @(.*)\)`)

	ErrInvalidUsername  = errors.New("error: invalid username")
	ErrInvalidUserAgent = errors.New("error: invalid twtxt user agent")
	ErrReservedUsername = errors.New("error: username is reserved")
)

type URI struct {
	Type string
	Path string
}

func (u *URI) String() string {
	return fmt.Sprintf("%s://%s", u.Type, u.Path)
}

type TwtxtUserAgent struct {
	ClientName    string
	ClientVersion string
	Nick          string
	URL           string
}

func DetectFollowerFromUserAgent(ua string) (*TwtxtUserAgent, error) {
	match := userAgentRegex.FindStringSubmatch(ua)
	if match == nil {
		return nil, ErrInvalidUserAgent
	}
	return &TwtxtUserAgent{
		ClientName:    match[1],
		ClientVersion: match[2],
		URL:           match[3],
		Nick:          match[4],
	}, nil
}

func ParseURI(uri string) (*URI, error) {
	parts := strings.Split(uri, "://")
	if len(parts) == 2 {
		return &URI{Type: strings.ToLower(parts[0]), Path: parts[1]}, nil
	}
	return nil, fmt.Errorf("invalid uri: %s", uri)
}

func NormalizeUsername(username string) string {
	return strings.TrimSpace(strings.ToLower(username))
}

func NormalizeURL(url string) string {
	if url == "" {
		return ""
	}
	u, err := urlx.Parse(url)
	if err != nil {
		log.WithError(err).Errorf("error parsing url %s", url)
		return ""
	}
	if u.Scheme == "http" && strings.HasSuffix(u.Host, ":80") {
		u.Host = strings.TrimSuffix(u.Host, ":80")
	}
	if u.Scheme == "https" && strings.HasSuffix(u.Host, ":443") {
		u.Host = strings.TrimSuffix(u.Host, ":443")
	}
	u.User = nil
	u.Path = strings.TrimSuffix(u.Path, "/")
	norm, err := urlx.Normalize(u)
	if err != nil {
		log.WithError(err).Errorf("error normalizing url %s", url)
		return ""
	}
	return norm
}

func URLForUser(baseURL, username string) string {
	return fmt.Sprintf(
		"%s/u/%s",
		strings.TrimSuffix(baseURL, "/"),
		username,
	)
}

// SafeParseInt ...
func SafeParseInt(s string, d int) int {
	n, e := strconv.Atoi(s)
	if e != nil {
		return d
	}
	return n
}

// ValidateUsername validates the username before allowing it to be created.
// This ensures usernames match a defined pattern and that some usernames
// that are reserved are never used by users.
func ValidateUsername(username string) error {
	username = NormalizeUsername(username)

	if !validUsername.MatchString(username) {
		return ErrInvalidUsername
	}

	for _, reservedUsername := range reservedUsernames {
		if username == reservedUsername {
			return ErrReservedUsername
		}
	}

	return nil
}

// CleanTweet cleans a tweet's text, replacing new lines with spaces and
// stripping surrounding spaces.
func CleanTweet(text string) string {
	text = strings.ReplaceAll(text, "\r\n", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.TrimSpace(text)

	return text
}

// FormatTweet formats a tweet into a valid HTML snippet
func FormatTweet(text string) template.HTML {
	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	md := []byte(FormatMentions(text))
	maybeUnsafeHTML := markdown.ToHTML(md, nil, renderer)
	p := bluemonday.UGCPolicy()
	p.AllowAttrs("target").OnElements("a")
	html := p.SanitizeBytes(maybeUnsafeHTML)

	return template.HTML(html)
}

// FormatMentions turns `@<nick URL>` into `<a href="URL">@nick</a>`
func FormatMentions(text string) string {
	re := regexp.MustCompile(`@<([^ ]+) *([^>]+)>`)
	return re.ReplaceAllStringFunc(text, func(match string) string {
		parts := re.FindStringSubmatch(match)
		nick, url := parts[1], parts[2]
		return fmt.Sprintf(`<a href="%s">@%s</a>`, url, nick)
	})
}

// FormatRequest generates ascii representation of a request
func FormatRequest(r *http.Request) string {
	return fmt.Sprintf(
		"%s %v %s%v %v (%s)",
		r.RemoteAddr,
		r.Method,
		r.Host,
		r.URL,
		r.Proto,
		r.UserAgent(),
	)
}
