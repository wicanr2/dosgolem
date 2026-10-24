package realspace

import "testing"

func TestTransformFractionBoundary(t *testing.T) {
	for _, tc := range []struct{ sum, want int32 }{{-257, -2}, {-256, -1}, {-255, -1}, {-128, -1}, {-1, -1}, {0, 0}, {127, 0}, {128, 0}, {255, 0}, {256, 1}} {
		var m [12]int32
		m[0] = 1
		got := Transform([3]int32{tc.sum, 0, 0}, m)
		if got[0] != tc.want || got[1] != 0 || got[2] != 0 {
			t.Fatalf("sum=%d XYZ=%v want [%d,0,0]", tc.sum, got, tc.want)
		}
	}
}
