package ebiten

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/presentation"
	"github.com/wicanr2/dosgolem/xlate"
)

type mouseOutput struct{ calls []string }

func draftLabels() HostLabels {
	return HostLabels{Settings: "設定", Apply: "套用", Cancel: "取消", Scale2: "2×", Scale3: "3×"}
}

func TestUpdatePausesAdvanceForPanelTurns(t *testing.T) {
	g, out := newDraftGame(t)
	steps := 0
	g.advance = func() error { steps++; return nil }
	input := frameInput{focused: true}
	g.readInput = func() frameInput { return input }
	update := func(want int) {
		t.Helper()
		if err := g.Update(); err != nil {
			t.Fatal(err)
		}
		if steps != want {
			t.Fatalf("Advance calls=%d want=%d input=%+v", steps, want, input)
		}
	}
	down := func(x, y int) { input = frameInput{focused: true, down: true, downX: x, downY: y} }
	up := func(x, y int) { input = frameInput{focused: true, up: true, upX: x, upY: y} }
	idle := func() { input = frameInput{focused: true} }

	idle()
	update(1)
	down(630, 8) // Open at 2×.
	update(1)
	up(630, 8)
	update(1)
	idle()
	update(1)     // Sustained open panel.
	down(140, 80) // Select 3×, but active remains 2×.
	update(1)
	up(140, 80)
	update(1)
	down(450, 140) // Cancel closes the panel.
	update(1)
	up(450, 140)
	update(2) // First closed turn, including host-captured release.

	down(630, 8)
	update(2)
	up(630, 8)
	update(2)
	down(140, 80)
	update(2)
	up(140, 80)
	update(2)
	down(300, 140) // Apply commits 3× and closes the panel.
	update(2)
	up(300, 140)
	update(3)
	state, err := g.panel.Snapshot()
	if err != nil || state.Open || state.Scales.ActiveScale != host.OutputScale3 {
		t.Fatalf("Apply state=%+v err=%v", state, err)
	}
	if len(out.calls) != 0 {
		t.Fatalf("host panel pointer reached DOS mouse: %v", out.calls)
	}
}

func TestUpdateKeepsPanelTransitionPauseWhenEntryAndExitAreClosed(t *testing.T) {
	g, _ := newDraftGame(t)
	calls := 0
	g.advance = func() error { calls++; return nil }
	g.readInput = func() frameInput {
		if err := g.routePointer(host.MouseEventDown, 630, 8); err != nil {
			t.Fatal(err)
		}
		if err := g.routePointer(host.MouseEventDown, 450, 140); err != nil {
			t.Fatal(err)
		}
		return frameInput{focused: true}
	}
	if err := g.Update(); err != nil || calls != 0 {
		t.Fatalf("closed→open→closed turn err=%v Advance calls=%d", err, calls)
	}
	g.readInput = func() frameInput { return frameInput{focused: true} }
	if err := g.Update(); err != nil || calls != 1 {
		t.Fatalf("next closed turn err=%v Advance calls=%d", err, calls)
	}
}

func TestUpdateFailureNeverAdvancesAgain(t *testing.T) {
	g, _ := newDraftGame(t)
	calls := 0
	want := errors.New("injected Advance failure")
	g.advance = func() error { calls++; return want }
	g.readInput = func() frameInput { return frameInput{focused: true} }
	if err := g.Update(); !errors.Is(err, want) || calls != 1 {
		t.Fatalf("first Update err=%v Advance calls=%d", err, calls)
	}
	if err := g.Update(); !errors.Is(err, want) || calls != 1 {
		t.Fatalf("failed Update retried err=%v Advance calls=%d", err, calls)
	}
	bad, _ := newDraftGame(t)
	badCalls := 0
	bad.advance = func() error { badCalls++; return nil }
	bad.panel = nil // Panel state/layout failure before input or Advance.
	bad.readInput = func() frameInput { return frameInput{focused: true} }
	if err := bad.Update(); err == nil || badCalls != 0 {
		t.Fatalf("invalid panel err=%v Advance calls=%d", err, badCalls)
	}
	if err := bad.Update(); err == nil || badCalls != 0 {
		t.Fatalf("invalid panel retried err=%v Advance calls=%d", err, badCalls)
	}
	route, _ := newDraftGame(t)
	routeCalls := 0
	route.advance = func() error { routeCalls++; return nil }
	route.keys = nil // Inject a keyboard route failure without a DOS write.
	route.readInput = func() frameInput { return frameInput{focused: true, keys: []ebiten.Key{ebiten.KeyEnter}} }
	if err := route.Update(); err == nil || routeCalls != 0 {
		t.Fatalf("keyboard route failure err=%v Advance calls=%d", err, routeCalls)
	}
	route.readInput = func() frameInput { return frameInput{focused: true} }
	if err := route.Update(); err == nil || routeCalls != 0 {
		t.Fatalf("keyboard route failure retried err=%v Advance calls=%d", err, routeCalls)
	}
	layout, _ := newDraftGame(t)
	layoutCalls := 0
	layout.advance = func() error { layoutCalls++; return nil }
	layout.mouse = nil // Initial unchanged layout is valid; Open forces ApplyLayout failure.
	layout.readInput = func() frameInput { return frameInput{focused: true, down: true, downX: 630, downY: 8} }
	if err := layout.Update(); err == nil || layoutCalls != 0 {
		t.Fatalf("post-transition layout failure err=%v Advance calls=%d", err, layoutCalls)
	}
	layout.readInput = func() frameInput { return frameInput{focused: true} }
	if err := layout.Update(); err == nil || layoutCalls != 0 {
		t.Fatalf("post-transition layout failure retried err=%v Advance calls=%d", err, layoutCalls)
	}
}

func TestUpdateMixedHostAndDOSInputKeepsPauseGate(t *testing.T) {
	m := machine.New()
	bios := dos.New(m, ".")
	bios.Install()
	panel, err := host.NewPanelController(host.OutputScale2)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := presentation.NewKeyboardBridgeWithBIOS(panel, m, bios)
	if err != nil {
		t.Fatal(err)
	}
	out := &mouseOutput{}
	mouse, err := host.NewMouseBridge(out)
	if err != nil {
		t.Fatal(err)
	}
	g, err := New(Config{Panel: panel, Keyboard: keys, Mouse: mouse,
		HostFont2: draftFont(16, 16), HostFont3: draftHostFont3(), Labels: draftLabels(),
		Snapshot: func(int) (presentation.LayerPresentationSnapshot, error) {
			return presentation.LayerPresentationSnapshot{}, nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	g.advance = func() error { calls++; return nil }
	run := func(in frameInput, wantCalls, wantKeys int) {
		t.Helper()
		g.readInput = func() frameInput { return in }
		if err := g.Update(); err != nil {
			t.Fatal(err)
		}
		if calls != wantCalls || bios.KeysPending() != wantKeys {
			t.Fatalf("mixed Update input=%+v Advance=%d BIOS=%d want=%d/%d", in, calls, bios.KeysPending(), wantCalls, wantKeys)
		}
	}
	// Pointer is routed before keyboard in the existing Game.Update. Open and
	// Select keep the panel open, so the same-frame key is host-consumed.
	run(frameInput{focused: true, down: true, downX: 630, downY: 8, keys: []ebiten.Key{ebiten.KeyEnter}}, 0, 0)
	run(frameInput{focused: true, up: true, upX: 630, upY: 8}, 0, 0)
	run(frameInput{focused: true, down: true, downX: 140, downY: 80, keys: []ebiten.Key{ebiten.KeyEnter}}, 0, 0)
	run(frameInput{focused: true, up: true, upX: 140, upY: 80}, 0, 0)
	// Apply closes the panel, but a key captured in the same frame still
	// belongs to the panel rather than entering the BIOS queue.
	run(frameInput{focused: true, down: true, downX: 300, downY: 140, keys: []ebiten.Key{ebiten.KeyEnter}}, 0, 0)
	run(frameInput{focused: true, up: true, upX: 300, upY: 140}, 1, 0)
	if len(out.calls) != 0 {
		t.Fatalf("host pointer reached DOS mouse: %v", out.calls)
	}
	// Closed-panel canvas pointer and key still follow their existing DOS
	// routes and advance once; the pause gate must not swallow either.
	y := g.layout.ChromeHeight + 120
	run(frameInput{focused: true, down: true, downX: 200, downY: y, keys: []ebiten.Key{ebiten.KeyEnter}}, 2, 1)
	if len(out.calls) != 2 || out.calls[0] != "move" || out.calls[1] != "press" {
		t.Fatalf("closed canvas pointer route=%v", out.calls)
	}
}

func TestUpdateClosingPanelConsumesWholeKeyboardBatch(t *testing.T) {
	for _, tc := range []struct {
		name      string
		x         int
		wantScale host.OutputScale
	}{
		{name: "Apply", x: 300, wantScale: host.OutputScale3},
		{name: "Cancel", x: 450, wantScale: host.OutputScale2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := machine.New()
			bios := dos.New(m, ".")
			bios.Install()
			panel, err := host.NewPanelController(host.OutputScale2)
			if err != nil {
				t.Fatal(err)
			}
			keys, err := presentation.NewKeyboardBridgeWithBIOS(panel, m, bios)
			if err != nil {
				t.Fatal(err)
			}
			out := &mouseOutput{}
			mouse, err := host.NewMouseBridge(out)
			if err != nil {
				t.Fatal(err)
			}
			g, err := New(Config{Panel: panel, Keyboard: keys, Mouse: mouse,
				HostFont2: draftFont(16, 16), HostFont3: draftHostFont3(), Labels: draftLabels(),
				Snapshot: func(int) (presentation.LayerPresentationSnapshot, error) {
					return presentation.LayerPresentationSnapshot{}, nil
				}})
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			g.advance = func() error { calls++; return nil }
			update := func(in frameInput) {
				t.Helper()
				g.readInput = func() frameInput { return in }
				if err := g.Update(); err != nil {
					t.Fatal(err)
				}
			}
			update(frameInput{focused: true, down: true, downX: 630, downY: 8})
			update(frameInput{focused: true, up: true, upX: 630, upY: 8})
			update(frameInput{focused: true, down: true, downX: 140, downY: 80})
			update(frameInput{focused: true, up: true, upX: 140, upY: 80})
			update(frameInput{focused: true, down: true, downX: tc.x, downY: 140,
				keys: []ebiten.Key{ebiten.KeyEnter, ebiten.KeyA, ebiten.KeyArrowLeft}})
			state, err := panel.Snapshot()
			if err != nil || state.Open || state.Scales.ActiveScale != tc.wantScale || bios.KeysPending() != 0 || calls != 0 || len(out.calls) != 0 {
				t.Fatalf("closing batch: state=%+v err=%v BIOS=%d Advance=%d mouse=%v", state, err, bios.KeysPending(), calls, out.calls)
			}
			update(frameInput{focused: true, up: true, upX: tc.x, upY: 140})
			if calls != 1 || bios.KeysPending() != 0 {
				t.Fatalf("closed resume: Advance=%d BIOS=%d", calls, bios.KeysPending())
			}
		})
	}
}

func (m *mouseOutput) MoveMouse(int, int) { m.calls = append(m.calls, "move") }
func (m *mouseOutput) PressMouse(int)     { m.calls = append(m.calls, "press") }
func (m *mouseOutput) ReleaseMouse(int)   { m.calls = append(m.calls, "release") }

func draftFont(w, h int) *xlate.Font {
	glyphs := map[rune][]byte{}
	for _, r := range []rune(strings.Join(draftLabels().all(), "")) {
		glyphs[r] = make([]byte, h*((w+7)/8))
		glyphs[r][0] = 0x80
	}
	return &xlate.Font{Name: "draft-test", W: w, H: h, Glyphs: glyphs}
}

func draftHostFont3() *HostFont3 {
	return &HostFont3{Wide: draftFont(24, 24), ASCII: draftFont(16, 24)}
}

func newDraftGame(t *testing.T) (*Game, *mouseOutput) {
	t.Helper()
	m := machine.New()
	bios := dos.New(m, ".")
	bios.Install()
	panel, err := host.NewPanelController(host.OutputScale2)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := presentation.NewKeyboardBridgeWithBIOS(panel, m, bios)
	if err != nil {
		t.Fatal(err)
	}
	out := &mouseOutput{}
	mouse, err := host.NewMouseBridge(out)
	if err != nil {
		t.Fatal(err)
	}
	g, err := New(Config{Panel: panel, Keyboard: keys, Mouse: mouse, HostFont2: draftFont(16, 16), HostFont3: draftHostFont3(), Labels: draftLabels(), Snapshot: func(int) (presentation.LayerPresentationSnapshot, error) {
		return presentation.LayerPresentationSnapshot{}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	return g, out
}

func TestNewRejectsKeyboardWithoutBIOSTransportBeforeLayout(t *testing.T) {
	m := machine.New()
	panel, err := host.NewPanelController(host.OutputScale2)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := presentation.NewKeyboardBridge(panel, m)
	if err != nil {
		t.Fatal(err)
	}
	out := &mouseOutput{}
	mouse, err := host.NewMouseBridge(out)
	if err != nil {
		t.Fatal(err)
	}
	makeConfig := func(p *host.PanelController) Config {
		return Config{Panel: p, Keyboard: plain, Mouse: mouse,
			HostFont2: draftFont(16, 16), HostFont3: draftHostFont3(), Labels: draftLabels(),
			Snapshot: func(int) (presentation.LayerPresentationSnapshot, error) {
				return presentation.LayerPresentationSnapshot{}, nil
			}}
	}
	if _, err := New(makeConfig(panel)); err == nil {
		t.Fatal("keyboard without BIOS passed Game.New")
	}
	bios := dos.New(m, ".")
	bios.Install()
	valid, err := presentation.NewKeyboardBridgeWithBIOS(panel, m, bios)
	if err != nil {
		t.Fatal(err)
	}
	cfg := makeConfig(panel)
	cfg.Keyboard = valid
	otherPanel, err := host.NewPanelController(host.OutputScale2)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Panel = otherPanel
	if _, err := New(cfg); err == nil {
		t.Fatal("different panel passed Game.New")
	}
	cfg.Panel = panel
	otherMachine := machine.New()
	bios.M = otherMachine
	if _, err := New(cfg); err == nil {
		t.Fatal("changed BIOS machine passed Game.New")
	}
	if _, ok := mouse.Layout(); ok || len(out.calls) != 0 || m.BIOSKeyCount() != 0 || otherMachine.BIOSKeyCount() != 0 || bios.Mouse.Buttons != 0 || m.Steps != 0 || otherMachine.Steps != 0 {
		t.Fatalf("rejected constructor wrote state: layout=%v mouse=%v keys=%d/%d buttons=%d steps=%d/%d", ok, out.calls, m.BIOSKeyCount(), otherMachine.BIOSKeyCount(), bios.Mouse.Buttons, m.Steps, otherMachine.Steps)
	}
}

func TestUpdateRejectsChangedBIOSMachineBeforeMouseOrKey(t *testing.T) {
	m := machine.New()
	bios := dos.New(m, ".")
	bios.Install()
	panel, err := host.NewPanelController(host.OutputScale2)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := presentation.NewKeyboardBridgeWithBIOS(panel, m, bios)
	if err != nil {
		t.Fatal(err)
	}
	out := &mouseOutput{}
	mouse, err := host.NewMouseBridge(out)
	if err != nil {
		t.Fatal(err)
	}
	g, err := New(Config{Panel: panel, Keyboard: keys, Mouse: mouse,
		HostFont2: draftFont(16, 16), HostFont3: draftHostFont3(), Labels: draftLabels(),
		Snapshot: func(int) (presentation.LayerPresentationSnapshot, error) {
			return presentation.LayerPresentationSnapshot{}, nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	steps := 0
	g.advance = func() error { steps++; return nil }
	g.readInput = func() frameInput {
		return frameInput{focused: true, down: true, downX: 100, downY: 100, keys: []ebiten.Key{ebiten.KeyEnter}}
	}
	otherMachine := machine.New()
	bios.M = otherMachine
	if err := g.Update(); err == nil {
		t.Fatal("changed BIOS machine passed Update")
	}
	if err := g.Update(); err == nil {
		t.Fatal("failed Update did not retain its error")
	}
	if len(out.calls) != 0 || mouse.Pressed() || bios.Mouse.Buttons != 0 || m.BIOSKeyCount() != 0 || otherMachine.BIOSKeyCount() != 0 || steps != 0 {
		t.Fatalf("failed Update changed DOS/input state: mouse=%v pressed=%v buttons=%d queues=%d/%d steps=%d", out.calls, mouse.Pressed(), bios.Mouse.Buttons, m.BIOSKeyCount(), otherMachine.BIOSKeyCount(), steps)
	}
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

func TestPanelHitUsesTheSameOutputSpaceRectanglesAtBothScales(t *testing.T) {
	for _, scale := range []host.OutputScale{host.OutputScale2, host.OutputScale3} {
		s := int(scale)
		closed := host.MouseLayout{Scale: scale, FrameWidth: 320 * s}
		open := closed
		open.PanelOpen = true
		cases := []struct {
			name   string
			layout host.MouseLayout
			x0, y0 int
			x1, y1 int
			want   host.PanelEvent
		}{
			{"Settings", closed, 244 * s, 2 * s, 316 * s, 15 * s, host.PanelEvent{Kind: host.PanelEventOpen}},
			{"2×", open, 8 * s, 35 * s, 62 * s, 58 * s, host.PanelEvent{Kind: host.PanelEventSelectScale, Scale: host.OutputScale2}},
			{"3×", open, 68 * s, 35 * s, 122 * s, 58 * s, host.PanelEvent{Kind: host.PanelEventSelectScale, Scale: host.OutputScale3}},
			{"Apply", open, 145 * s, 63 * s, 213 * s, 87 * s, host.PanelEvent{Kind: host.PanelEventApply}},
			{"Cancel", open, 220 * s, 63 * s, 288 * s, 87 * s, host.PanelEvent{Kind: host.PanelEventCancel}},
		}
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%dx/%s", s, tc.name), func(t *testing.T) {
				for _, point := range [][2]int{{tc.x0, tc.y0}, {tc.x1 - 1, tc.y1 - 1}} {
					got, hit := panelHit(tc.layout, point[0], point[1])
					if !hit || got != tc.want {
						t.Fatalf("point %v = (%+v, %v), want %+v", point, got, hit, tc.want)
					}
				}
				for _, point := range [][2]int{{tc.x1, tc.y0}, {tc.x0, tc.y1}} {
					if got, hit := panelHit(tc.layout, point[0], point[1]); hit {
						t.Fatalf("exclusive edge %v unexpectedly hit %+v", point, got)
					}
				}
			})
		}
		if got, hit := panelHit(open, 300*s, 100*s); hit {
			t.Fatalf("%d× open-panel blank area hit %+v", s, got)
		}
		if got, hit := panelHit(closed, 8*s, 35*s); hit {
			t.Fatalf("%d× closed-panel option area hit %+v", s, got)
		}
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
	bios := dos.New(m, ".")
	bios.Install()
	p, _ := host.NewPanelController(host.OutputScale2)
	k, _ := presentation.NewKeyboardBridgeWithBIOS(p, m, bios)
	mo, _ := host.NewMouseBridge(&mouseOutput{})
	if _, err := New(Config{Panel: p, Keyboard: k, Mouse: mo, HostFont2: &xlate.Font{W: 1, H: 1, Glyphs: map[rune][]byte{}}, HostFont3: draftHostFont3(), Labels: draftLabels(), Snapshot: func(int) (presentation.LayerPresentationSnapshot, error) { return good, nil }}); err == nil {
		t.Fatal("missing host glyphs accepted")
	}
	zero := draftFont(16, 16)
	for _, glyph := range zero.Glyphs {
		for i := range glyph {
			glyph[i] = 0
		}
	}
	if _, err := New(Config{Panel: p, Keyboard: k, Mouse: mo, HostFont2: zero, HostFont3: draftHostFont3(), Labels: draftLabels(), Snapshot: func(int) (presentation.LayerPresentationSnapshot, error) { return good, nil }}); err == nil {
		t.Fatal("zero-ink glyphs accepted")
	}
	if _, err := New(Config{Panel: p, Keyboard: k, Mouse: mo, HostFont2: draftFont(16, 16), HostFont3: draftHostFont3(), Snapshot: func(int) (presentation.LayerPresentationSnapshot, error) { return good, nil }}); err == nil {
		t.Fatal("empty host labels accepted")
	}
}
