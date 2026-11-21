package dom

import (
	"syscall/js"

	gw "code.eitanity.com/graphedwright"
)

// large runs the object's functions for a large drawing: find, reveal,
// focus, unfocus, fold, unfold and outline.
func large(j *jsHost, op int, args []js.Value) js.Value {
	switch op {
	case opFind:
		return find(j, args)
	case opReveal:
		return reveal(j, args)
	case opFocus:
		return focus(j, args)
	case opUnfocus:
		return result(true, j.h.Unfocus())
	case opFold, opUnfold:
		return fold(j, op == opFold, args)
	}
	return outline(j)
}

// vertexArg is the live vertex the first argument names by ID.
func vertexArg(j *jsHost, args []js.Value) (gw.VertexIndex, bool) {
	if len(args) < 1 || args[0].Type() != js.TypeString {
		return gw.None, false
	}
	v := j.h.d.VertexByID(gw.ID(args[0].String()))
	return v, v != gw.None
}

// find(g, text, after) is the ID of the next vertex after the one with
// the ID after, or from the first without it, whose label or ID holds
// the text, its case folded; null for none.
func find(j *jsHost, args []js.Value) js.Value {
	if len(args) < 1 || args[0].Type() != js.TypeString {
		return fail("find(handle, text, after)")
	}
	after := gw.VertexIndex(gw.None)
	if len(args) > 1 && args[1].Type() == js.TypeString {
		after = j.h.d.VertexByID(gw.ID(args[1].String()))
	}
	v := j.h.d.Find(args[0].String(), after)
	if v == gw.None {
		return result(js.Null(), nil)
	}
	return result(string(j.h.d.Vertex(v).ID()), nil)
}

// reveal(g, id) unfolds what holds the vertex, selects it and centres
// it.
func reveal(j *jsHost, args []js.Value) js.Value {
	v, ok := vertexArg(j, args)
	if !ok {
		return fail("reveal(handle, id): no such vertex")
	}
	err := j.h.Reveal(v)
	return result(err == nil, err)
}

// focus(g, depth, way) focuses on the selection's neighbourhood: way
// "both", the default, "successors" or "predecessors"; false with
// nothing selected.
func focus(j *jsHost, args []js.Value) js.Value {
	if len(args) < 1 || args[0].Type() != js.TypeNumber {
		return fail("focus(handle, depth, way)")
	}
	way := gw.Both
	if len(args) > 1 && args[1].Type() == js.TypeString {
		switch args[1].String() {
		case "successors":
			way = gw.Successors
		case "predecessors":
			way = gw.Predecessors
		case "both":
		default:
			return fail(`focus: way is "both", "successors" or "predecessors"`)
		}
	}
	ok, err := j.h.Focus(args[0].Int(), way)
	return result(ok, err)
}

// fold(g, id) and unfold(g, id) fold a container and unfold it, as
// commands.
func fold(j *jsHost, folding bool, args []js.Value) js.Value {
	v, ok := vertexArg(j, args)
	if !ok {
		return fail("fold(handle, id): no such vertex")
	}
	op := gw.Unfold
	if folding {
		op = gw.Fold
	}
	_, err := j.h.Apply(gw.Command{Op: op, Vertex: v})
	return result(err == nil, err)
}

// outline(g) is the live vertices in drawing order, each container
// before what it holds: [{id, label, depth, folded, holds}].
func outline(j *jsHost) js.Value {
	if n := j.h.d.Caps().Vertices; cap(j.outline) < n {
		j.outline = make([]gw.OutlineEntry, 0, n)
	}
	entries := j.h.d.Outline(j.outline[:0])
	arr := js.Global().Get("Array").New(len(entries))
	for i, en := range entries {
		o := js.Global().Get("Object").New()
		v := j.h.d.Vertex(en.Vertex)
		o.Set("id", string(v.ID()))
		o.Set("label", v.Label())
		o.Set("depth", en.Depth)
		o.Set("folded", en.Folded)
		o.Set("holds", en.Holds)
		arr.SetIndex(i, o)
	}
	return result(arr, nil)
}
