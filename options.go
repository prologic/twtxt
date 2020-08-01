package twtxt

import "time"

const (
	// DefaultData is the default data directory for storage
	DefaultData = "./data"

	// DefaultStore is the default data store used for accounts, sessions, etc
	DefaultStore = "bitcask://twtxt.db"

	// DefaultBaseURL is the default Base URL for the app used to construct feed URLs
	DefaultBaseURL = "http://0.0.0.0:8000"

	// DefaultAdminUser is the default admin user who has special features
	DefaultAdminUser = "admin"

	// DefaultName is the default instance name
	DefaultName = "twtxt.net"

	// DefaultTheme is the default theme to use ('light' or 'dark')
	DefaultTheme = "dark"

	// DefaultRegister is the default user registration flag
	DefaultRegister = false

	// DefaultRegisterMessage is the default message displayed when  registrations are disabled
	DefaultRegisterMessage = ""

	// DefaultCookieSecret is the server's default cookie secret
	DefaultCookieSecret = "PLEASE_CHANGE_ME!!!"

	// DefaultTweetsPerPage is the server's default tweets per page to display
	DefaultTweetsPerPage = 50

	// DefaultMaxTweetLength is the default maximum length of posts permitted
	DefaultMaxTweetLength = 288

	// DefaultMaxUploadSize is the default maximum upload size permitted
	DefaultMaxUploadSize = 1 << 24 // ~16MB (enough for high-res photos)

	// DefaultSessionExpiry is the server's default session expiry time
	DefaultSessionExpiry = 240 * time.Hour // 10 days

	// DefaultMagicLinkSecret is the jwt magic link secret
	DefaultMagicLinkSecret = "PLEASE_CHANGE_ME!!!"

	// Default SMTP configuration
	DefaultSMTPHost = "smtp.gmail.com"
	DefaultSMTPPort = 587
	DefaultSMTPUser = "PLEASE_CHANGE_ME!!!"
	DefaultSMTPPass = "PLEASE_CHANGE_ME!!!"
	DefaultSMTPFrom = "PLEASE_CHANGE_ME!!!"

	// Default maximum fetch fetch limit in bytes
	DefaultMaxFetchLimit = 1 << 21 // ~2MB (or more than enough for a year)
)

var (
	// DefaultFeedSources is the default list of external feed sources
	DefaultFeedSources = []string{
		"https://feeds.twtxt.net/we-are-feeds.txt",
		"https://raw.githubusercontent.com/mdom/we-are-twtxt/master/we-are-bots.txt",
		"https://raw.githubusercontent.com/mdom/we-are-twtxt/master/we-are-twtxt.txt",
	}

	// DefaultTweetPrompts are the set of default prompts  for tweet text(s)
	DefaultTweetPrompts = []string{
		`What's on your mind?`,
		`Share something insightful!`,
		`Good day to you! What's new?`,
		`Did something cool lately? Share it!`,
		`Hi! 👋 Don't forget to post a Twt today!`,
	}
)

func NewConfig() *Config {
	return &Config{
		Name:            DefaultName,
		Store:           DefaultStore,
		Theme:           DefaultTheme,
		BaseURL:         DefaultBaseURL,
		AdminUser:       DefaultAdminUser,
		FeedSources:     DefaultFeedSources,
		Register:        DefaultRegister,
		RegisterMessage: DefaultRegisterMessage,
		CookieSecret:    DefaultCookieSecret,
		TweetPrompts:    DefaultTweetPrompts,
		TweetsPerPage:   DefaultTweetsPerPage,
		MaxTweetLength:  DefaultMaxTweetLength,
		SessionExpiry:   DefaultSessionExpiry,
		MagicLinkSecret: DefaultMagicLinkSecret,
		SMTPHost:        DefaultSMTPHost,
		SMTPPort:        DefaultSMTPPort,
		SMTPUser:        DefaultSMTPUser,
		SMTPPass:        DefaultSMTPPass,
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
		cfg.BaseURL = baseURL
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

// WithTheme sets the default theme to use
func WithTheme(theme string) Option {
	return func(cfg *Config) error {
		cfg.Theme = theme
		return nil
	}
}

// WithRegister sets the user registration flag
func WithRegister(register bool) Option {
	return func(cfg *Config) error {
		cfg.Register = register
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

// WithTweetsPerPage sets the server's tweets per page
func WithTweetsPerPage(tweetsPerPage int) Option {
	return func(cfg *Config) error {
		cfg.TweetsPerPage = tweetsPerPage
		return nil
	}
}

// WithMaxTweetLength sets the maximum length of posts permitted on the server
func WithMaxTweetLength(maxTweetLength int) Option {
	return func(cfg *Config) error {
		cfg.MaxTweetLength = maxTweetLength
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

// WithSessionExpiry sets the server's session expiry time
func WithSessionExpiry(expiry time.Duration) Option {
	return func(cfg *Config) error {
		cfg.SessionExpiry = expiry
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
