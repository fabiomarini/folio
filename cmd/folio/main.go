// Command folio rasterizes PDF documents to per-page PNG (or JPEG) images
// using the folio library (PDFium via purego).
//
// Usage:
//
//	folio [flags] <input.pdf> [more.pdf ...]
//
// Output layout (default): <output>/<stem>/page-NNN.png
// With --flat:             <output>/<stem>-NNN.png
package main

import (
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/fabiomarini/folio"
)

func main() {
	var (
		output  = flag.String("o", ".", "output directory")
		dpi     = flag.Float64("dpi", folio.DefaultDPI, "rasterization DPI")
		format  = flag.String("format", "png", "output format: png or jpeg")
		quality = flag.Int("quality", 90, "JPEG quality (1-100)")
		lib     = flag.String("lib", "", "path to libpdfium (default: auto-detect)")
		flat    = flag.Bool("flat", false, "flat naming <stem>-NNN.<ext> instead of <stem>/page-NNN.<ext>")
		pages   = flag.String("pages", "", "page range (1-based), e.g. 1-5,8 (default: all)")
		list    = flag.Bool("list", false, "print page count and exit")
		detect  = flag.Bool("detect", false, "detect scanned vs digital pages (text layer) and exit")
	)
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: folio [flags] <input.pdf> [more.pdf ...]")
		flag.PrintDefaults()
		os.Exit(2)
	}

	r := folio.New(folio.Options{DPI: *dpi, LibPath: *lib})

	status := 0
	for _, in := range flag.Args() {
		if err := process(r, in, *output, *format, *quality, *flat, *pages, *list, *detect); err != nil {
			fmt.Fprintf(os.Stderr, "folio: %s: %v\n", in, err)
			status = 1
		}
	}
	os.Exit(status)
}

func process(r *folio.Renderer, in, outDir, format string, quality int, flat bool, pagesSpec string, list bool, detect bool) error {
	doc, err := r.OpenDocument(in)
	if err != nil {
		return err
	}
	defer doc.Close()

	total := doc.PageCount()
	if list {
		fmt.Printf("%s: %d pages\n", in, total)
		return nil
	}
	if detect {
		return detectScan(doc, in)
	}

	pages, err := parsePages(pagesSpec, total)
	if err != nil {
		return err
	}

	stem := strings.TrimSuffix(filepath.Base(in), filepath.Ext(in))
	ext := ".png"
	if format == "jpeg" || format == "jpg" {
		ext = ".jpg"
	}

	for _, p := range pages {
		img, err := doc.RenderPage(p - 1) // 0-based
		if err != nil {
			return fmt.Errorf("page %d: %w", p, err)
		}
		var outPath string
		if flat {
			outPath = filepath.Join(outDir, fmt.Sprintf("%s-%03d%s", stem, p, ext))
		} else {
			outPath = filepath.Join(outDir, stem, fmt.Sprintf("page-%03d%s", p, ext))
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		if err := writeImage(outPath, img, format, quality); err != nil {
			return err
		}
		fmt.Printf("wrote %s\n", outPath)
	}
	return nil
}

func writeImage(path string, img image.Image, format string, quality int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	switch format {
	case "png":
		return png.Encode(f, img)
	case "jpeg", "jpg":
		return jpeg.Encode(f, img, &jpeg.Options{Quality: quality})
	default:
		return fmt.Errorf("unsupported format %q (use png or jpeg)", format)
	}
}

// detectScan prints a per-page scanned/digital classification plus a
// document rollup, using the text-layer detection in folio, and returns.
func detectScan(doc *folio.Document, in string) error {
	fmt.Printf("%s: %d pages\n", in, doc.PageCount())
	for i := 0; i < doc.PageCount(); i++ {
		info, err := doc.PageTextInfo(i)
		if err != nil {
			return err
		}
		kind := "scanned"
		if info.HasMeaningfulText {
			kind = "digital"
		}
		fmt.Printf("  page %d: %s (%d chars)\n", i+1, kind, info.CharCount)
	}

	s, err := doc.TextSummary()
	if err != nil {
		return err
	}
	class := "mixed"
	switch {
	case s.AllText:
		class = "all-digital"
	case s.AllScanned:
		class = "all-scanned"
	}
	fmt.Printf("  summary: %d digital, %d scanned (%s)\n", s.TextPages, s.ScannedPages, class)
	return nil
}

// parsePages parses a 1-based page spec like "1-5,8" into a sorted, de-duped
// list of page numbers. An empty spec means all pages.
func parsePages(spec string, total int) ([]int, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		all := make([]int, total)
		for i := range all {
			all[i] = i + 1
		}
		return all, nil
	}

	seen := make(map[int]bool)
	var out []int
	add := func(p int) error {
		if p < 1 || p > total {
			return fmt.Errorf("page %d out of range [1,%d]", p, total)
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
		return nil
	}

	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if lo, hi, ok := strings.Cut(part, "-"); ok {
			a, err := strconv.Atoi(strings.TrimSpace(lo))
			if err != nil {
				return nil, fmt.Errorf("bad page range %q", part)
			}
			b, err := strconv.Atoi(strings.TrimSpace(hi))
			if err != nil {
				return nil, fmt.Errorf("bad page range %q", part)
			}
			if a > b {
				a, b = b, a
			}
			for p := a; p <= b; p++ {
				if err := add(p); err != nil {
					return nil, err
				}
			}
		} else {
			a, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("bad page %q", part)
			}
			if err := add(a); err != nil {
				return nil, err
			}
		}
	}
	sort.Ints(out)
	return out, nil
}
