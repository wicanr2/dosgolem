package opaque

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math/big"
	"sync"
)

type tableEpoch struct {
	reset, models, shade, serial, firstCU, lastCU uint64
	verified, poisoned                            bool
	version                                       uint64
	content                                       [4096]byte
}

// TableEvidence owns one verified table, not an unbounded history of epochs.
// Different source generations replace the current epoch; old results retain
// their version and cannot acquire new publication eligibility.
type TableEvidence struct {
	mu    sync.Mutex
	epoch tableEpoch
}

func (t *TableEvidence) Version() uint64 { t.mu.Lock(); defer t.mu.Unlock(); return t.epoch.version }
func (t *TableEvidence) Eligible(version uint64) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return !t.epoch.poisoned && version == t.epoch.version
}
func (t *TableEvidence) adopt(id ShadeIdentity, table *[4096]byte, rowValid bool) (uint64, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	e := &t.epoch
	same := e.reset == id.ResetGeneration && e.models == id.ModelsGeneration && e.shade == id.ShadeEpoch
	if !same && e.verified {
		older := id.ResetGeneration < e.reset || id.ResetGeneration == e.reset && (id.ModelsGeneration < e.models || id.ModelsGeneration == e.models && id.ShadeEpoch < e.shade)
		if older {
			return e.version, gateFailure(H3, -1, -1, "retired table source epoch")
		}
	}
	if same && e.verified && !e.poisoned && !bytes.Equal(e.content[:], table[:]) {
		e.poisoned = true
		e.version++
		e.lastCU = max(e.lastCU, id.CUKey)
		return e.version, gateFailure(H3, -1, -1, "verified table content changed")
	}
	if same && e.poisoned && id.CUKey <= e.lastCU {
		return e.version, gateFailure(H3, -1, -1, "poisoned table epoch")
	}
	if sha256.Sum256(table[:]) != hashHex(shadeTableSHA) {
		return e.version, gateFailure(H2, -1, -1, "first/new table SHA differs")
	}
	if !rowValid {
		return e.version, gateFailure(H2, -1, -1, "original same-call row differs")
	}
	if !same || !e.verified || e.poisoned {
		*e = tableEpoch{reset: id.ResetGeneration, models: id.ModelsGeneration, shade: id.ShadeEpoch, serial: e.serial + 1, version: e.version + 1, firstCU: id.CUKey, lastCU: id.CUKey, verified: true, content: *table}
	} else {
		e.lastCU = max(e.lastCU, id.CUKey)
	}
	return e.version, nil
}

type continuousPass struct {
	class  uint8
	scalar [3]uint16
	table  [4096]byte
}
type shadeContext struct {
	source       ShadeSource
	sourceError  *Failure
	evidence     *TableEvidence
	declaration  ShadeDeclaration
	adopted      bool
	sourceH4     bool
	tableVersion uint64
}

func texelAt(p triPass, w weights) (byte, error) {
	if p.integerUV {
		un, a, ok := w.integerNumerator(p.uInt)
		vn, _, ok2 := w.integerNumerator(p.vInt)
		if ok && ok2 {
			return p.bound.tex.pixels.read(uint16(uint64(p.bound.tex.texOff) + (vn/a/256)*uint64(p.bound.tex.pitch) + un/a/256))
		}
	}
	u, e := w.attribute(p.u)
	if e != nil {
		return 0, e
	}
	v, e := w.attribute(p.v)
	if e != nil {
		return 0, e
	}
	if u.Sign() < 0 || v.Sign() < 0 {
		return 0, fmt.Errorf("negative interpolated UV")
	}
	ui := floorRat(new(big.Rat).Quo(u, big.NewRat(256, 1)))
	vi := floorRat(new(big.Rat).Quo(v, big.NewRat(256, 1)))
	return p.bound.tex.pixels.read(uint16(uint64(p.bound.tex.texOff) + vi.Uint64()*uint64(p.bound.tex.pitch) + ui.Uint64()))
}
func evaluateCompositionPass(p triPass, w weights, x, y, i, j int) (SampleColor, bool, error) {
	s := p.bound.continuous
	if s == nil {
		return evaluatePass(p, w, x, y, i, j)
	}
	texel, e := texelAt(p, w)
	if e != nil {
		return SampleColor{}, false, e
	}
	if texel == 255 {
		return SampleColor{}, false, nil
	}
	if !p.bound.tex.shadeOn {
		return integerColor(texel), true, nil
	}
	if s.class == 7 {
		if v, ok := continuousRatio(uint64(s.scalar[0]), 1, 7, texel, &s.table); ok {
			return v, true, nil
		}
	}
	if p.integerWord {
		if num, area, ok := w.integerNumerator(p.wordInt); ok {
			if v, good := continuousRatio(num, area, s.class, texel, &s.table); good {
				return v, true, nil
			}
		}
	}
	scalar, e := w.attribute(p.word)
	if e != nil {
		return SampleColor{}, false, e
	}
	c, e := continuousR4(scalar, s.class, texel, s.table[:])
	return c, true, e
}
func evaluateCompositionTriangle(t boundTriangle, w weights, x, y, i, j int) (SampleColor, bool, uint16, error) {
	var c SampleColor
	var seq uint16
	wrote := false
	for _, p := range t.passes {
		v, write, e := evaluateCompositionPass(p, w, x, y, i, j)
		if e != nil {
			return SampleColor{}, false, 0, e
		}
		if write {
			c, seq, wrote = v, p.bound.call.Seq, true
		}
	}
	return c, wrote, seq, nil
}

// prepareShade first proves actual nonFF shade demand using exactly the same
// geometry/UV phase grid. Missing optional data never affects unused passes.
func (s *shadeContext) prepareShade(in Input, oi int, faces []boundFace) error {
	for fi := range faces {
		f := &faces[fi]
		vt := in.Scene.Objects[oi].Faces[f.index].FaceVT
		class := uint8(0)
		if vt == 0x33b9 {
			class = 7
		}
		if vt == 0x338d {
			class = 8
		}
		if class == 0 {
			continue
		}
		needed := map[uint16]*boundPass{}
		for _, tri := range f.triangles {
			if e := visitTriangle(tri, func(x, y, i, j int, w weights) error {
				for _, p := range tri.passes {
					if p.bound.kind != "R4" || !p.bound.tex.shadeOn {
						continue
					}
					v, e := texelAt(p, w)
					if e != nil {
						return e
					}
					if v != 255 {
						needed[p.bound.call.Seq] = p.bound
					}
				}
				return nil
			}); e != nil {
				return e
			}
		}
		for seq, b := range needed {
			if s.sourceError != nil && (s.sourceError.Gate == H3 || s.sourceError.Gate == H4) {
				return gateFailure(H3, oi, f.index, "source packet rejected: "+s.sourceError.Error())
			}
			if s.source == nil {
				return gateFailure(H2, oi, f.index, "continuous shade source absent")
			}
			var table [4096]byte
			var row [256]byte
			witness, e := s.source.ReadShadeCall(uint32(seq), &table, &row)
			if e != nil {
				if x, ok := e.(Failure); ok && x.Gate == H4 {
					s.sourceH4 = true
					return gateFailure(H4, oi, f.index, e.Error())
				}
				return gateFailure(H2, oi, f.index, e.Error())
			}
			if !witness.HasFace || (witness.Object != uint32(oi) || witness.Face != uint32(f.index)) {
				return gateFailure(H2, oi, f.index, "witness face differs")
			}
			// The source-only validator has checked every original role and proof.
			// Table/row mismatch remains inspectable to distinguish H2 from a poisoned
			// already-verified table; unrelated role/read errors never verify an epoch.
			if witness.Reason != 0 && witness.Reason != 10 && witness.Reason != 9 {
				return gateFailure(H2, oi, f.index, "source witness unqualified")
			}
			if !witness.TableComplete || !witness.RowComplete {
				return gateFailure(H2, oi, f.index, "same-call table/row incomplete")
			}
			scalar := [3]uint16{}
			raw := [3]byte{}
			if class == 7 {
				if !witness.HasRaw5 {
					return gateFailure(H2, oi, f.index, "class7 raw5 absent")
				}
				for k := range scalar {
					scalar[k] = uint16(witness.Raw5[4])
					raw[k] = witness.Raw5[4]
				}
			}
			if class == 8 {
				source := in.Scene.Objects[oi].Faces[f.index].Source
				if source == nil {
					return gateFailure(H2, oi, f.index, "class8 constructor absent")
				}
				for k := range scalar {
					raw[k] = source.BuildCall.FullProj[k][16]
					scalar[k] = uint16(raw[k])
				}
			}
			r, _ := originalShadeRow(class, raw)
			rowValid := bytes.Equal(row[:], table[r*256:(r+1)*256]) && witness.RowOff == uint16(uint32(witness.TableOff)+uint32(r*256)) && witness.RowSeg == witness.TableSeg
			s.declaration = s.source.CompositionDeclaration()
			version, e := s.evidence.adopt(s.declaration.Identity, &table, rowValid)
			if e != nil {
				return e
			}
			s.tableVersion = version
			b.continuous = &continuousPass{class: class, scalar: scalar, table: table}
			s.adopted = true
		}
		for ti := range f.triangles {
			tri := &f.triangles[ti]
			for pi := range tri.passes {
				p := &tri.passes[pi]
				if p.bound.continuous == nil {
					continue
				}
				for k, v := range tri.tri.Vertices {
					var e error
					p.word[k], e = sourceAttribute(p.bound.continuous.scalar, v.Source)
					if e != nil {
						return e
					}
				}
				p.wordInt, p.integerWord = integerAttrs(p.word)
			}
		}
	}
	return nil
}
