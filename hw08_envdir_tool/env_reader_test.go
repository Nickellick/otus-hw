package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadDir(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected Environment
	}{
		{
			name: "regular variables",
			files: map[string]string{
				"FOO": "hello",
				"BAR": "world",
			},
			expected: Environment{
				"FOO": {Value: "hello"},
				"BAR": {Value: "world"},
			},
		},
		{
			name: "empty file removes variable",
			files: map[string]string{
				"FOO": "",
			},
			expected: Environment{
				"FOO": {NeedRemove: true},
			},
		},
		{
			name: "empty first line",
			files: map[string]string{
				"FOO": "\nhello",
			},
			expected: Environment{
				"FOO": {Value: "", NeedRemove: false},
			},
		},
		{
			name: "only first line",
			files: map[string]string{
				"FOO": "hello\nworld",
			},
			expected: Environment{
				"FOO": {Value: "hello"},
			},
		},
		{
			name: "trailing spaces and tabs",
			files: map[string]string{
				"FOO": "hello \t  \nworld",
			},
			expected: Environment{
				"FOO": {Value: "hello"},
			},
		},
		{
			name: "null bytes",
			files: map[string]string{
				"FOO": "hello\x00world\nignored",
			},
			expected: Environment{
				"FOO": {Value: "hello\nworld"},
			},
		},
		{
			name: "invalid variable name",
			files: map[string]string{
				"FOO=BAR": "invalid",
				"VALID":   "hello",
			},
			expected: Environment{
				"VALID": {Value: "hello"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			for name, content := range tt.files {
				path := filepath.Join(dir, name)

				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					t.Fatalf("failed to create file %s: %v", name, err)
				}
			}

			actual, err := ReadDir(dir)
			if err != nil {
				t.Fatalf("ReadDir() returned error: %v", err)
			}

			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf(
					"ReadDir() = %#v, want %#v",
					actual,
					tt.expected,
				)
			}
		})
	}
}

func TestReadDir_NonexistentDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent")

	_, err := ReadDir(dir)
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestReadDir_SkipDirectories(t *testing.T) {
	dir := t.TempDir()

	if err := os.Mkdir(filepath.Join(dir, "SUBDIR"), 0o700); err != nil {
		t.Fatal(err)
	}

	env, err := ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(env) != 0 {
		t.Errorf("expected empty environment, got %v", env)
	}
}
