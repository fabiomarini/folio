package folio

import "testing"

func TestMeaningfulFromCount(t *testing.T) {
	cases := []struct {
		n    int
		want bool
	}{
		{0, false},
		{3, false}, // watermark / page number / stray glyph
		{9, false}, // below threshold
		{minTextChars, true},
		{1000, true},
	}
	for _, c := range cases {
		if got := meaningfulFromCount(c.n); got != c.want {
			t.Errorf("meaningfulFromCount(%d) = %v, want %v", c.n, got, c.want)
		}
	}
}
