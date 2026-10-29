package x

func assert(v any) int {
	if x, ok := v.(int); ok { // want "type assertion outside dom"
		return x
	}
	return 0
}
