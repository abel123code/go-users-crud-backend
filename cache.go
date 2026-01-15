package main

import (
	"errors"
	"time"
)

// ErrCacheMiss is returned when a user is not found in the cache
var ErrCacheMiss = errors.New("cache miss")

// getUserFromCache gets a user from the cache
func (a *api) getUserFromCache(id string) (User, error) {
	a.cacheMu.RLock()
	entry, ok := a.cache[id]
	expired := ok && time.Now().After(entry.expiresAt)
	a.cacheMu.RUnlock()

	if !ok {
		return User{}, ErrCacheMiss
	}

	// Check if entry has expired
	if expired {
		// Entry expired, remove it and return cache miss
		a.invalidateUserCache(id)
		return User{}, ErrCacheMiss
	}

	return entry.user, nil
}

// setUserCache stores a user in the cache
func (a *api) setUserCache(id string, u User, ttl time.Duration) {
	a.cacheMu.Lock()
	a.cache[id] = cacheEntry{
		user:      u,
		expiresAt: time.Now().Add(ttl),
	}
	a.cacheMu.Unlock()
}

// invalidateUserCache removes a user from the cache
func (a *api) invalidateUserCache(id string) {
	a.cacheMu.Lock()
	delete(a.cache, id)
	a.cacheMu.Unlock()
}
