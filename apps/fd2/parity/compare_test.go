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

func TestCompareRegionUsesAbsoluteCoordinates(t *testing.T) {
	a := image.NewRGBA(image.Rect(0, 0, Width, Height))
	b := image.NewRGBA(image.Rect(0, 0, Width, Height))
	b.SetRGBA(11, 23, color.RGBA{9, 8, 7, 255})
	got, err := CompareRegion(a, b, Region{Name: "dialogue", X: 10, Y: 20, Width: 4, Height: 5})
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalPixels != 20 || got.EqualPixels != 19 || got.DiffBox == nil ||
		*got.DiffBox != (Box{11, 23, 11, 23}) {
		t.Fatalf("region result=%+v", got)
	}
}

func TestCompareRegionRejectsInvalidBounds(t *testing.T) {
	a := image.NewRGBA(image.Rect(0, 0, Width, Height))
	for _, region := range []Region{
		{Name: "", X: 0, Y: 0, Width: 1, Height: 1},
		{Name: "bad", X: 319, Y: 0, Width: 2, Height: 1},
	} {
		if _, err := CompareRegion(a, a, region); err == nil {
			t.Fatalf("expected invalid region error: %+v", region)
		}
	}
}

func TestNormalizeRemakeAcceptsNativeAndExactDouble(t *testing.T) {
	native := image.NewRGBA(image.Rect(0, 0, Width, Height))
	native.SetRGBA(7, 9, color.RGBA{11, 22, 33, 255})
	got, mode, err := NormalizeRemake(native)
	if err != nil || mode != "native_320x200" || got.RGBAAt(7, 9) != (color.RGBA{11, 22, 33, 255}) {
		t.Fatalf("native: mode=%q pixel=%v err=%v", mode, got.RGBAAt(7, 9), err)
	}

	double := image.NewRGBA(image.Rect(0, 0, Width*2, Height*2))
	double.SetRGBA(14, 18, color.RGBA{44, 55, 66, 255})
	double.SetRGBA(15, 18, color.RGBA{200, 200, 200, 255})
	got, mode, err = NormalizeRemake(double)
	if err != nil || mode != "nearest_2x" || got.RGBAAt(7, 9) != (color.RGBA{44, 55, 66, 255}) {
		t.Fatalf("double: mode=%q pixel=%v err=%v", mode, got.RGBAAt(7, 9), err)
	}
}

func TestNormalizeRemakeRejectsOtherGeometryAndNonZeroOrigin(t *testing.T) {
	for _, bounds := range []image.Rectangle{
		image.Rect(0, 0, 960, 600),
		image.Rect(1, 1, Width+1, Height+1),
	} {
		if _, _, err := NormalizeRemake(image.NewRGBA(bounds)); err == nil {
			t.Fatalf("expected geometry error for %v", bounds)
		}
	}
}
