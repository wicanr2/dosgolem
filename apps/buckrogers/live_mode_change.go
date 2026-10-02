package buckrogers

import (
	"sort"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// Buck repo spec 052: a video mode set (int 10h AH=00) is a discontinuity of
// the whole display that no A000 write observer can see.  Every overlay family
// of every lane forgets what it shows, without failing, without counting a
// reset and without touching a family that is empty (an empty family changes no
// counter, no generation and no statistic).  Families whose state machines
// carry a monotonic generation (body icon, manual) are reset through new
// narrow entry points that keep it.

// Reset forgets the in-flight collection and the proven order; the generation
// keeps counting so the presenters (which require a larger one) accept the next
// transition.
func (w *LiveBodyIconWatcher) Reset() {
	if w != nil {
		w.state, w.collecting = "", nil
	}
}

// ClearAll removes everything drawn and every safe rectangle; generation,
// transition and the invalidation record stay (no invalidation is recorded:
// nothing was written to the rectangles).
func (o *RuntimeBodyIconOverlay) ClearAll() {
	if o == nil {
		return
	}
	o.layer.Clear(0, 0, 320, 200)
	o.active = map[string]bool{}
	o.rects = map[string]PixelRect{}
	o.missing = nil
}

// ResetDisplay is the default anchor branch of ObserveAnchorEvent under a
// name: no screen is known, nothing is collected.
func (w *ActionBarWatcher) ResetDisplay() {
	if w != nil {
		w.pending = nil
		w.collector.clear()
	}
}

// ResetDisplay forgets the screen and removes everything drawn (the default
// branch of ObserveAnchorEvent).
func (o *RuntimeActionBarOverlay) ResetDisplay() {
	if o != nil {
		o.screen = ""
		o.clearAll()
	}
}

// ObserveDisplayReset is the manual question's reaction to a display
// discontinuity.  A visible question is cleared through the presentation
// stream (one Clear of the current generation, no Observation: the proven
// clear call did not happen), so the presenter turns Visible into Cleared and
// the real 026F:029C clear that follows is not forwarded again; a question
// still being printed is poisoned, so it never produces a request.  An idle
// watcher is left alone.  The watcher is shared by every lane and must not be
// rebuilt: its generation has to keep increasing.
func (w *Watcher) ObserveDisplayReset(step uint64) {
	if w == nil || !w.collector.active() {
		return
	}
	c := &w.collector
	switch {
	case c.hasVisible:
		c.visible, c.hasVisible = Question{}, false
		w.presentation = append(w.presentation, ManualPresentationEvent{
			Step: step, Kind: ManualPresentationClear, Generation: c.generation,
		})
	case c.pending != nil:
		c.pending.poisoned = true
	}
}

// idle reports a post-join watcher with nothing in progress.
func (w *PostJoinMenuWatcher) idle() bool {
	return w != nil && w.stage == 0 && !w.active && w.pending == nil && !w.failed
}

// VideoModeChange is the live runtime's reaction to a video mode set.  It runs
// inside a Step, on the same goroutine as BeforeStep, Frame and Compose, and
// returns no error (a overlay problem never stops the game).
func (r *LiveRuntime) VideoModeChange(c machine.ModeChange) {
	if r == nil {
		return
	}
	for _, l := range r.lanes {
		l.modeReset()
	}
	// Shared watchers.  The post-join watcher: a failed one is left as it is
	// (its failure is counted at the next entry, as always); an idle one has
	// nothing to forget; the others are rebuilt without counting a reset.
	if w := r.postJoin; w != nil && !w.failed && !w.idle() {
		if n, err := NewPostJoinMenuWatcher(r.postShared); err == nil {
			r.postJoin = n
		}
	}
	r.action.ResetDisplay()
	r.body.Reset()
	r.manual.ObserveDisplayReset(c.Step)
	r.syncManual()
	// Story pages: the in-flight glyph frame belongs to the pages that were
	// just reset, so it goes only after all of them.
	r.storyPending = nil
	r.prevAt, r.prevOp = Address{}, 0
}

// modeReset resets every overlay family of this lane (spec 052 §3.2).
func (l *liveLane) modeReset() {
	if l.ecl != nil {
		l.ecl.ObserveDiscontinuity()
	}
	if l.hmenu != nil {
		l.hmenu.ObserveDiscontinuity()
	}
	if l.engDisp != nil {
		l.engDisp.ObserveDiscontinuity()
	}
	if l.logbook != nil {
		l.logbook.ObserveDiscontinuity()
	}
	for i := range liveScales {
		l.bodyPres[i].ClearAll()
		if l.skill[i] != nil {
			l.skill[i].Restore()
		}
		if l.exit[i] != nil {
			l.exit[i].Restore()
		}
		l.postPres[i].clear()
		l.actPres[i].ResetDisplay()
	}
	l.menuClear([4]uint8{24, 39, 0, 0})
	for _, f := range l.stories {
		f.modeReset()
	}
}

// ActiveFamilies lists the overlay families of lang's lane that show or hold
// something now.  Read-only; for tests and receipts.
func (r *LiveRuntime) ActiveFamilies(lang string) []string {
	if r == nil {
		return nil
	}
	i := r.laneIndex(lang)
	if i < 0 {
		return nil
	}
	l := r.lanes[i]
	var out []string
	add := func(name string, on bool) {
		if on {
			out = append(out, name)
		}
	}
	add("ecl-text", l.ecl != nil && len(l.ecl.Pages()) != 0)
	add("hmenu", l.hmenu != nil && l.hmenu.Page() != nil)
	add("engine-dispatch", l.engDisp != nil && len(l.engDisp.Lines()) != 0)
	add("logbook", l.logbook != nil && l.logbook.open != 0)
	for k := range liveScales {
		add("menu", len(l.menuPres[k].ActiveKeys()) != 0 && !contains(out, "menu"))
		add("body-icon", len(l.bodyPres[k].ActiveKeys()) != 0 && !contains(out, "body-icon"))
		add("post-join", len(l.postPres[k].ActiveKeys()) != 0 && !contains(out, "post-join"))
		add("action-bar", len(l.actPres[k].ActiveKeys()) != 0 && !contains(out, "action-bar"))
		add("manual", len(l.manPres[k].ActiveKeys()) != 0 && !contains(out, "manual"))
		if l.skill[k] != nil {
			add("skill-exit", len(l.skill[k].Presenter.ActiveKeys()) != 0 && !contains(out, "skill-exit"))
		}
		if l.exit[k] != nil {
			add("exit-prompt", len(l.exit[k].Presenter.ActiveKeys()) != 0 && !contains(out, "exit-prompt"))
		}
	}
	for _, f := range l.stories {
		add(f.name(), f.active())
	}
	sort.Strings(out)
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
