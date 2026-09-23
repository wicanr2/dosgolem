package buckrogers

// Exact, output-only watcher for the two skill-allocation exit questions.

import (
	"fmt"

	"github.com/wicanr2/dosgolem/internal/machine"
)

const skillExitRole = "exit_confirmation_prompt"

var skillExitEventHeader = []string{
	"event_key", "sequence", "event_role", "inference_level", "entry_step", "post_call_step",
	"original_length", "original_sha256", "caller", "background", "foreground", "row", "column",
}

type SkillExitPage uint8

const (
	SkillExitCareer SkillExitPage = iota + 1
	SkillExitTechnical
)

type skillExitEntry struct {
	eventKey string
	textKey  string
	text     string
	page     SkillExitPage
	id       menuIdentity
}

// SkillExitCatalog is immutable.  It deliberately accepts bytes from explicit
// caller-supplied paths; the app never assumes the enclosing project's layout.
type SkillExitCatalog struct {
	byIdentity map[menuIdentity]skillExitEntry
}

func LoadSkillExitCatalog(careerEvents, technicalEvents, translations []byte) (*SkillExitCatalog, error) {
	textsRows, err := readTSV("skill-exit-confirmation.zh-TW.tsv", translations, textHeader)
	if err != nil {
		return nil, err
	}
	texts := map[string]string{}
	for _, r := range textsRows {
		if r[0] == "" || r[1] == "" || texts[r[0]] != "" {
			return nil, fmt.Errorf("skill-exit translations: duplicate/empty key")
		}
		texts[r[0]] = r[1]
	}
	out := &SkillExitCatalog{byIdentity: map[menuIdentity]skillExitEntry{}}
	used := map[string]bool{}
	if err := out.addPage("career-skill-exit-events.tsv", careerEvents, SkillExitCareer, texts, used); err != nil {
		return nil, err
	}
	if err := out.addPage("technical-skill-exit-events.tsv", technicalEvents, SkillExitTechnical, texts, used); err != nil {
		return nil, err
	}
	if len(out.byIdentity) != 2 || len(used) != len(texts) {
		return nil, fmt.Errorf("skill-exit catalog has missing or orphan translations")
	}
	return out, nil
}

func (c *SkillExitCatalog) addPage(name string, data []byte, page SkillExitPage, texts map[string]string, used map[string]bool) error {
	rows, err := readTSV(name, data, skillExitEventHeader)
	if err != nil {
		return err
	}
	var found *skillExitEntry
	for _, r := range rows {
		if r[2] != skillExitRole {
			continue
		}
		if r[3] != "confirmed" || r[1] != "1" {
			return fmt.Errorf("%s: exit prompt evidence/order invalid", name)
		}
		length, e := menuByte(r[6])
		if e != nil {
			return fmt.Errorf("%s: invalid length", name)
		}
		hash, e := menuHash(r[7])
		if e != nil {
			return fmt.Errorf("%s: invalid sha", name)
		}
		caller, e := menuAddress(r[8])
		if e != nil {
			return fmt.Errorf("%s: invalid caller", name)
		}
		v := [4]uint8{}
		for i := range v {
			v[i], e = menuByte(r[9+i])
			if e != nil {
				return fmt.Errorf("%s: invalid style", name)
			}
		}
		id := menuIdentity{length, hash, caller, v[0], v[1], v[2], v[3]}
		text, ok := texts[r[0]]
		if !ok || used[r[0]] {
			return fmt.Errorf("%s: translation join invalid", name)
		}
		if found != nil {
			return fmt.Errorf("%s: expected one exit prompt", name)
		}
		found = &skillExitEntry{r[0], r[0], text, page, id}
	}
	if found == nil {
		return fmt.Errorf("%s: missing exit prompt", name)
	}
	if _, exists := c.byIdentity[found.id]; exists {
		return fmt.Errorf("skill-exit catalog: duplicate identity")
	}
	c.byIdentity[found.id] = *found
	used[found.textKey] = true
	return nil
}

type SkillExitGeneration struct {
	Generation            uint64
	Page                  SkillExitPage
	EventKey, Translation string
}
type skillExitState uint8

const (
	skillExitOpen skillExitState = iota
	skillExitClosed
	skillExitFailed
)

// SkillExitWatcher requires the exact entry followed by the TextRecorder's
// guarded-return event.  It owns no DOS state and cannot be revived after Stop.
type SkillExitWatcher struct {
	c            *SkillExitCatalog
	pending      *skillExitEntry
	pendingEvent TextEvent
	active       *SkillExitGeneration
	generation   uint64
	state        skillExitState
	reason       string
}

func NewSkillExitWatcher(c *SkillExitCatalog) (*SkillExitWatcher, error) {
	if c == nil || len(c.byIdentity) != 2 {
		return nil, fmt.Errorf("skill-exit catalog invalid")
	}
	return &SkillExitWatcher{c: c}, nil
}
func skillExitIdentity(e TextEvent) menuIdentity {
	return menuIdentity{e.OriginalLength, e.OriginalSHA256, e.Caller, e.Background, e.Foreground, e.Row, e.Column}
}
func (w *SkillExitWatcher) ObserveEntry(e TextEvent) error {
	if w == nil || w.state != skillExitOpen || w.pending != nil {
		return w.fail("entry state invalid")
	}
	if e.PostCallStep != 0 {
		return w.fail("entry already completed")
	}
	v, ok := w.c.byIdentity[skillExitIdentity(e)]
	if !ok {
		return w.fail("unknown exact entry")
	}
	w.pending, w.pendingEvent = &v, e
	return nil
}
func (w *SkillExitWatcher) ObserveReturn(e TextEvent) (SkillExitGeneration, error) {
	if w == nil || w.state != skillExitOpen || w.pending == nil || e.PostCallStep <= e.EntryStep {
		return SkillExitGeneration{}, w.fail("return state invalid")
	}
	if e.EntryStep != w.pendingEvent.EntryStep || skillExitIdentity(e) != skillExitIdentity(w.pendingEvent) {
		return SkillExitGeneration{}, w.fail("guarded return identity mismatch")
	}
	w.generation++
	g := SkillExitGeneration{w.generation, w.pending.page, w.pending.eventKey, w.pending.text}
	w.pending = nil
	w.active = &g
	return g, nil
}
func (w *SkillExitWatcher) ClearActive() {
	if w != nil {
		w.active = nil
	}
}
func (w *SkillExitWatcher) Stop() {
	if w != nil {
		w.pending = nil
		w.active = nil
		w.state = skillExitClosed
	}
}
func (w *SkillExitWatcher) Restore() {
	if w != nil && w.state == skillExitOpen {
		w.pending = nil
		w.active = nil
	}
}
func (w *SkillExitWatcher) Discontinuity() {
	if w != nil {
		w.pending = nil
		w.active = nil
		w.state = skillExitFailed
		w.reason = "execution discontinuity"
	}
}
func (w *SkillExitWatcher) Fault() {
	if w != nil {
		w.pending = nil
		w.active = nil
		w.state = skillExitFailed
		w.reason = "fault"
	}
}
func (w *SkillExitWatcher) fail(reason string) error {
	if w != nil {
		w.pending = nil
		w.active = nil
		if w.state == skillExitOpen {
			w.state = skillExitFailed
		}
		w.reason = reason
	}
	return fmt.Errorf("skill-exit fail-closed: %s", reason)
}
func (w *SkillExitWatcher) Active() bool {
	return w != nil && w.active != nil && w.state == skillExitOpen
}
func (w *SkillExitWatcher) Failed() bool { return w == nil || w.state == skillExitFailed }
func (w *SkillExitWatcher) Pending() bool {
	return w != nil && w.pending != nil && w.state == skillExitOpen
}

const skillExitVideoBase = 0xA0000

func NormalizeSkillExitVideoOffset(offset uint32) (uint32, error) {
	if offset >= 0x10000 {
		return 0, fmt.Errorf("A000 offset out of range: %#x", offset)
	}
	return skillExitVideoBase + offset, nil
}
func skillExitBody(page SkillExitPage) (uint32, uint32) {
	if page == SkillExitTechnical {
		return 0xF000, 0xF110
	}
	return 0xF000, 0xF108
}
func (w *SkillExitWatcher) Prewrite(v machine.VideoWrite) (bool, error) {
	if w == nil || w.state != skillExitOpen {
		return false, nil
	}
	off, err := NormalizeSkillExitVideoOffset(v.Offset)
	if err != nil {
		w.Fault()
		return false, err
	}
	if w.active == nil {
		return false, nil
	}
	lo, hi := skillExitBody(w.active.Page)
	local := off - skillExitVideoBase
	if local >= lo && local < hi {
		w.active = nil
		return true, nil
	}
	return false, nil
}
