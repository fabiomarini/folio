package folio

import "testing"

// newTestRenderer builds a Renderer for integration tests, skipping when the
// pdfium library is not found (mirrors the renderer_test.go pattern).
func newTestRenderer(t *testing.T) *Renderer {
	t.Helper()
	return New(Options{LibPath: testLibPath(t)})
}

func TestPageTextInfo_Digital(t *testing.T) {
	r := newTestRenderer(t)
	doc, err := r.OpenDocument("testdata/text.pdf")
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()

	info, err := doc.PageTextInfo(0)
	if err != nil {
		t.Fatal(err)
	}
	if !info.HasTextLayer {
		t.Fatalf("text.pdf should have a text layer, got %+v", info)
	}
	if !info.HasMeaningfulText {
		t.Fatalf("text.pdf should be meaningful text, got %+v", info)
	}
	if info.CharCount < 11 {
		t.Fatalf("expected >= 11 chars, got %d", info.CharCount)
	}
}

func TestPageTextInfo_Scanned(t *testing.T) {
	r := newTestRenderer(t)
	doc, err := r.OpenDocument("testdata/scanned.pdf")
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()

	info, err := doc.PageTextInfo(0)
	if err != nil {
		t.Fatal(err)
	}
	if info.HasTextLayer || info.HasMeaningfulText {
		t.Fatalf("scanned.pdf should have no text layer, got %+v", info)
	}
}

func TestPageTextInfo_OutOfRange(t *testing.T) {
	r := newTestRenderer(t)
	doc, err := r.OpenDocument("testdata/text.pdf")
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	if _, err := doc.PageTextInfo(99); err == nil {
		t.Fatal("expected out-of-range error for page 99")
	}
}

func TestTextSummary(t *testing.T) {
	r := newTestRenderer(t)
	doc, err := r.OpenDocument("testdata/text.pdf")
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	s, err := doc.TextSummary()
	if err != nil {
		t.Fatal(err)
	}
	if !s.AllText || s.Mixed || s.AllScanned {
		t.Fatalf("expected AllText summary, got %+v", s)
	}
}
