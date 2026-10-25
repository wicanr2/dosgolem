package opaque

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/geometry"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
	"sort"
)

func (r *Renderer) renderComposition(in Input, n int, shade *shadeContext) (*Result, error) {
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
	var retainedGlass uint64
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
			bf, err := r.bindCompositionFace(in, links, o, oi, fi, n, points, e, v.Candidates)
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
			objectErr = shade.prepareShade(in, oi, faces)
		}
		if objectErr == nil {
			// Texture addresses are checked at all covered samples before the first
			// write for this object. FF does not require a shade lookup.
			for _, f := range faces {
				for _, t := range f.triangles {
					err := visitTriangle(t, func(x, y, i, j int, w weights) error {
						for _, p := range t.passes {
							if p.bound.kind == "R4" {
								if _, _, err := evaluateCompositionPass(p, w, x, y, i, j); err != nil {
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
			var failure Failure
			if errors.As(objectErr, &failure) && (failure.Gate == H3 || failure.Gate == H4) {
				return nil, failure
			}
			rep.Status, rep.Gate, rep.Reason = realspace.Failed, H2, objectErr.Error()
			out.report.Diagnostics.H2Objects++
			out.report.Diagnostics.Problems = append(out.report.Diagnostics.Problems, gateFailure(H2, oi, badFace, rep.Reason))
			out.report.Objects = append(out.report.Objects, rep)
			continue
		}
		var keyErr error
		rep.MaterialKey, keyErr = boundedMaterialObjectKey(faces, in.Identity)
		if keyErr != nil {
			return nil, keyErr
		}
		for _, f := range faces {
			if f.glass != nil {
				retainedGlass += uint64(len(f.glass.triangles))*4096 + 128
				if retainedGlass > r.limits.IntermediateBytes {
					return nil, gateFailure(H3, oi, f.index, "retained glass geometry capacity")
				}
				out.compositionGlass = append(out.compositionGlass, *f.glass)
			}
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
					c, wrote, seq, err := evaluateCompositionTriangle(t, w, x, y, i, j)
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
						out.samples[at] = sample{color: c, depth: depth, owner: uint32(oi)<<16 | uint32(f.index), flags: flags | uint32(seq)<<8}
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
	sort.Slice(out.compositionGlass, func(i, j int) bool { return out.compositionGlass[i].seq < out.compositionGlass[j].seq })
	sort.Slice(out.deferred, func(i, j int) bool { return out.deferred[i].Call.Seq < out.deferred[j].Call.Seq })
	out.report.Needs3b = len(out.deferred) != 0
	out.report.Diagnostics.DeferredCalls = uint64(len(out.deferred))
	return out, nil
}
