package buckrogers

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// Buck repo spec 042 §5.4 / spec 043 §5.4: an opt-in record of every
// text-window call a replay makes, one TSV row per call and language lane.
// Every lane observes the same call, so one replay with several languages
// loaded shows where each of them overflows or falls back to English at the
// window the game really used.  A nil *ReplayTrace records nothing and the
// runtime is untouched (the deltas below are read from the watchers' own
// counters; no watcher changes its behaviour).
//
// Rows (tab separated; the first column is the kind):
//
//	ecl   step lang left top right bottom cursorRow cursorCol clear origLen sha12 key dHits dMisses dOverflows dPassthrough dFirstOnly dUnannotated dPlayer dSpaceDropped dFullStop dFullStopDropped
//	hmenu step lang row col textLen items dHits dMisses dOverflows
//	eng   step lang caller origLen dHits dMisses original zhUnits   (zhUnits: width of the catalog translation, -1 without one)
//	join  step lang n1 n2 result      (result: ok, nofit, nofragment)
type ReplayTrace struct {
	w    *bufio.Writer
	step uint64
	err  error
}

// NewReplayTrace writes the trace to w; call Flush when the replay ends.
func NewReplayTrace(w io.Writer) *ReplayTrace { return &ReplayTrace{w: bufio.NewWriter(w)} }

// Flush writes buffered rows and returns the first write error.
func (t *ReplayTrace) Flush() error {
	if t == nil {
		return nil
	}
	if err := t.w.Flush(); err != nil && t.err == nil {
		t.err = err
	}
	return t.err
}

func (t *ReplayTrace) setStep(step uint64) {
	if t != nil {
		t.step = step
	}
}

func (t *ReplayTrace) row(format string, args ...any) {
	if t == nil || t.err != nil {
		return
	}
	_, t.err = fmt.Fprintf(t.w, format+"\n", args...)
}

func (t *ReplayTrace) ecl(lang string, e EclTextEntry, key string, before, after EclTextStats) {
	if t == nil {
		return
	}
	sum := sha256.Sum256(e.Original)
	clear := 0
	if e.Clear {
		clear = 1
	}
	t.row("ecl\t%d\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d",
		e.Step, lang, e.Left, e.Top, e.Right, e.Bottom, e.CursorRow, e.CursorCol, clear, len(e.Original),
		hex.EncodeToString(sum[:6]), key,
		after.Hits-before.Hits, after.Misses-before.Misses, after.Overflows-before.Overflows,
		after.Passthrough-before.Passthrough, after.NameFirstOnly-before.NameFirstOnly,
		after.NameUnannotated-before.NameUnannotated,
		(after.PlayerNames+after.PlayerNameChineseOnly+after.PlayerNameEnglish)-(before.PlayerNames+before.PlayerNameChineseOnly+before.PlayerNameEnglish),
		after.SpaceDropped-before.SpaceDropped, after.FullStop-before.FullStop, after.FullStopDropped-before.FullStopDropped)
}

func (t *ReplayTrace) hmenu(lang string, step uint64, e HMenuEntry, before, after HMenuStats) {
	t.row("hmenu\t%d\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d", step, lang, e.Row, e.Col, len(e.Text), len(e.Items),
		after.Hits-before.Hits, after.Misses-before.Misses, after.Overflows-before.Overflows)
}

func (t *ReplayTrace) eng(lang string, step uint64, caller CodeKey, original []byte, units, dHits, dMisses int) {
	t.row("eng\t%d\t%s\t%v\t%d\t%d\t%d\t%q\t%d", step, lang, caller, len(original), dHits, dMisses, original, units)
}

func (t *ReplayTrace) join(lang string, n1, n2 int, result string) {
	if t == nil {
		return
	}
	t.row("join\t%d\t%s\t%d\t%d\t%s", t.step, lang, n1, n2, result)
}

// SetTrace turns the replay record on (nil turns it off).
func (r *LiveRuntime) SetTrace(t *ReplayTrace) {
	r.trace = t
	for _, l := range r.lanes {
		if l.engDisp == nil {
			continue
		}
		if t == nil {
			l.engDisp.SetJoinTrace(nil)
			continue
		}
		lang := l.lang
		l.engDisp.SetJoinTrace(func(n1, n2 int, result string) { t.join(lang, n1, n2, result) })
	}
}

// Nil-safe counter readers: a lane without a family has no watcher.
func eclStatsOf(w *EclTextWatcher) EclTextStats {
	if w == nil {
		return EclTextStats{}
	}
	return w.Stats
}

func hmenuStatsOf(w *HMenuWatcher) HMenuStats {
	if w == nil {
		return HMenuStats{}
	}
	return w.Stats
}

func engineStatsOf(w *EngineDispatchWatcher) (hits, misses int) {
	if w == nil {
		return 0, 0
	}
	return w.Stats.Hits, w.Stats.Misses
}

// engineUnits is the width of the catalog translation of an engine string
// (-1 without one): it shows whether a miss is a missing translation or a
// translation wider than the original allows.
func engineUnits(w *EngineDispatchWatcher, original []byte) int {
	if w == nil || w.catalog == nil {
		return -1
	}
	zh, ok := w.catalog.Translate(string(original))
	if !ok {
		return -1
	}
	return stringUnits(zh)
}
