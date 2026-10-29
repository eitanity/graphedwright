package doc

import (
	gw "code.eitanity.com/graphedwright"
)

// Builtin are the schemas the command knows without a host:
//
//   - "basic": a box, a pill, a diamond and a circle joined by arrows,
//     lines, curves and wires, any to any, with free attributes, and a
//     dashed group that holds any of them.
//   - "use-case": a UML use case diagram: actors, use cases in system
//     boundaries, associations from an actor to a use case, «include»
//     and «extend» between use cases, and generalisation.
//   - "threat-model": a data flow diagram for STRIDE threat modelling:
//     external entities, processes and data stores in nested trust
//     boundaries, joined by data flows; each element carries what an
//     analysis reads (trust levels, a flow's protocol and whether it is
//     encrypted and authenticated) and a free-text list of threats.
//   - "state-machine": a UML state machine diagram: an initial state,
//     states and composite states that hold others, choices and final
//     states, joined by transitions that carry their event, guard and
//     actions; a state carries its entry, exit and internal behaviour.
func Builtin() []Named {
	return []Named{
		{Name: "basic", Schema: basic()},
		{Name: "use-case", Schema: useCase()},
		{Name: "threat-model", Schema: threatModel()},
		{Name: "state-machine", Schema: stateMachine()},
	}
}

func basic() gw.Schema {
	return gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "box", Shape: gw.RoundRect, Size: gw.Size{W: 120, H: 36}},
			{Name: "pill", Shape: gw.Pill, Size: gw.Size{W: 100, H: 28}},
			{Name: "diamond", Shape: gw.Diamond, Size: gw.Size{W: 90, H: 54}},
			{Name: "circle", Shape: gw.Circle, Size: gw.Size{W: 44, H: 44}},
			{Name: "group", Shape: gw.RoundRect, Size: gw.Size{W: 200, H: 120}, Dashed: true, Container: true},
		},
		EdgeKinds: []gw.EdgeKind{
			{Name: "arrow", Route: gw.Straight, Arrow: true},
			{Name: "line", Route: gw.Straight},
			{Name: "curve", Route: gw.Curve, Arrow: true, Weighted: true},
			{Name: "wire", Route: gw.Orthogonal},
		},
	}
}

// The kinds of the use case schema.
const (
	ucActor gw.KindIndex = iota
	ucUseCase
)

func useCase() gw.Schema {
	return gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "actor", Shape: gw.Actor, Size: gw.Size{W: 40, H: 76}},
			{Name: "use case", Shape: gw.Circle, Size: gw.Size{W: 150, H: 48}},
			{Name: "system", Shape: gw.Rect, Size: gw.Size{W: 220, H: 140}, Container: true, Contains: []gw.KindIndex{ucUseCase}},
		},
		EdgeKinds: []gw.EdgeKind{
			{Name: "association", From: []gw.KindIndex{ucActor}, To: []gw.KindIndex{ucUseCase}},
			{Name: "include", From: []gw.KindIndex{ucUseCase}, To: []gw.KindIndex{ucUseCase}, Arrow: true, Head: gw.OpenHead, Dashed: true, Stereotype: "«include»"},
			{Name: "extend", From: []gw.KindIndex{ucUseCase}, To: []gw.KindIndex{ucUseCase}, Arrow: true, Head: gw.OpenHead, Dashed: true, Stereotype: "«extend»"},
			{Name: "generalisation", From: []gw.KindIndex{ucActor, ucUseCase}, To: []gw.KindIndex{ucActor, ucUseCase}, Arrow: true, Head: gw.HollowHead},
		},
	}
}

// The kinds of the threat model schema.
const (
	tmEntity gw.KindIndex = iota
	tmProcess
	tmStore
)

func threatModel() gw.Schema {
	trust := gw.AttrSpec{Key: "trust", Kind: gw.Choice, Choices: []string{"untrusted", "partial", "trusted"}, Default: "untrusted"}
	yesNo := func(key string) gw.AttrSpec {
		return gw.AttrSpec{Key: key, Kind: gw.Choice, Choices: []string{"unknown", "yes", "no"}, Default: "unknown"}
	}
	threats := gw.AttrSpec{Key: "threats", Kind: gw.Text}
	elements := []gw.KindIndex{tmEntity, tmProcess, tmStore}
	return gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "external entity", Shape: gw.Rect, Size: gw.Size{W: 140, H: 48}, Attrs: []gw.AttrSpec{trust, threats}},
			{Name: "process", Shape: gw.Circle, Size: gw.Size{W: 120, H: 64}, Attrs: []gw.AttrSpec{trust, threats}},
			{Name: "data store", Shape: gw.DataStore, Size: gw.Size{W: 130, H: 44}, Attrs: []gw.AttrSpec{yesNo("encrypted"), threats}},
			{Name: "trust boundary", Shape: gw.RoundRect, Size: gw.Size{W: 220, H: 140}, Dashed: true, Container: true, Attrs: []gw.AttrSpec{trust}},
		},
		EdgeKinds: []gw.EdgeKind{
			{Name: "data flow", From: elements, To: elements, Arrow: true,
				Attrs: []gw.AttrSpec{{Key: "protocol", Kind: gw.Text}, yesNo("encrypted"), yesNo("authenticated"), threats}},
		},
	}
}

// The kinds of the state machine schema.
const (
	smInitial gw.KindIndex = iota
	smState
	smChoice
	smFinal
	smComposite
)

func stateMachine() gw.Schema {
	text := func(key string) gw.AttrSpec { return gw.AttrSpec{Key: key, Kind: gw.Text} }
	behaviour := []gw.AttrSpec{text("entry"), text("exit"), text("internal")}
	return gw.Schema{
		VertexKinds: []gw.VertexKind{
			{Name: "initial", Shape: gw.Circle, Size: gw.Size{W: 24, H: 24}, Root: true},
			{Name: "state", Shape: gw.RoundRect, Size: gw.Size{W: 120, H: 40}, Attrs: behaviour},
			{Name: "choice", Shape: gw.Diamond, Size: gw.Size{W: 72, H: 44}},
			{Name: "final", Shape: gw.Circle, Size: gw.Size{W: 28, H: 28}, Leaf: true},
			{Name: "composite state", Shape: gw.RoundRect, Size: gw.Size{W: 240, H: 160}, Container: true,
				Contains: []gw.KindIndex{smInitial, smState, smChoice, smFinal, smComposite}, Attrs: behaviour},
		},
		EdgeKinds: []gw.EdgeKind{
			{Name: "transition", From: []gw.KindIndex{smInitial, smState, smChoice, smComposite},
				To: []gw.KindIndex{smState, smChoice, smFinal, smComposite}, Route: gw.Curve, Arrow: true,
				Attrs: []gw.AttrSpec{text("event"), text("guard"), text("actions")}},
		},
	}
}
