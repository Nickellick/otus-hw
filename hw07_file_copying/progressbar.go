package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

type ProgressReader struct {
	r     io.Reader
	total int64
	read  int64
}

func (p *ProgressReader) Read(buf []byte) (int, error) {
	n, err := p.r.Read(buf)
	if n > 0 {
		p.read += int64(n)
		p.print()
	}
	return n, err
}

func (p *ProgressReader) print() {
	if p.total <= 0 {
		fmt.Fprintf(os.Stderr, "\rcopied %d bytes", p.read)
		return
	}

	const width = 30

	percent := float64(p.read) / float64(p.total)
	if percent > 1 {
		percent = 1
	}

	filled := int(percent * width)
	bar := strings.Repeat("=", filled) + strings.Repeat(" ", width-filled)

	fmt.Fprintf(
		os.Stderr,
		"\r[%s] %6.2f%% %d/%d bytes",
		bar,
		percent*100,
		p.read,
		p.total,
	)
}
