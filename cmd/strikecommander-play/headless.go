package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	sc "github.com/wicanr2/dosgolem/apps/strikecommander"
	"github.com/wicanr2/dosgolem/apps/strikecommander/player"
	"github.com/wicanr2/dosgolem/audio/mixer"
	"github.com/wicanr2/dosgolem/session"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
)

func shaFile(p string) (string, error) {
	f, e := os.Open(p)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", e
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func recordingHeader(tree, state string) (player.RecordingHeader, error) {
	h := player.RecordingHeader{Format: "strikecommander-player-turns", Version: 1, StepsPerSecond: player.StepsPerSecond}
	var e error
	h.TreeManifestSHA256, e = shaFile(filepath.Join(filepath.Dir(tree), "tree-v1.manifest"))
	if e != nil {
		return h, e
	}
	if state != "" {
		h.StateSHA256, e = shaFile(state)
	} else {
		h.EXESHA256, e = shaFile(filepath.Join(tree, "SCCD.EXE"))
	}
	return h, e
}
func appPaths() (player.Paths, error) {
	home, e := os.UserHomeDir()
	if e != nil {
		return player.Paths{}, e
	}
	env := map[string]string{}
	for _, k := range []string{"XDG_DATA_HOME", "XDG_CACHE_HOME", "APPDATA", "LOCALAPPDATA", "USERPROFILE"} {
		env[k] = os.Getenv(k)
	}
	return player.PathsFor(runtime.GOOS, env, home)
}
func recordTargetCheck(t player.RecordedTurn, period, visible bool) error {
	point := func(x, y int) (int, int) { return player.DOSPoint(x, y-36, 2) }
	k1 := func(x, y int) bool { x, y = point(x, y); return x >= 90 && x <= 254 && y >= 71 && y <= 96 }
	if period {
		for _, m := range t.Input.PointerMoves {
			if k1(m.X, m.Y) {
				return errors.New("錄製含MENU k1座標")
			}
		}
		if t.Input.PointerDown != nil && k1(t.Input.PointerDown.X, t.Input.PointerDown.Y) {
			return errors.New("錄製含MENU k1按下")
		}
		if t.Input.PointerUp != nil && k1(t.Input.PointerUp.X, t.Input.PointerUp.Y) {
			return errors.New("錄製含MENU k1放開")
		}
	}
	if visible {
		for _, s := range t.Input.ScanEvents {
			if !s.Break {
				return errors.New("錄製在MENU可見時含按下碼")
			}
		}
		if p := t.Input.PointerDown; p != nil {
			x, y := point(p.X, p.Y)
			if x < 89 || x > 254 || y < 33 || y > 58 {
				return errors.New("錄製在MENU可見時按下不在k0")
			}
		}
	}
	return nil
}

func runHeadless(o options) int {
	if e := headless(o); e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	return 0
}
func headless(o options) error {
	if o.uiSmoke != 0 {
		return errors.New("headless不可執行UI smoke")
	}
	if o.replay == "" && o.script == "" && o.steps == 0 {
		return errors.New("headless須給-script、-steps或-replay")
	}
	if o.replay != "" && o.script != "" {
		return errors.New("-script與-replay不可並存")
	}
	paths, e := appPaths()
	if e != nil {
		return e
	}
	exe, _ := os.Executable()
	sources := player.OriginalCandidates(o.original, nil, filepath.Dir(exe), bundleDir(), paths.Original)
	if e = player.CheckIndependentPaths(paths.Cache, paths.Save, sources); e != nil {
		return e
	}
	lock, e := player.Lock(paths.Cache)
	if e != nil {
		return e
	}
	defer lock.Close()
	cache := player.Cache{Root: paths.Cache}
	tree, warnings, e := cache.Ensure(context.Background(), sources, paths.Save, o.reimport)
	if e != nil {
		return e
	}
	w, e := player.CheckSaveShadows(paths.Save, tree, player.RequiredFiles)
	warnings = append(warnings, w...)
	if e != nil {
		return e
	}
	// Headless never loads or saves settings and never imports an audio device.
	settings := player.DefaultSettings(o.hd)
	settings.HD = o.hd
	settings.Subtitles = o.subtitles
	if o.scale != 0 {
		if o.scale != 2 && o.scale != 3 {
			return errors.New("scale只收2或3")
		}
		settings.Scale = o.scale
	}
	if o.lang != "" {
		settings.Lang = o.lang
	}
	if settings.Lang != "en" && settings.Lang != "zh-TW" {
		return errors.New("lang只收en或zh-TW")
	}
	fonts, fe := sc.LoadTextFonts(filepath.Dir(resource("font/zh-TW.cjkb22.aaf")), resource("font/zh-TW.cjkb.expected.tsv"))
	if fe != nil {
		warnings = append(warnings, fe.Error())
		settings.Lang = "en"
	}
	mix := mixer.New(player.StepsPerSecond)
	live, e := openRuntime(tree, paths.Save, mix, fonts, o.state, o.rebase)
	if e != nil {
		return e
	}
	defer live.Close()
	initial, e := live.Owner().Digest()
	if e != nil {
		return e
	}
	header, e := recordingHeader(tree, o.state)
	if e != nil {
		return e
	}
	var writer *player.RecordingWriter
	var recordFile *os.File
	if o.record != "" {
		recordFile, e = os.OpenFile(o.record, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return e
		}
		defer recordFile.Close()
		writer, e = player.NewRecording(recordFile, header)
		if e != nil {
			return e
		}
	}
	var wav *player.WAVWriter
	if o.wav != "" {
		wav, e = player.NewWAV(o.wav)
		if e != nil {
			return e
		}
		defer wav.Close()
	}
	var samples []float32
	backing := make([]byte, 320*3*200*3*4)
	var lastPixels []byte
	var lastAppearance player.Appearance
	turns := 0
	current := initial.Steps
	execute := func(n uint64, u session.CapturedUpdate, appearance player.Appearance) error {
		r, e := player.Turn(live.Owner(), n, u)
		if e != nil {
			return e
		}
		samples = mix.Render(samples[:0], r.Tick.MachineStepsBefore, r.Tick.MachineStepsAfter)
		if wav != nil {
			if e = wav.Append(samples); e != nil {
				return e
			}
		}
		if writer != nil {
			if e = writer.Append(player.RecordedTurn{N: n, Input: r.Update, Appearance: appearance}); e != nil {
				return e
			}
		}
		current = r.Tick.MachineStepsAfter
		if r.Tick.Phase == session.PhaseStopped {
			if e := live.NotifyStopped(); e != nil {
				return e
			}
		}
		turns++
		lastAppearance = appearance
		size := 320 * appearance.Scale * 200 * appearance.Scale * 4
		lastPixels = backing[:size]
		return live.Prepare(appearance.Scale, appearance.Language, appearance.HD, appearance.Subtitles, appearance.Version, lastPixels)
	}
	appearance := player.Appearance{Scale: settings.Scale, Language: settings.Lang, HD: settings.HD, Subtitles: settings.Subtitles, Version: 1}
	if o.replay != "" {
		f, e := os.Open(o.replay)
		if e != nil {
			return e
		}
		defer f.Close()
		_, e = player.ReplayRecordingChecked(f, func(h player.RecordingHeader) error {
			if h != header {
				return errors.New("錄製的起點或快取manifest身分不同")
			}
			return nil
		}, func(t player.RecordedTurn) error {
			period, visible := live.MenuState()
			if e := recordTargetCheck(t, period, visible); e != nil {
				return e
			}
			return execute(t.N, t.Input, t.Appearance)
		})
		if e != nil {
			return e
		}
	} else {
		var script sc.Script
		if o.script != "" {
			b, e := os.ReadFile(o.script)
			if e != nil {
				return e
			}
			script, e = sc.ParseScript(b, current)
			if e != nil {
				return e
			}
		} else {
			if o.steps > ^uint64(0)-current {
				return errors.New("steps溢位")
			}
			script.Stop = current + o.steps
		}
		type event struct {
			steps uint64
			input *sc.ScriptedInput
			shot  *sc.Shot
		}
		var events []event
		for i := range script.Inputs {
			events = append(events, event{steps: script.Inputs[i].Steps, input: &script.Inputs[i]})
		}
		for i := range script.Shots {
			events = append(events, event{steps: script.Shots[i].Steps, shot: &script.Shots[i]})
		}
		events = append(events, event{steps: script.Stop})
		sort.Slice(events, func(i, j int) bool { return events[i].steps < events[j].steps })
		var state player.InputState
		held := map[string]bool{}
		in := player.HostInput{Held: held, MouseX: -1, MouseY: -1}
		menuCleared := false
		step := func(n uint64) error {
			period, visible := live.MenuState()
			if !visible {
				menuCleared = false
			}
			ctx := player.InputContext{MenuPeriod: period, MenuVisible: visible, ForceFocusLost: visible && !menuCleared}
			state.Observe(in, ctx)
			p := state.Build(in, ctx)
			if e := execute(n, p.Update, appearance); e != nil {
				return e
			}
			state.Commit(p)
			if ctx.ForceFocusLost {
				menuCleared = true
			}
			return nil
		}
		for _, ev := range events {
			for current < ev.steps {
				n := min(ev.steps-current, player.BaseSteps)
				before := current
				if e = step(n); e != nil {
					return e
				}
				if current <= before {
					return fmt.Errorf("原版在%d提前停止，目標%d", current, ev.steps)
				}
			}
			if ev.input != nil {
				x := ev.input
				switch x.Kind {
				case "kdown":
					held[player.ScriptHostName(x.Scan)] = true
				case "kup":
					delete(held, player.ScriptHostName(x.Scan))
				case "move":
					in.MouseX, in.MouseY = x.X, x.Y
				case "down":
					in.MouseX, in.MouseY, in.Left = x.X, x.Y, true
				case "up":
					in.MouseX, in.MouseY, in.Left = x.X, x.Y, false
				}
				if e = step(1); e != nil {
					return e
				}
			}
			if ev.shot != nil {
				if e = os.MkdirAll(paths.Screenshots, 0755); e != nil {
					return e
				}
				name := fmt.Sprintf("strikecommander-step-%d-%s.png", current, filepath.Base(ev.shot.Name))
				p := filepath.Join(paths.Screenshots, name)
				f, e := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
				if e != nil {
					return e
				}
				e = png.Encode(f, &image.RGBA{Pix: lastPixels, Stride: 320 * appearance.Scale * 4, Rect: image.Rect(0, 0, 320*appearance.Scale, 200*appearance.Scale)})
				ce := f.Close()
				if e == nil {
					e = ce
				}
				if e != nil {
					return e
				}
			}
		}
	}
	digest, e := live.Owner().Digest()
	if e != nil {
		return e
	}
	machine, e := live.Owner().MachineDigest()
	if e != nil {
		return e
	}
	if wav != nil {
		if e = wav.Close(); e != nil {
			return e
		}
	}
	if recordFile != nil {
		if e = recordFile.Sync(); e != nil {
			return e
		}
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"Mode": "headless", "InitialSteps": initial.Steps, "FinalSteps": current, "Turns": turns, "Digest": digest, "MachineDigest": machine, "Appearance": lastAppearance, "CompositeSHA256": sha256.Sum256(lastPixels), "Warnings": warnings, "Header": header, "Scope": "frontend Turn/runtime; normal GUI/platform acceptance remains separate"})
}
