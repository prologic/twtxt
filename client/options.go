package client

import "os"

const (
	// DefaultURI is the default base URI to use for the Twtxt API endpoint
	DefaultURI = "http://localhost:8000/api/v1/"
)

// NewConfig ...
func NewConfig() *Config {
	return &Config{
		URI:   DefaultURI,
		Token: os.Getenv("TWT_TOKEN"),
	}
}

// Option is a function that takes a config struct and modifies it
type Option func(*Config) error

// WithURI sets the base URI to used for the Twtxt API endpoint
func WithURI(uri string) Option {
	return func(cfg *Config) error {
		cfg.URI = uri
		return nil
	}
}

// WithToken sets the API token to use for authenticating to Twtxt endpoints
func WithToken(token string) Option {
	return func(cfg *Config) error {
		cfg.Token = token
		return nil
	}
}
