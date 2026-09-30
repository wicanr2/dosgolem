package buckrogers

import (
	"fmt"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/xlate"
)

// RuntimeSkillExitOverlay paints only the logical question body; the six-cell
// coloured suffix is intentionally outside both Clear and Stamp rectangles.
type RuntimeSkillExitOverlay struct {
	layer  *xlate.Layer
	font   *xlate.Font
	half   *xlate.Font // spec 039 §3.2; nil when the derivation failed
	scale  int
	active *SkillExitGeneration
}

func NewRuntimeSkillExitOverlay(font *xlate.Font, scale int) (*RuntimeSkillExitOverlay, error) {
	if font == nil || font.W != 16 || font.H != 16 || (scale != 2 && scale != 3) {
		return nil, fmt.Errorf("skill-exit presenter inputs invalid")
	}
	return &RuntimeSkillExitOverlay{layer: &xlate.Layer{W: 320, H: 200}, font: font,
		half: halfFontsOf(font).For(scale), scale: scale}, nil
}
func (o *RuntimeSkillExitOverlay) Apply(g SkillExitGeneration, p [256][3]uint8) error {
	if o == nil || g.Generation == 0 || g.Translation == "" {
		if o != nil {
			o.Clear()
		}
		return fmt.Errorf("skill-exit generation invalid")
	}
	width := 264
	if g.Page == SkillExitTechnical {
		width = 272
	}
	// Spec 039 §3.1: capacity in half units, two per original cell.
	translation := []rune(g.Translation)
	if textUnits(translation) > 2*(width/8) {
		o.Clear()
		return fmt.Errorf("skill-exit translation exceeds body")
	}
	fonts := segmentFonts{Full: o.font, FullX: manualGlyphOffset(o.scale), FullY: manualGlyphOffset(o.scale), Half: o.half}
	if miss := fonts.missingRunes(translation); len(miss) != 0 {
		o.Clear()
		return fmt.Errorf("skill-exit missing glyph U+%04X", miss[0])
	}
	stamps := fonts.segmentStamps(g.EventKey, 0, 192, padUnits(translation, 2*(width/8)), nil,
		func(int) ([3]uint8, [3]uint8) { return p[0], p[13] })
	ink, err := menuInkRectAll(stamps, o.scale)
	clear := PixelRect{0, 192 * o.scale, width * o.scale, 8 * o.scale}
	if err != nil || ink.X < clear.X || ink.Y < clear.Y || ink.X+ink.Width > clear.X+clear.Width || ink.Y+ink.Height > clear.Y+clear.Height {
		o.Clear()
		return fmt.Errorf("skill-exit ink outside body")
	}
	o.layer = &xlate.Layer{W: 320, H: 200}
	for _, stamp := range stamps {
		o.layer.Add(stamp)
	}
	o.active = &g
	return nil
}
func (o *RuntimeSkillExitOverlay) Clear() {
	if o != nil {
		o.layer = &xlate.Layer{W: 320, H: 200}
		o.active = nil
	}
}
func (o *RuntimeSkillExitOverlay) Prewrite(v machine.VideoWrite) (bool, error) {
	if o == nil || o.active == nil {
		return false, nil
	}
	off, err := NormalizeSkillExitVideoOffset(v.Offset)
	if err != nil {
		o.Clear()
		return false, err
	}
	lo, hi := skillExitBody(o.active.Page)
	if off-skillExitVideoBase >= lo && off-skillExitVideoBase < hi {
		o.layer.Clear(0, 192, int(hi-lo), 200)
		o.active = nil
		return true, nil
	}
	return false, nil
}
func (o *RuntimeSkillExitOverlay) Draw(indexed []byte, p [256][3]uint8) ([]byte, []rune, bool) {
	rgba := ScaleIndexedRGBA(indexed, p, o.scale)
	var miss []rune
	d := o.layer.Draw(rgba, o.scale, func(r rune) { miss = append(miss, r) })
	return rgba, miss, d
}
func (o *RuntimeSkillExitOverlay) ActiveKeys() []string {
	if o == nil || o.active == nil {
		return nil
	}
	return []string{o.active.EventKey}
}

// SkillExitOwner is the only lifecycle owner used by the runner.  Lifecycle
// transitions always clear watcher and presenter together.
type SkillExitOwner struct {
	Watcher   *SkillExitWatcher
	Presenter *RuntimeSkillExitOverlay
	// missing is the original body band of a question this language does
	// not translate (spec 040 §3.2); nil when the presenter drew it.
	missing *PixelRect
	// Missing counts untranslated questions (the original English shows).
	Missing int
}

// MissingRect is the row-24 clear band of the last untranslated question
// (spec 040 §3.2 yield rectangle), or nil.
func (o *SkillExitOwner) MissingRect() *PixelRect { return o.missing }

func NewSkillExitOwner(c *SkillExitCatalog, font *xlate.Font, scale int) (*SkillExitOwner, error) {
	w, e := NewSkillExitWatcher(c)
	if e != nil {
		return nil, e
	}
	p, e := NewRuntimeSkillExitOverlay(font, scale)
	if e != nil {
		return nil, e
	}
	return &SkillExitOwner{Watcher: w, Presenter: p}, nil
}
func (o *SkillExitOwner) ObserveEntry(e TextEvent) error {
	if err := o.Watcher.ObserveEntry(e); err != nil {
		o.Presenter.Clear()
		return err
	}
	return nil
}
func (o *SkillExitOwner) ObserveReturn(e TextEvent, p [256][3]uint8) error {
	o.missing = nil
	g, err := o.Watcher.ObserveReturn(e)
	if err != nil {
		o.Presenter.Clear()
		return err
	}
	if g.Translation == "" {
		// Spec 040 §3.2: untranslated — clear, keep the watcher, no error.
		lo, hi := skillExitBody(g.Page)
		o.missing = &PixelRect{0, 192, int(hi - lo), 8}
		o.Missing++
		o.Presenter.Clear()
		return nil
	}
	if err = o.Presenter.Apply(g, p); err != nil {
		o.Watcher.Fault()
		o.Presenter.Clear()
	}
	return err
}
func (o *SkillExitOwner) Prewrite(v machine.VideoWrite) error {
	_, err := o.Watcher.Prewrite(v)
	if err != nil {
		o.Presenter.Clear()
		return err
	}
	_, err = o.Presenter.Prewrite(v)
	if err != nil {
		o.Watcher.Fault()
		o.Presenter.Clear()
	}
	return err
}
func (o *SkillExitOwner) Stop()          { o.Watcher.Stop(); o.Presenter.Clear() }
func (o *SkillExitOwner) Restore()       { o.Watcher.Restore(); o.Presenter.Clear() }
func (o *SkillExitOwner) Discontinuity() { o.Watcher.Discontinuity(); o.Presenter.Clear() }
func (o *SkillExitOwner) Fault()         { o.Watcher.Fault(); o.Presenter.Clear() }
