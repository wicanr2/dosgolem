//go:build !headless

// Command strikecommander-play is the 016 window shell. The player package
// owns import, input, scheduling and saves; no guest memory is written here.
package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	sc "github.com/wicanr2/dosgolem/apps/strikecommander"
	"github.com/wicanr2/dosgolem/apps/strikecommander/player"
	"github.com/wicanr2/dosgolem/audio/mixer"
	"github.com/wicanr2/dosgolem/session"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type importResult struct {
	usedSources []string
	tree        string
	warnings    []string
	err         error
}
type glyphKey struct {
	px int
	r  rune
}
type game struct {
	frameBacking                                        []byte
	capsKnown                                           bool
	pendingFocusLost                                    bool
	frameScale                                          int
	glyphs                                              map[glyphKey]*ebiten.Image
	virtual                                             []player.VirtualSource
	worker                                              sync.WaitGroup
	opts                                                options
	paths                                               player.Paths
	settings, saved                                     player.Settings
	ui                                                  player.UIText
	fonts                                               *sc.TextFonts
	lock                                                *player.ProcessLock
	state                                               string
	status, errorText                                   string
	sources                                             []string
	progress                                            chan string
	imported                                            chan importResult
	cancel                                              context.CancelFunc
	live                                                runtimeGame
	input                                               player.InputState
	pace                                                player.Pace
	mix                                                 *mixer.Mixer
	ring                                                *mixer.Ring
	audioPlayer                                         *audio.Player
	samples                                             []float32
	help, confirm, focused, paused, resume, menuCleared bool
	updates                                             int
	last                                                time.Time
	version                                             uint64
	rgba                                                []byte
	texture                                             *ebiten.Image
	shot                                                bool
	lastCaps                                            bool
	warnings                                            []string
}

func newGame(o options) (*game, error) {
	home, e := os.UserHomeDir()
	if e != nil {
		return nil, e
	}
	env := map[string]string{}
	for _, k := range []string{"XDG_DATA_HOME", "XDG_CACHE_HOME", "APPDATA", "LOCALAPPDATA", "USERPROFILE"} {
		env[k] = os.Getenv(k)
	}
	paths, e := player.PathsFor(runtime.GOOS, env, home)
	if e != nil {
		return nil, e
	}
	saved, scaleSet, se := player.LoadSettings(paths.Settings, false)
	if se != nil {
		fmt.Fprintln(os.Stderr, se)
	}
	s := saved
	if o.scale != 0 {
		if o.scale != 2 && o.scale != 3 {
			return nil, errors.New("-scale只收2或3")
		}
		s.Scale = o.scale
		scaleSet = true
	}
	if o.lang != "" {
		if o.lang != "en" && o.lang != "zh-TW" {
			return nil, errors.New("-lang只收en或zh-TW")
		}
		s.Lang = o.lang
	}
	if o.explicit["hd"] {
		s.HD = o.hd
	}
	if o.explicit["subtitles"] {
		s.Subtitles = o.subtitles
	}
	if o.explicit["fullscreen"] {
		s.Fullscreen = o.fullscreen
	}
	if o.explicit["muted"] {
		s.Muted = o.muted
	}
	if o.explicit["pause-on-blur"] {
		s.PauseOnBlur = o.pauseOnBlur
	}
	w, h := ebiten.Monitor().Size()
	s = player.InitialDisplay(w, h, scaleSet, s)
	g := &game{opts: o, paths: paths, settings: s, saved: saved, state: "guide", focused: true, resume: true, version: 1, progress: make(chan string, 1), imported: make(chan importResult, 1), mix: mixer.New(player.StepsPerSecond)}
	g.fonts, e = sc.LoadTextFonts(filepath.Dir(resource("font/zh-TW.cjkb22.aaf")), resource("font/zh-TW.cjkb.expected.tsv"))
	if e != nil {
		g.settings.Lang = "en"
		g.ui.Reason = e.Error()
	} else {
		font := g.fonts.Font(22)
		g.ui = player.LoadUIText(resource("text/frontend.zh-TW.tsv"), func(c rune) bool {
			for _, r := range sc.NarrowAlnum([]rune{c}) {
				if font.Glyphs[r] == nil {
					return false
				}
			}
			return true
		})
		if g.ui.Reason != "" {
			g.settings.Lang = "en"
		}
	}
	exe, _ := os.Executable()
	if o.uiSmoke > 0 {
		fmt.Fprintf(os.Stderr, "ui_smoke language=%s formal_fonts=%t reason=%q\n", g.settings.Lang, g.fonts != nil, g.ui.Reason)
	}
	g.sources = player.OriginalCandidates(o.original, saved.Original, filepath.Dir(exe), bundleDir(), paths.Original)
	// This must precede Lock/MkdirAll: an invalid nesting must never write sources.
	if e = player.CheckIndependentPaths(paths.Cache, paths.Save, g.sources); e != nil {
		g.state = "error"
		g.errorText = e.Error()
		return g, nil
	}
	g.lock, e = player.Lock(paths.Cache)
	if e != nil {
		g.state = "error"
		g.errorText = e.Error()
		return g, nil
	}
	if o.uiSmoke == 0 {
		g.startImport()
	}
	return g, nil
}
func (g *game) logError(e error) {
	if e == nil {
		return
	}
	fmt.Fprintln(os.Stderr, e)
	if er := os.MkdirAll(g.paths.Data, 0755); er == nil {
		f, er := os.OpenFile(g.paths.ErrorLog, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if er == nil {
			fmt.Fprintf(f, "%s\n%s\n", time.Now().Format(time.RFC3339), e)
			f.Close()
		}
	}
}
func (g *game) startImport() {
	if g.state == "loading" || g.live != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	g.cancel = cancel
	g.state = "loading"
	g.status = g.ui.Text("source.progress")
	sources := append([]string(nil), g.sources...)
	g.worker.Add(1)
	virtual := append([]player.VirtualSource(nil), g.virtual...)
	go func() {
		defer g.worker.Done()
		c := player.Cache{Root: g.paths.Cache, Progress: func(s string) {
			select {
			case g.progress <- s:
			default:
			}
		}, Warn: func(s string) { fmt.Fprintln(os.Stderr, s) }}
		if len(virtual) > 0 {
			c.Resolve = func(ctx context.Context) (player.Selection, error) {
				return player.LocateWithVirtual(ctx, sources, virtual, player.RequiredFiles, c.Progress)
			}
		}
		tree, w, e := c.Ensure(ctx, sources, g.paths.Save, g.opts.reimport)
		used, _ := c.UsedSources()
		g.imported <- importResult{tree: tree, warnings: w, err: e, usedSources: used}
	}()
}
func (g *game) drop(fsys fs.FS) {
	if fsys == nil || g.state == "loading" || g.live != nil || g.lock == nil {
		return
	}
	entries, e := fs.ReadDir(fsys, ".")
	if e != nil {
		g.logError(e)
		return
	}
	var names []string
	for _, x := range entries {
		names = append(names, x.Name())
	}
	if e = player.ValidateDropNames(names); e != nil {
		g.errorText = e.Error()
		return
	}
	native := append([]string(nil), g.sources...)
	virtual := append([]player.VirtualSource(nil), g.virtual...)
	for _, n := range names {
		f, e := fsys.Open(n)
		if e != nil {
			g.errorText = e.Error()
			return
		}
		named, ok := f.(interface{ Name() string })
		if ok && filepath.IsAbs(named.Name()) {
			native = append(native, named.Name())
		} else {
			virtual = append(virtual, player.VirtualSource{FS: fsys, Root: n, Label: "dropped:" + n})
		}
		f.Close()
	}
	if e = player.CheckIndependentPaths(g.paths.Cache, g.paths.Save, native); e != nil {
		g.errorText = e.Error()
		return
	}
	g.sources = native
	g.virtual = virtual
	g.startImport()
}

func (g *game) Close() {
	if g.cancel != nil {
		g.cancel()
	}
	g.worker.Wait()
	if g.live != nil {
		if e := g.live.Close(); e != nil {
			g.logError(e)
		}
	}
	if g.audioPlayer != nil {
		g.audioPlayer.Close()
	}
	if g.lock != nil {
		g.lock.Close()
	}
	fmt.Fprintf(os.Stderr, "realtime=%.6f slow_seconds=%.6f", g.pace.Ratio(), g.pace.SlowSeconds())
	if g.ring != nil {
		u, d := g.ring.Stats()
		fmt.Fprintf(os.Stderr, " underruns=%d dropped_frames=%d", u, d)
	}
	fmt.Fprintln(os.Stderr)
}
func (g *game) save() {
	if e := player.SaveSettings(g.paths.Settings, g.saved); e != nil {
		g.logError(e)
	}
}
func (g *game) change(key ebiten.Key) {
	switch key {
	case ebiten.KeyDigit2:
		g.settings.Scale = 2
		g.saved.Scale = 2
	case ebiten.KeyDigit3:
		g.settings.Scale = 3
		g.saved.Scale = 3
	case ebiten.KeyL:
		if g.settings.Lang == "en" && g.ui.Reason == "" {
			g.settings.Lang = "zh-TW"
		} else {
			g.settings.Lang = "en"
		}
		g.saved.Lang = g.settings.Lang
	case ebiten.KeyT:
		g.settings.Subtitles = !g.settings.Subtitles
		g.saved.Subtitles = g.settings.Subtitles
	case ebiten.KeyH:
		g.settings.HD = !g.settings.HD
		g.saved.HD = g.settings.HD
	case ebiten.KeyF:
		g.settings.Fullscreen = !g.settings.Fullscreen
		g.saved.Fullscreen = g.settings.Fullscreen
	case ebiten.KeyM:
		g.settings.Muted = !g.settings.Muted
		g.saved.Muted = g.settings.Muted
	case ebiten.KeyP:
		g.shot = true
	case ebiten.KeyQ:
		g.confirm = true
	default:
		return
	}
	g.version++
	g.save()
	ebiten.SetFullscreen(g.settings.Fullscreen)
	if !g.settings.Fullscreen {
		ebiten.SetWindowSize(320*g.settings.Scale, 200*g.settings.Scale)
	}
	if g.audioPlayer != nil {
		volume := 1.0
		if g.settings.Muted {
			volume = 0
		}
		g.audioPlayer.SetVolume(volume)
	}
}
func hostInput(scale int) player.HostInput {
	h := map[string]bool{}
	for k := ebiten.Key(0); k <= ebiten.KeyMax; k++ {
		name := k.String()
		if strings.HasPrefix(name, "Digit") {
			name = strings.TrimPrefix(name, "Digit")
		}
		if _, ok := player.ScanCodes[name]; ok || name == "F11" || name == "F12" || name == "MetaLeft" || name == "MetaRight" {
			h[name] = ebiten.IsKeyPressed(k)
		}
	}
	x, y := ebiten.CursorPosition()
	x, y = player.DOSPoint(x, y, scale)
	return player.HostInput{Held: h, MouseX: x, MouseY: y, Left: ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft), Right: ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight)}
}
func (g *game) Update() error {
	g.updates++
	now := time.Now()
	dt := now.Sub(g.last)
	g.last = now
	if g.opts.uiSmoke > 0 && g.updates >= g.opts.uiSmoke {
		return ebiten.Termination
	}
	if ebiten.IsWindowBeingClosed() {
		if g.live == nil {
			return ebiten.Termination
		}
		g.help = true
		g.confirm = true
	}
	for {
		select {
		case s := <-g.progress:
			g.status = s
		default:
			goto doneProgress
		}
	}
doneProgress:
	select {
	case r := <-g.imported:
		if r.err != nil {
			g.state = "guide"
			g.errorText = r.err.Error()
			g.logError(r.err)
		} else {
			g.warnings = r.warnings
			if len(g.opts.original) == 0 {
				g.saved.Original = nil
				for _, p := range r.usedSources {
					if filepath.IsAbs(p) {
						g.saved.Original = append(g.saved.Original, p)
					}
				}
				g.save()
			}
			w, e := player.CheckSaveShadows(g.paths.Save, r.tree, player.RequiredFiles)
			g.warnings = append(g.warnings, w...)
			if e == nil {
				g.live, e = openRuntime(r.tree, g.paths.Save, g.mix, g.fonts, "", nil)
			}
			if e != nil {
				g.state = "error"
				g.errorText = e.Error()
				g.logError(e)
			} else {
				g.state = "game"
				g.resume = true

				os.Remove(g.paths.ErrorLog)
				g.startAudio()
			}
		}
	default:
	}
	if g.live == nil {
		g.drop(ebiten.DroppedFiles())
		ebiten.SetCursorMode(ebiten.CursorModeVisible)
		return nil
	}
	oldHelp := g.help
	keys := inpututil.AppendJustPressedKeys(nil)
	if g.confirm {
		if len(keys) > 0 {
			if inpututil.IsKeyJustPressed(ebiten.KeyY) {
				return ebiten.Termination
			}
			g.confirm = false
		}
	} else {
		if inpututil.IsKeyJustPressed(ebiten.KeyF11) || (runtime.GOOS == "darwin" && (ebiten.IsKeyPressed(ebiten.KeyMetaLeft) || ebiten.IsKeyPressed(ebiten.KeyMetaRight)) && inpututil.IsKeyJustPressed(ebiten.KeySlash)) {
			g.help = !g.help
		}
		if g.help {
			if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
				g.help = false
			}
			for _, k := range keys {
				g.change(k)
			}
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF12) {
		g.shot = true
	}
	focus := true
	if g.updates > 1 {
		focus = ebiten.IsFocused()
	}
	lost := g.focused && !focus
	if lost {
		g.pendingFocusLost = true
	}
	g.focused = focus
	pause := g.help || (!focus && g.settings.PauseOnBlur)
	resumed := g.paused && !pause || g.resume
	g.paused = pause
	in := hostInput(g.settings.Scale)
	if runtime.GOOS == "darwin" {
		caps := ebiten.IsKeyPressed(ebiten.KeyCapsLock)
		in.CapsToggle = g.capsKnown && caps != g.lastCaps
		g.capsKnown = true
		g.lastCaps = caps
	}
	period, visible := g.live.MenuState()
	if !visible {
		g.menuCleared = false
	}
	ctx := player.InputContext{Suppress: oldHelp || g.help || !focus, MenuPeriod: period, MenuVisible: visible, Resume: resumed, ForceFocusLost: g.pendingFocusLost || visible && !g.menuCleared, Mac: runtime.GOOS == "darwin"}
	g.input.Observe(in, ctx)
	if g.help || (!focus && g.settings.PauseOnBlur && !lost) {
		ebiten.SetCursorMode(ebiten.CursorModeVisible)
		g.resume = true
		if g.shot {
			if e := g.prepare(); e != nil {
				return e
			}
			g.shot = false
			g.screenshot()
		}
		return nil
	}
	step := g.pace.Step(dt, resumed)
	n := step.N
	if ctx.ForceFocusLost && n == 0 {
		n = 1
	}
	if n > 0 {
		proposed := g.input.Build(in, ctx)
		r, e := player.Turn(g.live.Owner(), n, proposed.Update)
		if e != nil {
			return e
		}
		g.input.Commit(proposed)
		if proposed.Update.FocusLost {
			g.pendingFocusLost = false
		}
		if ctx.ForceFocusLost && visible {
			g.menuCleared = true
		}
		g.samples = g.mix.Render(g.samples[:0], r.Tick.MachineStepsBefore, r.Tick.MachineStepsAfter)
		if g.ring != nil {
			g.ring.Push(g.samples)
		}
		g.pace.Record(step.DT, r.Tick.Steps)
		if r.Tick.Phase == session.PhaseStopped {
			if e := g.live.NotifyStopped(); e != nil {
				return e
			}
			return ebiten.Termination
		}
	}
	g.resume = false
	if e := g.prepare(); e != nil {
		return e
	}
	ebiten.SetCursorMode(ebiten.CursorModeHidden)
	if g.shot {
		g.shot = false
		g.screenshot()
	}
	return nil
}
func (g *game) prepare() error {
	if g.frameBacking == nil {
		g.frameBacking = make([]byte, 320*3*200*3*4)
	}
	size := 320 * g.settings.Scale * 200 * g.settings.Scale * 4
	g.rgba = g.frameBacking[:size]
	if e := g.live.Prepare(g.settings.Scale, g.settings.Lang, g.settings.HD, g.settings.Subtitles, g.version, g.rgba); e != nil {
		return e
	}
	g.frameScale = g.settings.Scale
	return nil
}
func (g *game) startAudio() {
	g.ring = mixer.NewRing(200)
	ctx := audio.NewContext(mixer.SampleRate)
	p, e := ctx.NewPlayerF32(g.ring)
	if e != nil {
		g.logError(fmt.Errorf("沒有音訊輸出：%w", e))
		g.warnings = append(g.warnings, g.ui.Text("audio.none"))
		g.ring = nil
		return
	}
	g.audioPlayer = p
	volume := 1.0
	if g.settings.Muted {
		volume = 0
	}
	p.SetVolume(volume)
	p.SetBufferSize(50 * time.Millisecond)
	p.Play()
}
func (g *game) screenshot() {
	if len(g.rgba) == 0 {
		return
	}
	now := time.Now()
	p := filepath.Join(g.paths.Screenshots, fmt.Sprintf("strikecommander-%s-%03d.png", now.Format("20060102-150405"), now.Nanosecond()/1000000))
	if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
		ebiten.SetWindowTitle(e.Error())
		return
	}
	f, e := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e == nil {
		e = png.Encode(f, &image.RGBA{Pix: g.rgba, Stride: 320 * g.frameScale * 4, Rect: image.Rect(0, 0, 320*g.frameScale, 200*g.frameScale)})
		ce := f.Close()
		if e == nil {
			e = ce
		}
	}
	if e != nil {
		ebiten.SetWindowTitle(e.Error())
	}
}
func (g *game) Layout(w, h int) (int, int) { return 320 * g.settings.Scale, 200 * g.settings.Scale }
func (g *game) Draw(dst *ebiten.Image) {
	w, h := g.Layout(0, 0)
	if g.live != nil && !g.help {
		if len(g.rgba) != w*h*4 {
			return
		}
		if g.texture == nil || g.texture.Bounds().Dx() != w {
			if g.texture != nil {
				g.texture.Deallocate()
			}
			g.texture = ebiten.NewImage(w, h)
		}
		g.texture.WritePixels(g.rgba)
		dst.DrawImage(g.texture, nil)
		return
	}
	dst.Fill(color.RGBA{12, 18, 26, 255})
	lines := []string{}
	if g.help {
		if g.confirm {
			lines = append(lines, g.ui.Text("quit.confirm"), g.ui.Text("quit.keys"))
		} else {
			lines = append(lines, g.ui.Text("help.title"), fmt.Sprintf("%s %dx   %s %s   %s %.3f", g.ui.Text("label.scale"), g.settings.Scale, g.ui.Text("label.language"), g.settings.Lang, g.ui.Text("label.rate"), g.pace.Ratio()), fmt.Sprintf("%s %t   HD %t   %s %t", g.ui.Text("label.subtitles"), g.settings.Subtitles, g.settings.HD, g.ui.Text("label.muted"), g.settings.Muted), g.ui.Text("help.keys"), g.ui.Text("help.keys2"), g.ui.Text("help.close"), g.ui.Text("help.menu"), g.ui.Text("help.keyboard"), g.ui.Text("help.mac"), g.ui.Text("help.save")+g.paths.Save, g.ui.Text("help.save_warning"))
			lines = append(lines, g.warnings...)
		}
	} else {
		title := "source.title"
		if g.state == "error" {
			title = "error.title"
		}
		lines = append(lines, g.ui.Text(title), g.ui.Text("source.guide"), g.ui.Text("source.paths"), g.ui.Text("source.examples"), g.ui.Text("source.windows"), g.ui.Text("source.mac"), g.status, g.errorText, g.ui.Text("error.close"))
	}
	if g.ui.Reason != "" {
		lines = append(lines, "English UI fallback: "+g.ui.Reason)
	}
	g.drawText(dst, lines, w, h)
}

// UI text uses immutable AAF glyphs and small per-glyph resources, not a third
// full-scene CPU framebuffer. Old glyph resources are retired after each page.
func (g *game) drawText(dst *ebiten.Image, lines []string, w, h int) {
	font := g.fonts.Font(22)
	if g.settings.Scale == 3 {
		font = g.fonts.Font(32)
	}
	if font == nil {
		var escaped []string
		for _, line := range lines {
			q := strconv.QuoteToASCII(line)
			escaped = append(escaped, q[1:len(q)-1])
		}
		ebitenutil.DebugPrintAt(dst, strings.Join(escaped, "\n\n"), 12, 12)
		return
	}
	if g.glyphs == nil {
		g.glyphs = map[glyphKey]*ebiten.Image{}
	}
	used := map[glyphKey]bool{}
	y := font.Ascent + 12
	for _, line := range lines {
		if y >= h {
			break
		}
		x := 12
		rs := sc.NarrowAlnum([]rune(line))
		var printable []rune
		for _, r := range rs {
			if font.Glyphs[r] != nil {
				printable = append(printable, r)
			} else {
				q := strconv.QuoteToASCII(string(r))
				printable = append(printable, []rune(q[1:len(q)-1])...)
			}
		}
		for _, r := range printable {
			glyph := font.Glyphs[r]
			if glyph == nil {
				continue
			}
			if x+glyph.Adv > w-12 {
				x = 12
				y += font.Px + 6
			}
			if y >= h {
				break
			}
			key := glyphKey{font.Px, r}
			img := g.glyphs[key]
			if glyph.FW > 0 && glyph.FH > 0 {
				if img == nil {
					pix := make([]byte, glyph.FW*glyph.FH*4)
					for i, a := range glyph.Fill {
						v := uint8(uint16(a) * 235 / 255)
						pix[4*i] = v
						pix[4*i+1] = v
						pix[4*i+2] = v
						pix[4*i+3] = a
					}
					img = ebiten.NewImage(glyph.FW, glyph.FH)
					img.WritePixels(pix)
					g.glyphs[key] = img
				}
				used[key] = true
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Translate(float64(x+glyph.FX), float64(y+glyph.FY))
				dst.DrawImage(img, op)
			}
			x += glyph.Adv
		}
		y += font.Px + 6
	}
	for k, img := range g.glyphs {
		if !used[k] {
			img.Deallocate()
			delete(g.glyphs, k)
		}
	}
}

func (g *game) DrawFinalScreen(dst ebiten.FinalScreen, src *ebiten.Image, geo ebiten.GeoM) {
	op := &ebiten.DrawImageOptions{GeoM: geo, Filter: ebiten.FilterNearest}
	if g.settings.Fullscreen {
		op.Filter = ebiten.FilterLinear
	}
	dst.DrawImage(src, op)
}
func main() { os.Exit(run()) }
func run() int {
	o := parse()
	if o.headless {
		return runHeadless(o)
	}
	ebiten.SetWindowClosingHandled(true)
	ebiten.SetTPS(ebiten.SyncWithFPS)
	ebiten.SetVsyncEnabled(true)
	ebiten.SetRunnableOnUnfocused(true)
	g, e := newGame(o)
	if e != nil {
		log.Print(e)
		return 1
	}
	defer g.Close()
	ebiten.SetWindowTitle("陸空戰將")
	ebiten.SetWindowSize(320*g.settings.Scale, 200*g.settings.Scale)
	ebiten.SetFullscreen(g.settings.Fullscreen)
	if e = ebiten.RunGame(g); e != nil {
		g.logError(e)
		return 1
	}

	return 0
}
