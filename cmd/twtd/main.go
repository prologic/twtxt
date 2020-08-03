package main

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"

	twtxt "github.com/prologic/twtxt/internal"
)

var (
	bind    string
	debug   bool
	version bool

	data           string
	store          string
	name           string
	theme          string
	register       bool
	baseURL        string
	adminUser      string
	feedSources    []string
	cookieSecret   string
	tweetsPerPage  int
	maxTweetLength int
	maxUploadSize  int64
	sessionExpiry  time.Duration

	magiclinkSecret string
	smtpHost        string
	smtpPort        int
	smtpUser        string
	smtpPass        string
	smtpFrom        string

	maxFetchLimit int64

	apiSessionTime time.Duration
	apiSigningKey  string
)

func init() {
	flag.BoolVarP(&version, "version", "v", false, "display version information")
	flag.BoolVarP(&debug, "debug", "D", false, "enable debug logging")
	flag.StringVarP(&bind, "bind", "b", "0.0.0.0:8000", "[int]:<port> to bind to")

	flag.StringVarP(&data, "data", "d", twtxt.DefaultData, "data directory")
	flag.StringVarP(&store, "store", "s", twtxt.DefaultStore, "store to use")
	flag.StringVarP(&name, "name", "n", twtxt.DefaultName, "set the instance's name")
	flag.StringVarP(&theme, "theme", "t", twtxt.DefaultTheme, "set the default theme")
	flag.BoolVarP(&register, "register", "r", twtxt.DefaultRegister, "enable user registration")
	flag.StringVarP(&baseURL, "base-url", "u", twtxt.DefaultBaseURL, "base url to use")
	flag.StringVarP(&adminUser, "admin-user", "A", twtxt.DefaultAdminUser, "default admin user to use")
	flag.StringSliceVarP(&feedSources, "feed-sources", "F", twtxt.DefaultFeedSources, "external feed sources")
	flag.StringVarP(&cookieSecret, "cookie-secret", "S", twtxt.DefaultCookieSecret, "cookie secret to use")
	flag.IntVarP(&maxTweetLength, "max-tweet-length", "L", twtxt.DefaultMaxTweetLength, "maximum length of posts")
	flag.Int64VarP(&maxUploadSize, "max-upload-size", "U", twtxt.DefaultMaxUploadSize, "maximum upload size of media")
	flag.IntVarP(&tweetsPerPage, "tweets-per-page", "T", twtxt.DefaultTweetsPerPage, "tweets per page to display")
	flag.DurationVarP(&sessionExpiry, "session-expiry", "E", twtxt.DefaultSessionExpiry, "session expiry to use")

	flag.StringVar(&magiclinkSecret, "magiclink-secret", twtxt.DefaultMagicLinkSecret, "magiclink secret to use for password reset tokens")

	flag.StringVar(&smtpHost, "smtp-host", twtxt.DefaultSMTPHost, "SMTP Host to use for email sending")
	flag.IntVar(&smtpPort, "smtp-port", twtxt.DefaultSMTPPort, "SMTP Port to use for email sending")
	flag.StringVar(&smtpUser, "smtp-user", twtxt.DefaultSMTPUser, "SMTP User to use for email sending")
	flag.StringVar(&smtpPass, "smtp-pass", twtxt.DefaultSMTPPass, "SMTP Pass to use for email sending")
	flag.StringVar(&smtpFrom, "smtp-from", twtxt.DefaultSMTPFrom, "SMTP From address to use for email sending")

	flag.Int64Var(&maxFetchLimit, "max-fetch-limit", twtxt.DefaultMaxFetchLimit, "Maximum feed fetch limit in bytes")

	flag.DurationVar(&apiSessionTime, "api-session-time", twtxt.DefaultAPISessionTime, "Maximum TTL for API tokens")
	flag.StringVar(&apiSigningKey, "api-signing-key", twtxt.DefaultAPISigningKey, "API JWT signing key for tokens")
}

func flagNameFromEnvironmentName(s string) string {
	s = strings.ToLower(s)
	s = strings.Replace(s, "_", "-", -1)
	return s
}

func ParseArgs() error {
	for _, v := range os.Environ() {
		vals := strings.SplitN(v, "=", 2)
		flagName := flagNameFromEnvironmentName(vals[0])
		fn := flag.CommandLine.Lookup(flagName)
		if fn == nil || fn.Changed {
			continue
		}
		if err := fn.Value.Set(vals[1]); err != nil {
			return err
		}
	}
	flag.Parse()
	return nil
}

func main() {
	ParseArgs()

	if version {
		fmt.Printf("twtxt v%s", twtxt.FullVersion())
		os.Exit(0)
	}

	if debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	svr, err := twtxt.NewServer(bind,
		twtxt.WithData(data),
		twtxt.WithName(name),
		twtxt.WithTheme(theme),
		twtxt.WithStore(store),
		twtxt.WithBaseURL(baseURL),
		twtxt.WithRegister(register),
		twtxt.WithAdminUser(adminUser),
		twtxt.WithFeedSources(feedSources),
		twtxt.WithCookieSecret(cookieSecret),
		twtxt.WithTweetsPerPage(tweetsPerPage),
		twtxt.WithSessionExpiry(sessionExpiry),
		twtxt.WithMaxTweetLength(maxTweetLength),
		twtxt.WithMaxUploadSize(maxUploadSize),
		twtxt.WithMagicLinkSecret(magiclinkSecret),

		twtxt.WithSMTPHost(smtpHost),
		twtxt.WithSMTPPort(smtpPort),
		twtxt.WithSMTPUser(smtpUser),
		twtxt.WithSMTPPass(smtpPass),
		twtxt.WithSMTPFrom(smtpFrom),

		twtxt.WithMaxFetchLimit(maxFetchLimit),

		twtxt.WithAPISessionTime(apiSessionTime),
		twtxt.WithAPISigningKey(apiSigningKey),
	)
	if err != nil {
		log.WithError(err).Fatal("error creating server")
	}

	log.Infof("%s listening on http://%s", path.Base(os.Args[0]), bind)
	if err := svr.Run(); err != nil {
		log.WithError(err).Fatal("error running or shutting down server")
	}
}
