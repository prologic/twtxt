package session

import (
	"encoding/json"
	"time"

	"github.com/patrickmn/go-cache"
)

//MemoryStore represents an in-memory session store.
//This should be used only for testing and prototyping.
//Production systems should use a shared server store like redis
type MemoryStore struct {
	entries *cache.Cache
}

//NewMemoryStore constructs and returns a new MemoryStore
func NewMemoryStore(sessionDuration time.Duration) *MemoryStore {
	if sessionDuration < 0 {
		sessionDuration = DefaultSessionDuration
	}
	return &MemoryStore{
		entries: cache.New(sessionDuration, time.Minute),
	}
}

//Store interface implementation

//Save associates the provided state data with the provided session id in the store.
func (ms *MemoryStore) Save(sid SessionID, state SessionData) error {
	j, err := json.Marshal(state)
	if nil != err {
		return err
	}
	ms.entries.Set(sid.String(), j, cache.DefaultExpiration)
	return nil
}

//Get retrieves the previously saved state data for the session id,
//and populates the `data` parameter with it. This will also
//reset the data's time to live in the store.
func (ms *MemoryStore) Get(sid SessionID, state SessionData) error {
	j, found := ms.entries.Get(sid.String())
	if !found {
		return ErrStateNotFound
	}
	return json.Unmarshal(j.([]byte), &state)
}

//Delete deletes all state data associated with the session id from the store.
func (ms *MemoryStore) Delete(sid SessionID) error {
	ms.entries.Delete(sid.String())
	return nil
}
