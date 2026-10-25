package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

type contextKey string

const (
	// RequestIDKey is the context key for tracing HTTP requests.
	RequestIDKey contextKey = "request_id"
)

// ContextWithRequestID attaches a request ID to the context.
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// RequestIDFromContext retrieves the request ID from the context, or returns an empty string.
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if val, ok := ctx.Value(RequestIDKey).(string); ok {
		return val
	}
	return ""
}

// contextHandler is a slog.Handler decorator that extracts context values (like request_id)
// and appends them to log records automatically.
type contextHandler struct {
	slog.Handler
}

// Handle extracts the request_id from context and attaches it as an attribute if present.
func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if reqID := RequestIDFromContext(ctx); reqID != "" {
		r.AddAttrs(slog.String("request_id", reqID))
	}
	return h.Handler.Handle(ctx, r)
}

// WithAttrs returns a new contextHandler with attributes added.
func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

// WithGroup returns a new contextHandler with group name added.
func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{Handler: h.Handler.WithGroup(name)}
}

// ParseLevel converts a string representation to slog.Level.
func ParseLevel(levelStr string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(levelStr)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// New creates and configures a new structured logger based on format and level.
// If output is nil, os.Stdout is used by default.
func New(levelStr, formatStr string, output io.Writer) *slog.Logger {
	if output == nil {
		output = os.Stdout
	}

	level := ParseLevel(levelStr)
	handlerOpts := &slog.HandlerOptions{
		Level: level,
	}

	var baseHandler slog.Handler
	if strings.ToLower(strings.TrimSpace(formatStr)) == "text" {
		baseHandler = slog.NewTextHandler(output, handlerOpts)
	} else {
		baseHandler = slog.NewJSONHandler(output, handlerOpts)
	}

	wrapped := &contextHandler{Handler: baseHandler}
	return slog.New(wrapped)
}
