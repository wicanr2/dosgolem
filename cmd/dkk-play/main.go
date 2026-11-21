// Command dkk-play 是《Death Knights of Krynn》繁中覆繪的可遊玩視窗（death-knights-of-krynn-cht #6）。
//
// 以 session 從原版目錄開機（bootroot 複製到可寫存檔目錄，原版唯讀），每個主機畫格推進
// 原版節奏的指令數，以 apps/dkk.Live 疊上路徑 A、B、C 的中文，2× 或 3× 輸出（F2 切換）。
// F12 即時切換中文／原版（原版看不到這個鍵），F2 切換 2×／3×，F11 全螢幕，F3 靜音，F10 截圖，F9 查驗題答案顯示，
// F5 翻閱已讀手札（PgUp／PgDn 翻頁，←／→ 換則，Esc 關閉）。
//
// 自動模式（-frames）供收據使用：依 -script 送鍵，結束時印記憶體與畫面雜湊，
// 並可輸出 -shot（中文合成 RGBA）與 -shot-original（原版合成 RGBA）。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/wicanr2/dosgolem/apps/dkk"
	"github.com/wicanr2/dosgolem/audio/mixer"
	"github.com/wicanr2/dosgolem/bootroot"
	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/session"
	"github.com/wicanr2/dosgolem/xlate"
)

const (
	windowTitle = "幽靈騎士"
	// startEXESHA256 是 G1 盤點的 START.EXE（death-knights-of-krynn-cht docs/re/001）。
	startEXESHA256 = "5426c2098b92eb0712e8a238a07fa3923fbb494a2667f3a16eb3be25c1f3a458"
)

func stepsPerHostFrame(clock int) uint64 {
	return machine.ScaleSteps(machine.DefaultVGAFrameEvery, clock) * 70 / 60
}

// observer 把 session 的觀測事件轉給 dkk.Live 與音訊混音器。
type observer struct {
	live *dkk.Live
	mix  *mixer.Mixer
}

func (o observer) BeforeStep(v session.StepView) error { return o.live.BeforeStep(v) }
func (o observer) VideoWrite(w machine.VideoWrite)     { o.live.VideoWrite(w.Offset) }
func (o observer) VideoModeChange(machine.ModeChange)  { o.live.ModeChange() }
func (o observer) Frame(indexed []byte, palette [256][3]uint8) {
	o.live.Frame(indexed, palette)
}
func (o observer) OPLWrite(w machine.OPLWrite)           { o.mix.OPLWrite(w) }
func (o observer) SpeakerSample(s machine.SpeakerSample) { o.mix.SpeakerSample(s) }
func (o observer) PITChannel2(c machine.PIT2Change)      { o.mix.PITChannel2(c) }

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
	owner    *session.Owner
	scale    int // 輸出倍率 2 或 3（F2 切換，比照拯救地球）
	live     *dkk.Live
	chinese  bool
	frame    int
	frames   int
	script   map[int][]scriptAction
	shot     string
	shotOrig string
	shotDir  string
	budget   uint64
	mix      *mixer.Mixer
	ring     *mixer.Ring
	player   *audio.Player
	muted    bool
	pressing bool
	focused  bool
	samples  []float32
	missing  map[rune]bool
}

func (g *game) hostInput(u *session.CapturedUpdate) []hostAction {
	in := frameInput{Chars: ebiten.AppendInputChars(nil), Held: map[string]int{},
		Ctrl: ebiten.IsKeyPressed(ebiten.KeyControl), Alt: ebiten.IsKeyPressed(ebiten.KeyAlt)}
	for k, name := range namedKeys {
		if d := inpututil.KeyPressDuration(k); d > 0 && (in.Held[name] == 0 || d < in.Held[name]) {
			in.Held[name] = d
		}
	}
	keys, acts := mapKeys(in, g.panelMode())
	u.BIOSKeys = append(u.BIOSKeys, keys...)
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

func (g *game) apply(acts []hostAction) {
	for _, a := range acts {
		switch a {
		case actToggle:
			g.chinese = !g.chinese
		case actMute:
			g.muted = !g.muted
			if g.player != nil {
				g.player.SetVolume(map[bool]float64{true: 0, false: 1}[g.muted])
			}
		case actScale:
			g.scale = 5 - g.scale // 2 ↔ 3
			if !ebiten.IsFullscreen() {
				ebiten.SetWindowSize(320*g.scale, 200*g.scale)
			}
		case actFullscreen:
			ebiten.SetFullscreen(!ebiten.IsFullscreen())
		case actShot:
			g.screenshot()
		case actAnswers:
			g.live.SetHideAnswers(!g.live.HideAnswers())
		case actJournal:
			if g.chinese && g.live.Journal != nil {
				g.live.Guide.Close() // 與攻略面板、地圖互斥（規格 008 §2.1、009 §2.3）
				g.live.Map.Close()
				g.live.Journal.ToggleBrowse()
			}
		case actGuide:
			if g.chinese && g.live.Guide != nil {
				if g.live.Journal.Browsing() {
					g.live.Journal.ToggleBrowse()
				}
				g.live.Map.Close()
				g.live.Guide.Toggle()
			}
		case actMap:
			if g.chinese && g.live.Map != nil {
				if g.live.Journal.Browsing() {
					g.live.Journal.ToggleBrowse()
				}
				g.live.Guide.Close()
				g.live.Map.Toggle()
			}
		case actReveal:
			g.live.Map.ToggleReveal()
		case actCloseJournal:
			if g.live.Map.Open() {
				g.live.Map.Close()
			} else if g.live.Guide.Open() {
				g.live.Guide.Close()
			} else if g.live.Journal.Browsing() {
				g.live.Journal.ToggleBrowse()
			}
		case actPageUp, actPageDown, actPrevEntry, actNextEntry:
			d := map[hostAction]int{actPageUp: -1, actPageDown: 1, actPrevEntry: -1, actNextEntry: 1}[a]
			switch {
			case g.live.Map.Open() && (a == actPageUp || a == actPageDown):
				g.live.Map.TurnList(d)
			case g.live.Map.Open():
			case g.live.Guide.Open() && (a == actPageUp || a == actPageDown):
				g.live.Guide.TurnPage(d)
			case g.live.Guide.Open():
				g.live.Guide.NextChapter(d)
			case a == actPageUp || a == actPageDown:
				g.live.Journal.TurnPage(d)
			default:
				g.live.Journal.NextEntry(d)
			}
		}
	}
}

func (g *game) panelMode() panelMode {
	if g.chinese && (g.live.Guide.Open() || g.live.Map.Open()) {
		return panelBrowse // 攻略面板與手札翻閱同一組鍵（規格 008 §2.1）
	}
	switch j := g.live.Journal; {
	case !g.chinese || !j.Open():
		return panelNone
	case j.Browsing():
		return panelBrowse
	}
	return panelAuto
}

func (g *game) compose() []byte {
	return g.live.Compose(g.scale, g.chinese, func(r rune) {
		if !g.missing[r] {
			g.missing[r] = true
			fmt.Fprintf(os.Stderr, "dkk-play: 字型缺字 %q（U+%04X）\n", r, r)
		}
	})
}

func (g *game) screenshot() {
	rgba := g.compose()
	if rgba == nil {
		return
	}
	img := &image.NRGBA{Pix: rgba, Stride: 4 * 320 * g.scale, Rect: image.Rect(0, 0, 320*g.scale, 200*g.scale)}
	name := fmt.Sprintf("dkk-%s.png", time.Now().Format("20060102-150405.000"))
	err := os.MkdirAll(g.shotDir, 0o755)
	if err == nil {
		var f *os.File
		if f, err = os.Create(filepath.Join(g.shotDir, name)); err == nil {
			err = png.Encode(f, img)
			if cerr := f.Close(); err == nil {
				err = cerr
			}
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "dkk-play: 截圖失敗：", err)
	}
}

func (g *game) Update() error {
	g.frame++
	var u session.CapturedUpdate
	var acts []hostAction
	for _, a := range g.script[g.frame] {
		if a.toggle {
			acts = append(acts, actToggle)
		} else if a.key != nil {
			u.BIOSKeys = append(u.BIOSKeys, *a.key)
		}
	}
	if g.frames == 0 {
		acts = append(acts, g.hostInput(&u)...)
	}
	g.apply(acts)
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
	g.samples = g.mix.Render(g.samples[:0], tick.MachineStepsBefore, tick.MachineStepsAfter)
	if g.ring != nil {
		g.ring.Push(g.samples)
	}
	if tick.Phase == session.PhaseStopped {
		return ebiten.Termination
	}
	if g.frames > 0 && g.frame >= g.frames {
		return g.finish()
	}
	return nil
}

func (g *game) finish() error {
	write := func(path string, overlay bool) error {
		if path == "" {
			return nil
		}
		rgba := g.live.Compose(g.scale, overlay, nil)
		if rgba == nil {
			return errors.New("尚無畫格可截圖")
		}
		return os.WriteFile(path, rgba, 0o600)
	}
	if err := write(g.shot, true); err != nil {
		return err
	}
	if err := write(g.shotOrig, false); err != nil {
		return err
	}
	if g.live.BindErr != nil {
		fmt.Fprintln(os.Stderr, "dkk-play:", g.live.BindErr)
	} else {
		fmt.Fprintf(os.Stderr, "dkk-play: 攔截點綁定於 #%d：A %05X（%d 次）C %05X\n", g.live.BoundAt, g.live.A.Entry, len(g.live.A.Hits), g.live.C.Entry)
		if g.live.B != nil {
			fmt.Fprintf(os.Stderr, "dkk-play: B %05X（%d 次）\n", g.live.B.Entry, len(g.live.B.Hits))
		}
		tally := func(name string, hs []dkk.Hit) {
			c := map[string]int{}
			for _, h := range hs {
				r := h.Rejected
				if r == "" {
					r = "覆繪"
				}
				c[r]++
			}
			fmt.Fprintf(os.Stderr, "dkk-play: %s %v\n", name, c)
		}
		tally("A", g.live.A.Hits)
		if g.live.B != nil {
			tally("B", g.live.B.Hits)
		}
		fmt.Fprintf(os.Stderr, "dkk-play: Layer stamps=%d\n", len(g.live.Layer.Stamps))
	}
	if d, err := g.owner.Digest(); err == nil {
		fmt.Fprintf(os.Stderr, "dkk-play: frames=%d steps=%d memory_sha256=%x indexed_sha256=%x\n", g.frame, d.Steps, d.MemorySHA256, d.IndexedSHA256)
	}
	return ebiten.Termination
}

func (g *game) Draw(screen *ebiten.Image) {
	rgba := g.compose()
	if b := screen.Bounds(); rgba != nil && len(rgba) == 4*b.Dx()*b.Dy() {
		screen.WritePixels(rgba)
	}
}

func (g *game) Layout(_, _ int) (int, int) { return 320 * g.scale, 200 * g.scale }

func fail(err error) {
	fmt.Fprintln(os.Stderr, "dkk-play：", err)
	os.Exit(1)
}

func main() {
	original := flag.String("original", "", "唯讀原版目錄（含 START.EXE）")
	save := flag.String("save", "", "可寫存檔目錄：不存在或為空時由原版複製；已有 START.EXE（雜湊相符）則沿用，存檔保留")
	textDir := flag.String("text-dir", "", "譯文目錄（ui-text-*、ecl-text-* TSV）")
	fontPath := flag.String("font", "", "16×16 GOLEMFNT")
	frames := flag.Int("frames", 0, "自動模式：跑這麼多畫格後結束（0＝互動）")
	script := flag.String("script", "", "自動模式腳本：畫格:動作[,…]（鍵名、單一字元、toggle）")
	shot := flag.String("shot", "", "自動模式結束時輸出中文合成 RGBA")
	shotOrig := flag.String("shot-original", "", "自動模式結束時輸出原版合成 RGBA")
	shotDir := flag.String("shot-dir", "", "F10 截圖目錄（預設存檔目錄旁的 screenshots）")
	clock := flag.Int("clock", 50, "機器時脈比例 10–100")
	adlib := flag.Bool("adlib", true, "模擬 AdLib")
	audioOn := flag.Bool("audio", true, "互動模式播放聲音（容器內沒有音訊裝置時關閉）")
	hideAnswers := flag.Bool("hide-answers", false, "開機時隱藏查驗題答案（F9 切換）")
	original0 := flag.Bool("original-view", false, "開機時顯示原版（F12 切換）")
	invPath := flag.String("inventory", "", "結束時把印字路徑盤點（issue #11）寫成這個 TSV；含原文，只放工作區")
	scaleFlag := flag.Int("scale", 2, "起始倍率（2 或 3；F2 切換）")
	hdDir := flag.String("hd", "", "HD 主題目錄（theme.json；主專案規格 010，只在 3× 顯示）")
	flag.Parse()
	if *original == "" || *save == "" || *textDir == "" || *fontPath == "" {
		flag.Usage()
		os.Exit(2)
	}
	if *clock < 10 || *clock > 100 {
		fail(fmt.Errorf("-clock 須在 10–100"))
	}
	if *scaleFlag != 2 && *scaleFlag != 3 {
		fail(fmt.Errorf("-scale 須為 2 或 3"))
	}
	font, err := xlate.LoadFont(*fontPath)
	if err != nil {
		fail(err)
	}
	font.Name = filepath.Base(*fontPath)
	ui, ecl, err := dkk.LoadTextDir(*textDir)
	if err != nil {
		fail(err)
	}
	keys, err := parseScript(*script)
	if err != nil {
		fail(err)
	}
	sum, _ := hex.DecodeString(startEXESHA256)
	var want [32]byte
	copy(want[:], sum)
	saveRoot := *save
	if _, err := os.Stat(filepath.Join(saveRoot, "START.EXE")); err != nil {
		out, err := bootroot.Prepare(bootroot.BootRootInput{OriginalRoot: *original, SaveRoot: *save,
			Required: []bootroot.RequiredFile{{Name: "START.EXE", SHA256: want}}})
		if err != nil {
			fail(err)
		}
		saveRoot = out.SaveRoot
	}
	exe, err := os.ReadFile(filepath.Join(saveRoot, "START.EXE"))
	if err != nil {
		fail(err)
	}
	if got := sha256.Sum256(exe); got != want {
		fail(errors.New("START.EXE 雜湊不符"))
	}
	live := dkk.NewLive(font, ui, ecl)
	if *hdDir != "" {
		if err := live.AddHD(*hdDir); err != nil {
			fail(err)
		}
	}
	live.Inventory = *invPath != ""
	if live.Answers, live.QuizFormat, err = dkk.LoadQuizDir(*textDir); err != nil {
		fail(err)
	}
	live.SetHideAnswers(*hideAnswers)
	if live.Journal, err = dkk.LoadJournalDir(*textDir); err != nil {
		fail(err)
	}
	if live.Guide, err = dkk.LoadGuideDir(*textDir); err != nil {
		fail(err)
	}
	if live.Guide != nil {
		posPath := filepath.Join(*save, "dkk-walkthrough-pos.txt")
		ch, page, err := dkk.ReadGuidePos(posPath)
		if err != nil {
			fail(err)
		}
		if ch != 0 || page != 0 {
			live.Guide.SetPos(ch, page)
		}
		live.Guide.OnMove = func(ch, page int) {
			if err := dkk.WriteGuidePos(posPath, ch, page); err != nil {
				fmt.Fprintln(os.Stderr, "dkk-play: 攻略位置寫入失敗：", err)
			}
		}
	}
	if live.Map, err = dkk.LoadAutoMapDir(*textDir); err != nil {
		fail(err)
	}
	if live.Map != nil {
		visitedPath := filepath.Join(*save, "dkk-map-visited.txt")
		v, err := dkk.ReadVisited(visitedPath)
		if err != nil {
			fail(err)
		}
		live.Map.SetVisited(v)
		live.Map.OnVisit = func(key string, x, y int) {
			if err := dkk.AppendVisited(visitedPath, key, x, y); err != nil {
				fmt.Fprintln(os.Stderr, "dkk-play: 地圖走過格子寫入失敗：", err)
			}
		}
	}
	seenPath := filepath.Join(*save, "dkk-journal-seen.txt")
	if live.Journal.Seen, err = dkk.ReadJournalSeen(seenPath); err != nil {
		fail(err)
	}
	live.Journal.OnSeen = func(n int) {
		if err := dkk.AppendJournalSeen(seenPath, n); err != nil {
			fmt.Fprintln(os.Stderr, "dkk-play: 已讀手札寫入失敗：", err)
		}
	}
	s := host.OutputScale2
	layout := host.MouseLayout{Epoch: 1, Scale: s, ChromeHeight: 18 * int(s), Canvas: host.Canvas{Width: 320, Height: 200},
		FrameWidth: 320 * int(s), FrameHeight: 18*int(s) + 200*int(s)}
	budget := stepsPerHostFrame(*clock)
	mix := mixer.New(budget * 60)
	owner, err := session.New(session.Config{InitialScale: s, InitialLayout: layout, AdLib: *adlib, ClockPercent: *clock,
		Observer: observer{live: live, mix: mix}})
	if err != nil {
		fail(err)
	}
	defer owner.Close()
	if _, err := owner.BootOriginal(session.BootInput{EXE: exe, ExpectedEXESHA256: want, SaveRoot: saveRoot}); err != nil {
		fail(err)
	}
	dir := *shotDir
	if dir == "" {
		dir = filepath.Join(filepath.Dir(filepath.Clean(saveRoot)), "screenshots")
	}
	g := &game{owner: owner, live: live, scale: *scaleFlag, chinese: !*original0, frames: *frames, script: keys, shot: *shot, shotOrig: *shotOrig,
		shotDir: dir, budget: budget, mix: mix, focused: true, missing: map[rune]bool{}}
	if *frames == 0 && *audioOn {
		g.ring = mixer.NewRing(200)
		player, err := audio.NewContext(mixer.SampleRate).NewPlayerF32(g.ring)
		if err != nil {
			fail(err)
		}
		player.SetBufferSize(50 * time.Millisecond)
		player.Play()
		g.player = player
	}
	ebiten.SetWindowTitle(windowTitle)
	ebiten.SetWindowSize(320*g.scale, 200*g.scale)
	ebiten.SetTPS(60)
	err = ebiten.RunGame(g)
	if live.Inv != nil {
		ovr, _ := os.ReadFile(filepath.Join(*original, "GAME.OVR"))
		if werr := live.Inv.WriteTSV(*invPath, ovr); werr != nil {
			fmt.Fprintln(os.Stderr, "dkk-play: 盤點寫入失敗：", werr)
		} else {
			fmt.Fprintf(os.Stderr, "dkk-play: 盤點 %d 筆寫到 %s\n", len(live.Inv.Events), *invPath)
		}
	}
	if err != nil {
		fail(err)
	}
}
