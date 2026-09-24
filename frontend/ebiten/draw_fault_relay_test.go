package ebiten

import (
	"errors"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/presentation"
)

func TestGameDrawReportsFirstFaultBeforeAnotherUpdate(t *testing.T) {
	base, _ := newDraftGame(t)
	mouse, err := host.NewMouseBridge(&mouseOutput{})
	if err != nil {
		t.Fatal(err)
	}
	fault := errors.New("injected snapshot fault")
	reports, advances := 0, 0
	var reported error
	g, err := New(Config{
		Panel: base.panel, Keyboard: base.keys, Mouse: mouse,
		HostFont2: draftFont(16, 16), HostFont3: draftHostFont3(), Labels: draftLabels(),
		Snapshot: func(int) (presentation.LayerPresentationSnapshot, error) {
			return presentation.LayerPresentationSnapshot{}, fault
		},
		Advance: func() error { advances++; return nil },
		OnDrawFault: func(err error) {
			reports++
			reported = err
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	screen := ebiten.NewImage(g.layout.FrameWidth, g.layout.FrameHeight)
	g.Draw(screen)
	if reports != 1 || !errors.Is(reported, fault) || !errors.Is(g.err, fault) {
		t.Fatalf("Draw did not synchronously report the first fault: reports=%d reported=%v game=%v", reports, reported, g.err)
	}
	g.Draw(screen)
	g.readInput = func() frameInput { return frameInput{focused: true, keys: []ebiten.Key{ebiten.KeyEnter}} }
	if err := g.Update(); !errors.Is(err, fault) || reports != 1 || advances != 0 {
		t.Fatalf("faulted Game resumed or reported twice: err=%v reports=%d advances=%d", err, reports, advances)
	}
}

func TestGameDrawReportsValidationAndChromeFaultsOnce(t *testing.T) {
	for _, tc := range []struct {
		name   string
		inject func(*testing.T, *Game)
	}{
		{"invalid-snapshot", func(_ *testing.T, g *Game) {
			g.snapshot = func(scale int) (presentation.LayerPresentationSnapshot, error) {
				return presentation.LayerPresentationSnapshot{
					Frame: host.PresentationSnapshot{Canvas: host.Canvas{Width: 320, Height: 200}}, Scale: scale,
				}, nil
			}
		}},
		{"3x-chrome-glyph", func(t *testing.T, g *Game) {
			for _, event := range []host.PanelEvent{
				{Kind: host.PanelEventOpen},
				{Kind: host.PanelEventSelectScale, Scale: host.OutputScale3},
				{Kind: host.PanelEventApply},
			} {
				if _, _, err := g.panel.Route(event); err != nil {
					t.Fatal(err)
				}
			}
			if err := g.refreshLayout(); err != nil {
				t.Fatal(err)
			}
			g.snapshot = func(scale int) (presentation.LayerPresentationSnapshot, error) {
				return presentation.LayerPresentationSnapshot{
					Frame: host.PresentationSnapshot{Canvas: host.Canvas{Width: 320, Height: 200}},
					Scale: scale, RGBA: make([]byte, 320*scale*200*scale*4),
				}, nil
			}
			delete(g.font3.Wide.Glyphs, '設')
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, _ := newDraftGame(t)
			reports := 0
			var first error
			g.onDrawFault = func(err error) { reports++; first = err }
			tc.inject(t, g)
			screen := ebiten.NewImage(g.layout.FrameWidth, g.layout.FrameHeight)
			g.Draw(screen)
			if reports != 1 || first == nil || !errors.Is(g.err, first) {
				t.Fatalf("Draw fault not reported: reports=%d first=%v game=%v", reports, first, g.err)
			}
			g.Draw(screen)
			if reports != 1 || !errors.Is(g.err, first) {
				t.Fatalf("first Draw fault not retained: reports=%d game=%v", reports, g.err)
			}
		})
	}
}
