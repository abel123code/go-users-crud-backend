// middleware.go contains middleware for request ID, logging, recovery for all functions.
package main

import (
	"context"
	"log"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

// GetRequestID safely extracts the request ID from context.
// Returns empty string if missing (shouldn't happen once middleware is wired).
func GetRequestID(ctx context.Context) string {
	v := ctx.Value(requestIDKey)
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-ID")
		if rid == "" {
			rid = uuid.NewString()
		}

		// Put it into context so handlers/loggers can access it.
		ctx := context.WithValue(r.Context(), requestIDKey, rid)
		r = r.WithContext(ctx)

		w.Header().Set("X-Request-ID", rid)

		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs the request and response.

// statusRecorder wraps http.ResponseWriter so we can capture the status code.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		sr := &statusRecorder{
			ResponseWriter: w,
			status:         http.StatusOK, // default if handler never calls WriteHeader
		}

		next.ServeHTTP(sr, r)

		rid := GetRequestID(r.Context())
		log.Printf(
			"request_id=%s method=%s path=%s status=%d duration=%s",
			rid,
			r.Method,
			r.URL.Path,
			sr.status,
			time.Since(start),
		)
	})
}

// recoverMiddleware recovers from panics and logs the panic.
// To test put panic("test panic recovery") at the start of the handler you want to test
func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Catch panics from downstream middleware/handlers.
		defer func() {
			if rec := recover(); rec != nil {
				rid := GetRequestID(r.Context())

				// Log panic + stack trace (stack trace is gold for debugging)
				log.Printf("panic recovered request_id=%s panic=%v\n%s", rid, rec, debug.Stack())

				// If headers/body already started, we can't reliably send a new response.
				// But for most handler panics, this will still work fine.
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
