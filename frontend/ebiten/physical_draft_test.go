package ebiten

// This opt-in DRAFT receipt uses genuine X11 events. It is never part of the
// normal unit suite and creates no production command.
import (
	"crypto/sha256"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/presentation"
	"github.com/wicanr2/dosgolem/xlate"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestPhysicalDraftOpenApplyCanvas(t *testing.T) {
	if os.Getenv("EBITEN_PHYSICAL_DRAFT") != "1" {
		t.Skip("opt-in physical X11 receipt")
	}
	out := os.Getenv("EBITEN_RECEIPT_OUT")
	if out == "" {
		t.Fatal("EBITEN_RECEIPT_OUT required")
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	trace := filepath.Join(out, "trace.txt")
	_ = os.WriteFile(trace, []byte("started\n"), 0644)
	appendTrace := func(s string) {
		f, _ := os.OpenFile(trace, os.O_APPEND|os.O_WRONLY, 0644)
		if f != nil {
			_, _ = f.WriteString(s + "\n")
			_ = f.Close()
		}
	}
	font2, err := xlate.LoadFont("/project/workplace/current-font/buckrogers-eten-top-pad.golemfnt")
	if err != nil {
		t.Fatal(err)
	}
	font3 := derive22(font2)
	labels, catalogSHA, err := loadDraftHostLabels("/project/text/host-ui.zh-TW.tsv")
	if err != nil {
		t.Fatal(err)
	}
	m := machine.New()
	p, _ := host.NewPanelController(host.OutputScale2)
	k, _ := presentation.NewKeyboardBridge(p, m)
	mouseOut := &mouseOutput{}
	mb, _ := host.NewMouseBridge(mouseOut)
	var updates, draws atomic.Int64
	var firstG, secondG, advanceG string
	snap := func(scale int) (presentation.LayerPresentationSnapshot, error) {
		appendTrace("draw scale=" + itoa(scale) + " goid=" + gid())
		draws.Add(1)
		g := gid()
		if firstG == "" {
			firstG = g
		} else {
			secondG = g
		}
		rgba := make([]byte, 320*scale*200*scale*4)
		for i := 0; i < len(rgba); i += 4 {
			rgba[i] = 20
			rgba[i+1] = 40
			rgba[i+2] = 80
			rgba[i+3] = 255
		}
		return presentation.LayerPresentationSnapshot{Scale: scale, Frame: host.PresentationSnapshot{Canvas: host.Canvas{Width: 320, Height: 200}}, RGBA: rgba}, nil
	}
	g, err := New(Config{Panel: p, Keyboard: k, Mouse: mb, HostFont2: font2, HostFont3: font3, Labels: labels, Snapshot: snap, Advance: func() error {
		advanceG = gid()
		appendTrace("advance goid=" + advanceG + " calls=" + strings.Join(mouseOut.calls, ","))
		updates.Add(1)
		if len(mouseOut.calls) >= 4 && draws.Load() > 4 {
			return ebiten.Termination
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	ebiten.SetWindowTitle("dosgolem frontend draft receipt")
	ebiten.SetWindowSize(640, 436)
	go func() {
		time.Sleep(time.Second)
		wid := strings.TrimSpace(run("xdotool", "search", "--name", "dosgolem frontend draft receipt"))
		if wid == "" {
			appendTrace("no-window")
			return
		}
		run("xdotool", "windowfocus", wid)
		run("import", "-window", wid, filepath.Join(out, "ui-2x.png"))
		click := func(x, y int) {
			appendTrace("click " + itoa(x) + "," + itoa(y))
			run("xdotool", "mousemove", "--window", wid, itoa(x), itoa(y))
			run("xdotool", "click", "--window", wid, "1")
			time.Sleep(250 * time.Millisecond)
		}
		click(630, 8)
		click(140, 80)
		click(300, 140)
		time.Sleep(500 * time.Millisecond)
		run("import", "-window", wid, filepath.Join(out, "ui-3x.png"))
		click(300, 300)
	}()
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	st, _ := p.Snapshot()
	if st.Open || st.Scales.ActiveScale != host.OutputScale3 {
		t.Fatalf("final panel=%+v", st)
	}
	if got := strings.Join(mouseOut.calls, ","); got != "move,press,move,release" {
		t.Fatalf("DOS calls %s", got)
	}
	if updates.Load() == 0 || draws.Load() == 0 || firstG == "" || secondG == "" || firstG != secondG || advanceG != firstG {
		t.Fatalf("callback ordering/goroutine updates=%d draws=%d first=%q second=%q", updates.Load(), draws.Load(), firstG, secondG)
	}
	_ = os.WriteFile(filepath.Join(out, "receipt.txt"), []byte("physical=true\nscale=3\ncatalog_sha256="+catalogSHA+"\ndos_calls=move,press,move,release\nupdates="+itoa(int(updates.Load()))+"\ndraws="+itoa(int(draws.Load()))+"\ngoid="+firstG+"\n"), 0644)
}

func loadDraftHostLabels(path string) (HostLabels, string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return HostLabels{}, "", err
	}
	sha := fmt.Sprintf("%x", sha256.Sum256(b))
	lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	if len(lines) != 6 || lines[0] != "key\ttranslation\tsource" {
		return HostLabels{}, "", fmt.Errorf("host catalog header/row count invalid")
	}
	want := map[string]bool{"host.settings": true, "host.apply": true, "host.cancel": true, "host.scale2": true, "host.scale3": true}
	got := map[string]string{}
	for _, line := range lines[1:] {
		row := strings.Split(line, "\t")
		if len(row) != 3 || !want[row[0]] || got[row[0]] != "" || row[1] == "" || row[2] != "runtime-interface" {
			return HostLabels{}, "", fmt.Errorf("host catalog row invalid: %q", line)
		}
		got[row[0]] = row[1]
	}
	return HostLabels{Settings: got["host.settings"], Apply: got["host.apply"], Cancel: got["host.cancel"], Scale2: got["host.scale2"], Scale3: got["host.scale3"]}, sha, nil
}
func derive22(src *xlate.Font) *xlate.Font {
	out := &xlate.Font{Name: src.Name + ".22", W: 22, H: 22, Glyphs: map[rune][]byte{}}
	for r, g := range src.Glyphs {
		d := make([]byte, 22*3)
		if r <= 0xff {
			for y := 0; y < 16; y++ {
				for x := 0; x < 16; x++ {
					if g[y*2+x/8]&(0x80>>uint(x%8)) != 0 {
						dx, dy := x+3, y+3
						d[dy*3+dx/8] |= 0x80 >> uint(dx%8)
					}
				}
			}
		} else {
			for y := 0; y < 22; y++ {
				for x := 0; x < 22; x++ {
					sx, sy := x*16/22, y*16/22
					if g[sy*2+sx/8]&(0x80>>uint(sx%8)) != 0 {
						d[y*3+x/8] |= 0x80 >> uint(x%8)
					}
				}
			}
		}
		out.Glyphs[r] = d
	}
	return out
}
func run(name string, args ...string) string {
	b, _ := exec.Command(name, args...).Output()
	return string(b)
}
func itoa(n int) string { return strconv.Itoa(n) }
func gid() string {
	b := make([]byte, 64)
	n := runtime.Stack(b, false)
	return strings.Fields(string(b[:n]))[1]
}
