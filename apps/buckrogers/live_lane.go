package buckrogers

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/xlate"
	"github.com/wicanr2/dosgolem/xlate/translit"
	"github.com/wicanr2/dosgolem/xlate/translitjk"
)

// liveLane is one language of spec 040 §3.2: the presenters of every A
// family at both scales, the owners that hold a watcher per scale (skill
// exit, post-join prompt, story page 9), the B-family watchers, the name
// data and the font.  All lanes see every observation; only the composed
// lane is drawn.
type liveLane struct {
	lang    string
	font    *xlate.Font
	textDir string // shared files
	langDir string // <family>.<lang>.tsv

	menuTexts map[string]string
	menuHdr   *HeaderColumns
	menuPres  [2]*RuntimeMenuOverlay

	skillCatalog *SkillExitCatalog
	skill        [2]*SkillExitOwner
	exitCatalog  *PostJoinExitPromptCatalog
	exit         [2]*PostJoinExitPromptOwner
	postCatalog  *PostJoinMenuCatalog
	postPres     [2]*RuntimePostJoinMenuOverlay
	actCatalog   *ActionBarRequestCatalog
	actPres      [2]*RuntimeActionBarOverlay
	bodyCatalog  *BodyIconCatalog
	bodyPres     [2]*RuntimeBodyIconOverlay
	manCatalog   *Catalog
	manPres      [2]*RuntimeManualOverlay
	manSync      [2]*ManualPresentationBridge
	manSeen      int
	manStyle     ManualTextStyle
	manHas       bool
	stories      []storyFamily

	// Spec 038 / 040 §3.1: per-language names; players is nil when the
	// transliterator of this language is not available.
	names      *NameGlossary
	players    *PlayerNames
	playersOff string

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
	// logbook is the spec-030 panel; nil without text/logbook.<lang>.tsv.
	logbook     *LogbookWatcher
	logbookPres [2]*LogbookOverlay

	// resets counts rebuilds after an error; rebuilds counts the normal
	// lifecycle rebuilds (obs.Dropped, spec 040 §3.3: not an error);
	// skips counts layers left out at draw time for a missing glyph.
	resets   map[string]int
	rebuilds map[string]int
	skips    map[string]int
}

// loadLane builds one language lane.  zh-TW is the reference (exact
// files); another language may leave rows out.  Every translated row is
// built once per scale before the lane is accepted (spec 040 §3.3).
func (r *LiveRuntime) loadLane(lang string, font *xlate.Font, langDir string) (*liveLane, error) {
	l := &liveLane{lang: lang, font: font, textDir: r.textDir, langDir: langDir,
		resets: map[string]int{}, rebuilds: map[string]int{}, skips: map[string]int{}}
	if lang != LangZhTW {
		// Spec 040 §3.3: the half-font derivation is part of the check.
		if h := halfFontsOf(font); h.Err != nil {
			return nil, fmt.Errorf("半形字型：%w", h.Err)
		}
	}
	file := func(fam string) ([]byte, error) {
		if lang == LangZhTW {
			b, err := os.ReadFile(filepath.Join(langDir, LangFile(fam, lang)))
			if err != nil {
				return nil, fmt.Errorf("buckrogers: 讀取 %s：%w", LangFile(fam, lang), err)
			}
			return b, nil
		}
		return readLangFile(langDir, fam, lang)
	}
	f := r.files
	var err error
	// B families first: the dispatcher part of the header list of a
	// non-reference language depends on its engine catalog.
	if err := l.loadEclText(r.menu.HeaderColumns()); err != nil {
		return nil, err
	}
	if err := l.loadHMenu(); err != nil {
		return nil, err
	}
	// Menu presenters.
	if l.menuTexts, err = r.menuShared.laneTexts(langDir, lang); err != nil {
		return nil, err
	}
	l.menuHdr = r.menuShared.headers
	if lang != LangZhTW {
		l.menuHdr = r.menuShared.headers.forLane(r.menuShared.full, l.menuTexts, nil)
	}
	for _, scale := range liveScales {
		if err := validateMenuLane(r.menuShared, l.menuTexts, l.menuHdr, font, scale); err != nil {
			return nil, err
		}
	}
	if err := l.resetMenu(r); err != nil {
		return nil, err
	}
	// Skill exit and post-join prompt: owners per scale.
	b, err := file("skill-exit-confirmation")
	if err != nil {
		return nil, err
	}
	if l.skillCatalog, err = LoadSkillExitCatalogLang(f["career-skill-exit-events.tsv"], f["technical-skill-exit-events.tsv"], b, lang); err != nil {
		return nil, err
	}
	if b, err = file("post-join-exit-prompt"); err != nil {
		return nil, err
	}
	if l.exitCatalog, err = LoadPostJoinExitPromptCatalogLang(f["post-join-exit-prompt-events.tsv"], b, lang); err != nil {
		return nil, err
	}
	for i := range liveScales {
		if err := l.resetSkill(i); err != nil {
			return nil, err
		}
		if err := l.resetExit(i); err != nil {
			return nil, err
		}
	}
	if err := l.validateOwners(); err != nil {
		return nil, err
	}
	// Post-join menu.
	if b, err = file("post-join-menu"); err != nil {
		return nil, err
	}
	if lang == LangZhTW {
		l.postCatalog = r.postShared
	} else if l.postCatalog, err = LoadPostJoinMenuCatalogLang(f["post-join-menu-events.tsv"], f["post-join-menu-variants.tsv"], b, lang); err != nil {
		return nil, err
	}
	if err := l.resetPost(); err != nil {
		return nil, err
	}
	for i := range liveScales {
		for sel := 0; sel < len(postJoinKeys); sel++ {
			p, err := NewRuntimePostJoinMenuOverlay(l.postCatalog, font, liveScales[i])
			if err != nil {
				return nil, err
			}
			if err := p.Apply(PostJoinMenuGeneration{Generation: 1, Selected: sel}, [256][3]uint8{}); err != nil {
				return nil, fmt.Errorf("buckrogers: 加入後選單（%d×）：%w", liveScales[i], err)
			}
		}
	}
	// Action bar.
	if b, err = file("skill-action-bar"); err != nil {
		return nil, err
	}
	if lang == LangZhTW {
		l.actCatalog = r.actRef
	} else if l.actCatalog, err = LoadActionBarRequestCatalogLang(f["skill-action-bar-events.tsv"], b, lang); err != nil {
		return nil, err
	}
	if err := l.resetAction(r); err != nil {
		return nil, err
	}
	for i := range liveScales {
		if err := l.actPres[i].validateAll(); err != nil {
			return nil, err
		}
	}
	// Body icon.
	if b, err = file("body-icon"); err != nil {
		return nil, err
	}
	if l.bodyCatalog, err = LoadBodyIconCatalogLang(f["body-icon-events.tsv"], f["body-icon-affixes.tsv"], b, f["body-icon-text-safe-rects.tsv"], lang); err != nil {
		return nil, err
	}
	for i, scale := range liveScales {
		if l.bodyPres[i], err = NewRuntimeBodyIconOverlay(l.bodyCatalog, font, scale, BodyIconLive); err != nil {
			return nil, err
		}
		if err := l.bodyPres[i].validateAll(); err != nil {
			return nil, err
		}
	}
	// Manual.
	if b, err = file("manual"); err != nil {
		return nil, err
	}
	if lang == LangZhTW {
		l.manCatalog = r.manCatalog
	} else if l.manCatalog, err = LoadCatalogLang(f["manual-events.tsv"], f["manual-ordinals.tsv"], b, lang); err != nil {
		return nil, err
	}
	for i, scale := range liveScales {
		if l.manPres[i], err = NewRuntimeManualOverlayLang(r.manLayout, l.manCatalog, font, scale, lang); err != nil {
			return nil, err
		}
		consumer, err := NewManualPresentationConsumer(l.manPres[i])
		if err != nil {
			return nil, err
		}
		if l.manSync[i], err = NewManualPresentationBridge(r.manual, consumer); err != nil {
			return nil, err
		}
	}
	// Story pages.
	if l.stories, err = storyPagesLang(r.textDir, langDir, lang, font); err != nil {
		return nil, err
	}
	return l, nil
}

// validateOwners prebuilds both skill-exit questions and both post-join
// prompt bodies at both scales (spec 040 §3.3).
func (l *liveLane) validateOwners() error {
	for _, scale := range liveScales {
		for _, e := range l.skillCatalog.byIdentity {
			if e.text == "" {
				continue
			}
			p, err := NewRuntimeSkillExitOverlay(l.font, scale)
			if err != nil {
				return err
			}
			if err := p.Apply(SkillExitGeneration{Generation: 1, Page: e.page, EventKey: e.eventKey, Translation: e.text}, [256][3]uint8{}); err != nil {
				return fmt.Errorf("buckrogers: 技能離開 %s（%d×）：%w", e.eventKey, scale, err)
			}
		}
		for _, e := range l.exitCatalog.byIdentity {
			if e.text == "" {
				continue
			}
			p, err := NewRuntimePostJoinExitPromptOverlay(l.font, scale)
			if err != nil {
				return err
			}
			if err := p.Apply(PostJoinExitPromptGeneration{Generation: 1, EventKey: e.key, Translation: e.text, Width: e.width}, [256][3]uint8{}); err != nil {
				return fmt.Errorf("buckrogers: 加入後提示 %s（%d×）：%w", e.key, scale, err)
			}
		}
	}
	return nil
}

func (l *liveLane) resetMenu(r *LiveRuntime) error {
	var pres [2]*RuntimeMenuOverlay
	for i, scale := range liveScales {
		p, err := NewRuntimeMenuOverlay(r.menuShared.rects, l.font, scale)
		if err != nil {
			return err
		}
		p.SetHeaderColumns(l.menuHdr)
		p.SetTexts(l.menuTexts)
		pres[i] = p
	}
	l.menuPres = pres
	return nil
}

func (l *liveLane) resetSkill(i int) error {
	o, err := NewSkillExitOwner(l.skillCatalog, l.font, liveScales[i])
	if err == nil {
		l.skill[i] = o
	}
	return err
}

func (l *liveLane) resetExit(i int) error {
	o, err := NewPostJoinExitPromptOwner(l.exitCatalog, l.font, liveScales[i])
	if err == nil {
		l.exit[i] = o
	}
	return err
}

func (l *liveLane) resetPost() error {
	var pres [2]*RuntimePostJoinMenuOverlay
	for i, scale := range liveScales {
		p, err := NewRuntimePostJoinMenuOverlay(l.postCatalog, l.font, scale)
		if err != nil {
			return err
		}
		pres[i] = p
	}
	l.postPres = pres
	return nil
}

func (l *liveLane) resetAction(r *LiveRuntime) error {
	var pres [2]*RuntimeActionBarOverlay
	for i, scale := range liveScales {
		p, err := NewRuntimeActionBarOverlayLang(l.actCatalog, r.actRef, r.actRects, l.font, scale, l.lang == LangZhTW)
		if err != nil {
			return err
		}
		pres[i] = p
	}
	l.actPres = pres
	return nil
}

// recover counts an error and rebuilds this lane's family only.  A rebuild
// failure is a programming error in catalog loading and is returned.
func (l *liveLane) recover(family string, rebuild func() error) error {
	l.resets[family]++
	return rebuild()
}

// rebuild is a normal lifecycle rebuild (obs.Dropped): not an error.
func (l *liveLane) rebuild(family string, rebuild func() error) error {
	l.rebuilds[family]++
	return rebuild()
}

// yieldMenu lets the newest family writer win: the original just drew these
// rectangles, so any menu stamp of this lane there is already stale.
func (l *liveLane) yieldMenu(rects []PixelRect) {
	for _, rect := range rects {
		for i := range liveScales {
			l.menuPres[i].clearRect(rect.X, rect.Y, rect.X+rect.Width, rect.Y+rect.Height)
		}
	}
}

func (l *liveLane) menuClear(c [4]uint8) {
	for i := range liveScales {
		if err := l.menuPres[i].ClearTextCells(c[0], c[1], c[2], c[3]); err != nil {
			l.resets["menu"]++
			_ = l.resetMenuSelf()
			return
		}
	}
}

func (l *liveLane) menuApply(e TextEvent, req DisplayRequest, palette [256][3]uint8) {
	for i := range liveScales {
		if err := l.menuPres[i].Apply(e, req, palette); err != nil {
			l.resets["menu"]++
			_ = l.resetMenuSelf()
			return
		}
	}
}

// resetMenuSelf rebuilds the menu presenters from what the lane already holds.
func (l *liveLane) resetMenuSelf() error {
	for i, scale := range liveScales {
		old := l.menuPres[i]
		p, err := NewRuntimeMenuOverlay(old.rects, l.font, scale)
		if err != nil {
			return err
		}
		p.SetHeaderColumns(l.menuHdr)
		p.SetTexts(l.menuTexts)
		l.menuPres[i] = p
	}
	return nil
}

func (l *liveLane) actionClear(c [4]uint8) {
	for i := range liveScales {
		if err := l.actPres[i].ClearTextCells(c[0], c[1], c[2], c[3]); err != nil {
			l.resets["action-bar"]++
			_ = l.resetActionSelf()
			return
		}
	}
}

func (l *liveLane) actionApply(e ActionBarEvent, req DisplayRequest, palette [256][3]uint8) error {
	for i := range liveScales {
		if err := l.actPres[i].Apply(e, req, palette); err != nil {
			l.resets["action-bar"]++
			return l.resetActionSelf()
		}
	}
	return nil
}

func (l *liveLane) resetActionSelf() error {
	for i, scale := range liveScales {
		old := l.actPres[i]
		p, err := NewRuntimeActionBarOverlayLang(l.actCatalog, old.reference(), old.rects, l.font, scale, l.lang == LangZhTW)
		if err != nil {
			return err
		}
		l.actPres[i] = p
	}
	return nil
}

// ownersDropped rebuilds a pending owner whose dispatcher frame the
// recorder dropped (normal lifecycle, spec 040 §3.3).
func (l *liveLane) ownersDropped(dropped bool) error {
	if !dropped {
		return nil
	}
	for i := range liveScales {
		if l.skill[i].Watcher.Pending() {
			if err := l.rebuild("skill-exit", func() error { return l.resetSkill(i) }); err != nil {
				return err
			}
		}
		if l.exit[i].Watcher.Pending() {
			if err := l.rebuild("exit-prompt", func() error { return l.resetExit(i) }); err != nil {
				return err
			}
		}
	}
	return nil
}

func (l *liveLane) ownersEntry(e TextEvent, row24 bool) error {
	for i := range liveScales {
		if err := l.skill[i].ObserveEntry(e); err != nil {
			if err := l.recover("skill-exit", func() error { return l.resetSkill(i) }); err != nil {
				return err
			}
		}
	}
	if row24 {
		for i := range liveScales {
			if l.exit[i].Watcher.ShouldObserveEntry(e) {
				if err := l.exit[i].ObserveEntry(e); err != nil {
					if err := l.recover("exit-prompt", func() error { return l.resetExit(i) }); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (l *liveLane) ownersReturn(obs *StepObservation, palette [256][3]uint8) error {
	for i := range liveScales {
		if l.skill[i].Watcher.Pending() && obs.Dropped {
			if err := l.rebuild("skill-exit", func() error { return l.resetSkill(i) }); err != nil {
				return err
			}
			continue
		}
		if l.skill[i].Watcher.Pending() && obs.NewEvent {
			if err := l.skill[i].ObserveReturn(obs.Event, palette); err != nil {
				if err := l.recover("skill-exit", func() error { return l.resetSkill(i) }); err != nil {
					return err
				}
			} else if i == 0 {
				l.yieldMenu(withRect(l.skill[0].Presenter.LayerRects(), l.skill[0].MissingRect()))
			}
		}
	}
	for i := range liveScales {
		if l.exit[i].Watcher.Pending() && obs.Dropped {
			if err := l.rebuild("exit-prompt", func() error { return l.resetExit(i) }); err != nil {
				return err
			}
			continue
		}
		if l.exit[i].Watcher.Pending() && obs.NewEvent {
			if err := l.exit[i].ObserveReturn(obs.Event, palette); err != nil {
				if err := l.recover("exit-prompt", func() error { return l.resetExit(i) }); err != nil {
					return err
				}
			} else if i == 0 {
				l.yieldMenu(withRect(l.exit[0].Presenter.LayerRects(), l.exit[0].MissingRect()))
			}
		}
	}
	return nil
}

func withRect(rects []PixelRect, extra *PixelRect) []PixelRect {
	if extra != nil {
		rects = append(rects, *extra)
	}
	return rects
}

func (l *liveLane) ownersPrewrite(w machine.VideoWrite) {
	for i := range liveScales {
		if err := l.skill[i].Prewrite(w); err != nil {
			_ = l.recover("skill-exit", func() error { return l.resetSkill(i) })
		}
		if err := l.exit[i].Prewrite(w); err != nil {
			_ = l.recover("exit-prompt", func() error { return l.resetExit(i) })
		}
	}
}

func (l *liveLane) bodyApply(t BodyIconTransition, palette [256][3]uint8) {
	for i := range liveScales {
		if err := l.bodyPres[i].Apply(t, palette); err != nil {
			l.resets["body-icon"]++
		}
	}
	l.yieldMenu(append(l.bodyPres[0].SafeLogicalRects(), l.bodyPres[0].MissingRects()...))
}

func (l *liveLane) postApply(g PostJoinMenuGeneration, palette [256][3]uint8) error {
	for i := range liveScales {
		if err := l.postPres[i].Apply(g, palette); err != nil {
			// Spec 040 §3.3: only this lane's presenters are rebuilt.
			return l.recover("post-join", l.resetPost)
		}
	}
	l.yieldMenu(append(l.postPres[0].LayerRects(), l.postPres[0].MissingRects()...))
	return nil
}

func (l *liveLane) syncManual(style ManualTextStyle, ok bool, n int) {
	if ok && (!l.manHas || style != l.manStyle) {
		for i := range liveScales {
			if err := l.manPres[i].SetStyle(style); err != nil {
				l.resets["manual"]++
				return
			}
		}
		l.manStyle, l.manHas = style, true
	}
	if n != l.manSeen {
		l.manSeen = n
		for i := range liveScales {
			if _, err := l.manSync[i].Sync(); err != nil {
				l.resets["manual"]++
			}
		}
	}
}

func (l *liveLane) frame(indexed []byte, palette [256][3]uint8) {
	for i := range liveScales {
		l.menuPres[i].Frame(indexed, palette)
	}
	for _, f := range l.stories {
		f.frame(indexed, palette)
	}
	for i := range liveScales {
		l.manPres[i].Frame(indexed, palette)
	}
	for i := range liveScales {
		l.actPres[i].Frame(indexed, palette)
	}
}

// genericActive reports whether a spec 027–030 family has content to draw.
func (l *liveLane) genericActive() bool {
	return l.ecl != nil && len(l.ecl.Pages()) != 0 || l.hmenu != nil && l.hmenu.Page() != nil ||
		l.engDisp != nil && len(l.engDisp.Lines()) != 0 || l.logbook != nil && l.logbook.open != 0
}

// missing counts requests the lane left to the original English.
func (l *liveLane) missing() int {
	n := 0
	for i := range liveScales {
		if l.menuPres[i] != nil {
			n += l.menuPres[i].Missing
		}
		if l.skill[i] != nil {
			n += l.skill[i].Missing
		}
		if l.exit[i] != nil {
			n += l.exit[i].Missing
		}
		if l.postPres[i] != nil {
			n += l.postPres[i].Missing
		}
		if l.actPres[i] != nil {
			n += l.actPres[i].Missing
		}
		if l.bodyPres[i] != nil {
			n += l.bodyPres[i].Missing
		}
		if l.manPres[i] != nil {
			n += l.manPres[i].Missing
		}
	}
	return n
}

// stats is the lane part of DebugSummary (the B-family counters).
func (l *liveLane) stats() string {
	s := ""
	if l.ecl != nil {
		s += fmt.Sprintf(" ecl=%+v", l.ecl.Stats)
	}
	if l.hmenu != nil {
		s += fmt.Sprintf(" hmenu=%+v", l.hmenu.Stats)
	}
	if l.engDisp != nil {
		s += fmt.Sprintf(" engine-dispatch=%+v", l.engDisp.Stats)
		if ps := l.engDisp.PartyStats; ps.Extended != 0 || ps.ChineseOnly != 0 {
			s += fmt.Sprintf(" party-panel=%+v", ps)
		}
		if hs := l.engDisp.HeaderStats; hs.Anchored != 0 || hs.Mismatches != 0 {
			s += fmt.Sprintf(" header-columns=%+v", hs)
		}
	}
	return s
}

// halfFontError is the spec 039 §3.2 diagnostic: a base font that fails
// the half-width ink check leaves the half fonts empty.
func (l *liveLane) halfFontError() string {
	if h := halfFontsOf(l.font); h.Err != nil {
		return " half-font-error=" + h.Err.Error()
	}
	return ""
}

// counters are the spec 040 lane counters: lifecycle rebuilds, draw-time
// skips and untranslated requests (printed only when non-zero).
func (l *liveLane) counters() string {
	s := ""
	if len(l.rebuilds) != 0 {
		s += fmt.Sprintf(" rebuilds=%v", l.rebuilds)
	}
	if len(l.skips) != 0 {
		s += fmt.Sprintf(" skips=%v", l.skips)
	}
	if n := l.missing(); n != 0 {
		s += fmt.Sprintf(" missing=%d", n)
	}
	return s
}

// --- B families --------------------------------------------------------------

// loadEclText wires the spec-027 family when the catalog exists; a missing
// translation file means nothing is translated yet.
func (l *liveLane) loadEclText(headers *HeaderColumns) error {
	events, err := os.ReadFile(filepath.Join(l.textDir, "ecl-text-events.tsv"))
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	tr, err := os.ReadFile(filepath.Join(l.langDir, LangFile("ecl-text", l.lang)))
	if os.IsNotExist(err) {
		tr = headerOnly()
	} else if err != nil {
		return err
	}
	c, err := LoadEclTextCatalogLang(events, tr, l.lang)
	if err != nil {
		return err
	}
	names, err := loadNameGlossaryLang(l.textDir, l.langDir, l.lang)
	if err != nil {
		return err
	}
	l.names = names
	l.ecl = NewEclTextWatcher(c)
	l.ecl.SetNames(names)
	l.ecl.SetLayout(LayoutFor(l.lang))
	// Spec 038: without the transliterator data the player-name display is
	// off (names stay English); the runtime still starts.  Spec 040 §3.1:
	// the transliterators of other languages come with their own specs;
	// spec 041 §3.6: zh-CN wraps the zh-TW transliterator with the shared
	// character map, and a map that fails to load only turns zh-CN player
	// names off.
	switch l.lang {
	case LangZhTW:
		if tr, err := translit.Load(l.textDir); err != nil {
			l.playersOff = err.Error()
		} else {
			l.players = NewPlayerNames(tr, names)
			l.ecl.SetPlayerNames(l.players)
		}
	case LangZhCN:
		if tr, err := translit.Load(l.textDir); err != nil {
			l.playersOff = err.Error()
		} else if b, err := os.ReadFile(filepath.Join(l.textDir, TranslitMapFile(l.lang))); err != nil {
			l.playersOff = "translit-map: " + err.Error()
		} else if m, err := LoadTranslitCharMap(TranslitMapFile(l.lang), b); err != nil {
			l.playersOff = "translit-map: " + err.Error()
		} else {
			l.players = NewPlayerNames(&charMapTransliterator{inner: tr, chars: m}, names)
			l.ecl.SetPlayerNames(l.players)
		}
	case LangJa, LangKo:
		// Specs 044, 045: the katakana and Hangul transliterator.  A font
		// that lacks a character the rules can emit turns the player names
		// off at load time instead of drawing a hole in a name.
		if tr, err := translitjk.Load(l.textDir, l.lang); err != nil {
			l.playersOff = "translit-" + l.lang + ": " + err.Error()
		} else if missing := fontLacksRunes(l.font, tr.Allowed()); len(missing) > 0 {
			l.playersOff = fmt.Sprintf("translit-font: 缺 U+%04X 等 %d 字", missing[0], len(missing))
		} else {
			l.players = NewPlayerNames(tr, names)
			l.ecl.SetPlayerNames(l.players)
		}
	default:
		l.playersOff = "no-transliterator"
	}
	l.eclGen = l.ecl.Generation()
	if eng, err := loadEngineTextLang(l.textDir, l.langDir, l.lang); err != nil {
		return err
	} else if eng != nil {
		l.ecl.SetEngine(eng)
		if err := l.loadEngineDispatch(eng, headers); err != nil {
			return err
		}
		if l.engDisp != nil {
			l.engDisp.SetEclCatalog(c)
			l.engDisp.SetPlayerNames(l.players)
		}
		if err := l.loadLogbook(eng, names); err != nil {
			return err
		}
	}
	for i, scale := range liveScales {
		if l.eclPres[i], err = NewEclTextOverlay(l.font, scale); err != nil {
			return err
		}
	}
	return nil
}

func (l *liveLane) loadHMenu() error {
	events, err := os.ReadFile(filepath.Join(l.textDir, "hmenu-item-events.tsv"))
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	tr, err := os.ReadFile(filepath.Join(l.langDir, LangFile("hmenu", l.lang)))
	if os.IsNotExist(err) {
		tr = headerOnly()
	} else if err != nil {
		return err
	}
	c, err := LoadHMenuCatalogLang(events, tr, l.lang)
	if err != nil {
		return err
	}
	l.hmenu = NewHMenuWatcher(c)
	l.hmenuGen = l.hmenu.Generation()
	for i, scale := range liveScales {
		if l.hmenuPres[i], err = NewHMenuOverlay(l.font, scale); err != nil {
			return err
		}
	}
	return nil
}

// loadEngineTextLang loads the spec-029 catalogs when the fragment catalog
// exists; missing translation files mean nothing is translated yet.
func loadEngineTextLang(textDir, langDir, lang string) (*EngineTextCatalog, error) {
	read := func(dir, name string, required bool) ([]byte, error) {
		b, err := os.ReadFile(filepath.Join(dir, name))
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
	f := EngineTextFiles{Lang: lang}
	f.FragmentEvents = fe
	for _, x := range []struct {
		dst  *[]byte
		dir  string
		name string
		req  bool
	}{
		{&f.FragmentText, langDir, LangFile("engine-fragment", lang), false},
		{&f.ItemEvents, textDir, "item-word-events.tsv", true},
		{&f.ItemText, langDir, LangFile("item-word", lang), false},
		{&f.PhraseEvents, textDir, "item-phrase-events.tsv", false},
		{&f.PhraseText, langDir, LangFile("item-phrase", lang), false},
		{&f.TemplateEvents, textDir, "engine-template-events.tsv", false},
		{&f.TemplateText, langDir, LangFile("engine-template", lang), false},
		{&f.MonsterEvents, textDir, "monster-name-events.tsv", false},
		{&f.MonsterText, langDir, LangFile("monster-name", lang), false},
	} {
		if *x.dst, err = read(x.dir, x.name, x.req); err != nil {
			return nil, err
		}
	}
	c, err := LoadEngineTextCatalog(f)
	if err != nil {
		return nil, err
	}
	if cb, err := read(langDir, LangFile("coordinate-line", lang), false); err != nil {
		return nil, err
	} else if cb != nil {
		if err := c.LoadCoordinateTextLang(cb, lang); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func (l *liveLane) loadEngineDispatch(eng *EngineTextCatalog, headers *HeaderColumns) error {
	textDir := l.textDir
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
	l.engDisp = NewEngineDispatchWatcher(eng, allow)
	l.engDisp.SetLayout(LayoutFor(l.lang))
	h := headers
	if l.lang != LangZhTW {
		h = headers.forLane(nil, nil, eng)
	}
	if err := l.engDisp.SetHeaderColumns(h); err != nil {
		return err
	}
	if nb, err := os.ReadFile(filepath.Join(textDir, "engine-dispatch-name-callers.tsv")); err == nil {
		names, err := LoadEngineDispatchCallers(nb, nil)
		if err != nil {
			return err
		}
		l.engDisp.SetNameCallers(names)
	} else if !os.IsNotExist(err) {
		return err
	}
	if sb, err := os.ReadFile(filepath.Join(textDir, "engine-dispatch-shared-callers.tsv")); err == nil {
		shared, err := LoadEngineDispatchCallers(sb, nil)
		if err != nil {
			return err
		}
		if err := l.engDisp.SetSharedCallers(shared, OwnedCallers(files)); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	l.engDispGen = l.engDisp.Generation()
	for i, scale := range liveScales {
		if l.engDispPres[i], err = NewHMenuOverlay(l.font, scale); err != nil {
			return err
		}
	}
	return nil
}

func (l *liveLane) loadLogbook(eng *EngineTextCatalog, names *NameGlossary) error {
	b, err := os.ReadFile(filepath.Join(l.langDir, LangFile("logbook", l.lang)))
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	c, err := LoadLogbookCatalogLang(b, names, l.lang)
	if err != nil {
		return err
	}
	if pb, err := os.ReadFile(filepath.Join(l.langDir, LangFile("logbook-panel", l.lang))); err == nil {
		if err := c.LoadLogbookPanelTextLang(pb, l.lang); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	l.logbook = NewLogbookWatcher(c, eng)
	for i, scale := range liveScales {
		if l.logbookPres[i], err = NewLogbookOverlay(l.font, scale); err != nil {
			return err
		}
	}
	return nil
}

// syncHMenu, syncEngineDispatch, syncEclText and syncLogbook rebuild this
// lane's presenters after a generation change.  A page the font cannot draw
// is dropped (spec 040 §3.2: only this lane's watcher sees the discontinuity
// and only this lane's presenters are cleared).
func (l *liveLane) syncHMenu(palette [256][3]uint8) {
	if l.hmenu == nil {
		return
	}
	if g := l.hmenu.Generation(); g != l.hmenuGen {
		p := l.hmenu.Page()
		for i := range liveScales {
			if miss := l.hmenuPres[i].Sync(p, g, palette); len(miss) != 0 {
				l.resets["hmenu"]++
				l.hmenu.ObserveDiscontinuity()
				g = l.hmenu.Generation()
				for j := range liveScales {
					l.hmenuPres[j].Sync(nil, g, palette)
				}
				break
			}
		}
		l.hmenuGen = g
	}
	for i := range liveScales {
		l.hmenuPres[i].Frame(palette)
	}
}

func (l *liveLane) syncLogbook(palette [256][3]uint8) {
	if l.logbook == nil {
		return
	}
	for i := range liveScales {
		if miss := l.logbookPres[i].Sync(l.logbook, palette); len(miss) != 0 {
			l.resets["logbook"]++
			l.logbook.ObserveDiscontinuity()
			for j := range liveScales {
				l.logbookPres[j].Sync(l.logbook, palette)
			}
			break
		}
	}
}

func (l *liveLane) syncEngineDispatch(palette [256][3]uint8) {
	if l.engDisp == nil {
		return
	}
	if g := l.engDisp.Generation(); g != l.engDispGen {
		p := l.engDisp.Page()
		for i := range liveScales {
			if miss := l.engDispPres[i].Sync(p, g, palette); len(miss) != 0 {
				l.resets["engine-dispatch"]++
				l.engDisp.ObserveDiscontinuity()
				g = l.engDisp.Generation()
				for j := range liveScales {
					l.engDispPres[j].Sync(nil, g, palette)
				}
				break
			}
		}
		l.engDispGen = g
	}
	for i := range liveScales {
		l.engDispPres[i].Frame(palette)
	}
}

func (l *liveLane) syncEclText(palette [256][3]uint8) {
	if l.ecl == nil {
		return
	}
	if g := l.ecl.Generation(); g != l.eclGen {
		p := l.ecl.Pages()
		for i := range liveScales {
			if miss := l.eclPres[i].Sync(p, g, palette); len(miss) != 0 {
				l.resets["ecl-text"]++
				l.ecl.ObserveDiscontinuity()
				g = l.ecl.Generation()
				for j := range liveScales {
					l.eclPres[j].Sync(nil, g, palette)
				}
				break
			}
		}
		l.eclGen = g
	}
	for i := range liveScales {
		l.eclPres[i].Frame(palette)
	}
}

// compose draws this lane at scale index i.  Spec 040 §3.2: a layer with
// a missing glyph is left out and counted; the menu family is the base, so
// its missing glyph makes the base the original frame.
func (l *liveLane) compose(r *LiveRuntime, i, scale int) ([]byte, error) {
	out, missing, _ := l.menuPres[i].Draw(r.indexed, r.palette)
	if len(missing) != 0 {
		l.skips["menu"]++
		out = ScaleIndexedRGBA(r.indexed, r.palette, scale)
	}
	var base []byte
	layer := func(family string, rgba []byte, missing []rune) {
		if len(missing) != 0 {
			l.skips[family]++
			return
		}
		if base == nil {
			base = ScaleIndexedRGBA(r.indexed, r.palette, scale)
		}
		for p := 0; p < len(base); p += 4 {
			if rgba[p] != base[p] || rgba[p+1] != base[p+1] || rgba[p+2] != base[p+2] || rgba[p+3] != base[p+3] {
				copy(out[p:p+4], rgba[p:p+4])
			}
		}
	}
	if len(l.skill[i].Presenter.ActiveKeys()) != 0 {
		rgba, missing, _ := l.skill[i].Presenter.Draw(r.indexed, r.palette)
		layer("skill-exit", rgba, missing)
	}
	if len(l.exit[i].Presenter.ActiveKeys()) != 0 {
		rgba, missing, _, err := l.exit[i].Presenter.Draw(r.indexed, r.palette)
		if err != nil {
			return nil, err
		}
		layer("exit-prompt", rgba, missing)
	}
	if len(l.bodyPres[i].ActiveKeys()) != 0 {
		rgba, missing, _ := l.bodyPres[i].Draw(r.indexed, r.palette)
		layer("body-icon", rgba, missing)
	}
	if len(l.manPres[i].ActiveKeys()) != 0 {
		rgba, missing, _ := l.manPres[i].Draw(r.indexed, r.palette)
		layer("manual", rgba, missing)
	}
	// Spec 034: the local English excerpt panel is shown with zh-TW only.
	if l.lang == LangZhTW && r.manEng != nil && r.manEngPres[i].sync(r.manEng, l.manPres[i], r.palette) {
		rgba := ScaleIndexedRGBA(r.indexed, r.palette, scale)
		missing := []rune{}
		r.manEngPres[i].layer.Draw(rgba, scale, func(ch rune) { missing = append(missing, ch) })
		layer("manual-english", rgba, missing)
	}
	for _, f := range l.stories {
		if rgba, missing, active := f.draw(i, r.indexed, r.palette); active {
			layer(f.name(), rgba, missing)
		}
	}
	if len(l.actPres[i].ActiveKeys()) != 0 {
		rgba, missing, _ := l.actPres[i].Draw(r.indexed, r.palette)
		layer("action-bar", rgba, missing)
	}
	if len(l.postPres[i].ActiveKeys()) != 0 {
		rgba, missing, _ := l.postPres[i].Draw(r.indexed, r.palette)
		layer("post-join", rgba, missing)
	}
	if l.ecl != nil && l.eclPres[i].Active() {
		rgba, missing := l.eclPres[i].Draw(r.indexed, r.palette)
		layer("ecl-text", rgba, missing)
	}
	// Spec 028: families with their own reviewed catalogs own their rows; the
	// generic menu family draws only rows nobody else changed.
	if l.hmenu != nil && l.hmenuPres[i].Active() && !r.rowsTouched(out, scale, l.hmenu.Page()) {
		rgba, missing := l.hmenuPres[i].Draw(r.indexed, r.palette)
		layer("hmenu", rgba, missing)
	}
	// Spec 029 §2.6.1: engine lines yield row by row, judged only on the
	// row's own cells; any pixel another family changed there counts.
	if l.engDisp != nil && l.engDispPres[i].Active() {
		if keep := r.untouchedRows(out, scale, l.engDisp.Page()); len(keep) != 0 {
			rgba, missing := l.engDispPres[i].Draw(r.indexed, r.palette)
			if len(missing) != 0 {
				l.skips["engine-dispatch"]++
			} else {
				w := 320 * scale
				for _, row := range keep {
					// Spec 039 §3.5: the overlay's own cells (incl. 038 extension).
					for y := int(row.Row) * 8 * scale; y < (int(row.Row)+1)*8*scale; y++ {
						for x := int(row.Col) * 8 * scale; x < (int(row.Col)+row.WidthCells())*8*scale && x < w; x++ {
							o := 4 * (y*w + x)
							c := r.palette[r.indexed[(y/scale)*320+x/scale]]
							if rgba[o] != c[0] || rgba[o+1] != c[1] || rgba[o+2] != c[2] {
								copy(out[o:o+4], rgba[o:o+4])
							}
						}
					}
				}
			}
		}
	}
	// The logbook panel is drawn last: it sits over everything while open.
	// It is opaque: pixels other families changed inside its rectangle must
	// not show through, so the rectangle is copied whole.
	if l.logbook != nil && l.logbookPres[i].Active() {
		rgba, missing := l.logbookPres[i].Draw(r.indexed, r.palette)
		if len(missing) != 0 {
			l.skips["logbook"]++
		} else {
			x0, y0, x1, y1 := l.logbookPres[i].Rect()
			w := 320 * scale
			for y := y0 * scale; y < y1*scale; y++ {
				copy(out[4*(y*w+x0*scale):4*(y*w+x1*scale)], rgba[4*(y*w+x0*scale):4*(y*w+x1*scale)])
			}
		}
	}
	return out, nil
}

// loadEngineText is loadEngineTextLang for the zh-TW reference.
func loadEngineText(textDir string) (*EngineTextCatalog, error) {
	return loadEngineTextLang(textDir, textDir, LangZhTW)
}

// fontLacksRunes lists the runes (ascending, as given) the font has no glyph for.
func fontLacksRunes(f *xlate.Font, runes []rune) []rune {
	var out []rune
	for _, r := range runes {
		if _, ok := f.Glyphs[r]; !ok {
			out = append(out, r)
		}
	}
	return out
}
