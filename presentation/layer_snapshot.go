package presentation

import (
	"fmt"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/xlate"
)

// LayerPresentationSnapshot is a host-owned, scaled RGBA presentation image.
// It contains copied indexed input and a newly allocated RGBA result; it does
// not expose Machine, xlate.Layer, or any input API.
type LayerPresentationSnapshot struct {
	Frame   host.PresentationSnapshot
	Scale   int
	RGBA    []uint8
	Missing []rune
	Drew    bool
}

// LayerSnapshotProvider combines one FrameSource with the currently active
// xlate.Layer. Snapshot must be called on the same goroutine that owns both
// the machine stepping loop and layer lifecycle.
//
// The layer owner remains responsible for calling Layer.Frame with the same
// logical input before Snapshot. Snapshot itself never calls Frame: Frame can
// expire stamps and invoke watchers/OnDrop, while this boundary is strictly a
// read-only presentation projection. To keep that projection immutable, it
// serializes the active stamps and restores them into a private temporary
// layer before Draw.
//
// xlate.Layer snapshots do not include watchers, Frozen, or OnDrop. This is
// intentional: those are lifecycle hooks, not the already-active stamp state
// the presenter may draw. A frontend must not use this provider to advance an
// overlay lifecycle.
type LayerSnapshotProvider struct {
	frames *host.PresentationSnapshotProvider
	layer  *xlate.Layer
	fonts  map[string]*xlate.Font
}

// NewLayerSnapshotProvider constructs a read-only active-layer projection.
// fonts maps every named active stamp font to its loaded font. The map is
// copied so later caller map mutations cannot alter this provider's lookup.
func NewLayerSnapshotProvider(source host.FrameSource, layer *xlate.Layer, fonts map[string]*xlate.Font) (*LayerSnapshotProvider, error) {
	frames, err := host.NewPresentationSnapshotProvider(source)
	if err != nil {
		return nil, err
	}
	if layer == nil {
		return nil, fmt.Errorf("presentation: active xlate layer 不得為 nil")
	}
	fontCopy := make(map[string]*xlate.Font, len(fonts))
	for name, font := range fonts {
		if name == "" || font == nil || font.Name != name {
			return nil, fmt.Errorf("presentation: xlate font map 含無效項目 %q", name)
		}
		fontCopy[name] = font
	}
	return &LayerSnapshotProvider{frames: frames, layer: layer, fonts: fontCopy}, nil
}

// Snapshot obtains one machine frame and immediately projects the already
// active xlate stamps into a private RGBA buffer. scale must be positive.
func (p *LayerSnapshotProvider) Snapshot(scale int) (LayerPresentationSnapshot, error) {
	if p == nil || p.frames == nil || p.layer == nil {
		return LayerPresentationSnapshot{}, fmt.Errorf("presentation: LayerSnapshotProvider 不得為 nil")
	}
	if scale <= 0 {
		return LayerPresentationSnapshot{}, fmt.Errorf("presentation: 輸出倍率必須為正數，得到 %d", scale)
	}
	frame, err := p.frames.Snapshot()
	if err != nil {
		return LayerPresentationSnapshot{}, err
	}
	if err := p.validateActiveLayer(frame.Canvas); err != nil {
		return LayerPresentationSnapshot{}, err
	}
	copy, err := p.copyActiveLayer()
	if err != nil {
		return LayerPresentationSnapshot{}, err
	}
	rgba, err := scaleIndexedRGBA(frame, scale)
	if err != nil {
		return LayerPresentationSnapshot{}, err
	}
	missing := []rune{}
	drew := copy.Draw(rgba, scale, func(r rune) { missing = append(missing, r) })
	return LayerPresentationSnapshot{
		Frame: frame, Scale: scale, RGBA: rgba,
		Missing: append([]rune(nil), missing...), Drew: drew,
	}, nil
}

func (p *LayerSnapshotProvider) validateActiveLayer(canvas host.Canvas) error {
	w, h := p.layer.W, p.layer.H
	if w == 0 {
		w = 320
	}
	if h == 0 {
		h = 200
	}
	if w != canvas.Width || h != canvas.Height {
		return fmt.Errorf("presentation: xlate layer 尺寸 %dx%d 與畫布 %dx%d 不符", w, h, canvas.Width, canvas.Height)
	}
	for _, stamp := range p.layer.Stamps {
		if stamp == nil || stamp.Font == nil {
			continue
		}
		if stamp.Font.Name == "" {
			return fmt.Errorf("presentation: active stamp %q 的 font 未命名，不能建立不可變 snapshot", stamp.Key)
		}
		if p.fonts[stamp.Font.Name] != stamp.Font {
			return fmt.Errorf("presentation: active stamp %q 的 font %q 未登錄或不是同一個字型", stamp.Key, stamp.Font.Name)
		}
	}
	return nil
}

func (p *LayerSnapshotProvider) copyActiveLayer() (*xlate.Layer, error) {
	data, err := p.layer.Snapshot()
	if err != nil {
		return nil, fmt.Errorf("presentation: snapshot active xlate layer: %w", err)
	}
	copy := &xlate.Layer{}
	if err := copy.Restore(data, p.fonts); err != nil {
		return nil, fmt.Errorf("presentation: restore active xlate layer copy: %w", err)
	}
	return copy, nil
}

func scaleIndexedRGBA(frame host.PresentationSnapshot, scale int) ([]uint8, error) {
	if frame.Canvas.Width <= 0 || frame.Canvas.Height <= 0 || scale <= 0 {
		return nil, fmt.Errorf("presentation: 無效的畫布或倍率")
	}
	maxInt := int(^uint(0) >> 1)
	if frame.Canvas.Width > maxInt/scale || frame.Canvas.Height > maxInt/scale {
		return nil, fmt.Errorf("presentation: 放大後畫布尺寸溢位")
	}
	w, h := frame.Canvas.Width*scale, frame.Canvas.Height*scale
	if w > maxInt/h || w*h > maxInt/4 {
		return nil, fmt.Errorf("presentation: RGBA 長度溢位")
	}
	rgba := make([]uint8, w*h*4)
	for y := 0; y < frame.Canvas.Height; y++ {
		for x := 0; x < frame.Canvas.Width; x++ {
			color := frame.Palette[frame.Indexed[y*frame.Canvas.Width+x]]
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					i := ((y*scale+dy)*w + x*scale + dx) * 4
					rgba[i], rgba[i+1], rgba[i+2], rgba[i+3] = color[0], color[1], color[2], 255
				}
			}
		}
	}
	return rgba, nil
}
