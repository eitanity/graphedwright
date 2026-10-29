// Command graphedwright draws a document as SVG or checks it against a
// schema the command knows.
//
//	graphedwright render -f document.json [--layout [--flow top-to-bottom]] [--css] > graph.svg
//	graphedwright check -f document.json [--schema basic]
//
// Exit codes: 0 ok, 1 refused with a reason on stderr, 2 bad arguments.
package main

import (
	"flag"
	"io"
	"os"

	gw "code.eitanity.com/graphedwright"
	"code.eitanity.com/graphedwright/doc"
	"code.eitanity.com/graphedwright/layout"
	"code.eitanity.com/graphedwright/svg"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

const usage = "usage: graphedwright render -f document.json [--layout [--flow top-to-bottom]] [--css]\n       graphedwright check -f document.json [--schema name]\n"

// run is main without the exit, for the tests.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = io.WriteString(stderr, usage)
		return 2
	}
	switch args[0] {
	case "render":
		return render(args[1:], stdout, stderr)
	case "check":
		return check(args[1:], stdout, stderr)
	}
	_, _ = io.WriteString(stderr, "unknown command "+args[0]+"\n"+usage)
	return 2
}

// load reads and decodes the file; it returns the exit code on failure.
func load(path, schema string, stderr io.Writer) (*gw.Document, int) {
	if path == "" {
		_, _ = io.WriteString(stderr, "-f is required\n"+usage)
		return nil, 2
	}
	// Read one byte past the reader's limit, so that a file too big, or
	// one with no end such as /dev/zero, is refused without being read
	// whole.
	f, err := os.Open(path) //nolint:gosec // the file the user names
	if err != nil {
		_, _ = io.WriteString(stderr, err.Error()+"\n")
		return nil, 1
	}
	data, err := io.ReadAll(io.LimitReader(f, doc.MaxInput+1))
	_ = f.Close()
	if err != nil {
		_, _ = io.WriteString(stderr, err.Error()+"\n")
		return nil, 1
	}
	opts := doc.Options{Schemas: doc.Builtin()}
	if schema != "" {
		opts.Schemas = only(opts.Schemas, schema)
	}
	d, err := doc.Decode(data, opts)
	if err != nil {
		_, _ = io.WriteString(stderr, err.Error()+"\n")
		return nil, 1
	}
	return d, 0
}

// only keeps the named schema, so that a check against one name fails
// for a document naming another.
func only(schemas []doc.Named, name string) []doc.Named {
	out := make([]doc.Named, 0, 1)
	for _, s := range schemas {
		if s.Name == name {
			out = append(out, s)
		}
	}
	return out
}

func render(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("f", "", "the document file")
	relayout := fs.Bool("layout", false, "lay every vertex out afresh")
	flow := fs.String("flow", "", "with -layout, the way the layers run: left-to-right or top-to-bottom")
	css := fs.Bool("css", false, "embed the default stylesheet")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	d, code := load(*path, "", stderr)
	if code != 0 {
		return code
	}
	if *relayout {
		if _, err := d.Apply(gw.Command{Op: gw.Layout, Text: "all", Value: *flow}); err != nil {
			_, _ = io.WriteString(stderr, err.Error()+"\n")
			return 1
		}
		if _, err := layout.New(d.Caps()).Run(d, layout.Options{All: true}); err != nil {
			_, _ = io.WriteString(stderr, err.Error()+"\n")
			return 1
		}
	}
	opts := svg.Options{}
	if *css {
		opts.Style = svg.Stylesheet
	}
	if err := svg.Render(d, opts, stdout); err != nil {
		_, _ = io.WriteString(stderr, err.Error()+"\n")
		return 1
	}
	return 0
}

func check(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("f", "", "the document file")
	schema := fs.String("schema", "", "the schema the document must name")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	d, code := load(*path, *schema, stderr)
	if code != 0 {
		return code
	}
	_, _ = io.WriteString(stdout, "ok: "+itoa(d.LiveVertices())+" vertices, "+itoa(d.LiveEdges())+" edges\n")
	return 0
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
