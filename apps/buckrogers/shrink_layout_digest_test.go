package buckrogers

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

// Spec 057 §5.5: the layout digest of the synthetic corpus of spec 056 §5.3
// (pre056EachPlace, pre056EachPlayer), per language.  Everything the layout
// returns goes in, shrink level included; the lines of a call that does not
// fit are left out (they are what the last failed try left).  The constants
// were computed on the clean export of dosgolem bba48d9.

func shrinkDumpLines(sb *strings.Builder, ls []EclTextLine) {
	for _, l := range ls {
		fmt.Fprintf(sb, "[%d,%d,%d,%s]", l.Row, l.Col, l.Shrink, string(l.Text))
	}
}

func shrinkDumpPlacement(sb *strings.Builder, pl eclPlacement) {
	if pl.fits {
		shrinkDumpLines(sb, pl.lines)
	}
	fmt.Fprintf(sb, " %d %d %v %d %d %v %d %d %d\n", pl.endRow, pl.endCol, pl.fits, pl.tier, pl.shrink, pl.marker, pl.spaceDropped, pl.fullStop, pl.fullStopDropped)
}

func shrinkDumpPlayer(sb *strings.Builder, c playerLayout) {
	if c.fits {
		shrinkDumpLines(sb, c.lines)
	}
	fmt.Fprintf(sb, " %d %v %d %d %d %v\n", c.kind, c.spaced, c.shrink, c.endRow, c.endCol, c.fits)
}

// shrinkLayoutDigests returns the digest and the number of calls per language.
func shrinkLayoutDigests(t *testing.T) (map[string]string, map[string]int) {
	t.Helper()
	digests, calls := map[string]string{}, map[string]int{}
	for _, l := range pre056Langs() {
		w := pre056Watcher(t, l)
		h := sha256.New()
		var sb strings.Builder
		flush := func() {
			h.Write([]byte(sb.String()))
			sb.Reset()
		}
		pre056EachPlace(t, l, 4, func(c pre056PlaceCase) {
			shrinkDumpPlacement(&sb, adapterPlace(w, c))
			calls[l.lang]++
			if sb.Len() > 1<<20 {
				flush()
			}
		})
		pre056EachPlayer(l, 2, func(c pre056PlayerCase) {
			shrinkDumpPlayer(&sb, adapterPlayer(l.lang, l.prof, c))
			calls[l.lang]++
		})
		flush()
		digests[l.lang] = fmt.Sprintf("%x", h.Sum(nil))
	}
	return digests, calls
}
