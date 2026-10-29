package graphedwright

import (
	"errors"
	"strconv"
)

func itoa(v int) string { return strconv.Itoa(v) }

func utoa(v uint64) string { return strconv.FormatUint(v, 10) }

// schemaError names the kind at fault, or the schema as a whole.
func schemaError(where, msg string) error {
	if where == "" {
		return errors.New("schema: " + msg)
	}
	return errors.New("schema: " + where + ": " + msg)
}

// capError refuses a capacity out of range.
func capError(name string, n int) error {
	return errors.New("capacity " + name + " " + itoa(n) + " is not between 0 and " + itoa(MaxCap))
}

// limitError is the refusal of an edit past a capacity, naming it.
func limitError(what string, limit int) error {
	return errors.New(what + " limit " + itoa(limit) + " reached")
}

// refusal is the refusal of a command, naming the rule.
func refusal(msg string) error { return errors.New(msg) }

// vertexName is how a refusal names a vertex: its quoted ID.
func (d *Document) vertexName(v VertexIndex) string {
	return strconv.Quote(string(d.Vertices[v].ID))
}

// kindName is the quoted name of a vertex kind.
func (d *Document) kindName(k KindIndex) string {
	return strconv.Quote(d.Schema.VertexKinds[k].Name)
}
