package geometry

import (
	"bytes"
	"image/color"
	"math"
	"math/big"
	"reflect"
	"testing"
)

func TestTransformOracle(t *testing.T) {
	cases := []struct {
		v ModelPoint
		m Matrix
	}{{ModelPoint{-1, 2, 3}, Matrix{1, 0, 0, 0, 0, 256, 0, 0, 0, 0, 256, 0}}, {ModelPoint{math.MaxInt32, math.MinInt32, math.MaxInt32}, Matrix{math.MaxInt32, math.MinInt32, math.MaxInt32, math.MaxInt32, math.MinInt32, math.MinInt32, math.MinInt32, 0, 256, 0, 0, math.MaxInt32}}}
	for _, tc := range cases {
		a := [3]int32{tc.v.X, tc.v.Y, tc.v.Z}
		var expected [3]int32
		for r := 0; r < 3; r++ {
			sum := new(big.Int)
			for j := 0; j < 3; j++ {
				sum.Add(sum, new(big.Int).Mul(big.NewInt(int64(a[j])), big.NewInt(int64(tc.m[r*4+j]))))
			}
			mod := new(big.Int).Lsh(big.NewInt(1), 64)
			sum.Mod(sum, mod)
			if sum.Bit(63) != 0 {
				sum.Sub(sum, mod)
			}
			sum.Rsh(sum, 8)
			sum.Add(sum, big.NewInt(int64(tc.m[r*4+3])))
			expected[r] = int32(sum.Int64())
		}
		if got := Transform(tc.v, tc.m); got != (Point3{expected[0], expected[1], expected[2]}) {
			t.Fatal(got, expected)
		}
	}
}
func oracleIntersection(i, o Point3, near int32) Point3 {
	result := Point3{Z: near}
	for axis := 0; axis < 2; axis++ {
		a, b := i.X, o.X
		if axis == 1 {
			a, b = i.Y, o.Y
		}
		num := new(big.Int).Mul(big.NewInt(int64(a)-int64(b)), big.NewInt(int64(i.Z)-int64(near)))
		num.Quo(num, big.NewInt(int64(i.Z)-int64(o.Z)))
		num.Sub(big.NewInt(int64(a)), num)
		if axis == 0 {
			result.X = int32(num.Int64())
		} else {
			result.Y = int32(num.Int64())
		}
	}
	return result
}
func TestNear(t *testing.T) {
	cam := Camera{FX: 256, FY: 256}
	cases := []struct {
		p    [3]Point3
		want []Point3
	}{
		{[3]Point3{{0, 0, 0}, {4, 4, 0}, {0, 4, 4}}, []Point3{{2, 2, 0}, {4, 4, 0}, {0, 4, 4}, {0, 2, 2}}}, // Pa-first, P0 outside
		{[3]Point3{{0, 4, 0}, {4, 0, 0}, {0, 0, 4}}, []Point3{{0, 4, 0}, {2, 2, 0}, {0, 2, 2}}},
		{[3]Point3{{0, 0, 0}, {1, 1, 0}, {0, 1, 1}}, nil},
		{[3]Point3{{0, 2, 0}, {4, 4, 0}, {0, 4, 4}}, []Point3{{0, 2, 0}, {4, 4, 0}, {0, 4, 4}}},
	}
	for _, tc := range cases {
		tris, info, err := ProjectFace(FaceInput{Vertices: tc.p, Camera: cam, Near: 2}, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(tc.want) == 0 {
			if !info.AllClipped || len(tris) != 0 {
				t.Fatal(info, tris)
			}
			continue
		}
		if len(tris) != len(tc.want)-2 {
			t.Fatal(len(tris), tc.want)
		}
		for fan, tr := range tris {
			indices := [3]int{0, fan + 1, fan + 2}
			for j, idx := range indices {
				p := tc.want[idx]
				v := tr.Vertices[j]
				qx := float64(p.X) / float64(p.Z)
				qy := 1 - float64(p.Y)/float64(p.Z)
				if v.QX != qx || v.QY != qy || v.Z != p.Z {
					t.Fatal("Pa-first/fan", v, p)
				}
			}
		}
	}
	for _, pair := range [][2]Point3{{{-5, 3, 0}, {0, 0, 0}}, {{math.MaxInt32, math.MaxInt32, math.MinInt32}, {math.MinInt32, math.MinInt32, math.MaxInt32}}} {
		near := int32(1)
		want := oracleIntersection(pair[0], pair[1], near)
		if a, b := nearIntersection(pair[0], pair[1], near), nearIntersection(pair[1], pair[0], near); a != want || b != want {
			t.Fatal(a, b, want)
		}
	}
	f := FaceInput{Vertices: [3]Point3{{0, 1, 0}, {4, 4, 0}, {0, 4, 4}}, Camera: cam, Flag17BC: 1, Near: 2}
	ts, info, err := ProjectFace(f, 2)
	if err != nil || len(ts) != 1 || info.Clipped || info.NearUnexpectedVertices != 1 {
		t.Fatal("K18", ts, info, err)
	}
	f.Flag17BC = 0
	ts, info, err = ProjectFace(f, 2)
	if err != nil || len(ts) != 2 || !info.Clipped {
		t.Fatal(ts, info, err)
	}
	f.Near = 0
	if _, _, err = ProjectFace(f, 2); err == nil {
		t.Fatal("invalid Near accepted")
	}
}
func TestProjection(t *testing.T) {
	uy := float64(float64(1) / float64(3))
	wantY := float64(float64(1) - uy)
	for _, n := range []int{2, 3} {
		p, err := ProjectPoint(Point3{1, 3, 1}, Camera{FX: 256, FY: 256}, n)
		if err != nil || p.QY != wantY || p.NY != float64(n)*p.QY || p.InvZQ48 != (1<<48)/3 {
			t.Fatal("K9/scale", p, err)
		}
	}
	for _, z := range []int32{0, -1} {
		if _, err := ProjectPoint(Point3{Z: z}, Camera{}, 2); err == nil {
			t.Fatal("Z accepted")
		}
	}
	if _, err := ProjectPoint(Point3{Z: 1}, Camera{}, 1); err == nil {
		t.Fatal("N accepted")
	}
}

func projected(x, y float64, z int32) Projection {
	return Projection{NX: x, NY: y, Z: z, InvZQ48: (1 << 48) / uint64(z)}
}
func smallTriangle() Triangle {
	return Triangle{Vertices: [3]Projection{projected(.25, .25, 3), projected(5.25, .25, 5), projected(.25, 5.25, 7)}, Ref: FaceRef{Object: 1, Face: 2, Clamped: true}, WireColor: color.RGBA{255, 10, 20, 255}}
}

// Oracle independently rounds with floor+fraction, then uses original Q8 edges.
// No production quantizer, edge, viewport, or depth helper is used.
func oracleQ(x float64) *big.Int {
	r := new(big.Rat).SetFloat64(x)
	r.Mul(r, big.NewRat(256, 1))
	q := new(big.Int).Div(r.Num(), r.Denom())
	frac := new(big.Rat).Sub(r, new(big.Rat).SetInt(q))
	cmp := frac.Cmp(big.NewRat(1, 2))
	if cmp > 0 || (cmp == 0 && q.Bit(0) != 0) {
		q.Add(q, big.NewInt(1))
	}
	return q
}

type oraclePoint struct{ x, y *big.Int }

func oracleEdge(a, b, p oraclePoint) *big.Int {
	dx := new(big.Int).Sub(b.x, a.x)
	dy := new(big.Int).Sub(b.y, a.y)
	px := new(big.Int).Sub(p.x, a.x)
	py := new(big.Int).Sub(p.y, a.y)
	return new(big.Int).Sub(new(big.Int).Mul(dx, py), new(big.Int).Mul(dy, px))
}
func oracleAt(tri Triangle, x, y int) (uint64, bool) {
	var q [3]oraclePoint
	var d [3]*big.Int
	for i, v := range tri.Vertices {
		q[i] = oraclePoint{oracleQ(v.NX), oracleQ(v.NY)}
		d[i] = new(big.Int).Quo(new(big.Int).Lsh(big.NewInt(1), 48), big.NewInt(int64(v.Z)))
	}
	area := oracleEdge(q[0], q[1], q[2])
	if area.Sign() == 0 {
		return 0, false
	}
	if area.Sign() < 0 {
		q[1], q[2] = q[2], q[1]
		d[1], d[2] = d[2], d[1]
		area.Neg(area)
	}
	p := oraclePoint{big.NewInt(int64(x*256 + 128)), big.NewInt(int64(y*256 + 128))}
	num := new(big.Int)
	for i := 0; i < 3; i++ {
		a, b := q[(i+1)%3], q[(i+2)%3]
		e := oracleEdge(a, b, p)
		cmp := b.y.Cmp(a.y)
		left := cmp < 0 || (cmp == 0 && b.x.Cmp(a.x) > 0)
		if e.Sign() < 0 || (e.Sign() == 0 && !left) {
			return 0, false
		}
		num.Add(num, new(big.Int).Mul(e, d[i]))
	}
	num.Quo(num, area)
	return num.Uint64(), true
}
func TestRasterOracleAndPaths(t *testing.T) {
	for _, n := range []int{2, 3} {
		for _, tc := range []Triangle{smallTriangle(), {Vertices: [3]Projection{projected(-2097152, 0, 1), projected(2097152, 0, 2), projected(0, 3, 3)}}, {Vertices: [3]Projection{projected(2097152+0.25, 0, 1), projected(2097152-0.25, 0, 2), projected(0, 3, 3)}}} {
			a, _ := NewTarget(n)
			b, _ := NewTarget(n)
			if err := a.Draw(tc); err != nil {
				t.Fatal(err)
			}
			if err := b.draw(tc, true); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(a.frame.DepthQ48, b.frame.DepthQ48) || !reflect.DeepEqual(a.frame.Owner, b.frame.Owner) || !bytes.Equal(a.frame.Wire.Pix, b.frame.Wire.Pix) {
				t.Fatal("fast/Big mismatch")
			}
			// The +/-G adjacent cases have independently known safe int64
			// determinants even just outside the guard; force the fast primitive
			// as well, without adding a production path-selection option.
			manual, _ := NewTarget(n)
			var qm [3]point64
			for i, v := range tc.Vertices {
				qm[i] = point64{oracleQ(v.NX).Int64(), oracleQ(v.NY).Int64()}
			}
			manual.rasterFast(qm, tc)
			if !reflect.DeepEqual(a.frame.DepthQ48, manual.frame.DepthQ48) || !reflect.DeepEqual(a.frame.Owner, manual.frame.Owner) {
				t.Fatal("guard-adjacent forced fast/Big")
			}
			for y := 0; y < 8; y++ {
				for x := 0; x < 12; x++ {
					d, in := oracleAt(tc, x, y)
					i := y*a.frame.Width + x
					if a.frame.DepthQ48[i] != d || (a.frame.DepthQ48[i] != 0) != in {
						t.Fatalf("oracle %d,%d: %d/%d", x, y, a.frame.DepthQ48[i], d)
					}
				}
			}
		}
	}
}
func TestBigOriginalEdges(t *testing.T) {
	// Legal X/fx, Z=1, N=3 makes Q8 larger than int64. The independent
	// integer oracle retains every original Q8 edge and its barycentric depth.
	for _, n := range []int{2, 3} {
		tri := Triangle{Ref: FaceRef{Object: 7}}
		cam := Camera{FX: math.MaxInt32, FY: math.MaxInt32}
		points := [3]Point3{{math.MinInt32, 1, math.MaxInt32}, {math.MaxInt32, 2, math.MaxInt32}, {0, 1, math.MinInt32}}
		for i, p := range points {
			v, err := ProjectPoint(p, cam, n)
			if err != nil {
				t.Fatal(err)
			}
			tri.Vertices[i] = v
		}
		target, _ := NewTarget(n)
		if err := target.Draw(tri); err != nil {
			t.Fatal(err)
		}
		// Full coverage/depth at every output subpixel, using a separately prepared
		// oracle below to avoid repeating quantization per pixel.
		checkFullOracle(t, tri, target.frame)
		if target.frame.Stats.BigTriangles != 1 {
			t.Fatal("K26: big path not selected")
		}
	}
}
func checkFullOracle(t *testing.T, tri Triangle, f Frame) {
	var q [3]oraclePoint
	var depths [3]*big.Int
	for i, v := range tri.Vertices {
		q[i] = oraclePoint{oracleQ(v.NX), oracleQ(v.NY)}
		depths[i] = new(big.Int).Quo(new(big.Int).Lsh(big.NewInt(1), 48), big.NewInt(int64(v.Z)))
	}
	area := oracleEdge(q[0], q[1], q[2])
	if area.Sign() < 0 {
		q[1], q[2] = q[2], q[1]
		depths[1], depths[2] = depths[2], depths[1]
		area.Neg(area)
	}
	for y := 0; y < f.Height; y++ {
		for x := 0; x < f.Width; x++ {
			p := oraclePoint{big.NewInt(int64(x*256 + 128)), big.NewInt(int64(y*256 + 128))}
			sum := new(big.Int)
			in := true
			for i := 0; i < 3; i++ {
				a, b := q[(i+1)%3], q[(i+2)%3]
				e := oracleEdge(a, b, p)
				cmp := b.y.Cmp(a.y)
				incl := cmp < 0 || (cmp == 0 && b.x.Cmp(a.x) > 0)
				if e.Sign() < 0 || (e.Sign() == 0 && !incl) {
					in = false
					break
				}
				sum.Add(sum, new(big.Int).Mul(e, depths[i]))
			}
			d := uint64(0)
			if in {
				sum.Quo(sum, area)
				d = sum.Uint64()
			}
			if got := f.DepthQ48[y*f.Width+x]; got != d {
				t.Fatalf("full oracle %d,%d got %d want %d", x, y, got, d)
			}
		}
	}
}
func TestTieWindingDegenerateAndView(t *testing.T) {
	a, _ := NewTarget(2)
	tr := smallTriangle()
	tr.Vertices = [3]Projection{projected(0, 0, 16777216), projected(4, 0, 16777216), projected(0, 4, 16777216)}
	if err := a.Draw(tr); err != nil {
		t.Fatal(err)
	}
	view := a.Frame()
	first := view.Owner[0]
	tr.Ref.Object = 9
	a.Draw(tr)
	if a.frame.Owner[0] != first {
		t.Fatal("equal tie overwritten")
	}
	for i := range tr.Vertices {
		tr.Vertices[i].Z = 16777215
		tr.Vertices[i].InvZQ48 = (1 << 48) / 16777215
	}
	a.Draw(tr)
	if a.frame.Owner[0].Object != 9 || a.frame.DepthQ48[0] != 16777217 {
		t.Fatal("depth difference one")
	}
	b, _ := NewTarget(2)
	tr.Vertices[1], tr.Vertices[2] = tr.Vertices[2], tr.Vertices[1]
	b.Draw(tr)
	if !reflect.DeepEqual(a.frame.DepthQ48, b.frame.DepthQ48) {
		t.Fatal("winding")
	}
	a.Clear()
	if view.DepthQ48[0] != 0 || a.Frame().Stats != (Stats{}) {
		t.Fatal("view/Clear")
	}
	tr.Vertices[2] = tr.Vertices[1]
	a.Draw(tr)
	if a.frame.Stats.DegenerateTriangles != 1 || a.frame.Stats.WrittenSamples != 0 {
		t.Fatal("degenerate")
	}
	for _, v := range a.frame.Wire.Pix {
		if v != 0 {
			t.Fatal("degenerate wire")
		}
	}
	bad := tr
	bad.Vertices[0].NX = math.Inf(1)
	if a.Draw(bad) == nil {
		t.Fatal("nonfinite accepted")
	}
}
func TestWireCanonical(t *testing.T) {
	pairs := [][2]pointBig{{{big.NewInt(-256), big.NewInt(128)}, {big.NewInt(2048), big.NewInt(1152)}}, {{big.NewInt(128), big.NewInt(128)}, {big.NewInt(128), big.NewInt(128)}}, {{big.NewInt(-999999999), big.NewInt(-999999999)}, {big.NewInt(999999999), big.NewInt(999999999)}}}
	for _, p := range pairs {
		a, _ := NewTarget(2)
		b, _ := NewTarget(2)
		c := color.RGBA{255, 255, 255, 255}
		a.wire(p[0], p[1], c)
		b.wire(p[1], p[0], c)
		if !bytes.Equal(a.frame.Wire.Pix, b.frame.Wire.Pix) {
			t.Fatal("wire reverse")
		}
	}
	a, _ := NewTarget(2)
	a.wire(pointBig{big.NewInt(128), big.NewInt(128)}, pointBig{big.NewInt(896), big.NewInt(640)}, color.RGBA{1, 2, 3, 255})
	want := [][2]int{{0, 0}, {1, 1}, {2, 1}, {3, 2}}
	seen := 0
	for y := 0; y < 4; y++ {
		for x := 0; x < 5; x++ {
			if a.frame.Wire.RGBAAt(x, y).A != 0 {
				seen++
				found := false
				for _, p := range want {
					if p == [2]int{x, y} {
						found = true
					}
				}
				if !found {
					t.Fatal("Bresenham", x, y)
				}
			}
		}
	}
	if seen != len(want) {
		t.Fatal(seen)
	}
}
func BenchmarkFast(b *testing.B) {
	tr := smallTriangle()
	t, _ := NewTarget(2)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		t.Draw(tr)
	}
}
func BenchmarkBig(b *testing.B) {
	tr := smallTriangle()
	t, _ := NewTarget(2)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		t.draw(tr, true)
	}
}
