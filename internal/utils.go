package internal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"image"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	// Blank import so we can handle image/jpeg
	_ "image/gif"
	_ "image/jpeg"
	"image/png"

	"github.com/PuerkitoBio/goquery"
	"github.com/chai2010/webp"
	"github.com/disintegration/imageorient"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/goware/urlx"
	"github.com/h2non/filetype"
	shortuuid "github.com/lithammer/shortuuid/v3"
	"github.com/microcosm-cc/bluemonday"
	"github.com/nfnt/resize"
	"github.com/nullrocks/identicon"
	"github.com/prologic/twtxt"
	"github.com/prologic/twtxt/types"
	log "github.com/sirupsen/logrus"
	"github.com/writeas/slug"
)

const (
	avatarsDir  = "avatars"
	externalDir = "external"
	mediaDir    = "media"

	newsSpecialUser    = "news"
	helpSpecialUser    = "help"
	supportSpecialUser = "support"

	me       = "me"
	twtxtBot = "twtxt"
	statsBot = "stats"

	maxUsernameLength = 15 // avg 6 chars / 2 syllables per name commonly
	maxFeedNameLength = 25 // avg 4.7 chars per word in English so ~5 words

	requestTimeout = time.Second * 30

	DayAgo   = time.Hour * 24
	WeekAgo  = DayAgo * 7
	MonthAgo = DayAgo * 30
	YearAgo  = MonthAgo * 12
)

var (
	slugs *sync.Map

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

	validFeedName  = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)
	validUsername  = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]+$`)
	userAgentRegex = regexp.MustCompile(`(.*?)\/(.*?) ?\(\+(https?://.*); @(.*)\)`)

	ErrInvalidFeedName  = errors.New("error: invalid feed name")
	ErrFeedNameTooLong  = errors.New("error: feed name is too long")
	ErrInvalidUsername  = errors.New("error: invalid username")
	ErrUsernameTooLong  = errors.New("error: username is too long")
	ErrInvalidUserAgent = errors.New("error: invalid twtxt user agent")
	ErrReservedUsername = errors.New("error: username is reserved")
	ErrInvalidImage     = errors.New("error: invalid image")
	ErrInvalidVideo     = errors.New("error: invalid video")
	ErrInvalidVideoSize = errors.New("error: invalid video size")
)

func init() {
	slugs = &sync.Map{}
}

func Slugify(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		log.WithError(err).Warnf("Slugify(): error parsing uri: %s", uri)
		return ""
	}

	s := slug.Make(fmt.Sprintf("%s/%s", u.Hostname(), u.Path))
	slugs.Store(s, u)

	return s
}

func GenerateAvatar(conf *Config, username string) (image.Image, error) {
	ig, err := identicon.New(conf.Name, 5, 3)
	if err != nil {
		log.WithError(err).Error("error creating identicon generator")
		return nil, err
	}

	ii, err := ig.Draw(username)
	if err != nil {
		log.WithError(err).Errorf("error generating avatar for %s", username)
		return nil, err
	}

	return ii.Image(AvatarResolution), nil
}

func ReplaceExt(fn, newExt string) string {
	oldExt := filepath.Ext(fn)
	return fmt.Sprintf("%s%s", strings.TrimSuffix(fn, oldExt), newExt)
}

func ImageToPng(fn string) error {
	if !IsImage(fn) {
		return ErrInvalidImage
	}

	f, err := os.Open(fn)
	if err != nil {
		log.WithError(err).Errorf("error opening image  %s", fn)
		return err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		log.WithError(err).Error("image.Decode failed")
		return err
	}

	of, err := os.OpenFile(ReplaceExt(fn, ".png"), os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.WithError(err).Error("error opening output file")
		return err
	}
	defer of.Close()

	if err := png.Encode(of, img); err != nil {
		log.WithError(err).Error("error reencoding image")
		return err
	}

	return nil
}

func VideoToMp4(conf *Config, fn string) error {
	if !IsVideo(fn) {
		return ErrInvalidVideo
	}

	of := ReplaceExt(fn, ".mp4")

	if err := RunCmd(
		conf.TranscoderTimeout,
		"ffmpeg",
		"-y",
		"-i", fn,
		"-vcodec", "h264",
		"-acodec", "aac",
		"-strict", "-2",
		"-loglevel", "quiet",
		of,
	); err != nil {
		log.WithError(err).Error("error transcoding video")
		return err
	}

	return nil
}

func GetExternalAvatar(conf *Config, nick, uri string) string {
	name := Slugify(uri)

	fn := filepath.Join(conf.Data, externalDir, fmt.Sprintf("%s.webp", name))
	if FileExists(fn) {
		return URLForExternalAvatar(conf, nick, uri)
	}

	if !strings.HasSuffix(uri, "/") {
		uri += "/"
	}

	base, err := url.Parse(uri)
	if err != nil {
		log.WithError(err).Errorf("error parsing uri: %s", uri)
		return ""
	}

	candidates := []string{
		"../avatar.webp", "./avatar.webp",
		"../avatar.png", "./avatar.png",
		"../logo.png", "./logo.png",
		"../avatar.jpg", "./avatar.jpg",
		"../logo.jpg", "./logo.jpg",
		"../avatar.jpeg", "./avatar.jpeg",
		"../logo.jpeg", "./logo.jpeg",
	}

	for _, candidate := range candidates {
		source, _ := base.Parse(candidate)
		if ResourceExists(conf, source.String()) {
			opts := &ImageOptions{Resize: true, ResizeW: AvatarResolution, ResizeH: AvatarResolution}
			_, err := DownloadImage(conf, source.String(), externalDir, name, opts)
			if err != nil {
				log.WithError(err).
					WithField("base", base.String()).
					WithField("source", source.String()).
					Error("error downloading external avatar")
				return ""
			}
			return URLForExternalAvatar(conf, nick, uri)
		}
	}

	log.Warnf("unable to find a suitable avatar for %s", uri)

	return ""
}

func Request(conf *Config, method, url string, headers http.Header) (*http.Response, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		log.WithError(err).Errorf("%s: http.NewRequest fail: %s", url, err)
		return nil, err
	}

	if headers == nil {
		headers = make(http.Header)
	}

	headers.Set(
		"User-Agent",
		fmt.Sprintf(
			"twtxt/%s (Pod: %s Support: %s)",
			twtxt.FullVersion(), conf.Name, URLForPage(conf.BaseURL, "support"),
		),
	)
	req.Header = headers

	client := http.Client{
		Timeout: requestTimeout,
	}

	res, err := client.Do(req)
	if err != nil {
		log.WithError(err).Errorf("%s: client.Do fail: %s", url, err)
		return nil, err
	}

	return res, nil
}

func ResourceExists(conf *Config, url string) bool {
	res, err := Request(conf, http.MethodHead, url, nil)
	if err != nil {
		log.WithError(err).Errorf("error checking if %s exists", url)
		return false
	}
	defer res.Body.Close()

	return res.StatusCode/100 == 2
}

func FileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// CmdExists ...
func CmdExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// RunCmd ...
func RunCmd(timeout time.Duration, command string, args ...string) error {
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)

	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()

	cmd := exec.CommandContext(ctx, command, args...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		log.WithError(err).WithField("out", string(out)).Error("error running command")
		return err
	}

	return nil
}

// RenderString ...
func RenderString(tpl string, ctx *Context) (string, error) {
	t := template.Must(template.New("tpl").Parse(tpl))
	buf := bytes.NewBuffer([]byte{})
	err := t.Execute(buf, ctx)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func IsExternalFeedFactory(conf *Config) func(url string) bool {
	baseURL := NormalizeURL(conf.BaseURL)
	externalBaseURL := fmt.Sprintf("%s/external", strings.TrimSuffix(baseURL, "/"))

	return func(url string) bool {
		if NormalizeURL(url) == "" {
			return false
		}
		return strings.HasPrefix(NormalizeURL(url), externalBaseURL)
	}
}

func IsLocalURLFactory(conf *Config) func(url string) bool {
	return func(url string) bool {
		if NormalizeURL(url) == "" {
			return false
		}
		return strings.HasPrefix(NormalizeURL(url), NormalizeURL(conf.BaseURL))
	}
}

func GetUserFromURL(conf *Config, db Store, url string) (*User, error) {
	if !strings.HasPrefix(url, conf.BaseURL) {
		return nil, fmt.Errorf("error: %s does not match our base url of %s", url, conf.BaseURL)
	}

	userURL := UserURL(url)
	username := filepath.Base(userURL)

	return db.GetUser(username)
}

func WebMention(target, source string) error {
	targetURL, err := url.Parse(target)
	if err != nil {
		log.WithError(err).Error("error parsing target url")
		return err
	}
	sourceURL, err := url.Parse(source)
	if err != nil {
		log.WithError(err).Error("error parsing source url")
		return err
	}
	webmentions.SendNotification(targetURL, sourceURL)
	return nil
}

func StringKeys(kv map[string]string) []string {
	var res []string
	for k := range kv {
		res = append(res, k)
	}
	return res
}

func StringValues(kv map[string]string) []string {
	var res []string
	for _, v := range kv {
		res = append(res, v)
	}
	return res
}

func MapStrings(xs []string, f func(s string) string) []string {
	var res []string
	for _, x := range xs {
		res = append(res, f(x))
	}
	return res
}

func HasString(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}

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

func IsVideo(fn string) bool {
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

	if filetype.IsVideo(head) {
		return true
	}

	return false
}

type ImageOptions struct {
	Resize  bool
	ResizeW int
	ResizeH int
}

type VideoOptions struct {
	Resize bool
	Size   int
}

func DownloadImage(conf *Config, url string, resource, name string, opts *ImageOptions) (string, error) {
	res, err := http.Get(url)
	if err != nil {
		log.WithError(err).Errorf("error downloading image from %s", url)
		return "", err
	}
	defer res.Body.Close()

	tf, err := ioutil.TempFile("", "rss2twtxt-*")
	if err != nil {
		log.WithError(err).Error("error creating temporary file")
		return "", err
	}
	defer tf.Close()

	if _, err := io.Copy(tf, res.Body); err != nil {
		log.WithError(err).Error("error writng temporary file")
		return "", err
	}

	if _, err := tf.Seek(0, io.SeekStart); err != nil {
		log.WithError(err).Error("error seeking temporary file")
		return "", err
	}

	if !IsImage(tf.Name()) {
		return "", ErrInvalidImage
	}

	if _, err := tf.Seek(0, io.SeekStart); err != nil {
		log.WithError(err).Error("error seeking temporary file")
		return "", err
	}

	if _, err := tf.Seek(0, io.SeekStart); err != nil {
		log.WithError(err).Error("error seeking temporary file")
		return "", err
	}

	img, _, err := image.Decode(tf)
	if err != nil {
		log.WithError(err).Error("jpeg.Decode failed")
		return "", err
	}

	newImg := img

	if opts != nil {
		if opts.Resize && (opts.ResizeW+opts.ResizeH > 0) && (opts.ResizeH > 0 || img.Bounds().Size().X > opts.ResizeW) {
			newImg = resize.Resize(uint(opts.ResizeW), uint(opts.ResizeH), img, resize.Lanczos3)
		}
	}

	p := filepath.Join(conf.Data, resource)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating avatars directory")
		return "", err
	}

	var fn string

	if name == "" {
		uuid := shortuuid.New()
		fn = filepath.Join(p, fmt.Sprintf("%s.webp", uuid))
	} else {
		fn = fmt.Sprintf("%s.webp", filepath.Join(p, name))
	}

	of, err := os.OpenFile(fn, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.WithError(err).Error("error opening output file")
		return "", err
	}
	defer of.Close()

	if err := webp.Encode(of, newImg, &webp.Options{Lossless: true}); err != nil {
		log.WithError(err).Error("error reencoding image")
		return "", err
	}

	// Re-encode to PNG (for older browsers)
	if err := of.Close(); err != nil {
		log.WithError(err).Warnf("error cloding file %s", fn)
	}
	if err := ImageToPng(fn); err != nil {
		log.WithError(err).Warnf("error reencoding image to PNG (for older browsers: %s", fn)
	}

	return fmt.Sprintf(
		"%s/%s/%s",
		strings.TrimSuffix(conf.BaseURL, "/"),
		resource, strings.TrimSuffix(filepath.Base(fn), filepath.Ext(fn)),
	), nil
}

func StoreUploadedImage(conf *Config, f io.Reader, resource, name string, opts *ImageOptions) (string, error) {
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
		return "", ErrInvalidImage
	}

	if _, err := tf.Seek(0, io.SeekStart); err != nil {
		log.WithError(err).Error("error seeking temporary file")
		return "", err
	}

	p := filepath.Join(conf.Data, resource)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating avatars directory")
		return "", err
	}

	var fn string

	if name == "" {
		uuid := shortuuid.New()
		fn = filepath.Join(p, fmt.Sprintf("%s.webp", uuid))
	} else {
		fn = fmt.Sprintf("%s.webp", filepath.Join(p, name))
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

	img, _, err := imageorient.Decode(tf)
	if err != nil {
		log.WithError(err).Error("imageorient.Decode failed")
		return "", err
	}

	newImg := img

	if opts != nil {
		if opts.Resize && (opts.ResizeW+opts.ResizeH > 0) && (opts.ResizeH > 0 || img.Bounds().Size().X > opts.ResizeW) {
			newImg = resize.Resize(uint(opts.ResizeW), uint(opts.ResizeH), img, resize.Lanczos3)
		}
	}

	if err := webp.Encode(of, newImg, &webp.Options{Lossless: true}); err != nil {
		log.WithError(err).Error("error reencoding image")
		return "", err
	}

	// Re-encode to PNG (for older browsers)
	if err := of.Close(); err != nil {
		log.WithError(err).Warnf("error cloding file %s", fn)
	}
	if err := ImageToPng(fn); err != nil {
		log.WithError(err).Warnf("error reencoding image to PNG (for older browsers: %s", fn)
	}

	return fmt.Sprintf(
		"%s/%s/%s",
		strings.TrimSuffix(conf.BaseURL, "/"),
		resource, filepath.Base(fn),
	), nil
}

func StoreUploadedVideo(conf *Config, f io.Reader, resource, name string, opts *VideoOptions) (string, error) {
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

	if !IsVideo(tf.Name()) {
		return "", ErrInvalidVideo
	}

	if _, err := tf.Seek(0, io.SeekStart); err != nil {
		log.WithError(err).Error("error seeking temporary file")
		return "", err
	}

	p := filepath.Join(conf.Data, resource)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating avatars directory")
		return "", err
	}

	var fn string

	if name == "" {
		uuid := shortuuid.New()
		fn = filepath.Join(p, fmt.Sprintf("%s.webm", uuid))
	} else {
		fn = fmt.Sprintf("%s.webm", filepath.Join(p, name))
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

	args := []string{
		"-y",
		"-i", tf.Name(),
	}

	if opts.Resize {
		var (
			//size  string
			scale string
		)

		switch opts.Size {
		case 640:
			//size = "hd480"
			scale = "scale=640:-2"
		default:
			log.Warnf("error invalid video size: %s", opts.Size)
			return "", ErrInvalidVideoSize
		}

		args = append(args, []string{
			//"-s", size,
			"-vf", scale,
		}...)
	}

	args = append(args, []string{
		"-c:v", "libvpx",
		"-c:a", "libvorbis",
		"-crf", "18",
		"-strict", "-2",
		"-loglevel", "quiet",
		of.Name(),
	}...)

	if err := RunCmd(
		conf.TranscoderTimeout,
		"ffmpeg",
		args...,
	); err != nil {
		log.WithError(err).Error("error transcoding video")
		return "", err
	}

	// Re-encode to MP4 (for older browsers)
	if err := of.Close(); err != nil {
		log.WithError(err).Warnf("error cloding file %s", fn)
	}
	if err := VideoToMp4(conf, fn); err != nil {
		log.WithError(err).Warnf("error reencoding video to MP4 (for older browsers: %s", fn)
	}

	return fmt.Sprintf(
		"%s/%s/%s",
		strings.TrimSuffix(conf.BaseURL, "/"),
		resource, filepath.Base(fn),
	), nil
}

/*
	// TODO: Make this a background job
	// Resize for lower quality options
	for size, suffix := range a.Config.Transcoder.Sizes {
		log.
			WithField("size", size).
			WithField("vf", filepath.Base(vf)).
			Info("resizing video for lower quality playback")
		sf := fmt.Sprintf(
			"%s#%s.mp4",
			strings.TrimSuffix(vf, filepath.Ext(vf)),
			suffix,
		)

		if err := utils.RunCmd(
			a.Config.Transcoder.Timeout,
			"ffmpeg",
			"-y",
			"-i", vf,
			"-s", size,
			"-c:v", "libx264",
			"-c:a", "aac",
			"-crf", "18",
			"-strict", "-2",
			"-loglevel", "quiet",
			"-metadata", fmt.Sprintf("title=%s", title),
			"-metadata", fmt.Sprintf("comment=%s", description),
			sf,
		); err != nil {
			err := fmt.Errorf("error transcoding video: %w", err)
			log.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
*/

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

func URLForBlogs(baseURL, author string) string {
	return fmt.Sprintf(
		"%s/blogs/%s",
		strings.TrimSuffix(baseURL, "/"),
		author,
	)
}

func URLForPage(baseURL, page string) string {
	return fmt.Sprintf(
		"%s/%s",
		strings.TrimSuffix(baseURL, "/"),
		page,
	)
}

func URLForTwt(baseURL, hash string) string {
	return fmt.Sprintf(
		"%s/twt/%s",
		strings.TrimSuffix(baseURL, "/"),
		hash,
	)
}

func URLForUser(conf *Config, username string) string {
	return fmt.Sprintf(
		"%s/user/%s/twtxt.txt",
		strings.TrimSuffix(conf.BaseURL, "/"),
		username,
	)
}

func URLForAvatar(conf *Config, username string) string {
	return fmt.Sprintf(
		"%s/user/%s/avatar",
		strings.TrimSuffix(conf.BaseURL, "/"),
		username,
	)
}

func URLForExternalProfile(conf *Config, nick, url string) string {
	return fmt.Sprintf(
		"%s/external/%s/%s",
		strings.TrimSuffix(conf.BaseURL, "/"),
		Slugify(url), nick,
	)
}

func URLForExternalAvatar(conf *Config, nick, url string) string {
	return fmt.Sprintf(
		"%s/external/%s/%s/avatar",
		strings.TrimSuffix(conf.BaseURL, "/"),
		Slugify(url), nick,
	)
}

func URLForBlogFactory(conf *Config, blogs *BlogsCache) func(twt types.Twt) string {
	return func(twt types.Twt) string {
		subject := twt.Subject()
		if subject == "" {
			return ""
		}

		var hash string

		re := regexp.MustCompile(`\(#([a-z0-9]+)\)`)
		match := re.FindStringSubmatch(subject)
		if match != nil {
			hash = match[1]
		} else {
			re = regexp.MustCompile(`(@|#)<([^ ]+) *([^>]+)>`)
			match = re.FindStringSubmatch(subject)
			if match != nil {
				hash = match[2]
			}
		}

		blogPost, ok := blogs.Get(hash)
		if !ok {
			return ""
		}

		return blogPost.URL(conf.BaseURL)
	}
}

func URLForConvFactory(conf *Config, cache *Cache) func(twt types.Twt) string {
	return func(twt types.Twt) string {
		subject := twt.Subject()
		if subject == "" {
			return ""
		}

		var hash string

		re := regexp.MustCompile(`\(#([a-z0-9]+)\)`)
		match := re.FindStringSubmatch(subject)
		if match != nil {
			hash = match[1]
		} else {
			re = regexp.MustCompile(`(@|#)<([^ ]+) *([^>]+)>`)
			match = re.FindStringSubmatch(subject)
			if match != nil {
				hash = match[2]
			}
		}

		if _, ok := cache.Lookup(hash); !ok {
			return ""
		}

		return fmt.Sprintf(
			"%s/conv/%s",
			strings.TrimSuffix(conf.BaseURL, "/"),
			hash,
		)
	}
}

func URLForTag(baseURL, tag string) string {
	return fmt.Sprintf(
		"%s/search?tag=%s",
		strings.TrimSuffix(baseURL, "/"),
		tag,
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

// UnparseTwtFactory is the opposite of CleanTwt and ExpandMentions/ExpandTags
func UnparseTwtFactory(conf *Config) func(text string) string {
	isLocalURL := IsLocalURLFactory(conf)
	return func(text string) string {
		text = strings.ReplaceAll(text, "\u2028", "\n")

		re := regexp.MustCompile(`(@|#)<([^ ]+) *([^>]+)>`)
		return re.ReplaceAllStringFunc(text, func(match string) string {
			parts := re.FindStringSubmatch(match)
			prefix, nick, uri := parts[1], parts[2], parts[3]

			switch prefix {
			case "@":
				if uri != "" && !isLocalURL(uri) {
					u, err := url.Parse(uri)
					if err != nil {
						log.WithField("uri", uri).Warn("UnparseTwt(): error parsing uri")
						return match
					}
					return fmt.Sprintf("@%s@%sd", nick, u.Hostname())
				}
				return fmt.Sprintf("@%s", nick)
			case "#":
				return fmt.Sprintf("#%s", nick)
			default:
				log.
					WithField("prefix", prefix).
					WithField("nick", nick).
					WithField("uri", uri).
					Warn("UnprocessTwt(): invalid prefix")
			}
			return match
		})
	}
}

// CleanTwt cleans a twt's text, replacing new lines with spaces and
// stripping surrounding spaces.
func CleanTwt(text string) string {
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\n", "\u2028")
	return text
}

// RenderAudio ...
func RenderAudio(conf *Config, uri string) string {
	isLocalURL := IsLocalURLFactory(conf)

	if isLocalURL(uri) {
		u, err := url.Parse(uri)
		if err != nil {
			log.WithError(err).Warnf("error parsing uri: %s", uri)
			return ""
		}

		oggURI := u.String()
		u.Path = ReplaceExt(u.Path, ".mp3")
		mp3URI := u.String()

		return fmt.Sprintf(`<audio controls="controls">
  <source type="audio/ogg" src="%s"></source>
  <source type="audio/mp3" src="%s"></source>
  Your browser does not support the audio element.
</audio>`, oggURI, mp3URI)
	}

	return fmt.Sprintf(`<audio controls="controls">
  <source type="audio/mp3" src="%s"></source>
  Your browser does not support the audio element.
</audio>`, uri)
}

// RenderVideo ...
func RenderVideo(conf *Config, uri string) string {
	isLocalURL := IsLocalURLFactory(conf)

	if isLocalURL(uri) {
		u, err := url.Parse(uri)
		if err != nil {
			log.WithError(err).Warnf("error parsing uri: %s", uri)
			return ""
		}

		webmURI := u.String()
		u.Path = ReplaceExt(u.Path, ".mp4")
		mp4URI := u.String()

		return fmt.Sprintf(`<video controls preload="metadata">
    <source type="video/webm" src="%s" />
    <source type="video/mp4" src="%s" />
    Your browser does not support the video element.
  </video>`, webmURI, mp4URI)
	}

	return fmt.Sprintf(`<video controls preload="metadata">
    <source type="video/mp4" src="%s" />
    Your browser does not support the video element.
    </video>`, uri)
}

// PreprocessMedia ...
func PreprocessMedia(conf *Config, u *url.URL, alt string) string {
	var html string

	// Normalize the domain name
	domain := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")

	whitelisted, local := conf.WhitelistedDomain(domain)

	if whitelisted {
		if local {
			// Ensure all local links match our BaseURL scheme
			u.Scheme = conf.baseURL.Scheme
		} else {
			// Ensure all extern links are served over TLS
			u.Scheme = "https"
		}

		switch filepath.Ext(u.Path) {
		case ".mp4", ".webm":
			html = RenderVideo(conf, u.String())
		case ".mp3", ".ogg":
			html = RenderAudio(conf, u.String())
		default:
			src := u.String()
			html = fmt.Sprintf(`<img alt="%s" src="%s" loading=lazy>`, alt, src)
		}
	} else {
		src := u.String()
		html = fmt.Sprintf(
			`<a href="%s" alt="%s" target="_blank"><i class="icss-image"></i></a>`,
			src, alt,
		)
	}

	return html
}

func FormatForDateTime(t time.Time) string {
	var format string

	dt := time.Since(t)

	if dt > YearAgo {
		format = "Mon, Jan 2 3:04PM 2006"
	} else if dt > MonthAgo {
		format = "Mon, Jan 2 3:04PM"
	} else if dt > WeekAgo {
		format = "Mon, Jan 2 3:04PM"
	} else if dt > DayAgo {
		format = "Mon 2, 3:04PM"
	} else {
		format = "3:04PM"
	}

	return format
}

// FormatTwtFactory formats a twt into a valid HTML snippet
func FormatTwtFactory(conf *Config) func(text string) template.HTML {
	return func(text string) template.HTML {
		renderHookProcessURLs := func(w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, bool) {
			// Ensure only whitelisted ![](url) images
			image, ok := node.(*ast.Image)
			if ok && entering {
				u, err := url.Parse(string(image.Destination))
				if err != nil {
					log.WithError(err).Warn("error parsing url")
					return ast.GoToNext, false
				}

				html := PreprocessMedia(conf, u, string(image.Title))

				io.WriteString(w, html)

				return ast.SkipChildren, true
			}

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

			// Ensure only whitelisted img src=(s) and fix non-secure links
			img := doc.Find("img")
			if img.Length() > 0 {
				src, ok := img.Attr("src")
				if !ok {
					return ast.GoToNext, false
				}

				alt, _ := img.Attr("alt")

				u, err := url.Parse(src)
				if err != nil {
					log.WithError(err).Warn("error parsing URL")
					return ast.GoToNext, false
				}

				html := PreprocessMedia(conf, u, alt)

				io.WriteString(w, html)

				return ast.GoToNext, true
			}

			// Let it go! Lget it go!
			return ast.GoToNext, false
		}

		// Replace  `LS: Line Separator, U+2028` with `\n` so the Markdown
		// renderer can interpreter newlines as `<br />` and `<p>`.
		text = strings.ReplaceAll(text, "\u2028", "\n")

		extensions := parser.CommonExtensions | parser.HardLineBreak
		mdParser := parser.NewWithExtensions(extensions)

		htmlFlags := html.CommonFlags | html.HrefTargetBlank
		opts := html.RendererOptions{
			Flags:          htmlFlags,
			Generator:      "",
			RenderNodeHook: renderHookProcessURLs,
		}
		renderer := html.NewRenderer(opts)

		md := []byte(FormatMentionsAndTags(conf, text))
		maybeUnsafeHTML := markdown.ToHTML(md, mdParser, renderer)
		p := bluemonday.UGCPolicy()
		p.AllowAttrs("id", "controls").OnElements("audio")
		p.AllowAttrs("id", "controls", "preload", "poster").OnElements("video")
		p.AllowAttrs("src", "type").OnElements("source")
		p.AllowAttrs("target").OnElements("a")
		p.AllowAttrs("class").OnElements("i")
		p.AllowAttrs("alt", "loading").OnElements("a", "img")
		p.AllowAttrs("style").OnElements("a", "code", "img", "p", "pre", "span")
		html := p.SanitizeBytes(maybeUnsafeHTML)

		return template.HTML(html)
	}
}

// FormatMentionsAndTags turns `@<nick URL>` into `<a href="URL">@nick</a>`
// and `#<tag URL>` into `<a href="URL">#tag</a>` and a `!<hash URL>`
// into a `<a href="URL">!hash</a>`.
func FormatMentionsAndTags(conf *Config, text string) string {
	isLocalURL := IsLocalURLFactory(conf)
	re := regexp.MustCompile(`(@|#)<([^ ]+) *([^>]+)>`)
	return re.ReplaceAllStringFunc(text, func(match string) string {
		parts := re.FindStringSubmatch(match)
		prefix, nick, url := parts[1], parts[2], parts[3]
		switch prefix {
		case "@":
			if isLocalURL(url) && strings.HasSuffix(url, "/twtxt.txt") {
				return fmt.Sprintf(
					`<a href="%s">@%s</a>`,
					UserURL(url), nick,
				)
			}
			return fmt.Sprintf(
				`<a href="%s">@%s</a>`,
				URLForExternalProfile(conf, nick, url), nick,
			)
		default:
			return fmt.Sprintf(
				`<a href="%s">%s%s</a>`,
				url, prefix, nick,
			)
		}
	})
}

// FormatMentionsAndTagsForSubject turns `@<nick URL>` into `@nick`
func FormatMentionsAndTagsForSubject(text string) string {
	re := regexp.MustCompile(`(@|#)<([^ ]+) *([^>]+)>`)
	return re.ReplaceAllStringFunc(text, func(match string) string {
		parts := re.FindStringSubmatch(match)
		prefix, nick := parts[1], parts[2]
		return fmt.Sprintf(`%s%s`, prefix, nick)
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

func GetMediaNamesFromText(text string) []string {

	var mediaNames []string

	textSplit := strings.Split(text, "![](")

	for i, textSplitItem := range textSplit {
		if i > 0 {
			mediaEndIndex := strings.Index(textSplitItem, ")")
			mediaURL := textSplitItem[:mediaEndIndex]

			mediaURLSplit := strings.Split(mediaURL, "media/")
			for j, mediaURLSplitItem := range mediaURLSplit {
				if j > 0 {
					mediaPath := mediaURLSplitItem
					mediaNames = append(mediaNames, mediaPath)
				}
			}
		}
	}

	return mediaNames
}
