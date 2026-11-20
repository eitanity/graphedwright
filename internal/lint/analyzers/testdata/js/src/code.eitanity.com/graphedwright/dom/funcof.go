package dom

import "syscall/js"

var made int

func funcOf(fn func(js.Value, []js.Value) any) js.Func {
	made++
	return js.FuncOf(fn)
}

func listen(el js.Value) {
	el.Call("addEventListener", "click", js.FuncOf(func(js.Value, []js.Value) any { return nil })) // want `js.FuncOf outside funcOf`
	el.Call("addEventListener", "click", funcOf(func(js.Value, []js.Value) any { return nil }))
}

var later = js.FuncOf // want `js.FuncOf outside funcOf`

type set struct{}

// A method of that name is not the one place.
func (set) funcOf(fn func(js.Value, []js.Value) any) js.Func {
	return js.FuncOf(fn) // want `js.FuncOf outside funcOf`
}
