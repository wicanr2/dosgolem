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
	action   *ActionBarWatcher
	actPres  [2]*RuntimeActionBarOverlay
	stories  []storyFamily
	body     *LiveBodyIconWatcher
	bodyPres [2]*RuntimeBodyIconOverlay
	manual   *Watcher
	manPres  [2]*RuntimeManualOverlay
	manSync  [2]*ManualPresentationBridge
	manSeen  int
	manStyle ManualTextStyle
	manHas   bool

	// storyPending is the shared in-flight 0763:026B glyph call; prev* is
	// the instruction before the current one, recorded while it is set.
	storyPending *storyFrame
	prevAt       Address
	prevOp       byte
	storyDirty   bool

	// ecl is the spec-027 generic text-window family; nil without catalog.
	ecl     *EclTextWatcher
	eclPres [2]*EclTextOverlay
	eclGen  uint64

	// hmenu is the spec-028 horizontal-menu family; nil without catalog.
	hmenu     *HMenuWatcher
	hmenuPres [2]*HMenuOverlay
	hmenuGen  uint64

	// engDisp is the spec-029 dispatcher path; nil without allow list.
	engDisp     *EngineDispatchWatcher
	engDispPres [2]*HMenuOverlay
	engDispGen  uint64

	// logbook is the spec-030 panel; nil without text/logbook.zh-TW.tsv.
	logbook     *LogbookWatcher
	logbookPres [2]*LogbookOverlay

	font         *xlate.Font
	skillCatalog *SkillExitCatalog
	exitCatalog  *PostJoinExitPromptCatalog
	postCatalog  *PostJoinMenuCatalog
	actCatalog   *ActionBarRequestCatalog
	actRects     *MenuOverlayRects

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
		"skill-action-bar-events.tsv", "skill-action-bar.zh-TW.tsv", "skill-action-bar-text-safe-rects.tsv",
		"manual-events.tsv", "manual-ordinals.tsv", "manual.zh-TW.tsv", "manual-overlay-layout.tsv",
		"body-icon-events.tsv", "body-icon-affixes.tsv", "body-icon.zh-TW.tsv", "body-icon-text-safe-rects.tsv",
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
	if r.actCatalog, err = LoadActionBarRequestCatalog(files["skill-action-bar-events.tsv"], files["skill-action-bar.zh-TW.tsv"]); err != nil {
		return nil, err
	}
	if r.actRects, err = LoadActionBarOverlayRects("skill-action-bar-text-safe-rects.tsv", files["skill-action-bar-text-safe-rects.tsv"]); err != nil {
		return nil, err
	}
	if err := r.resetAction(); err != nil {
		return nil, err
	}
	if r.stories, err = storyPages(textDir, font); err != nil {
		return nil, err
	}
	bodyCatalog, err := LoadBodyIconCatalog(files["body-icon-events.tsv"], files["body-icon-affixes.tsv"], files["body-icon.zh-TW.tsv"], files["body-icon-text-safe-rects.tsv"])
	if err != nil {
		return nil, err
	}
	if err := menu.RegisterAffix(bodyCatalog.SaveAffixShape()); err != nil {
		return nil, err
	}
	r.body = NewLiveBodyIconWatcher(bodyCatalog)
	for i, scale := range liveScales {
		if r.bodyPres[i], err = NewRuntimeBodyIconOverlay(bodyCatalog, font, scale, BodyIconLive); err != nil {
			return nil, err
		}
	}
	manualCatalog, err := LoadCatalog(files["manual-events.tsv"], files["manual-ordinals.tsv"], files["manual.zh-TW.tsv"])
	if err != nil {
		return nil, err
	}
	manualLayout, err := LoadManualOverlayLayout("manual-overlay-layout.tsv", files["manual-overlay-layout.tsv"])
	if err != nil {
		return nil, err
	}
	r.manual = NewWatcher(manualCatalog)
	for i, scale := range liveScales {
		if r.manPres[i], err = NewRuntimeManualOverlay(manualLayout, manualCatalog, font, scale); err != nil {
			return nil, err
		}
		consumer, err := NewManualPresentationConsumer(r.manPres[i])
		if err != nil {
			return nil, err
		}
		if r.manSync[i], err = NewManualPresentationBridge(r.manual, consumer); err != nil {
			return nil, err
		}
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
	if err := r.loadEclText(textDir); err != nil {
		return nil, err
	}
	if err := r.loadHMenu(textDir); err != nil {
		return nil, err
	}
	return r, nil
}

// loadEclText wires the spec-027 family when the catalog exists; a missing
// translation file means nothing is translated yet.
func (r *LiveRuntime) loadEclText(textDir string) error {
	events, err := os.ReadFile(filepath.Join(textDir, "ecl-text-events.tsv"))
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	tr, err := os.ReadFile(filepath.Join(textDir, "ecl-text.zh-TW.tsv"))
	if os.IsNotExist(err) {
		tr = []byte("key\ttranslation\tsource\n")
	} else if err != nil {
		return err
	}
	c, err := LoadEclTextCatalog(events, tr)
	if err != nil {
		return err
	}
	r.ecl = NewEclTextWatcher(c)
	r.eclGen = r.ecl.Generation()
	if eng, err := loadEngineText(textDir); err != nil {
		return err
	} else if eng != nil {
		r.ecl.SetEngine(eng)
		if err := r.loadEngineDispatch(textDir, eng); err != nil {
			return err
		}
		if err := r.loadLogbook(textDir, eng); err != nil {
			return err
		}
	}
	for i, scale := range liveScales {
		if r.eclPres[i], err = NewEclTextOverlay(r.font, scale); err != nil {
			return err
		}
	}
	return nil
}

func (r *LiveRuntime) loadHMenu(textDir string) error {
	events, err := os.ReadFile(filepath.Join(textDir, "hmenu-item-events.tsv"))
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	tr, err := os.ReadFile(filepath.Join(textDir, "hmenu.zh-TW.tsv"))
	if os.IsNotExist(err) {
		tr = []byte("key\ttranslation\tsource\n")
	} else if err != nil {
		return err
	}
	c, err := LoadHMenuCatalog(events, tr)
	if err != nil {
		return err
	}
	r.hmenu = NewHMenuWatcher(c)
	r.hmenuGen = r.hmenu.Generation()
	for i, scale := range liveScales {
		if r.hmenuPres[i], err = NewHMenuOverlay(r.font, scale); err != nil {
			return err
		}
	}
	return nil
}

// observeHMenu reads the 37F1:0243 parent frame (spec 028 §2).
func (r *LiveRuntime) observeHMenu(v StepReader, at Address) {
	ss, sp := v.SS(), v.SP()
	if at != hmenuPrinter {
		r.hmenu.ObserveInstruction(at, ss, sp)
		return
	}
	pb := v.Read16(linear(ss, sp+4))
	byteAt := func(off int) uint8 { return v.Read8(linear(ss, uint16(int(pb)+off))) }
	text := make([]byte, byteAt(-0x213))
	for i := range text {
		text[i] = byteAt(-0x200 + 1 + i)
	}
	items := make([][2]uint8, byteAt(-0x23D))
	for i := range items {
		items[i] = [2]uint8{byteAt(-0x23E + 2*(i+1)), byteAt(-0x23D + 2*(i+1))}
	}
	ds := v.DS()
	r.hmenu.ObserveEntry(HMenuEntry{
		SS: ss, SP: sp, Return: Address{Segment: v.Read16(linear(ss, sp+2)), Offset: v.Read16(linear(ss, sp))},
		Text: text, Row: byteAt(-0x24A), Col: byteAt(-0x214), Items: items,
		Selected: uint8(v.Read16(linear(ss, sp+6))),
		Normal:   v.Read8(linear(ds, 0x6B46)), Hot: v.Read8(linear(ds, 0x6B47)),
	})
}

func (r *LiveRuntime) syncHMenu(palette [256][3]uint8) {
	if r.hmenu == nil {
		return
	}
	if g := r.hmenu.Generation(); g != r.hmenuGen {
		p := r.hmenu.Page()
		for i := range liveScales {
			if miss := r.hmenuPres[i].Sync(p, g, palette); len(miss) != 0 {
				r.resets["hmenu"]++
				r.hmenu.ObserveDiscontinuity()
				g = r.hmenu.Generation()
				for j := range liveScales {
					r.hmenuPres[j].Sync(nil, g, palette)
				}
				break
			}
		}
		r.hmenuGen = g
	}
	for i := range liveScales {
		r.hmenuPres[i].Frame(palette)
	}
}

// loadEngineText loads the spec-029 catalogs when the fragment catalog
// exists; missing translation files mean nothing is translated yet.
func loadEngineText(textDir string) (*EngineTextCatalog, error) {
	read := func(name string, required bool) ([]byte, error) {
		b, err := os.ReadFile(filepath.Join(textDir, name))
		if os.IsNotExist(err) && !required {
			return nil, nil
		}
		return b, err
	}
	fe, err := os.ReadFile(filepath.Join(textDir, "engine-fragment-events.tsv"))
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var f EngineTextFiles
	f.FragmentEvents = fe
	for _, x := range []struct {
		dst  *[]byte
		name string
		req  bool
	}{
		{&f.FragmentText, "engine-fragment.zh-TW.tsv", false},
		{&f.ItemEvents, "item-word-events.tsv", true},
		{&f.ItemText, "item-word.zh-TW.tsv", false},
		{&f.PhraseEvents, "item-phrase-events.tsv", false},
		{&f.PhraseText, "item-phrase.zh-TW.tsv", false},
		{&f.TemplateEvents, "engine-template-events.tsv", false},
		{&f.TemplateText, "engine-template.zh-TW.tsv", false},
	} {
		if *x.dst, err = read(x.name, x.req); err != nil {
			return nil, err
		}
	}
	return LoadEngineTextCatalog(f)
}

func (r *LiveRuntime) loadEngineDispatch(textDir string, eng *EngineTextCatalog) error {
	allowData, err := os.ReadFile(filepath.Join(textDir, "engine-dispatch-callers.tsv"))
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	names, err := filepath.Glob(filepath.Join(textDir, "*-events.tsv"))
	if err != nil {
		return err
	}
	files := map[string][]byte{}
	for _, n := range names {
		b, err := os.ReadFile(n)
		if err != nil {
			return err
		}
		files[filepath.Base(n)] = b
	}
	allow, err := LoadEngineDispatchCallers(allowData, OwnedCallers(files))
	if err != nil {
		return err
	}
	r.engDisp = NewEngineDispatchWatcher(eng, allow)
	r.engDispGen = r.engDisp.Generation()
	for i, scale := range liveScales {
		if r.engDispPres[i], err = NewHMenuOverlay(r.font, scale); err != nil {
			return err
		}
	}
	return nil
}

func (r *LiveRuntime) loadLogbook(textDir string, eng *EngineTextCatalog) error {
	b, err := os.ReadFile(filepath.Join(textDir, "logbook.zh-TW.tsv"))
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	c, err := LoadLogbookCatalog(b)
	if err != nil {
		return err
	}
	r.logbook = NewLogbookWatcher(c, eng)
	for i, scale := range liveScales {
		if r.logbookPres[i], err = NewLogbookOverlay(r.font, scale); err != nil {
			return err
		}
	}
	return nil
}

// LogbookTurn flips the logbook panel from the host; false means the key
// was not taken (no panel open) and belongs to the game.
func (r *LiveRuntime) LogbookTurn(delta int) bool { return r.logbook.Turn(delta) }

func (r *LiveRuntime) syncLogbook(palette [256][3]uint8) {
	if r.logbook == nil {
		return
	}
	for i := range liveScales {
		if miss := r.logbookPres[i].Sync(r.logbook, palette); len(miss) != 0 {
			r.resets["logbook"]++
		}
	}
}

func (r *LiveRuntime) syncEngineDispatch(palette [256][3]uint8) {
	if r.engDisp == nil {
		return
	}
	if g := r.engDisp.Generation(); g != r.engDispGen {
		p := r.engDisp.Page()
		for i := range liveScales {
			if miss := r.engDispPres[i].Sync(p, g, palette); len(miss) != 0 {
				r.resets["engine-dispatch"]++
				r.engDisp.ObserveDiscontinuity()
				g = r.engDisp.Generation()
				for j := range liveScales {
					r.engDispPres[j].Sync(nil, g, palette)
				}
				break
			}
		}
		r.engDispGen = g
	}
	for i := range liveScales {
		r.engDispPres[i].Frame(palette)
	}
}

// observeEclText handles the text-window printer entry and its return.
func (r *LiveRuntime) observeEclText(v StepReader, at Address) {
	if at == eclTextPrinter {
		ss, sp := v.SS(), v.SP()
		arg := func(i uint16) uint16 { return v.Read16(linear(ss, sp+i)) }
		base := linear(arg(6), arg(4))
		orig := make([]byte, v.Read8(base))
		for i := range orig {
			orig[i] = v.Read8(base + 1 + uint32(i))
		}
		ds := v.DS()
		if r.logbook != nil {
			r.logbook.ObserveEntry(orig, uint8(arg(20)), uint8(arg(18)), uint8(arg(16)), uint8(arg(14)))
		}
		r.ecl.ObserveEntry(EclTextEntry{
			Step: v.Steps(), SS: ss, SP: sp, Return: Address{Segment: arg(2), Offset: arg(0)}, Original: orig,
			Clear: uint8(arg(8)) != 0, Background: uint8(arg(10)), Foreground: uint8(arg(12)),
			Bottom: uint8(arg(14)), Right: uint8(arg(16)), Top: uint8(arg(18)), Left: uint8(arg(20)),
			CursorCol: v.Read8(linear(ds, 0x5F3E)), CursorRow: v.Read8(linear(ds, 0x5F3F)),
		})
		return
	}
	r.ecl.ObserveInstruction(at, v.SS(), v.SP())
}

// syncEclText rebuilds both presenters after a generation change.  A page
// the font cannot draw is dropped like an overflow (the original shows).
func (r *LiveRuntime) syncEclText(palette [256][3]uint8) {
	if r.ecl == nil {
		return
	}
	if g := r.ecl.Generation(); g != r.eclGen {
		p := r.ecl.Page()
		for i := range liveScales {
			if miss := r.eclPres[i].Sync(p, g, palette); len(miss) != 0 {
				r.resets["ecl-text"]++
				r.ecl.ObserveDiscontinuity()
				g = r.ecl.Generation()
				for j := range liveScales {
					r.eclPres[j].Sync(nil, g, palette)
				}
				break
			}
		}
		r.eclGen = g
	}
	for i := range liveScales {
		r.eclPres[i].Frame(palette)
	}
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

func (r *LiveRuntime) resetAction() error {
	var pres [2]*RuntimeActionBarOverlay
	for i, scale := range liveScales {
		style := HotkeyPreservingActionBarNormalStyle()
		p, err := NewRuntimeActionBarOverlay(r.actCatalog, r.actRects, r.font, scale, &style)
		if err != nil {
			return err
		}
		pres[i] = p
	}
	r.action, r.actPres = NewActionBarRequestWatcher(r.actCatalog), pres
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

var glyphEntry = Address{Segment: 0x0763, Offset: 0x026B}

type storyFrame struct {
	entryStep uint64
	caller    Address
	ss, sp    uint16
}

// BeforeStep dispatches one instruction in the receipt runner's order.
func (r *LiveRuntime) BeforeStep(v StepReader) error {
	at := Address{Segment: v.CS(), Offset: v.IP()}
	if r.ecl != nil && (at == eclTextPrinter || r.ecl.InCall()) {
		r.observeEclText(v, at)
	}
	if r.hmenu != nil && (at == hmenuPrinter || r.hmenu.InCall()) {
		r.observeHMenu(v, at)
	}
	if r.engDisp.InCall() {
		r.engDisp.ObserveInstruction(at, v.SS(), v.SP())
	}
	if p := r.storyPending; p != nil && r.prevAt == storyGlyphReturnSite && r.prevOp == 0xCA && at == p.caller {
		if ss, sp := v.SS(), v.SP(); ss == p.ss && sp == p.sp+storyReturnStackDelta {
			for _, f := range r.stories {
				f.verifiedReturn(p.entryStep, p.caller, r.prevAt, r.prevOp, ss, sp, v.Steps())
			}
			r.storyPending = nil
		}
	}
	actionRequests := -1
	if r.action.Pending() {
		actionRequests = len(r.action.requests)
		r.action.ObserveInstruction(at, v.SS(), v.SP(), v.Steps())
	}
	obs, err := r.menu.Observe(v)
	if err != nil {
		// The menu family owns the shared recorder; rebuilding it is not
		// possible without losing the other families' frames, so this stays
		// fatal (it has never been observed on a normal path).
		return err
	}
	if obs.Kind == ObservedEntry && r.engDisp != nil {
		r.engDisp.ObserveEntry(obs.Caller, obs.SS, obs.SP, obs.Caller, obs.Args, obs.Original)
	}
	switch obs.Kind {
	case ObservedClear:
		// A proven clear can also be the guarded return of an in-flight manual
		// call: keep that post-call before applying the clear (runner order).
		r.manual.ObserveInstruction(at, obs.SS, obs.SP, v.Steps())
		r.manual.ObserveClear(v.Steps())
		c := obs.Clear
		r.action.ObserveClear(c[0], c[1], c[2], c[3])
		for i := range liveScales {
			if err := r.actPres[i].ClearTextCells(c[0], c[1], c[2], c[3]); err != nil {
				if err := r.recover("action-bar", r.resetAction); err != nil {
					return err
				}
				break
			}
		}
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
		r.manual.ObserveDispatchEntryWithStyle(obs.Caller, obs.SS, obs.SP, string(obs.Original), ManualTextStyle{
			Background: uint8(obs.Args[2]), Foreground: uint8(obs.Args[3]), Row: uint8(obs.Args[4]), Column: uint8(obs.Args[5]),
		}, steps)
		if postJoinEntryRow(obs.Caller, uint8(obs.Args[4])) {
			if err := r.postJoin.ObserveEntry(textEvent(steps, obs.Caller, obs.Args, obs.Original)); err != nil {
				if err := r.recover("post-join", r.resetPostJoin); err != nil {
					return err
				}
			}
		}
	case ObservedOther:
		if obs.NewRequest {
			r.action.ObserveAnchorEvent(obs.Request.EventKey)
			for i := range liveScales {
				r.actPres[i].ObserveAnchorEvent(obs.Request.EventKey)
			}
		}
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
		if obs.NewEvent {
			for _, t := range r.body.Observe(obs.Event) {
				for i := range liveScales {
					if err := r.bodyPres[i].Apply(t, palette); err != nil {
						r.resets["body-icon"]++
					}
				}
				r.yieldMenu(r.bodyPres[0].SafeLogicalRects())
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
	if obs.Kind == ObservedOther || (obs.Kind == ObservedNothing && r.manual.pending != nil) {
		r.manual.ObserveInstruction(at, v.SS(), v.SP(), v.Steps())
	}
	if err := r.syncManual(); err != nil {
		return err
	}
	if at == glyphEntry {
		ss, sp := v.SS(), v.SP()
		caller := Address{Segment: v.Read16(linear(ss, sp+2)), Offset: v.Read16(linear(ss, sp))}
		var args [7]uint16
		for i := range args {
			args[i] = v.Read16(linear(ss, sp+4+uint16(i)*2))
		}
		steps := v.Steps()
		for _, f := range r.stories {
			if r.storyPending != nil {
				f.discontinuity()
			}
			f.glyphEntry(caller, ss, sp, args, steps)
		}
		r.storyPending = &storyFrame{entryStep: steps, caller: caller, ss: ss, sp: sp}
		r.action.ObserveGlyphEntry(caller, ss, sp, args, steps)
	}
	if at == storyClearWrite {
		es, di, cx := v.ES(), v.DI(), v.CX()
		for _, f := range r.stories {
			f.clearWrite(at, es, di, cx)
		}
		r.storyDirty = true
	}
	if r.storyDirty || r.storyPending != nil || at == glyphEntry {
		// Generations can only advance after a glyph entry/return, a clear
		// write or a page-9 pre-write; only then check, and read the palette
		// only for a family that actually has a new generation.
		var palette [256][3]uint8
		read := false
		for _, f := range r.stories {
			if !f.needsApply() {
				continue
			}
			if !read {
				palette, read = v.Palette(), true
			}
			if err := f.apply(palette); err != nil {
				r.resets[f.name()]++
				f.clear()
			}
		}
		r.storyDirty = r.storyPending != nil
	}
	if r.storyPending != nil {
		r.prevAt, r.prevOp = at, v.Read8(linear(at.Segment, at.Offset))
	}
	if actionRequests >= 0 && len(r.action.requests) > actionRequests {
		events := r.action.collector.events
		request := r.action.requests[len(r.action.requests)-1]
		palette := v.Palette()
		for i := range liveScales {
			if err := r.actPres[i].Apply(events[len(events)-1], request, palette); err != nil {
				return r.recover("action-bar", r.resetAction)
			}
		}
	}
	return nil
}

// syncManual forwards new manual presentation events and style changes to
// both scales, only when something changed (the runner does it every step).
func (r *LiveRuntime) syncManual() error {
	if style, ok := r.manual.ManualStyle(); ok && (!r.manHas || style != r.manStyle) {
		for i := range liveScales {
			if err := r.manPres[i].SetStyle(style); err != nil {
				r.resets["manual"]++
				return nil
			}
		}
		r.manStyle, r.manHas = style, true
	}
	if n := len(r.manual.presentation); n != r.manSeen {
		r.manSeen = n
		for i := range liveScales {
			if _, err := r.manSync[i].Sync(); err != nil {
				r.resets["manual"]++
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
	if r.ecl != nil {
		r.ecl.ObserveVideoWrite(w.Offset)
	}
	if r.hmenu != nil {
		r.hmenu.ObserveVideoWrite(w.Offset)
	}
	if r.engDisp != nil {
		r.engDisp.ObserveVideoWrite(w.Offset)
	}
	if r.logbook != nil {
		r.logbook.ObserveVideoWrite(w.CS, w.IP, w.Offset)
	}
	for i := range liveScales {
		r.bodyPres[i].Prewrite(w)
	}
	for _, f := range r.stories {
		f.prewrite(w)
	}
	r.storyDirty = true
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
	for _, f := range r.stories {
		f.frame(indexed, palette)
	}
	for i := range liveScales {
		r.manPres[i].Frame(indexed, palette)
	}
	for i := range liveScales {
		r.actPres[i].Frame(indexed, palette)
	}
	r.syncEclText(palette)
	r.syncHMenu(palette)
	r.syncEngineDispatch(palette)
	r.syncLogbook(palette)
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
	return r.ComposeWith(r.indexed, r.palette, scale)
}

// ComposeWith composes the overlay onto a caller-supplied frame instead of
// the last retrace; the receipt runner uses it to compare at an exact step.
func (r *LiveRuntime) ComposeWith(indexed []byte, palette [256][3]uint8, scale int) ([]byte, bool, error) {
	saveIndexed, savePalette := r.indexed, r.palette
	r.indexed, r.palette = indexed, palette
	defer func() { r.indexed, r.palette = saveIndexed, savePalette }()
	// Generic families rebuild lazily; catch up with invalidations that
	// happened after the last retrace before drawing.
	r.syncEclText(palette)
	r.syncHMenu(palette)
	r.syncEngineDispatch(palette)
	r.syncLogbook(palette)
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
	if len(r.bodyPres[i].ActiveKeys()) != 0 {
		rgba, missing, _ := r.bodyPres[i].Draw(r.indexed, r.palette)
		if err := layer(rgba, missing); err != nil {
			return nil, false, err
		}
	}
	if len(r.manPres[i].ActiveKeys()) != 0 {
		rgba, missing, _ := r.manPres[i].Draw(r.indexed, r.palette)
		if err := layer(rgba, missing); err != nil {
			return nil, false, err
		}
	}
	for _, f := range r.stories {
		if rgba, missing, active := f.draw(i, r.indexed, r.palette); active {
			if err := layer(rgba, missing); err != nil {
				return nil, false, err
			}
		}
	}
	if len(r.actPres[i].ActiveKeys()) != 0 {
		rgba, missing, _ := r.actPres[i].Draw(r.indexed, r.palette)
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
	if r.ecl != nil && r.eclPres[i].Active() {
		rgba, missing := r.eclPres[i].Draw(r.indexed, r.palette)
		if err := layer(rgba, missing); err != nil {
			return nil, false, err
		}
	}
	// Spec 028: families with their own reviewed catalogs own their rows; the
	// generic menu family draws only rows nobody else changed.
	if r.hmenu != nil && r.hmenuPres[i].Active() && !r.rowsTouched(out, scale, r.hmenu.Page()) {
		rgba, missing := r.hmenuPres[i].Draw(r.indexed, r.palette)
		if err := layer(rgba, missing); err != nil {
			return nil, false, err
		}
	}
	if r.engDisp != nil && r.engDispPres[i].Active() && !r.rowsTouched(out, scale, r.engDisp.Page()) {
		rgba, missing := r.engDispPres[i].Draw(r.indexed, r.palette)
		if err := layer(rgba, missing); err != nil {
			return nil, false, err
		}
	}
	// The logbook panel is drawn last: it sits over everything while open.
	if r.logbook != nil && r.logbookPres[i].Active() {
		rgba, missing := r.logbookPres[i].Draw(r.indexed, r.palette)
		if err := layer(rgba, missing); err != nil {
			return nil, false, err
		}
	}
	return out, true, nil
}

// Frames counts retraces observed so far.
func (r *LiveRuntime) Frames() uint64 { return r.frameSeen }

// DebugSummary reports family counters for diagnostics (no original text).
func (r *LiveRuntime) DebugSummary() string {
	s := fmt.Sprintf("resets=%v", r.Resets())
	if r.ecl != nil {
		s += fmt.Sprintf(" ecl=%+v", r.ecl.Stats)
	}
	if r.hmenu != nil {
		s += fmt.Sprintf(" hmenu=%+v", r.hmenu.Stats)
	}
	if r.engDisp != nil {
		s += fmt.Sprintf(" engine-dispatch=%+v", r.engDisp.Stats)
	}
	return s
}

// rowsTouched reports whether out already differs from the plain scaled
// frame on any text row the menu page uses.
func (r *LiveRuntime) rowsTouched(out []byte, scale int, p *HMenuPage) bool {
	if p == nil {
		return false
	}
	w := 320 * scale
	for _, row := range p.Rows {
		for y := int(row.Row) * 8; y < int(row.Row)*8+8; y++ {
			for x := 0; x < 320; x++ {
				c := r.palette[r.indexed[y*320+x]]
				o := ((y*scale)*w + x*scale) * 4
				if out[o] != c[0] || out[o+1] != c[1] || out[o+2] != c[2] {
					return true
				}
			}
		}
	}
	return false
}
