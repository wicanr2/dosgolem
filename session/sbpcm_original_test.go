package session

// 462 G5/G6 的選配原版驗收。只在 SBPCM462_ROOT、SBPCM462_STATE、
// SBPCM462_OUT 都設定時執行。CASE=T10 或 K1；STOP 可縮短停止步。
// 原始輸入唯讀，SaveRoot 只在測試暫存目錄中。OPL 合成靜音，客體硬體不變。

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/audio/mixer"
	"github.com/wicanr2/dosgolem/audio/sbpcm"
	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/machine"
)

type originalPCMObserver struct {
	events                                          []machine.SBDMAEvent
	mix                                             *mixer.Mixer
	source                                          *sbpcm.Source
	first                                           uint64
	started                                         bool
	oplWrites, speakerNonzero, pit2Nonzero, mutated uint64
}

func (*originalPCMObserver) BeforeStep(StepView) error     { return nil }
func (*originalPCMObserver) VideoWrite(machine.VideoWrite) {}
func (*originalPCMObserver) Frame([]byte, [256][3]uint8)   {}
func (r *originalPCMObserver) SBDMA(e machine.SBDMAEvent)  { r.events = append(r.events, e) }
func (r *originalPCMObserver) SBPCM(e machine.SBPCMEvent) {
	if e.Kind == "start" && !r.started {
		r.first = e.Start * 48000 / 11550000
		r.started = true
	}
	if r.mix != nil {
		r.mix.SBPCM(e)
		r.source.Apply(e)
	}
}

// G6「AdLib 關閉」具體化為不送 OPL 合成器，保留客體 OPL 存在及寫入。
func (r *originalPCMObserver) OPLWrite(machine.OPLWrite) { r.oplWrites++ }
func (r *originalPCMObserver) SpeakerSample(e machine.SpeakerSample) {
	if e.Gate != 0 || e.Level != 0 {
		r.speakerNonzero++
	}
	if r.mix != nil {
		r.mix.SpeakerSample(e)
	}
}
func (r *originalPCMObserver) PITChannel2(e machine.PIT2Change) {
	if e.Divisor != 0 {
		r.pit2Nonzero++
	}
	if r.mix != nil {
		r.mix.PITChannel2(e)
	}
}

type originalPCMInput struct {
	step uint64
	kind string
	x, y int
}
type originalPCMResult struct {
	Digest                                                                                              StateDigest
	MachineDigest                                                                                       FullDigest
	MachineStateSHA256, DOSStateSHA256, SBDMASHA256                                                     string
	FirstSample, EndStep, Samples, ClippedSamples, OPLWrites, SpeakerNonzero, PIT2Nonzero, SBPCMMutated uint64
	OPLSynthesisMuted                                                                                   bool
	state, dosState                                                                                     []byte
	events                                                                                              []machine.SBDMAEvent
}

func TestSBPCMOriginalG5G6(t *testing.T) {
	root, statePath, out := os.Getenv("SBPCM462_ROOT"), os.Getenv("SBPCM462_STATE"), os.Getenv("SBPCM462_OUT")
	if root == "" || statePath == "" || out == "" {
		t.Skip("需設定 SBPCM462_ROOT／SBPCM462_STATE／SBPCM462_OUT，缺原版素材不造替代")
	}
	for _, path := range []string{root, statePath} {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Skipf("缺原版輸入 %s", path)
		} else if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(out); err == nil {
		t.Fatalf("輸出目錄已存在，不覆寫收據：%s", out)
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	state, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	caseName := os.Getenv("SBPCM462_CASE")
	if caseName == "" {
		caseName = "T10"
	}
	var start, stop uint64
	var inputs []originalPCMInput
	click := func(at uint64, x, y int) {
		inputs = append(inputs, originalPCMInput{at - 200000, "move", x, y}, originalPCMInput{at, "down", x, y}, originalPCMInput{at + 2000000, "up", x, y})
	}
	switch caseName {
	case "T10":
		start, stop = 5000600000, 5900000000
		click(5005000000, 56, 140)
		click(5040000000, 248, 104)
	case "K1":
		start, stop = 5100000000, 5170000000
		click(5102000000, 160, 100)
	default:
		t.Fatalf("未知 CASE %q", caseName)
	}
	if value := os.Getenv("SBPCM462_STOP"); value != "" {
		stop, err = strconv.ParseUint(value, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
	}
	if stop <= start {
		t.Fatal("STOP 必須大於初始步數")
	}
	saveRoot := filepath.Join(t.TempDir(), "save")
	run := func(mode int) originalPCMResult {
		if err := os.RemoveAll(saveRoot); err != nil {
			t.Fatal(err)
		}
		if err := copyLegacyTree(root, saveRoot); err != nil {
			t.Fatal(err)
		}
		r := &originalPCMObserver{}
		cfg := legacyTriggerConfig(nil, true)
		if mode > 0 {
			cfg.Observer = r
		}
		if mode == 2 {
			r.mix = mixer.New(11550000)
			r.source = sbpcm.New(11550000)
		}
		o, err := New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		defer o.Close()
		boot, err := o.BootFromState(StateInput{State: state, ExpectedStateSHA256: sha256.Sum256(state), SaveRoot: saveRoot, RebaseFrom: []string{"/scratch"}})
		if err != nil {
			t.Fatal(err)
		}
		if boot.Steps != start {
			t.Fatalf("state 起點 %d want %d", boot.Steps, start)
		}
		var f *os.File
		if mode == 2 {
			f, err = os.Create(filepath.Join(out, "mixer-left.f32le"))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
		}
		inputIndex := 0
		nextFrame := min(start+192500, stop)
		renderBefore := start
		var samples uint64
		for o.machine.Steps < stop {
			v, err := o.View()
			if err != nil {
				t.Fatal(err)
			}
			u := CapturedUpdate{StartedPanel: v.Panel, Layout: v.Layout, SourceGeneration: v.SourceGeneration}
			for inputIndex < len(inputs) && inputs[inputIndex].step == o.machine.Steps {
				in := inputs[inputIndex]
				switch in.kind {
				case "move":
					u.PointerMoves = append(u.PointerMoves, dosMove(in.x, in.y))
				case "down":
					u.PointerDown = dosPointer(host.MouseEventDown, in.x, in.y)
				case "up":
					u.PointerUp = dosPointer(host.MouseEventUp, in.x, in.y)
				}
				inputIndex++
			}
			after := nextFrame
			if inputIndex < len(inputs) && inputs[inputIndex].step < after {
				after = inputs[inputIndex].step
			}
			if after <= o.machine.Steps {
				t.Fatal("輸入排程未前進")
			}
			if _, err := o.Deliver(u); err != nil {
				t.Fatal(err)
			}
			turn, err := o.Advance(InstructionBudget(after - o.machine.Steps))
			if err != nil || turn.MachineStepsAfter != after {
				t.Fatalf("mode=%d step=%d: %v", mode, turn.MachineStepsAfter, err)
			}
			if after == nextFrame {
				if mode == 2 {
					buf := r.mix.Render(nil, renderBefore, after)
					j := renderBefore * 48000 / 11550000
					var raw []byte
					for i := 0; i < len(buf); i += 2 {
						want := float32(float64(r.source.Sample(j)) / 32768)
						if math.Float32bits(buf[i]) != math.Float32bits(want) || math.Float32bits(buf[i+1]) != math.Float32bits(want) {
							t.Fatalf("PCM-only mixer 不符 j=%d: %v/%v want %v；SpeakerNonzero=%d PIT2Nonzero=%d", j, buf[i], buf[i+1], want, r.speakerNonzero, r.pit2Nonzero)
						}
						if r.started && j >= r.first {
							raw = binary.LittleEndian.AppendUint32(raw, math.Float32bits(buf[i]))
							samples++
						}
						j++
					}
					if _, err := f.Write(raw); err != nil {
						t.Fatal(err)
					}
					r.source.Prune(after * 48000 / 11550000)
				}
				renderBefore = after
				nextFrame = min(after+192500, stop)
			}
		}
		digest, err := o.Digest()
		if err != nil {
			t.Fatal(err)
		}
		md, err := o.MachineDigest()
		if err != nil {
			t.Fatal(err)
		}
		var ms, ds bytes.Buffer
		if err := o.machine.SaveState(&ms); err != nil {
			t.Fatal(err)
		}
		if err := o.dos.SaveState(&ds); err != nil {
			t.Fatal(err)
		}
		events := r.events
		if mode == 0 {
			events = o.machine.SBDMALog
		}
		var rows strings.Builder
		fmt.Fprintln(&rows, strings.Join(machine.SBDMAEventFields, "\t"))
		for _, e := range events {
			fmt.Fprintln(&rows, strings.Join(e.Fields(), "\t"))
		}
		if err := os.WriteFile(filepath.Join(out, fmt.Sprintf("mode-%d-sbdma.tsv", mode)), []byte(rows.String()), 0644); err != nil {
			t.Fatal(err)
		}
		res := originalPCMResult{Digest: digest, MachineDigest: md, MachineStateSHA256: fmt.Sprintf("%x", sha256.Sum256(ms.Bytes())), DOSStateSHA256: fmt.Sprintf("%x", sha256.Sum256(ds.Bytes())), SBDMASHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(rows.String()))), FirstSample: r.first, EndStep: stop, Samples: samples, OPLWrites: r.oplWrites, SpeakerNonzero: r.speakerNonzero, PIT2Nonzero: r.pit2Nonzero, SBPCMMutated: o.machine.SBPCMMutated(), OPLSynthesisMuted: true, state: ms.Bytes(), dosState: ds.Bytes(), events: events}
		encoded, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, fmt.Sprintf("mode-%d.json", mode)), append(encoded, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
		t.Logf("mode=%d firstSample=%d endStep=%d samples=%d clips=0 oplMuted=%d speakerNonzero=%d pit2Nonzero=%d", mode, r.first, stop, samples, r.oplWrites, r.speakerNonzero, r.pit2Nonzero)
		return res
	}
	base := run(0)
	for _, mode := range []int{1, 2} {
		got := run(mode)
		if got.Digest != base.Digest || got.MachineDigest != base.MachineDigest || !bytes.Equal(got.state, base.state) || !bytes.Equal(got.dosState, base.dosState) || got.SBDMASHA256 != base.SBDMASHA256 {
			t.Fatalf("G5 mode %d 改變客體狀態或 SBDMA", mode)
		}
		if mode == 2 && got.Samples == 0 {
			t.Fatal("G6 沒有 PCM 輸出樣本")
		}
	}
}
