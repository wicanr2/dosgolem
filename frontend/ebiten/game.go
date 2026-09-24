// Package ebiten provides the Linux Ebitengine host loop.  It deliberately
// owns window input and pixels only; DOS machine stepping and active overlay
// lifecycle remain caller responsibilities on this same goroutine.
package ebiten

import (
	"fmt"
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/presentation"
	"github.com/wicanr2/dosgolem/xlate"
)

type Snapshot func(scale int) (presentation.LayerPresentationSnapshot, error)
type Advance func() error

// HostLabels are presentation-only strings supplied by the caller's catalog.
type HostLabels struct {
	Settings, Apply, Cancel, Scale2, Scale3 string
}

func (l HostLabels) all() []string {
	return []string{l.Settings, l.Apply, l.Cancel, l.Scale2, l.Scale3}
}

type Config struct {
	Panel    *host.PanelController
	Keyboard *presentation.KeyboardBridge
	Mouse    *host.MouseBridge
	Snapshot Snapshot
	Advance  Advance
	// OnDrawFault synchronously reports the first inspectable Draw failure to
	// a caller-owned session. It does not make this open Config a sealed owner.
	OnDrawFault func(error)
	HostFont2   *xlate.Font // local 2× font; bytes are never bundled here.
	HostFont3   *HostFont3  // local native 24-point host face; never scale up HostFont2.
	Labels      HostLabels
}

type Game struct {
	panel                *host.PanelController
	keys                 *presentation.KeyboardBridge
	mouse                *host.MouseBridge
	snapshot             Snapshot
	advance              Advance
	onDrawFault          func(error)
	font2                *xlate.Font
	font3                *HostFont3
	labels               HostLabels
	epoch                uint64
	layout               host.MouseLayout
	rgba                 []byte
	width, height        int
	err                  error
	readInput            func() frameInput
	panelEventThisUpdate bool
}

// frameInput is captured once per Update. Tests replace readInput so the
// production scheduling gate can be exercised without synthetic X11 events.
type frameInput struct {
	down, up     bool
	downX, downY int
	upX, upY     int
	focused      bool
	keys         []ebiten.Key
}

func readFrameInput() frameInput {
	in := frameInput{focused: ebiten.IsFocused(), keys: inpututil.AppendJustPressedKeys(nil)}
	in.down = inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	if in.down {
		in.downX, in.downY = ebiten.CursorPosition()
	}
	in.up = inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft)
	if in.up {
		in.upX, in.upY = ebiten.CursorPosition()
	}
	return in
}

func New(cfg Config) (*Game, error) {
	if cfg.Panel == nil || cfg.Keyboard == nil || cfg.Mouse == nil || cfg.Snapshot == nil || cfg.HostFont2 == nil || cfg.HostFont3 == nil {
		return nil, fmt.Errorf("frontend/ebiten: Panel、Keyboard、Mouse、Snapshot、HostFont2、HostFont3 均為必填")
	}
	if err := cfg.Keyboard.ValidateBIOSForPanel(cfg.Panel); err != nil {
		return nil, err
	}
	if err := validateHostFont2(cfg.HostFont2, cfg.Labels); err != nil {
		return nil, err
	}
	if err := validateHostFont3(cfg.HostFont3, cfg.Labels); err != nil {
		return nil, err
	}
	// The caller owns the local font inputs. Pin the validated bytes for this
	// Game so a later change to either source map cannot alter host chrome.
	g := &Game{panel: cfg.Panel, keys: cfg.Keyboard, mouse: cfg.Mouse, snapshot: cfg.Snapshot, advance: cfg.Advance,
		onDrawFault: cfg.OnDrawFault,
		font2:       cloneHostFont(cfg.HostFont2),
		font3:       &HostFont3{Wide: cloneHostFont(cfg.HostFont3.Wide), ASCII: cloneHostFont(cfg.HostFont3.ASCII)},
		labels:      cfg.Labels, readInput: readFrameInput}
	if err := g.refreshLayout(); err != nil {
		return nil, err
	}
	return g, nil
}

func cloneHostFont(source *xlate.Font) *xlate.Font {
	font := &xlate.Font{Name: source.Name, W: source.W, H: source.H, Glyphs: make(map[rune][]byte, len(source.Glyphs))}
	for r, glyph := range source.Glyphs {
		font.Glyphs[r] = append([]byte(nil), glyph...)
	}
	return font
}

func validateHostFont2(font *xlate.Font, labels HostLabels) error {
	if font.W != 16 || font.H != 16 {
		return fmt.Errorf("frontend/ebiten: 2× host 字型必須是 16x16")
	}
	for _, label := range labels.all() {
		if label == "" {
			return fmt.Errorf("frontend/ebiten: host label 為空")
		}
	}
	for _, r := range []rune(labels.Settings + labels.Apply + labels.Cancel + labels.Scale2 + labels.Scale3) {
		glyph, ok := font.Glyphs[r]
		if !ok || font.W <= 0 || font.H <= 0 || len(glyph) != font.H*((font.W+7)/8) {
			return fmt.Errorf("frontend/ebiten: 2× host 字型缺少或損壞字元 %q", r)
		}
		hasInk := false
		for _, b := range glyph {
			hasInk = hasInk || b != 0
		}
		if !hasInk {
			return fmt.Errorf("frontend/ebiten: 2× host 字型 %q 沒有可見墨跡", r)
		}
	}
	for _, item := range []struct {
		text string
		w, h int
	}{{labels.Settings, 72 * 2, 13 * 2}, {labels.Scale2, 54 * 2, 23 * 2}, {labels.Scale3, 54 * 2, 23 * 2}, {labels.Apply, 68 * 2, 24 * 2}, {labels.Cancel, 68 * 2, 24 * 2}} {
		if len([]rune(item.text))*font.W > item.w || font.H > item.h {
			return fmt.Errorf("frontend/ebiten: 2× host 字型不容於 %q 安全矩形", item.text)
		}
	}
	return nil
}

func (g *Game) refreshLayout() error {
	state, err := g.panel.Snapshot()
	if err != nil {
		return err
	}
	s := int(state.Scales.ActiveScale)
	if s != 2 && s != 3 {
		return fmt.Errorf("frontend/ebiten: unsupported scale %d", s)
	}
	chrome := 18 * s
	if state.Open {
		chrome = 92 * s
	}
	if g.layout.Epoch != 0 && g.layout.Scale == state.Scales.ActiveScale && g.layout.PanelOpen == state.Open {
		return nil
	}
	g.epoch++
	g.layout = host.MouseLayout{Epoch: g.epoch, Scale: state.Scales.ActiveScale, ChromeHeight: chrome, Canvas: host.Canvas{Width: 320, Height: 200}, FrameWidth: 320 * s, FrameHeight: chrome + 200*s, PanelOpen: state.Open}
	// Logical output pixels are the selected DOS scale, never a resize-to-fit
	// leftover from the previous scale. This remains frontend-only state.
	ebiten.SetWindowSize(g.layout.FrameWidth, g.layout.FrameHeight)
	return g.mouse.ApplyLayout(g.layout)
}

func (g *Game) Update() error {
	if g.err != nil {
		return g.err
	}
	if err := g.keys.ValidateBIOSForPanel(g.panel); err != nil {
		return g.fail(err)
	}
	if err := g.refreshLayout(); err != nil {
		return g.fail(err)
	}
	before, err := g.panel.Snapshot()
	if err != nil {
		return g.fail(err)
	}
	g.panelEventThisUpdate = false
	in := g.readInput()
	if in.down {
		if err := g.routePointer(host.MouseEventDown, in.downX, in.downY); err != nil {
			return g.fail(err)
		}
	}
	if in.up {
		if err := g.routePointer(host.MouseEventUp, in.upX, in.upY); err != nil {
			return g.fail(err)
		}
	}
	if !in.focused {
		g.mouse.Handle(g.layout, host.MouseEvent{Kind: host.MouseEventFocusLost})
	}
	// The panel owns the entire keyboard batch if it was open on entry. A
	// pointer Apply/Cancel may close it before this loop, but must not make
	// simultaneous keys visible to DOS. A same-batch Open is host-owned too.
	if !before.Open && !g.panelEventThisUpdate {
		for _, key := range in.keys {
			if err := g.key(key); err != nil {
				return g.fail(err)
			}
		}
	}
	after, err := g.panel.Snapshot()
	if err != nil {
		return g.fail(err)
	}
	// A closing Apply/Cancel leaves after.Open=false; the entry state and
	// transition flag still suppress DOS advancement in this same Update.
	if before.Open || after.Open || g.panelEventThisUpdate {
		return nil
	}
	if g.advance != nil {
		if err := g.advance(); err != nil {
			return g.fail(err)
		}
	}
	return nil
}

func (g *Game) fail(err error) error {
	if g.err == nil {
		g.err = err
	}
	return g.err
}

// routePointer is deliberately coordinate-injected for deterministic tests.
// Ebitengine callback plumbing is the only caller in production candidate code.
func (g *Game) routePointer(kind host.MouseEventKind, x, y int) error {
	if kind == host.MouseEventDown {
		if event, hit := g.hostHit(x, y); hit {
			// Capture the host Down before it changes panel geometry.
			g.mouse.Handle(g.layout, host.MouseEvent{Kind: kind, Button: 0, X: x, Y: y, Target: host.MouseTargetHost})
			if _, _, err := g.panel.Route(event); err != nil {
				return err
			}
			g.panelEventThisUpdate = true
			return g.refreshLayout()
		}
	}
	target := host.MouseTargetOutside
	if !g.layout.PanelOpen && x >= 0 && x < g.layout.FrameWidth && y >= g.layout.ChromeHeight && y < g.layout.FrameHeight {
		target = host.MouseTargetCanvas
	}
	// Open-panel blank misses are deliberately host-consumed by MouseBridge.
	g.mouse.Handle(g.layout, host.MouseEvent{Kind: kind, Button: 0, X: x, Y: y, Target: target})
	return nil
}

func (g *Game) hostHit(x, y int) (host.PanelEvent, bool) {
	s := int(g.layout.Scale)
	w := g.layout.FrameWidth
	in := func(x0, y0, x1, y1 int) bool { return x >= x0 && x < x1 && y >= y0 && y < y1 }
	if in(w-76*s, 2*s, w-4*s, 15*s) && !g.layout.PanelOpen {
		return host.PanelEvent{Kind: host.PanelEventOpen}, true
	}
	if !g.layout.PanelOpen {
		return host.PanelEvent{}, false
	}
	switch {
	case in(8*s, 35*s, 62*s, 58*s):
		return host.PanelEvent{Kind: host.PanelEventSelectScale, Scale: host.OutputScale2}, true
	case in(68*s, 35*s, 122*s, 58*s):
		return host.PanelEvent{Kind: host.PanelEventSelectScale, Scale: host.OutputScale3}, true
	case in(145*s, 63*s, 213*s, 87*s):
		return host.PanelEvent{Kind: host.PanelEventApply}, true
	case in(220*s, 63*s, 288*s, 87*s):
		return host.PanelEvent{Kind: host.PanelEventCancel}, true
	}
	return host.PanelEvent{}, false
}

func (g *Game) key(k ebiten.Key) error {
	key, ok := mapKey(k)
	if !ok {
		return nil
	}
	_, _, err := g.keys.DeliverBIOSKey(key)
	return err
}

func mapKey(k ebiten.Key) (dos.Key, bool) {
	named := map[ebiten.Key]string{
		ebiten.KeyEnter: "Enter", ebiten.KeyEscape: "Escape", ebiten.KeyBackspace: "Backspace", ebiten.KeyTab: "Tab",
		ebiten.KeySpace: "Space", ebiten.KeyArrowUp: "Up", ebiten.KeyArrowDown: "Down", ebiten.KeyArrowLeft: "Left", ebiten.KeyArrowRight: "Right",
	}
	if n, ok := named[k]; ok {
		return dos.KeyNamed(n)
	}
	if k >= ebiten.KeyA && k <= ebiten.KeyZ {
		return dos.KeyForRune(rune('A' + k - ebiten.KeyA))
	}
	if k >= ebiten.Key0 && k <= ebiten.Key9 {
		return dos.KeyForRune(rune('0' + k - ebiten.Key0))
	}
	return dos.Key{}, false
}

func (g *Game) Draw(screen *ebiten.Image) {
	state, err := g.panel.Snapshot()
	if err != nil {
		g.drawFault(err)
		return
	}
	s, err := g.snapshot(int(state.Scales.ActiveScale))
	if err != nil {
		g.drawFault(err)
		return
	}
	w, h, err := g.validateSnapshot(state, s)
	if err != nil {
		g.drawFault(err)
		return
	}
	canvas := ebiten.NewImage(w, h)
	canvas.WritePixels(s.RGBA)
	screen.Fill(color.RGBA{16, 24, 39, 255})
	screen.DrawImage(canvas, &ebiten.DrawImageOptions{GeoM: func() ebiten.GeoM { var m ebiten.GeoM; m.Translate(0, float64(g.layout.ChromeHeight)); return m }()})
	if err := g.drawChrome(screen, state); err != nil {
		g.drawFault(err)
	}
}

func (g *Game) drawFault(err error) {
	if err == nil || g.err != nil {
		return
	}
	g.err = err
	if g.onDrawFault != nil {
		g.onDrawFault(err)
	}
}

func (g *Game) validateSnapshot(state host.PanelState, s presentation.LayerPresentationSnapshot) (int, int, error) {
	scale := int(state.Scales.ActiveScale)
	if scale != 2 && scale != 3 || s.Scale != scale || s.Frame.Canvas.Width != 320 || s.Frame.Canvas.Height != 200 {
		return 0, 0, fmt.Errorf("frontend/ebiten: snapshot canvas/scale invalid")
	}
	if g.layout.Scale != state.Scales.ActiveScale || g.layout.Canvas.Width != 320 || g.layout.Canvas.Height != 200 || g.layout.FrameWidth != 320*scale || g.layout.FrameHeight != g.layout.ChromeHeight+200*scale {
		return 0, 0, fmt.Errorf("frontend/ebiten: current layout inconsistent with snapshot")
	}
	w, h := 320*scale, 200*scale
	if len(s.RGBA) != w*h*4 {
		return 0, 0, fmt.Errorf("frontend/ebiten: snapshot RGBA length invalid")
	}
	return w, h, nil
}

func (g *Game) Layout(_, _ int) (int, int) { return g.layout.FrameWidth, g.layout.FrameHeight }

func (g *Game) drawChrome(screen *ebiten.Image, state host.PanelState) error {
	fill := func(r image.Rectangle, c color.Color) {
		i := ebiten.NewImage(r.Dx(), r.Dy())
		i.Fill(c)
		o := &ebiten.DrawImageOptions{}
		o.GeoM.Translate(float64(r.Min.X), float64(r.Min.Y))
		screen.DrawImage(i, o)
	}
	s := int(state.Scales.ActiveScale)
	w := g.layout.FrameWidth
	drawLabel := func(x, y int, label string) error {
		if s == 3 {
			return g.drawText3(screen, x, y, label, color.RGBA{255, 255, 255, 255})
		}
		g.drawText(screen, g.font2, x, y, label, color.RGBA{255, 255, 255, 255})
		return nil
	}
	fill(image.Rect(0, 0, w, g.layout.ChromeHeight), color.RGBA{16, 24, 39, 255})
	if !state.Open {
		fill(image.Rect(w-76*s, 2*s, w-4*s, 15*s), color.RGBA{37, 99, 235, 255})
		return drawLabel(w-72*s, 2*s+2, g.labels.Settings)
	}
	fill(image.Rect(8*s, 35*s, 62*s, 58*s), choose(state.Scales.SelectedScale == host.OutputScale2))
	fill(image.Rect(68*s, 35*s, 122*s, 58*s), choose(state.Scales.SelectedScale == host.OutputScale3))
	fill(image.Rect(145*s, 63*s, 213*s, 87*s), color.RGBA{22, 163, 74, 255})
	fill(image.Rect(220*s, 63*s, 288*s, 87*s), color.RGBA{100, 116, 139, 255})
	for _, item := range []struct {
		x, y int
		text string
	}{{12 * s, 38 * s, g.labels.Scale2}, {72 * s, 38 * s, g.labels.Scale3}, {150 * s, 66 * s, g.labels.Apply}, {225 * s, 66 * s, g.labels.Cancel}} {
		if err := drawLabel(item.x, item.y, item.text); err != nil {
			return err
		}
	}
	return nil
}
func (g *Game) drawText(screen *ebiten.Image, font *xlate.Font, x, y int, text string, c color.Color) {
	bytesPerRow := (font.W + 7) / 8
	for _, r := range text {
		glyph := font.Glyphs[r]
		for gy := 0; gy < font.H; gy++ {
			for gx := 0; gx < font.W; gx++ {
				if glyph[gy*bytesPerRow+gx/8]&(0x80>>uint(gx%8)) != 0 {
					screen.Set(x+gx, y+gy, c)
				}
			}
		}
		x += font.W
	}
}
func choose(selected bool) color.RGBA {
	if selected {
		return color.RGBA{22, 163, 74, 255}
	}
	return color.RGBA{71, 85, 105, 255}
}
