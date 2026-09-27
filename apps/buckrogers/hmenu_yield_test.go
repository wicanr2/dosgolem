package buckrogers

import "testing"

// Spec 028 §3.4 (2026-09-27): the generic menu yields only when another
// family changed an output pixel inside its own mask cells.
func TestHMenuYieldJudgesOwnMaskCells(t *testing.T) {
	r := &LiveRuntime{indexed: make([]byte, 320*200)}
	r.palette[0] = [3]uint8{0, 0, 0}
	base := func(scale int) []byte {
		out := make([]byte, 320*scale*200*scale*4)
		for i := 3; i < len(out); i += 4 {
			out[i] = 255
		}
		return out
	}
	set := func(out []byte, scale, ox, oy int) { out[(oy*320*scale+ox)*4] = 200 }
	page := &HMenuPage{Rows: []HMenuRow{{Row: 24, Col: 18}, {Row: 23, Col: 5}}}
	for _, scale := range []int{2, 3} {
		out := base(scale)
		if r.rowsTouched(out, scale, page) {
			t.Fatalf("%dx: untouched page yielded", scale)
		}
		// Column 17 on row 24 is outside the mask: draw anyway.
		set(out, scale, 17*8*scale+3, 24*8*scale+2)
		if r.rowsTouched(out, scale, page) {
			t.Fatalf("%dx: change left of mask yielded", scale)
		}
		// Column 18 on row 24 is inside.
		out = base(scale)
		set(out, scale, 18*8*scale, 24*8*scale)
		if !r.rowsTouched(out, scale, page) {
			t.Fatalf("%dx: change at start column ignored", scale)
		}
		// Each row uses its own start column: column 6 on row 23 counts,
		// column 6 on row 24 does not.
		out = base(scale)
		set(out, scale, 6*8*scale, 24*8*scale)
		if r.rowsTouched(out, scale, page) {
			t.Fatalf("%dx: row 24 used row 23's start", scale)
		}
		set(out, scale, 6*8*scale, 23*8*scale)
		if !r.rowsTouched(out, scale, page) {
			t.Fatalf("%dx: row 23 change ignored", scale)
		}
	}
	// 3x: a single pixel off the sampling grid counts.
	out := base(3)
	set(out, 3, 20*8*3+1, 24*8*3+1)
	if !r.rowsTouched(out, 3, page) {
		t.Fatal("3x off-grid pixel ignored")
	}
}
