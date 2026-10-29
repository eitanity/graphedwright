// Test module, so that merilo's explorer and these tests stay out of the
// main module.
module code.eitanity.com/graphedwright/internal/verify

go 1.27

toolchain go1.27.2

replace code.eitanity.com/graphedwright => ../..

require (
	code.eitanity.com/graphedwright v0.0.0-00010101000000-000000000000
	code.eitanity.com/merilo v0.1.0
)
