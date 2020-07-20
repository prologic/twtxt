package twtxt

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"time"
)

// Config contains the server configuration parameters
type Config struct {
	Data            string        `json:"data"`
	Name            string        `json:"name"`
	Store           string        `json:"store"`
	BaseURL         string        `json:"base_url"`
	Register        bool          `json:"register"`
	RegisterMessage string        `json:"register_message"`
	CookieSecret    string        `json:"cookie_secret"`
	TweetsPerPage   int           `json:"tweets_per_page"`
	SessionExpiry   time.Duration `json:"session_expiry"`
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
