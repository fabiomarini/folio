package folio

import "fmt"

// minTextChars is the minimum character count for a page to be treated as
// having a meaningful text layer. A page with 1–3 chars (a stray glyph,
// watermark, or page number) is not a "digital" page for routing purposes —
// the caller should route it to its VLM path. Exposed so callers can build
// their own thresholds on the raw count if they need different behavior.
const minTextChars = 10

// meaningfulFromCount applies the minTextChars threshold to a raw character
// count. Factored out so the pure-Go unit test exercises the production path.
func meaningfulFromCount(n int) bool { return n >= minTextChars }

// PageTextInfo reports whether a page has a usable text layer.
type PageTextInfo struct {
	// CharCount is the number of characters PDFium finds in the page's text
	// layer (FPDFText_CountChars). 0 means the page has no extractable text
	// (e.g. a scanned page backed only by an image). A small non-zero value
	// may be a watermark, page number, or stray glyph — check HasMeaningfulText.
	CharCount int

	// HasTextLayer is true when CharCount > 0 (PDFium found any text).
	HasTextLayer bool

	// HasMeaningfulText is true when CharCount >= minTextChars. This is the
	// routing signal doc-extract uses to decide the fast text path.
	HasMeaningfulText bool
}

// PageHasText reports whether page i has any text layer at all.
// Convenience wrapper over PageTextInfo.
func (d *Document) PageHasText(i int) (bool, error) {
	info, err := d.PageTextInfo(i)
	if err != nil {
		return false, err
	}
	return info.HasTextLayer, nil
}

// PageTextInfo inspects page i (0-based) and reports its text-layer presence.
// A page with no text layer is NOT an error — it returns a zero CharCount.
// Errors are reserved for real failures: out-of-range page index, or pdfium
// failing to load the page/textpage (e.g. a corrupt page).
func (d *Document) PageTextInfo(i int) (PageTextInfo, error) {
	if i < 0 || i >= d.PageCount() {
		return PageTextInfo{}, fmt.Errorf("folio: page %d out of range (0..%d)", i, d.PageCount()-1)
	}
	page := fpdfLoadPage(d.doc, i)
	if page == nil {
		return PageTextInfo{}, fmt.Errorf("folio: load page %d for text failed", i)
	}
	defer fpdfClosePage(page)

	tp := fpdfTextLoadPage(page)
	if tp == nil {
		// PDFium could not build a text page — treat as no text layer.
		return PageTextInfo{}, nil
	}
	defer fpdfTextClosePage(tp)

	n := fpdfTextCountChars(tp)
	return PageTextInfo{
		CharCount:         n,
		HasTextLayer:      n > 0,
		HasMeaningfulText: meaningfulFromCount(n),
	}, nil
}

// TextSummary is a whole-document rollup of per-page text presence.
type TextSummary struct {
	TotalPages   int
	TextPages    int // pages with HasMeaningfulText
	ScannedPages int // pages without HasMeaningfulText
	AllText      bool
	AllScanned   bool
	Mixed        bool
}

// TextSummary classifies every page and rolls the results up. It returns an
// error if any page fails to load (the caller can fall back to a VLM path).
// Equivalent to building TextSummary from repeated PageTextInfo, provided as
// a convenience for doc-extract's document-level routing decision.
func (d *Document) TextSummary() (TextSummary, error) {
	n := d.PageCount()
	s := TextSummary{TotalPages: n}
	for i := range n {
		info, err := d.PageTextInfo(i)
		if err != nil {
			return s, err
		}
		if info.HasMeaningfulText {
			s.TextPages++
		} else {
			s.ScannedPages++
		}
	}
	s.AllText = s.TextPages == n && n > 0
	s.AllScanned = s.ScannedPages == n && n > 0
	s.Mixed = !s.AllText && !s.AllScanned
	return s, nil
}
