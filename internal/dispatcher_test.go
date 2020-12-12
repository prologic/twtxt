package internal

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDispatcher_Dispatch(t *testing.T) {
	a := 0
	aMu := sync.RWMutex{}

	b := 0
	bMu := sync.RWMutex{}

	c := 0
	cMu := sync.RWMutex{}

	d := NewDispatcher(10, 3)
	d.Start()

	_, _ = d.DispatchFunc(func() error {
		aMu.Lock()
		a = 1
		aMu.Unlock()
		return nil
	})

	_, _ = d.DispatchFunc(func() error {
		bMu.Lock()
		b = 2
		bMu.Unlock()
		return nil
	})

	_, _ = d.DispatchFunc(func() error {
		cMu.Lock()
		c = 3
		cMu.Unlock()
		return nil
	})

	time.Sleep(time.Second)

	aMu.RLock()
	assert.Equal(t, 1, a)
	aMu.RUnlock()

	bMu.RLock()
	assert.Equal(t, 2, b)
	bMu.RUnlock()

	cMu.RLock()
	assert.Equal(t, 3, c)
	cMu.RUnlock()
}

func TestDispatcher_Dispatch_Mutex(t *testing.T) {
	n := 100
	mu := &sync.RWMutex{}

	d := NewDispatcher(10, n)
	d.Start()

	var v []int

	for i := 0; i < n; i++ {
		_, _ = d.DispatchFunc(func() error {
			mu.Lock()
			v = append(v, 0)
			mu.Unlock()
			return nil
		})
	}

	time.Sleep(time.Second)

	mu.RLock()
	assert.Equal(t, n, len(v))
	mu.RUnlock()
}

func TestDispatcher_Stop(t *testing.T) {
	c := 0
	mu := sync.RWMutex{}

	d := NewDispatcher(1, 3)
	d.Start()

	_, _ = d.DispatchFunc(func() error {
		mu.Lock()
		c++
		mu.Unlock()
		return nil
	})

	time.Sleep(time.Millisecond * 100)
	d.Stop()
	time.Sleep(time.Millisecond * 100)

	_, err := d.DispatchFunc(func() error {
		mu.Lock()
		c++
		mu.Unlock()
		return nil
	})
	assert.NotNil(t, err)
}
