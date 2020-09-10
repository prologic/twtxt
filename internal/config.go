package internal

import (
	"errors"
	"io"
	"io/ioutil"
	"math/rand"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/apex/log"
	"github.com/goccy/go-yaml"
)

var (
	ErrConfigPathMissing = errors.New("error: config file missing")
)

// Config contains the server configuration parameters
type Config struct {
	Data              string        `yaml:"data"`
	Name              string        `yaml:"name"`
	Description       string        `yaml:"description"`
	Store             string        `yaml:"store"`
	Theme             string        `yaml:"theme"`
	BaseURL           string        `yaml:"base_url"`
	AdminUser         string        `yaml:"admin_user"`
	AdminName         string        `yaml:"admin_name"`
	AdminEmail        string        `yaml:"admin_email"`
	FeedSources       []string      `yaml:"feed_sources"`
	RegisterMessage   string        `yaml:"register_message"`
	CookieSecret      string        `yaml:"cookie_secret"`
	TwtPrompts        []string      `yaml:"twt_prompts"`
	TwtsPerPage       int           `yaml:"twts_per_page"`
	MaxUploadSize     int64         `yaml:"max_upload_size"`
	MaxTwtLength      int           `yaml:"max_twt_length"`
	MaxCacheTTL       time.Duration `yaml:"max_cache_ttl"`
	MaxCacheItems     int           `yaml:"max_cache_items"`
	OpenProfiles      bool          `yaml:"open_profiles"`
	OpenRegistrations bool          `yaml:"open_registrations"`
	SessionExpiry     time.Duration `yaml:"session_expiry"`
	SessionCacheTTL   time.Duration `yaml:"session_cache_ttl"`

	MagicLinkSecret string `json:"magiclink_secret"`

	SMTPHost string `yaml:"smtp_host"`
	SMTPPort int    `yaml:"smtp_port"`
	SMTPUser string `yaml:"smtp_user"`
	SMTPPass string `yaml:"smtp_pass"`
	SMTPFrom string `yaml:"smtp_from"`

	MaxFetchLimit int64 `yaml:"max_fetch_limit"`

	APISessionTime time.Duration `yaml:"api_session_time"`
	APISigningKey  []byte        `yaml:"api_signing_key"`

	baseURL *url.URL

	whitelistedDomains []*regexp.Regexp
	WhitelistedDomains []string `yaml:"whitelisted_domains"`

	path string
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

// ConfigFromReader reads an io.Reader `r` and pares it into a *Config object
func ConfigFromReader(r io.Reader) (cfg *Config, err error) {
	var data []byte

	data, err = ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Load loads a configuration from the given path
func Load(path string) (*Config, error) {
	var cfg Config

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	cfg.path = path

	return &cfg, nil
}

func (c *Config) String() string {
	data, err := yaml.MarshalWithOptions(c, yaml.Indent(4))
	if err != nil {
		log.WithError(err).Warn("error marshalling config")
		return ""
	}
	return string(data)
}

// Save saves the configuration to the provided path
func (c *Config) Save(path string) error {
	if path == "" {
		path = c.path
	}
	if path == "" {
		return ErrConfigPathMissing
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	data, err := yaml.MarshalWithOptions(c, yaml.Indent(4))
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
