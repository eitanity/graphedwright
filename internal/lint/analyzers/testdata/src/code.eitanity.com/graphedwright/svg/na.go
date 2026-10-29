package svg

func anys(v any) int { // want "any in the core"
	var e interface{} // want "empty interface in the core"
	_ = e
	switch v.(type) { // want "type switch outside dom"
	case int:
		return 1
	}
	if x, ok := v.(int); ok { // want "type assertion outside dom"
		return x
	}
	return 0
}
