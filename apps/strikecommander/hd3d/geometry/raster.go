package geometry

import (
	"fmt"
	"image"
	"math"
	"math/big"
	"math/bits"
)

func NewTarget(n int) (*Target, error) {
	if !validScale(n) {
		return nil, fmt.Errorf("geometry: scale must be 2/3")
	}
	w, h := 320*n, 200*n
	return &Target{frame: Frame{Scale: n, Width: w, Height: h, Owner: make([]FaceRef, w*h), DepthQ48: make([]uint64, w*h), Wire: image.NewRGBA(image.Rect(0, 0, w, h))}}, nil
}
func (t *Target) Clear() {
	clear(t.frame.Owner)
	clear(t.frame.DepthQ48)
	clear(t.frame.Wire.Pix)
	t.frame.Stats = Stats{}
}
func (t *Target) Frame() Frame { return t.frame }

type point64 struct{ x, y int64 }
type pointBig struct{ x, y *big.Int }

func edge64(a, b, p point64) int64 { return (b.x-a.x)*(p.y-a.y) - (b.y-a.y)*(p.x-a.x) }
func left64(a, b point64) bool     { return b.y < a.y || (b.y == a.y && b.x > a.x) }

// roundRat rounds an exact rational to its nearest even integer.
func roundRat(r *big.Rat) *big.Int {
	neg := r.Sign() < 0
	n := new(big.Int).Abs(r.Num())
	q, rem := new(big.Int), new(big.Int)
	q.QuoRem(n, r.Denom(), rem)
	twice := new(big.Int).Lsh(rem, 1)
	cmp := twice.Cmp(r.Denom())
	if cmp > 0 || (cmp == 0 && q.Bit(0) != 0) {
		q.Add(q, big.NewInt(1))
	}
	if neg {
		q.Neg(q)
	}
	return q
}
func quantize(x float64) *big.Int {
	r := new(big.Rat).SetFloat64(x)
	r.Mul(r, big.NewRat(256, 1))
	return roundRat(r)
}
func edgeBig(a, b, p pointBig) *big.Int {
	dx := new(big.Int).Sub(b.x, a.x)
	dy := new(big.Int).Sub(b.y, a.y)
	px := new(big.Int).Sub(p.x, a.x)
	py := new(big.Int).Sub(p.y, a.y)
	return new(big.Int).Sub(new(big.Int).Mul(dx, py), new(big.Int).Mul(dy, px))
}
func leftBig(a, b pointBig) bool { y := b.y.Cmp(a.y); return y < 0 || (y == 0 && b.x.Cmp(a.x) > 0) }
func boundBig(q [3]pointBig, axis bool, limit int) (int, int) {
	get := func(p pointBig) *big.Int {
		if axis {
			return p.y
		}
		return p.x
	}
	lo, hi := new(big.Int).Set(get(q[0])), new(big.Int).Set(get(q[0]))
	for _, p := range q[1:] {
		v := get(p)
		if v.Cmp(lo) < 0 {
			lo.Set(v)
		}
		if v.Cmp(hi) > 0 {
			hi.Set(v)
		}
	}
	// ceil((lo-128)/256), floor((hi-128)/256), using Euclidean floor.
	lo.Sub(lo, big.NewInt(128))
	lo.Neg(lo)
	lo.Div(lo, big.NewInt(256))
	lo.Neg(lo)
	hi.Sub(hi, big.NewInt(128))
	hi.Div(hi, big.NewInt(256))
	zero, max := big.NewInt(0), big.NewInt(int64(limit-1))
	if lo.Cmp(zero) < 0 {
		lo.Set(zero)
	}
	if hi.Cmp(max) > 0 {
		hi.Set(max)
	}
	if lo.Cmp(max) > 0 || hi.Sign() < 0 || lo.Cmp(hi) > 0 {
		return 0, -1
	}
	return int(lo.Int64()), int(hi.Int64())
}

func (t *Target) Draw(tri Triangle) error { return t.draw(tri, false) }
func (t *Target) draw(tri Triangle, forceBig bool) error {
	fast := !forceBig
	for _, v := range tri.Vertices {
		if !finite(v.NX) || !finite(v.NY) || !finite(v.QX) || !finite(v.QY) || !finite(v.UX) || !finite(v.UY) || v.Z <= 0 || v.InvZQ48 != (uint64(1)<<48)/uint64(v.Z) {
			return fmt.Errorf("geometry: invalid projected vertex")
		}
		if math.Abs(v.NX) > 1<<21 || math.Abs(v.NY) > 1<<21 {
			fast = false
		}
	}
	var qb [3]pointBig
	for i, v := range tri.Vertices {
		if fast {
			qb[i] = pointBig{big.NewInt(int64(math.RoundToEven(float64(v.NX * 256)))), big.NewInt(int64(math.RoundToEven(float64(v.NY * 256))))}
		} else {
			qb[i] = pointBig{quantize(v.NX), quantize(v.NY)}
		}
	}
	// A Q8-degenerate fan writes neither coverage nor diagnostic wire.
	degenerate := false
	if fast {
		q := [3]point64{}
		for i, p := range qb {
			q[i] = point64{p.x.Int64(), p.y.Int64()}
		}
		degenerate = edge64(q[0], q[1], q[2]) == 0
	} else {
		degenerate = edgeBig(qb[0], qb[1], qb[2]).Sign() == 0
	}
	if degenerate {
		if fast {
			t.frame.Stats.FastTriangles++
		} else {
			t.frame.Stats.BigTriangles++
		}
		t.frame.Stats.DegenerateTriangles++
		return nil
	}
	// Wire uses the original Q8 edges, including each fan's internal diagonal.
	for i := 0; i < 3; i++ {
		t.wire(qb[i], qb[(i+1)%3], tri.WireColor)
		t.frame.Stats.WireEdges++
	}
	if fast {
		var q [3]point64
		for i, p := range qb {
			q[i] = point64{p.x.Int64(), p.y.Int64()}
		}
		t.frame.Stats.FastTriangles++
		t.rasterFast(q, tri)
	} else {
		t.frame.Stats.BigTriangles++
		t.rasterBig(qb, tri)
	}
	return nil
}
func (t *Target) write(i int, d uint64, ref FaceRef) {
	if d > t.frame.DepthQ48[i] {
		t.frame.DepthQ48[i] = d
		t.frame.Owner[i] = ref
		t.frame.Stats.WrittenSamples++
	}
}
func depth128(e [3]int64, z [3]uint64, area uint64) uint64 {
	var hi, lo uint64
	for i, v := range e {
		a, b := bits.Mul64(uint64(v), z[i])
		var carry uint64
		lo, carry = bits.Add64(lo, b, 0)
		hi, _ = bits.Add64(hi, a, carry)
	}
	q, _ := bits.Div64(hi, lo, area)
	return q
}
func (t *Target) rasterFast(q [3]point64, tri Triangle) {
	area := edge64(q[0], q[1], q[2])
	if area == 0 {
		t.frame.Stats.DegenerateTriangles++
		return
	}
	z := [3]uint64{tri.Vertices[0].InvZQ48, tri.Vertices[1].InvZQ48, tri.Vertices[2].InvZQ48}
	if area < 0 {
		q[1], q[2] = q[2], q[1]
		z[1], z[2] = z[2], z[1]
		area = -area
	}
	xmin, xmax, ymin, ymax := q[0].x, q[0].x, q[0].y, q[0].y
	for _, p := range q[1:] {
		xmin = min(xmin, p.x)
		xmax = max(xmax, p.x)
		ymin = min(ymin, p.y)
		ymax = max(ymax, p.y)
	}
	ceil := func(x int64) int64 { return -((-x) >> 8) }
	x0, x1 := max(int64(0), ceil(xmin-128)), min(int64(t.frame.Width-1), (xmax-128)>>8)
	y0, y1 := max(int64(0), ceil(ymin-128)), min(int64(t.frame.Height-1), (ymax-128)>>8)
	incl := [3]bool{left64(q[1], q[2]), left64(q[2], q[0]), left64(q[0], q[1])}
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			p := point64{256*x + 128, 256*y + 128}
			e := [3]int64{edge64(q[1], q[2], p), edge64(q[2], q[0], p), edge64(q[0], q[1], p)}
			inside := true
			for i, v := range e {
				if v < 0 || (v == 0 && !incl[i]) {
					inside = false
					break
				}
			}
			if inside {
				t.write(int(y)*t.frame.Width+int(x), depth128(e, z, uint64(area)), tri.Ref)
			}
		}
	}
}
func (t *Target) rasterBig(q [3]pointBig, tri Triangle) {
	area := edgeBig(q[0], q[1], q[2])
	if area.Sign() == 0 {
		t.frame.Stats.DegenerateTriangles++
		return
	}
	z := [3]uint64{tri.Vertices[0].InvZQ48, tri.Vertices[1].InvZQ48, tri.Vertices[2].InvZQ48}
	if area.Sign() < 0 {
		q[1], q[2] = q[2], q[1]
		z[1], z[2] = z[2], z[1]
		area.Neg(area)
	}
	x0, x1 := boundBig(q, false, t.frame.Width)
	y0, y1 := boundBig(q, true, t.frame.Height)
	incl := [3]bool{leftBig(q[1], q[2]), leftBig(q[2], q[0]), leftBig(q[0], q[1])}
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			p := pointBig{big.NewInt(int64(256*x + 128)), big.NewInt(int64(256*y + 128))}
			e := [3]*big.Int{edgeBig(q[1], q[2], p), edgeBig(q[2], q[0], p), edgeBig(q[0], q[1], p)}
			inside := true
			for i, v := range e {
				if v.Sign() < 0 || (v.Sign() == 0 && !incl[i]) {
					inside = false
					break
				}
			}
			if inside {
				sum := new(big.Int)
				for i, v := range e {
					sum.Add(sum, new(big.Int).Mul(v, new(big.Int).SetUint64(z[i])))
				}
				sum.Quo(sum, area)
				t.write(y*t.frame.Width+x, sum.Uint64(), tri.Ref)
			}
		}
	}
}
