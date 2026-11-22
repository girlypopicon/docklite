package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Limits on failed logins that don't depend on one IP-and-username pair: a username that is being guessed from many
// addresses, and an address that tries many usernames, are both stopped. (The older limiter in auth_login.go keys on
// the pair.) In memory, like it; a restart forgets them.

type failLimiter struct {
	mu      sync.Mutex
	max     int
	window  time.Duration
	entries map[string]*loginAttempt
}

func newFailLimiter(max int, window time.Duration) *failLimiter {
	return &failLimiter{max: max, window: window, entries: map[string]*loginAttempt{}}
}

func (l *failLimiter) blocked(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[key]
	if !ok {
		return false
	}
	if time.Since(e.FirstAttempt) > l.window {
		delete(l.entries, key)
		return false
	}
	return e.Count >= l.max
}

func (l *failLimiter) record(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if len(l.entries) > 10000 { // keep the table from growing without bound
		for k, e := range l.entries {
			if now.Sub(e.FirstAttempt) > l.window {
				delete(l.entries, k)
			}
		}
	}
	if e, ok := l.entries[key]; ok && now.Sub(e.FirstAttempt) <= l.window {
		e.Count++
		return
	}
	l.entries[key] = &loginAttempt{Count: 1, FirstAttempt: now}
}

var (
	ipFailures   = newFailLimiter(40, 5*time.Minute)
	userFailures = newFailLimiter(20, 15*time.Minute)
)

var (
	dummyOnce sync.Once
	dummyHash string
)

// dummyPasswordHash is a real hash of a random password, for comparing against when the username doesn't exist.
func dummyPasswordHash() string {
	dummyOnce.Do(func() {
		b := make([]byte, 18)
		_, _ = rand.Read(b)
		h, _ := bcrypt.GenerateFromPassword([]byte(base64.RawURLEncoding.EncodeToString(b)), bcrypt.DefaultCost)
		dummyHash = string(h)
	})
	return dummyHash
}
