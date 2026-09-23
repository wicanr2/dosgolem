package ebiten

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/presentation"
	"github.com/wicanr2/dosgolem/xlate"
)

type mouseOutput struct{ calls []string }

func (m *mouseOutput) MoveMouse(int, int) { m.calls = append(m.calls, "move") }
func (m *mouseOutput) PressMouse(int)     { m.calls = append(m.calls, "press") }
func (m *mouseOutput) ReleaseMouse(int)   { m.calls = append(m.calls, "release") }

func draftFont(w, h int) *xlate.Font {
	glyphs := map[rune][]byte{}
	for _, r := range "設定套用取消2×3" {
		glyphs[r] = make([]byte, h*((w+7)/8))
	}
	return &xlate.Font{Name: "draft-test", W: w, H: h, Glyphs: glyphs}
}

func newDraftGame(t *testing.T) (*Game, *mouseOutput) {
	t.Helper()
	m := machine.New()
	panel, err := host.NewPanelController(host.OutputScale2)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := presentation.NewKeyboardBridge(panel, m)
	if err != nil {
		t.Fatal(err)
	}
	out := &mouseOutput{}
	mouse, err := host.NewMouseBridge(out)
	if err != nil {
		t.Fatal(err)
	}
	g, err := New(Config{Panel: panel, Keyboard: keys, Mouse: mouse, HostFont2: draftFont(16, 16), HostFont3: draftFont(22, 22), Snapshot: func(int) (presentation.LayerPresentationSnapshot, error) {
		return presentation.LayerPresentationSnapshot{}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	return g, out
}

func TestLayoutEpochOnlyChangesWithHostLayout(t *testing.T) {
	g, out := newDraftGame(t)
	first := g.layout
	if err := g.refreshLayout(); err != nil {
		t.Fatal(err)
	}
	if g.layout.Epoch != first.Epoch {
		t.Fatalf("idle refresh changed epoch %d -> %d", first.Epoch, g.layout.Epoch)
	}
	x, y := 100*int(g.layout.Scale), g.layout.ChromeHeight+82*int(g.layout.Scale)
	g.mouse.Handle(g.layout, host.MouseEvent{Kind: host.MouseEventDown, Button: 0, X: x, Y: y, Target: host.MouseTargetCanvas})
	if err := g.refreshLayout(); err != nil {
		t.Fatal(err)
	}
	g.mouse.Handle(g.layout, host.MouseEvent{Kind: host.MouseEventUp, Button: 0, X: x, Y: y, Target: host.MouseTargetCanvas})
	if got := len(out.calls); got != 4 {
		t.Fatalf("same-layout Down/Up calls=%v", out.calls)
	}
	if _, _, err := g.panel.Route(host.PanelEvent{Kind: host.PanelEventOpen}); err != nil {
		t.Fatal(err)
	}
	if err := g.refreshLayout(); err != nil {
		t.Fatal(err)
	}
	if g.layout.Epoch != first.Epoch+1 || !g.layout.PanelOpen {
		t.Fatalf("open layout=%+v first=%+v", g.layout, first)
	}
}

func TestMappedKeysUseOnlyExistingDOSContract(t *testing.T) {
	if _, ok := mapKey(ebiten.KeyEnter); !ok {
		t.Fatal("Enter missing")
	}
	if _, ok := mapKey(ebiten.KeyA); !ok {
		t.Fatal("A missing")
	}
	if _, ok := mapKey(ebiten.KeyArrowDown); !ok {
		t.Fatal("Down missing")
	}
	if _, ok := mapKey(ebiten.KeyF1); ok {
		t.Fatal("unmapped F1 accepted")
	}
}

func TestRoutePointerConsumesOpenPanelMissAndHostCapture(t *testing.T) {
	g, out := newDraftGame(t)
	if err := g.routePointer(host.MouseEventDown, g.layout.FrameWidth-10, 5); err != nil {
		t.Fatal(err)
	}
	if !g.layout.PanelOpen || !g.mouse.HostCaptured() {
		t.Fatalf("settings down did not host-capture: %+v", g.layout)
	}
	if err := g.routePointer(host.MouseEventUp, 100, g.layout.ChromeHeight+20); err != nil {
		t.Fatal(err)
	}
	if g.mouse.HostCaptured() || len(out.calls) != 0 {
		t.Fatalf("host capture leaked to DOS: %v", out.calls)
	}
	if err := g.routePointer(host.MouseEventDown, 130, 50); err != nil {
		t.Fatal(err)
	}
	if err := g.routePointer(host.MouseEventUp, 130, 50); err != nil {
		t.Fatal(err)
	}
	if len(out.calls) != 0 || g.mouse.Pressed() {
		t.Fatalf("open miss reached DOS: %v", out.calls)
	}
}

func TestApplyCancelAndAcceptedPressCleanup(t *testing.T) {
	g, out := newDraftGame(t)
	if _, _, err := g.panel.Route(host.PanelEvent{Kind: host.PanelEventOpen}); err != nil {
		t.Fatal(err)
	}
	if err := g.refreshLayout(); err != nil {
		t.Fatal(err)
	}
	if err := g.routePointer(host.MouseEventDown, 70*2, 40*2); err != nil {
		t.Fatal(err)
	}
	if err := g.routePointer(host.MouseEventUp, 70*2, 40*2); err != nil {
		t.Fatal(err)
	}
	if _, _, err := g.panel.Route(host.PanelEvent{Kind: host.PanelEventCancel}); err != nil {
		t.Fatal(err)
	}
	if err := g.refreshLayout(); err != nil {
		t.Fatal(err)
	}
	state, _ := g.panel.Snapshot()
	if state.Open || state.Scales.ActiveScale != host.OutputScale2 || state.Scales.SelectedScale != host.OutputScale2 {
		t.Fatalf("cancel=%+v", state)
	}
	if _, _, err := g.panel.Route(host.PanelEvent{Kind: host.PanelEventOpen}); err != nil {
		t.Fatal(err)
	}
	if err := g.refreshLayout(); err != nil {
		t.Fatal(err)
	}
	if err := g.routePointer(host.MouseEventDown, 70*2, 40*2); err != nil {
		t.Fatal(err)
	}
	if err := g.routePointer(host.MouseEventUp, 70*2, 40*2); err != nil {
		t.Fatal(err)
	}
	if err := g.routePointer(host.MouseEventDown, 150*2, 70*2); err != nil {
		t.Fatal(err)
	}
	if err := g.routePointer(host.MouseEventUp, 150*3, 70*3); err != nil {
		t.Fatal(err)
	}
	state, _ = g.panel.Snapshot()
	if state.Open || state.Scales.ActiveScale != host.OutputScale3 {
		t.Fatalf("apply=%+v", state)
	}
	x, y := 100*3, g.layout.ChromeHeight+82*3
	g.mouse.Handle(g.layout, host.MouseEvent{Kind: host.MouseEventDown, Button: 0, X: x, Y: y, Target: host.MouseTargetCanvas})
	if _, _, err := g.panel.Route(host.PanelEvent{Kind: host.PanelEventOpen}); err != nil {
		t.Fatal(err)
	}
	if err := g.refreshLayout(); err != nil {
		t.Fatal(err)
	}
	r := g.mouse.Handle(g.layout, host.MouseEvent{Kind: host.MouseEventFocusLost})
	if !r.Cleanup || len(out.calls) != 3 || out.calls[2] != "release" {
		t.Fatalf("focus cleanup=%+v calls=%v", r, out.calls)
	}
}

func TestSnapshotAndFontFailClosed(t *testing.T) {
	g, _ := newDraftGame(t)
	state, _ := g.panel.Snapshot()
	good := presentation.LayerPresentationSnapshot{Scale: 2, Frame: host.PresentationSnapshot{Canvas: host.Canvas{Width: 320, Height: 200}}, RGBA: make([]byte, 320*2*200*2*4)}
	if _, _, err := g.validateSnapshot(state, good); err != nil {
		t.Fatal(err)
	}
	bad := good
	bad.Frame.Canvas.Width = 321
	if _, _, err := g.validateSnapshot(state, bad); err == nil {
		t.Fatal("wrong canvas accepted")
	}
	bad = good
	bad.RGBA = bad.RGBA[:1]
	if _, _, err := g.validateSnapshot(state, bad); err == nil {
		t.Fatal("wrong rgba accepted")
	}
	m := machine.New()
	p, _ := host.NewPanelController(host.OutputScale2)
	k, _ := presentation.NewKeyboardBridge(p, m)
	mo, _ := host.NewMouseBridge(&mouseOutput{})
	if _, err := New(Config{Panel: p, Keyboard: k, Mouse: mo, HostFont2: &xlate.Font{W: 1, H: 1, Glyphs: map[rune][]byte{}}, HostFont3: draftFont(22, 22), Snapshot: func(int) (presentation.LayerPresentationSnapshot, error) { return good, nil }}); err == nil {
		t.Fatal("missing host glyphs accepted")
	}
}
