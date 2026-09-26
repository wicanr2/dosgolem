package buckrogers

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/xlate"
)

// liveScales are the output scales every live family renders at.
var liveScales = [2]int{2, 3}

var dispatchPrompt = Address{Segment: 0x37F1, Offset: 0x101E}

// LiveRuntime is the Buck Rogers overlay observer for a live session.  It
// receives the same per-step, video-write, and frame events as the receipt
// runner, dispatches them to each family in the runner's order, and keeps the
// last retraced frame so the host can compose it at either output scale.
//
// Receipts fail closed; a live game must not stop because an overlay family
// lost track.  A family that errors is cleared (the original English shows)
// and rebuilt from its catalog, and the reset is counted.
type LiveRuntime struct {
	menu     *LiveMenuRuntime
	skill    [2]*SkillExitOwner
	exit     [2]*PostJoinExitPromptOwner
	postJoin *PostJoinMenuWatcher
	postPres [2]*RuntimePostJoinMenuOverlay

	font         *xlate.Font
	skillCatalog *SkillExitCatalog
	exitCatalog  *PostJoinExitPromptCatalog
	postCatalog  *PostJoinMenuCatalog

	indexed   []byte
	palette   [256][3]uint8
	hasFrame  bool
	frameSeen uint64
	resets    map[string]int
}

// LoadLiveRuntime builds every live family from a Buck Rogers text directory
// and a local 16×16 GOLEMFNT.
func LoadLiveRuntime(textDir, fontPath string) (*LiveRuntime, error) {
	menu, err := LoadLiveMenuRuntime(textDir, fontPath)
	if err != nil {
		return nil, err
	}
	font, err := xlate.LoadFont(fontPath)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	for _, name := range []string{
		"career-skill-exit-events.tsv", "technical-skill-exit-events.tsv", "skill-exit-confirmation.zh-TW.tsv",
		"post-join-exit-prompt-events.tsv", "post-join-exit-prompt.zh-TW.tsv",
		"post-join-menu-events.tsv", "post-join-menu-variants.tsv", "post-join-menu.zh-TW.tsv",
	} {
		b, err := os.ReadFile(filepath.Join(textDir, name))
		if err != nil {
			return nil, fmt.Errorf("buckrogers: 讀取 %s：%w", name, err)
		}
		files[name] = b
	}
	r := &LiveRuntime{menu: menu, font: font, resets: map[string]int{}}
	if r.skillCatalog, err = LoadSkillExitCatalog(files["career-skill-exit-events.tsv"], files["technical-skill-exit-events.tsv"], files["skill-exit-confirmation.zh-TW.tsv"]); err != nil {
		return nil, err
	}
	if r.exitCatalog, err = LoadPostJoinExitPromptCatalog(files["post-join-exit-prompt-events.tsv"], files["post-join-exit-prompt.zh-TW.tsv"]); err != nil {
		return nil, err
	}
	if r.postCatalog, err = LoadPostJoinMenuCatalog(files["post-join-menu-events.tsv"], files["post-join-menu-variants.tsv"], files["post-join-menu.zh-TW.tsv"]); err != nil {
		return nil, err
	}
	for i := range liveScales {
		if err := r.resetSkill(i); err != nil {
			return nil, err
		}
		if err := r.resetExit(i); err != nil {
			return nil, err
		}
	}
	if err := r.resetPostJoin(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *LiveRuntime) resetSkill(i int) error {
	o, err := NewSkillExitOwner(r.skillCatalog, r.font, liveScales[i])
	if err == nil {
		r.skill[i] = o
	}
	return err
}

func (r *LiveRuntime) resetExit(i int) error {
	o, err := NewPostJoinExitPromptOwner(r.exitCatalog, r.font, liveScales[i])
	if err == nil {
		r.exit[i] = o
	}
	return err
}

func (r *LiveRuntime) resetPostJoin() error {
	w, err := NewPostJoinMenuWatcher(r.postCatalog)
	if err != nil {
		return err
	}
	var pres [2]*RuntimePostJoinMenuOverlay
	for i, scale := range liveScales {
		if pres[i], err = NewRuntimePostJoinMenuOverlay(r.postCatalog, r.font, scale); err != nil {
			return err
		}
	}
	r.postJoin, r.postPres = w, pres
	return nil
}

// recover rebuilds a family after an error.  A rebuild failure is a
// programming error in catalog loading and is returned.
func (r *LiveRuntime) recover(family string, rebuild func() error) error {
	r.resets[family]++
	return rebuild()
}

// Resets reports how many times each family was rebuilt after an error.
func (r *LiveRuntime) Resets() map[string]int {
	out := make(map[string]int, len(r.resets))
	for k, v := range r.resets {
		out[k] = v
	}
	return out
}

func textEvent(steps uint64, caller Address, args [6]uint16, original []byte) TextEvent {
	return TextEvent{EntryStep: steps, Caller: caller, OriginalLength: uint8(len(original)), OriginalSHA256: sha256.Sum256(original),
		Background: uint8(args[2]), Foreground: uint8(args[3]), Row: uint8(args[4]), Column: uint8(args[5])}
}

func postJoinEntryRow(caller Address, row uint8) bool {
	known := row == 13 || row == 14 || row == 15 || row == 16 || row == 18 || row == 19 || row == 20
	return caller.Segment == 0x37f1 && (known || (caller.Offset == 0x175d && row == 21)) &&
		(caller.Offset == 0x15bd || caller.Offset == 0x175d || caller.Offset == 0x1856)
}

// BeforeStep dispatches one instruction in the receipt runner's order.
func (r *LiveRuntime) BeforeStep(v StepReader) error {
	obs, err := r.menu.Observe(v)
	if err != nil {
		// The menu family owns the shared recorder; rebuilding it is not
		// possible without losing the other families' frames, so this stays
		// fatal (it has never been observed on a normal path).
		return err
	}
	switch obs.Kind {
	case ObservedEntry:
		steps := v.Steps()
		for i := range liveScales {
			if r.skill[i].Watcher.Pending() && obs.Dropped {
				if err := r.recover("skill-exit", func() error { return r.resetSkill(i) }); err != nil {
					return err
				}
			}
			if r.exit[i].Watcher.Pending() && obs.Dropped {
				if err := r.recover("exit-prompt", func() error { return r.resetExit(i) }); err != nil {
					return err
				}
			}
		}
		if obs.Caller == dispatchPrompt {
			e := textEvent(steps, obs.Caller, obs.Args, obs.Original)
			for i := range liveScales {
				if err := r.skill[i].ObserveEntry(e); err != nil {
					if err := r.recover("skill-exit", func() error { return r.resetSkill(i) }); err != nil {
						return err
					}
				}
			}
			if uint8(obs.Args[4]) == 24 && uint8(obs.Args[5]) == 0 {
				for i := range liveScales {
					if r.exit[i].Watcher.ShouldObserveEntry(e) {
						if err := r.exit[i].ObserveEntry(e); err != nil {
							if err := r.recover("exit-prompt", func() error { return r.resetExit(i) }); err != nil {
								return err
							}
						}
					}
				}
			}
		}
		if postJoinEntryRow(obs.Caller, uint8(obs.Args[4])) {
			if err := r.postJoin.ObserveEntry(textEvent(steps, obs.Caller, obs.Args, obs.Original)); err != nil {
				if err := r.recover("post-join", r.resetPostJoin); err != nil {
					return err
				}
			}
		}
	case ObservedOther:
		palette := [256][3]uint8{}
		if obs.NewEvent {
			palette = v.Palette()
		}
		for i := range liveScales {
			if r.skill[i].Watcher.Pending() && obs.Dropped {
				if err := r.recover("skill-exit", func() error { return r.resetSkill(i) }); err != nil {
					return err
				}
				continue
			}
			if r.skill[i].Watcher.Pending() && obs.NewEvent {
				if err := r.skill[i].ObserveReturn(obs.Event, palette); err != nil {
					if err := r.recover("skill-exit", func() error { return r.resetSkill(i) }); err != nil {
						return err
					}
				} else if i == 0 {
					r.yieldMenu(r.skill[0].Presenter.LayerRects())
				}
			}
		}
		for i := range liveScales {
			if r.exit[i].Watcher.Pending() && obs.Dropped {
				if err := r.recover("exit-prompt", func() error { return r.resetExit(i) }); err != nil {
					return err
				}
				continue
			}
			if r.exit[i].Watcher.Pending() && obs.NewEvent {
				if err := r.exit[i].ObserveReturn(obs.Event, palette); err != nil {
					if err := r.recover("exit-prompt", func() error { return r.resetExit(i) }); err != nil {
						return err
					}
				} else if i == 0 {
					r.yieldMenu(r.exit[0].Presenter.LayerRects())
				}
			}
		}
		if obs.NewEvent && postJoinRuntimeGate(obs.Event) {
			prior := len(r.postJoin.Generations())
			if err := r.postJoin.ObserveReturn(obs.Event); err != nil {
				return r.recover("post-join", r.resetPostJoin)
			}
			for _, g := range r.postJoin.Generations()[prior:] {
				for i := range liveScales {
					if err := r.postPres[i].Apply(g, palette); err != nil {
						return r.recover("post-join", r.resetPostJoin)
					}
				}
				r.yieldMenu(r.postPres[0].LayerRects())
			}
		}
	}
	return nil
}

// yieldMenu lets the newest family writer win: the original just drew these
// rectangles, so any menu stamp there is already stale.
func (r *LiveRuntime) yieldMenu(rects []PixelRect) {
	for _, rect := range rects {
		r.menu.ClearRect(rect)
	}
}

func postJoinRuntimeGate(e TextEvent) bool { return postJoinEntryRow(e.Caller, e.Row) }

// VideoWrite forwards an A000 pre-write to every family that invalidates on
// writes, in the runner's order.
func (r *LiveRuntime) VideoWrite(w machine.VideoWrite) {
	if r.postJoin.Active() {
		r.postJoin.Prewrite(w)
		for i := range liveScales {
			r.postPres[i].Prewrite(w)
		}
	}
	for i := range liveScales {
		if err := r.skill[i].Prewrite(w); err != nil {
			_ = r.recover("skill-exit", func() error { return r.resetSkill(i) })
		}
		if err := r.exit[i].Prewrite(w); err != nil {
			_ = r.recover("exit-prompt", func() error { return r.resetExit(i) })
		}
	}
}

func (r *LiveRuntime) Frame(indexed []byte, palette [256][3]uint8) {
	r.menu.Frame(indexed, palette)
	r.indexed, r.palette, r.hasFrame = indexed, palette, true
	r.frameSeen++
}

// Compose returns the last retraced frame with every active family's overlay
// at scale, or false before the first retrace.  Families draw into disjoint
// safe rectangles, so each family's pixels that differ from the baseline are
// layered onto the menu family's output.
func (r *LiveRuntime) Compose(scale int) ([]byte, bool, error) {
	if !r.hasFrame {
		return nil, false, nil
	}
	i := 0
	if scale == 3 {
		i = 1
	} else if scale != 2 {
		return nil, false, fmt.Errorf("buckrogers: 不支援 %d×", scale)
	}
	out, missing, _, err := r.menu.Draw(r.indexed, r.palette, scale)
	if err != nil {
		return nil, false, err
	}
	if len(missing) != 0 {
		return nil, false, errors.New("buckrogers: live runtime 選單缺字")
	}
	var base []byte
	layer := func(rgba []byte, missing []rune) error {
		if len(missing) != 0 {
			return errors.New("buckrogers: live runtime 缺字")
		}
		if base == nil {
			base = ScaleIndexedRGBA(r.indexed, r.palette, scale)
		}
		for p := 0; p < len(base); p += 4 {
			if rgba[p] != base[p] || rgba[p+1] != base[p+1] || rgba[p+2] != base[p+2] || rgba[p+3] != base[p+3] {
				copy(out[p:p+4], rgba[p:p+4])
			}
		}
		return nil
	}
	if len(r.skill[i].Presenter.ActiveKeys()) != 0 {
		rgba, missing, _ := r.skill[i].Presenter.Draw(r.indexed, r.palette)
		if err := layer(rgba, missing); err != nil {
			return nil, false, err
		}
	}
	if len(r.exit[i].Presenter.ActiveKeys()) != 0 {
		rgba, missing, _, err := r.exit[i].Presenter.Draw(r.indexed, r.palette)
		if err != nil {
			return nil, false, err
		}
		if err := layer(rgba, missing); err != nil {
			return nil, false, err
		}
	}
	if len(r.postPres[i].ActiveKeys()) != 0 {
		rgba, missing, _ := r.postPres[i].Draw(r.indexed, r.palette)
		if err := layer(rgba, missing); err != nil {
			return nil, false, err
		}
	}
	return out, true, nil
}

// Frames counts retraces observed so far.
func (r *LiveRuntime) Frames() uint64 { return r.frameSeen }
