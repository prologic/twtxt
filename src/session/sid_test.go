package session

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"testing"
)

const testSigningKey = "a very secret key"

func TestNewID(t *testing.T) {
	sid, err := NewSessionID(testSigningKey)

	if err != nil {
		t.Fatal(err)
	}

	if 0 == len(sid) {
		t.Errorf("Signed ID string was empty")
	}

	sid2, err := ValidateID(sid.String(), testSigningKey)
	if nil != err {
		fmt.Printf("generated: %v \n expected: %v\n", sid, sid2)
		t.Fatal(err)
	}

}

func TestInvalidKey(t *testing.T) {
	sid, err := NewSessionID(testSigningKey)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ValidateID(sid.String(), "some other signing key")
	if nil == err {
		t.Errorf("Was able to validate with incorrect signign key")
	}
}

func TestModified(t *testing.T) {
	sid, err := NewSessionID(testSigningKey)
	if err != nil {
		t.Fatal(err)
	}

	runes := []rune(sid.String())
	runes[0]++
	modsid := string(runes)

	_, err = ValidateID(modsid, testSigningKey)
	if nil == err {
		t.Errorf("Was able to validate modified encoded string")
	}
}

func TestEmptyID(t *testing.T) {
	_, err := ValidateID("", testSigningKey)
	if err == nil {
		t.Error("Able to validate empty key")
	}
}

func TestBadKey(t *testing.T) {
	buf := make([]byte, signedLength)
	if _, err := rand.Read(buf); nil != err {
		t.Fatal(err)
	}
	badid := base64.URLEncoding.EncodeToString(buf)

	_, err := ValidateID(badid, testSigningKey)
	if err == nil {
		t.Error("Able to validate bad key")
	}
}
