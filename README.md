# folio

Rasterize PDF pages to PNG/JPEG images from pure Go, by driving the **PDFium**
C library directly through [purego](https://github.com/ebitengine/purego) —
**no cgo**. It builds with `CGO_ENABLED=0` and cross-compiles cleanly.

This is a self-contained library with a small, stable public API, designed to
be embedded as a package in larger Go projects or used standalone via the CLI.

```
folio/
├── doc.go              package documentation
├── errors.go           sentinel errors
├── pdfium.go           PDFium C bindings (purego) — all C-API detail lives here
├── bitmap.go           BGRA buffer -> image.NRGBA (pure Go, unit-tested)
├── renderer.go         public API: Options, Renderer, Document
├── *_test.go           unit + integration tests
├── scripts/            get-pdfium.sh — fetch the PDFium shared library
└── cmd/folio/          thin CLI wrapper
```

## Why purego + pdfium

- **Single static binary.** No cgo, no CGO toolchain, no `.so` linking at build
  time. The pdfium shared library is loaded at runtime.
- **Uses a standard prebuilt `libpdfium.{dylib,so,dll}`** (see [Obtaining the
  PDFium library](#obtaining-the-pdfium-library)) — no native dependency to
  build or link.
- **In-process.** No subprocess, no IPC; rendering returns an `image.Image`.
- **BSD 3-Clause** (PDFium) — permissive, and compatible with this project's
  MIT license.

The pattern is proven in the wild: `go-fitz` (PyMuPDF's Go port) uses purego in
its `nocgo` build mode.

## Building

```sh
cd folio
CGO_ENABLED=0 go build -o folio ./cmd/folio
```

The pdfium shared library is located at runtime (see [Library location](#library-location)).

## Obtaining the PDFium library

folio does not bundle PDFium — it loads a shared library at runtime. Fetch a
prebuilt one with the helper script:

```sh
scripts/get-pdfium.sh                 # latest release, current platform
scripts/get-pdfium.sh chromium/8009   # pin a specific release tag
```

The script downloads the right binary for your platform from
[bblanchon/pdfium-binaries](https://github.com/bblanchon/pdfium-binaries) and
places it in `pdfium/` together with its BSD 3-Clause license files. It honours
`GOOS`/`GOARCH` when Go is installed, so it also works for cross-compilation
targets. Supported: darwin / linux / windows × amd64 / arm64 / 386.

Prefer to manage the library yourself? Any of these work:

- `--lib /path/to/libpdfium.dylib` (CLI flag), or
- `PDFIUM_LIB=/path/to/libpdfium.dylib` (environment variable), or
- drop `libpdfium.{dylib,so,dll}` in the current directory, `./pdfium/`, or
  next to the executable.

> PDFium is **BSD 3-Clause**. Keep the `LICENSE` / `licenses/` files the script
> writes alongside the library when you redistribute it.

## CLI

```
folio [flags] <input.pdf> [more.pdf ...]
```

| flag | default | description |
|------|---------|-------------|
| `-o` | `.` | output directory |
| `--dpi` | `150` | rasterization DPI (`scale = DPI/72`) |
| `--format` | `png` | `png` or `jpeg` |
| `--quality` | `90` | JPEG quality (1–100) |
| `--lib` | auto | explicit path to `libpdfium` |
| `--flat` | off | flat naming `<stem>-NNN.<ext>` |
| `--pages` | all | 1-based range, e.g. `1-5,8` |
| `--list` | off | print page count and exit |

Output layout (default): `<output>/<stem>/page-NNN.png`. With `--flat`:
`<output>/<stem>-NNN.png`.

```sh
# Render a document at 300 DPI
folio --dpi 300 -o out document.pdf

# First 3 pages, flat layout, JPEG
folio --pages 1-3 --flat --format jpeg -o out document.pdf
```

## Library

Add it as a dependency, then import and use it:

```sh
go get github.com/fabiomarini/folio
```

```go
import "github.com/fabiomarini/folio"

r := folio.New(folio.Options{
    DPI:     150,                    // 0 -> default (150)
    LibPath: "libpdfium.dylib",      // "" -> auto-detect
})

doc, err := r.OpenDocument("file.pdf")
if err != nil {
    // ErrNoLibrary if the lib can't be found; other errors if the PDF is bad
}
defer doc.Close()

for i := 0; i < doc.PageCount(); i++ {
    img, err := doc.RenderPage(i)    // image.Image (concretely *image.NRGBA)
    if err != nil {
        // ...
    }
    // encode img to PNG/JPEG, feed a VLM, etc.
}
```

The API follows the `io.Reader` / `database.Conn` resource pattern: `New` makes
a reusable `Renderer` (the pdfium library is loaded once, lazily, on first use);
`OpenDocument` opens a `Document` that you `Close` when done.

### Concurrency

- A `Renderer` is safe to share across goroutines.
- A `Document` is **not** safe for concurrent use — render one page at a time
  from a single goroutine.
- Multiple `Document`s **can** be processed in parallel (each on its own
  goroutine), matching PDFium's one-document-per-thread model.

## Library location

`Options.LibPath` is used if set. Otherwise the library is searched (in order):

1. the `PDFIUM_LIB` environment variable,
2. `./libpdfium.{dylib,so,so.0}` and `./pdfium/…`,
3. the directory of the running executable (and its `pdfium/` subdir).

Set `--lib` or `PDFIUM_LIB` for an explicit path.

## Rendering model

Each page is rendered as follows:

1. Load the document and page.
2. Read the page size in points (CropBox, falling back to MediaBox).
3. `target = round(points * DPI/72)`; `scale = target / points` (so the page
   fills the bitmap exactly — using the raw `DPI/72` leaves a sub-pixel drift).
4. Create a BGRA bitmap, clear it to opaque white.
5. `FPDF_RenderPageBitmapWithMatrix` with a scale matrix and a full-bounds clip
   rect in device coords, `FPDF_ANNOT` flag.
6. Convert the BGRA buffer to `image.NRGBA` (swap R↔B).

### PDFium gotchas (verified against this lib)

These are non-obvious and cost real debugging time — keep them in mind:

- **Clip rect must be the full bitmap `(0,0,w,h)`, not NULL.** A NULL clip
  renders *nothing* (the page comes out blank).
- **`FPDFBitmap_Destroy` takes the handle by value**, not a pointer
  (`void FPDFBitmap_Destroy(FPDF_BITMAP)`). Passing `&handle` intermittently
  crashes (SIGTRAP) because the pointer is treated as a garbage handle.
- **`FPDFPage_GetCropBox`/`GetMediaBox` out-params are `left, bottom, right,
  top`** — not `left, bottom, width, height`. Compute `width = right-left`,
  `height = top-bottom` (they coincide only when the box origin is `(0,0)`).
- **`FPDF_RenderPageBitmapWithMatrix` returns `void`** in this build (not
  `BOOL`).
- The document-close symbol is **`FPDF_CloseDocument`** (older naming), not
  `FPDF_DestroyDocument`.

## Validation

Rendered output was compared pixel-for-pixel against a reference PDFium-based
rasterizer (using the *same* `libpdfium`) at 300 DPI, across a set of
multi-page sample documents. Every page matched to within **≤0.63/255** mean
per-channel difference with **99%+** near-identical pixels; one document
matched exactly (**0.00/255**, 100%).

The residual difference is font anti-aliasing (sub-pixel glyph rendering) and
is immaterial for downstream VLM/OCR use.

## Tests

```sh
go test ./...
```

- `bitmap_test.go` — pure-Go unit tests for the BGRA→NRGBA conversion (channel
  swap, stride/padding). No native library required.
- `renderer_test.go` — integration tests that build a minimal PDF in-memory,
  render it, and assert on size and pixel color. **Skipped** automatically if
  the pdfium library isn't found (set `PDFIUM_LIB` to enable).
- `cmd/folio/main_test.go` — page-range parser tests.

## Licensing

- **folio** (this code): **MIT** — see [LICENSE](LICENSE).
- **PDFium** (`libpdfium.{dylib,so,dll}`): **BSD 3-Clause**. It is *not* part
  of this module — you provide the shared library at runtime (see
  [Library location](#library-location)). Because BSD 3-Clause is permissive it
  is fully compatible with MIT, but when you distribute the library alongside
  folio you must preserve PDFium's copyright notice and license text.
