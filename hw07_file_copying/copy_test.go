package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCopySuccess(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		offset int64
		limit  int64
		want   string
	}{
		{
			name:   "copies whole file when limit is zero",
			input:  "hello world",
			offset: 0,
			limit:  0,
			want:   "hello world",
		},
		{
			name:   "copies from offset to end when limit is zero",
			input:  "hello world",
			offset: 6,
			limit:  0,
			want:   "world",
		},
		{
			name:   "copies limited bytes from offset",
			input:  "hello world",
			offset: 6,
			limit:  3,
			want:   "wor",
		},
		{
			name:   "copies empty file",
			input:  "",
			offset: 0,
			limit:  0,
			want:   "",
		},
		{
			name:   "copies zero bytes when offset equals file size",
			input:  "hello",
			offset: 5,
			limit:  0,
			want:   "",
		},
		{
			name:   "copies available bytes when limit exceeds available bytes",
			input:  "hello",
			offset: 2,
			limit:  100,
			want:   "llo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			fromPath := writeTestFile(t, dir, "input.txt", tt.input)
			toPath := filepath.Join(dir, "output.txt")

			err := Copy(fromPath, toPath, tt.offset, tt.limit)
			if err != nil {
				t.Fatal(err)
			}

			got := readTestFile(t, toPath)
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCopyValidationErrors(t *testing.T) {
	t.Run("returns error when source path is empty", func(t *testing.T) {
		dir := t.TempDir()

		toPath := filepath.Join(dir, "output.txt")

		err := Copy("", toPath, 0, 0)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns error when destination path is empty", func(t *testing.T) {
		dir := t.TempDir()

		fromPath := writeTestFile(t, dir, "input.txt", "hello")

		err := Copy(fromPath, "", 0, 0)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns error when offset is negative", func(t *testing.T) {
		dir := t.TempDir()

		fromPath := writeTestFile(t, dir, "input.txt", "hello")
		toPath := filepath.Join(dir, "output.txt")

		err := Copy(fromPath, toPath, -1, 0)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns error when limit is negative", func(t *testing.T) {
		dir := t.TempDir()

		fromPath := writeTestFile(t, dir, "input.txt", "hello")
		toPath := filepath.Join(dir, "output.txt")

		err := Copy(fromPath, toPath, 0, -1)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestCopyOffsetExceedsFileSize(t *testing.T) {
	dir := t.TempDir()

	fromPath := writeTestFile(t, dir, "input1.txt", "hello")
	toPath := filepath.Join(dir, "output.txt")

	err := Copy(fromPath, toPath, 100, 0)
	if !errors.Is(err, ErrOffsetExceedsFileSize) {
		t.Fatalf("got %v, want %v", err, ErrOffsetExceedsFileSize)
	}
}

func TestCopyUnsupportedFile(t *testing.T) {
	dir := t.TempDir()

	fromPath := filepath.Join(dir, "source-dir")
	toPath := filepath.Join(dir, "output.txt")

	err := os.Mkdir(fromPath, 0o755)
	if err != nil {
		t.Fatal(err)
	}

	err = Copy(fromPath, toPath, 0, 0)
	if !errors.Is(err, ErrUnsupportedFile) {
		t.Fatalf("got %v, want %v", err, ErrUnsupportedFile)
	}
}

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)

	err := os.WriteFile(path, []byte(content), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	return path
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}
