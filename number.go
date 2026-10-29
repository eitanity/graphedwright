package graphedwright

// MaxCoord bounds a coordinate's magnitude, a size and an edge's weight,
// so that geometry stays finite through every sum the layout and the
// renderer make, and the drawing's numbers short.
const MaxCoord = 1e9

// coord reports whether x is a coordinate: within MaxCoord of zero. It
// is false for NaN and the infinities.
func coord(x float64) bool { return x >= -MaxCoord && x <= MaxCoord }

// extent reports whether x is a size: positive and at most MaxCoord.
func extent(x float64) bool { return x > 0 && x <= MaxCoord }

// point reports whether both coordinates are.
func point(p Point) bool { return coord(p.X) && coord(p.Y) }

const (
	posRule    = "position must be finite and within 1e9 of the origin"
	sizeRule   = "size must be positive and finite, at most 1e9"
	weightRule = "weight must be finite, not negative and at most 1e9"
)
