package folio

import (
	"image"
	"image/color"
	"testing"
)

// TestBGRAToImage verifies the B,G,R,A -> R,G,B,A channel swap and stride
// handling with a known buffer.
func TestBGRAToImage(t *testing.T) {
	const w, h = 2, 2
	// Two rows of BGRA. Row 0: pixel(0,0)=blue, pixel(1,0)=red.
	// Row 1: pixel(0,1)=green, pixel(1,1)=white.
	// BGRA bytes: B G R A
	buf := []byte{
		0xFF, 0x00, 0x00, 0xFF, // blue   (B=FF)
		0x00, 0x00, 0xFF, 0xFF, // red    (R=FF)
		0x00, 0xFF, 0x00, 0xFF, // green  (G=FF)
		0xFF, 0xFF, 0xFF, 0xFF, // white
	}
	img := bgraToImage(buf, w, h, w*4)

	want := map[image.Point]color.Color{
		{0, 0}: color.RGBA{R: 0x00, G: 0x00, B: 0xFF, A: 0xFF}, // blue
		{1, 0}: color.RGBA{R: 0xFF, G: 0x00, B: 0x00, A: 0xFF}, // red
		{0, 1}: color.RGBA{R: 0x00, G: 0xFF, B: 0x00, A: 0xFF}, // green
		{1, 1}: color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}, // white
	}
	for p, c := range want {
		if got := img.At(p.X, p.Y); !colorEqual(got, c) {
			t.Errorf("pixel %v = %v, want %v", p, got, c)
		}
	}
}

// TestBGRAToImage_Stride verifies that row padding (stride > width*4) is
// skipped correctly.
func TestBGRAToImage_Stride(t *testing.T) {
	const w, h = 1, 2
	stride := 8 // 2x the row width (4 bytes) -> 4 bytes of padding per row
	buf := make([]byte, stride*h)
	buf[0], buf[1], buf[2], buf[3] = 0x00, 0x00, 0xFF, 0xFF                             // red
	buf[stride+0], buf[stride+1], buf[stride+2], buf[stride+3] = 0xFF, 0x00, 0x00, 0xFF // blue

	img := bgraToImage(buf, w, h, stride)
	if got := img.At(0, 0); !colorEqual(got, color.RGBA{0xFF, 0x00, 0x00, 0xFF}) {
		t.Errorf("pixel(0,0) = %v, want red", got)
	}
	if got := img.At(0, 1); !colorEqual(got, color.RGBA{0x00, 0x00, 0xFF, 0xFF}) {
		t.Errorf("pixel(0,1) = %v, want blue", got)
	}
}

func colorEqual(a, b color.Color) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}
