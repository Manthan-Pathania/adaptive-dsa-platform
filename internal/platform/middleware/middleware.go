package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/adaptive-dsa/platform/internal/platform/httperr"
	"github.com/adaptive-dsa/platform/internal/platform/logger"
	"github.com/google/uuid"
)

const HeaderXRequestID = "X-Request-ID"

// RequestID ensures every incoming request has a unique identifier for tracing.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get(HeaderXRequestID)
		if reqID == "" {
			reqID = uuid.NewString()
		}

		// Propagate into context and response header
		ctx := logger.ContextWithRequestID(r.Context(), reqID)
		w.Header().Set(HeaderXRequestID, reqID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// responseRecorder captures the status code written to the response.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	bytesSent  int
}

func (rr *responseRecorder) WriteHeader(code int) {
	rr.statusCode = code
	rr.ResponseWriter.WriteHeader(code)
}

func (rr *responseRecorder) Write(b []byte) (int, error) {
	n, err := rr.ResponseWriter.Write(b)
	rr.bytesSent += n
	return n, err
}

// RequestLogger logs incoming HTTP requests and their completion status/latency.
func RequestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &responseRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK, // default if WriteHeader is not explicitly called
			}

			next.ServeHTTP(rec, r)

			latency := time.Since(start)
			ctx := r.Context()

			// Do not log healthz requests at INFO level to keep production logs clean
			if r.URL.Path == "/healthz" {
				log.DebugContext(ctx, "http request",
					"method", r.Method,
					"path", r.URL.Path,
					"status", rec.statusCode,
					"duration_ms", latency.Milliseconds(),
				)
				return
			}

			log.InfoContext(ctx, "http request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.statusCode,
				"duration_ms", latency.Milliseconds(),
				"bytes", rec.bytesSent,
				"remote_addr", r.RemoteAddr,
				"user_agent", r.UserAgent(),
			)
		})
	}
}

// Recoverer intercepts panics, logs the failure with request ID, and returns RFC 9457 HTTP 500.
func Recoverer(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rvr := recover(); rvr != nil {
					ctx := r.Context()
					errStr := fmt.Sprintf("%v", rvr)
					stack := string(debug.Stack())

					log.ErrorContext(ctx, "panic recovered in http handler",
						"panic", errStr,
						"stack", stack,
						"path", r.URL.Path,
						"method", r.Method,
					)

					httperr.InternalServerError(w, r, fmt.Errorf("panic: %s", errStr), log)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
