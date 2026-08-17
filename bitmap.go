package folio

import "image"

// bgraToImage converts a BGRA pixel buffer (row-major, stride bytes per row)
// into an *image.NRGBA. PDFium stores pixels as B,G,R,A; NRGBA is R,G,B,A, so
// the red and blue channels are swapped. stride may exceed width*4 (row
// padding); only the first width*4 bytes of each row are used.
func bgraToImage(buf []byte, width, height, stride int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	dst := img.Pix
	rowBytes := width * 4
	for y := 0; y < height; y++ {
		src := buf[y*stride : y*stride+rowBytes]
		d := dst[y*rowBytes : y*rowBytes+rowBytes]
		for x := 0; x < rowBytes; x += 4 {
			d[x+0] = src[x+2] // R
			d[x+1] = src[x+1] // G
			d[x+2] = src[x+0] // B
			d[x+3] = src[x+3] // A
		}
	}
	return img
}
