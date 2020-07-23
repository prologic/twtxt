package password

import (
	"time"

	scrypt "github.com/elithrar/simple-scrypt"
	log "github.com/sirupsen/logrus"
)

const (
	DefaultMaxTimeout = 500 * time.Millisecond // default max timeout in ms
	DefaultMaxMemory  = 64                     // default max memory in MB
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

// Manager ...
type Manager struct {
	options *Options
	params  scrypt.Params
}

// NewManager ...
func NewManager(options *Options) *Manager {
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

	return &Manager{
		options,
		params,
	}
}

// NewPassword ...
func (m *Manager) NewPassword(password string) (string, error) {
	hash, err := scrypt.GenerateFromPassword([]byte(password), m.params)
	return string(hash), err
}

// Check ...
func (m *Manager) Check(hash, password string) error {
	return scrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
