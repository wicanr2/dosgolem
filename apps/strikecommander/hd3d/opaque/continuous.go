package opaque

import (
	"fmt"
	"math/big"
	"math/bits"
)

const CompositionRuleVersion = "013c-continuous-painter-glass-v1"

// continuousR4 uses the same affine scalar as the geometric sample. Class7
// is constant; class8 applies 9/8 before clamping, with no intermediate floor.
func continuousR4(s *big.Rat, class uint8, texel byte, table []byte) (SampleColor, error) {
	if s == nil || len(table) != 4096 || (class != 7 && class != 8) {
		return SampleColor{}, fmt.Errorf("continuous shade inputs")
	}
	if err := bounded(s); err != nil {
		return SampleColor{}, err
	}
	v := new(big.Rat).Set(s)
	if class == 8 {
		v.Mul(v, big.NewRat(9, 8))
	}
	if v.Sign() < 0 {
		v.SetInt64(0)
	}
	if v.Cmp(big.NewRat(255, 1)) > 0 {
		v.SetInt64(255)
	}
	r := new(big.Rat).Quo(new(big.Rat).Sub(big.NewRat(255, 1), v), big.NewRat(16, 1))
	if r.Cmp(big.NewRat(15, 1)) > 0 {
		r.SetInt64(15)
	}
	lo := floorRat(r).Int64()
	hi := min(lo+1, 15)
	f := new(big.Rat).Sub(r, big.NewRat(lo, 1))
	w, e := weight24(f)
	if e != nil {
		return SampleColor{}, e
	}
	a := table[int(lo)*256+int(texel)]
	if w == 0 || hi == lo {
		return integerColor(a), nil
	}
	b := table[int(hi)*256+int(texel)]
	if w == WeightDenominator {
		return integerColor(b), nil
	}
	return SampleColor{a, b, w}, nil
}

func originalShadeRow(class uint8, s [3]uint8) (int, error) {
	switch class {
	case 7:
		return (255 - int(s[0])) >> 4, nil
	case 8:
		v := (3 * (int(s[0]) + int(s[1]) + int(s[2]))) / 8
		return (255 - min(v, 255)) >> 4, nil
	}
	return 0, fmt.Errorf("unsupported scalar class")
}

// Exact channel arithmetic is kept through every glass layer and all four
// coverage samples. RGB code values are rounded only once at the output pixel.
func componentChannel(c SampleColor, p [256][3]byte, ch int) (*big.Rat, error) {
	if !validColor(c) || ch < 0 || ch > 2 {
		return nil, fmt.Errorf("component channel")
	}
	n := (uint64(WeightDenominator)-uint64(c.W24))*uint64(p[c.Lo][ch]) + uint64(c.W24)*uint64(p[c.Hi][ch])
	return new(big.Rat).SetFrac(new(big.Int).SetUint64(n), new(big.Int).SetUint64(uint64(WeightDenominator))), nil
}
func applyGlass134(c *big.Rat) (*big.Rat, error) {
	if c == nil {
		return nil, fmt.Errorf("missing glass background")
	}
	v := new(big.Rat).Quo(new(big.Rat).Add(c, big.NewRat(134, 1)), big.NewRat(2, 1))
	return v, bounded(v)
}
func resolveFour(values [4]*big.Rat) (byte, error) {
	sum := new(big.Rat)
	for _, v := range values {
		if v == nil || v.Sign() < 0 || v.Cmp(big.NewRat(255, 1)) > 0 {
			return 0, fmt.Errorf("invalid channel")
		}
		sum.Add(sum, v)
		if e := bounded(sum); e != nil {
			return 0, e
		}
	}
	sum.Quo(sum, big.NewRat(4, 1))
	q := roundEven(sum.Num(), sum.Denom())
	return byte(q.Uint64()), nil
}

// continuousRatio is the identical rational/Q24 rule for ordinary integer
// barycentric attributes. The guarded ranges keep all scalar arithmetic in
// uint64; only the Q24 product needs an exact unsigned128-bit intermediate.
func continuousRatio(num, area uint64, class uint8, texel byte, table *[4096]byte) (SampleColor, bool) {
	if area == 0 || area >= 1<<47 || num > 255*area || table == nil {
		return SampleColor{}, false
	}
	factor, den := uint64(1), area
	if class == 8 {
		factor = 9
		den *= 8
	} else if class != 7 {
		return SampleColor{}, false
	}
	value := num * factor
	if value >= 255*den {
		return integerColor(table[texel]), true
	}
	rn, rd := 255*den-value, 16*den
	if rn >= 15*rd {
		return integerColor(table[15*256+int(texel)]), true
	}
	row, rem := rn/rd, rn%rd
	a := table[int(row)*256+int(texel)]
	hi, lo := bits.Mul64(rem, uint64(WeightDenominator))
	q, r := bits.Div64(hi, lo, rd)
	if r*2 > rd || r*2 == rd && q&1 != 0 {
		q++
	}
	if q == 0 {
		return integerColor(a), true
	}
	z := table[(int(row)+1)*256+int(texel)]
	if q == uint64(WeightDenominator) {
		return integerColor(z), true
	}
	return SampleColor{a, z, uint32(q)}, true
}

// compositionGlassVisible is the Painter-extension/depth predicate used by
// actual RGB resolution. Glass does not write the opaque depth buffer.
func compositionGlassVisible(bgSeq uint16, opaqueDepth uint64, opaqueSeq uint16, glassDepth uint64, glassSeq uint16) bool {
	return bgSeq < glassSeq && (opaqueDepth == 0 || glassDepth > opaqueDepth || glassDepth == opaqueDepth && glassSeq > opaqueSeq)
}

// resolveCompositionPixel is shared by the renderer and the independent-input
// test bridge. It performs one final ties-even round after all four samples.
func resolveCompositionPixel(colors [4]SampleColor, layers [4]uint16, palette [256][3]byte) ([4]byte, error) {
	var out [4]byte
	anyGlass := false
	for _, n := range layers {
		anyGlass = anyGlass || n != 0
	}
	for ch := 0; ch < 3; ch++ {
		if !anyGlass {
			var sum uint64
			for _, v := range colors {
				if !validColor(v) {
					return out, fmt.Errorf("invalid color component")
				}
				sum += (uint64(WeightDenominator)-uint64(v.W24))*uint64(palette[v.Lo][ch]) + uint64(v.W24)*uint64(palette[v.Hi][ch])
			}
			den := uint64(WeightDenominator) * 4
			q, rem := sum/den, sum%den
			if rem*2 > den || rem*2 == den && q&1 != 0 {
				q++
			}
			out[ch] = byte(q)
		} else {
			var values [4]*big.Rat
			for k, v := range colors {
				z, e := componentChannel(v, palette, ch)
				if e != nil {
					return out, e
				}
				for n := uint16(0); n < layers[k]; n++ {
					z, e = applyGlass134(z)
					if e != nil {
						return out, gateFailure(H3, -1, -1, e.Error())
					}
				}
				values[k] = z
			}
			v, e := resolveFour(values)
			if e != nil {
				return out, e
			}
			out[ch] = v
		}
	}
	out[3] = 255
	return out, nil
}
