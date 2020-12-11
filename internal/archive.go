package internal

import (
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/jointwt/twtxt/types"
)

const (
	archiveDir = "archive"
)

var (
	ErrTwtAlreadyArchived = errors.New("error: twt already archived")
	ErrTwtNotArchived     = errors.New("error: twt not found in archived")
	ErrInvalidTwtHash     = errors.New("error: invalid twt hash")
)

// Archiver is an interface for retrieving old twts from an archive storage
// such as an on-disk hash layout with one directory per 2-letter part of
// the hash sequence.
type Archiver interface {
	Del(hash string) error
	Has(hash string) bool
	Get(hash string) (types.Twt, error)
	Archive(twt types.Twt) error
	Count() (int, error)
}

// NullArchiver implements Archiver using dummy implementation stubs
type NullArchiver struct{}

func NewNullArchiver() (Archiver, error) {
	return &NullArchiver{}, nil
}

func (a *NullArchiver) Del(hash string) error              { return nil }
func (a *NullArchiver) Has(hash string) bool               { return false }
func (a *NullArchiver) Get(hash string) (types.Twt, error) { return types.Twt{}, nil }
func (a *NullArchiver) Archive(twt types.Twt) error        { return nil }
func (a *NullArchiver) Count() (int, error)                { return 0, nil }

// DiskArchiver implements Archiver using an on-disk hash layout directory
// structure with one directory per 2-letter hash sequence with a single
// JSON encoded file per twt.
type DiskArchiver struct {
	path string
}

func NewDiskArchiver(p string) (Archiver, error) {
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating archive directory")
		return nil, err
	}

	return &DiskArchiver{path: p}, nil
}

func (a *DiskArchiver) makePath(hash string) (string, error) {
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	bs, err := encoding.DecodeString(strings.ToUpper(hash))
	if err != nil {
		log.WithError(err).Warnf("error decoding hash %s", hash)
		return "", err
	}

	if len(bs) < 2 {
		return "", ErrInvalidTwtHash
	}

	// Produces a path structure of:
	// ./data/archive/[0-9a-f]{2,}0/[0-9a-f]+.json
	components := []string{a.path, fmt.Sprintf("%x", bs[0:1]), fmt.Sprintf("%x.json", bs[1:])}

	return filepath.Join(components...), nil
}

func (a *DiskArchiver) fileExists(fn string) bool {
	if _, err := os.Stat(fn); err != nil {
		return false
	}
	return true
}

func (a *DiskArchiver) Del(hash string) error {
	fn, err := a.makePath(hash)
	if err != nil {
		log.WithError(err).Errorf("error computing archive file for twt %s", hash)
		return err
	}

	if a.fileExists(fn) {
		return os.Remove(fn)
	}

	return nil
}

func (a *DiskArchiver) Has(hash string) bool {
	fn, err := a.makePath(hash)
	if err != nil {
		log.WithError(err).Errorf("error computing archive file for twt %s", hash)
		return false
	}

	return a.fileExists(fn)
}

func (a *DiskArchiver) Get(hash string) (types.Twt, error) {
	fn, err := a.makePath(hash)
	if err != nil {
		log.WithError(err).Errorf("error computing archive file for twt %s", hash)
		return types.Twt{}, err
	}

	if !a.fileExists(fn) {
		log.Warnf("twt %s not found in archive", hash)
		return types.Twt{}, ErrTwtNotArchived
	}

	data, err := ioutil.ReadFile(fn)
	if err != nil {
		log.WithError(err).Errorf("error reading archived twt %s", hash)
		return types.Twt{}, err
	}

	var twt types.Twt

	if err := json.Unmarshal(data, &twt); err != nil {
		log.WithError(err).Errorf("error decoding archived twt %s", hash)
		return types.Twt{}, err
	}

	return twt, nil
}

func (a *DiskArchiver) Archive(twt types.Twt) error {
	fn, err := a.makePath(twt.Hash())
	if err != nil {
		log.WithError(err).Errorf("error computing archive file for twt %s", twt.Hash())
		return err
	}

	if a.fileExists(fn) {
		log.Warnf("archived twt %s already exists", twt.Hash())
		return ErrTwtAlreadyArchived
	}

	if err := os.MkdirAll(filepath.Dir(fn), 0755); err != nil {
		log.WithError(err).Errorf("error creating archive directory for twt %s", twt.Hash())
		return err
	}

	data, err := json.Marshal(&twt)
	if err != nil {
		log.WithError(err).Errorf("error encoding twt %s", twt.Hash())
		return err
	}

	if err := ioutil.WriteFile(fn, data, 0644); err != nil {
		log.WithError(err).Errorf("error writing twt %s to archive", twt.Hash())
		return err
	}

	return nil
}

func (a *DiskArchiver) Count() (int, error) {
	var count int

	err := filepath.Walk(a.path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.WithError(err).Error("error walking archive directory")
			return err
		}

		if !info.IsDir() && filepath.Ext(info.Name()) == ".json" {
			count++
		}
		return nil
	})
	if err != nil {
		log.WithError(err).Errorf("error listing all archived twtw")
	}

	return count, err
}
