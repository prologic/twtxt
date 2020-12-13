package internal

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base32"
	"errors"
	"fmt"
	"html/template"
	"image"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	// Blank import so we can handle image/jpeg
	_ "image/gif"
	_ "image/jpeg"
	"image/png"

	"github.com/PuerkitoBio/goquery"
	"github.com/bakape/thumbnailer/v2"
	"github.com/chai2010/webp"
	"github.com/disintegration/gift"
	"github.com/disintegration/imageorient"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/goware/urlx"
	"github.com/h2non/filetype"
	"github.com/jointwt/twtxt"
	"github.com/jointwt/twtxt/types"
	"github.com/jointwt/twtxt/types/retwt"
	shortuuid "github.com/lithammer/shortuuid/v3"
	"github.com/microcosm-cc/bluemonday"
	"github.com/nullrocks/identicon"
	log "github.com/sirupsen/logrus"
	"github.com/writeas/slug"
	"golang.org/x/crypto/blake2b"
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

// TwtTextFormat represents the format of which the twt text gets formatted to
type TwtTextFormat int

const (
	// MarkdownFmt to use markdown format
	MarkdownFmt TwtTextFormat = iota
	// HTMLFmt to use HTML format
	HTMLFmt
	// TextFmt to use for og:description
	TextFmt
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

	validFeedName  = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)
	validUsername  = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]+$`)
	userAgentRegex = regexp.MustCompile(`(.*?)\s+?\(\+?(https?://.*?);? @?(.*)\)`)

	ErrInvalidFeedName  = errors.New("error: invalid feed name")
	ErrBadRequest       = errors.New("error: request failed with non-200 response")
	ErrFeedNameTooLong  = errors.New("error: feed name is too long")
	ErrInvalidUsername  = errors.New("error: invalid username")
	ErrUsernameTooLong  = errors.New("error: username is too long")
	ErrInvalidUserAgent = errors.New("error: invalid twtxt user agent")
	ErrReservedUsername = errors.New("error: username is reserved")
	ErrInvalidImage     = errors.New("error: invalid image")
	ErrInvalidAudio     = errors.New("error: invalid audio")
	ErrInvalidVideo     = errors.New("error: invalid video")
	ErrInvalidVideoSize = errors.New("error: invalid video size")

	thumbnailerOpts = thumbnailer.Options{
		ThumbDims: thumbnailer.Dims{
			Width:  640,
			Height: 480,
		},
	}
)

func FastHash(s string) string {
	sum := blake2b.Sum256([]byte(s))

	// Base32 is URL-safe, unlike Base64, and shorter than hex.
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	hash := strings.ToLower(encoding.EncodeToString(sum[:]))

	return hash
}

func IntPow(x, y int) int {
	return int(math.Pow(float64(x), float64(y)))
}

func Slugify(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		log.WithError(err).Warnf("Slugify(): error parsing uri: %s", uri)
		return ""
	}

	return slug.Make(fmt.Sprintf("%s/%s", u.Hostname(), u.Path))
}

func GenerateAvatar(conf *Config, username string) (image.Image, error) {
	ig, err := identicon.New(conf.Name, 7, 4)
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

func GetExternalAvatar(conf *Config, nick, uri string) string {
	slug := Slugify(uri)

	fn := filepath.Join(conf.Data, externalDir, fmt.Sprintf("%s.webp", slug))
	if FileExists(fn) {
		return URLForExternalAvatar(conf, uri)
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
			opts := &ImageOptions{Resize: true, Width: AvatarResolution, Height: AvatarResolution}
			_, err := DownloadImage(conf, source.String(), externalDir, slug, opts)
			if err != nil {
				log.WithError(err).
					WithField("base", base.String()).
					WithField("source", source.String()).
					Error("error downloading external avatar")
				return ""
			}
			return URLForExternalAvatar(conf, uri)
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

	// Set a default User-Agent (if none set)
	if headers.Get("User-Agent") == "" {
		headers.Set(
			"User-Agent",
			fmt.Sprintf(
				"twtxt/%s (Pod: %s Support: %s)",
				twtxt.FullVersion(), conf.Name, URLForPage(conf.BaseURL, "support"),
			),
		)
	}

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

func LineCount(r io.Reader) (int, error) {
	buf := make([]byte, 32*1024)
	count := 0
	lineSep := []byte{'\n'}

	for {
		c, err := r.Read(buf)
		count += bytes.Count(buf[:c], lineSep)

		switch {
		case err == io.EOF:
			return count, nil

		case err != nil:
			return count, err
		}
	}
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
		if exitError, ok := err.(*exec.ExitError); ok {
			if ws, ok := exitError.Sys().(syscall.WaitStatus); ok && ws.Signal() == syscall.SIGKILL {
				err = &ErrCommandKilled{Err: err, Signal: ws.Signal()}
			} else {
				err = &ErrCommandFailed{Err: err, Status: exitError.ExitCode()}
			}
		}

		log.
			WithError(err).
			WithField("out", string(out)).
			Errorf("error running command")

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

func IsAudio(fn string) bool {
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

	if filetype.IsAudio(head) {
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
	Resize bool
	Width  int
	Height int
}

type AudioOptions struct {
	Resample   bool
	Channels   int
	Samplerate int
	Bitrate    int
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

	return ProcessImage(conf, tf.Name(), resource, name, opts)
}

func ReceiveAudio(r io.Reader) (string, error) {
	tf, err := ioutil.TempFile("", "twtxt-upload-*")
	if err != nil {
		log.WithError(err).Error("error creating temporary file")
		return "", err
	}

	if _, err := io.Copy(tf, r); err != nil {
		log.WithError(err).Error("error writng temporary file")
		return "", err
	}

	if _, err := tf.Seek(0, io.SeekStart); err != nil {
		log.WithError(err).Error("error seeking temporary file")
		return "", err
	}

	if !IsAudio(tf.Name()) {
		return "", ErrInvalidAudio
	}

	return tf.Name(), nil
}

func ReceiveImage(r io.Reader) (string, error) {
	tf, err := ioutil.TempFile("", "twtxt-upload-*")
	if err != nil {
		log.WithError(err).Error("error creating temporary file")
		return "", err
	}

	if _, err := io.Copy(tf, r); err != nil {
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

	return tf.Name(), nil
}

func ReceiveVideo(r io.Reader) (string, error) {
	tf, err := ioutil.TempFile("", "twtxt-upload-*")
	if err != nil {
		log.WithError(err).Error("error creating temporary file")
		return "", err
	}

	if _, err := io.Copy(tf, r); err != nil {
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

	return tf.Name(), nil
}

func TranscodeAudio(conf *Config, ifn string, resource, name string, opts *AudioOptions) (string, error) {
	p := filepath.Join(conf.Data, resource)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Errorf("error creating %s directory", resource)
		return "", err
	}

	var ofn string

	if name == "" {
		uuid := shortuuid.New()
		ofn = filepath.Join(p, fmt.Sprintf("%s.ogg", uuid))
	} else {
		ofn = fmt.Sprintf("%s.ogg", filepath.Join(p, name))
	}

	of, err := os.OpenFile(ofn, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.WithError(err).Error("error opening output file")
		return "", err
	}
	defer of.Close()

	wg := sync.WaitGroup{}

	TranscodeOGG := func(ctx context.Context, errs chan error) {
		defer wg.Done()

		args := []string{"-y", "-i", ifn}

		if opts.Resample {
			args = append(args, []string{
				"-ac", fmt.Sprintf("%d", opts.Channels),
				"-ar", fmt.Sprintf("%d", opts.Samplerate),
				"-b:a", fmt.Sprintf("%dk", opts.Bitrate),
			}...)
		}

		args = append(args, []string{
			"-c:a", "libvorbis",
			"-strict", "-2",
			"-loglevel", "quiet",
			of.Name(),
		}...)

		if err := RunCmd(
			conf.TranscoderTimeout,
			"ffmpeg",
			args...,
		); err != nil {
			log.WithError(err).Error("error transcoding audio")
			errs <- err
			return
		}
	}

	TranscodeMP3 := func(ctx context.Context, errs chan error) {
		defer wg.Done()

		if err := RunCmd(
			conf.TranscoderTimeout,
			"ffmpeg",
			"-y",
			"-i", ifn,
			"-acodec", "mp3",
			"-strict", "-2",
			"-loglevel", "quiet",
			ReplaceExt(ofn, ".mp3"),
		); err != nil {
			log.WithError(err).Error("error transcoding video")
			errs <- err
			return
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var finalErr error

	nErrors := 0
	errChan := make(chan error)

	wg.Add(2)

	go TranscodeOGG(ctx, errChan)
	go TranscodeMP3(ctx, errChan)

	go func(ctx context.Context) {
		for {
			select {
			case err, ok := <-errChan:
				if !ok {
					return
				}
				nErrors++
				log.WithError(err).Errorf("TranscodeVideo() error")

				if errors.Is(err, &ErrCommandKilled{}) {
					finalErr = &ErrTranscodeTimeout{Err: err}
				} else {
					finalErr = &ErrTranscodeFailed{Err: err}
				}
			case <-ctx.Done():
				return
			}
		}
	}(ctx)

	wg.Wait()
	close(errChan)

	if nErrors > 0 {
		err = &ErrAudioUploadFailed{Err: finalErr}
		log.WithError(err).Error("TranscodeAudio() too many errors")
		return "", err
	}

	return fmt.Sprintf(
		"%s/%s/%s",
		strings.TrimSuffix(conf.BaseURL, "/"),
		resource, filepath.Base(ofn),
	), nil
}

func ProcessImage(conf *Config, ifn string, resource, name string, opts *ImageOptions) (string, error) {
	p := filepath.Join(conf.Data, resource)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating avatars directory")
		return "", err
	}

	var ofn string

	if name == "" {
		uuid := shortuuid.New()
		ofn = filepath.Join(p, fmt.Sprintf("%s.webp", uuid))
	} else {
		ofn = fmt.Sprintf("%s.webp", filepath.Join(p, name))
	}

	f, err := os.Open(ifn)
	if err != nil {
		log.WithError(err).Error("error opening input file")
		return "", err
	}
	defer f.Close()

	img, _, err := imageorient.Decode(f)
	if err != nil {
		log.WithError(err).Error("imageorient.Decode failed")
		return "", err
	}

	g := gift.New(
		gift.UnsharpMask(1, 1, 0),
	)

	if opts != nil && opts.Resize {
		if opts.Width > 0 && opts.Height > 0 {
			g.Add(gift.ResizeToFit(opts.Width, opts.Height, gift.LanczosResampling))
		} else if (opts.Width+opts.Height > 0) && (opts.Height > 0 || img.Bounds().Size().X > opts.Width) {
			g.Add(gift.Resize(opts.Width, opts.Height, gift.LanczosResampling))
		} else {
			log.Warnf(
				"not resizing image with bounds %s to %dx%d",
				img.Bounds(), opts.Width, opts.Height,
			)
		}
	}

	newImg := image.NewRGBA(g.Bounds(img.Bounds()))

	g.Draw(newImg, img)

	of, err := os.OpenFile(ofn, os.O_WRONLY|os.O_CREATE, 0644)
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
		log.WithError(err).Warnf("error closing file %s", ofn)
	}
	if err := ImageToPng(ofn); err != nil {
		log.WithError(err).Warnf("error reencoding image to PNG (for older browsers: %s", ofn)
	}

	return fmt.Sprintf(
		"%s/%s/%s",
		strings.TrimSuffix(conf.BaseURL, "/"),
		resource, strings.TrimSuffix(filepath.Base(ofn), filepath.Ext(ofn)),
	), nil
}

func TranscodeVideo(conf *Config, ifn string, resource, name string, opts *VideoOptions) (string, error) {
	p := filepath.Join(conf.Data, resource)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Errorf("error creating %s directory", resource)
		return "", err
	}

	var ofn string

	if name == "" {
		uuid := shortuuid.New()
		ofn = filepath.Join(p, fmt.Sprintf("%s.webm", uuid))
	} else {
		ofn = fmt.Sprintf("%s.webm", filepath.Join(p, name))
	}

	of, err := os.OpenFile(ofn, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.WithError(err).Error("error opening output file")
		return "", err
	}
	defer of.Close()

	wg := sync.WaitGroup{}

	TranscodeWebM := func(ctx context.Context, errs chan error) {
		defer wg.Done()

		args := []string{"-y", "-i", ifn}

		if opts.Resize {
			var scale string

			switch opts.Size {
			case 640:
				scale = "scale=640:-2"
			default:
				log.Warnf("error invalid video size: %d", opts.Size)
				errs <- ErrInvalidVideoSize
				return
			}

			args = append(args, []string{
				"-vf", scale,
			}...)
		}

		args = append(args, []string{
			"-c:v", "libvpx",
			"-c:a", "libvorbis",
			"-crf", "18",
			"-r", "24",
			"-preset", "ultrafast",
			"-strict", "-2",
			"-loglevel", "quiet",
			ofn,
		}...)

		if err := RunCmd(
			conf.TranscoderTimeout,
			"ffmpeg",
			args...,
		); err != nil {
			log.WithError(err).Error("error transcoding video")
			errs <- err
			return
		}
	}

	TranscodeMP4 := func(ctx context.Context, errs chan error) {
		defer wg.Done()

		if err := RunCmd(
			conf.TranscoderTimeout,
			"ffmpeg",
			"-y",
			"-i", ifn,
			"-r", "24",
			"-preset", "ultrafast",
			"-vcodec", "h264",
			"-acodec", "aac",
			"-strict", "-2",
			"-loglevel", "quiet",
			ReplaceExt(ofn, ".mp4"),
		); err != nil {
			log.WithError(err).Error("error transcoding video")
			errs <- err
			return
		}
	}

	GeneratePoster := func(ctx context.Context, errs chan error) {
		defer wg.Done()

		f, err := os.Open(ifn)
		if err != nil {
			log.WithError(err).Error("error generating video poster thumbnail")
			errs <- err
			return
		}
		defer f.Close()

		// Generate poster / thumbnail
		_, thumb, err := thumbnailer.Process(f, thumbnailerOpts)
		if err != nil {
			log.WithError(err).Error("error generating video poster thumbnail")
			errs <- err
			return
		}

		pf, err := os.OpenFile(ReplaceExt(ofn, ".webp"), os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			log.WithError(err).Error("error opening thumbnail output file")
			errs <- err
			return
		}
		defer pf.Close()

		if err := webp.Encode(pf, thumb, &webp.Options{Lossless: true}); err != nil {
			log.WithError(err).Error("error reencoding thumbnail image")
			errs <- err
			return
		}

		if err := ImageToPng(ReplaceExt(ofn, ".webp")); err != nil {
			log.WithError(err).Errorf("error reencoding thumbnail image to PNG (for older browsers: %s", ofn)
			errs <- err
			return
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var finalErr error

	nErrors := 0
	errChan := make(chan error)

	wg.Add(3)

	go TranscodeWebM(ctx, errChan)
	go TranscodeMP4(ctx, errChan)
	go GeneratePoster(ctx, errChan)

	go func(ctx context.Context) {
		for {
			select {
			case err, ok := <-errChan:
				if !ok {
					return
				}
				nErrors++
				log.WithError(err).Errorf("TranscodeVideo() error")

				if errors.Is(err, &ErrCommandKilled{}) {
					finalErr = &ErrTranscodeTimeout{Err: err}
				} else {
					finalErr = &ErrTranscodeFailed{Err: err}
				}
			case <-ctx.Done():
				return
			}
		}
	}(ctx)

	wg.Wait()
	close(errChan)

	if nErrors > 0 {
		err = &ErrVideoUploadFailed{Err: finalErr}
		log.WithError(err).Error("TranscodeVideo() too many errors")
		return "", err
	}

	return fmt.Sprintf(
		"%s/%s/%s",
		strings.TrimSuffix(conf.BaseURL, "/"),
		resource, filepath.Base(ofn),
	), nil
}

func StoreUploadedImage(conf *Config, r io.Reader, resource, name string, opts *ImageOptions) (string, error) {
	fn, err := ReceiveImage(r)
	if err != nil {
		log.WithError(err).Error("error receiving image")
		return "", err
	}

	return ProcessImage(conf, fn, resource, name, opts)
}

func NormalizeFeedName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ToLower(name)
	return name
}

func ValidateFeed(conf *Config, nick, url string) error {
	res, err := Request(conf, http.MethodGet, url, nil)
	if err != nil {
		log.WithError(err).Errorf("error fetching feed %s", url)
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return ErrBadRequest
	}

	limitedReader := &io.LimitedReader{R: res.Body, N: conf.MaxFetchLimit}
	scanner := bufio.NewScanner(limitedReader)
	twter := types.Twter{Nick: nick, URL: url}
	_, _, err = retwt.ParseFile(scanner, twter, conf.MaxCacheTTL, conf.MaxCacheItems)
	if err != nil {
		return err
	}

	return nil
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

func (u URI) IsZero() bool {
	return u.Type == "" && u.Path == ""
}

func (u URI) String() string {
	return fmt.Sprintf("%s://%s", u.Type, u.Path)
}

type TwtxtUserAgent struct {
	Client string
	Nick   string
	URL    string
}

func DetectFollowerFromUserAgent(ua string) (*TwtxtUserAgent, error) {
	match := userAgentRegex.FindStringSubmatch(ua)
	if match == nil {
		return nil, ErrInvalidUserAgent
	}

	return &TwtxtUserAgent{
		Client: match[1],
		URL:    match[2],
		Nick:   match[3],
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

func HostnameFromURL(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		log.WithError(err).Warnf("HostnameFromURL(): error parsing url: %s", uri)
		return uri
	}

	return u.Hostname()
}

func PrettyURL(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		log.WithError(err).Warnf("PrettyURL(): error parsing url: %s", uri)
		return uri
	}

	return fmt.Sprintf("%s/%s", u.Hostname(), strings.TrimPrefix(u.EscapedPath(), "/"))
}

// IsAdminUserFactory returns a function that returns true if the user provided
// is the configured pod administrator, false otherwise.
func IsAdminUserFactory(conf *Config) func(user *User) bool {
	return func(user *User) bool {
		return NormalizeUsername(conf.AdminUser) == NormalizeUsername(user.Username)
	}
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

func URLForExternalProfile(conf *Config, nick, uri string) string {
	return fmt.Sprintf(
		"%s/external?uri=%s&nick=%s",
		strings.TrimSuffix(conf.BaseURL, "/"),
		uri, nick,
	)
}

func URLForExternalAvatar(conf *Config, uri string) string {
	return fmt.Sprintf(
		"%s/externalAvatar?uri=%s",
		strings.TrimSuffix(conf.BaseURL, "/"),
		uri,
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

func URLForTask(baseURL, uuid string) string {
	return fmt.Sprintf(
		"%s/task/%s",
		strings.TrimSuffix(baseURL, "/"),
		uuid,
	)
}

func URLForWhoFollows(baseURL string, feed types.Feed) string {
	token := GenerateToken()

	return fmt.Sprintf(
		"%s/whoFollows?uri=%s&nick=%s&token=%s",
		strings.TrimSuffix(baseURL, "/"),
		feed.URL, feed.Nick, token,
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

// FilterTwts filters out Twts from users/feeds that a User has chosen to mute
func FilterTwts(user *User, twts types.Twts) (filtered types.Twts) {
	if user == nil {
		return twts
	}
	return user.Filter(twts)
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

		u.Path = ReplaceExt(u.Path, "")
		posterURI := u.String()

		return fmt.Sprintf(`<video controls playsinline preload="auto" poster="%s">
    <source type="video/webm" src="%s" />
    <source type="video/mp4" src="%s" />
    Your browser does not support the video element.
  </video>`, posterURI, webmURI, mp4URI)
	}

	return fmt.Sprintf(`<video controls playsinline preload="auto">
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

				_, _ = io.WriteString(w, html)

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

				_, _ = io.WriteString(w, html)

				return ast.GoToNext, true
			}

			// Let it go! Lget it go!
			return ast.GoToNext, false
		}

		// Replace  `LS: Line Separator, U+2028` with `\n` so the Markdown
		// renderer can interpreter newlines as `<br />` and `<p>`.
		text = strings.ReplaceAll(text, "\u2028", "\n")
		// Replace simple '#just-tag' entrys with local link
		text = ExpandTag(conf, nil, nil, text)
		extensions := parser.CommonExtensions | parser.HardLineBreak | parser.NoEmptyLineBeforeBlock
		mdParser := parser.NewWithExtensions(extensions)

		htmlFlags := html.CommonFlags | html.HrefTargetBlank
		opts := html.RendererOptions{
			Flags:          htmlFlags,
			Generator:      "",
			RenderNodeHook: renderHookProcessURLs,
		}
		renderer := html.NewRenderer(opts)

		md := []byte(FormatMentionsAndTags(conf, text, HTMLFmt))
		maybeUnsafeHTML := markdown.ToHTML(md, mdParser, renderer)
		p := bluemonday.UGCPolicy()
		p.AllowAttrs("id", "controls").OnElements("audio")
		p.AllowAttrs("id", "controls", "playsinline", "preload", "poster").OnElements("video")
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
func FormatMentionsAndTags(conf *Config, text string, format TwtTextFormat) string {
	isLocalURL := IsLocalURLFactory(conf)
	re := regexp.MustCompile(`(@|#)<([^ ]+) *([^>]+)>`)
	return re.ReplaceAllStringFunc(text, func(match string) string {
		parts := re.FindStringSubmatch(match)
		prefix, nick, url := parts[1], parts[2], parts[3]

		if format == TextFmt {
			switch prefix {
			case "@":
				if isLocalURL(url) && strings.HasSuffix(url, "/twtxt.txt") {
					return fmt.Sprintf("%s@%s", nick, conf.baseURL.Hostname())
				}
				return fmt.Sprintf("@%s", nick)
			default:
				return fmt.Sprintf("%s%s", prefix, nick)
			}
		}

		if format == HTMLFmt {
			switch prefix {
			case "@":
				if isLocalURL(url) && strings.HasSuffix(url, "/twtxt.txt") {
					return fmt.Sprintf(`<a href="%s">@%s</a>`, UserURL(url), nick)
				}
				return fmt.Sprintf(`<a href="%s">@%s</a>`, URLForExternalProfile(conf, nick, url), nick)
			default:
				return fmt.Sprintf(`<a href="%s">%s%s</a>`, url, prefix, nick)
			}
		}

		switch prefix {
		case "@":
			// Using (#) anchors to add the nick to URL for now. The Fluter app needs it since
			// 	the Markdown plugin doesn't include the link text that contains the nick in its onTap callback
			// https://github.com/flutter/flutter_markdown/issues/286
			return fmt.Sprintf(`[@%s](%s#%s)`, nick, url, nick)
		default:
			return fmt.Sprintf(`[%s%s](%s)`, prefix, nick, url)
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
