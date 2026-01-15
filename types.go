package main

import (
	"database/sql"
	"net/http"
	"sync"
	"time"
)

// User represents a user in the system
type User struct {
	ID        string    `json:"id"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
	CreatedAt time.Time `json:"createdAt"`
}

// cacheEntry represents a user in the cache
type cacheEntry struct {
	user      User
	expiresAt time.Time
}

// api represents the API server with database and cache
type api struct {
	addr    string
	db      *sql.DB
	cacheMu sync.RWMutex
	cache   map[string]cacheEntry
	// inflight dedupe helps to prevent duplicate requests for the same resource
	inflightMu sync.Mutex
	inflight   map[string]chan fetchResult
}

type fetchResult struct {
	user User
	err  error
}

// ctxKey is used for context keys to avoid collisions
type ctxKey string

// statusRecorder wraps http.ResponseWriter to capture status codes for logging
type statusRecorder struct {
	http.ResponseWriter
	status int
}
