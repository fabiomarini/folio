package folio

import "errors"

// Sentinel errors returned by the package. Use errors.Is to test for them.
var (
	// ErrNoLibrary is returned when the pdfium shared library cannot be
	// located (Options.LibPath empty and no candidate found on disk).
	ErrNoLibrary = errors.New("folio: pdfium shared library not found")

	// ErrNotBound is returned when a rendering method is called before the
	// pdfium library has been successfully loaded.
	ErrNotBound = errors.New("folio: pdfium library not loaded")
)
