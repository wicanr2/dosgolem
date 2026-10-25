package opaque

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
)

const VisibilityRuleVersion = "painter-seq-extension-v1"

type CompositionDiagnostics struct {
	ActualMaskSHA                                                                      [32]byte
	NoExtra                                                                            bool
	ApproxVisibility, OpaqueSamples, GlassSamples, PreservedFallback, OldPixelsCleared uint64
	RGBSHA, MaskSHA, DepthOwnerSHA, GlassSHA                                           [32]byte
}
type compositionBG struct {
	replay            hd3d.Result
	visible, fallback [hd3d.PixelCount]bool
	glass             map[uint16]bool
}

func (r *CompositionResult) background() (*compositionBG, error) {
	in := r.snapshot.core.input
	l, e := qualifyOriginal(in, r.manager.core.tables)
	if e != nil {
		return nil, e
	}
	b := &compositionBG{glass: make(map[uint16]bool, len(r.core.compositionGlass))}
	for _, g := range r.core.compositionGlass {
		b.glass[g.seq] = true
	}
	success := make([]bool, len(in.Scene.Objects))
	// ObjStatus is a string enum; use the actual gate and successful object list
	// below without relying on caller-provided Selected or call.Object fields.
	for _, o := range r.core.report.Objects {
		success[o.Object] = o.Gate == ""
	}
	first := uint16(0xffff)
	for _, c := range in.Scene.Raster.Calls {
		h, has := l.headers[c.Element]
		if has && !l.badHeaders[c.Element] && objectVT(binary.LittleEndian.Uint16(h[:])) && c.Kind != "BlackBox" {
			first = min(first, c.Seq)
		}
	}
	g := in.Scene.Raster
	g.Calls = nil
	g.Buf3D = nil
	for _, c := range in.Scene.Raster.Calls {
		h, has := l.headers[c.Element]
		model := has && !l.badHeaders[c.Element] && objectVT(binary.LittleEndian.Uint16(h[:]))
		if len(l.owners[c.Element]) != 0 && !model {
			return nil, gateFailure(H3, -1, -1, "linked element outside supported model header set")
		}
		if c.Kind == hd3d.BlackBox {
			if len(c.Patch) != 0 || c.Seq >= first {
				return nil, gateFailure(H3, -1, -1, "counterfactual BlackBox dependency unknown")
			}
			g.Calls = append(g.Calls, c)
			continue
		}
		if model {
			if c.Kind != hd3d.R1 && c.Kind != hd3d.R2 && c.Kind != hd3d.R3 && c.Kind != hd3d.R4 {
				return nil, gateFailure(H3, -1, -1, "unsupported linked model raster operator")
			}
			owners := l.owners[c.Element]
			if len(owners) != 1 {
				return nil, gateFailure(H3, -1, -1, "counterfactual model owner ambiguous")
			}
			owner := owners[0]
			obj := in.Scene.Objects[owner.object]
			if c.Kind == hd3d.R2 && !b.glass[c.Seq] && obj.RecVT != 0x314a {
				return nil, gateFailure(H2, owner.object, owner.face, "unresolved raw R2 cannot disappear")
			}
			// Remove all uniquely classified model operators, never a Patch After.
			continue
		}
		c.Object = false
		g.Calls = append(g.Calls, c)
	}
	b.replay = hd3d.Replay(g, r.manager.core.tables)
	if !b.replay.Valid {
		return nil, gateFailure(H3, -1, -1, fmt.Sprintf("counterfactual BG replay invalid: %v", b.replay.Diagnostics.Faults))
	}
	for p, seq := range in.Scene.Original.Writer {
		c := in.Scene.Raster.Calls[seq]
		h, has := l.headers[c.Element]
		if !has || l.badHeaders[c.Element] || !objectVT(binary.LittleEndian.Uint16(h[:])) {
			continue
		}
		owners := l.owners[c.Element]
		if len(owners) != 1 {
			return nil, gateFailure(H3, -1, -1, "last model writer owner ambiguous")
		}
		oi := owners[0].object
		fallback := !success[oi] || in.Scene.Objects[oi].RecVT == 0x314a && c.Kind == hd3d.R2
		b.fallback[p] = fallback
		b.visible[p] = !fallback
	}
	return b, nil
}

// WriteRGBA resolves into caller-owned, pre-reserved output storage. No slice
// aliases the immutable core. The caller must not publish dst on an error.
func (r *CompositionResult) WriteRGBA(p PaletteSnapshot, dst []byte) (CompositionDiagnostics, error) {
	return r.writeRGBA(p, dst, nil)
}

// WriteRGBAWithMask additionally writes one byte per output pixel:1 only when
// the resolved3D RGB differs from that pixel's original nearest source. The
// caller reserves this core-workspace mask, then intersects the per-image Gate.
func (r *CompositionResult) WriteRGBAWithMask(p PaletteSnapshot, dst, mask []byte) (CompositionDiagnostics, error) {
	return r.writeRGBA(p, dst, mask)
}
func (r *CompositionResult) writeRGBA(p PaletteSnapshot, dst, actualMask []byte) (CompositionDiagnostics, error) {
	c := r.manager
	c.worker.Lock()
	defer c.worker.Unlock()
	c.mu.Lock()
	if e := r.eligibleLocked(); e != nil {
		c.mu.Unlock()
		return CompositionDiagnostics{}, e
	}
	core := r.core
	in := r.snapshot.core.input
	if core.capture == nil || p.Identity != core.identity || p.Capture != *core.capture || p.SHA != PaletteHash(p.Colors) || len(dst) != core.width*core.height*4 || actualMask != nil && len(actualMask) != core.width*core.height {
		c.mu.Unlock()
		return CompositionDiagnostics{}, gateFailure(H3, -1, -1, "resolve capture/palette/output differs")
	}
	r.snapshot.active++
	c.active.Add(1)
	c.mu.Unlock()
	defer func() { c.mu.Lock(); r.snapshot.active--; c.mu.Unlock(); c.active.Done() }()
	var d CompositionDiagnostics
	e := c.budget.CompositionWorkspace(func() error {
		bg, e := r.background()
		if e != nil {
			return e
		}
		d.NoExtra = r.noExtra
		maskHash := sha256.New()
		depthHash := sha256.New()
		glassHash := sha256.New()
		for y := 0; y < core.height; y++ {
			for x := 0; x < core.width; x++ {
				pix := (y/core.scale)*320 + x/core.scale
				if !bg.replay.Written[pix] || bg.replay.Writer[pix] == 0xffff {
					return gateFailure(H3, -1, -1, "BGKnown false")
				}
				var colors [4]SampleColor
				var layers [4]uint16
				for k := 0; k < 4; k++ {
					s := core.samples[(y*core.width+x)*4+k]
					color := integerColor(in.Scene.Raster.Buf3D[pix])
					covered := false
					seq := uint16(s.flags >> 8)
					if bg.visible[pix] {
						color = integerColor(bg.replay.Pixels[pix])
						if k == 0 {
							d.OldPixelsCleared++
						}
					}
					if s.depth != 0 {
						oi := int(s.owner >> 16)
						if bg.fallback[pix] {
							return gateFailure(H3, oi, int(s.owner&65535), "MixedFallbackUnknown")
						}
						if bg.replay.Writer[pix] < seq {
							color = s.color
							covered = true
							d.OpaqueSamples++
							if !bg.visible[pix] {
								d.ApproxVisibility++
							}
						}
					}
					if bg.fallback[pix] {
						d.PreservedFallback++
					}
					for gi := range core.compositionGlass {
						g := &core.compositionGlass[gi]
						depth, hit, e := glassDepthRead(g, x, y, k&1, k>>1)
						if e != nil {
							return e
						}
						if !hit {
							continue
						}
						if bg.fallback[pix] {
							return gateFailure(H3, g.object, g.face, "glass/fallback crossing unknown")
						}
						if !compositionGlassVisible(bg.replay.Writer[pix], s.depth, seq, depth, g.seq) {
							continue
						}
						if p.Colors[20] != ([3]byte{134, 134, 134}) {
							return gateFailure(H2, g.object, g.face, "glass frozen palette profile differs")
						}
						layers[k]++
						d.GlassSamples++
						var raw [16]byte
						binary.LittleEndian.PutUint16(raw[:], g.seq)
						binary.LittleEndian.PutUint64(raw[2:], depth)
						binary.LittleEndian.PutUint32(raw[10:], uint32((y*core.width+x)*4+k))
						_, _ = glassHash.Write(raw[:])
					}
					colors[k] = color
					var m [1]byte
					if covered {
						m[0] |= 1
					}
					if layers[k] != 0 {
						m[0] |= 2
					}
					if covered && !bg.visible[pix] {
						m[0] |= 4
					}
					if bg.fallback[pix] {
						m[0] |= 8
					}
					_, _ = maskHash.Write(m[:])
					var raw [16]byte
					if covered {
						binary.LittleEndian.PutUint64(raw[:], s.depth)
						binary.LittleEndian.PutUint32(raw[8:], s.owner)
						binary.LittleEndian.PutUint16(raw[12:], seq)
					}
					_, _ = depthHash.Write(raw[:])
				}
				at := (y*core.width + x) * 4
				rgba, e := resolveCompositionPixel(colors, layers, p.Colors)
				if e != nil {
					return e
				}
				copy(dst[at:at+4], rgba[:])
				if actualMask != nil {
					v := p.Colors[in.Scene.Raster.Buf3D[pix]]
					actualMask[y*core.width+x] = 0
					if dst[at] != v[0] || dst[at+1] != v[1] || dst[at+2] != v[2] {
						actualMask[y*core.width+x] = 1
					}
				}
			}
		}
		copy(d.MaskSHA[:], maskHash.Sum(nil))
		copy(d.DepthOwnerSHA[:], depthHash.Sum(nil))
		copy(d.GlassSHA[:], glassHash.Sum(nil))
		d.RGBSHA = sha256.Sum256(dst)
		if actualMask != nil {
			d.ActualMaskSHA = sha256.Sum256(actualMask)
		}
		return nil
	})
	c.mu.Lock()
	defer c.mu.Unlock()
	if e == nil {
		e = r.eligibleLocked()
	}
	if e != nil {
		return CompositionDiagnostics{}, e
	}
	return d, nil
}
