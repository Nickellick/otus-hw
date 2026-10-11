package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Environment map[string]EnvValue

// EnvValue helps to distinguish between empty files and files with the first empty line.
type EnvValue struct {
	Value      string
	NeedRemove bool
}

// ReadDir reads a specified directory and returns map of env variables.
// Variables represented as files where filename is name of variable, file first line is a value.
func ReadDir(dir string) (Environment, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}
	environments := make(Environment, len(entries))
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		if strings.Contains(entry.Name(), "=") {
			continue
		}
		environments[entry.Name()], err = readFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
	}
	return environments, nil
}

func readFile(path string) (envValue EnvValue, err error) {
	file, err := os.Open(path)
	if err != nil {
		return EnvValue{}, fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer func() {
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
	}()
	reader := bufio.NewReader(file)
	firstLine, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return EnvValue{}, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	if len(firstLine) == 0 {
		return EnvValue{NeedRemove: true}, nil
	}
	firstLine = strings.TrimSuffix(firstLine, "\n")
	firstLine = strings.TrimRight(firstLine, " \t")
	firstLine = strings.ReplaceAll(firstLine, "\x00", "\n")
	return EnvValue{Value: firstLine, NeedRemove: false}, nil
}
