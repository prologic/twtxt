package internal

import (
	"net/url"
	"regexp"
	"time"
)

const (
	// InvalidConfigValue is the constant value for invalid config values
	// which must be changed for production configurations before successful
	// startup
	InvalidConfigValue = "INVALID CONFIG VALUE - PLEASE CHANGE THIS VALUE"

	// DefaultDebug is the default debug mode
	DefaultDebug = false

	// DefaultParser is the default Twt parser used by the backend
	DefaultParser = "lextwt"

	// DefaultData is the default data directory for storage
	DefaultData = "./data"

	// DefaultStore is the default data store used for accounts, sessions, etc
	DefaultStore = "bitcask://twtxt.db"

	// DefaultBaseURL is the default Base URL for the app used to construct feed URLs
	DefaultBaseURL = "http://0.0.0.0:8000"

	// DefaultAdminXXX is the default admin user / pod operator
	DefaultAdminUser  = "admin"
	DefaultAdminName  = "Administrator"
	DefaultAdminEmail = "support@twt.social"

	// DefaultName is the default instance name
	DefaultName = "twtxt.net"

	// DefaultLogo is the default logo (SVG)
	// DefaultLogo = `<svg aria-hidden="true" focusable="false" role="img" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1000 1000" height="56px"><path fill="currentColor" d="M633.43 429.23c0 118.38-49.76 184.72-138.87 184.72-53 0-92.04-25.37-108.62-67.32h-2.6v203.12H250V249.7h133.67v64.72h2.28c17.24-43.9 55.3-69.92 107-69.92 90.4 0 140.48 66.02 140.48 184.73zm-136.6 0c0-49.76-22.1-81.96-56.9-81.96s-56.9 32.2-57.24 82.28c.33 50.4 22.1 81.63 57.24 81.63 35.12 0 56.9-31.87 56.9-81.95zM682.5 547.5c0-37.32 30.18-67.5 67.5-67.5s67.5 30.18 67.5 67.5S787.32 615 750 615s-67.5-30.18-67.5-67.5z"></path></svg>`
	DefaultLogo = `<svg aria-hidden="true" focusable="false" role="img" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 449 249.1" style="enable-background:new 0 0 449 249.1;" height="3.5rem"><path fill="currentColor" d="m73.1 156.6v-93.3h-48.4v-22.2h122.8v22.2h-48.5v93.3z"></path><path fill="currentColor" d="m320.7 64.8v-25.9h-25.7v25.9h-4.8l-9.3 20.5h14.1v71.3h25.7v-71.3h29v-20.5z"></path><path fill="currentColor" d="m277.2 64.8h-17l-27.8 61.8-24.9-54.9c-1.5-3.3-3.2-5.7-5.2-7.2s-4.9-2.3-8.5-2.3c-3.7 0-6.6.8-8.6 2.5-2 1.6-3.8 4-5.3 7l-26.2 54.9-22.6-53h-25.5l32.9 76.3c1.2 2.8 2.8 5.1 4.8 6.7s4.9 2.5 8.7 2.5c3.4 0 6.2-.8 8.6-2.5 2.3-1.6 4.1-3.9 5.5-6.7l27.3-56.6 25.9 56.6c1.2 2.6 3 4.8 5.3 6.5 2.3 1.8 5.1 2.6 8.4 2.6 3.4 0 6.2-.9 8.6-2.6 2.3-1.7 4.1-3.9 5.3-6.5l29.1-64.6 9.2-20.5z"></path><path fill="currentColor" d="m128.5 166.1v9.5h-1c-.8-3-1.8-5-3-6.1s-2.8-1.6-4.6-1.6c-1.4 0-2.6.4-3.4 1.1-.9.8-1.3 1.6-1.3 2.5 0 1.1.3 2.1 1 2.9.6.8 1.9 1.7 3.9 2.7l4.4 2.2c4.1 2 6.2 4.6 6.2 8 0 2.5-1 4.6-2.9 6.1-1.9 1.6-4.1 2.3-6.5 2.3-1.7 0-3.6-.3-5.8-.9-.7-.2-1.2-.3-1.7-.3s-.8.3-1.1.8h-1v-10h1c.6 2.9 1.7 5 3.3 6.4s3.4 2.2 5.4 2.2c1.4 0 2.5-.4 3.4-1.2s1.3-1.8 1.3-3c0-1.4-.5-2.6-1.5-3.5s-3-2.2-5.9-3.6c-2.9-1.5-4.9-2.8-5.8-4s-1.4-2.6-1.4-4.4c0-2.3.8-4.2 2.4-5.8 1.6-1.5 3.6-2.3 6.1-2.3 1.1 0 2.5.2 4 .7 1 .3 1.7.5 2 .5s.6-.1.8-.2.4-.4.7-.9h1z"></path><path fill="currentColor" d="m152.3 166.1c4.2 0 7.6 1.6 10.2 4.8 2.2 2.8 3.3 5.9 3.3 9.5 0 2.5-.6 5-1.8 7.6s-2.8 4.5-4.9 5.8-4.4 2-7.1 2c-4.2 0-7.5-1.7-10-5-2.1-2.8-3.1-6-3.1-9.5 0-2.6.6-5.1 1.9-7.7s2.9-4.4 5-5.6c2-1.3 4.2-1.9 6.5-1.9zm-.9 2c-1.1 0-2.1.3-3.2 1-1.1.6-2 1.8-2.7 3.4s-1 3.7-1 6.2c0 4.1.8 7.6 2.4 10.5s3.8 4.4 6.4 4.4c2 0 3.6-.8 4.9-2.4s1.9-4.4 1.9-8.4c0-5-1.1-8.9-3.2-11.8-1.4-1.9-3.3-2.9-5.5-2.9z"></path><path fill="currentColor" d="m197.5 184.3c-.8 3.7-2.2 6.5-4.4 8.5s-4.6 3-7.3 3c-3.2 0-6-1.3-8.3-4-2.4-2.7-3.5-6.3-3.5-10.8 0-4.4 1.3-8 3.9-10.7s5.8-4.1 9.4-4.1c2.8 0 5 .7 6.8 2.2s2.7 3 2.7 4.5c0 .8-.3 1.4-.8 1.9s-1.2.7-2.1.7c-1.2 0-2.1-.4-2.7-1.1-.3-.4-.5-1.2-.7-2.4-.1-1.2-.5-2.1-1.2-2.7s-1.7-.9-3-.9c-2 0-3.7.7-4.9 2.2-1.6 2-2.5 4.6-2.5 7.9s.8 6.3 2.4 8.8c1.6 2.6 3.8 3.8 6.7 3.8 2 0 3.8-.7 5.3-2 1.1-.9 2.2-2.6 3.3-5.1z"></path><path fill="currentColor" d="m215.1 166.1v22.4c0 1.8.1 2.9.4 3.5s.6 1 1.1 1.3 1.4.4 2.7.4v1.1h-13.6v-1.1c1.4 0 2.3-.1 2.7-.4.5-.3.8-.7 1.1-1.3s.4-1.8.4-3.5v-10.8c0-3-.1-5-.3-5.9-.2-.7-.4-1.1-.7-1.4s-.7-.4-1.3-.4-1.2.2-2 .5l-.5-1.1 8.4-3.4h1.6zm-2.6-14.6c.9 0 1.6.3 2.2.9s.9 1.3.9 2.2c0 .8-.3 1.6-.9 2.2s-1.3.9-2.2.9-1.6-.3-2.2-.9-.9-1.3-.9-2.2.3-1.6.9-2.2 1.3-.9 2.2-.9z"></path><path fill="currentColor" d="m242.5 190.9c-2.9 2.2-4.7 3.5-5.4 3.8-1.1.5-2.3.8-3.5.8-1.9 0-3.5-.7-4.8-2s-1.9-3.1-1.9-5.2c0-1.4.3-2.5.9-3.5.8-1.4 2.3-2.7 4.3-3.9 2.1-1.2 5.5-2.7 10.3-4.5v-1.1c0-2.8-.4-4.7-1.3-5.7s-2.2-1.6-3.9-1.6c-1.3 0-2.3.3-3 1-.8.7-1.2 1.5-1.2 2.4l.1 1.8c0 .9-.2 1.7-.7 2.2s-1.1.8-1.9.8-1.4-.3-1.9-.8-.7-1.3-.7-2.2c0-1.7.9-3.3 2.7-4.8s4.3-2.2 7.5-2.2c2.5 0 4.5.4 6.1 1.3 1.2.6 2.1 1.6 2.7 3 .4.9.5 2.7.5 5.4v9.5c0 2.7.1 4.3.2 4.9s.3 1 .5 1.2.5.3.8.3.6-.1.8-.2c.4-.3 1.3-1 2.5-2.2v1.7c-2.3 3-4.5 4.5-6.6 4.5-1 0-1.8-.3-2.4-1-.4-.9-.7-2-.7-3.7zm0-2v-10.7c-3.1 1.2-5 2.1-6 2.6-1.6.9-2.7 1.8-3.4 2.8s-1 2-1 3.2c0 1.5.4 2.7 1.3 3.7s1.9 1.5 3 1.5c1.5-.1 3.6-1.1 6.1-3.1z"></path><path fill="currentColor" d="m267.9 151.5v37.1c0 1.8.1 2.9.4 3.5s.6 1 1.2 1.3c.5.3 1.5.4 3 .4v1.1h-13.7v-1.1c1.3 0 2.2-.1 2.6-.4.5-.3.8-.7 1.1-1.3s.4-1.8.4-3.5v-25.4c0-3.2-.1-5.1-.2-5.8s-.4-1.2-.7-1.5-.7-.4-1.2-.4-1.2.2-2 .5l-.5-1.1 8.3-3.4z"></path><path fill="currentColor" d="m102.6 186.7c1.3 0 2.4.4 3.2 1.3.9.9 1.3 1.9 1.3 3.2 0 1.2-.4 2.3-1.3 3.2s-1.9 1.3-3.2 1.3c-1.2 0-2.3-.4-3.2-1.3s-1.3-2-1.3-3.2c0-1.3.4-2.3 1.3-3.2.9-.8 2-1.3 3.2-1.3z"></path></svg>`

	// DefaultMetaxxx are the default set of <meta> tags used on non-specific views
	DefaultMetaTitle       = ""
	DefaultMetaAuthor      = "twtxt.net / twt.social"
	DefaultMetaKeywords    = "twtxt, twt, blog, micro-blogging, social, media, decentralised, pod"
	DefaultMetaDescription = "📕 twtxt is a Self-Hosted, Twitter™-like Decentralised microBlogging platform. No ads, no tracking, your content, your data!"

	// DefaultTheme is the default theme to use ('light' or 'dark')
	DefaultTheme = "auto"

	// DefaultLang is the default language to use ('en' or 'zh-CN')
	DefaultLang = "auto"

	// DefaultOpenRegistrations is the default for open user registrations
	DefaultOpenRegistrations = false

	// DefaultRegisterMessage is the default message displayed when  registrations are disabled
	DefaultRegisterMessage = ""

	// DefaultCookieSecret is the server's default cookie secret
	DefaultCookieSecret = InvalidConfigValue

	// DefaultTwtsPerPage is the server's default twts per page to display
	DefaultTwtsPerPage = 50

	// DefaultMaxTwtLength is the default maximum length of posts permitted
	DefaultMaxTwtLength = 288

	// DefaultMaxCacheTTL is the default maximum cache ttl of twts in memory
	DefaultMaxCacheTTL = time.Hour * 24 * 10 // 10 days 28 days 28 days 28 days

	// DefaultMaxCacheItems is the default maximum cache items (per feed source)
	// of twts in memory
	DefaultMaxCacheItems = DefaultTwtsPerPage * 3 // We get bored after paging thorughh > 3 pages :D

	// DefaultMsgPerPage is the server's default msgs per page to display
	DefaultMsgsPerPage = 20

	// DefaultOpenProfiles is the default for whether or not to have open user profiles
	DefaultOpenProfiles = false

	// DefaultMaxUploadSize is the default maximum upload size permitted
	DefaultMaxUploadSize = 1 << 24 // ~16MB (enough for high-res photos)

	// DefaultSessionCacheTTL is the server's default session cache ttl
	DefaultSessionCacheTTL = 1 * time.Hour

	// DefaultSessionExpiry is the server's default session expiry time
	DefaultSessionExpiry = 240 * time.Hour // 10 days

	// DefaultTranscoderTimeout is the default vodeo transcoding timeout
	DefaultTranscoderTimeout = 10 * time.Minute // 10mins

	// DefaultMagicLinkSecret is the jwt magic link secret
	DefaultMagicLinkSecret = InvalidConfigValue

	// Default Messaging settings
	DefaultSMTPBind = "0.0.0.0:8025"
	DefaultPOP3Bind = "0.0.0.0:8110"

	// Default SMTP configuration
	DefaultSMTPHost = "smtp.gmail.com"
	DefaultSMTPPort = 587
	DefaultSMTPUser = InvalidConfigValue
	DefaultSMTPPass = InvalidConfigValue
	DefaultSMTPFrom = InvalidConfigValue

	// DefaultMaxFetchLimit is the maximum fetch fetch limit in bytes
	DefaultMaxFetchLimit = 1 << 21 // ~2MB (or more than enough for a year)

	// DefaultAPISessionTime is the server's default session time for API tokens
	DefaultAPISessionTime = 240 * time.Hour // 10 days

	// DefaultAPISigningKey is the default API JWT signing key for tokens
	DefaultAPISigningKey = InvalidConfigValue
)

var (
	// DefaultFeedSources is the default list of external feed sources
	DefaultFeedSources = []string{
		"https://feeds.twtxt.net/we-are-feeds.txt",
		"https://raw.githubusercontent.com/jointwt/we-are-twtxt/master/we-are-bots.txt",
		"https://raw.githubusercontent.com/jointwt/we-are-twtxt/master/we-are-twtxt.txt",
	}

	// DefaultTwtPrompts are the set of default prompts  for twt text(s)
	DefaultTwtPrompts = `What's on your mind?\nShare something insightful!\nGood day to you! What's new?\nDid something cool lately? Share it!\nHi! 👋 Don't forget to post a Twt today!`

	// DefaultWhitelistedDomains is the default list of domains to whitelist for external images
	DefaultWhitelistedDomains = []string{
		`imgur\.com`,
		`giphy\.com`,
		`imgs\.xkcd\.com`,
		`tube\.mills\.io`,
		`reactiongifs\.com`,
		`githubusercontent\.com`,
	}
)

func NewConfig() *Config {
	return &Config{
		Debug: DefaultDebug,

		Name:              DefaultName,
		Logo:              DefaultLogo,
		Description:       DefaultMetaDescription,
		Store:             DefaultStore,
		Theme:             DefaultTheme,
		BaseURL:           DefaultBaseURL,
		AdminUser:         DefaultAdminUser,
		FeedSources:       DefaultFeedSources,
		RegisterMessage:   DefaultRegisterMessage,
		CookieSecret:      DefaultCookieSecret,
		TwtPrompts:        DefaultTwtPrompts,
		TwtsPerPage:       DefaultTwtsPerPage,
		MaxTwtLength:      DefaultMaxTwtLength,
		MsgsPerPage:       DefaultMsgsPerPage,
		OpenProfiles:      DefaultOpenProfiles,
		OpenRegistrations: DefaultOpenRegistrations,
		SessionExpiry:     DefaultSessionExpiry,
		MagicLinkSecret:   DefaultMagicLinkSecret,
		SMTPHost:          DefaultSMTPHost,
		SMTPPort:          DefaultSMTPPort,
		SMTPUser:          DefaultSMTPUser,
		SMTPPass:          DefaultSMTPPass,
	}
}

// Option is a function that takes a config struct and modifies it
type Option func(*Config) error

// WithDebug sets the debug mode lfag
func WithDebug(debug bool) Option {
	return func(cfg *Config) error {
		cfg.Debug = debug
		return nil
	}
}

// WithParser sets the parser used by the backend
func WithParser(parser string) Option {
	return func(cfg *Config) error {
		cfg.Parser = parser
		return nil
	}
}

// WithData sets the data directory to use for storage
func WithData(data string) Option {
	return func(cfg *Config) error {
		cfg.Data = data
		return nil
	}
}

// WithStore sets the store to use for accounts, sessions, etc.
func WithStore(store string) Option {
	return func(cfg *Config) error {
		cfg.Store = store
		return nil
	}
}

// WithBaseURL sets the Base URL used for constructing feed URLs
func WithBaseURL(baseURL string) Option {
	return func(cfg *Config) error {
		u, err := url.Parse(baseURL)
		if err != nil {
			return err
		}
		cfg.BaseURL = baseURL
		cfg.baseURL = u
		return nil
	}
}

// WithAdminUser sets the Admin user used for granting special features to
func WithAdminUser(adminUser string) Option {
	return func(cfg *Config) error {
		cfg.AdminUser = adminUser
		return nil
	}
}

// WithAdminName sets the Admin name used to identify the pod operator
func WithAdminName(adminName string) Option {
	return func(cfg *Config) error {
		cfg.AdminName = adminName
		return nil
	}
}

// WithAdminEmail sets the Admin email used to contact the pod operator
func WithAdminEmail(adminEmail string) Option {
	return func(cfg *Config) error {
		cfg.AdminEmail = adminEmail
		return nil
	}
}

// WithFeedSources sets the feed sources  to use for external feeds
func WithFeedSources(feedSources []string) Option {
	return func(cfg *Config) error {
		cfg.FeedSources = feedSources
		return nil
	}
}

// WithName sets the instance's name
func WithName(name string) Option {
	return func(cfg *Config) error {
		cfg.Name = name
		return nil
	}
}

// WithDescription sets the instance's description
func WithDescription(description string) Option {
	return func(cfg *Config) error {
		cfg.Description = description
		return nil
	}
}

// WithTheme sets the default theme to use
func WithTheme(theme string) Option {
	return func(cfg *Config) error {
		cfg.Theme = theme
		return nil
	}
}

// WithOpenRegistrations sets the open registrations flag
func WithOpenRegistrations(openRegistrations bool) Option {
	return func(cfg *Config) error {
		cfg.OpenRegistrations = openRegistrations
		return nil
	}
}

// WithCookieSecret sets the server's cookie secret
func WithCookieSecret(secret string) Option {
	return func(cfg *Config) error {
		cfg.CookieSecret = secret
		return nil
	}
}

// WithTwtsPerPage sets the server's twts per page
func WithTwtsPerPage(twtsPerPage int) Option {
	return func(cfg *Config) error {
		cfg.TwtsPerPage = twtsPerPage
		return nil
	}
}

// WithMaxTwtLength sets the maximum length of posts permitted on the server
func WithMaxTwtLength(maxTwtLength int) Option {
	return func(cfg *Config) error {
		cfg.MaxTwtLength = maxTwtLength
		return nil
	}
}

// WithMaxCacheTTL sets the maximum cache ttl of twts in memory
func WithMaxCacheTTL(maxCacheTTL time.Duration) Option {
	return func(cfg *Config) error {
		cfg.MaxCacheTTL = maxCacheTTL
		return nil
	}
}

// WithMaxCacheItems sets the maximum cache items (per feed source) of twts in memory
func WithMaxCacheItems(maxCacheItems int) Option {
	return func(cfg *Config) error {
		cfg.MaxCacheItems = maxCacheItems
		return nil
	}
}

// WithOpenProfiles sets whether or not to have open user profiles
func WithOpenProfiles(openProfiles bool) Option {
	return func(cfg *Config) error {
		cfg.OpenProfiles = openProfiles
		return nil
	}
}

// WithMaxUploadSize sets the maximum upload size permitted by the server
func WithMaxUploadSize(maxUploadSize int64) Option {
	return func(cfg *Config) error {
		cfg.MaxUploadSize = maxUploadSize
		return nil
	}
}

// WithSessionCacheTTL sets the server's session cache ttl
func WithSessionCacheTTL(cacheTTL time.Duration) Option {
	return func(cfg *Config) error {
		cfg.SessionCacheTTL = cacheTTL
		return nil
	}
}

// WithSessionExpiry sets the server's session expiry time
func WithSessionExpiry(expiry time.Duration) Option {
	return func(cfg *Config) error {
		cfg.SessionExpiry = expiry
		return nil
	}
}

// WithTranscoderTimeout sets the video transcoding timeout
func WithTranscoderTimeout(timeout time.Duration) Option {
	return func(cfg *Config) error {
		cfg.TranscoderTimeout = timeout
		return nil
	}
}

// WithMagicLinkSecret sets the MagicLinkSecert used to create password reset tokens
func WithMagicLinkSecret(secret string) Option {
	return func(cfg *Config) error {
		cfg.MagicLinkSecret = secret
		return nil
	}
}

// WithSMTPBind sets the interface and port to bind to for SMTP
func WithSMTPBind(smtpBind string) Option {
	return func(cfg *Config) error {
		cfg.SMTPBind = smtpBind
		return nil
	}
}

// WithPOP3Bind sets the interface and port to use for POP3
func WithPOP3Bind(pop3Bind string) Option {
	return func(cfg *Config) error {
		cfg.POP3Bind = pop3Bind
		return nil
	}
}

// WithSMTPHost sets the SMTPHost to use for sending email
func WithSMTPHost(host string) Option {
	return func(cfg *Config) error {
		cfg.SMTPHost = host
		return nil
	}
}

// WithSMTPPort sets the SMTPPort to use for sending email
func WithSMTPPort(port int) Option {
	return func(cfg *Config) error {
		cfg.SMTPPort = port
		return nil
	}
}

// WithSMTPUser sets the SMTPUser to use for sending email
func WithSMTPUser(user string) Option {
	return func(cfg *Config) error {
		cfg.SMTPUser = user
		return nil
	}
}

// WithSMTPPass sets the SMTPPass to use for sending email
func WithSMTPPass(pass string) Option {
	return func(cfg *Config) error {
		cfg.SMTPPass = pass
		return nil
	}
}

// WithSMTPFrom sets the SMTPFrom address to use for sending email
func WithSMTPFrom(from string) Option {
	return func(cfg *Config) error {
		cfg.SMTPFrom = from
		return nil
	}
}

// WithMaxFetchLimit sets the maximum feed fetch limit in bytes
func WithMaxFetchLimit(limit int64) Option {
	return func(cfg *Config) error {
		cfg.MaxFetchLimit = limit
		return nil
	}
}

// WithAPISessionTime sets the API session time for tokens
func WithAPISessionTime(duration time.Duration) Option {
	return func(cfg *Config) error {
		cfg.APISessionTime = duration
		return nil
	}
}

// WithAPISigningKey sets the API JWT signing key for tokens
func WithAPISigningKey(key string) Option {
	return func(cfg *Config) error {
		cfg.APISigningKey = key
		return nil
	}
}

// WithWhitelistedDomains sets the list of domains whitelisted and permitted for external iamges
func WithWhitelistedDomains(whitelistedDomains []string) Option {
	return func(cfg *Config) error {
		for _, whitelistedDomain := range whitelistedDomains {
			re, err := regexp.Compile(whitelistedDomain)
			if err != nil {
				return err
			}
			cfg.whitelistedDomains = append(cfg.whitelistedDomains, re)
		}
		return nil
	}
}
