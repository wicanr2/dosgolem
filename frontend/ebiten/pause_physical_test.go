package ebiten

// This opt-in test drives the formal Game.Update through genuine X11 mouse
// events. It uses a synthetic canvas and does not load or distribute a game.
import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/presentation"
)

func TestPhysicalPanelPauseGate(t *testing.T) {
	if os.Getenv("EBITEN_PAUSE_PHYSICAL") != "1" {
		t.Skip("opt-in genuine X11 panel-pause receipt")
	}
	g, out := newDraftGame(t)
	g.snapshot = func(scale int) (presentation.LayerPresentationSnapshot, error) {
		return presentation.LayerPresentationSnapshot{
			Scale: scale,
			Frame: host.PresentationSnapshot{Canvas: host.Canvas{Width: 320, Height: 200}},
			RGBA:  make([]byte, 320*scale*200*scale*4),
		}, nil
	}
	start := time.Now()
	turn, calls := 0, 0
	var closeTurns, resumeTurns []int
	var closeKinds []host.PanelEventKind
	var closeScales []host.OutputScale
	transitions := map[host.PanelEventKind]int{}
	pauseThisTurn := false
	g.readInput = func() frameInput {
		turn++
		if time.Since(start) > 10*time.Second {
			g.err = ebiten.Termination
			return frameInput{focused: true}
		}
		before, _ := g.panel.Snapshot()
		in := readFrameInput()
		pauseThisTurn = before.Open
		if in.down {
			if event, hit := g.hostHit(in.downX, in.downY); hit {
				transitions[event.Kind]++
				pauseThisTurn = true
				if event.Kind == host.PanelEventCancel || event.Kind == host.PanelEventApply {
					closeTurns = append(closeTurns, turn)
					closeKinds = append(closeKinds, event.Kind)
					closeScales = append(closeScales, before.Scales.ActiveScale)
				}
			}
		}
		return in
	}
	g.advance = func() error {
		calls++
		state, err := g.panel.Snapshot()
		if err != nil || pauseThisTurn || state.Open {
			return fmt.Errorf("Advance during panel turn %d: state=%+v pause=%t err=%v", turn, state, pauseThisTurn, err)
		}
		if len(resumeTurns) < len(closeTurns) {
			resumeTurns = append(resumeTurns, turn)
		}
		if len(closeTurns) == 4 && len(resumeTurns) == 4 && time.Since(start) > 6*time.Second {
			return ebiten.Termination
		}
		return nil
	}
	ebiten.SetWindowTitle("dosgolem panel pause gate receipt")
	driver := make(chan error, 1)
	go func() {
		time.Sleep(time.Second)
		cmd := exec.Command("xdotool", "search", "--name", "dosgolem panel pause gate receipt")
		found, err := cmd.Output()
		if err != nil || strings.TrimSpace(string(found)) == "" {
			driver <- fmt.Errorf("find X11 window: %v", err)
			return
		}
		id := strings.Fields(string(found))[0]
		run := func(args ...string) error {
			output, err := exec.Command("xdotool", args...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("xdotool %v: %w: %s", args, err, output)
			}
			return nil
		}
		click := func(x, y string) error {
			for _, args := range [][]string{
				{"mousemove", "--window", id, x, y},
				{"mousedown", "--window", id, "1"},
			} {
				if err := run(args...); err != nil {
					return err
				}
			}
			time.Sleep(100 * time.Millisecond)
			if err := run("mouseup", "--window", id, "1"); err != nil {
				return err
			}
			time.Sleep(250 * time.Millisecond)
			return nil
		}
		if err := run("windowfocus", id); err != nil {
			driver <- err
			return
		}
		for _, p := range [][2]string{
			{"630", "8"}, {"140", "80"}, {"450", "140"},
			{"630", "8"}, {"140", "80"}, {"300", "140"},
			{"930", "8"}, {"700", "210"},
			{"930", "8"}, {"80", "120"}, {"500", "210"},
		} {
			if err := click(p[0], p[1]); err != nil {
				driver <- err
				return
			}
		}
		driver <- nil
	}()
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if err := <-driver; err != nil {
		t.Fatal(err)
	}
	state, err := g.panel.Snapshot()
	if err != nil || state.Open || state.Scales.ActiveScale != host.OutputScale2 {
		t.Fatalf("final panel state=%+v err=%v", state, err)
	}
	for _, kind := range []host.PanelEventKind{host.PanelEventOpen, host.PanelEventSelectScale, host.PanelEventCancel, host.PanelEventApply} {
		want := 2
		if kind == host.PanelEventOpen || kind == host.PanelEventSelectScale {
			want = 4
			if kind == host.PanelEventSelectScale {
				want = 3
			}
		}
		if transitions[kind] != want {
			t.Fatalf("physical transition %d count=%d want=%d all=%v", kind, transitions[kind], want, transitions)
		}
	}
	if len(closeTurns) != 4 || len(resumeTurns) != 4 || calls == 0 || len(out.calls) != 0 {
		t.Fatalf("pause receipt closes=%v resumes=%v Advance=%d DOS mouse=%v", closeTurns, resumeTurns, calls, out.calls)
	}
	for i := range closeTurns {
		if resumeTurns[i] <= closeTurns[i] {
			t.Fatalf("close %d resumed in same turn: close=%d resume=%d", i, closeTurns[i], resumeTurns[i])
		}
	}
	if closeKinds[0] != host.PanelEventCancel || closeKinds[1] != host.PanelEventApply || closeKinds[2] != host.PanelEventCancel || closeKinds[3] != host.PanelEventApply ||
		closeScales[0] != host.OutputScale2 || closeScales[1] != host.OutputScale2 || closeScales[2] != host.OutputScale3 || closeScales[3] != host.OutputScale3 {
		t.Fatalf("unexpected close order kinds=%v scales=%v", closeKinds, closeScales)
	}
	t.Logf("physical pause: transitions=%v closes=%v resumes=%v kinds=%v entry_scales=%v Advance_calls=%d final_scale=%d", transitions, closeTurns, resumeTurns, closeKinds, closeScales, calls, state.Scales.ActiveScale)
}
