package r5asset

import "math"

type validator struct{ err error }

func (v *validator) fail(code, detail string) {
	if v.err == nil {
		v.err = bad(code, detail)
	}
}
func bad(code, detail string) error {
	if code == "quota" || code == "json-length" {
		return reject(Capacity, 0, 100)
	}
	return reject(Structure, 0, 101)
}
func words(s string) int {
	if s == "" {
		return 0
	}
	n := 1
	for i := range s {
		if s[i] == ' ' {
			n++
		}
	}
	return n
}
func wordIndex(s string, key value) int {
	i, n := 0, 0
	for i < len(s) {
		j := i
		for j < len(s) && s[j] != ' ' {
			j++
		}
		if key.equalString(s[i:j]) {
			return n
		}
		i = j + 1
		n++
	}
	return -1
}
func (v *validator) obj(x value, required, optional string) value {
	if x.kind() != tObject {
		v.fail("shape", "object")
		return value{}
	}
	nr, no := words(required), words(optional)
	if nr+no > 32 {
		v.fail("shape", "role capacity")
		return value{}
	}
	var seen uint32
	i, end := x.i+1, x.a.tokens[x.i].Next-1
	for i < end {
		key := value{x.a, i, true}
		vi := i + 2
		k := wordIndex(required, key)
		if k < 0 {
			k = wordIndex(optional, key)
			if k >= 0 {
				k += nr
			}
		}
		if k < 0 || x.a.tokens[vi].Kind == tNull {
			v.fail("shape", "unknown/null")
			return value{}
		}
		bit := uint32(1) << uint(k)
		if seen&bit != 0 {
			v.fail("duplicate", "decoded key")
			return value{}
		}
		seen |= bit
		i = x.a.tokens[vi].Next
		if i < end {
			i++
		}
	}
	want := ^uint32(0)
	if nr < 32 {
		want = (uint32(1) << uint(nr)) - 1
	}
	if seen&want != want {
		v.fail("shape", "missing")
	}
	return x
}
func (v *validator) arr(x value, n int) value {
	if x.kind() != tArray || x.length() != n {
		v.fail("shape", "array count")
		return value{}
	}
	return x
}
func (v *validator) u(x value) uint64 {
	n, ok := x.uint()
	if !ok {
		v.fail("integer", "uint64")
	}
	return n
}
func (v *validator) f(x value) float64 {
	n, ok := x.float()
	if !ok {
		v.fail("finite", "number")
	}
	return n
}
func (v *validator) eq(x value, w any) {
	ok := false
	switch a := w.(type) {
	case string:
		ok = x.equalString(a)
	case bool:
		ok = (a && x.kind() == tTrue) || (!a && x.kind() == tFalse)
	}
	if !ok {
		v.fail("profile", "scalar mismatch")
	}
}
func (v *validator) num(x value, w uint64) {
	if v.u(x) != w {
		v.fail("profile", "integer mismatch")
	}
}
func (v *validator) floats(x value, w []float64) {
	a := v.arr(x, len(w))
	if v.err != nil {
		return
	}
	for i := range w {
		if v.f(a.at(i)) != w[i] {
			v.fail("profile", "factor mismatch")
		}
	}
}
func addSpan(off, count, arity, total uint64) (uint64, bool) {
	if arity == 0 || arity > 16 || count > (^uint64(0))/arity {
		return 0, false
	}
	n := count * arity
	if n > ^uint64(0)/4 {
		return 0, false
	}
	n *= 4
	if off > total || n > total-off {
		return 0, false
	}
	return n, true
}

type Vertex struct {
	Position, Normal [3]float32
	Tangent          [4]float32
	UV               [2]float32
}
type Primitive struct {
	Vertices, Indices uint64
	Offset            [5]uint64
	Material          uint8
}
type Material struct {
	Kind                                                           uint8
	Presence                                                       uint16
	DoubleSided                                                    bool
	AlphaMode                                                      uint8
	BaseColor                                                      [4]float64
	Emissive                                                       [3]float64
	Metallic, Roughness                                            float64
	BaseTexture, NormalTexture, MetalRoughTexture, EmissiveTexture int8
	Sampler                                                        uint8
	MagFilter, MinFilter, WrapS, WrapT                             uint16
}
type meshData struct {
	Prim      [6]Primitive
	Materials [6]Material
	Count     int
}

func inverse(p [3]float32) [3]float64 {
	k := float64(167) / float64(88800)
	return [3]float64{-float64(p[0]) / k, float64(p[2]) / k, float64(p[1]) / k}
}
func inverseDirection(p [3]float32) [3]float32 { return [3]float32{-p[0], p[2], p[1]} }
func finite(x float32) bool                    { return !math.IsNaN(float64(x)) && !math.IsInf(float64(x), 0) }
