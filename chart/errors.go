package chart

import (
	"errors"
	"strconv"
)

// A refusal is built only when a chart is refused, which is not the
// drawing; this file is apart from it, and makes what it says.

// The refusals every kind shares.
var (
	ErrEmpty = errors.New("chart: nothing to draw")
)

// refuse is an error naming what was refused and where.
func refuse(what string, index int, why string) error {
	if index < 0 {
		return errors.New("chart: " + what + ": " + why)
	}
	return errors.New("chart: " + what + " " + strconv.Itoa(index) + ": " + why)
}

// capErr is the error for a chart past capacity name.
func capErr(name string, need, have int) error {
	return errors.New("chart: " + strconv.Itoa(need) + " " + name + " past the capacity of " + strconv.Itoa(have))
}
