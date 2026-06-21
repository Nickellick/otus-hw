package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

var (
	from, to      string
	limit, offset int64
)

func main() {
	flag.StringVar(&from, "from", "", "file to read from")
	flag.StringVar(&to, "to", "", "file to write to")
	flag.Int64Var(&limit, "limit", 0, "limit of bytes to copy")
	flag.Int64Var(&offset, "offset", 0, "offset in input file")
	flag.Parse()

	if from == "" {
		fmt.Fprintln(os.Stderr, "-from is required")
		os.Exit(1)
	}

	if to == "" {
		fmt.Fprintln(os.Stderr, "-to is required")
		os.Exit(1)
	}

	if offset < 0 {
		fmt.Fprintln(os.Stderr, "-offset must be >= 0")
		os.Exit(1)
	}

	if limit < 0 {
		fmt.Fprintln(os.Stderr, "-limit must be >= 0")
		os.Exit(1)
	}

	src, err := os.Open(from)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open source:", err)
		os.Exit(1)
	}
	defer src.Close()

	dst, err := os.Create(to)
	if err != nil {
		fmt.Fprintln(os.Stderr, "create destination:", err)
		os.Exit(1)
	}
	defer dst.Close()

	_, err = src.Seek(offset, io.SeekStart)
	if err != nil {
		fmt.Fprintln(os.Stderr, "seek source:", err)
		os.Exit(1)
	}

	var reader io.Reader = src

	if limit > 0 {
		reader = io.LimitReader(src, limit)
	}

	written, err := io.Copy(dst, reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "copy:", err)
		os.Exit(1)
	}

	fmt.Printf("copied %d bytes\n", written)
}
