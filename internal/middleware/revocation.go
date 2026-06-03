package middleware

import (
	"sync"
	"time"
)

// RevocationStore tracks revoked JWT IDs (jti claims) until their natural expiry.
// Once a token's exp time passes the entry is removed automatically, so the store
// stays bounded to the set of tokens that are both revoked and still unexpired.
type RevocationStore struct {
	mu    sync.RWMutex
	items map[string]time.Time // jti → token expiry
}

// NewRevocationStore creates an empty store with background cleanup.
func NewRevocationStore() *RevocationStore {
	rs := &RevocationStore{items: make(map[string]time.Time)}
	go rs.runCleanup()
	return rs
}

// Revoke marks a JTI as revoked. Pass the token's exp time so the entry is
// automatically pruned once the token would have expired anyway.
func (rs *RevocationStore) Revoke(jti string, expiry time.Time) {
	rs.mu.Lock()
	rs.items[jti] = expiry
	rs.mu.Unlock()
}

// IsRevoked reports whether the given JTI has been explicitly revoked.
func (rs *RevocationStore) IsRevoked(jti string) bool {
	rs.mu.RLock()
	_, ok := rs.items[jti]
	rs.mu.RUnlock()
	return ok
}

func (rs *RevocationStore) runCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		now := time.Now()
		rs.mu.Lock()
		for jti, exp := range rs.items {
			if now.After(exp) {
				delete(rs.items, jti)
			}
		}
		rs.mu.Unlock()
	}
}
