package buckrogers

import (
	"bytes"
	"fmt"
	"unicode"

	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/xlate"
)

// RuntimePostJoinExitPromptOverlay owns only RGBA output. Its tail sentinel
// verifies every Draw leaves the six original cells byte-for-byte intact.
type RuntimePostJoinExitPromptOverlay struct {
	layer  *xlate.Layer
	font   *xlate.Font
	scale  int
	active *PostJoinExitPromptGeneration
}

func NewRuntimePostJoinExitPromptOverlay(font *xlate.Font, scale int) (*RuntimePostJoinExitPromptOverlay, error) {
	if font == nil || font.W != 16 || font.H != 16 || (scale != 2 && scale != 3) {
		return nil, fmt.Errorf("Exit prompt presenter inputs invalid")
	}
	return &RuntimePostJoinExitPromptOverlay{layer: &xlate.Layer{W: 320, H: 200}, font: font, scale: scale}, nil
}
func (o *RuntimePostJoinExitPromptOverlay) Apply(g PostJoinExitPromptGeneration, p [256][3]uint8) error {
	if o == nil || g.Generation == 0 || g.Translation == "" || (g.EventKey != postJoinExitQ1 && g.EventKey != postJoinExitQ2) || (g.EventKey == postJoinExitQ1 && g.Width != 96) || (g.EventKey == postJoinExitQ2 && g.Width != 240) {
		if o != nil {
			o.Clear()
		}
		return fmt.Errorf("Exit prompt generation invalid")
	}
	if len([]rune(g.Translation)) > int(g.Width/8) {
		o.Clear()
		return fmt.Errorf("Exit prompt text too wide")
	}
	for _, r := range g.Translation {
		if unicode.IsControl(r) {
			o.Clear()
			return fmt.Errorf("Exit prompt control character")
		}
		if r == ' ' {
			continue
		}
		if glyph, ok := o.font.Glyphs[r]; !ok || len(glyph) != 32 {
			o.Clear()
			return fmt.Errorf("Exit prompt missing glyph U+%04X", r)
		}
	}
	stamp := &xlate.Stamp{Key: g.EventKey, X: 0, Y: 192, Cells: int(g.Width / 8), CellW: 8, CellH: 8, Font: o.font, GlyphX: manualGlyphOffset(o.scale), GlyphY: manualGlyphOffset(o.scale), Text: []rune(g.Translation), State: xlate.Shown, BG: p[0], FG: p[14]}
	ink, err := menuInkRect(stamp, o.scale)
	if err != nil || ink.X < 0 || ink.Y < 192*o.scale || ink.X+ink.Width > int(g.Width)*o.scale || ink.Y+ink.Height > 200*o.scale {
		o.Clear()
		return fmt.Errorf("Exit prompt ink exceeds body")
	}
	o.layer = &xlate.Layer{W: 320, H: 200}
	o.layer.Add(stamp)
	o.active = &g
	return nil
}
func (o *RuntimePostJoinExitPromptOverlay) Clear() {
	if o != nil {
		o.layer = &xlate.Layer{W: 320, H: 200}
		o.active = nil
	}
}
func (o *RuntimePostJoinExitPromptOverlay) Prewrite(v machine.VideoWrite) {
	if o == nil || o.active == nil {
		return
	}
	linear, err := NormalizeSkillExitVideoOffset(v.Offset)
	if err != nil {
		o.Clear()
		return
	}
	if postJoinExitBodyIntersects(linear-skillExitVideoBase, o.active.Width) {
		o.Clear()
	}
}
func (o *RuntimePostJoinExitPromptOverlay) Draw(indexed []byte, p [256][3]uint8) ([]byte, []rune, bool, error) {
	if o == nil || len(indexed) != 320*200 {
		return nil, nil, false, fmt.Errorf("Exit prompt frame invalid")
	}
	rgba := ScaleIndexedRGBA(indexed, p, o.scale)
	var sentinel []byte
	if o.active != nil {
		for y := 192 * o.scale; y < 200*o.scale; y++ {
			a := (y*320*o.scale + int(o.active.Width)*o.scale) * 4
			b := (y*320*o.scale + int(o.active.Width+48)*o.scale) * 4
			sentinel = append(sentinel, rgba[a:b]...)
		}
	}
	var missing []rune
	drew := o.layer.Draw(rgba, o.scale, func(r rune) { missing = append(missing, r) })
	if o.active != nil {
		var tail []byte
		for y := 192 * o.scale; y < 200*o.scale; y++ {
			a := (y*320*o.scale + int(o.active.Width)*o.scale) * 4
			b := (y*320*o.scale + int(o.active.Width+48)*o.scale) * 4
			tail = append(tail, rgba[a:b]...)
		}
		if !bytes.Equal(sentinel, tail) {
			o.Clear()
			return nil, nil, false, fmt.Errorf("Exit prompt protected suffix changed")
		}
	}
	return rgba, missing, drew, nil
}
func (o *RuntimePostJoinExitPromptOverlay) ActiveKeys() []string {
	if o == nil || o.active == nil {
		return nil
	}
	return []string{o.active.EventKey}
}

type PostJoinExitPromptOwner struct {
	Watcher   *PostJoinExitPromptWatcher
	Presenter *RuntimePostJoinExitPromptOverlay
}

func NewPostJoinExitPromptOwner(c *PostJoinExitPromptCatalog, font *xlate.Font, scale int) (*PostJoinExitPromptOwner, error) {
	w, e := NewPostJoinExitPromptWatcher(c)
	if e != nil {
		return nil, e
	}
	p, e := NewRuntimePostJoinExitPromptOverlay(font, scale)
	if e != nil {
		return nil, e
	}
	return &PostJoinExitPromptOwner{w, p}, nil
}
func (o *PostJoinExitPromptOwner) ObserveEntry(e TextEvent) error {
	if err := o.Watcher.ObserveEntry(e); err != nil {
		o.Presenter.Clear()
		return err
	}
	return nil
}
func (o *PostJoinExitPromptOwner) ObserveReturn(e TextEvent, p [256][3]uint8) error {
	g, err := o.Watcher.ObserveReturn(e)
	if err != nil {
		o.Presenter.Clear()
		return err
	}
	if err = o.Presenter.Apply(g, p); err != nil {
		o.Fault()
	}
	return err
}
func (o *PostJoinExitPromptOwner) Prewrite(v machine.VideoWrite) error {
	cleared, err := o.Watcher.Prewrite(v)
	if err != nil {
		o.Presenter.Clear()
		return err
	}
	if cleared {
		o.Presenter.Prewrite(v)
	}
	return nil
}
func (o *PostJoinExitPromptOwner) Stop()          { o.Watcher.Stop(); o.Presenter.Clear() }
func (o *PostJoinExitPromptOwner) Restore()       { o.Watcher.Restore(); o.Presenter.Clear() }
func (o *PostJoinExitPromptOwner) Discontinuity() { o.Watcher.Discontinuity(); o.Presenter.Clear() }
func (o *PostJoinExitPromptOwner) Fault()         { o.Watcher.Fault(); o.Presenter.Clear() }
