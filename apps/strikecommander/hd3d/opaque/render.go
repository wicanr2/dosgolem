package opaque

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/geometry"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
	"math/big"
	"math/bits"
	"sort"
)

type weights struct {
	fast  bool
	small geometry.FastSampleHit
	large geometry.SampleHit
}

func integerAttrs(a [3]*big.Rat) ([3]uint16, bool) {
	var v [3]uint16
	for i, q := range a {
		if q == nil || !q.IsInt() || q.Sign() < 0 || !q.Num().IsUint64() || q.Num().Uint64() > 65535 {
			return v, false
		}
		v[i] = uint16(q.Num().Uint64())
	}
	return v, true
}
func sampleWeights(t geometry.MeshTriangle, x, y, i, j int) (weights, bool, error) {
	f, ok, e := geometry.EvaluateSampleFast(t, x, y, i, j)
	if e != nil {
		return weights{}, false, e
	}
	if ok {
		return weights{fast: true, small: f}, f.Coverage, nil
	}
	v, e := geometry.EvaluateSample(t, x, y, i, j)
	return weights{large: v}, v.Coverage, e
}
func (w weights) depth() uint64 {
	if w.fast {
		return w.small.DepthQ48
	}
	return w.large.DepthQ48
}
func (w weights) order() [3]int {
	if w.fast {
		return w.small.SourceIndices
	}
	return w.large.SourceIndices
}
func (w weights) attribute(a [3]*big.Rat) (*big.Rat, error) {
	order := w.order()
	out := new(big.Rat)
	var area *big.Int
	if w.fast {
		area = new(big.Int).SetUint64(w.small.Area)
	} else {
		area = w.large.Area
	}
	if area == nil || area.Sign() <= 0 {
		return nil, fmt.Errorf("attribute area")
	}
	for i, k := range order {
		if k < 0 || k >= 3 || a[k] == nil {
			return nil, fmt.Errorf("attribute source index")
		}
		var edge *big.Int
		if w.fast {
			edge = new(big.Int).SetUint64(w.small.Edges[i])
		} else {
			edge = w.large.Edges[i]
		}
		if edge == nil || edge.Sign() < 0 {
			return nil, fmt.Errorf("attribute edge")
		}
		out.Add(out, new(big.Rat).Mul(a[k], new(big.Rat).SetInt(edge)))
		if err := bounded(out); err != nil {
			return nil, err
		}
	}
	out.Quo(out, new(big.Rat).SetInt(area))
	return out, bounded(out)
}
func (w weights) integerNumerator(v [3]uint16) (uint64, uint64, bool) {
	if !w.fast || w.small.Area == 0 || w.small.Area >= 1<<47 {
		return 0, 0, false
	}
	n := uint64(0)
	for i, k := range w.small.SourceIndices {
		n += uint64(v[k]) * w.small.Edges[i]
	}
	return n, w.small.Area, true
}
func fastR3(num, area uint64, b Band, phase int) SampleColor {
	if phase != 0 {
		num += 128 * area
	}
	den := area * 256
	top := uint64(b.Base+b.Range) * den
	if num >= top {
		return integerColor(byte(b.Base + b.Range))
	}
	lo, rem := num/den, num%den
	hi, low := bits.Mul64(rem, uint64(WeightDenominator))
	q, r := bits.Div64(hi, low, den)
	if r*2 > den || r*2 == den && q&1 != 0 {
		q++
	}
	if q == 0 {
		return integerColor(byte(lo))
	}
	if q == uint64(WeightDenominator) {
		return integerColor(byte(lo + 1))
	}
	return SampleColor{byte(lo), byte(lo + 1), uint32(q)}
}
func evaluatePass(p triPass, w weights, x, y, i, j int) (SampleColor, bool, error) {
	switch p.bound.kind {
	case "R1":
		return p.bound.constant, true, nil
	case "R3":
		if p.integerWord {
			if num, area, ok := w.integerNumerator(p.wordInt); ok {
				return fastR3(num, area, p.bound.band, ((2*x+i)^(2*y+j))&1), true, nil
			}
		}
		c, e := w.attribute(p.word)
		if e != nil {
			return SampleColor{}, false, e
		}
		c.Quo(c, big.NewRat(256, 1))
		v, e := r3Color(c, p.bound.band, x, y, i, j)
		return v, true, e
	case "R4":
		if p.integerUV {
			un, a, ok := w.integerNumerator(p.uInt)
			vn, _, ok2 := w.integerNumerator(p.vInt)
			if ok && ok2 {
				u, v := (un/a)/256, (vn/a)/256
				return p.bound.tex.colorAddress(uint16(uint64(p.bound.tex.texOff) + v*uint64(p.bound.tex.pitch) + u))
			}
		}
		u, e := w.attribute(p.u)
		if e != nil {
			return SampleColor{}, false, e
		}
		v, e := w.attribute(p.v)
		if e != nil {
			return SampleColor{}, false, e
		}
		return p.bound.tex.color(u, v)
	}
	return SampleColor{}, false, fmt.Errorf("unbound material pass")
}
func evaluateTriangle(t boundTriangle, w weights, x, y, i, j int) (SampleColor, bool, error) {
	var local SampleColor
	wrote := false
	for _, p := range t.passes {
		c, write, e := evaluatePass(p, w, x, y, i, j)
		if e != nil {
			return SampleColor{}, false, e
		}
		if write {
			local, wrote = c, true
		}
	}
	return local, wrote, nil
}
func visitTriangle(t boundTriangle, fn func(x, y, i, j int, w weights) error) error {
	for j := 0; j < 2; j++ {
		for i := 0; i < 2; i++ {
			box, e := geometry.SampleBounds(t.tri, i, j)
			if e != nil {
				return e
			}
			for y := box.Min.Y; y < box.Max.Y; y++ {
				for x := box.Min.X; x < box.Max.X; x++ {
					w, covered, e := sampleWeights(t.tri, x, y, i, j)
					if e != nil {
						return e
					}
					if covered {
						if e = fn(x, y, i, j, w); e != nil {
							return e
						}
					}
				}
			}
		}
	}
	return nil
}
func pointBytesMatch(o realspace.ObjObs) bool {
	if len(o.VertXYZ) != 12*len(o.VertRT) {
		return false
	}
	for i, v := range o.VertRT {
		for j, n := range v {
			if int32(binary.LittleEndian.Uint32(o.VertXYZ[i*12+j*4:])) != n {
				return false
			}
		}
	}
	return true
}
func gateFailure(g FailureGate, oi, fi int, reason string) Failure {
	return Failure{Gate: g, Object: oi, Face: fi, Pass: -1, Reason: reason}
}
func (r *Renderer) renderSnapshot(in Input, n int) (*Result, error) {
	links, e := qualifyOriginal(in, r.tables)
	if e != nil {
		return nil, e
	}
	out := &Result{identity: in.Identity, scale: n, width: 320 * n, height: 200 * n, report: Report{Identity: in.Identity, Scale: n, Valid: true}}
	if in.Capture != nil {
		c := *in.Capture
		out.capture = &c
	}
	out.report.Diagnostics.OriginalChecked = true
	// Preserve every original R2 before object admission. Missing or ambiguous
	// E1 ownership is not evidence that glass vanished; -1 records unknown identity.
	for ptr, calls := range links.calls {
		id := faceRef{-1, -1}
		if owners := links.owners[ptr]; len(owners) == 1 {
			id = owners[0]
		}
		for _, c := range calls {
			if c.Kind == "R2" {
				out.deferred = append(out.deferred, DeferredOp{Object: id.object, Face: id.face, Call: c})
			}
		}
	}
	out.samples = make([]sample, 4*out.width*out.height)
	for oi, o := range in.Scene.Objects {
		rep := ObjectReport{Object: oi}
		out.report.Diagnostics.Objects++
		v := realspace.CheckObject(r.lib, in.Scene.Cameras, &o)
		rep.Status, rep.Reason, rep.Candidates = v.Status, v.Reason, len(v.Candidates)
		if o.RecVT != binary.LittleEndian.Uint16(o.RecRaw[:]) || o.LOD != o.RecRaw[9] || o.RecVT == 0x314a && !pointBytesMatch(o) {
			rep.Status, rep.Reason = realspace.Failed, "record/runtime typed metadata differs"
		}
		if rep.Status != realspace.Passed || v.Selected == nil {
			rep.Gate = H1
			out.report.Diagnostics.H1Objects++
			out.report.Diagnostics.Problems = append(out.report.Diagnostics.Problems, gateFailure(H1, oi, -1, rep.Reason))
			out.report.Objects = append(out.report.Objects, rep)
			continue
		}
		e := v.Selected
		points := make([]geometry.Point3, len(e.Verts))
		apex := map[int]bool{}
		if e.Kind == realspace.APPR {
			for _, i := range e.Apex {
				apex[i] = true
			}
		}
		m := geometry.Matrix(realspace.MatrixFromRecord(o.RecRaw))
		for i, p := range e.Verts {
			if apex[i] {
				p = o.VertRT[i]
			}
			points[i] = geometry.Transform(geometry.ModelPoint{X: p[0], Y: p[1], Z: p[2]}, m)
		}
		var faces []boundFace
		var objectErr error
		badFace := -1
		var metadata uint64 = uint64(len(points)) * 12
		for fi, f := range o.Faces {
			if f.AL == 0 {
				out.report.Diagnostics.BackfaceFaces++
				continue
			}
			rep.VisibleFaces++
			bf, err := r.bindFace(in, links, o, oi, fi, n, points, e, v.Candidates)
			if err != nil {
				var failure Failure
				if errors.As(err, &failure) && (failure.Gate == H3 || failure.Gate == H4) {
					return nil, failure
				}
				objectErr, badFace = err, fi
				break
			}
			// Bound triangles/attributes are one-object scratch, not per-sample state.
			metadata += uint64(len(bf.triangles))*4096 + 256
			for _, t := range bf.triangles {
				for _, p := range t.passes {
					for _, seg := range []segment{p.bound.tex.pixels, p.bound.tex.shade} {
						for _, s := range seg.spans {
							metadata += uint64(len(s.Data)) + 32
						}
					}
				}
			}
			if metadata > r.limits.IntermediateBytes {
				return nil, gateFailure(H3, oi, fi, "bound object intermediate capacity")
			}
			faces = append(faces, bf)
		}
		if objectErr == nil {
			// Texture addresses are checked at all covered samples before the first
			// write for this object. FF does not require a shade lookup.
			for _, f := range faces {
				for _, t := range f.triangles {
					err := visitTriangle(t, func(x, y, i, j int, w weights) error {
						for _, p := range t.passes {
							if p.bound.kind == "R4" {
								if _, _, err := evaluatePass(p, w, x, y, i, j); err != nil {
									return err
								}
							}
						}
						return nil
					})
					if err != nil {
						objectErr, badFace = err, f.index
						break
					}
				}
				if objectErr != nil {
					break
				}
			}
		}
		if objectErr != nil {
			rep.Status, rep.Gate, rep.Reason = realspace.Failed, H2, objectErr.Error()
			out.report.Diagnostics.H2Objects++
			out.report.Diagnostics.Problems = append(out.report.Diagnostics.Problems, gateFailure(H2, oi, badFace, rep.Reason))
			out.report.Objects = append(out.report.Objects, rep)
			continue
		}
		rep.MaterialKey = materialObjectKey(faces, in.Identity)
		for _, f := range faces {
			if f.clipped {
				out.report.Diagnostics.ClippedFaces++
			}
			if f.allClipped {
				out.report.Diagnostics.AllClippedFaces++
			}
			// Deferred raw calls were preserved before object admission.
			rep.Triangles += len(f.triangles)
			for _, t := range f.triangles {
				out.report.Diagnostics.Triangles++
				err := visitTriangle(t, func(x, y, i, j int, w weights) error {
					c, wrote, err := evaluateTriangle(t, w, x, y, i, j)
					if err != nil {
						return err
					}
					if w.fast {
						out.report.Diagnostics.FastSamples++
					} else {
						out.report.Diagnostics.BigSamples++
					}
					if !wrote {
						out.report.Diagnostics.TransparentSamples++
						return nil
					}
					at := (y*out.width+x)*4 + j*2 + i
					depth := w.depth()
					if depth == 0 {
						return fmt.Errorf("covered sample has zero depth")
					}
					if depth > out.samples[at].depth {
						flags := uint32(0)
						if t.tri.Ref.Clipped {
							flags |= 2
						}
						if t.tri.Ref.Clamped {
							flags |= 4
						}
						out.samples[at] = sample{color: c, depth: depth, owner: uint32(oi)<<16 | uint32(f.index), flags: flags}
						rep.WrittenSamples++
						out.report.Diagnostics.WrittenSamples++
					}
					return nil
				})
				if err != nil {
					return nil, gateFailure(H3, oi, f.index, "unexpected material error after preflight: "+err.Error())
				}
			}
		}
		out.report.Diagnostics.SuccessfulObjects++
		out.report.Objects = append(out.report.Objects, rep)
	}
	sort.Slice(out.deferred, func(i, j int) bool { return out.deferred[i].Call.Seq < out.deferred[j].Call.Seq })
	out.report.Needs3b = len(out.deferred) != 0
	out.report.Diagnostics.DeferredCalls = uint64(len(out.deferred))
	return out, nil
}
