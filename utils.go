package twtxt

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"html/template"
	"image"

	// Blank import so we can handle image/jpeg
	_ "image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/gomail.v2"

	"github.com/PuerkitoBio/goquery"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/html"
	"github.com/goware/urlx"
	"github.com/h2non/filetype"
	"github.com/microcosm-cc/bluemonday"
	"github.com/nfnt/resize"
	log "github.com/sirupsen/logrus"
)

const (
	avatarsDir = "avatars"
	mediaDir = "media"

	newsSpecialUser    = "news"
	helpSpecialUser    = "help"
	supportSpecialUser = "support"

	me       = "me"
	twtxtBot = "twtxt"
	statsBot = "stats"

	maxUsernameLength = 15 // avg 6 chars / 2 syllables per name commonly
	maxFeedNameLength = 25 // avg 4.7 chars per word in English so ~5 words
)

var (
	specialUsernames = []string{
		newsSpecialUser,
		helpSpecialUser,
		supportSpecialUser,
	}
	reservedUsernames = []string{
		me,
		statsBot,
		twtxtBot,
	}
	twtxtBots = []string{
		statsBot,
		twtxtBot,
	}

	validFeedName  = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_ ]*$`)
	validUsername  = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]+$`)
	userAgentRegex = regexp.MustCompile(`(.*?)\/(.*?) ?\(\+(https?://.*); @(.*)\)`)

	ErrInvalidFeedName    = errors.New("error: invalid feed name")
	ErrFeedNameTooLong    = errors.New("error: feed name is too long")
	ErrInvalidUsername    = errors.New("error: invalid username")
	ErrUsernameTooLong    = errors.New("error: username is too long")
	ErrInvalidUserAgent   = errors.New("error: invalid twtxt user agent")
	ErrReservedUsername   = errors.New("error: username is reserved")
	ErrSendingEmail       = errors.New("error: unable to send email")
	ErrInvalidImageUPload = errors.New("error: invalid or corrupted image uploaded")
)

func UniqStrings(xs []string) []string {
	set := make(map[string]bool)
	for _, x := range xs {
		if _, ok := set[x]; !ok {
			set[x] = true
		}
	}

	res := []string{}
	for k := range set {
		res = append(res, k)
	}
	return res
}

func RemoveString(xs []string, e string) []string {
	res := []string{}
	for _, x := range xs {
		if x == e {
			continue
		}
		res = append(res, x)
	}
	return res
}

func SHA256Sum(fn string) ([]byte, error) {
	f, err := os.Open(fn)
	if err != nil {
		log.WithError(err).Warnf("error opening file %s", fn)
		return nil, err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		log.WithError(err).Errorf("error reading file %s", fn)
		return nil, err
	}

	return h.Sum(nil), nil
}

func IsImage(fn string) bool {
	f, err := os.Open(fn)
	if err != nil {
		log.WithError(err).Warnf("error opening file %s", fn)
		return false
	}
	defer f.Close()

	head := make([]byte, 261)
	if _, err := f.Read(head); err != nil {
		log.WithError(err).Warnf("error reading from file %s", fn)
		return false
	}

	if filetype.IsImage(head) {
		return true
	}

	return false
}

type UploadOptions struct {
	Resize  bool
	ResizeW int
	ResizeH int
}

func StoreUploadedImage(conf *Config, f io.Reader, resource, name string, opts *UploadOptions) (string, error) {
	tf, err := ioutil.TempFile("", "twtxt-upload-*")
	if err != nil {
		log.WithError(err).Error("error creating temporary file")
		return "", err
	}
	defer tf.Close()

	if _, err := io.Copy(tf, f); err != nil {
		log.WithError(err).Error("error writng temporary file")
		return "", err
	}

	if _, err := tf.Seek(0, io.SeekStart); err != nil {
		log.WithError(err).Error("error seeking temporary file")
		return "", err
	}

	if !IsImage(tf.Name()) {
		return "", ErrInvalidImageUPload
	}

	if _, err := tf.Seek(0, io.SeekStart); err != nil {
		log.WithError(err).Error("error seeking temporary file")
		return "", err
	}

	shasum, err := SHA256Sum(tf.Name())
	if err != nil {
		log.WithError(err).Error("error computing SHA256SUM of temporary file")
		return "", err
	}

	hash := string(shasum)

	p := filepath.Join(conf.Data, resource)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating avatars directory")
		return "", err
	}

	var fn string

	if name == "" {
		fn = filepath.Join(p, fmt.Sprintf("%s.png", hash))
	} else {
		fn = fmt.Sprintf("%s.png", filepath.Join(p, name))
	}

	of, err := os.OpenFile(fn, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.WithError(err).Error("error opening output file")
		return "", err
	}
	defer of.Close()

	if _, err := tf.Seek(0, io.SeekStart); err != nil {
		log.WithError(err).Error("error seeking temporary file")
		return "", err
	}

	img, _, err := image.Decode(tf)
	if err != nil {
		log.WithError(err).Error("error decoding image")
		return "", err
	}

	newImg := img

	if opts != nil {
		if opts.Resize && (opts.ResizeW+opts.ResizeH) > 0 {
			newImg = resize.Resize(uint(opts.ResizeW), uint(opts.ResizeH), img, resize.Lanczos3)
		}
	}

	// Encode uses a Writer, use a Buffer if you need the raw []byte
	if err := png.Encode(of, newImg); err != nil {
		log.WithError(err).Error("error reencoding image")
		return "", err
	}

	return fmt.Sprintf(
		"%s/%s/%s",
		strings.TrimSuffix(conf.BaseURL, "/"),
		resource, filepath.Base(fn),
	), nil
}

func NormalizeFeedName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ToLower(name)
	return name
}

func ValidateFeedName(path string, name string) error {
	if !validFeedName.MatchString(name) {
		return ErrInvalidFeedName
	}
	if len(name) > maxFeedNameLength {
		return ErrFeedNameTooLong
	}

	return nil
}

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

func RedirectURL(r *http.Request, conf *Config, defaultURL string) string {
	referer := NormalizeURL(r.Header.Get("Referer"))
	if referer != "" && strings.HasPrefix(referer, conf.BaseURL) {
		return referer
	}

	return defaultURL
}

func PrettyURL(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		log.WithError(err).Warnf("PrettyURL(): error parsing url: %s", uri)
		return uri
	}

	return fmt.Sprintf("%s/%s", u.Hostname(), strings.TrimPrefix(u.EscapedPath(), "/"))
}

func UserURL(url string) string {
	if strings.HasSuffix(url, "/twtxt.txt") {
		return strings.TrimSuffix(url, "/twtxt.txt")
	}
	return url
}

func URLForUser(baseURL, username string) string {
	return fmt.Sprintf(
		"%s/user/%s/twtxt.txt",
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

	if len(username) > maxUsernameLength {
		return ErrUsernameTooLong
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
func FormatTweetFactory(conf *Config) func(text string) template.HTML {
	isLocal := func(url string) bool {
		if NormalizeURL(url) == "" {
			return false
		}
		return strings.HasPrefix(NormalizeURL(url), NormalizeURL(conf.BaseURL))
	}

	return func(text string) template.HTML {
		renderHookProcessURLs := func(w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, bool) {
			span, ok := node.(*ast.HTMLSpan)
			if !ok {
				return ast.GoToNext, false
			}

			leaf := span.Leaf
			doc, err := goquery.NewDocumentFromReader(bytes.NewReader(leaf.Literal))
			if err != nil {
				log.WithError(err).Warn("error parsing HTMLSpan")
				return ast.GoToNext, false
			}

			a := doc.Find("a")
			href, ok := a.Attr("href")
			if !ok {
				return ast.GoToNext, false
			}

			if isLocal(href) {
				href = UserURL(href)
			} else {
				return ast.GoToNext, false
			}

			html := fmt.Sprintf(`<a href="%s">`, href)

			io.WriteString(w, html)

			return ast.GoToNext, true
		}

		htmlFlags := html.CommonFlags | html.HrefTargetBlank
		opts := html.RendererOptions{
			Flags:          htmlFlags,
			RenderNodeHook: renderHookProcessURLs,
		}
		renderer := html.NewRenderer(opts)

		md := []byte(FormatMentions(text))
		maybeUnsafeHTML := markdown.ToHTML(md, nil, renderer)
		p := bluemonday.UGCPolicy()
		p.AllowAttrs("target").OnElements("a")
		html := p.SanitizeBytes(maybeUnsafeHTML)

		return template.HTML(html)
	}
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

func (c *Config) SendEmail(recipients []string, subject string, body string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", c.SMTPFrom)
	m.SetHeader("To", recipients...)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	d := gomail.NewDialer(c.SMTPHost, c.SMTPPort, c.SMTPUser, c.SMTPPass)

	err := d.DialAndSend(m)
	if err != nil {
		log.WithError(err).Error("SendEmail() failed")
		return ErrSendingEmail
	}

	return nil
}
