package internalhttp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type capturedLog struct {
	message string
	args    []any
}

type captureLogger struct {
	entries []capturedLog
}

func (l *captureLogger) Info(msg string, args ...any) {
	l.entries = append(l.entries, capturedLog{
		message: msg,
		args:    args,
	})
}

func (l *captureLogger) Error(string, ...any) {}

func TestLoggingMiddleware(t *testing.T) {
	logg := &captureLogger{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodPost, "/hello?q=1", nil)
	req.RemoteAddr = "192.0.2.10:12345"
	req.Header.Set("User-Agent", "calendar-test")

	rec := httptest.NewRecorder()

	loggingMiddleware(logg, handler).ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Len(t, logg.entries, 1)

	entry := logg.entries[0]

	require.Equal(t, "http request", entry.message)

	attrs := make(map[string]any)

	for i := 0; i < len(entry.args); i += 2 {
		key, ok := entry.args[i].(string)
		require.True(t, ok)

		attrs[key] = entry.args[i+1]
	}

	require.Equal(t, "192.0.2.10", attrs["ip"])
	require.Equal(t, "POST", attrs["method"])
	require.Equal(t, "/hello", attrs["path"])
	require.Equal(t, "q=1", attrs["query"])
	require.Equal(t, "HTTP/1.1", attrs["proto"])
	require.Equal(t, 201, attrs["status"])
	require.Equal(t, "calendar-test", attrs["user_agent"])

	latency, ok := attrs["latency_ms"].(float64)
	require.True(t, ok)
	require.GreaterOrEqual(t, latency, 0.0)
}

type flushRecorder struct {
	*httptest.ResponseRecorder
	flushed bool
}

func (r *flushRecorder) Flush() {
	r.flushed = true
	r.ResponseRecorder.Flush()
}

func TestResponseWriterFlush(t *testing.T) {
	recorder := &flushRecorder{
		ResponseRecorder: httptest.NewRecorder(),
	}

	wrapped := &responseWriter{
		ResponseWriter: recorder,
	}

	wrapped.Flush()

	require.True(t, recorder.flushed)
	require.Equal(t, http.StatusOK, wrapped.status)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.True(t, recorder.Flushed)
}
