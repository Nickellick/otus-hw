package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// RunCmd runs a command + arguments (cmd) with environment variables from env.
func RunCmd(cmd []string, env Environment) (returnCode int) {
	if len(cmd) == 0 {
		fmt.Fprintln(os.Stderr, "command is required")
		return 1
	}
	currentEnv := os.Environ()
	currentEnvMap := make(map[string]string, len(currentEnv))
	for _, line := range currentEnv {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		currentEnvMap[key] = value
	}
	for key, value := range env {
		if value.NeedRemove {
			delete(currentEnvMap, key)
			continue
		}
		currentEnvMap[key] = value.Value
	}
	newEnvs := make([]string, 0, len(currentEnvMap))
	for key, value := range currentEnvMap {
		newEnvs = append(newEnvs, key+"="+value)
	}

	// #nosec G204 -- Executing a user-provided command is the intended behavior of envdir.
	command := exec.Command(cmd[0], cmd[1:]...)
	command.Env = newEnvs
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	err := command.Run()
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError

	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	fmt.Fprintf(os.Stderr, "failed to execute %q: %v\n", cmd[0], err)
	return 1
}
