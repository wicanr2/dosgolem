// Command buckrogers-play is the first playable Linux window for the Buck
// Rogers traditional-Chinese overlay: a sealed session booted from the
// original tree, driven one host frame at a time, with the live overlay
// composed at 2× or 3× (F2 switches; the game never sees F2).
//
// It deliberately has no settings panel or mouse routing yet; those remain
// with frontend/ebiten and specs 004／019.
package main

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/wicanr2/dosgolem/apps/buckrogers"
	"github.com/wicanr2/dosgolem/audio/mixer"
	"github.com/wicanr2/dosgolem/bootroot"
	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/session"
)

// stepsPerHostFrame keeps the original pace: one VGA retrace is
// machine.DefaultVGAFrameEvery steps at 70 Hz, and the host runs at 60 Hz.
const stepsPerHostFrame = machine.DefaultVGAFrameEvery * 70 / 60

// liveObserver adapts the Buck live runtime to the sealed session observer.
// The cursor avoids boxing a fresh StepView into an interface every step.
type liveObserver struct {
	r      *buckrogers.LiveRuntime
	cursor *buckrogers.StepCursor[session.StepView]
	mix    *mixer.Mixer // nil：不收音訊
}

func newLiveObserver(r *buckrogers.LiveRuntime, mix *mixer.Mixer) liveObserver {
	return liveObserver{r: r, cursor: &buckrogers.StepCursor[session.StepView]{}, mix: mix}
}

// The audio methods make liveObserver a session.AudioObserver (spec 240).
func (o liveObserver) OPLWrite(w machine.OPLWrite) {
	if o.mix != nil {
		o.mix.OPLWrite(w)
	}
}
func (o liveObserver) SpeakerSample(s machine.SpeakerSample) {
	if o.mix != nil {
		o.mix.SpeakerSample(s)
	}
}
func (o liveObserver) PITChannel2(c machine.PIT2Change) {
	if o.mix != nil {
		o.mix.PITChannel2(c)
	}
}

func (o liveObserver) BeforeStep(v session.StepView) error {
	o.cursor.Set(v)
	return o.r.BeforeStep(o.cursor)
}
func (o liveObserver) VideoWrite(w machine.VideoWrite) { o.r.VideoWrite(w) }
func (o liveObserver) Frame(indexed []byte, palette [256][3]uint8) {
	o.r.Frame(indexed, palette)
}

// namedKeys maps host keys that produce no input character to BIOS keys.
var namedKeys = map[ebiten.Key]string{
	ebiten.KeyEnter: "Enter", ebiten.KeyNumpadEnter: "Enter", ebiten.KeyEscape: "Escape",
	ebiten.KeyBackspace: "Backspace", ebiten.KeyTab: "Tab",
	ebiten.KeyArrowUp: "Up", ebiten.KeyArrowDown: "Down", ebiten.KeyArrowLeft: "Left", ebiten.KeyArrowRight: "Right",
	ebiten.KeyHome: "Home", ebiten.KeyEnd: "End", ebiten.KeyPageUp: "PgUp", ebiten.KeyPageDown: "PgDn",
	ebiten.KeyInsert: "Insert", ebiten.KeyDelete: "Delete",
}

type game struct {
	owner   *session.Owner
	live    *buckrogers.LiveRuntime
	scale   int
	frame   int
	frames  int // >0: stop after this many frames (automated runs)
	script  map[int][]dos.Key
	shot    string
	lastErr error

	mix     *mixer.Mixer
	ring    *mixer.Ring // nil：沒有播放裝置（自動模式）
	samples []float32
	wav     []int16 // 自動模式 -wav 收集的樣本
	wavPath string
}

// renderAudio synthesises this turn's steps (spec 240 §3.5).
func (g *game) renderAudio(tick session.TickReceipt) {
	if g.mix == nil {
		return
	}
	g.samples = g.mix.Render(g.samples[:0], tick.MachineStepsBefore, tick.MachineStepsAfter)
	if g.ring != nil {
		g.ring.Push(g.samples)
	}
	if g.wavPath != "" {
		for _, v := range g.samples {
			g.wav = append(g.wav, int16(v*32767))
		}
	}
}

func (g *game) hostKeys() []dos.Key {
	var keys []dos.Key
	for _, r := range ebiten.AppendInputChars(nil) {
		if k, ok := dos.KeyForRune(r); ok {
			keys = append(keys, k)
		}
	}
	for _, k := range inpututil.AppendJustPressedKeys(nil) {
		if k == ebiten.KeyF2 {
			g.scale = 5 - g.scale // 2 ↔ 3
			continue
		}
		// An open logbook panel (Buck spec 030) takes PgDn/PgUp; otherwise
		// they go to the game as usual.
		if k == ebiten.KeyPageDown && g.live.LogbookTurn(1) || k == ebiten.KeyPageUp && g.live.LogbookTurn(-1) {
			continue
		}
		if name, ok := namedKeys[k]; ok {
			if key, found := dos.KeyNamed(name); found {
				keys = append(keys, key)
			}
		}
	}
	return keys
}

func (g *game) Update() error {
	g.frame++
	keys := append(g.hostKeys(), g.script[g.frame]...)
	view, err := g.owner.View()
	if err != nil {
		return err
	}
	if _, err := g.owner.Deliver(session.CapturedUpdate{StartedPanel: view.Panel, Layout: view.Layout, SourceGeneration: view.SourceGeneration, BIOSKeys: keys}); err != nil {
		return err
	}
	tick, err := g.owner.Advance(session.InstructionBudget(stepsPerHostFrame))
	if err != nil {
		return err
	}
	g.renderAudio(tick)
	if tick.Phase == session.PhaseStopped {
		return ebiten.Termination
	}
	if g.frames > 0 && g.frame >= g.frames {
		if g.shot != "" {
			rgba, ok, err := g.live.Compose(g.scale)
			if err != nil {
				return err
			}
			if !ok {
				return errors.New("尚無畫格可截圖")
			}
			if err := os.WriteFile(g.shot, rgba, 0o600); err != nil {
				return err
			}
		}
		return ebiten.Termination
	}
	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	// Recomposed every frame: the overlay layer changes state on retraces
	// (pending stamps become shown, anchors expire) even when the pixels do
	// not, so a pixel-keyed cache could show a stale overlay.
	rgba, ok, err := g.live.Compose(g.scale)
	if err != nil {
		g.lastErr = err
		return
	}
	if ok {
		screen.WritePixels(rgba)
	}
}

func (g *game) Layout(_, _ int) (int, int) { return 320 * g.scale, 200 * g.scale }

func die(err error) {
	fmt.Fprintln(os.Stderr, "buckrogers-play:", err)
	os.Exit(1)
}

func parseScript(s string) (map[int][]dos.Key, error) {
	out := map[int][]dos.Key{}
	if s == "" {
		return out, nil
	}
	for _, item := range strings.Split(s, ",") {
		frameText, name, ok := strings.Cut(item, ":")
		frame, err := strconv.Atoi(frameText)
		if !ok || err != nil || frame < 1 {
			return nil, fmt.Errorf("script 項目 %q 須為 畫格:鍵", item)
		}
		key, found := dos.KeyNamed(name)
		if !found && len([]rune(name)) == 1 {
			key, found = dos.KeyForRune([]rune(name)[0])
		}
		if !found {
			return nil, fmt.Errorf("script 鍵 %q 無 BIOS 對映", name)
		}
		out[frame] = append(out[frame], key)
	}
	return out, nil
}

func main() {
	original := flag.String("original", "", "唯讀原版樹目錄")
	save := flag.String("save", "", "可寫存檔目錄（須不存在或為空）")
	exe := flag.String("exe", "START.EXE", "開機執行檔名")
	exeSHA := flag.String("exe-sha256", "", "開機執行檔 SHA-256（hex）")
	textDir := flag.String("text-dir", "", "繁中 catalog 目錄")
	fontPath := flag.String("font", "", "本機 16×16 倚天 GOLEMFNT")
	manualEnglish := flag.String("manual-english", "", "本機手冊英文摘錄（規格 034；不給則關閉）")
	scale := flag.Int("scale", 2, "起始倍率（2 或 3；F2 切換）")
	frames := flag.Int("frames", 0, "自動模式：跑這麼多畫格後結束（0＝互動）")
	script := flag.String("script", "", "自動模式送鍵：`畫格:鍵[,…]`")
	shot := flag.String("shot", "", "自動模式結束時輸出合成 RGBA")
	cpuProfile := flag.String("cpuprofile", "", "把 CPU 剖析寫到這個檔")
	adlib := flag.Bool("adlib", true, "模擬 AdLib（規格 240；關閉則原版只用 PC 喇叭）")
	wavPath := flag.String("wav", "", "自動模式：把合成的音訊寫成 WAV")
	version := flag.Bool("version", false, "顯示版本與第三方元件授權")
	flag.Parse()
	if *version {
		fmt.Print(versionText)
		return
	}
	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			die(err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			die(err)
		}
		defer pprof.StopCPUProfile()
	}
	if *original == "" || *save == "" || *exeSHA == "" || *textDir == "" || *fontPath == "" || (*scale != 2 && *scale != 3) {
		flag.Usage()
		os.Exit(2)
	}
	sum, err := hex.DecodeString(*exeSHA)
	if err != nil || len(sum) != 32 {
		die(errors.New("exe-sha256 須為 64 hex 字元"))
	}
	var want [32]byte
	copy(want[:], sum)
	keys, err := parseScript(*script)
	if err != nil {
		die(err)
	}
	live, err := buckrogers.LoadLiveRuntime(*textDir, *fontPath)
	if err != nil {
		die(err)
	}
	if err := live.SetManualEnglish(*manualEnglish); err != nil {
		die(err)
	}
	out, err := bootroot.Prepare(bootroot.BootRootInput{OriginalRoot: *original, SaveRoot: *save,
		Required: []bootroot.RequiredFile{{Name: *exe, SHA256: want}}})
	if err != nil {
		die(err)
	}
	exeBytes, err := os.ReadFile(filepath.Join(out.SaveRoot, *exe))
	if err != nil {
		die(err)
	}
	s := host.OutputScale2
	layout := host.MouseLayout{Epoch: 1, Scale: s, ChromeHeight: 18 * int(s), Canvas: host.Canvas{Width: 320, Height: 200},
		FrameWidth: 320 * int(s), FrameHeight: 18*int(s) + 200*int(s)}
	mix := mixer.New(stepsPerHostFrame * 60)
	owner, err := session.New(session.Config{InitialScale: s, InitialLayout: layout, AdLib: *adlib, Observer: newLiveObserver(live, mix)})
	if err != nil {
		die(err)
	}
	defer owner.Close()
	if _, err := owner.BootOriginal(session.BootInput{EXE: exeBytes, ExpectedEXESHA256: want, SaveRoot: out.SaveRoot}); err != nil {
		die(err)
	}
	g := &game{owner: owner, live: live, scale: *scale, frames: *frames, script: keys, shot: *shot, mix: mix, wavPath: *wavPath}
	if *frames == 0 {
		// 播放只在互動模式：自動模式常在沒有音效裝置的容器裡跑。
		g.ring = mixer.NewRing(200)
		player, err := audio.NewContext(mixer.SampleRate).NewPlayerF32(g.ring)
		if err != nil {
			die(err)
		}
		player.SetBufferSize(50 * time.Millisecond)
		player.Play()
	}
	ebiten.SetWindowTitle("拯救地球（繁中）")
	ebiten.SetTPS(60)
	if err := ebiten.RunGame(g); err != nil {
		die(err)
	}
	if g.lastErr != nil {
		die(g.lastErr)
	}
	if g.wavPath != "" {
		if err := os.WriteFile(g.wavPath, wavBytes(g.wav), 0o600); err != nil {
			die(err)
		}
	}
}

// wavBytes wraps 48 kHz stereo int16 samples in a RIFF/WAVE header.
func wavBytes(pcm []int16) []byte {
	data := len(pcm) * 2
	b := make([]byte, 0, 44+data)
	b = append(b, "RIFF"...)
	b = binary.LittleEndian.AppendUint32(b, uint32(36+data))
	b = append(b, "WAVEfmt "...)
	b = binary.LittleEndian.AppendUint32(b, 16)
	b = binary.LittleEndian.AppendUint16(b, 1) // PCM
	b = binary.LittleEndian.AppendUint16(b, 2)
	b = binary.LittleEndian.AppendUint32(b, mixer.SampleRate)
	b = binary.LittleEndian.AppendUint32(b, mixer.SampleRate*4)
	b = binary.LittleEndian.AppendUint16(b, 4)
	b = binary.LittleEndian.AppendUint16(b, 16)
	b = append(b, "data"...)
	b = binary.LittleEndian.AppendUint32(b, uint32(data))
	for _, v := range pcm {
		b = binary.LittleEndian.AppendUint16(b, uint16(v))
	}
	return b
}
