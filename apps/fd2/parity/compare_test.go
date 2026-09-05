package parity

import (
	"image"
	"image/color"
	"testing"
)

func TestCompareExactAndSinglePixel(t *testing.T) {
	a := image.NewRGBA(image.Rect(0, 0, Width, Height))
	b := image.NewRGBA(image.Rect(0, 0, Width, Height))
	got, _, err := Compare(a, b)
	if err != nil || got.EqualPixels != Width*Height || got.DiffBox != nil {
		t.Fatalf("exact: got=%+v err=%v", got, err)
	}

	b.SetRGBA(17, 23, color.RGBA{30, 60, 90, 255})
	got, diff, err := Compare(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if got.EqualPixels != Width*Height-1 || got.DiffBox == nil || *got.DiffBox != (Box{17, 23, 17, 23}) {
		t.Fatalf("single pixel: %+v", got)
	}
	if got.MeanAbsRGB <= 0 || diff.RGBAAt(17, 23) != (color.RGBA{255, 0, 255, 255}) {
		t.Fatalf("diff output/result missing: %+v %v", got, diff.RGBAAt(17, 23))
	}
}

func TestCompareRejectsNonCanonicalGeometry(t *testing.T) {
	bad := image.NewRGBA(image.Rect(0, 0, 640, 400))
	good := image.NewRGBA(image.Rect(0, 0, Width, Height))
	if _, _, err := Compare(bad, good); err == nil {
		t.Fatal("expected original geometry error")
	}
	if _, _, err := Compare(good, bad); err == nil {
		t.Fatal("expected remake geometry error")
	}
}
