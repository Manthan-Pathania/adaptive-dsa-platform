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

// SecurityHeaders injects standard defensive HTTP response headers.
func SecurityHeaders(isProduction bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Permissions-Policy", "geolocation=(), camera=(), microphone=(), payment=(self \"https://checkout.razorpay.com\")")

			if isProduction {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
			}

			next.ServeHTTP(w, r)
		})
	}
}

// BodyLimit constrains incoming request payload sizes to defend against resource exhaustion DoS.
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	if maxBytes <= 0 {
		maxBytes = 1 << 20 // 1 MB default
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// CORS inspects incoming origins against an explicit whitelist and handles preflight OPTIONS requests.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	originMap := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originMap[o] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				w.Header().Set("Vary", "Origin")
				if originMap[origin] {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Credentials", "true")
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token, X-Request-ID, X-Razorpay-Signature, X-Razorpay-Event-Id")
					w.Header().Set("Access-Control-Max-Age", "86400")
				}
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
