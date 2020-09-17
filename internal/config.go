package internal

import (
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"time"
)

// Config contains the pod settings that are directly mutable by the pod owner
// through the /manage endpoint and interface as well as publicly accessible
type Config struct {
	PodName           string        `json:"pod_name"`
	DefaultTheme      string        `json:"default_theme"`
	BaseURL           string        `json:"base_url"`
	MaxUploadSize     int64         `json:"max_upload_size"`
	MaxTwtLength      int           `json:"max_twt_length"`
	MaxCacheTTL       time.Duration `json:"max_cache_ttl"`
	MaxCacheItems     int           `json:"max_cache_items"`
	OpenProfiles      bool          `json:"open_profiles"`
	OpenRegistrations bool          `json:"open_registrations"`
	TranscoderTimeout time.Duration `json:"transcoder_timeout"`
	MaxFetchLimit     int64         `json:"max_fetch_limit"`
}

type config struct {
	*Config

	data            string
	store           string
	baseURL         string
	adminUser       string
	adminName       string
	adminEmail      string
	feedSources     []string
	registerMessage string
	cookieSecret    string
	twtPrompts      []string
	twtsPerPage     int
	sessionExpiry   time.Duration
	sessionCacheTTL time.Duration

	magicLinkSecret string

	smtpHost string
	smtpPort int
	smtpUser string
	smtpPass string
	smtpFrom string

	apiSessionTime time.Duration
	apiSigningKey  []byte

	whitelistedDomains   []string
	whitelistedDomainsRe []*regexp.Regexp
}

// WhitelistedDomain returns true if the domain provided is a whiltelisted
// domain as per the configuration
func (c *config) WhitelistedDomain(domain string) (bool, bool) {
	// Always permit our own domain
	ourDomain := strings.TrimPrefix(strings.ToLower(c.baseURL.Hostname()), "www.")
	if domain == ourDomain {
		return true, true
	}

	// Check against list of whitelistedDomains (regexes)
	for _, re := range c.whitelistedDomainsRe {
		if re.MatchString(domain) {
			return true, false
		}
	}
	return false, false
}

// RandomTwtPrompt returns a random  Twt Prompt for display by the UI
func (c *config) RandomTwtPrompt() string {
	n := rand.Int() % len(c.twtPrompts)
	return c.twtPrompts[n]
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
