package folio

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// makeSolidPDF builds a minimal single-page PDF whose page is a solid-color
// rectangle (fill is an "r g b rg" triple in 0..1). It computes a correct xref
// table so strict parsers accept it.
func makeSolidPDF(fill string, w, h int) []byte {
	var buf bytes.Buffer
	offsets := make([]int, 5)
	buf.WriteString("%PDF-1.4\n%\xE2\xE3\xCF\xD3\n")

	offsets[1] = buf.Len()
	buf.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	offsets[2] = buf.Len()
	buf.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")
	offsets[3] = buf.Len()
	fmt.Fprintf(&buf, "3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] /Resources << /ProcSet [/PDF] >> /Contents 4 0 R >>\nendobj\n", w, h)

	content := fmt.Sprintf("%s\n0 0 %d %d re\nf\n", fill, w, h)
	offsets[4] = buf.Len()
	fmt.Fprintf(&buf, "4 0 obj\n<< /Length %d >>\nstream\n", len(content))
	buf.WriteString(content)
	buf.WriteString("endstream\nendobj\n")

	xref := buf.Len()
	buf.WriteString("xref\n0 5\n0000000000 65535 f \n")
	for i := 1; i <= 4; i++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[i])
	}
	buf.WriteString("trailer\n<< /Size 5 /Root 1 0 R >>\nstartxref\n")
	buf.WriteString(strconv.Itoa(xref))
	buf.WriteString("\n%%EOF\n")
	return buf.Bytes()
}

// testLibPath returns a path to the pdfium library, or skips the test.
func testLibPath(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("PDFIUM_LIB"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	for _, c := range []string{
		"../pageseer-macos-arm64/libpdfium.dylib",
		"../pageseer-macos-arm64/libpdfium.so",
		"libpdfium.dylib",
		"libpdfium.so",
	} {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	t.Skip("pdfium library not found (set PDFIUM_LIB to its path)")
	return ""
}

func writeTempPDF(t *testing.T, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "test.pdf")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestRender_SolidPage is the end-to-end validation: render a known PDF and
// check the output size and a sample pixel color. This catches binding errors,
// wrong scaling, and the BGRA->RGBA byte-order swap.
func TestRender_SolidPage(t *testing.T) {
	lib := testLibPath(t)
	r := New(Options{LibPath: lib, DPI: 72}) // 72 DPI => 1 px per point

	pdfPath := writeTempPDF(t, makeSolidPDF("0.20 0.40 0.80 rg", 200, 100))
	doc, err := r.OpenDocument(pdfPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer doc.Close()

	if got := doc.PageCount(); got != 1 {
		t.Fatalf("page count = %d, want 1", got)
	}
	img, err := doc.RenderPage(0)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 200 || b.Dy() != 100 {
		t.Fatalf("size = %dx%d, want 200x100", b.Dx(), b.Dy())
	}

	// Center pixel should be the fill color ~ (51,102,204) in 8-bit.
	r16, g16, b16, _ := img.At(100, 50).RGBA()
	check := func(name string, got16, want8 uint32) {
		want16 := int64(want8) * 257
		diff := int64(got16) - want16
		if diff < 0 {
			diff = -diff
		}
		if diff > 512 { // ~2/255 tolerance
			t.Errorf("%s = %d (8-bit %d), want ~%d", name, got16, got16>>8, want8)
		}
	}
	check("R", r16, 51)
	check("G", g16, 102)
	check("B", b16, 204)
}

// TestRender_Scaling verifies DPI scaling produces the expected pixel size.
func TestRender_Scaling(t *testing.T) {
	lib := testLibPath(t)
	r := New(Options{LibPath: lib, DPI: 144}) // 2 px per point
	pdfPath := writeTempPDF(t, makeSolidPDF("1 0 0 rg", 100, 50))
	doc, err := r.OpenDocument(pdfPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer doc.Close()
	img, err := doc.RenderPage(0)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 200 || b.Dy() != 100 {
		t.Fatalf("size = %dx%d, want 200x100 (100x50pt @144dpi)", b.Dx(), b.Dy())
	}
}
