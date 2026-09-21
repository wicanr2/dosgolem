package buckrogers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type actionFixtureRow struct {
	screen, key, text string
	column            int
}

var actionFixtureRows = []actionFixtureRow{
	{"career", "action.add", "aaa", 0},
	{"career", "action.subtract", "bbbbbbbb", 4},
	{"career", "action.done", "cccc", 13},
	{"technical", "action.add", "aaa", 0},
	{"technical", "action.subtract", "bbbbbbbb", 4},
	{"technical", "action.prev", "dddd", 13},
	{"technical", "action.next", "eeee", 18},
	{"technical", "action.done", "cccc", 23},
}

func actionFixture(t *testing.T) *ActionBarCatalog {
	t.Helper()
	var out strings.Builder
	out.WriteString(strings.Join(actionBarHeader, "\t") + "\n")
	for _, row := range actionFixtureRows {
		h := sha256.Sum256([]byte(row.text))
		fmt.Fprintf(&out, "%s\t%s\t%d\t%s\t24\t%d\t%d\t192\t%d\t200\t37F1:0391\t37F1:03CE\t0\t15\t10\t37F1:0337\t15\t0\tconfirmed\tunknown\n",
			row.screen, row.key, len(row.text), hex.EncodeToString(h[:]), row.column, row.column*8, (row.column+len(row.text))*8)
	}
	c, err := LoadActionBarCatalog([]byte(out.String()))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func emitActionLabel(t *testing.T, w *ActionBarWatcher, row actionFixtureRow, focus bool, step *uint64) {
	t.Helper()
	for i, glyph := range []byte(row.text) {
		caller, bg, fg := Address{0x37F1, 0x03CE}, uint16(0), uint16(10)
		if i == 0 {
			caller, fg = Address{0x37F1, 0x0391}, 15
		}
		if focus {
			caller, bg, fg = Address{0x37F1, 0x0337}, 15, 0
		}
		args := [7]uint16{1, uint16(glyph), 1, bg, fg, 24, uint16(row.column + i)}
		w.ObserveGlyphEntry(caller, 0x2000, 0x3000, args, *step)
		*step++
		w.ObserveInstruction(caller, 0x2000, 0x3012, *step)
		*step++
	}
}

func TestActionBarWatcherRebuildsCareerAndTechnicalLabels(t *testing.T) {
	for _, tc := range []struct {
		anchor string
		screen string
		want   int
	}{
		{"career.screen.remaining_points.heading", "career", 3},
		{"technical.screen.general_points.heading", "technical", 5},
	} {
		t.Run(tc.screen, func(t *testing.T) {
			w := NewActionBarWatcher(actionFixture(t))
			if !w.ObserveAnchorEvent(tc.anchor) {
				t.Fatal("proven anchor must be accepted")
			}
			step := uint64(100)
			for i, row := range actionFixtureRows {
				if row.screen == tc.screen {
					emitActionLabel(t, w, row, i%2 == 0, &step)
				}
			}
			events := w.Events()
			if len(events) != tc.want || w.Pending() || w.Drops() != 0 || w.Misses() != 0 {
				t.Fatalf("events=%d pending=%v drops=%d misses=%d", len(events), w.Pending(), w.Drops(), w.Misses())
			}
			for _, event := range events {
				if event.Screen != tc.screen || event.EventKey == "" || event.PostCallStep <= event.EntryStep || event.Y0 != 192 || event.Y1 != 200 {
					t.Fatalf("event=%#v", event)
				}
			}
		})
	}
}

func TestActionBarWatcherRejectsUnanchoredPartialAndDrift(t *testing.T) {
	row := actionFixtureRows[0]
	for _, tc := range []struct {
		name   string
		mutate func(*[7]uint16, *Address, *uint16, *uint16)
	}{
		{"mode", func(a *[7]uint16, _ *Address, _, _ *uint16) { a[0] = 0 }},
		{"repeat", func(a *[7]uint16, _ *Address, _, _ *uint16) { a[2] = 2 }},
		{"row", func(a *[7]uint16, _ *Address, _, _ *uint16) { a[5] = 23 }},
		{"column", func(a *[7]uint16, _ *Address, _, _ *uint16) { a[6] = 1 }},
		{"caller", func(_ *[7]uint16, c *Address, _, _ *uint16) { *c = Address{1, 2} }},
		{"ss", func(_ *[7]uint16, _ *Address, ss, _ *uint16) { *ss = 0x2001 }},
		{"sp", func(_ *[7]uint16, _ *Address, _, sp *uint16) { *sp = 0x3011 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := NewActionBarWatcher(actionFixture(t))
			w.ObserveAnchorEvent("career.screen.remaining_points.heading")
			caller, ss, returnSP := Address{0x37F1, 0x0337}, uint16(0x2000), uint16(0x3012)
			args := [7]uint16{1, 'a', 1, 15, 0, 24, 0}
			tc.mutate(&args, &caller, &ss, &returnSP)
			w.ObserveGlyphEntry(caller, 0x2000, 0x3000, args, 1)
			w.ObserveInstruction(caller, ss, returnSP, 2)
			if len(w.Events()) != 0 {
				t.Fatal("drift must not emit")
			}
		})
	}

	unanchored := NewActionBarWatcher(actionFixture(t))
	step := uint64(1)
	emitActionLabel(t, unanchored, row, true, &step)
	if len(unanchored.Events()) != 0 {
		t.Fatal("content alone must not anchor")
	}

	partial := NewActionBarWatcher(actionFixture(t))
	partial.ObserveAnchorEvent("career.screen.remaining_points.heading")
	args := [7]uint16{1, 'a', 1, 15, 0, 24, 0}
	partial.ObserveGlyphEntry(Address{0x37F1, 0x0337}, 1, 2, args, 1)
	partial.ObserveInstruction(Address{0x37F1, 0x0337}, 1, 0x14, 2)
	partial.ObserveClear(24, 39, 24, 0)
	if partial.Pending() || len(partial.Events()) != 0 {
		t.Fatal("clear must invalidate partial candidate")
	}
	step = 20
	emitActionLabel(t, partial, row, true, &step)
	if len(partial.Events()) != 1 {
		t.Fatal("clear must preserve exact screen anchor")
	}
	partial.ObserveAnchorEvent("career.screen.skills.heading")
	emitActionLabel(t, partial, row, true, &step)
	if len(partial.Events()) != 2 {
		t.Fatal("same-screen exact event must preserve anchor")
	}
	partial.ObserveAnchorEvent("class.option.warrior")
	emitActionLabel(t, partial, row, true, &step)
	if len(partial.Events()) != 2 {
		t.Fatal("unrelated exact event must revoke anchor")
	}
	technical := NewActionBarWatcher(actionFixture(t))
	technical.ObserveAnchorEvent("technical.screen.general_points.heading")
	technical.ObserveAnchorEvent("career.screen.maximum_per_skill.heading")
	technical.ObserveAnchorEvent("career.screen.columns.heading")
	step = 100
	emitActionLabel(t, technical, actionFixtureRows[3], true, &step)
	if len(technical.Events()) != 1 || technical.Events()[0].Screen != "technical" {
		t.Fatal("two proven shared career keys must preserve technical anchor")
	}
	technical.ObserveAnchorEvent("career.screen.skill.notice.normal")
	emitActionLabel(t, technical, actionFixtureRows[3], true, &step)
	if len(technical.Events()) != 1 {
		t.Fatal("other career keys must revoke technical anchor")
	}
}

func TestActionBarWatcherRejectsUnknownHashAndReturnsCopies(t *testing.T) {
	w := NewActionBarWatcher(actionFixture(t))
	w.ObserveAnchorEvent("career.screen.remaining_points.heading")
	row := actionFixtureRows[0]
	row.text = "aaz"
	step := uint64(1)
	emitActionLabel(t, w, row, true, &step)
	if len(w.Events()) != 0 || w.Misses() != 1 {
		t.Fatalf("events=%d misses=%d", len(w.Events()), w.Misses())
	}
	row.text = "aaa"
	emitActionLabel(t, w, row, true, &step)
	events := w.Events()
	if len(events) != 1 {
		t.Fatalf("events=%d", len(events))
	}
	events[0].EventKey = "mutated"
	if w.Events()[0].EventKey == "mutated" {
		t.Fatal("returned slice must not mutate watcher")
	}
}

func TestFormalProjectActionBarCatalog(t *testing.T) {
	root := os.Getenv("BUCKROGERS_CHT_ROOT")
	if root == "" {
		t.Skip("BUCKROGERS_CHT_ROOT not set")
	}
	data, err := os.ReadFile(filepath.Join(root, "text", "skill-action-bar-events.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := LoadActionBarCatalog(data)
	if err != nil || len(c.byScreen["career"]) != 3 || len(c.byScreen["technical"]) != 5 {
		t.Fatalf("catalog=%#v err=%v", c, err)
	}
}
