package main

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"

	"github.com/prologic/twtxt"
	"github.com/prologic/twtxt/internal"
)

var (
	bind    string
	debug   bool
	version bool

	data          string
	store         string
	name          string
	theme         string
	register      bool
	baseURL       string
	adminUser     string
	feedSources   []string
	cookieSecret  string
	twtsPerPage   int
	maxTwtLength  int
	openProfiles  bool
	maxUploadSize int64
	sessionExpiry time.Duration

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

	flag.StringVarP(&data, "data", "d", internal.DefaultData, "data directory")
	flag.StringVarP(&store, "store", "s", internal.DefaultStore, "store to use")
	flag.StringVarP(&name, "name", "n", internal.DefaultName, "set the instance's name")
	flag.StringVarP(&theme, "theme", "t", internal.DefaultTheme, "set the default theme")
	flag.BoolVarP(&register, "register", "r", internal.DefaultRegister, "enable user registration")
	flag.StringVarP(&baseURL, "base-url", "u", internal.DefaultBaseURL, "base url to use")
	flag.StringVarP(&adminUser, "admin-user", "A", internal.DefaultAdminUser, "default admin user to use")
	flag.StringSliceVarP(&feedSources, "feed-sources", "F", internal.DefaultFeedSources, "external feed sources")
	flag.StringVarP(&cookieSecret, "cookie-secret", "S", internal.DefaultCookieSecret, "cookie secret to use")
	flag.IntVarP(&maxTwtLength, "max-twt-length", "L", internal.DefaultMaxTwtLength, "maximum length of posts")
	flag.BoolVarP(&openProfiles, "open-profiles", "O", internal.DefaultOpenProfiles, "whether or not to have open user profiles")
	flag.Int64VarP(&maxUploadSize, "max-upload-size", "U", internal.DefaultMaxUploadSize, "maximum upload size of media")
	flag.IntVarP(&twtsPerPage, "twts-per-page", "T", internal.DefaultTwtsPerPage, "twts per page to display")
	flag.DurationVarP(&sessionExpiry, "session-expiry", "E", internal.DefaultSessionExpiry, "session expiry to use")

	flag.StringVar(&magiclinkSecret, "magiclink-secret", internal.DefaultMagicLinkSecret, "magiclink secret to use for password reset tokens")

	flag.StringVar(&smtpHost, "smtp-host", internal.DefaultSMTPHost, "SMTP Host to use for email sending")
	flag.IntVar(&smtpPort, "smtp-port", internal.DefaultSMTPPort, "SMTP Port to use for email sending")
	flag.StringVar(&smtpUser, "smtp-user", internal.DefaultSMTPUser, "SMTP User to use for email sending")
	flag.StringVar(&smtpPass, "smtp-pass", internal.DefaultSMTPPass, "SMTP Pass to use for email sending")
	flag.StringVar(&smtpFrom, "smtp-from", internal.DefaultSMTPFrom, "SMTP From address to use for email sending")

	flag.Int64Var(&maxFetchLimit, "max-fetch-limit", internal.DefaultMaxFetchLimit, "Maximum feed fetch limit in bytes")

	flag.DurationVar(&apiSessionTime, "api-session-time", internal.DefaultAPISessionTime, "Maximum TTL for API tokens")
	flag.StringVar(&apiSigningKey, "api-signing-key", internal.DefaultAPISigningKey, "API JWT signing key for tokens")
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

	svr, err := internal.NewServer(bind,
		internal.WithData(data),
		internal.WithName(name),
		internal.WithTheme(theme),
		internal.WithStore(store),
		internal.WithBaseURL(baseURL),
		internal.WithRegister(register),
		internal.WithAdminUser(adminUser),
		internal.WithFeedSources(feedSources),
		internal.WithCookieSecret(cookieSecret),
		internal.WithTwtsPerPage(twtsPerPage),
		internal.WithSessionExpiry(sessionExpiry),
		internal.WithMaxTwtLength(maxTwtLength),
		internal.WithOpenProfiles(openProfiles),
		internal.WithMaxUploadSize(maxUploadSize),
		internal.WithMagicLinkSecret(magiclinkSecret),

		internal.WithSMTPHost(smtpHost),
		internal.WithSMTPPort(smtpPort),
		internal.WithSMTPUser(smtpUser),
		internal.WithSMTPPass(smtpPass),
		internal.WithSMTPFrom(smtpFrom),

		internal.WithMaxFetchLimit(maxFetchLimit),

		internal.WithAPISessionTime(apiSessionTime),
		internal.WithAPISigningKey(apiSigningKey),
	)
	if err != nil {
		log.WithError(err).Fatal("error creating server")
	}

	log.Infof("%s listening on http://%s", path.Base(os.Args[0]), bind)
	if err := svr.Run(); err != nil {
		log.WithError(err).Fatal("error running or shutting down server")
	}
}
