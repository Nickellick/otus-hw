package main

import (
	"flag"
	"fmt"
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
	err := Copy(from, to, offset, limit)
	if err != nil {
		fmt.Fprintln(os.Stderr, "copy:", err)
		os.Exit(1)
	}
}
