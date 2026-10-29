package st

func stmts() (err error) {
	defer func() {
		if r := recover(); r != nil { // want "recover"
			err = nil
		}
	}()
	go func() {}() // want "go statement"
	goto end       // want "goto"
end:
	return nil
}
