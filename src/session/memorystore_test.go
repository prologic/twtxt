package session

import (
	"reflect"
	"testing"
	"time"
)

func TestMemStore(t *testing.T) {
	type State struct {
		Requests int
	}

	state := &State{
		Requests: 100,
	}

	sid, err := NewSessionID(testSigningKey)
	if nil != err {
		t.Fatal(err)
	}

	memstore := NewMemoryStore(30 * time.Minute)
	err = memstore.Save(sid, state)
	if nil != err {
		t.Fatal(err)
	}

	state2 := &State{}
	err = memstore.Get(sid, state2)
	if nil != err {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(state, state2) {
		t.Errorf("retrieved state did not match saved state:\n got %v\n expected %v\n", state2, state)
	}

	err = memstore.Delete(sid)
	if nil != err {
		t.Fatal(err)
	}

	state3 := &State{}
	err = memstore.Get(sid, state2)
	if ErrStateNotFound != err {
		t.Fatal(err)
	}
	if state3.Requests != 0 {
		t.Errorf("found deleted session data in store:\n got %v", state3)
	}
}
