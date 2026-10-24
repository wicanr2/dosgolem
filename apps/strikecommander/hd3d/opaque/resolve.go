package opaque

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/geometry"
	"image"
)

func cloneCall(c hd3d.Call) hd3d.Call {
	c.Verts = append([]byte(nil), c.Verts...)
	c.Table = append([]byte(nil), c.Table...)
	c.Aux.Table = append([]byte(nil), c.Aux.Table...)
	c.Patch = append([]hd3d.PixelSet(nil), c.Patch...)
	cp := func(s hd3d.Capture) hd3d.Capture {
		o := hd3d.Capture{}
		for _, v := range s.Spans {
			o.Spans = append(o.Spans, hd3d.Span{Off: v.Off, Data: append([]byte(nil), v.Data...)})
		}
		return o
	}
	c.Tex.Pixels = cp(c.Tex.Pixels)
	c.Tex.Shade = cp(c.Tex.Shade)
	return c
}
func PaletteHash(p [256][3]uint8) [32]byte {
	var b [768]byte
	for i, c := range p {
		copy(b[3*i:], c[:])
	}
	return sha256.Sum256(b[:])
}
func captureValid(id SceneIdentity, c CaptureIdentity) bool {
	return c.Scene == id && c.Steps != 0 && (c.Kind == CaptureBuf3D || c.Kind == CaptureVGA || c.Kind == CaptureFrame) && c.IndexedSHA != ([32]byte{}) && c.OriginalPaletteSHA != ([32]byte{})
}
func NewPaletteSnapshot(id SceneIdentity, c CaptureIdentity, p [256][3]uint8) (PaletteSnapshot, error) {
	if !captureValid(id, c) {
		return PaletteSnapshot{}, fmt.Errorf("palette capture identity invalid")
	}
	return PaletteSnapshot{Identity: id, Capture: c, Colors: p, SHA: PaletteHash(p)}, nil
}
func bgKey(b *BackgroundComponents) [32]byte {
	h := sha256.New()
	head, _ := json.Marshal(struct {
		I SceneIdentity
		C CaptureIdentity
		N int
		R string
		P [32]byte
	}{b.identity, b.capture, b.scale, b.rule, b.paletteSHA})
	h.Write(head)
	var raw [6]byte
	for _, c := range b.colors {
		raw[0], raw[1] = c.Lo, c.Hi
		binary.LittleEndian.PutUint32(raw[2:], c.W24)
		h.Write(raw[:])
	}
	var k [32]byte
	copy(k[:], h.Sum(nil))
	return k
}
func NewBackgroundComponents(id SceneIdentity, c CaptureIdentity, n int, indexed []byte, p PaletteSnapshot) (*BackgroundComponents, error) {
	if n != 2 && n != 3 {
		return nil, fmt.Errorf("background scale")
	}
	if len(indexed) != 64000 || sha256.Sum256(indexed) != c.IndexedSHA || !captureValid(id, c) || p.Identity != id || p.Capture != c || p.SHA != PaletteHash(p.Colors) {
		return nil, fmt.Errorf("background source/capture/palette mismatch")
	}
	w, h := 320*n, 200*n
	b := &BackgroundComponents{identity: id, capture: c, scale: n, width: w, height: h, rule: geometry.SampleRuleVersion, paletteSHA: p.SHA, colors: make([]SampleColor, 4*w*h)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := integerColor(indexed[(y/n)*320+x/n])
			for k := 0; k < 4; k++ {
				b.colors[4*(y*w+x)+k] = v
			}
		}
	}
	b.key = bgKey(b)
	return b, nil
}
func validColor(c SampleColor) bool { return c.W24 < WeightDenominator && (c.W24 != 0 || c.Lo == c.Hi) }

// ResolveRGB is a pure RGB-code resolve. The result and background stay
// immutable, and no geometry, guest state, or current palette is consulted.
func ResolveRGB(r *Result, b *BackgroundComponents, p PaletteSnapshot) (*image.RGBA, error) {
	if r == nil || b == nil || r.capture == nil {
		return nil, fmt.Errorf("missing bound result/background")
	}
	if !r.report.Valid || r.report.Needs3b {
		return nil, fmt.Errorf("result invalid or requires3b")
	}
	if r.identity != b.identity || r.identity != p.Identity || *r.capture != b.capture || *r.capture != p.Capture || !captureValid(r.identity, *r.capture) || r.scale != b.scale || r.width != b.width || r.height != b.height || b.rule != geometry.SampleRuleVersion || p.SHA != PaletteHash(p.Colors) || b.paletteSHA != p.SHA || b.key != bgKey(b) {
		return nil, fmt.Errorf("mixed source/capture/phase/palette")
	}
	if r.width != 320*r.scale || r.height != 200*r.scale || len(r.samples) != 4*r.width*r.height || len(b.colors) != len(r.samples) {
		return nil, fmt.Errorf("resolve dimensions")
	}
	extra := uint64(len(b.colors))*8 + uint64(r.width)*uint64(r.height)*4
	owned := uint64(len(r.samples)) * 24
	limit := DefaultLimits().TotalBytes
	if r.renderer != nil {
		r.renderer.mu.Lock()
		owned = r.renderer.owned + r.renderer.materials.OwnedBytes()
		limit = r.renderer.limits.TotalBytes
		r.renderer.mu.Unlock()
	}
	if owned+extra > limit {
		return nil, Failure{Gate: H3, Object: -1, Face: -1, Pass: -1, Reason: "RGB/background total capacity"}
	}
	out := image.NewRGBA(image.Rect(0, 0, r.width, r.height))
	const den uint64 = uint64(WeightDenominator) * 4
	for y := 0; y < r.height; y++ {
		for x := 0; x < r.width; x++ {
			var sums [3]uint64
			for k := 0; k < 4; k++ {
				i := 4*(y*r.width+x) + k
				c := b.colors[i]
				if r.samples[i].depth != 0 {
					c = r.samples[i].color
				}
				if !validColor(c) {
					return nil, fmt.Errorf("invalid palette component")
				}
				for channel := 0; channel < 3; channel++ {
					sums[channel] += (uint64(WeightDenominator)-uint64(c.W24))*uint64(p.Colors[c.Lo][channel]) + uint64(c.W24)*uint64(p.Colors[c.Hi][channel])
				}
			}
			at := y*out.Stride + 4*x
			for ch, n := range sums {
				q, rem := n/den, n%den
				if rem*2 > den || rem*2 == den && q&1 != 0 {
					q++
				}
				out.Pix[at+ch] = byte(q)
			}
			out.Pix[at+3] = 255
		}
	}
	return out, nil
}
