package passwords

import (
	"time"

	scrypt "github.com/elithrar/simple-scrypt"
	log "github.com/sirupsen/logrus"
)

const (
	// DefaultMaxTimeout default max timeout in ms
	DefaultMaxTimeout = 500 * time.Millisecond

	// DefaultMaxMemory default max memory in MB
	DefaultMaxMemory = 64
)

// Options ...
type Options struct {
	maxTimeout time.Duration
	maxMemory  int
}

// NewOptions ...
func NewOptions(maxTimeout time.Duration, maxMemory int) *Options {
	return &Options{maxTimeout, maxMemory}
}

// ScryptPasswords ...
type ScryptPasswords struct {
	options *Options
	params  scrypt.Params
}

// NewScryptPasswords ...
func NewScryptPasswords(options *Options) Passwords {
	if options == nil {
		options = &Options{}
	}

	if options.maxTimeout == 0 {
		options.maxTimeout = DefaultMaxTimeout
	}
	if options.maxMemory == 0 {
		options.maxMemory = DefaultMaxMemory
	}

	log.Info("Calibrating scrypt parameters ...")
	params, err := scrypt.Calibrate(
		options.maxTimeout,
		options.maxMemory,
		scrypt.DefaultParams,
	)
	if err != nil {
		log.Fatalf("error calibrating scrypt params: %s", err)
	}

	log.WithField("params", params).Info("scrypt params")

	return &ScryptPasswords{options, params}
}

// CreatePassword ...
func (sp *ScryptPasswords) CreatePassword(password string) (string, error) {
	hash, err := scrypt.GenerateFromPassword([]byte(password), sp.params)
	return string(hash), err
}

// CheckPassword ...
func (sp *ScryptPasswords) CheckPassword(hash, password string) error {
	return scrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
