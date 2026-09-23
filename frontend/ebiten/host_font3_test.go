package ebiten

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/presentation"
)

func TestNativeHostFont3MeasuresFiveSafeLabels(t *testing.T) {
	face := draftHostFont3()
	labels := draftLabels()
	if err := validateHostFont3(face, labels); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		label string
		at    image.Point
		width int
	}{
		{labels.Settings, image.Pt(744, 8), 48},
		{labels.Scale2, image.Pt(36, 114), 40},
		{labels.Scale3, image.Pt(216, 114), 40},
		{labels.Apply, image.Pt(450, 198), 48},
		{labels.Cancel, image.Pt(675, 198), 48},
	} {
		advance, ink, err := measureHostText3(face, tc.at, tc.label)
		if err != nil || advance.Dx() != tc.width || advance.Dy() != 24 || ink.Empty() {
			t.Fatalf("%q advance=%v ink=%v err=%v", tc.label, advance, ink, err)
		}
	}
}

func TestNativeHostFont3DrawUsesMeasuredAdvanceAndLeavesTwoXAlone(t *testing.T) {
	g, _ := newDraftGame(t)
	white := color.RGBA{255, 255, 255, 255}
	before := append([]byte(nil), g.font2.Glyphs['2']...)
	three := ebiten.NewImage(64, 24)
	if err := g.drawText3(three, 0, 0, "2×", white); err != nil {
		t.Fatal(err)
	}
	two := ebiten.NewImage(64, 16)
	g.drawText(two, g.font2, 0, 0, "2×", white)
	advance, _, err := measureHostText3(g.font3, image.Point{}, "2×")
	if err != nil || advance.Dx() != 40 || g.font2.W != 16 || g.font2.H != 16 || string(g.font2.Glyphs['2']) != string(before) {
		t.Fatalf("3× advance=%v err=%v; 2× dimensions=%dx%d", advance, err, g.font2.W, g.font2.H)
	}
}

func TestNativeHostFont3DrawFailureLatchesBeforeNextDOSInput(t *testing.T) {
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
	advanceCalls := 0
	g, err := New(Config{Panel: panel, Keyboard: keys, Mouse: mouse,
		HostFont2: draftFont(16, 16), HostFont3: draftHostFont3(), Labels: draftLabels(),
		Snapshot: func(scale int) (presentation.LayerPresentationSnapshot, error) {
			return presentation.LayerPresentationSnapshot{
				Frame: host.PresentationSnapshot{Canvas: host.Canvas{Width: 320, Height: 200}},
				Scale: scale, RGBA: make([]byte, 320*scale*200*scale*4),
			}, nil
		},
		Advance: func() error { advanceCalls++; m.Steps++; return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []host.PanelEvent{
		{Kind: host.PanelEventOpen},
		{Kind: host.PanelEventSelectScale, Scale: host.OutputScale3},
		{Kind: host.PanelEventApply},
	} {
		if _, _, err := panel.Route(event); err != nil {
			t.Fatal(err)
		}
	}
	if err := g.refreshLayout(); err != nil {
		t.Fatal(err)
	}
	if g.layout.Scale != host.OutputScale3 || g.layout.PanelOpen {
		t.Fatalf("not in closed 3× layout: %+v", g.layout)
	}
	// New validated the original font; make one required glyph invalid only
	// afterward, so this error must originate in Draw's 3× chrome painter.
	delete(g.font3.Wide.Glyphs, '設')
	g.Draw(ebiten.NewImage(g.layout.FrameWidth, g.layout.FrameHeight))
	drawErr := g.err
	if drawErr == nil || !strings.Contains(drawErr.Error(), "缺少或損壞") {
		t.Fatalf("Draw did not latch the missing-glyph error: %v", drawErr)
	}
	stepsBefore, keysBefore, mouseBefore := m.Steps, bios.KeysPending(), len(out.calls)
	g.readInput = func() frameInput {
		return frameInput{focused: true, down: true, downX: 200, downY: g.layout.ChromeHeight + 120,
			keys: []ebiten.Key{ebiten.KeyEnter}}
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := g.Update(); err != drawErr {
			t.Fatalf("Update %d returned %v, want latched Draw error %v", attempt, err, drawErr)
		}
		if advanceCalls != 0 || m.Steps != stepsBefore || bios.KeysPending() != keysBefore || len(out.calls) != mouseBefore {
			t.Fatalf("Update %d added DOS effects: Advance=%d steps=%d/%d BIOS=%d/%d mouse=%d/%d",
				attempt, advanceCalls, m.Steps, stepsBefore, bios.KeysPending(), keysBefore, len(out.calls), mouseBefore)
		}
	}
}

func TestNativeHostFont3GameNewRejectsInvalidInputs(t *testing.T) {
	g, _ := newDraftGame(t)
	valid := func(face *HostFont3, labels HostLabels) error {
		font2 := draftFont(16, 16)
		font2.Glyphs['A'] = append([]byte(nil), font2.Glyphs['2']...)
		_, err := New(Config{Panel: g.panel, Keyboard: g.keys, Mouse: g.mouse,
			HostFont2: font2, HostFont3: face, Labels: labels,
			Snapshot: func(int) (presentation.LayerPresentationSnapshot, error) {
				return presentation.LayerPresentationSnapshot{}, nil
			}})
		return err
	}
	for _, tc := range []struct {
		name   string
		mutate func(*HostFont3, *HostLabels)
		want   string
	}{
		{"missing-wide", func(f *HostFont3, _ *HostLabels) { f.Wide = nil }, "24x24"},
		{"old-22", func(f *HostFont3, _ *HostLabels) { f.Wide = draftFont(22, 22) }, "24x24"},
		{"wrong-ascii", func(f *HostFont3, _ *HostLabels) { f.ASCII = draftFont(24, 24) }, "16x24"},
		{"missing-digit", func(f *HostFont3, _ *HostLabels) { delete(f.ASCII.Glyphs, '2') }, "缺少"},
		{"missing-wide-glyph", func(f *HostFont3, _ *HostLabels) { delete(f.Wide.Glyphs, '×') }, "缺少"},
		{"bad-row-length", func(f *HostFont3, _ *HostLabels) { f.Wide.Glyphs['設'] = []byte{0x80} }, "損壞"},
		{"blank-glyph", func(f *HostFont3, _ *HostLabels) { f.ASCII.Glyphs['3'] = make([]byte, 48) }, "墨跡"},
		{"unsupported-ascii", func(_ *HostFont3, l *HostLabels) { l.Settings = "A" }, "不支援"},
		{"overflow", func(_ *HostFont3, l *HostLabels) { l.Settings = strings.Repeat("設", 10) }, "安全矩形"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			face, labels := draftHostFont3(), draftLabels()
			tc.mutate(face, &labels)
			if err := valid(face, labels); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v want %q", err, tc.want)
			}
		})
	}
}
