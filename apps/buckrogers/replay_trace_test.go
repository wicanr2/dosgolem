package buckrogers

import (
	"bytes"
	"strings"
	"testing"
)

// Buck repo spec 042 §5.4: the replay record is opt-in and read-only.

func TestReplayTraceRows(t *testing.T) {
	var nilTrace *ReplayTrace
	nilTrace.setStep(1)
	nilTrace.ecl("ko", EclTextEntry{}, "", EclTextStats{}, EclTextStats{})
	nilTrace.join("ko", 1, 2, "ok")
	if err := nilTrace.Flush(); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	tr := NewReplayTrace(&buf)
	e := EclTextEntry{Step: 281020237, Original: []byte("abc"), Clear: true, Left: 1, Top: 17, Right: 38, Bottom: 22, CursorCol: 3, CursorRow: 18}
	tr.ecl("ko", e, "ecl.1.16.00260", EclTextStats{Hits: 4, Misses: 2, Overflows: 1, PlayerNames: 1},
		EclTextStats{Hits: 5, Misses: 2, Overflows: 1, Passthrough: 1, NameFirstOnly: 1, PlayerNames: 1, PlayerNameEnglish: 1})
	tr.hmenu("ja", 42, HMenuEntry{Row: 24, Col: 11, Text: []byte("Yes No"), Items: [][2]uint8{{1, 3}, {5, 6}}},
		HMenuStats{Hits: 1}, HMenuStats{Hits: 1, Overflows: 1})
	tr.eng("zh-CN", 43, CodeKey{Segment: 0x0763, Offset: 0x1282}, []byte("treated."), 16, 1, 0)
	tr.setStep(44)
	tr.join("ko", 31, 8, "nofit")
	if err := tr.Flush(); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"ecl\t281020237\tko\t1\t17\t38\t22\t18\t3\t1\t3\tba7816bf8f01\tecl.1.16.00260\t1\t0\t0\t1\t1\t0\t1\t0",
		"hmenu\t42\tja\t24\t11\t6\t2\t0\t0\t1",
		"eng\t43\tzh-CN\t0763:1282\t8\t1\t0\t\"treated.\"\t16",
		"join\t44\tko\t31\t8\tnofit",
	}
	if got := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n"); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("紀錄列：\n%s\n應為\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// The join record reports every outcome of the wrapped-fragment path and
// never changes it.
func TestEngineJoinTrace(t *testing.T) {
	caller := CodeKey{Segment: 0x0763, Offset: 0x1282}
	ret := Address{0x0763, 0x1282}
	type rec struct {
		n1, n2 int
		res    string
	}
	run := func(w *EngineDispatchWatcher, s1, s2 string) []rec {
		var got []rec
		w.SetJoinTrace(func(n1, n2 int, res string) { got = append(got, rec{n1, n2, res}) })
		w.ObserveEntry(caller, 1, 0x100, ret, [6]uint16{0, 0, 0, 13, 3, 1}, []byte(s1))
		w.ObserveInstruction(ret, 1, 0x100+engineDispatchReturnDelta)
		w.ObserveEntry(caller, 1, 0x100, ret, [6]uint16{0, 0, 0, 13, 4, 1}, []byte(s2))
		return got
	}
	s1, s2 := "  No injuries were successfully", "treated."
	if got := run(wrapFixture(t), s1, s2); len(got) != 1 || got[0] != (rec{len(s1), len(s2), "ok"}) {
		t.Errorf("ok：%+v", got)
	}
	if got := run(wrapFixture(t), s1, "xyz."); len(got) != 1 || got[0] != (rec{len(s1), 4, "nofragment"}) {
		t.Errorf("nofragment：%+v", got)
	}

	full := "ab cd"
	c, err := LoadEngineTextCatalog(EngineTextFiles{
		FragmentEvents: engineTSV("frag", full, "ab cdx"),
		FragmentText:   engineZh("frag", full, "一二三四", "ab cdx", "一二三四五六七八"),
		ItemEvents:     engineTSV("item", "Bolt", "Gun"),
		ItemText:       engineZh("item", "Bolt", "爆能", "Gun", "槍"),
		MonsterEvents:  engineTSV("monster", "NEO WARRIOR"),
		MonsterText:    engineZh("monster", "NEO WARRIOR", "NEO 戰士"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := run(NewEngineDispatchWatcher(c, map[CodeKey]bool{caller: true}), "ab", "cdx"); len(got) != 1 || got[0] != (rec{2, 3, "nofit"}) {
		t.Errorf("nofit：%+v", got)
	}
	// Without a trace nothing is recorded and the page is the same.
	w := wrapFixture(t)
	w.ObserveEntry(caller, 1, 0x100, ret, [6]uint16{0, 0, 0, 13, 3, 1}, []byte(s1))
	w.ObserveInstruction(ret, 1, 0x100+engineDispatchReturnDelta)
	w.ObserveEntry(caller, 1, 0x100, ret, [6]uint16{0, 0, 0, 13, 4, 1}, []byte(s2))
	if w.Page() == nil {
		t.Error("沒有紀錄時拆兩列照常運作")
	}
}
