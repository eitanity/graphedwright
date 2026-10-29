package svg

import (
	"math"

	gw "code.eitanity.com/graphedwright"
)

// HitKind is what a point lands on.
type HitKind uint8

// The kinds of hit, from the background up.
const (
	Background HitKind = iota
	VertexHit
	PortHit
	EdgeHit
	OutlineHit // near the outline of a vertex without ports, where an edge starts
)

// Hit names the element under a point: the port and its vertex, the
// outline of a vertex without ports, the vertex, the edge, or the
// background.
type Hit struct {
	Kind   HitKind
	Vertex gw.VertexIndex
	Port   gw.PortIndex
	Edge   gw.EdgeIndex
}

// HitTest names the element drawn at a document point, from the same
// geometry Render draws: a port's circle first, then the vertex drawn
// last, its outline band when it has no ports or its box, then an edge
// within the tolerance of its path or on its label's text, then the
// container drawn uppermost whose box holds the point, else the
// background.
func HitTest(d *gw.Document, p gw.Point) Hit {
	if h, ok := hitPort(d, p); ok {
		return h
	}
	for i := len(d.Vertices) - 1; i >= 0; i-- {
		v := &d.Vertices[i]
		if !v.Live || d.Schema.VertexKinds[v.Kind].Container {
			continue
		}
		if len(v.Ports) == 0 && math.Abs(outlineDistance(d.Schema.VertexKinds[v.Kind].Shape, v, p)) <= band(v) {
			return Hit{Kind: OutlineHit, Vertex: gw.VertexIndex(i), Port: gw.None, Edge: gw.None} //nolint:gosec // bounded by the capacity
		}
		if inBox(v, p) {
			return Hit{Kind: VertexHit, Vertex: gw.VertexIndex(i), Port: gw.None, Edge: gw.None} //nolint:gosec // bounded by the capacity
		}
	}
	for i := len(d.Edges) - 1; i >= 0; i-- {
		if d.Edges[i].Live && nearEdge(d, gw.EdgeIndex(i), p) { //nolint:gosec // bounded by the capacity
			return Hit{Kind: EdgeHit, Vertex: gw.None, Port: gw.None, Edge: gw.EdgeIndex(i)} //nolint:gosec // bounded by the capacity
		}
	}
	for i := len(d.Edges) - 1; i >= 0; i-- {
		if d.Edges[i].Live && inLabel(d, gw.EdgeIndex(i), p) { //nolint:gosec // bounded by the capacity
			return Hit{Kind: EdgeHit, Vertex: gw.None, Port: gw.None, Edge: gw.EdgeIndex(i)} //nolint:gosec // bounded by the capacity
		}
	}
	if c := ContainerAt(d, p, gw.None); c != gw.None {
		return Hit{Kind: VertexHit, Vertex: c, Port: gw.None, Edge: gw.None}
	}
	return Hit{Kind: Background, Vertex: gw.None, Port: gw.None, Edge: gw.None}
}

// hitPort names the port whose circle holds the point, the later
// vertex's first.
func hitPort(d *gw.Document, p gw.Point) (Hit, bool) {
	for i := len(d.Vertices) - 1; i >= 0; i-- {
		v := &d.Vertices[i]
		if !v.Live {
			continue
		}
		for j := range v.Ports {
			q := portAt(v, gw.PortIndex(j)) //nolint:gosec // bounded by the ports per vertex
			dx, dy := p.X-q.X, p.Y-q.Y
			if float64(dx*dx)+float64(dy*dy) <= (portRadius+2)*(portRadius+2) {
				return Hit{Kind: PortHit, Vertex: gw.VertexIndex(i), Port: gw.PortIndex(j), Edge: gw.None}, true //nolint:gosec // bounded by the capacities
			}
		}
	}
	return Hit{}, false
}

// ContainerAt is the live container drawn uppermost whose box holds
// the point, the innermost of those nested there, and, when v is not
// None, that may hold v and is neither v nor in it: where v would go if
// dropped there. It is None when there is none.
func ContainerAt(d *gw.Document, p gw.Point, v gw.VertexIndex) gw.VertexIndex {
	order := d.Containers()
	for i := len(order) - 1; i >= 0; i-- {
		c := order[i]
		if !inBox(&d.Vertices[c], p) {
			continue
		}
		if v == gw.None || (!d.Within(c, v) && holds(d.Schema.VertexKinds[d.Vertices[c].Kind].Contains, d.Vertices[v].Kind)) {
			return c
		}
	}
	return gw.None
}

// holds reports whether a container's list of kinds takes the kind, an
// empty list taking any.
func holds(kinds []gw.KindIndex, k gw.KindIndex) bool {
	if len(kinds) == 0 {
		return true
	}
	for _, x := range kinds {
		if x == k {
			return true
		}
	}
	return false
}

// HandleSize is the side of the square the overlay draws on a selected
// vertex's bottom right corner, by which it is resized.
const HandleSize = 8

// HandleBox is the square of vertex v's resize handle, centred on the
// bottom right corner of its box.
func HandleBox(d *gw.Document, v gw.VertexIndex) gw.Viewport {
	x := &d.Vertices[v]
	return gw.Viewport{X: x.Pos.X + float64(x.Size.W/2) - HandleSize/2, Y: x.Pos.Y + float64(x.Size.H/2) - HandleSize/2, W: HandleSize, H: HandleSize}
}
