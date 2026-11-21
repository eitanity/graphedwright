// Command escapes is the escape-analysis gate. It reads on stdin the
// output of `go build -gcflags=-m ./... 2>&1`, or of a TinyGo build
// with -print-allocs, and the hot-path file list, and fails if any
// listed file has a value that escapes to or is moved to the heap, or
// that TinyGo allocates on it. The two differ: TinyGo keeps on the heap
// what Go's escape analysis keeps on the stack, among them what a
// closure captures and a buffer handed to strconv.
//
//	go build -gcflags=-m ./... 2>&1 | escapes internal/lint/hotpath.txt
//	tinygo build -print-allocs=. ... 2>&1 | escapes internal/lint/hotpath.txt
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: escapes hotpath.txt < go-build-m-output")
		os.Exit(2)
	}
	files, err := readList(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "escapes:", err)
		os.Exit(2)
	}
	bad := 0
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		line := sc.Text()
		if !strings.Contains(line, "escapes to heap") && !strings.Contains(line, "moved to heap") &&
			!strings.Contains(line, "object allocated on the heap") {
			continue
		}
		for _, f := range files {
			if strings.Contains(line, f+":") {
				fmt.Println("hot path allocation:", line)
				bad++
			}
		}
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "escapes:", err)
		os.Exit(2)
	}
	if bad > 0 {
		os.Exit(1)
	}
}

// readList returns the non-blank, non-comment lines of the allowlist.
func readList(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, l := range strings.Split(string(data), "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		out = append(out, l)
	}
	return out, nil
}
