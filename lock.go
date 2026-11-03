package graphedwright

import "errors"

// ErrLocked is Apply's refusal while the document is locked.
var ErrLocked = errors.New("the document is locked: it may be read, not changed")

// Lock makes the document read-only through its methods until Unlock:
// Apply refuses with ErrLocked; Undo, Redo, Discard, Place and SetBends report
// that they did nothing; ReserveBends gives no room; Compact renumbers
// nothing and returns nil maps; and Group, EndGroup, ClearUndo, Fit and
// ClearBends do nothing. It is for a caller that hands the document to
// code that may only read it, such as a host's method called while the
// caller is in the middle of a change, so that the code cannot change
// the document underneath it. The exported fields are not guarded.
func (d *Document) Lock() { d.locked = true }

// Unlock ends a Lock.
func (d *Document) Unlock() { d.locked = false }

// Locked reports whether the document is locked.
func (d *Document) Locked() bool { return d.locked }
