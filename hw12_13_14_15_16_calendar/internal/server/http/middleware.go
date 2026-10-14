package internalhttp

import (
	"net"
	"net/http"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}

	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}

	return w.ResponseWriter.Write(data)
}

func (w *responseWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}

	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func loggingMiddleware(logger Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{
			ResponseWriter: w,
		}

		next.ServeHTTP(wrapped, r)

		status := wrapped.status
		if status == 0 {
			status = http.StatusOK
		}

		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}

		logger.Info(
			"http request",
			"ip", ip,
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
			"proto", r.Proto,
			"status", status,
			"latency_ms", float64(time.Since(start).Microseconds())/1000,
			"user_agent", r.UserAgent(),
		)
	})
}
