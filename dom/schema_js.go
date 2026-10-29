package dom

import (
	"errors"
	"strconv"
	"syscall/js"

	gw "code.eitanity.com/graphedwright"
)

// schemaFromJS reads a schema described as an object:
//
//	{ name, acyclic, vertexKinds: [{name, shape, w, h, root, leaf, max, dashed,
//	  container, contains: [kindName], ports: [{name, side, max}],
//	  attrs: [{key, kind, choices, required, default}]}],
//	  edgeKinds: [{name, from: [kindName], to: [kindName], route, arrow, head,
//	  dashed, stereotype, weighted, attrs}] }
//
// Shapes are "rect", "round", "pill", "diamond", "circle", "actor",
// "datastore"; sides "left", "right", "top", "bottom"; routes
// "straight", "curve", "orthogonal"; heads "filled", "open", "hollow";
// attribute kinds "text", "number", "choice". The kinds an edge joins
// and a container holds are named, and resolved here, once every vertex
// kind is read.
func schemaFromJS(v js.Value) (gw.Schema, error) {
	s := gw.Schema{Acyclic: get(v, "acyclic").Truthy(), Connected: get(v, "connected").Truthy()}
	vks := get(v, "vertexKinds")
	for i := 0; vks.Type() == js.TypeObject && i < vks.Length(); i++ {
		vk, err := vertexKindFromJS(vks.Index(i))
		if err != nil {
			return s, err
		}
		s.VertexKinds = append(s.VertexKinds, vk)
	}
	for i := range s.VertexKinds {
		var err error
		if s.VertexKinds[i].Contains, err = kindsOf(s, get(vks.Index(i), "contains")); err != nil {
			return s, err
		}
	}
	eks := get(v, "edgeKinds")
	for i := 0; eks.Type() == js.TypeObject && i < eks.Length(); i++ {
		ek, err := edgeKindFromJS(s, eks.Index(i))
		if err != nil {
			return s, err
		}
		s.EdgeKinds = append(s.EdgeKinds, ek)
	}
	return s, nil
}

func vertexKindFromJS(k js.Value) (gw.VertexKind, error) {
	vk := gw.VertexKind{Name: str(k, "name"), Size: gw.Size{W: float(k, "w"), H: float(k, "h")}, Root: get(k, "root").Truthy(), Leaf: get(k, "leaf").Truthy(), Max: count(k, "max"), Dashed: get(k, "dashed").Truthy(), Container: get(k, "container").Truthy()}
	var err error
	if vk.Shape, err = shapeOf(str(k, "shape")); err != nil {
		return vk, err
	}
	ports := get(k, "ports")
	for j := 0; ports.Type() == js.TypeObject && j < ports.Length(); j++ {
		p := ports.Index(j)
		side, err := sideOf(str(p, "side"))
		if err != nil {
			return vk, err
		}
		vk.Ports = append(vk.Ports, gw.Port{Name: str(p, "name"), Side: side, Max: count(p, "max")})
	}
	vk.Attrs, err = attrsFromJS(get(k, "attrs"))
	return vk, err
}

func edgeKindFromJS(s gw.Schema, k js.Value) (gw.EdgeKind, error) {
	ek := gw.EdgeKind{Name: str(k, "name"), Arrow: get(k, "arrow").Truthy(), Weighted: get(k, "weighted").Truthy(), Dashed: get(k, "dashed").Truthy(), Stereotype: str(k, "stereotype")}
	var err error
	if ek.Route, err = routeOf(str(k, "route")); err != nil {
		return ek, err
	}
	if ek.Head, err = headOf(str(k, "head")); err != nil {
		return ek, err
	}
	if ek.From, err = kindsOf(s, get(k, "from")); err != nil {
		return ek, err
	}
	if ek.To, err = kindsOf(s, get(k, "to")); err != nil {
		return ek, err
	}
	ek.Attrs, err = attrsFromJS(get(k, "attrs"))
	return ek, err
}

func attrsFromJS(v js.Value) ([]gw.AttrSpec, error) {
	var out []gw.AttrSpec
	for i := 0; v.Type() == js.TypeObject && i < v.Length(); i++ {
		a := v.Index(i)
		spec := gw.AttrSpec{Key: str(a, "key"), Required: get(a, "required").Truthy(), Default: str(a, "default")}
		switch str(a, "kind") {
		case "", "text":
			spec.Kind = gw.Text
		case "number":
			spec.Kind = gw.Number
		case "choice":
			spec.Kind = gw.Choice
		default:
			return nil, errors.New("attribute kind " + strconv.Quote(str(a, "kind")) + " is not text, number or choice")
		}
		choices := get(a, "choices")
		for j := 0; choices.Type() == js.TypeObject && j < choices.Length(); j++ {
			spec.Choices = append(spec.Choices, choices.Index(j).String())
		}
		out = append(out, spec)
	}
	return out, nil
}

// kindsOf resolves a list of vertex kind names against the kinds read.
func kindsOf(s gw.Schema, v js.Value) ([]gw.KindIndex, error) {
	var out []gw.KindIndex
	for i := 0; v.Type() == js.TypeObject && i < v.Length(); i++ {
		name := v.Index(i).String()
		found := false
		for j, k := range s.VertexKinds {
			if k.Name == name {
				out = append(out, gw.KindIndex(j)) //nolint:gosec // bounded by the schema
				found = true
			}
		}
		if !found {
			return nil, errors.New("schema names an unknown vertex kind " + strconv.Quote(name))
		}
	}
	return out, nil
}

func shapeOf(name string) (gw.Shape, error) {
	switch name {
	case "", "rect":
		return gw.Rect, nil
	case "round":
		return gw.RoundRect, nil
	case "pill":
		return gw.Pill, nil
	case "diamond":
		return gw.Diamond, nil
	case "circle":
		return gw.Circle, nil
	case "actor":
		return gw.Actor, nil
	case "datastore":
		return gw.DataStore, nil
	}
	return gw.Rect, errors.New("shape " + strconv.Quote(name) + " is not rect, round, pill, diamond, circle, actor or datastore")
}

func headOf(name string) (gw.Head, error) {
	switch name {
	case "", "filled":
		return gw.FilledHead, nil
	case "open":
		return gw.OpenHead, nil
	case "hollow":
		return gw.HollowHead, nil
	}
	return gw.FilledHead, errors.New("head " + strconv.Quote(name) + " is not filled, open or hollow")
}

func sideOf(name string) (gw.Side, error) {
	switch name {
	case "", "left":
		return gw.Left, nil
	case "right":
		return gw.Right, nil
	case "top":
		return gw.Top, nil
	case "bottom":
		return gw.Bottom, nil
	}
	return gw.Left, errors.New("side " + strconv.Quote(name) + " is not left, right, top or bottom")
}

func routeOf(name string) (gw.Route, error) {
	switch name {
	case "", "straight":
		return gw.Straight, nil
	case "curve":
		return gw.Curve, nil
	case "orthogonal":
		return gw.Orthogonal, nil
	}
	return gw.Straight, errors.New("route " + strconv.Quote(name) + " is not straight, curve or orthogonal")
}
