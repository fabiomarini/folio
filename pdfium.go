package folio

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// PDFium constants (fpdfview.h).
const (
	bitmapBGRA  = 3 // FPDFBitmap_BGRA: 4 bytes/pixel, B,G,R,A
	renderAnnot = 1 // FPDF_ANNOT: render annotations
)

// fpdfMatrix mirrors pdfium's FPDF_MATRIX (CPDF_Matrix): six float32, no
// padding. It maps page coordinates to device (bitmap) coordinates.
type fpdfMatrix struct{ A, B, C, D, E, F float32 }

// fpdfRectF mirrors pdfium's FPDF_RECTF: four float32 in device or page
// coordinates, in the order left, top, right, bottom (this order is the
// on-memory layout pdfium reads).
type fpdfRectF struct{ Left, Top, Right, Bottom float32 }

// FPDF function pointers, bound against the loaded library. They are
// process-global: pdfium is a single shared library per process.
var (
	fpdfInitLibrary  func()
	fpdfLoadDocument func(path string, password *byte) unsafe.Pointer
	fpdfGetPageCount func(doc unsafe.Pointer) int
	fpdfDestroyDoc   func(doc unsafe.Pointer)
	fpdfLoadPage     func(doc unsafe.Pointer, index int) unsafe.Pointer
	fpdfClosePage    func(page unsafe.Pointer)
	fpdfPageCropBox  func(page unsafe.Pointer, l, b, w, h *float32) int
	fpdfPageMediaBox func(page unsafe.Pointer, l, b, w, h *float32) int
	fpdfBmpCreateEx  func(w, h, format int, buf []byte, stride int) unsafe.Pointer
	fpdfBmpFillRect  func(bmp unsafe.Pointer, l, t, w, h int, color uint32)
	fpdfBmpDestroy   func(bmp unsafe.Pointer)
	fpdfRenderMatrix func(bmp, page unsafe.Pointer, m *fpdfMatrix, clip *fpdfRectF, flags int)
)

var (
	bindOnce sync.Once
	bindErr  error
)

// bind loads the pdfium shared library at libPath and registers the FPDF
// symbols. It is idempotent — the first call wins and later calls return the
// same result. libPath must be non-empty and resolvable.
func bind(libPath string) error {
	bindOnce.Do(func() {
		handle, err := purego.Dlopen(libPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			bindErr = fmt.Errorf("load pdfium %q: %w", libPath, err)
			return
		}
		purego.RegisterLibFunc(&fpdfInitLibrary, handle, "FPDF_InitLibrary")
		purego.RegisterLibFunc(&fpdfLoadDocument, handle, "FPDF_LoadDocument")
		purego.RegisterLibFunc(&fpdfGetPageCount, handle, "FPDF_GetPageCount")
		purego.RegisterLibFunc(&fpdfDestroyDoc, handle, "FPDF_CloseDocument")
		purego.RegisterLibFunc(&fpdfLoadPage, handle, "FPDF_LoadPage")
		purego.RegisterLibFunc(&fpdfClosePage, handle, "FPDF_ClosePage")
		purego.RegisterLibFunc(&fpdfPageCropBox, handle, "FPDFPage_GetCropBox")
		purego.RegisterLibFunc(&fpdfPageMediaBox, handle, "FPDFPage_GetMediaBox")
		purego.RegisterLibFunc(&fpdfBmpCreateEx, handle, "FPDFBitmap_CreateEx")
		purego.RegisterLibFunc(&fpdfBmpFillRect, handle, "FPDFBitmap_FillRect")
		purego.RegisterLibFunc(&fpdfBmpDestroy, handle, "FPDFBitmap_Destroy")
		purego.RegisterLibFunc(&fpdfRenderMatrix, handle, "FPDF_RenderPageBitmapWithMatrix")
		fpdfInitLibrary()
	})
	return bindErr
}
