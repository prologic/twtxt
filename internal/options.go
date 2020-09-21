package internal

import (
	"net/url"
	"regexp"
	"time"
)

const (
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

	// DefaultMetaxxx are the default set of <meta> tags used on non-specific views
	DefaultMetaTitle       = ""
	DefaultMetaAuthor      = "twtxt.net / twt.social"
	DefaultMetaKeywords    = "twtxt, twt, blog, micro-blogging, social, media, decentralised, pod"
	DefaultMetaDescription = "📕 twtxt is a Self-Hosted, Twitter™-like Decentralised microBlogging platform. No ads, no tracking, your content, your data!"

	// DefaultTheme is the default theme to use ('light' or 'dark')
	DefaultTheme = "dark"

	// DefaultOpenRegistrations is the default for open user registrations
	DefaultOpenRegistrations = false

	// DefaultRegisterMessage is the default message displayed when  registrations are disabled
	DefaultRegisterMessage = ""

	// DefaultCookieSecret is the server's default cookie secret
	DefaultCookieSecret = "PLEASE_CHANGE_ME!!!"

	// DefaultTwtsPerPage is the server's default twts per page to display
	DefaultTwtsPerPage = 50

	// DefaultMaxTwtLength is the default maximum length of posts permitted
	DefaultMaxTwtLength = 288

	// DefaultMaxCacheTTL is the default maximum cache ttl of twts in memory
	DefaultMaxCacheTTL = time.Hour * 24 * 10 // 10 days 28 days 28 days 28 days

	// DefaultMaxCacheItems is the default maximum cache items (per feed source)
	// of twts in memory
	DefaultMaxCacheItems = DefaultTwtsPerPage * 3 // We get bored after paging thorughh > 3 pages :D

	// DefaultOpenProfiles is the default for whether or not to have open user profiles
	DefaultOpenProfiles = false

	// DefaultMaxUploadSize is the default maximum upload size permitted
	DefaultMaxUploadSize = 1 << 24 // ~16MB (enough for high-res photos)

	// DefaultSessionCacheTTL is the server's default session cache ttl
	DefaultSessionCacheTTL = 1 * time.Hour

	// DefaultSessionExpiry is the server's default session expiry time
	DefaultSessionExpiry = 240 * time.Hour // 10 days

	// DefaultTranscoderTimeout is the default vodeo transcoding timeout
	DefaultTranscoderTimeout = 5 * time.Minute // 5mins

	// DefaultMagicLinkSecret is the jwt magic link secret
	DefaultMagicLinkSecret = "PLEASE_CHANGE_ME!!!"

	// Default SMTP configuration
	DefaultSMTPHost = "smtp.gmail.com"
	DefaultSMTPPort = 587
	DefaultSMTPUser = "PLEASE_CHANGE_ME!!!"
	DefaultSMTPPass = "PLEASE_CHANGE_ME!!!"
	DefaultSMTPFrom = "PLEASE_CHANGE_ME!!!"

	// DefaultMaxFetchLimit is the maximum fetch fetch limit in bytes
	DefaultMaxFetchLimit = 1 << 21 // ~2MB (or more than enough for a year)

	// DefaultAPISessionTime is the server's default session time for API tokens
	DefaultAPISessionTime = 240 * time.Hour // 10 days

	// DefaultAPISigningKey is the default API JWT signing key for tokens
	DefaultAPISigningKey = "PLEASE_CHANGE_ME!!!"
)

var (
	// DefaultFeedSources is the default list of external feed sources
	DefaultFeedSources = []string{
		"https://feeds.twtxt.net/we-are-feeds.txt",
		"https://raw.githubusercontent.com/mdom/we-are-twtxt/master/we-are-bots.txt",
		"https://raw.githubusercontent.com/mdom/we-are-twtxt/master/we-are-twtxt.txt",
	}

	// DefaultTwtPrompts are the set of default prompts  for twt text(s)
	DefaultTwtPrompts = []string{
		`What's on your mind?`,
		`Share something insightful!`,
		`Good day to you! What's new?`,
		`Did something cool lately? Share it!`,
		`Hi! 👋 Don't forget to post a Twt today!`,
	}

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
		Name:              DefaultName,
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
