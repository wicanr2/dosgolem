// Package opaque implements 013a's pure, offline material core. It never reads
// the machine or a live palette, and cannot publish a player presentation.
package opaque

import (
	"fmt"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"math/big"
	"sort"
)

const WeightDenominator uint32 = 1 << 24

type SampleColor struct {
	Lo, Hi uint8
	W24    uint32
}
type Band struct{ Base, Unknown, Range uint16 }

func (b Band) valid() bool            { return b.Base <= 255 && b.Range <= 255-b.Base }
func integerColor(v byte) SampleColor { return SampleColor{v, v, 0} }
func floorRat(r *big.Rat) *big.Int {
	q, rem := new(big.Int), new(big.Int)
	q.QuoRem(r.Num(), r.Denom(), rem)
	if r.Sign() < 0 && rem.Sign() != 0 {
		q.Sub(q, big.NewInt(1))
	}
	return q
}
func roundEven(n, d *big.Int) *big.Int {
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(n, d, r)
	cmp := new(big.Int).Lsh(new(big.Int).Abs(r), 1).Cmp(d)
	if cmp > 0 || cmp == 0 && q.Bit(0) == 1 {
		if n.Sign() < 0 {
			q.Sub(q, big.NewInt(1))
		} else {
			q.Add(q, big.NewInt(1))
		}
	}
	return q
}
func bounded(r *big.Rat) error {
	if r.Num().BitLen() > 512 || r.Denom().BitLen() > 512 {
		return fmt.Errorf("material rational exceeds 512 bits")
	}
	return nil
}
func weight24(f *big.Rat) (uint32, error) {
	if err := bounded(f); err != nil {
		return 0, err
	}
	if f.Sign() < 0 || f.Cmp(big.NewRat(1, 1)) > 0 {
		return 0, fmt.Errorf("weight outside unit interval")
	}
	n := new(big.Int).Mul(f.Num(), new(big.Int).SetUint64(uint64(WeightDenominator)))
	q := roundEven(n, f.Denom())
	return uint32(q.Uint64()), nil
}
func r3Color(c *big.Rat, b Band, x, y, i, j int) (SampleColor, error) {
	if !b.valid() || i < 0 || i > 1 || j < 0 || j > 1 {
		return SampleColor{}, fmt.Errorf("invalid R3 band or phase")
	}
	if err := bounded(c); err != nil {
		return SampleColor{}, err
	}
	low, high := big.NewRat(int64(b.Base), 1), big.NewRat(int64(b.Base+b.Range), 1)
	if c.Cmp(low) < 0 || c.Cmp(high) > 0 {
		return SampleColor{}, fmt.Errorf("R3 source outside band")
	}
	t := new(big.Rat).Set(c)
	if ((2*x+i)^(2*y+j))&1 != 0 {
		t.Add(t, big.NewRat(1, 2))
	}
	if t.Cmp(high) > 0 {
		t.Set(high)
	}
	lo := floorRat(t).Uint64()
	hi := lo + 1
	if hi > uint64(b.Base+b.Range) {
		hi = lo
	}
	if hi == lo {
		return integerColor(byte(lo)), nil
	}
	f := new(big.Rat).Sub(t, new(big.Rat).SetInt(new(big.Int).SetUint64(lo)))
	w, err := weight24(f)
	if err != nil {
		return SampleColor{}, err
	}
	if w == 0 {
		return integerColor(byte(lo)), nil
	}
	if w == WeightDenominator {
		return integerColor(byte(hi)), nil
	}
	return SampleColor{byte(lo), byte(hi), w}, nil
}
func r1Color(c hd3d.Call) (SampleColor, error) {
	if c.Params[1] != 3 || len(c.Verts) != 66 {
		return SampleColor{}, fmt.Errorf("R1 requires three captured vertices")
	}
	return integerColor(c.Verts[4]), nil
}

// A compiled capture owns sorted, disjoint spans. Conflicts are rejected even
// if a conflicting address happens not to be sampled in this triangle.
type segment struct{ spans []hd3d.Span }

func compileCapture(c hd3d.Capture) (segment, error) {
	var bytes [65536]byte
	var seen [65536]bool
	total := 0
	for _, s := range c.Spans {
		total += len(s.Data)
		if total > 16<<20 {
			return segment{}, fmt.Errorf("capture size limit")
		}
		for i, b := range s.Data {
			a := uint16(uint64(s.Off) + uint64(i))
			if seen[a] && bytes[a] != b {
				return segment{}, fmt.Errorf("conflicting captured address %04x", a)
			}
			seen[a] = true
			bytes[a] = b
		}
	}
	out := segment{}
	for i := 0; i < 65536; {
		if !seen[i] {
			i++
			continue
		}
		a := i
		for i < 65536 && seen[i] {
			i++
		}
		out.spans = append(out.spans, hd3d.Span{Off: uint16(a), Data: append([]byte(nil), bytes[a:i]...)})
	}
	return out, nil
}
func (s segment) read(a uint16) (byte, error) {
	i := sort.Search(len(s.spans), func(i int) bool { return uint32(s.spans[i].Off)+uint32(len(s.spans[i].Data)) > uint32(a) })
	if i == len(s.spans) || a < s.spans[i].Off {
		return 0, fmt.Errorf("uncaptured address %04x", a)
	}
	return s.spans[i].Data[int(a)-int(s.spans[i].Off)], nil
}

type texture struct {
	pixels, shade           segment
	texOff, pitch, shadeOff uint16
	shadeOn                 bool
}

func compileTexture(c hd3d.Call) (texture, error) {
	p, e := compileCapture(c.Tex.Pixels)
	if e != nil {
		return texture{}, e
	}
	s, e := compileCapture(c.Tex.Shade)
	if e != nil {
		return texture{}, e
	}
	return texture{p, s, c.Params[4], c.Params[6], c.Params[7], c.Params[7] != 0 || c.Params[8] != 0}, nil
}
func (t texture) color(u, v *big.Rat) (SampleColor, bool, error) {
	if err := bounded(u); err != nil {
		return SampleColor{}, false, err
	}
	if err := bounded(v); err != nil {
		return SampleColor{}, false, err
	}
	// Unsigned source UV and nonnegative source/barycentric weights never become
	// negative; rejecting it exposes a malformed binding rather than guessing wrap.
	if u.Sign() < 0 || v.Sign() < 0 {
		return SampleColor{}, false, fmt.Errorf("negative interpolated UV")
	}
	ui := floorRat(new(big.Rat).Quo(u, big.NewRat(256, 1)))
	vi := floorRat(new(big.Rat).Quo(v, big.NewRat(256, 1)))
	off := uint16(uint64(t.texOff) + vi.Uint64()*uint64(t.pitch) + ui.Uint64())
	return t.colorAddress(off)
}
func (t texture) colorAddress(off uint16) (SampleColor, bool, error) {
	texel, e := t.pixels.read(off)
	if e != nil {
		return SampleColor{}, false, e
	}
	if texel == 255 {
		return SampleColor{}, false, nil
	}
	if t.shadeOn {
		texel, e = t.shade.read(uint16(uint32(t.shadeOff) + uint32(texel)))
		if e != nil {
			return SampleColor{}, false, e
		}
	}
	return integerColor(texel), true, nil
}
func r4Color(c hd3d.Call, u, v *big.Rat) (SampleColor, bool, error) {
	t, e := compileTexture(c)
	if e != nil {
		return SampleColor{}, false, e
	}
	return t.color(u, v)
}
func clipAttribute(v [3]uint16, kind uint8, in, out int, n, d uint64) (*big.Rat, error) {
	if in < 0 || in > 2 {
		return nil, fmt.Errorf("invalid original corner")
	}
	a := big.NewRat(int64(v[in]), 1)
	if kind == 0 {
		return a, nil
	}
	if kind != 1 || out < 0 || out > 2 || in == out || d == 0 || n >= d {
		return nil, fmt.Errorf("invalid near source")
	}
	diff := new(big.Rat).SetInt64(int64(v[in]) - int64(v[out]))
	t := new(big.Rat).SetFrac(new(big.Int).SetUint64(n), new(big.Int).SetUint64(d))
	a.Sub(a, diff.Mul(diff, t))
	return a, bounded(a)
}
