// Package analyzers holds the go/analysis passes behind the custom checks:
// loop bound comments, no function-typed fields, no any or type switches in
// the engine, the statement bans, and a stated reason on every range over
// a map, whose order Go randomises, a stated reason on every error
// assigned to the blank identifier, and state kept sealed: no exported
// field on a state type and no slice, map or open pointer into it
// returned, and js.FuncOf named only in the browser binding's funcOf.
// Package classification is by import path under the main
// module.
package analyzers
