package api

import (
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	pinHeader        = "X-Reader-Pin"
	maxFailures      = 5
	lockoutDuration  = 15 * time.Minute
	globalWindow     = time.Minute
	globalMaxFailure = 20
	recordTTL        = time.Hour
)

type attemptRecord struct {
	failures    int
	lockedUntil time.Time
	seen        time.Time
}

type attemptLimiter struct {
	mu           sync.Mutex
	records      map[string]*attemptRecord
	globalCount  int
	globalWindow time.Time
}

func newAttemptLimiter() *attemptLimiter {
	return &attemptLimiter{records: make(map[string]*attemptRecord)}
}

func (l *attemptLimiter) retryAfter(client string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	l.prune(now)

	if l.globalCount >= globalMaxFailure && now.Before(l.globalWindow.Add(globalWindow)) {
		return l.globalWindow.Add(globalWindow).Sub(now)
	}

	record, found := l.records[client]
	if found && now.Before(record.lockedUntil) {
		return record.lockedUntil.Sub(now)
	}
	return 0
}

func (l *attemptLimiter) fail(client string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()

	if now.After(l.globalWindow.Add(globalWindow)) {
		l.globalWindow = now
		l.globalCount = 0
	}
	l.globalCount++

	record, found := l.records[client]
	if !found {
		record = &attemptRecord{}
		l.records[client] = record
	}

	record.seen = now
	record.failures++

	if record.failures >= maxFailures {
		record.failures = 0
		record.lockedUntil = now.Add(lockoutDuration)
	}
}

func (l *attemptLimiter) succeed(client string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.records, client)
}

func (l *attemptLimiter) prune(now time.Time) {
	for key, record := range l.records {
		if now.After(record.lockedUntil) && now.Sub(record.seen) > recordTTL {
			delete(l.records, key)
		}
	}
}

func withPIN(pin string) func(http.Handler) http.Handler {
	limiter := newAttemptLimiter()
	expected := []byte(pin)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if pin == "" {
				next.ServeHTTP(w, r)
				return
			}

			client := clientAddress(r)

			if wait := limiter.retryAfter(client); wait > 0 {
				seconds := int(wait.Seconds()) + 1
				w.Header().Set("Retry-After", fmt.Sprintf("%d", seconds))
				writeError(w, http.StatusTooManyRequests,
					fmt.Sprintf("too many wrong PINs, try again in %d minutes", (seconds+59)/60))
				return
			}

			supplied := []byte(strings.TrimSpace(r.Header.Get(pinHeader)))
			if subtle.ConstantTimeCompare(supplied, expected) != 1 {
				limiter.fail(client)
				writeError(w, http.StatusUnauthorized, "wrong PIN")
				return
			}

			limiter.succeed(client)
			next.ServeHTTP(w, r)
		})
	}
}

func clientAddress(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if closest := strings.TrimSpace(parts[len(parts)-1]); closest != "" {
			return closest
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
