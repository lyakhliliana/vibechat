package middleware

import (
	"net/http"
	"runtime/debug"
	"time"

	"github.com/rs/zerolog"
)

// RequestLogger logs every HTTP request using the context logger (seeded by RequestID,
// enriched by Auth with user_id). Level depends on response status:
//
//	2xx/3xx → Debug
//	4xx     → Warn
//	5xx     → Error
func RequestLogger() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rw, r)

			logger := zerolog.Ctx(r.Context())
			evt := logEvent(logger, rw.status)
			evt.Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", rw.status).
				Dur("latency", time.Since(start)).
				Str("ip", clientIP(r)).
				Msg("http")
		})
	}
}

func logEvent(logger *zerolog.Logger, status int) *zerolog.Event {
	switch {
	case status >= 500:
		return logger.Error()
	case status >= 400:
		return logger.Warn()
	default:
		return logger.Debug()
	}
}

func LimitBody(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

func Recovery() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rv := recover(); rv != nil {
					zerolog.Ctx(r.Context()).Error().
						Interface("panic", rv).
						Bytes("stack", debug.Stack()).
						Msg("recovered from panic")
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"error":"internal server error"}`))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(status int) {
	sw.status = status
	sw.ResponseWriter.WriteHeader(status)
}
