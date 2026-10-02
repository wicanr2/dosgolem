package buckrogers

import (
	"fmt"

	"github.com/wicanr2/dosgolem/xlate"
)

// Buck repo spec 053: the 3× zh-TW lane presenter of the live runtime uses
// the E1 plan (English words are never split at a line end).  E1 stays off
// for every other language and for 2×.
//
// Status strings of the zh-TW lane (DebugSummary, no text, no keys):
//
//	on
//	off(preflight:M)  M paragraphs failed the preflight
//	off(keyword:N)    N questions whose E1 paragraph is longer than the
//	                  fixed-cell one, so the keyword row of spec 034 could
//	                  overlap it
//	off(runtime)      a request failed under E1 and the fixed cells took over
const (
	manualE1On      = "on"
	manualE1Runtime = "off(runtime)"
)

// rowsUsed is the number of rows the E1 paragraph occupies: the last row
// with a token, plus one.
func (p *ManualE1Plan) rowsUsed() int {
	k := 0
	for i, line := range p.lines {
		if len(line.tokens) != 0 {
			k = i + 1
		}
	}
	return k
}

// enableManualE1 runs the spec 053 §3.1 preflight on this 3× presenter: every
// catalog paragraph must build an E1 plan and its text layer with the
// presenter's own fonts.  Nothing is applied and no state is kept.  When all
// of them build, e1Base is set and the rows each paragraph uses are returned
// (event key → rows); otherwise e1Base stays nil and failed counts the
// paragraphs that did not build.
//
// base is the lane's 16×16 font; the presenter must have been built from it
// (spec 053 §3.2: not from a clone, so the half font stays shared).
func (o *RuntimeManualOverlay) enableManualE1(base *xlate.Font) (rows map[string]int, failed int) {
	if o == nil || o.scale != 3 || base == nil {
		return nil, 0
	}
	// The derived 22-point font needs an identity before a plan can be built
	// (spec 053 §3.2 step 2); it is the presenter's own, so naming it does
	// not change any fixed-cell output.
	o.font.Name = manualDerivedFontIdentity
	clone := cloneManualBaseFont(base)
	rows = map[string]int{}
	for _, entry := range o.catalog.byIdentity {
		if entry.translation == "" {
			continue
		}
		request := DisplayRequest{Generation: 1, EventKey: entry.eventKey, TextKey: entry.textKey, Translation: entry.translation}
		plan, err := BuildManualE1Plan(o.layout, o.catalog, request, clone, o.font, o.half)
		if err == nil {
			_, err = plan.TextLayer(clone, o.font, o.half)
		}
		if err != nil {
			failed++
			continue
		}
		rows[entry.eventKey] = plan.rowsUsed()
	}
	if failed != 0 {
		return nil, failed
	}
	o.e1Base = clone
	return rows, 0
}

// setupManualE1 enables E1 on the zh-TW lane's 3× presenter (spec 053 §3.1).
func (l *liveLane) setupManualE1(i int) {
	if l.lang != LangZhTW || liveScales[i] != 3 {
		return
	}
	rows, failed := l.manPres[i].enableManualE1(l.font)
	if failed != 0 {
		l.manE1 = fmt.Sprintf("off(preflight:%d)", failed)
		return
	}
	l.manE1Rows = rows
	l.manE1 = manualE1On
}

// manualE1Index is the index of the 3× presenter in liveScales.
func manualE1Index() int {
	for i, s := range liveScales {
		if s == 3 {
			return i
		}
	}
	return -1
}

// manualE1Active reports whether the zh-TW lane's 3× presenter draws E1.
func (l *liveLane) manualE1Active() bool {
	i := manualE1Index()
	return l.lang == LangZhTW && i >= 0 && l.manPres[i] != nil && l.manPres[i].e1Base != nil
}

// checkManualE1Keyword is the spec 053 §3.4 coexistence check, run once the
// local keyword rows are loaded: the rows sit under the fixed-cell paragraph,
// so E1 may use at most as many rows as the fixed cells do.  Otherwise E1 is
// turned off as a whole (nothing else changes).
func (l *liveLane) checkManualE1Keyword(m *ManualEnglish) {
	if m == nil || !l.manualE1Active() {
		return
	}
	violations := 0
	for _, entry := range l.manCatalog.byIdentity {
		if _, has := m.plans[entry.eventKey]; !has {
			continue
		}
		paragraph, err := manualRows(entry.translation, manualEnglishColumns, manualEnglishLastRow+1)
		if err != nil || l.manE1Rows[entry.eventKey] > manualUsedRows(paragraph) {
			violations++
		}
	}
	if violations != 0 {
		l.manPres[manualE1Index()].e1Base = nil
		l.manE1 = fmt.Sprintf("off(keyword:%d)", violations)
	}
}

// manualE1Fallback is the spec 053 §3.1 runtime exit: a Sync that failed
// while E1 was on is tried once more with the fixed cells.  It returns
// whether the second attempt succeeded; on failure E1 is left as it was
// (the failure was not E1's) and the caller counts the reset.
func (l *liveLane) manualE1Fallback(i int) bool {
	p := l.manPres[i]
	saved := p.e1Base
	p.e1Base = nil
	if _, err := l.manSync[i].Sync(); err != nil {
		p.e1Base = saved
		return false
	}
	l.manE1 = manualE1Runtime
	return true
}
