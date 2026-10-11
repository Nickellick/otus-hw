package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestHelperProcess выполняется в дочернем процессе.
// При обычном запуске go test сразу завершается.
func TestHelperProcess(_ *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	switch os.Getenv("HELPER_ACTION") {
	case "print_env":
		for _, key := range strings.Split(os.Getenv("HELPER_KEYS"), ",") {
			value, exists := os.LookupEnv(key)
			fmt.Printf("%s=%q,exists=%t\n", key, value, exists)
		}

	case "exit":
		os.Exit(42)

	case "streams":
		var input string
		fmt.Fscanln(os.Stdin, &input)
		fmt.Fprintln(os.Stdout, "stdout:"+input)
		fmt.Fprintln(os.Stderr, "stderr:"+input)

	case "write_env":
		keys := strings.Split(os.Getenv("HELPER_KEYS"), ",")
		var lines []string

		for _, key := range keys {
			value, exists := os.LookupEnv(key)
			lines = append(lines,
				fmt.Sprintf("%s=%q,exists=%t", key, value, exists),
			)
		}

		err := os.WriteFile(
			os.Getenv("HELPER_OUTPUT"),
			[]byte(strings.Join(lines, "\n")),
			0o600,
		)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	os.Exit(0)
}

func TestRunCmd_Environment(t *testing.T) {
	tests := []struct {
		name     string
		initial  map[string]string
		env      Environment
		expected string
	}{
		{
			name: "inherit environment",
			initial: map[string]string{
				"TEST_FOO": "original",
			},
			env:      Environment{},
			expected: `TEST_FOO="original",exists=true`,
		},
		{
			name: "replace variable",
			initial: map[string]string{
				"TEST_FOO": "original",
			},
			env: Environment{
				"TEST_FOO": {Value: "replaced"},
			},
			expected: `TEST_FOO="replaced",exists=true`,
		},
		{
			name: "remove variable",
			initial: map[string]string{
				"TEST_FOO": "original",
			},
			env: Environment{
				"TEST_FOO": {NeedRemove: true},
			},
			expected: `TEST_FOO="",exists=false`,
		},
		{
			name:    "add empty variable",
			initial: map[string]string{},
			env: Environment{
				"TEST_FOO": {Value: ""},
			},
			expected: `TEST_FOO="",exists=true`,
		},
		{
			name:    "value containing equals",
			initial: map[string]string{},
			env: Environment{
				"TEST_FOO": {Value: "hello=world"},
			},
			expected: `TEST_FOO="hello=world",exists=true`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GO_WANT_HELPER_PROCESS", "1")
			t.Setenv("HELPER_ACTION", "write_env")
			t.Setenv("HELPER_KEYS", "TEST_FOO")

			for key, value := range tt.initial {
				t.Setenv(key, value)
			}

			output := t.TempDir() + "/output.txt"
			t.Setenv("HELPER_OUTPUT", output)

			cmd := []string{
				os.Args[0],
				"-test.run=^TestHelperProcess$",
			}

			code := RunCmd(cmd, tt.env)
			if code != 0 {
				t.Fatalf("RunCmd() returned code %d, want 0", code)
			}

			data, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}

			actual := strings.TrimSpace(string(data))

			if actual != tt.expected {
				t.Errorf(
					"RunCmd() environment = %q, want %q",
					actual,
					tt.expected,
				)
			}
		})
	}
}

func TestRunCmd_ExitCode(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("HELPER_ACTION", "exit")

	cmd := []string{
		os.Args[0],
		"-test.run=^TestHelperProcess$",
	}

	code := RunCmd(cmd, Environment{})

	if code != 42 {
		t.Errorf("RunCmd() returned %d, want 42", code)
	}
}

func TestRunCmd_Errors(t *testing.T) {
	tests := []struct {
		name string
		cmd  []string
	}{
		{
			name: "empty command",
			cmd:  []string{},
		},
		{
			name: "nonexistent command",
			cmd:  []string{"__nonexistent_command_123456__"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := RunCmd(tt.cmd, Environment{})

			if code == 0 {
				t.Error("expected non-zero exit code")
			}
		})
	}
}

func TestRunCmd_Streams(t *testing.T) {
	dir := t.TempDir()

	inputPath := dir + "/stdin"
	stdoutPath := dir + "/stdout"
	stderrPath := dir + "/stderr"

	if err := os.WriteFile(inputPath, []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	input, err := os.Open(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()

	stdout, err := os.Create(stdoutPath)
	if err != nil {
		t.Fatal(err)
	}
	defer stdout.Close()

	stderr, err := os.Create(stderrPath)
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()

	oldStdin := os.Stdin
	oldStdout := os.Stdout
	oldStderr := os.Stderr

	os.Stdin = input
	os.Stdout = stdout
	os.Stderr = stderr

	t.Cleanup(func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	})

	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("HELPER_ACTION", "streams")

	cmd := []string{
		os.Args[0],
		"-test.run=^TestHelperProcess$",
	}

	code := RunCmd(cmd, Environment{})
	if code != 0 {
		t.Fatalf("RunCmd() returned %d, want 0", code)
	}

	stdoutData, err := os.ReadFile(stdoutPath)
	if err != nil {
		t.Fatal(err)
	}

	stderrData, err := os.ReadFile(stderrPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(stdoutData) != "stdout:hello\n" {
		t.Errorf("unexpected stdout: %q", stdoutData)
	}

	if string(stderrData) != "stderr:hello\n" {
		t.Errorf("unexpected stderr: %q", stderrData)
	}
}
