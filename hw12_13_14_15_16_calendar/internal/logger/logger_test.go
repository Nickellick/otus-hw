package logger

import (
	"bufio"
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLogger(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected []string
	}{
		{
			name:     "debug",
			level:    "debug",
			expected: []string{"DEBUG", "INFO", "WARN", "ERROR"},
		},
		{
			name:     "info",
			level:    "info",
			expected: []string{"INFO", "WARN", "ERROR"},
		},
		{
			name:     "warn",
			level:    "warn",
			expected: []string{"WARN", "ERROR"},
		},
		{
			name:     "error",
			level:    "error",
			expected: []string{"ERROR"},
		},
		{
			name:     "unknown level defaults to info",
			level:    "unknown",
			expected: []string{"INFO", "WARN", "ERROR"},
		},
		{
			name:     "case insensitive",
			level:    "DEBUG",
			expected: []string{"DEBUG", "INFO", "WARN", "ERROR"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer

			logg := newLogger(tt.level, &output)

			logg.Debug("debug message")
			logg.Info("info message")
			logg.Warn("warn message")
			logg.Error("error message")

			var actual []string

			scanner := bufio.NewScanner(&output)

			for scanner.Scan() {
				var entry struct {
					Time  string `json:"time"`
					Level string `json:"level"`
					Msg   string `json:"msg"`
				}

				err := json.Unmarshal(scanner.Bytes(), &entry)
				require.NoError(t, err)

				require.NotEmpty(t, entry.Time)
				require.NotEmpty(t, entry.Msg)

				actual = append(actual, entry.Level)
			}

			require.NoError(t, scanner.Err())
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestLoggerMessage(t *testing.T) {
	var output bytes.Buffer

	logg := newLogger("info", &output)

	logg.Info("calendar started")

	var entry struct {
		Level string `json:"level"`
		Msg   string `json:"msg"`
	}

	err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &entry)
	require.NoError(t, err)

	require.Equal(t, "INFO", entry.Level)
	require.Equal(t, "calendar started", entry.Msg)
}
