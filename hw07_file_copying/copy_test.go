package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCopy(t *testing.T) {
	t.Run("copies whole file when limit is zero", func(t *testing.T) {
		dir := t.TempDir()

		fromPath := filepath.Join(dir, "input.txt")
		toPath := filepath.Join(dir, "output.txt")

		err := os.WriteFile(fromPath, []byte("hello world"), 0644)
		if err != nil {
			t.Fatal(err)
		}

		err = Copy(fromPath, toPath, 0, 0)
		if err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(toPath)
		if err != nil {
			t.Fatal(err)
		}

		want := "hello world"
		if string(got) != want {
			t.Fatalf("got %q, want %q", string(got), want)
		}
	})

	t.Run("copies from offset to end when limit is zero", func(t *testing.T) {
		dir := t.TempDir()

		fromPath := filepath.Join(dir, "input.txt")
		toPath := filepath.Join(dir, "output.txt")

		err := os.WriteFile(fromPath, []byte("hello world"), 0644)
		if err != nil {
			t.Fatal(err)
		}

		err = Copy(fromPath, toPath, 6, 0)
		if err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(toPath)
		if err != nil {
			t.Fatal(err)
		}

		want := "world"
		if string(got) != want {
			t.Fatalf("got %q, want %q", string(got), want)
		}
	})

	t.Run("copies limited bytes from offset", func(t *testing.T) {
		dir := t.TempDir()

		fromPath := filepath.Join(dir, "input.txt")
		toPath := filepath.Join(dir, "output.txt")

		err := os.WriteFile(fromPath, []byte("hello world"), 0644)
		if err != nil {
			t.Fatal(err)
		}

		err = Copy(fromPath, toPath, 6, 3)
		if err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(toPath)
		if err != nil {
			t.Fatal(err)
		}

		want := "wor"
		if string(got) != want {
			t.Fatalf("got %q, want %q", string(got), want)
		}
	})

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

		fromPath := filepath.Join(dir, "input.txt")

		err := os.WriteFile(fromPath, []byte("hello"), 0644)
		if err != nil {
			t.Fatal(err)
		}

		err = Copy(fromPath, "", 0, 0)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns error when offset is negative", func(t *testing.T) {
		dir := t.TempDir()

		fromPath := filepath.Join(dir, "input.txt")
		toPath := filepath.Join(dir, "output.txt")

		err := os.WriteFile(fromPath, []byte("hello"), 0644)
		if err != nil {
			t.Fatal(err)
		}

		err = Copy(fromPath, toPath, -1, 0)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns error when limit is negative", func(t *testing.T) {
		dir := t.TempDir()

		fromPath := filepath.Join(dir, "input.txt")
		toPath := filepath.Join(dir, "output.txt")

		err := os.WriteFile(fromPath, []byte("hello"), 0644)
		if err != nil {
			t.Fatal(err)
		}

		err = Copy(fromPath, toPath, 0, -1)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns ErrOffsetExceedsFileSize when offset is greater than file size", func(t *testing.T) {
		dir := t.TempDir()

		fromPath := filepath.Join(dir, "input.txt")
		toPath := filepath.Join(dir, "output.txt")

		err := os.WriteFile(fromPath, []byte("hello"), 0644)
		if err != nil {
			t.Fatal(err)
		}

		err = Copy(fromPath, toPath, 100, 0)
		if !errors.Is(err, ErrOffsetExceedsFileSize) {
			t.Fatalf("got %v, want %v", err, ErrOffsetExceedsFileSize)
		}
	})

	t.Run("returns ErrUnsupportedFile when source is directory", func(t *testing.T) {
		dir := t.TempDir()

		fromPath := filepath.Join(dir, "source-dir")
		toPath := filepath.Join(dir, "output.txt")

		err := os.Mkdir(fromPath, 0755)
		if err != nil {
			t.Fatal(err)
		}

		err = Copy(fromPath, toPath, 0, 0)
		if !errors.Is(err, ErrUnsupportedFile) {
			t.Fatalf("got %v, want %v", err, ErrUnsupportedFile)
		}
	})
}
