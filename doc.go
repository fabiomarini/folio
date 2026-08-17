// Package folio rasterizes PDF pages to images by driving the PDFium C
// library directly from Go via purego — no cgo, so it builds with
// CGO_ENABLED=0 and cross-compiles cleanly.
//
// It is a self-contained prototype designed to be embedded later as a library
// or internal package. The public surface is intentionally small:
//
//	r := folio.New(folio.Options{LibPath: "libpdfium.dylib"})
//	doc, err := r.OpenDocument("file.pdf")
//	if err != nil { ... }
//	defer doc.Close()
//	for i := 0; i < doc.PageCount(); i++ {
//	    img, err := doc.RenderPage(i) // image.Image (RGBA)
//	    // encode img to PNG/JPEG ...
//	}
//
// Rendering mirrors the reference rasterizer: each page is drawn into a
// white-cleared BGRA bitmap at the requested DPI (scale = DPI/72) with
// annotations, then converted to an RGBA image.
//
// The PDFium shared library (libpdfium.{dylib,so,dll}) is loaded at runtime;
// see Options.LibPath and findLibrary for how it is located.
package folio
