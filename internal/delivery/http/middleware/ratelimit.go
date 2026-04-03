package middleware

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// RateLimit is a per-IP fixed-window limiter; rejected requests get 429.
// ctx controls the lifetime of the background cleanup goroutine — pass the server context.
func RateLimit(ctx context.Context, maxReqs int, window time.Duration) func(http.Handler) http.Handler {
	type entry struct {
		count   int
		resetAt time.Time
	}

	var mu sync.Mutex
	clients := make(map[string]*entry)

	// Purge stale entries to prevent unbounded map growth.
	go func() {
		ticker := time.NewTicker(window * 10)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := time.Now()
				mu.Lock()
				for ip, e := range clients {
					if now.After(e.resetAt) {
						delete(clients, ip)
					}
				}
				mu.Unlock()
			}
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)

			mu.Lock()
			now := time.Now()
			e, ok := clients[ip]
			if !ok || now.After(e.resetAt) {
				clients[ip] = &entry{count: 1, resetAt: now.Add(window)}
				mu.Unlock()
				next.ServeHTTP(w, r)
				return
			}
			e.count++
			over := e.count > maxReqs
			mu.Unlock()

			if over {
				zerolog.Ctx(r.Context()).Warn().
					Str("ip", ip).
					Str("path", r.URL.Path).
					Msg("rate limit exceeded")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"too many requests"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP extracts the real client IP from proxy headers, falling back to RemoteAddr.
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		// X-Forwarded-For may be a comma-separated list; take the first entry.
		return strings.TrimSpace(strings.SplitN(fwd, ",", 2)[0])
	}
	if idx := strings.LastIndex(r.RemoteAddr, ":"); idx != -1 {
		return r.RemoteAddr[:idx]
	}
	return r.RemoteAddr
}
