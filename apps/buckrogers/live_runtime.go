package buckrogers

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
// Spec 040 (Buck repo): one observation dispatch serves every enabled
// language.  The shared, language-independent state lives here (recorder,
// watchers of the A families, party snapshot, story in-flight call, …);
// each language owns a lane (presenters at both scales, the B-family
// watchers, glossary, player names, font).  Every call the runtime makes to
// a per-scale presenter or owner is made for every lane, so switching only
// changes the lane Compose reads.
//
// Receipts fail closed; a live game must not stop because an overlay family
// lost track.  A family that errors is cleared (the original English shows)
// and rebuilt from its catalog, and the reset is counted.
type LiveRuntime struct {
	help []string // 前端說明頁（spec 241）；zh-TW
	font *xlate.Font
	// menu is the shared recorder (spec 040 §3.3: no presenters in it).
	menu       *LiveMenuRuntime
	menuShared *liveMenuShared
	postJoin   *PostJoinMenuWatcher
	postShared *PostJoinMenuCatalog
	action     *ActionBarWatcher
	actRef     *ActionBarRequestCatalog // zh-TW, hotkey reference
	actRects   *MenuOverlayRects
	body       *LiveBodyIconWatcher
	manual     *Watcher
	manLayout  *ManualOverlayLayout
	// Spec 034: optional local English keyword rows (zh-TW only).
	manCatalog *Catalog
	manEng     *ManualEnglish
	manEngPres [2]manualEnglishPresenter
	manEngOff  string
	textDir    string
	started    bool
	files      map[string][]byte // shared A-family files

	// storyPending is the shared in-flight 0763:026B glyph call; prev* is
	// the instruction before the current one, recorded while it is set.
	storyPending *storyFrame
	prevAt       Address
	prevOp       byte
	storyDirty   bool

	// Spec 038: party snapshot (taken only at the name hooks).
	party partyTracker

	ovl       OverlayUnits // spec 032
	norm      LegacyNormaliser
	normFrame uint64
	// Spec 031: original Latin glyphs, kept for diagnostics (spec 039 §3.6).
	asciiFound bool
	asciiTry   uint64 // frame of the last search + 1 (0 = never)
	asciiScans int
	asciiStep  uint64 // step at which the table was acquired (0 = not yet)
	// asciiStoryErrs counts story families whose SetFont failed (they keep
	// the previous font); diagnostic only, never a reset.
	asciiStoryErrs int

	// lanes[0] is zh-TW; cur indexes the composed lane, -1 is English.
	lanes []*liveLane
	cur   int
	// off lists languages that were tried and are not enabled, with why.
	off map[string]string

	indexed   []byte
	palette   [256][3]uint8
	hasFrame  bool
	frameSeen uint64
	// resets counts rebuilds of shared watchers (spec 040 §3.3).
	resets map[string]int
}

// LiveOptions selects the languages a runtime loads (spec 040 §3.4).
type LiveOptions struct {
	TextDir string
	// FontPath is the zh-TW 16×16 GOLEMFNT (required).
	FontPath string
	// Langs are the other languages to try, in load order; nil tries none.
	Langs []string
	// LangFonts maps a language to its font; a language without one stays
	// off (spec 040 §3.4: font/buckrogers-<lang>.golemfnt).
	LangFonts map[string]string
	// LangDirs overrides the directory of a language's <family>.<lang>.tsv
	// (the test-only language of spec 040 §5.3); default TextDir.
	LangDirs map[string]string
}

// LoadLiveRuntime builds every live family from a Buck Rogers text directory
// and a local 16×16 GOLEMFNT, with zh-TW and English only.
func LoadLiveRuntime(textDir, fontPath string) (*LiveRuntime, error) {
	return LoadLiveRuntimeOptions(LiveOptions{TextDir: textDir, FontPath: fontPath})
}

// sharedFiles are the language-independent files of the A families.
var sharedFiles = []string{
	"career-skill-exit-events.tsv", "technical-skill-exit-events.tsv",
	"post-join-exit-prompt-events.tsv",
	"post-join-menu-events.tsv", "post-join-menu-variants.tsv",
	"skill-action-bar-events.tsv", "skill-action-bar-text-safe-rects.tsv",
	"manual-events.tsv", "manual-ordinals.tsv", "manual-overlay-layout.tsv",
	"body-icon-events.tsv", "body-icon-affixes.tsv", "body-icon-text-safe-rects.tsv",
}

// laneFamilies are the A-family language files (spec 040 §3.4: a language
// is enabled only when all of them exist and pass the load check).
var laneFamilies = []string{
	"skill-exit-confirmation", "post-join-exit-prompt", "post-join-menu", "skill-action-bar", "manual", "body-icon",
	"story-opening", "story-page2", "story-page3", "story-page4", "story-page5", "story-page6", "story-page7", "story-page8", "story-page9",
}

// LoadLiveRuntimeOptions loads the shared families, the zh-TW lane (a
// failure is a start failure) and every requested language (a failure
// only leaves that language off, spec 040 §3.3).
func LoadLiveRuntimeOptions(o LiveOptions) (*LiveRuntime, error) {
	textDir := o.TextDir
	font, err := xlate.LoadFont(o.FontPath)
	if err != nil {
		return nil, err
	}
	r := &LiveRuntime{font: font, textDir: textDir, resets: map[string]int{}, off: map[string]string{}, files: map[string][]byte{}}
	if r.menuShared, err = loadLiveMenuShared(textDir); err != nil {
		return nil, err
	}
	r.menu = newLiveMenuRecorder(r.menuShared.full.identityOnly(), r.menuShared.headers)
	for _, name := range sharedFiles {
		b, err := os.ReadFile(filepath.Join(textDir, name))
		if err != nil {
			return nil, fmt.Errorf("buckrogers: 讀取 %s：%w", name, err)
		}
		r.files[name] = b
	}
	zh := map[string][]byte{}
	for _, fam := range []string{"skill-exit-confirmation", "post-join-exit-prompt", "post-join-menu", "skill-action-bar", "manual", "body-icon"} {
		b, err := os.ReadFile(filepath.Join(textDir, LangFile(fam, LangZhTW)))
		if err != nil {
			return nil, fmt.Errorf("buckrogers: 讀取 %s：%w", LangFile(fam, LangZhTW), err)
		}
		zh[fam] = b
	}
	if r.postShared, err = LoadPostJoinMenuCatalog(r.files["post-join-menu-events.tsv"], r.files["post-join-menu-variants.tsv"], zh["post-join-menu"]); err != nil {
		return nil, err
	}
	if r.actRef, err = LoadActionBarRequestCatalog(r.files["skill-action-bar-events.tsv"], zh["skill-action-bar"]); err != nil {
		return nil, err
	}
	if r.actRects, err = LoadActionBarOverlayRects("skill-action-bar-text-safe-rects.tsv", r.files["skill-action-bar-text-safe-rects.tsv"]); err != nil {
		return nil, err
	}
	r.action = NewActionBarRequestWatcher(r.actRef.identityOnly())
	if r.help, err = LoadHelp(textDir, font); err != nil {
		return nil, err
	}
	bodyCatalog, err := LoadBodyIconCatalog(r.files["body-icon-events.tsv"], r.files["body-icon-affixes.tsv"], zh["body-icon"], r.files["body-icon-text-safe-rects.tsv"])
	if err != nil {
		return nil, err
	}
	if err := r.menu.RegisterAffix(bodyCatalog.SaveAffixShape()); err != nil {
		return nil, err
	}
	r.body = NewLiveBodyIconWatcher(bodyCatalog)
	if r.manCatalog, err = LoadCatalog(r.files["manual-events.tsv"], r.files["manual-ordinals.tsv"], zh["manual"]); err != nil {
		return nil, err
	}
	if r.manLayout, err = LoadManualOverlayLayout("manual-overlay-layout.tsv", r.files["manual-overlay-layout.tsv"]); err != nil {
		return nil, err
	}
	r.manual = newSharedWatcher(r.manCatalog)
	if r.postJoin, err = NewPostJoinMenuWatcher(r.postShared); err != nil {
		return nil, err
	}
	lane, err := r.loadLane(LangZhTW, font, textDir)
	if err != nil {
		return nil, err
	}
	r.lanes = []*liveLane{lane}
	// Spec 039 §3.4 欄名列: dispatcher rows are checked against the engine
	// catalog when the dispatcher loads; without it they cannot be.
	if r.menu.HeaderColumns().HasDispatcher() && lane.engDisp == nil {
		return nil, fmt.Errorf("buckrogers: %s 有 dispatcher 列，但引擎 dispatcher 未載入", HeaderColumnsFile)
	}
	for _, lang := range o.Langs {
		if lang == LangZhTW || lang == LangEn {
			continue
		}
		if !KnownLang(lang) {
			return nil, fmt.Errorf("buckrogers: 不認得的語言代碼 %q", lang)
		}
		if _, dup := r.off[lang]; dup || r.laneIndex(lang) >= 0 {
			continue
		}
		dir := textDir
		if d := o.LangDirs[lang]; d != "" {
			dir = d
		}
		why := ""
		if missing := missingLaneFiles(dir, lang); missing != "" {
			why = "缺語言檔 " + missing
		} else if o.LangFonts[lang] == "" {
			why = "缺字型"
		} else if f, err := xlate.LoadFont(o.LangFonts[lang]); err != nil {
			why = "字型：" + err.Error()
		} else if l, err := r.loadLane(lang, f, dir); err != nil {
			why = err.Error()
		} else {
			r.lanes = append(r.lanes, l)
			continue
		}
		r.off[lang] = why
		fmt.Fprintf(os.Stderr, "buckrogers: 語言 %s 停用：%s\n", lang, why)
	}
	return r, nil
}

// missingLaneFiles names the first A-family language file that is absent.
func missingLaneFiles(dir, lang string) string {
	for _, fam := range laneFamilies {
		if _, err := os.Stat(filepath.Join(dir, LangFile(fam, lang))); err != nil {
			return LangFile(fam, lang)
		}
	}
	for _, src := range liveMenuSources {
		name := LangFile(menuFamilyStem(src.translations), lang)
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			return name
		}
	}
	return ""
}

func (r *LiveRuntime) laneIndex(lang string) int {
	for i, l := range r.lanes {
		if l.lang == lang {
			return i
		}
	}
	return -1
}

// LangStatus is one language of the F4 cycle (spec 040 §3.4).
type LangStatus struct {
	Code    string
	Enabled bool
	Reason  string // why it is off; empty when enabled
}

// Languages lists the F4 cycle with each language's state, then the test
// language when it is loaded.
func (r *LiveRuntime) Languages() []LangStatus {
	var out []LangStatus
	for _, c := range LangCycle {
		s := LangStatus{Code: c, Enabled: c == LangEn || r.laneIndex(c) >= 0}
		if !s.Enabled {
			if why, ok := r.off[c]; ok {
				s.Reason = why
			} else {
				s.Reason = "未載入"
			}
		}
		out = append(out, s)
	}
	if r.laneIndex(LangTest) >= 0 {
		out = append(out, LangStatus{Code: LangTest, Enabled: true})
	}
	return out
}

// Language is the composed language.
func (r *LiveRuntime) Language() string {
	if r.cur < 0 {
		return LangEn
	}
	return r.lanes[r.cur].lang
}

// SetLanguage switches the composed language.  It changes no lane and no
// observation state: every lane is already current (spec 040 §3.2).
func (r *LiveRuntime) SetLanguage(code string) error {
	if !KnownLang(code) {
		return fmt.Errorf("buckrogers: 不認得的語言代碼 %q", code)
	}
	if code == LangEn {
		r.cur = -1
		return nil
	}
	i := r.laneIndex(code)
	if i < 0 {
		why := r.off[code]
		if why == "" {
			why = "未載入"
		}
		return fmt.Errorf("buckrogers: 語言 %s 未啟用：%s", code, why)
	}
	r.cur = i
	return nil
}

// NextLanguage is the next enabled language of the F4 cycle.
func (r *LiveRuntime) NextLanguage() string {
	cur := r.Language()
	at := 0
	for i, c := range LangCycle {
		if c == cur {
			at = i
		}
	}
	for k := 1; k <= len(LangCycle); k++ {
		c := LangCycle[(at+k)%len(LangCycle)]
		if c == LangEn || r.laneIndex(c) >= 0 {
			return c
		}
	}
	return cur
}

// LogbookTurn flips the logbook panel from the host (spec 040 §3.4): the
// composed language's panel decides whether the key is taken; then every
// lane with an open panel turns within its own pages.  false means the key
// was not taken and belongs to the game.
func (r *LiveRuntime) LogbookTurn(delta int) bool {
	if r.cur < 0 {
		return false
	}
	cur := r.lanes[r.cur]
	if cur.logbook == nil || cur.logbook.open == 0 {
		return false
	}
	for _, l := range r.lanes {
		if l.logbook != nil {
			l.logbook.Turn(delta)
		}
	}
	return true
}

// Resets reports how many times each family was rebuilt after an error:
// the shared watchers and the zh-TW lane, under the established names.
func (r *LiveRuntime) Resets() map[string]int {
	out := make(map[string]int, len(r.resets))
	for k, v := range r.resets {
		out[k] = v
	}
	if len(r.lanes) != 0 {
		for k, v := range r.lanes[0].resets {
			out[k] += v
		}
	}
	return out
}

// LaneResets reports one language lane's rebuilds, draw-time skips and
// untranslated requests (spec 040 §3.3 per-language counters).
func (r *LiveRuntime) LaneResets(lang string) (resets, skips map[string]int, ok bool) {
	i := r.laneIndex(lang)
	if i < 0 {
		return nil, nil, false
	}
	l := r.lanes[i]
	resets, skips = map[string]int{}, map[string]int{}
	for k, v := range l.resets {
		resets[k] = v
	}
	for k, v := range l.skips {
		skips[k] = v
	}
	return resets, skips, true
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

// genericActive reports whether a spec 027–030 family has content to draw
// in any lane.
func (r *LiveRuntime) genericActive() bool {
	for _, l := range r.lanes {
		if l.genericActive() {
			return true
		}
	}
	return false
}

// origASCIIFinder is the table search used by LiveRuntime; tests swap it
// for a synthetic-hash search.  Production always uses FindOriginalASCII.
var origASCIIFinder = FindOriginalASCII

// findOriginalASCII keeps the spec 031 §3.1 table search (same trigger and
// 60-frame throttle) for diagnostics only.  Spec 039 §3.6: the original
// glyphs are no longer used by any overlay family (half-width characters
// use the half font derived from the unmodified base font), so a hit no
// longer rebuilds presenters or switches the story fonts.
func (r *LiveRuntime) findOriginalASCII(v StepReader, story bool) error {
	if r.asciiFound || r.asciiTry != 0 && r.frameSeen < r.asciiTry-1+60 || !story && !r.genericActive() {
		return nil
	}
	r.asciiTry = r.frameSeen + 1
	r.asciiScans++
	if _, ok := origASCIIFinder(v, origASCIIScanLimit); !ok {
		return nil
	}
	r.asciiFound = true
	r.asciiStep = v.Steps()
	return nil
}

// applyStories applies every story family with a new generation in every
// lane.  Spec 031 §3.4: before the apply loop, a story family that needs
// apply may trigger the shared original-glyph search (same throttle as the
// generic path).
func (r *LiveRuntime) applyStories(v StepReader) error {
	if !r.asciiFound {
	search:
		for _, l := range r.lanes {
			for _, f := range l.stories {
				if f.needsApply() {
					if err := r.findOriginalASCII(v, true); err != nil {
						return err
					}
					break search
				}
			}
		}
	}
	var palette [256][3]uint8
	read := false
	for _, l := range r.lanes {
		for _, f := range l.stories {
			if !f.needsApply() {
				continue
			}
			if !read {
				palette, read = v.Palette(), true
			}
			if err := f.apply(palette); err != nil {
				l.resets[f.name()]++
				f.clear()
			}
		}
	}
	return nil
}

// resetPostJoin rebuilds the shared post-join watcher after its own error
// and clears every lane's presenters (they showed its generations).
func (r *LiveRuntime) resetPostJoin() error {
	r.resets["post-join"]++
	w, err := NewPostJoinMenuWatcher(r.postShared)
	if err != nil {
		return err
	}
	r.postJoin = w
	for _, l := range r.lanes {
		if err := l.resetPost(); err != nil {
			return err
		}
	}
	return nil
}

// BeforeStep dispatches one instruction in the receipt runner's order.
func (r *LiveRuntime) BeforeStep(v StepReader) error {
	r.started = true
	if err := r.findOriginalASCII(v, false); err != nil {
		return err
	}
	at := Address{Segment: v.CS(), Offset: v.IP()}
	// Spec 033: rebuild the legacy normaliser each retrace and at every
	// dispatcher / ECL printer entry; older families get a normalised copy.
	if r.normFrame != r.frameSeen+1 || at == dispatchEntry || at == eclTextPrinter {
		r.norm.Rebuild(&r.ovl, v)
		r.normFrame = r.frameSeen + 1
		r.menu.Norm = &r.norm
	}
	legacyAt := r.norm.Addr(at)
	// Spec 030 §3.3-4: compare before a 056C entry reopens and re-arms.
	if r.anyLogbook() {
		head := v.Read16(BDAKeyHead)
		for _, l := range r.lanes {
			if l.logbook != nil {
				l.logbook.ObserveKeyHead(head)
			}
		}
	}
	r.observeEclText(v, at)
	r.observeHMenu(v, at)
	for _, l := range r.lanes {
		if l.engDisp.InCall() {
			l.engDisp.ObserveInstruction(at, v.SS(), v.SP())
		}
	}
	if p := r.storyPending; p != nil && r.prevAt == storyGlyphReturnSite && r.prevOp == 0xCA && legacyAt == p.caller {
		if ss, sp := v.SS(), v.SP(); ss == p.ss && sp == p.sp+storyReturnStackDelta {
			for _, l := range r.lanes {
				for _, f := range l.stories {
					f.verifiedReturn(p.entryStep, p.caller, r.prevAt, r.prevOp, ss, sp, v.Steps())
				}
			}
			r.storyPending = nil
		}
	}
	actionRequests := -1
	if r.action.Pending() {
		actionRequests = len(r.action.requests)
		r.action.ObserveInstruction(legacyAt, v.SS(), v.SP(), v.Steps())
	}
	obs, err := r.menu.observeRef(v, at)
	if err != nil {
		// The shared recorder has no presenters (spec 040 §3.3); its own
		// error cannot be rebuilt without losing the other families'
		// frames, so it stays fatal (never observed on a normal path).
		return err
	}
	// Spec 040 §3.2: the menu presenters moved out of the recorder; every
	// lane receives the clear and the new request, in the recorder's place.
	switch {
	case obs.Kind == ObservedClear:
		c := obs.Clear
		for _, l := range r.lanes {
			l.menuClear(c)
		}
	case obs.Kind == ObservedOther && obs.NewEvent && obs.NewRequest:
		palette := v.Palette()
		for _, l := range r.lanes {
			l.menuApply(obs.Event, obs.Request, palette)
		}
	}
	if obs.Kind == ObservedEntry && r.lanes[0].engDisp != nil {
		key := r.ovl.Key(v, obs.Caller)
		var party *PartySnapshot
		if r.lanes[0].engDisp.NeedsParty(key) {
			party = r.party.refresh(v, v.DS())
		}
		for _, l := range r.lanes {
			l.engDisp.ObserveEntryParty(key, obs.SS, obs.SP, obs.Caller, obs.Args, obs.Original, party, v)
		}
	}
	switch obs.Kind {
	case ObservedClear:
		// A proven clear can also be the guarded return of an in-flight manual
		// call: keep that post-call before applying the clear (runner order).
		r.manual.ObserveInstruction(legacyAt, obs.SS, obs.SP, v.Steps())
		r.manual.ObserveClear(v.Steps())
		c := obs.Clear
		r.action.ObserveClear(c[0], c[1], c[2], c[3])
		for _, l := range r.lanes {
			l.actionClear(c)
		}
	case ObservedEntry:
		steps := v.Steps()
		for _, l := range r.lanes {
			if err := l.ownersDropped(obs.Dropped); err != nil {
				return err
			}
		}
		if obs.LegacyCaller == dispatchPrompt {
			e := textEvent(steps, obs.LegacyCaller, obs.Args, obs.Original)
			row24 := uint8(obs.Args[4]) == 24 && uint8(obs.Args[5]) == 0
			for _, l := range r.lanes {
				if err := l.ownersEntry(e, row24); err != nil {
					return err
				}
			}
		}
		r.manual.ObserveDispatchEntryWithStyle(obs.LegacyCaller, obs.SS, obs.SP, string(obs.Original), ManualTextStyle{
			Background: uint8(obs.Args[2]), Foreground: uint8(obs.Args[3]), Row: uint8(obs.Args[4]), Column: uint8(obs.Args[5]),
		}, steps)
		if postJoinEntryRow(obs.LegacyCaller, uint8(obs.Args[4])) {
			if err := r.postJoin.ObserveEntry(textEvent(steps, obs.LegacyCaller, obs.Args, obs.Original)); err != nil {
				if err := r.resetPostJoin(); err != nil {
					return err
				}
			}
		}
	case ObservedOther:
		if obs.NewRequest {
			r.action.ObserveAnchorEvent(obs.Request.EventKey)
			for _, l := range r.lanes {
				for i := range liveScales {
					l.actPres[i].ObserveAnchorEvent(obs.Request.EventKey)
				}
			}
		}
		palette := [256][3]uint8{}
		if obs.NewEvent {
			palette = v.Palette()
		}
		for _, l := range r.lanes {
			if err := l.ownersReturn(obs, palette); err != nil {
				return err
			}
		}
		if obs.NewEvent {
			for _, t := range r.body.Observe(obs.Event) {
				for _, l := range r.lanes {
					l.bodyApply(t, palette)
				}
			}
		}
		if obs.NewEvent && postJoinRuntimeGate(obs.Event) {
			prior := len(r.postJoin.Generations())
			if err := r.postJoin.ObserveReturn(obs.Event); err != nil {
				// Spec 040 §3.3: rebuild the shared watcher; no early return.
				if err := r.resetPostJoin(); err != nil {
					return err
				}
			} else {
				for _, g := range r.postJoin.Generations()[prior:] {
					for _, l := range r.lanes {
						if err := l.postApply(g, palette); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	if obs.Kind == ObservedOther || (obs.Kind == ObservedNothing && r.manual.pending != nil) {
		r.manual.ObserveInstruction(legacyAt, v.SS(), v.SP(), v.Steps())
	}
	r.syncManual()
	if at == glyphEntry {
		ss, sp := v.SS(), v.SP()
		caller := r.norm.Addr(Address{Segment: v.Read16(linear(ss, sp+2)), Offset: v.Read16(linear(ss, sp))})
		var args [7]uint16
		for i := range args {
			args[i] = v.Read16(linear(ss, sp+4+uint16(i)*2))
		}
		steps := v.Steps()
		for _, l := range r.lanes {
			for _, f := range l.stories {
				if r.storyPending != nil {
					f.discontinuity()
				}
				f.glyphEntry(caller, ss, sp, args, steps)
			}
		}
		r.storyPending = &storyFrame{entryStep: steps, caller: caller, ss: ss, sp: sp}
		r.action.ObserveGlyphEntry(caller, ss, sp, args, steps)
	}
	if at == storyClearWrite {
		es, di, cx := v.ES(), v.DI(), v.CX()
		for _, l := range r.lanes {
			for _, f := range l.stories {
				f.clearWrite(at, es, di, cx)
			}
		}
		r.storyDirty = true
	}
	if r.storyDirty || r.storyPending != nil || at == glyphEntry {
		// Generations can only advance after a glyph entry/return, a clear
		// write or a page-9 pre-write; only then check, and read the palette
		// only for a family that actually has a new generation.
		if err := r.applyStories(v); err != nil {
			return err
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
		for _, l := range r.lanes {
			if err := l.actionApply(events[len(events)-1], request, palette); err != nil {
				return err
			}
		}
	}
	return nil
}

// observeEclText handles the text-window printer entry and its return for
// every lane whose watcher needs it; the entry is read once.
func (r *LiveRuntime) observeEclText(v StepReader, at Address) {
	if r.lanes[0].ecl == nil {
		return
	}
	if at == eclTextPrinter {
		ss, sp := v.SS(), v.SP()
		arg := func(i uint16) uint16 { return v.Read16(linear(ss, sp+i)) }
		base := linear(arg(6), arg(4))
		orig := make([]byte, v.Read8(base))
		for i := range orig {
			orig[i] = v.Read8(base + 1 + uint32(i))
		}
		ds := v.DS()
		ret := Address{Segment: arg(2), Offset: arg(0)}
		// Spec 038 §3.1–§3.2: only the ECL printer callers take a snapshot
		// and read the operand table, with the DS of this entry.
		var player *EclPlayerContext
		if caller := r.ovl.Key(v, ret); caller == eclNameCallerB79 || caller == eclNameCallerB4A {
			player = readEclPlayerContext(v, ds, caller, r.party.refresh(v, ds))
		}
		head := uint16(0)
		if r.anyLogbook() {
			head = v.Read16(BDAKeyHead)
		}
		e := EclTextEntry{
			Step: v.Steps(), SS: ss, SP: sp, Return: ret, Original: orig, Player: player,
			Clear: uint8(arg(8)) != 0, Background: uint8(arg(10)), Foreground: uint8(arg(12)),
			Bottom: uint8(arg(14)), Right: uint8(arg(16)), Top: uint8(arg(18)), Left: uint8(arg(20)),
			CursorCol: v.Read8(linear(ds, 0x5F3E)), CursorRow: v.Read8(linear(ds, 0x5F3F)),
		}
		for _, l := range r.lanes {
			if l.logbook != nil {
				l.logbook.ObserveEntryColors(orig, uint8(arg(20)), uint8(arg(18)), uint8(arg(16)), uint8(arg(14)), uint8(arg(10)), uint8(arg(12)))
				l.logbook.ArmKeyHead(head)
			}
			l.ecl.ObserveEntry(e)
		}
		return
	}
	ss, sp := uint16(0), uint16(0)
	read := false
	for _, l := range r.lanes {
		if l.ecl.InCall() {
			if !read {
				ss, sp, read = v.SS(), v.SP(), true
			}
			l.ecl.ObserveInstruction(at, ss, sp)
		}
	}
}

// observeHMenu reads the 37F1:0243 parent frame (spec 028 §2) once and
// feeds every lane whose watcher needs it.
func (r *LiveRuntime) observeHMenu(v StepReader, at Address) {
	if r.lanes[0].hmenu == nil {
		return
	}
	offset := at.Offset == hmenuPrinter.Offset
	need := offset
	for _, l := range r.lanes {
		need = need || l.hmenu.InCall()
	}
	if !need {
		return
	}
	ss, sp := v.SS(), v.SP()
	if !offset || r.ovl.Key(v, at) != hmenuPrinter {
		for _, l := range r.lanes {
			if offset || l.hmenu.InCall() {
				l.hmenu.ObserveInstruction(at, ss, sp)
			}
		}
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
	e := HMenuEntry{
		SS: ss, SP: sp, Return: Address{Segment: v.Read16(linear(ss, sp+2)), Offset: v.Read16(linear(ss, sp))},
		Text: text, Row: byteAt(-0x24A), Col: byteAt(-0x214), Items: items,
		Selected: uint8(v.Read16(linear(ss, sp+6))),
		Normal:   v.Read8(linear(ds, 0x6B46)), Hot: v.Read8(linear(ds, 0x6B47)),
	}
	for _, l := range r.lanes {
		l.hmenu.ObserveEntry(e)
	}
}

func (r *LiveRuntime) anyLogbook() bool {
	for _, l := range r.lanes {
		if l.logbook != nil {
			return true
		}
	}
	return false
}

// syncManual forwards new manual presentation events and style changes to
// every lane, only when something changed (the runner does it every step).
// Spec 040 §3.3: one lane's failure never skips another lane.
func (r *LiveRuntime) syncManual() {
	style, ok := r.manual.ManualStyle()
	n := len(r.manual.presentation)
	for _, l := range r.lanes {
		l.syncManual(style, ok, n)
	}
}

func postJoinRuntimeGate(e TextEvent) bool { return postJoinEntryRow(e.Caller, e.Row) }

// VideoWrite forwards an A000 pre-write to every family that invalidates on
// writes, in the runner's order, in every lane.
func (r *LiveRuntime) VideoWrite(w machine.VideoWrite) {
	for _, l := range r.lanes {
		if l.ecl != nil {
			l.ecl.ObserveVideoWrite(w.Offset)
		}
		if l.hmenu != nil {
			l.hmenu.ObserveVideoWrite(w.Offset)
		}
		if l.engDisp != nil {
			l.engDisp.ObserveVideoWrite(w.Offset)
		}
		if l.logbook != nil {
			l.logbook.ObserveVideoWrite(w.CS, w.IP, w.Offset)
		}
		for i := range liveScales {
			l.bodyPres[i].Prewrite(w)
		}
		for _, f := range l.stories {
			f.prewrite(w)
		}
	}
	r.storyDirty = true
	if r.postJoin.Active() {
		r.postJoin.Prewrite(w)
		for _, l := range r.lanes {
			for i := range liveScales {
				l.postPres[i].Prewrite(w)
			}
		}
	}
	for _, l := range r.lanes {
		l.ownersPrewrite(w)
	}
}

// Frame forwards one retrace to every lane, then runs the B-family sync of
// every lane (spec 040 §3.2 first time point).
func (r *LiveRuntime) Frame(indexed []byte, palette [256][3]uint8) {
	for _, l := range r.lanes {
		l.frame(indexed, palette)
	}
	r.syncB(palette)
	r.indexed, r.palette, r.hasFrame = indexed, palette, true
	r.frameSeen++
}

// syncB is the B-family sync of every lane (spec 040 §3.2: at each
// retrace and before ComposeWith, always for all enabled languages).
func (r *LiveRuntime) syncB(palette [256][3]uint8) {
	for _, l := range r.lanes {
		l.syncEclText(palette)
		l.syncHMenu(palette)
		l.syncEngineDispatch(palette)
		l.syncLogbook(palette)
	}
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
// English (spec 040 §3.4) is the original ScaleIndexedRGBA, byte for byte.
func (r *LiveRuntime) ComposeWith(indexed []byte, palette [256][3]uint8, scale int) ([]byte, bool, error) {
	saveIndexed, savePalette := r.indexed, r.palette
	r.indexed, r.palette = indexed, palette
	defer func() { r.indexed, r.palette = saveIndexed, savePalette }()
	// Generic families rebuild lazily; catch up with invalidations that
	// happened after the last retrace before drawing (every lane).
	r.syncB(palette)
	i := 0
	if scale == 3 {
		i = 1
	} else if scale != 2 {
		return nil, false, fmt.Errorf("buckrogers: 不支援 %d×", scale)
	}
	if r.cur < 0 {
		return ScaleIndexedRGBA(r.indexed, r.palette, scale), true, nil
	}
	out, err := r.lanes[r.cur].compose(r, i, scale)
	if err != nil {
		return nil, false, err
	}
	return out, true, nil
}

// PartyNames lists the latest accepted party snapshot as NAME/M|F|?
// (spec 038 §5.3); empty before the first accepted snapshot.
func (r *LiveRuntime) PartyNames() []string { return r.party.last.Names() }

// Frames counts retraces observed so far.
func (r *LiveRuntime) Frames() uint64 { return r.frameSeen }

// DebugSummary reports family counters for diagnostics (no original text;
// the spec-038 party list shows the player-entered names).  The shared
// counters and the zh-TW lane keep the established fields; spec 040 adds
// one lane[<lang>] block per other language and the languages left off.
func (r *LiveRuntime) DebugSummary() string {
	l0 := r.lanes[0]
	s := fmt.Sprintf("resets=%v orig-ascii=%v/%d orig-ascii-step=%d ovl-scans=%d ovl-ambiguous=%d", r.Resets(), r.asciiFound, r.asciiScans, r.asciiStep, r.ovl.Scans, r.ovl.Ambiguous)
	s += l0.halfFontError()
	if r.asciiStoryErrs != 0 {
		s += fmt.Sprintf(" orig-ascii-story-errs=%d", r.asciiStoryErrs)
	}
	s += l0.stats()
	// Spec 038 §5.3: the latest accepted party snapshot (player names).
	s += " party={" + r.party.summary() + "}"
	if l0.playersOff != "" {
		s += " 玩家名=off(" + l0.playersOff + ")"
	}
	switch {
	case r.manEng != nil:
		s += fmt.Sprintf(" 英文列=on(%d)", r.manEng.Len())
		if n := len(r.manEng.Excluded); n > 0 {
			reasons := map[string]int{}
			for _, why := range r.manEng.Excluded {
				reasons[why]++
			}
			s += fmt.Sprintf(" 英文列排除=%v", reasons)
		}
	case r.manEngOff != "":
		s += " 英文列=off(" + r.manEngOff + ")"
	}
	s += l0.counters()
	for _, l := range r.lanes[1:] {
		s += " lane[" + l.lang + "]={resets=" + fmt.Sprint(l.resets) + l.halfFontError() + l.counters() + l.stats()
		if l.playersOff != "" {
			s += " 玩家名=off(" + l.playersOff + ")"
		}
		s += "}"
	}
	if len(r.off) != 0 {
		keys := make([]string, 0, len(r.off))
		for k := range r.off {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var parts []string
		for _, k := range keys {
			parts = append(parts, k+"("+r.off[k]+")")
		}
		s += " 語言停用=" + strings.Join(parts, ",")
	}
	if len(r.lanes) > 1 || r.cur != 0 {
		s += " lang=" + r.Language()
	}
	return s
}

// SetManualEnglish loads the local spec-034 excerpt (zh-TW lane only). It
// must be called before the first step. A file-level problem turns the
// feature off with a reason code and is not an error for the caller.
func (r *LiveRuntime) SetManualEnglish(path string) error {
	if r.started {
		return errors.New("buckrogers: 英文列必須在第一個 step 之前設定")
	}
	if path == "" || r.manCatalog == nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		r.manEngOff = "read"
		return nil
	}
	panel, err := os.ReadFile(filepath.Join(r.textDir, "manual-english-panel.zh-TW.tsv"))
	if err != nil {
		r.manEngOff = "panel"
		return nil
	}
	l0 := r.lanes[0]
	// Spec 039: half-width characters are drawn with each scale's half font.
	fonts := []*xlate.Font{l0.manPres[0].font, l0.manPres[1].font, l0.manPres[0].half, l0.manPres[1].half}
	m, err := LoadManualEnglish(data, r.manCatalog, panel, fonts)
	if err != nil {
		r.manEngOff = strings.TrimPrefix(err.Error(), "manual-english: ")
		return nil
	}
	r.manEng = m
	return nil
}

// untouchedRows returns the page rows whose own cells still show the plain
// original frame in out (spec 029 §2.6.1).
func (r *LiveRuntime) untouchedRows(out []byte, scale int, p *HMenuPage) []HMenuRow {
	if p == nil {
		return nil
	}
	w := 320 * scale
	var keep []HMenuRow
	for _, row := range p.Rows {
		touched := false
		for y := int(row.Row) * 8 * scale; y < (int(row.Row)+1)*8*scale && !touched; y++ {
			for x := int(row.Col) * 8 * scale; x < (int(row.Col)+row.WidthCells())*8*scale && x < w; x++ {
				c := r.palette[r.indexed[(y/scale)*320+x/scale]]
				o := (y*w + x) * 4
				if out[o] != c[0] || out[o+1] != c[1] || out[o+2] != c[2] {
					touched = true
					break
				}
			}
		}
		if !touched {
			keep = append(keep, row)
		}
	}
	return keep
}

// rowsTouched reports whether out already differs from the plain scaled
// frame on any text row the menu page uses.
func (r *LiveRuntime) rowsTouched(out []byte, scale int, p *HMenuPage) bool {
	if p == nil {
		return false
	}
	// Spec 028 §3.4 (2026-09-27): only the family's own mask cells count,
	// every output pixel (no sampling), from the row's first item column.
	w := 320 * scale
	for _, row := range p.Rows {
		for oy := int(row.Row) * 8 * scale; oy < (int(row.Row)*8+8)*scale; oy++ {
			for ox := int(row.Col) * 8 * scale; ox < w; ox++ {
				c := r.palette[r.indexed[(oy/scale)*320+ox/scale]]
				o := (oy*w + ox) * 4
				if out[o] != c[0] || out[o+1] != c[1] || out[o+2] != c[2] {
					return true
				}
			}
		}
	}
	return false
}
