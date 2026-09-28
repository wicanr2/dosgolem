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
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime/pprof"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/wicanr2/dosgolem/apps/buckrogers"
	"github.com/wicanr2/dosgolem/audio/mixer"
	"github.com/wicanr2/dosgolem/bootroot"
	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/session"
)

// stepsPerHostFrame keeps the original pace: one VGA retrace is
// VGAFrameEvery steps at 70 Hz, and the host runs at 60 Hz.  With a clock
// percentage (docs/spec/242) both scale by the same machine.ScaleSteps.
func stepsPerHostFrame(clock int) uint64 {
	return machine.ScaleSteps(machine.DefaultVGAFrameEvery, clock) * 70 / 60
}

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

// namedKeys maps host keys to the names used by input.go (spec 241).
var namedKeys = map[ebiten.Key]string{
	ebiten.KeyEnter: "Enter", ebiten.KeyNumpadEnter: "Enter", ebiten.KeyEscape: "Escape",
	ebiten.KeyBackspace: "Backspace", ebiten.KeyTab: "Tab",
	ebiten.KeyArrowUp: "Up", ebiten.KeyArrowDown: "Down", ebiten.KeyArrowLeft: "Left", ebiten.KeyArrowRight: "Right",
	ebiten.KeyHome: "Home", ebiten.KeyEnd: "End", ebiten.KeyPageUp: "PgUp", ebiten.KeyPageDown: "PgDn",
	ebiten.KeyInsert: "Insert", ebiten.KeyDelete: "Delete",
	ebiten.KeyNumpad0: "KP0", ebiten.KeyNumpad1: "KP1", ebiten.KeyNumpad2: "KP2", ebiten.KeyNumpad3: "KP3",
	ebiten.KeyNumpad4: "KP4", ebiten.KeyNumpad5: "KP5", ebiten.KeyNumpad6: "KP6", ebiten.KeyNumpad7: "KP7",
	ebiten.KeyNumpad8: "KP8", ebiten.KeyNumpad9: "KP9", ebiten.KeyNumpadDecimal: "KPDot",
	ebiten.KeyF1: "F1", ebiten.KeyF2: "F2", ebiten.KeyF3: "F3", ebiten.KeyF4: "F4", ebiten.KeyF5: "F5",
	ebiten.KeyF6: "F6", ebiten.KeyF7: "F7", ebiten.KeyF8: "F8", ebiten.KeyF9: "F9", ebiten.KeyF10: "F10",
	ebiten.KeyF11: "F11", ebiten.KeyF12: "F12",
}

func init() {
	for k := ebiten.KeyA; k <= ebiten.KeyZ; k++ {
		namedKeys[k] = string(rune('A' + (k - ebiten.KeyA)))
	}
}

type game struct {
	owner   *session.Owner
	live    *buckrogers.LiveRuntime
	scale   int
	frame   int
	frames  int // >0: stop after this many frames (automated runs)
	script  map[int][]scriptAction
	shot    string
	shotDir string
	lastErr error

	help     bool
	pressing bool // 左鍵的按下已送出、尚未放開
	focused  bool
	muted    bool
	player   *audio.Player

	budget  uint64 // 每主機畫格的指令數（spec 242）
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

// hostInput reads this frame's keyboard and mouse (interactive mode only).
func (g *game) hostInput(u *session.CapturedUpdate) []hostAction {
	in := frameInput{Chars: ebiten.AppendInputChars(nil), Held: map[string]int{},
		Ctrl: ebiten.IsKeyPressed(ebiten.KeyControl), Alt: ebiten.IsKeyPressed(ebiten.KeyAlt)}
	for k, name := range namedKeys {
		if d := inpututil.KeyPressDuration(k); d > 0 && (in.Held[name] == 0 || d < in.Held[name]) {
			in.Held[name] = d
		}
	}
	keys, acts := mapKeys(in, g.help, g.live.LogbookTurn)
	u.BIOSKeys = append(u.BIOSKeys, keys...)
	if g.help {
		return acts
	}
	cx, cy := ebiten.CursorPosition()
	dx, dy := dosPoint(cx, cy, g.scale)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && !g.pressing {
		if ev := mouseEvent(host.MouseEventDown, dx, dy); ev.Target == host.MouseTargetCanvas {
			u.PointerDown, g.pressing = &ev, true
		}
	}
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) && g.pressing {
		ev := mouseEvent(host.MouseEventUp, dx, dy)
		u.PointerUp, g.pressing = &ev, false
	}
	focused := ebiten.IsFocused()
	if g.focused && !focused && g.pressing {
		u.FocusLost, g.pressing = true, false
	}
	g.focused = focused
	return acts
}

// scriptInput applies this frame's automated actions (spec 241 §3.4).
func (g *game) scriptInput(u *session.CapturedUpdate) []hostAction {
	var acts []hostAction
	for _, a := range g.script[g.frame] {
		switch {
		case a.help:
			acts = append(acts, actHelp)
		case a.scale:
			acts = append(acts, actScale)
		case g.help:
			// 說明頁開啟時不送輸入。
		case a.key != nil:
			u.BIOSKeys = append(u.BIOSKeys, *a.key)
		case a.blur:
			u.FocusLost, g.pressing = true, false
		case a.mouse == host.MouseEventDown:
			ev := mouseEvent(host.MouseEventDown, a.x, a.y)
			u.PointerDown, g.pressing = &ev, true
		case a.mouse == host.MouseEventUp:
			ev := mouseEvent(host.MouseEventUp, a.x, a.y)
			u.PointerUp, g.pressing = &ev, false
		}
	}
	return acts
}

func (g *game) apply(acts []hostAction) {
	for _, a := range acts {
		switch a {
		case actHelp:
			g.help = !g.help
		case actScale:
			g.scale = 5 - g.scale // 2 ↔ 3
			if !ebiten.IsFullscreen() {
				ebiten.SetWindowSize(320*g.scale, 200*g.scale)
			}
		case actMute:
			g.muted = !g.muted
			if g.player != nil {
				g.player.SetVolume(map[bool]float64{true: 0, false: 1}[g.muted])
			}
		case actFullscreen:
			ebiten.SetFullscreen(!ebiten.IsFullscreen())
		case actShot:
			g.screenshot()
		}
	}
}

// screenshot writes the composed frame as PNG; failure only shows in the title.
func (g *game) screenshot() {
	rgba, ok, err := g.live.Compose(g.scale)
	if err == nil && ok {
		w, h := 320*g.scale, 200*g.scale
		img := &image.NRGBA{Pix: append([]byte(nil), rgba...), Stride: 4 * w, Rect: image.Rect(0, 0, w, h)}
		now := time.Now()
		name := fmt.Sprintf("buckrogers-%s-%03d.png", now.Format("20060102-150405"), now.Nanosecond()/1e6)
		if err = os.MkdirAll(g.shotDir, 0o755); err == nil {
			var f *os.File
			if f, err = os.Create(filepath.Join(g.shotDir, name)); err == nil {
				err = png.Encode(f, img)
				if cerr := f.Close(); err == nil {
					err = cerr
				}
			}
		}
	}
	if err != nil || !ok {
		ebiten.SetWindowTitle(windowTitle + "（截圖失敗）")
		return
	}
	ebiten.SetWindowTitle(windowTitle)
}

const windowTitle = "拯救地球（繁中）"

func (g *game) Update() error {
	g.frame++
	var u session.CapturedUpdate
	acts := g.scriptInput(&u)
	if g.frames == 0 {
		acts = append(acts, g.hostInput(&u)...)
	}
	wasHelp := g.help
	g.apply(acts)
	if wasHelp || g.help {
		// 說明頁：View、Deliver、Advance 三者一起跳過（spec 241 §3.3）。
		return g.finishFrame()
	}
	view, err := g.owner.View()
	if err != nil {
		return err
	}
	u.StartedPanel, u.Layout, u.SourceGeneration = view.Panel, view.Layout, view.SourceGeneration
	if _, err := g.owner.Deliver(u); err != nil {
		return err
	}
	tick, err := g.owner.Advance(session.InstructionBudget(g.budget))
	if err != nil {
		return err
	}
	g.renderAudio(tick)
	if tick.Phase == session.PhaseStopped {
		return ebiten.Termination
	}
	return g.finishFrame()
}

// composed returns the frame to show: the live overlay, plus the help page
// when it is open.
func (g *game) composed() ([]byte, bool, error) {
	rgba, ok, err := g.live.Compose(g.scale)
	if err != nil || !ok || !g.help {
		return rgba, ok, err
	}
	out := append([]byte(nil), rgba...)
	buckrogers.DrawHelp(out, g.scale, g.live.Font(), g.live.HelpLines())
	return out, true, nil
}

// finishFrame ends automated runs after -frames, writing -shot.
func (g *game) finishFrame() error {
	if g.frames == 0 || g.frame < g.frames {
		return nil
	}
	if g.shot != "" {
		rgba, ok, err := g.composed()
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
	if d, err := g.owner.Digest(); err == nil {
		fmt.Fprintf(os.Stderr, "buckrogers-play: steps=%d phase=%v clock=%d irq0_clamped=%d\n", d.Steps, g.owner.Status().Phase, d.ClockPercent, d.IRQ0Clamped)
	}
	return ebiten.Termination
}

func (g *game) Draw(screen *ebiten.Image) {
	// Recomposed every frame: the overlay layer changes state on retraces
	// (pending stamps become shown, anchors expire) even when the pixels do
	// not, so a pixel-keyed cache could show a stale overlay.
	rgba, ok, err := g.composed()
	if err != nil {
		g.lastErr = err
		return
	}
	// F2 切換倍率後，這一格的 screen 可能仍是舊尺寸（Layout 下一格才生效）；
	// 尺寸不合就略過這一格，不把新尺寸的像素寫進舊畫面。
	if b := screen.Bounds(); ok && len(rgba) == 4*b.Dx()*b.Dy() {
		screen.WritePixels(rgba)
	}
}

func (g *game) Layout(_, _ int) (int, int) { return 320 * g.scale, 200 * g.scale }

func die(err error) { fatal(err) }

func main() {
	original := flag.String("original", "", "唯讀原版目錄（預設找發行包旁的 original）")
	save := flag.String("save", "", "開發模式：可寫存檔目錄（須不存在或為空，每次重新複製）；不給則用使用者資料目錄")
	exe := flag.String("exe", "START.EXE", "開機執行檔名")
	exeSHA := flag.String("exe-sha256", requiredOriginals[0].sha, "開機執行檔 SHA-256（hex）")
	textDir := flag.String("text-dir", "", "繁中 catalog 目錄")
	fontPath := flag.String("font", "", "16×16 GOLEMFNT（預設為發行包的 font/buckrogers-unifont.golemfnt）")
	manualEnglish := flag.String("manual-english", "", "本機手冊英文摘錄（規格 034；不給則關閉）")
	scale := flag.Int("scale", 2, "起始倍率（2 或 3；F2 切換）")
	frames := flag.Int("frames", 0, "自動模式：跑這麼多畫格後結束（0＝互動）")
	script := flag.String("script", "", "自動模式腳本：`畫格:動作[,…]`（鍵名、Ctrl+X、Alt+X、click@x;y、press@x;y、release@x;y、blur、help、scale）")
	shot := flag.String("shot", "", "自動模式結束時輸出合成 RGBA")
	shotDir := flag.String("shot-dir", "", "F12 截圖目錄（預設為存檔目錄上一層的 screenshots）")
	cpuProfile := flag.String("cpuprofile", "", "把 CPU 剖析寫到這個檔")
	clock := flag.Int("clock", 50, "機器時脈比例 10–100（規格 242；主機跑不動時調低，遊戲時間仍對齊實際時間）")
	adlib := flag.Bool("adlib", true, "模擬 AdLib（規格 240；關閉則原版只用 PC 喇叭）")
	wavPath := flag.String("wav", "", "自動模式：把合成的音訊寫成 WAV")
	version := flag.Bool("version", false, "顯示版本與第三方元件授權")
	flag.Parse()
	if *version {
		fmt.Print(versionString() + versionText)
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
	if *clock < 10 || *clock > 100 {
		die(fmt.Errorf("-clock 須在 10–100，得 %d", *clock))
	}
	if *scale != 2 && *scale != 3 {
		flag.Usage()
		os.Exit(2)
	}
	data, err := userDataDir()
	if err != nil {
		die(err)
	}
	if *save == "" {
		logDir = data
	}
	if *textDir == "" {
		if *textDir, err = resourcePath("text"); err != nil {
			die(err)
		}
	}
	if *fontPath == "" {
		// 本機自用完整版另附倚天字型（Buck repo 規格 035 §1.1），有就優先用；一般版只有 Unifont。
		if p, e := resourcePath(filepath.Join("font", "buckrogers-eten-top-pad.golemfnt")); e == nil {
			*fontPath = p
		} else if *fontPath, err = resourcePath(filepath.Join("font", "buckrogers-unifont.golemfnt")); err != nil {
			die(err)
		}
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
	var saveRoot, exeName string
	if *save != "" {
		if *original == "" {
			die(errors.New("開發模式（-save）須同時給 -original"))
		}
		out, err := bootroot.Prepare(bootroot.BootRootInput{OriginalRoot: *original, SaveRoot: *save,
			Required: []bootroot.RequiredFile{{Name: *exe, SHA256: want}}})
		if err != nil {
			die(err)
		}
		saveRoot, exeName = out.SaveRoot, *exe
	} else if saveRoot, exeName, err = prepareGame(*original, data); err != nil {
		die(err)
	}
	exeBytes, err := os.ReadFile(filepath.Join(saveRoot, exeName))
	if err != nil {
		die(err)
	}
	s := host.OutputScale2
	layout := host.MouseLayout{Epoch: 1, Scale: s, ChromeHeight: 18 * int(s), Canvas: host.Canvas{Width: 320, Height: 200},
		FrameWidth: 320 * int(s), FrameHeight: 18*int(s) + 200*int(s)}
	budget := stepsPerHostFrame(*clock)
	mix := mixer.New(budget * 60)
	owner, err := session.New(session.Config{InitialScale: s, InitialLayout: layout, AdLib: *adlib, ClockPercent: *clock, Observer: newLiveObserver(live, mix)})
	if err != nil {
		die(err)
	}
	defer owner.Close()
	if _, err := owner.BootOriginal(session.BootInput{EXE: exeBytes, ExpectedEXESHA256: want, SaveRoot: saveRoot}); err != nil {
		die(err)
	}
	clearErrorLog()
	dir := *shotDir
	if dir == "" {
		dir = filepath.Join(filepath.Dir(filepath.Clean(saveRoot)), "screenshots")
	}
	g := &game{owner: owner, live: live, scale: *scale, frames: *frames, script: keys, shot: *shot, shotDir: dir, budget: budget,
		mix: mix, wavPath: *wavPath, focused: true}
	if *frames == 0 {
		// 播放只在互動模式：自動模式常在沒有音效裝置的容器裡跑。
		g.ring = mixer.NewRing(200)
		player, err := audio.NewContext(mixer.SampleRate).NewPlayerF32(g.ring)
		if err != nil {
			die(err)
		}
		player.SetBufferSize(50 * time.Millisecond)
		player.Play()
		g.player = player
	}
	ebiten.SetWindowTitle(windowTitle)
	ebiten.SetWindowSize(320*g.scale, 200*g.scale)
	ebiten.SetTPS(60)
	started := time.Now()
	if err := ebiten.RunGame(g); err != nil {
		die(err)
	}
	if g.ring != nil {
		// 回報用：實際畫格率與播放斷流次數（規格 240 §3.5）。
		wall := time.Since(started).Seconds()
		u, d := g.ring.Stats()
		fmt.Fprintf(os.Stderr, "buckrogers-play: frames=%d wall=%.1fs fps=%.1f audio_underruns=%d audio_dropped_frames=%d\n",
			g.frame, wall, float64(g.frame)/wall, u, d)
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
