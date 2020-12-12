package session

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
)

// InvalidSessionID represents an empty, invalid session ID
const InvalidSessionID ID = ""

const idLength = 32
const signedLength = idLength + sha256.Size

// ID represents a valid, digitally-signed session ID
type ID string

// ErrInvalidID is returned when an invalid session id is passed to ValidateID()
var ErrInvalidID = errors.New("Invalid Session ID")

// NewSessionID creates and returns a new digitally-signed session ID,
// using `signingKey` as the HMAC signing key. An error is returned only
// if there was an error generating random bytes for the session ID
func NewSessionID(signingKey string) (ID, error) {
	buf := make([]byte, signedLength)
	_, err := rand.Read(buf[:idLength])
	if err != nil {
		return InvalidSessionID, err
	}

	mac := hmac.New(sha256.New, []byte(signingKey))
	_, _ = mac.Write(buf[:idLength])
	sig := mac.Sum(nil)
	copy(buf[idLength:], sig)

	return ID(base64.URLEncoding.EncodeToString(buf)), nil
}

// ValidateSessionID validates the `id` parameter using the `signingKey`
// and returns an error if invalid, or a SignedID if valid
func ValidateSessionID(id string, signingKey string) (ID, error) {
	buf, err := base64.URLEncoding.DecodeString(id)
	if err != nil {
		return InvalidSessionID, err
	}

	if len(buf) < signedLength {
		return InvalidSessionID, ErrInvalidID
	}

	mac := hmac.New(sha256.New, []byte(signingKey))
	_, _ = mac.Write(buf[:idLength])
	messageMAC := mac.Sum(nil)
	if !hmac.Equal(messageMAC, buf[idLength:]) {
		return InvalidSessionID, ErrInvalidID
	}

	return ID(id), nil
}

func (sid ID) String() string {
	return string(sid)
}
