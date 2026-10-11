package internalhttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type testLogger struct {
	messages []string
}

func (l *testLogger) Info(msg string) {
	l.messages = append(l.messages, msg)
}

func (l *testLogger) Error(string) {}

func TestLoggingMiddleware(t *testing.T) {
	logger := &testLogger{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	})

	wrapped := loggingMiddleware(logger, handler)

	request := httptest.NewRequest(
		http.MethodPost,
		"/hello?q=1",
		nil,
	)

	request.RemoteAddr = "127.0.0.1:12345"

	request.Header.Set("User-Agent", "calendar-test")

	recorder := httptest.NewRecorder()

	wrapped.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Equal(t, "created", recorder.Body.String())

	require.Len(t, logger.messages, 1)

	require.Contains(t, logger.messages[0], "POST /hello?q=1 HTTP/1.1")
	require.Contains(t, logger.messages[0], "201")
	require.Contains(t, logger.messages[0], "calendar-test")

	require.True(t, strings.Contains(logger.messages[0], "127.0.0.1"))
}
