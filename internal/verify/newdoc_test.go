package verify_test

import gw "code.eitanity.com/graphedwright"

// makeDoc compiles the schema and makes a document of it: the refusal of
// either.
func makeDoc(s gw.Schema, caps gw.Caps) (*gw.Document, error) {
	k, err := gw.Compile(s)
	if err != nil {
		return nil, err
	}
	return gw.New(k, caps)
}
