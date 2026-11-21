package chart

import (
	"errors"
	"strconv"

	"code.eitanity.com/graphedwright/internal/xmltext"
)

// New makes a renderer of the capacities given, refusing one that is
// below 1 or above MaxCap.
func New(caps Caps) (*Renderer, error) {
	for _, c := range [...]struct {
		name string
		n    int
	}{{"Points", caps.Points}, {"Series", caps.Series}, {"Rows", caps.Rows}, {"Text", caps.Text}} {
		if c.n < 1 || c.n > MaxCap {
			return nil, errors.New("chart: capacity " + c.name + " " + strconv.Itoa(c.n) + " is not between 1 and " + strconv.Itoa(MaxCap))
		}
	}
	return &Renderer{
		caps:  caps,
		o:     xmltext.New(),
		order: make([]int32, caps.Rows),
		keys:  make([]float64, caps.Rows),
		num:   make([]byte, 0, numRoom),
	}, nil
}
