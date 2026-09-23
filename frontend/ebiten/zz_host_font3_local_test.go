package ebiten

import (
	"bytes"
	"image"
	"os"
	"os/exec"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/presentation"
	"github.com/wicanr2/dosgolem/xlate"
)

type hostPixelShot struct {
	w, h int
	rgba []byte
}

type hostPixelGame struct {
	inner          *Game
	inputs         []frameInput
	step           int
	applied, drawn bool
	shots          map[int]hostPixelShot
	err            error
}

func (g *hostPixelGame) Update() error {
	if g.err != nil {
		return g.err
	}
	if g.drawn {
		g.step++
		g.applied, g.drawn = false, false
	}
	if g.step == len(g.inputs) {
		return ebiten.Termination
	}
	if !g.applied {
		in := g.inputs[g.step]
		g.inner.readInput = func() frameInput { return in }
		if err := g.inner.Update(); err != nil {
			return err
		}
		g.applied = true
	}
	return nil
}

func (g *hostPixelGame) Draw(screen *ebiten.Image) {
	g.inner.Draw(screen)
	if g.inner.err != nil {
		g.err = g.inner.err
		return
	}
	if g.step == 0 || g.step == 7 || g.step == 9 || g.step == 14 {
		w, h := screen.Bounds().Dx(), screen.Bounds().Dy()
		pix := make([]byte, w*h*4)
		screen.ReadPixels(pix)
		g.shots[g.step] = hostPixelShot{w, h, pix}
	}
	g.drawn = true
}

func (g *hostPixelGame) Layout(_, _ int) (int, int) { return g.inner.Layout(0, 0) }

func checkHostLabelPixels(t *testing.T, shot hostPixelShot, face *HostFont3, safe image.Rectangle, at image.Point, label string) int {
	t.Helper()
	if safe.Max.X > shot.w || safe.Max.Y > shot.h {
		t.Fatalf("safe rect %v outside %dx%d shot", safe, shot.w, shot.h)
	}
	count := 0
	for y := safe.Min.Y; y < safe.Max.Y; y++ {
		for x := safe.Min.X; x < safe.Max.X; x++ {
			want := false
			cursor := at.X
			for _, r := range label {
				font, glyph, err := face.glyph(r)
				if err != nil {
					t.Fatal(err)
				}
				if x >= cursor && x < cursor+font.W && y >= at.Y && y < at.Y+font.H {
					gx, gy := x-cursor, y-at.Y
					want = glyph[gy*((font.W+7)/8)+gx/8]&(0x80>>uint(gx%8)) != 0
				}
				cursor += font.W
			}
			o := (y*shot.w + x) * 4
			got := shot.rgba[o] == 255 && shot.rgba[o+1] == 255 && shot.rgba[o+2] == 255 && shot.rgba[o+3] == 255
			if got != want {
				t.Fatalf("%q pixel mismatch at (%d,%d): got white=%v want=%v", label, x, y, got, want)
			}
			if got {
				count++
			}
		}
	}
	if count == 0 {
		t.Fatalf("%q has no visible pixels", label)
	}
	return count
}

func nativeTailInk(face *HostFont3, label string) int {
	count := 0
	for _, r := range label {
		font, glyph, err := face.glyph(r)
		if err != nil || font.W != 24 {
			continue
		}
		for y := 0; y < font.H; y++ {
			for x := 0; x < font.W; x++ {
				if (x >= 22 || y >= 22) && glyph[y*((font.W+7)/8)+x/8]&(0x80>>uint(x%8)) != 0 {
					count++
				}
			}
		}
	}
	return count
}

func checkWhiteChromeWithin(t *testing.T, shot hostPixelShot, chromeHeight int, safe ...image.Rectangle) {
	t.Helper()
	for y := 0; y < chromeHeight; y++ {
		for x := 0; x < shot.w; x++ {
			o := (y*shot.w + x) * 4
			if shot.rgba[o] != 255 || shot.rgba[o+1] != 255 || shot.rgba[o+2] != 255 || shot.rgba[o+3] != 255 {
				continue
			}
			inside := false
			for _, r := range safe {
				if image.Pt(x, y).In(r) {
					inside = true
					break
				}
			}
			if !inside {
				t.Fatalf("white chrome pixel outside label safe rectangles at (%d,%d)", x, y)
			}
		}
	}
}

// This is a host-only pixel receipt, not a normal DOS player-path test.  The
// native subset paths are local ignored inputs, and pixels are read only from
// Ebitengine's Draw callback while RunGame owns the graphics context.
func TestNativeHostFont3LocalPixels(t *testing.T) {
	widePath, asciiPath := os.Getenv("HOST_FONT3_WIDE"), os.Getenv("HOST_FONT3_ASCII")
	if widePath == "" || asciiPath == "" {
		t.Skip("local native host font subsets not supplied")
	}
	if os.Getenv("HOST_FONT3_PIXEL_CHILD") != "1" {
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(exe, "-test.run=^TestNativeHostFont3LocalPixels$", "-test.v")
		cmd.Env = append(os.Environ(), "HOST_FONT3_PIXEL_CHILD=1")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("isolated Ebitengine pixel receipt failed: %v\n%s", err, output)
		}
		t.Logf("isolated Ebitengine pixel receipt:\n%s", output)
		return
	}
	wide, err := xlate.LoadFont(widePath)
	if err != nil {
		t.Fatal(err)
	}
	ascii, err := xlate.LoadFont(asciiPath)
	if err != nil {
		t.Fatal(err)
	}
	face := &HostFont3{Wide: wide, ASCII: ascii}
	if err := validateHostFont3(face, draftLabels()); err != nil {
		t.Fatal(err)
	}
	panel, err := host.NewPanelController(host.OutputScale2)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := presentation.NewKeyboardBridge(panel, machine.New())
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
		HostFont2: draftFont(16, 16), HostFont3: face, Labels: draftLabels(),
		Snapshot: func(scale int) (presentation.LayerPresentationSnapshot, error) {
			return presentation.LayerPresentationSnapshot{
				Frame: host.PresentationSnapshot{Canvas: host.Canvas{Width: 320, Height: 200}},
				Scale: scale, RGBA: make([]byte, 320*scale*200*scale*4),
			}, nil
		},
		Advance: func() error { advanceCalls++; return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	inputs := []frameInput{
		{focused: true},
		{focused: true, down: true, downX: 630, downY: 8},
		{focused: true, up: true, upX: 630, upY: 8},
		{focused: true, down: true, downX: 140, downY: 80},
		{focused: true, up: true, upX: 140, upY: 80},
		{focused: true, down: true, downX: 300, downY: 140},
		{focused: true, up: true, upX: 300, upY: 140},
		{focused: true},
		{focused: true, down: true, downX: 930, downY: 10},
		{focused: true, up: true, upX: 930, upY: 10},
		{focused: true, down: true, downX: 50, downY: 120},
		{focused: true, up: true, upX: 50, upY: 120},
		{focused: true, down: true, downX: 500, downY: 220},
		{focused: true, up: true, upX: 500, upY: 220},
		{focused: true},
	}
	wrapped := &hostPixelGame{inner: g, inputs: inputs, shots: make(map[int]hostPixelShot)}
	ebiten.SetWindowSize(g.layout.FrameWidth, g.layout.FrameHeight)
	ebiten.SetWindowTitle("host-only local native 3x font pixel receipt")
	if err := ebiten.RunGame(wrapped); err != nil {
		t.Fatal(err)
	}
	if wrapped.err != nil {
		t.Fatal(wrapped.err)
	}
	for _, step := range []int{0, 7, 9, 14} {
		if _, ok := wrapped.shots[step]; !ok {
			t.Fatalf("missing screen pixel shot at step %d", step)
		}
	}
	before, after := wrapped.shots[0], wrapped.shots[14]
	if before.w != after.w || before.h != after.h || !bytes.Equal(before.rgba, after.rgba) {
		t.Fatal("2x chrome/canvas pixels changed after 2x->3x->2x roundtrip")
	}
	closed, open := wrapped.shots[7], wrapped.shots[9]
	if closed.w != 960 || closed.h != 654 || open.w != 960 || open.h != 876 {
		t.Fatalf("3x screen sizes: closed=%dx%d open=%dx%d", closed.w, closed.h, open.w, open.h)
	}
	labels := draftLabels()
	settingsSafe := image.Rect(732, 6, 948, 45)
	scale2Safe := image.Rect(24, 105, 186, 174)
	scale3Safe := image.Rect(204, 105, 366, 174)
	applySafe := image.Rect(435, 189, 639, 261)
	cancelSafe := image.Rect(660, 189, 864, 261)
	counts := []int{
		checkHostLabelPixels(t, closed, face, settingsSafe, image.Pt(744, 8), labels.Settings),
		checkHostLabelPixels(t, open, face, scale2Safe, image.Pt(36, 114), labels.Scale2),
		checkHostLabelPixels(t, open, face, scale3Safe, image.Pt(216, 114), labels.Scale3),
		checkHostLabelPixels(t, open, face, applySafe, image.Pt(450, 198), labels.Apply),
		checkHostLabelPixels(t, open, face, cancelSafe, image.Pt(675, 198), labels.Cancel),
	}
	checkWhiteChromeWithin(t, closed, 54, settingsSafe)
	checkWhiteChromeWithin(t, open, 276, scale2Safe, scale3Safe, applySafe, cancelSafe)
	tail := 0
	for _, label := range []string{labels.Settings, labels.Scale2, labels.Scale3, labels.Apply, labels.Cancel} {
		tail += nativeTailInk(face, label)
	}
	if tail == 0 {
		t.Fatal("native 24-point glyphs have no ink beyond 22-point crop boundary")
	}
	if len(out.calls) != 0 || advanceCalls != 5 {
		t.Fatalf("host-only route effects: mouse calls=%d Advance=%d", len(out.calls), advanceCalls)
	}
	if g.layout.Scale != host.OutputScale2 || g.layout.PanelOpen {
		t.Fatalf("roundtrip final layout=%+v", g.layout)
	}
	t.Logf("screen RGBA pixel receipt: 3x label ink=%v, native >22 tail ink=%d, 2x roundtrip bytes=%d identical, DOS mouse calls=%d, closed Advance=%d", counts, tail, len(before.rgba), len(out.calls), advanceCalls)
}
