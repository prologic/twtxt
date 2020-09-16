package internal

import (
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// Config contains the server configuration parameters
type Config struct {
	Data              string        `json:"data"`
	Name              string        `json:"name"`
	Store             string        `json:"store"`
	Theme             string        `json:"theme"`
	BaseURL           string        `json:"base_url"`
	AdminUser         string        `json:"admin_user"`
	AdminName         string        `json:"admin_name"`
	AdminEmail        string        `json:"admin_email"`
	FeedSources       []string      `json:"feed_sources"`
	RegisterMessage   string        `json:"register_message"`
	CookieSecret      string        `json:"cookie_secret"`
	TwtPrompts        []string      `json:"twt_prompts"`
	TwtsPerPage       int           `json:"twts_per_page"`
	MaxUploadSize     int64         `json:"max_upload_size"`
	MaxTwtLength      int           `json:"max_twt_length"`
	MaxCacheTTL       time.Duration `json:"max_cache_ttl"`
	MaxCacheItems     int           `json:"max_cache_items"`
	OpenProfiles      bool          `json:"open_profiles"`
	OpenRegistrations bool          `json:"open_registrations"`
	SessionExpiry     time.Duration `json:"session_expiry"`
	SessionCacheTTL   time.Duration `json:"session_cache_ttl"`
	TranscoderTimeout time.Duration `json:"transcoder_timeout"`

	MagicLinkSecret string `json:"magiclink_secret"`

	SMTPHost string `json:"smtp_host"`
	SMTPPort int    `json:"smtp_port"`
	SMTPUser string `json:"smtp_user"`
	SMTPPass string `json:"smtp_pass"`
	SMTPFrom string `json:"smtp_from"`

	MaxFetchLimit int64 `json:"max_fetch_limit"`

	APISessionTime time.Duration `json:"api_session_time"`
	APISigningKey  []byte        `json:"api_signing_key"`

	baseURL *url.URL

	WhitelistedDomains []string `json:"whitelisted_domains"`
	whitelistedDomains []*regexp.Regexp
}

// WhitelistedDomain returns true if the domain provided is a whiltelisted
// domain as per the configuration
func (c *Config) WhitelistedDomain(domain string) (bool, bool) {
	// Always per mit our own domain
	ourDomain := strings.TrimPrefix(strings.ToLower(c.baseURL.Hostname()), "www.")
	if domain == ourDomain {
		return true, true
	}

	// Check against list of whitelistedDomains (regexes)
	for _, re := range c.whitelistedDomains {
		if re.MatchString(domain) {
			return true, false
		}
	}
	return false, false
}

// RandomTwtPrompt returns a random  Twt Prompt for display by the UI
func (c *Config) RandomTwtPrompt() string {
	n := rand.Int() % len(c.TwtPrompts)
	return c.TwtPrompts[n]
}

// Load loads a configuration from the given path
func Load(path string) (*Config, error) {
	var cfg Config

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Save saves the configuration to the provided path
func (c *Config) Save(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	data, err := json.Marshal(c)
	if err != nil {
		return err
	}

	if _, err = f.Write(data); err != nil {
		return err
	}

	if err = f.Sync(); err != nil {
		return err
	}

	return f.Close()
}
