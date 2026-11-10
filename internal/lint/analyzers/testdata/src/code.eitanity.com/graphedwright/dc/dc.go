package dc

import "errors"

type myErr struct{}

func (*myErr) Error() string { return "mine" }

func fail() error            { return errors.New("x") }
func pair() (int, error)     { return 0, nil }
func errFirst() (error, int) { return nil, 0 }
func mine() *myErr           { return nil }
func count() int             { return 0 }
func two() (int, int)        { return 0, 0 }

func uses() int {
	_ = fail()         // want "an error assigned to _ without a stated reason"
	_, _ = pair()      // want "an error assigned to _ without a stated reason"
	n, _ := pair()     // want "an error assigned to _ without a stated reason"
	_, m := errFirst() // want "an error assigned to _ without a stated reason"
	_ = mine()         // want "an error assigned to _ without a stated reason"
	a, _ := fail(), count()
	_, _ = count(), fail() // want "an error assigned to _ without a stated reason"
	_ = count()
	_, _ = two()
	e := fail()
	// discarded: the result is reported the line before
	_ = e
	_ = fail() // discarded: nothing to report to
	return n + m + count() + len(a.Error())
}
