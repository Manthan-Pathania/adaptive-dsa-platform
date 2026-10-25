package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLevel(t *testing.T) {
	assert.Equal(t, slog.LevelDebug, ParseLevel("debug"))
	assert.Equal(t, slog.LevelInfo, ParseLevel("info"))
	assert.Equal(t, slog.LevelWarn, ParseLevel("warn"))
	assert.Equal(t, slog.LevelWarn, ParseLevel("warning"))
	assert.Equal(t, slog.LevelError, ParseLevel("error"))
	assert.Equal(t, slog.LevelInfo, ParseLevel("unknown"))
}

func TestLogger_JSONFormat_WithRequestID(t *testing.T) {
	buf := new(bytes.Buffer)
	log := New("info", "json", buf)

	ctx := ContextWithRequestID(context.Background(), "test-req-999")
	log.InfoContext(ctx, "processing payment order", "order_id", "ord_123")

	var entry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &entry)
	require.NoError(t, err)

	assert.Equal(t, "processing payment order", entry["msg"])
	assert.Equal(t, "INFO", entry["level"])
	assert.Equal(t, "ord_123", entry["order_id"])
	assert.Equal(t, "test-req-999", entry["request_id"])
}

func TestLogger_TextFormat_WithoutRequestID(t *testing.T) {
	buf := new(bytes.Buffer)
	log := New("debug", "text", buf)

	log.DebugContext(context.Background(), "debugging connection pool", "idle_conns", 5)

	out := buf.String()
	assert.Contains(t, out, "level=DEBUG")
	assert.Contains(t, out, "msg=\"debugging connection pool\"")
	assert.Contains(t, out, "idle_conns=5")
	assert.NotContains(t, out, "request_id=")
}

func TestLogger_LevelFiltering(t *testing.T) {
	buf := new(bytes.Buffer)
	log := New("warn", "json", buf)

	log.Info("this should be ignored")
	assert.Empty(t, buf.String())

	log.Warn("this should be recorded")
	assert.NotEmpty(t, buf.String())
	assert.Contains(t, buf.String(), "this should be recorded")
}
