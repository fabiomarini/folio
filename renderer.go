package folio

import (
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"unsafe"
)

// DefaultDPI is used when Options.DPI is zero. 150 DPI puts an A4 page at
// ~1240×1754 px, right around the "large" VLM resolution budget.
const DefaultDPI = 150

// Options configures a Renderer. Zero values are replaced with defaults.
type Options struct {
	// DPI is the rasterization resolution (default DefaultDPI). scale = DPI/72.
	DPI float64
	// LibPath is the path to the pdfium shared library. If empty, common
	// locations are searched (see findLibrary).
	LibPath string
}

// Renderer renders PDF pages to images. A Renderer is safe to share; the
// pdfium library is loaded once, lazily, on first use.
type Renderer struct {
	dpi     float64
	libPath string
}

// New creates a Renderer, applying defaults to zero-valued options.
func New(opts Options) *Renderer {
	dpi := opts.DPI
	if dpi <= 0 {
		dpi = DefaultDPI
	}
	return &Renderer{dpi: dpi, libPath: opts.LibPath}
}

// Document is an open PDF ready for page rendering. Callers must Close it.
// A Document is not safe for concurrent use.
type Document struct {
	r     *Renderer
	doc   unsafe.Pointer
	pages int // cached at open; FFI page count is stable and costly to re-fetch
}

// OpenDocument opens the PDF at path for rendering. It loads the pdfium
// library on first use. The caller must Close the returned Document.
func (r *Renderer) OpenDocument(path string) (*Document, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("open %q: %w", path, err)
	}
	libPath, err := r.resolveLib()
	if err != nil {
		return nil, err
	}
	if err := bind(libPath); err != nil {
		return nil, err
	}
	doc := fpdfLoadDocument(path, nil)
	if doc == nil {
		return nil, fmt.Errorf("open %q: pdfium failed to load document (encrypted or corrupt?)", path)
	}
	return &Document{r: r, doc: doc, pages: fpdfGetPageCount(doc)}, nil
}

// PageCount returns the number of pages in the document.
func (d *Document) PageCount() int {
	return d.pages
}

// RenderPage renders the 0-based page i to an RGBA image.
func (d *Document) RenderPage(i int) (image.Image, error) {
	n := d.PageCount()
	if i < 0 || i >= n {
		return nil, fmt.Errorf("page %d out of range [0,%d)", i, n)
	}
	page := fpdfLoadPage(d.doc, i)
	if page == nil {
		return nil, fmt.Errorf("load page %d: pdfium failed", i)
	}
	defer fpdfClosePage(page)

	wPt, hPt, err := pageSize(page)
	if err != nil {
		return nil, err
	}
	base := d.r.dpi / 72.0
	w := clamp1(int(math.Round(wPt * base)))
	h := clamp1(int(math.Round(hPt * base)))
	// Use the exact scale that maps the page onto the rounded target size so
	// the page fills the bitmap precisely (using the raw dpi/72 leaves a
	// sub-pixel drift that misaligns text).
	sx := float64(w) / wPt
	sy := float64(h) / hPt

	stride := w * 4
	buf := make([]byte, stride*h)
	bmp := fpdfBmpCreateEx(w, h, bitmapBGRA, buf, stride)
	if bmp == nil {
		return nil, fmt.Errorf("create %dx%d bitmap: pdfium failed", w, h)
	}
	defer fpdfBmpDestroy(bmp)

	// Opaque white background, then render the page (with annotations) scaled
	// to the target size. The clip rect is the full bitmap in device coords;
	// passing nil clips the render to nothing.
	fpdfBmpFillRect(bmp, 0, 0, w, h, 0xFFFFFFFF)
	m := fpdfMatrix{A: float32(sx), D: float32(sy)}
	clip := fpdfRectF{Left: 0, Top: 0, Right: float32(w), Bottom: float32(h)}
	fpdfRenderMatrix(bmp, page, &m, &clip, renderAnnot)
	return bgraToImage(buf, w, h, stride), nil
}

// Close releases the document's native resources. It is idempotent.
func (d *Document) Close() error {
	if d.doc != nil {
		fpdfDestroyDoc(d.doc)
		d.doc = nil
	}
	return nil
}

// resolveLib returns the pdfium library path, using the configured path or
// searching common locations.
func (r *Renderer) resolveLib() (string, error) {
	if r.libPath != "" {
		return r.libPath, nil
	}
	return findLibrary()
}

// pageSize returns the page's width and height in points, using the CropBox
// and falling back to the MediaBox. The box out-params are left, bottom,
// right, top, so width = right-left and height = top-bottom.
func pageSize(page unsafe.Pointer) (float64, float64, error) {
	var l, b, r, t float32
	if fpdfPageCropBox(page, &l, &b, &r, &t) != 0 && r > l && t > b {
		return float64(r - l), float64(t - b), nil
	}
	if fpdfPageMediaBox(page, &l, &b, &r, &t) != 0 && r > l && t > b {
		return float64(r - l), float64(t - b), nil
	}
	return 0, 0, fmt.Errorf("determine page size: pdfium returned no box")
}

// findLibrary locates the pdfium shared library, searching (in order) the
// PDFIUM_LIB environment variable, the current directory, ./pdfium/, and the
// directory of the running executable.
func findLibrary() (string, error) {
	names := []string{"libpdfium.dylib", "libpdfium.so", "libpdfium.so.0", "pdfium.dll"}

	if p := os.Getenv("PDFIUM_LIB"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	dirs := []string{".", "./pdfium"}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		dirs = append(dirs, exeDir, filepath.Join(exeDir, "pdfium"))
	}
	for _, dir := range dirs {
		for _, name := range names {
			p := filepath.Join(dir, name)
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}
	return "", ErrNoLibrary
}

func clamp1(v int) int {
	if v < 1 {
		return 1
	}
	return v
}
