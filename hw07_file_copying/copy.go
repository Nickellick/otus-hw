package main

import (
	"errors"
	"fmt"
	"io"
	"os"
)

var (
	ErrUnsupportedFile       = errors.New("unsupported file")
	ErrOffsetExceedsFileSize = errors.New("offset exceeds file size")
)

func Copy(fromPath, toPath string, offset, limit int64) error {
	if fromPath == "" {
		return fmt.Errorf("-from is required")
	}

	if toPath == "" {
		return fmt.Errorf("-to is required")
	}

	if offset < 0 {
		return fmt.Errorf("-offset must be positive")
	}

	if limit < 0 {
		return fmt.Errorf("-limit must be positive")
	}

	src, err := os.Open(fromPath)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(toPath)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer dst.Close()

	var reader io.Reader = src

	info, err := src.Stat()
	if err != nil {
		return fmt.Errorf("unstatable source: %w", err)
	}

	if !info.Mode().IsRegular() {
		return ErrUnsupportedFile
	}

	_, err = src.Seek(offset, io.SeekStart)
	if err != nil {
		return fmt.Errorf("unseekable source: %w", err)
	}

	available := info.Size() - offset
	if available < 0 {
		return ErrOffsetExceedsFileSize
	}

	total := available
	if limit > 0 {
		if limit < available {
			total = limit
		}
		reader = io.LimitReader(src, limit)
	}

	progressReader := &ProgressReader{
		r:     reader,
		total: total,
	}

	_, err = io.Copy(dst, progressReader)
	if err != nil {
		return fmt.Errorf("copy: %w", err)
	}

	return nil
}
