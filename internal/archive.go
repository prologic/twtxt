package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/prologic/twtxt/types"
)

const (
	archiveDir = "archive"
)

var (
	ErrTwtAlreadyArchived = errors.New("error: twt already archived")
	ErrTwtNotArchived     = errors.New("error: twt not found in archived")
)

func Chunks(s string, chunkSize int) []string {
	if chunkSize >= len(s) {
		return []string{s}
	}
	var chunks []string
	chunk := make([]rune, chunkSize)
	len := 0
	for _, r := range s {
		chunk[len] = r
		len++
		if len == chunkSize {
			chunks = append(chunks, string(chunk))
			len = 0
		}
	}
	if len > 0 {
		chunks = append(chunks, string(chunk[:len]))
	}
	return chunks
}

// Archiver is an interface for retrieving old twts from an archive storage
// such as an on-disk hash layout with one directory per 2-letter part of
// the hash sequence.
type Archiver interface {
	Has(hash string) bool
	Get(hash string) (types.Twt, error)
	Archive(twt types.Twt) error
}

// NullArchiver implements Archiver using dummy implementaiton stubs
type NullArchiver struct{}

func NewNullArchiver() (Archiver, error) {
	return &NullArchiver{}, nil
}

func (a *NullArchiver) Has(hash string) bool               { return false }
func (a *NullArchiver) Get(hash string) (types.Twt, error) { return types.Twt{}, nil }
func (a *NullArchiver) Archive(twt types.Twt) error        { return nil }

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

func (a *DiskArchiver) makePath(hash string) string {
	chunks := Chunks(hash, 2)
	for _, chunk := range chunks {
		if len(chunk) != 2 {
			chunk = fmt.Sprintf("0%s", chunk)
		}
	}

	return filepath.Join(append([]string{a.path}, append(chunks, "twt.json")...)...)
}

func (a *DiskArchiver) fileExists(fn string) bool {
	if _, err := os.Stat(fn); err != nil {
		return false
	}
	return true
}

func (a *DiskArchiver) Has(hash string) bool {
	return a.fileExists(a.makePath(hash))
}

func (a *DiskArchiver) Get(hash string) (types.Twt, error) {
	fn := a.makePath(hash)
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
	fn := a.makePath(twt.Hash())
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
