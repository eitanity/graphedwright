package edit

type Point struct{ X, Y float64 }

// Editor is a state type.
type Editor struct {
	Open   int // want "state type Editor exports field Open"
	*Point     // want "state type Editor embeds an exported type"
	sel    []int
	pts    []Point
	byKey  map[string]int
	one    Point
	inner  *Sealed
}

// Sealed has no exported field: a pointer to one is no way in.
type Sealed struct{ n int }

// Options is not a state type: its fields are the caller's.
type Options struct{ Width int }

func (e *Editor) Selected() []int { return e.sel } // want "Selected returns a slice taken from its receiver"

func (e *Editor) Tail() []Point { return e.pts[1:] } // want "Tail returns a slice taken from its receiver"

func (e *Editor) Keys() map[string]int { return e.byKey } // want "Keys returns a map taken from its receiver"

func (e *Editor) One() *Point { return &e.one } // want "One returns a pointer taken from its receiver"

func (e *Editor) Some() []Point {
	b := e.pts[:2]
	c := b
	return c // want "Some returns a slice taken from its receiver"
}

// Copied: into the caller's buffer.
func (e *Editor) Copied(dst []int) []int { return append(dst, e.sel...) }

// Wrapped: in a value that gives no way in.
type List struct{ s []int }

func (e *Editor) List() List { return List{e.sel} }

// Inner: a pointer to a sealed type.
func (e *Editor) Inner() *Sealed { return e.inner }

// Fresh: not from the receiver.
func (e *Editor) Fresh() []int { return make([]int, 2) }

// unexported methods are the package's own.
func (e *Editor) sels() []int { return e.sel }
