package main

import (
	"expvar"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	profiler "github.com/wblakecaldwell/profiler"

	"github.com/prologic/twtxt"
	"github.com/prologic/twtxt/internal"
)

var (
	bind    string
	debug   bool
	version bool

	// Basic options
	name    string
	data    string
	store   string
	theme   string
	baseURL string

	// Pod Oeprator
	adminUser  string
	adminName  string
	adminEmail string

	// Pod Settings
	openProfiles      bool
	openRegistrations bool

	// Pod Limits
	twtsPerPage   int
	maxTwtLength  int
	maxUploadSize int64
	maxFetchLimit int64
	maxCacheTTL   time.Duration

	// Pod Secrets
	apiSigningKey   string
	cookieSecret    string
	magiclinkSecret string

	// Email Setitngs
	smtpHost string
	smtpPort int
	smtpUser string
	smtpPass string
	smtpFrom string

	// Timeouts
	sessionExpiry  time.Duration
	apiSessionTime time.Duration

	// Whitelists, Sources
	feedSources        []string
	whitelistedDomains []string
)

func init() {
	flag.BoolVarP(&debug, "debug", "D", false, "enable debug logging")
	flag.StringVarP(&bind, "bind", "b", "0.0.0.0:8000", "[int]:<port> to bind to")
	flag.BoolVarP(&version, "version", "v", false, "display version information")

	// Basic options
	flag.StringVarP(&name, "name", "n", internal.DefaultName, "set the pod's name")
	flag.StringVarP(&data, "data", "d", internal.DefaultData, "data directory")
	flag.StringVarP(&store, "store", "s", internal.DefaultStore, "store to use")
	flag.StringVarP(&theme, "theme", "t", internal.DefaultTheme, "set the default theme")
	flag.StringVarP(&baseURL, "base-url", "u", internal.DefaultBaseURL, "base url to use")

	// Pod Oeprator
	flag.StringVarP(&adminName, "admin-name", "N", internal.DefaultAdminName, "default admin user name")
	flag.StringVarP(&adminEmail, "admin-email", "E", internal.DefaultAdminEmail, "default admin user email")
	flag.StringVarP(&adminUser, "admin-user", "A", internal.DefaultAdminUser, "default admin user to use")

	// Pod Settings
	flag.BoolVarP(
		&openRegistrations, "open-registrations", "R", internal.DefaultOpenRegistrations,
		"whether or not to have open user registgration",
	)
	flag.BoolVarP(
		&openProfiles, "open-profiles", "O", internal.DefaultOpenProfiles,
		"whether or not to have open user profiles",
	)

	// Pod Limits
	flag.IntVarP(
		&twtsPerPage, "twts-per-page", "T", internal.DefaultTwtsPerPage,
		"maximum twts per page to display",
	)
	flag.IntVarP(
		&maxTwtLength, "max-twt-length", "L", internal.DefaultMaxTwtLength,
		"maximum length of posts",
	)
	flag.Int64VarP(
		&maxUploadSize, "max-upload-size", "U", internal.DefaultMaxUploadSize,
		"maximum upload size of media",
	)
	flag.Int64VarP(
		&maxFetchLimit, "max-fetch-limit", "F", internal.DefaultMaxFetchLimit,
		"maximum feed fetch limit in bytes",
	)
	flag.DurationVarP(
		&maxCacheTTL, "max-cache-ttl", "C", internal.DefaultMaxCacheTTL,
		"maximum cache ttl (time-to-live) of cached twts in memory",
	)

	// Pod Secrets
	flag.StringVar(
		&apiSigningKey, "api-signing-key", internal.DefaultAPISigningKey,
		"secret to use for signing api tokens",
	)
	flag.StringVar(
		&cookieSecret, "cookie-secret", internal.DefaultCookieSecret,
		"cookie secret to use secure sessions",
	)
	flag.StringVar(
		&magiclinkSecret, "magiclink-secret", internal.DefaultMagicLinkSecret,
		"magiclink secret to use for password reset tokens",
	)

	// Email Setitngs
	flag.StringVar(&smtpHost, "smtp-host", internal.DefaultSMTPHost, "SMTP Host to use for email sending")
	flag.IntVar(&smtpPort, "smtp-port", internal.DefaultSMTPPort, "SMTP Port to use for email sending")
	flag.StringVar(&smtpUser, "smtp-user", internal.DefaultSMTPUser, "SMTP User to use for email sending")
	flag.StringVar(&smtpPass, "smtp-pass", internal.DefaultSMTPPass, "SMTP Pass to use for email sending")
	flag.StringVar(&smtpFrom, "smtp-from", internal.DefaultSMTPFrom, "SMTP From to use for email sending")

	// Timeouts
	flag.DurationVar(
		&sessionExpiry, "session-expiry", internal.DefaultSessionExpiry,
		"timeout for sessions to expire",
	)
	flag.DurationVar(
		&apiSessionTime, "api-session-time", internal.DefaultAPISessionTime,
		"timeout for api tokens to expire",
	)

	// Whitelists, Sources
	flag.StringSliceVar(
		&feedSources, "feed-sources", internal.DefaultFeedSources,
		"external feed sources for discovery of other feeds",
	)
	flag.StringSliceVar(
		&whitelistedDomains, "whitelist-domain", internal.DefaultWhitelistedDomains,
		"whitelist of external domains to permit for display of inline images",
	)
}

func flagNameFromEnvironmentName(s string) string {
	s = strings.ToLower(s)
	s = strings.Replace(s, "_", "-", -1)
	return s
}

func parseArgs() error {
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

func extraServiceInfoFactory(svr *internal.Server) profiler.ExtraServiceInfoRetriever {
	return func() map[string]interface{} {
		extraInfo := make(map[string]interface{})

		expvar.Get("stats").(*expvar.Map).Do(func(kv expvar.KeyValue) {
			extraInfo[kv.Key] = kv.Value.String()
		})

		return extraInfo
	}
}

func main() {
	parseArgs()

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
		// Basic options
		internal.WithName(name),
		internal.WithData(data),
		internal.WithStore(store),
		internal.WithTheme(theme),
		internal.WithBaseURL(baseURL),

		// Pod Oeprator
		internal.WithAdminUser(adminUser),
		internal.WithAdminName(adminName),
		internal.WithAdminEmail(adminEmail),

		// Pod Settings
		internal.WithOpenProfiles(openProfiles),
		internal.WithOpenRegistrations(openRegistrations),

		// Pod Limits
		internal.WithTwtsPerPage(twtsPerPage),
		internal.WithMaxTwtLength(maxTwtLength),
		internal.WithMaxUploadSize(maxUploadSize),
		internal.WithMaxFetchLimit(maxFetchLimit),
		internal.WithMaxCacheTTL(maxCacheTTL),

		// Pod Secrets
		internal.WithAPISigningKey(apiSigningKey),
		internal.WithCookieSecret(cookieSecret),
		internal.WithMagicLinkSecret(magiclinkSecret),

		// Email Setitngs
		internal.WithSMTPHost(smtpHost),
		internal.WithSMTPPort(smtpPort),
		internal.WithSMTPUser(smtpUser),
		internal.WithSMTPPass(smtpPass),
		internal.WithSMTPFrom(smtpFrom),

		// Timeouts
		internal.WithSessionExpiry(sessionExpiry),
		internal.WithAPISessionTime(apiSessionTime),

		// Whitelists, Sources
		internal.WithFeedSources(feedSources),
		internal.WithWhitelistedDomains(whitelistedDomains),
	)
	if err != nil {
		log.WithError(err).Fatal("error creating server")
	}

	if debug {
		log.Info("starting memory profiler (debug mode) ...")

		go func() {
			// add the profiler handler endpoints
			profiler.AddMemoryProfilingHandlers()

			// add realtime extra key/value diagnostic info (optional)
			profiler.RegisterExtraServiceInfoRetriever(extraServiceInfoFactory(svr))

			// start the profiler on service start (optional)
			profiler.StartProfiling()

			// listen on port 6060 (pick a port)
			http.ListenAndServe(":6060", nil)
		}()
	}

	log.Infof("%s v%s listening on http://%s", path.Base(os.Args[0]), twtxt.FullVersion(), bind)
	if err := svr.Run(); err != nil {
		log.WithError(err).Fatal("error running or shutting down server")
	}
}
