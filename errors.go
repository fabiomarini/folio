package folio

import "errors"

// Sentinel errors returned by the package. Use errors.Is to test for them.
var (
	// ErrNoLibrary is returned when the pdfium shared library cannot be
	// located (Options.LibPath empty and no candidate found on disk).
	ErrNoLibrary = errors.New("folio: pdfium shared library not found")
)
