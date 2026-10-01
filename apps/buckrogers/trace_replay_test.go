package buckrogers

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Buck repo spec 042 §5.4 / spec 043 §5.4: replay the narrative-window (W)
// and horizontal-menu (H) records of the phase257 traces through the real
// watchers of every language and report where a language overflows or falls
// back to English.  A W record carries the window, the cursor and the
// printable bytes of the original string (a byte outside 0x20..0x7E is
// recorded as '~' and cannot be matched, those calls are counted apart).
// Video writes are not in the records, so a window accumulates rows until
// the next fresh call exactly as the watcher does without interference.
//
// Inputs: BUCKROGERS_CHT_ROOT, BUCKROGERS_ZHTW_FONT, the language fonts
// (BUCKROGERS_{JA,KO}_FONT; zh-CN is replayed when BUCKROGERS_ZHCN_FONT is
// set) and BUCKROGERS_TRACE_DIR (the phase257 cp/ directory).  The test
// skips with its reason without them.  BUCKROGERS_TRACE_REPORT names an
// output TSV with every call that did not hit in some language.

type traceW struct {
	file  string
	e     EclTextEntry
	text  string
	lossy bool
}

type traceD struct {
	file             string
	caller           string // segment:offset as recorded
	bg, fg, row, col uint16
	text             string
	lossy            bool
}

type traceH struct {
	file  string
	e     HMenuEntry
	lossy bool
}

func parseTraceDir(t *testing.T, dir string) (ws []traceW, hs []traceH, ds []traceD, files int) {
	t.Helper()
	names, err := filepath.Glob(filepath.Join(dir, "*.tsv"))
	if err != nil || len(names) == 0 {
		t.Skipf("BUCKROGERS_TRACE_DIR 沒有 *.tsv：%v", err)
	}
	sort.Strings(names)
	for _, name := range names {
		f, err := os.Open(name)
		if err != nil {
			t.Fatal(err)
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		used := false
		for sc.Scan() {
			p := strings.Split(sc.Text(), "\t")
			switch p[0] {
			case "W":
				if len(p) < 8 {
					continue
				}
				w, ok := parseW(filepath.Base(name), p)
				if ok {
					ws = append(ws, w)
					used = true
				}
			case "D":
				if len(p) < 8 {
					continue
				}
				var d traceD
				d.file, d.caller, d.text = filepath.Base(name), p[2], p[7]
				ok := true
				for i, dst := range []*uint16{&d.bg, &d.fg, &d.row, &d.col} {
					n, err := strconv.Atoi(p[3+i])
					if err != nil || n < 0 || n > 0xFFFF {
						ok = false
					}
					*dst = uint16(n)
				}
				if ok {
					d.lossy = strings.ContainsRune(d.text, '~')
					ds = append(ds, d)
					used = true
				}
			case "H":
				if len(p) < 8 {
					continue
				}
				if h, ok := parseH(filepath.Base(name), p); ok {
					hs = append(hs, h)
					used = true
				}
			}
		}
		f.Close()
		if err := sc.Err(); err != nil {
			t.Fatal(err)
		}
		if used {
			files++
		}
	}
	return
}

func numAfter(s, prefix string) (int, bool) {
	if !strings.HasPrefix(s, prefix) {
		return 0, false
	}
	n, err := strconv.Atoi(s[len(prefix):])
	return n, err == nil
}

func parseW(file string, p []string) (traceW, bool) {
	var e EclTextEntry
	step, err := strconv.ParseUint(p[1], 10, 64)
	flag, ok1 := numAfter(p[3], "flag=")
	if err != nil || !ok1 {
		return traceW{}, false
	}
	var bg, fg int
	if _, err := fmt.Sscanf(p[4], "a=%d,%d", &bg, &fg); err != nil {
		return traceW{}, false
	}
	var l, tp, r, b int
	if _, err := fmt.Sscanf(p[5], "L%d T%d R%d B%d", &l, &tp, &r, &b); err != nil {
		return traceW{}, false
	}
	var cc, cr int
	if _, err := fmt.Sscanf(p[6], "cur=%d,%d", &cc, &cr); err != nil {
		return traceW{}, false
	}
	text := p[7]
	e = EclTextEntry{Step: step, SS: 1, SP: 0x100, Return: Address{Segment: 0x1000, Offset: 0x10}, Original: []byte(text),
		Clear: flag != 0, Background: uint8(bg), Foreground: uint8(fg),
		Left: uint8(l), Top: uint8(tp), Right: uint8(r), Bottom: uint8(b), CursorCol: uint8(cc), CursorRow: uint8(cr)}
	return traceW{file: file, e: e, text: text, lossy: strings.ContainsRune(text, '~')}, true
}

func parseH(file string, p []string) (traceH, bool) {
	step, err := strconv.ParseUint(p[1], 10, 64)
	sel, ok1 := numAfter(p[2], "sel=")
	row, ok2 := numAfter(p[3], "row=")
	col, ok3 := numAfter(p[4], "col=")
	if err != nil || !ok1 || !ok2 || !ok3 || len(p) < 8 {
		return traceH{}, false
	}
	tab := strings.Split(p[6], ",")
	if len(tab) != 12 {
		return traceH{}, false
	}
	pair := func(s string) (uint8, uint8, bool) {
		var a, b int
		_, err := fmt.Sscanf(s, "%d-%d", &a, &b)
		return uint8(a), uint8(b), err == nil
	}
	_, count, ok := pair(tab[0])
	if !ok || int(count) > len(tab)-1 {
		return traceH{}, false
	}
	items := make([][2]uint8, count)
	for i := range items {
		a, b, ok := pair(tab[i+1])
		if !ok {
			return traceH{}, false
		}
		items[i] = [2]uint8{a, b}
	}
	text := strings.TrimSuffix(strings.TrimPrefix(p[7], "|"), "|")
	e := HMenuEntry{SS: 1, SP: 0x100, Return: Address{Segment: 0x1000, Offset: 0x10}, Text: []byte(text), Row: uint8(row), Col: uint8(col),
		Items: items, Selected: uint8(sel), Normal: 2, Hot: 14}
	_ = step
	return traceH{file: file, e: e, lossy: strings.ContainsRune(text, '~')}, true
}

func traceInputs(t *testing.T) (text, twFont string, fonts map[string]string, dir string) {
	root := os.Getenv("BUCKROGERS_CHT_ROOT")
	twFont = os.Getenv("BUCKROGERS_ZHTW_FONT")
	dir = os.Getenv("BUCKROGERS_TRACE_DIR")
	if root == "" || twFont == "" || dir == "" {
		t.Skip("BUCKROGERS_CHT_ROOT／BUCKROGERS_ZHTW_FONT／BUCKROGERS_TRACE_DIR 未設定")
	}
	fonts = map[string]string{}
	for lang, env := range map[string]string{LangJa: "BUCKROGERS_JA_FONT", LangKo: "BUCKROGERS_KO_FONT", LangZhCN: "BUCKROGERS_ZHCN_FONT"} {
		if p := os.Getenv(env); p != "" {
			fonts[lang] = p
		}
	}
	if len(fonts) == 0 {
		t.Skip("沒有任何語言字型環境變數")
	}
	return filepath.Join(root, "text"), twFont, fonts, dir
}

func TestReplayPhase257Windows(t *testing.T) {
	text, twFont, fonts, dir := traceInputs(t)
	var langs []string
	for l := range fonts {
		langs = append(langs, l)
	}
	sort.Strings(langs)
	r, err := LoadLiveRuntimeOptions(LiveOptions{TextDir: text, FontPath: twFont, Langs: langs, LangFonts: fonts})
	if err != nil {
		t.Fatal(err)
	}
	ws, hs, ds, files := parseTraceDir(t, dir)
	var lossy, lossyH int
	for _, w := range ws {
		if w.lossy {
			lossy++
		}
	}
	for _, h := range hs {
		if h.lossy {
			lossyH++
		}
	}
	t.Logf("trace %d 檔：W %d 次（含無法比對的 %d）、H %d 次（含無法比對的 %d）、D %d 次", files, len(ws), lossy, len(hs), lossyH, len(ds))

	var report *bufio.Writer
	if p := os.Getenv("BUCKROGERS_TRACE_REPORT"); p != "" {
		f, err := os.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		report = bufio.NewWriter(f)
		defer report.Flush()
		fmt.Fprintln(report, "kind\tlang\tfile\tstep\tkey\tL\tT\tR\tB\tcursor\torigLen\toutcome")
	}
	// BUCKROGERS_TRACE_PAGES: every message (the calls of one window between
	// two fresh calls) with the originals as printed and the rows the
	// language shows, for the concatenation audit (spec 042 §5.5).
	var pages *bufio.Writer
	if p := os.Getenv("BUCKROGERS_TRACE_PAGES"); p != "" {
		f, err := os.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		pages = bufio.NewWriter(f)
		defer pages.Flush()
		fmt.Fprintln(pages, "kind\tlang\tfile\tstep\twindow\tcalls\thits\tmisses\toverflows\toriginal\tshown\tcalls_orig")
	}
	windows := map[[4]uint8]int{}
	for _, w := range ws {
		windows[[4]uint8{w.e.Left, w.e.Top, w.e.Right, w.e.Bottom}]++
	}
	type win struct {
		k [4]uint8
		n int
	}
	var wl []win
	for k, n := range windows {
		wl = append(wl, win{k, n})
	}
	sort.Slice(wl, func(i, j int) bool {
		return wl[i].n > wl[j].n || wl[i].n == wl[j].n && fmt.Sprint(wl[i].k) < fmt.Sprint(wl[j].k)
	})
	for _, x := range wl {
		t.Logf("視窗 L%d T%d R%d B%d（寬 %d 單位、高 %d 列）：%d 次", x.k[0], x.k[1], x.k[2], x.k[3], (int(x.k[2])-int(x.k[0])+1)*2, int(x.k[3])-int(x.k[1])+1, x.n)
	}

	for _, l := range r.lanes {
		if l.ecl == nil {
			continue
		}
		var st struct{ calls, hit, miss, over, pass, first, none int }
		perWindow := map[[4]uint8]int{}
		// Rows left below the page after a hit (Bottom - end row): how close
		// each language gets to the bottom of each window.
		minMargin := map[[4]uint8]int{}
		atEdge := map[[4]uint8]int{}
		type message struct {
			file                      string
			step                      uint64
			calls, hits, misses, over int
			orig                      strings.Builder
			callTexts                 []string
			shown                     string
		}
		msgs := map[[4]uint8]*message{}
		flush := func(k [4]uint8) {
			if m := msgs[k]; m != nil && pages != nil && m.calls > 0 {
				fmt.Fprintf(pages, "page\t%s\t%s\t%d\tL%d T%d R%d B%d\t%d\t%d\t%d\t%d\t%s\t%s\t%s\n", l.lang, m.file, m.step,
					k[0], k[1], k[2], k[3], m.calls, m.hits, m.misses, m.over, m.orig.String(), m.shown, strings.Join(m.callTexts, "\x1f"))
			}
			delete(msgs, k)
		}
		flushAll := func() {
			var ks [][4]uint8
			for k := range msgs {
				ks = append(ks, k)
			}
			sort.Slice(ks, func(i, j int) bool { return fmt.Sprint(ks[i]) < fmt.Sprint(ks[j]) })
			for _, k := range ks {
				flush(k)
			}
		}
		prevFile := ""
		for _, w := range ws {
			if w.lossy {
				continue
			}
			if w.file != prevFile {
				flushAll()
				l.ecl.ObserveDiscontinuity()
				prevFile = w.file
			}
			wk := [4]uint8{w.e.Left, w.e.Top, w.e.Right, w.e.Bottom}
			if w.e.Clear || w.e.CursorCol < w.e.Left || w.e.CursorCol > w.e.Right || w.e.CursorRow < w.e.Top || w.e.CursorRow > w.e.Bottom {
				// A fresh call ends the message of this window and of every window it overlaps.
				var ks [][4]uint8
				for k := range msgs {
					if k == wk || k[0] <= wk[2] && wk[0] <= k[2] && k[1] <= wk[3] && wk[1] <= k[3] {
						ks = append(ks, k)
					}
				}
				sort.Slice(ks, func(i, j int) bool { return fmt.Sprint(ks[i]) < fmt.Sprint(ks[j]) })
				for _, k := range ks {
					flush(k)
				}
			}
			before := l.ecl.Stats
			l.ecl.ObserveEntry(w.e)
			if l.ecl.InCall() {
				l.ecl.ObserveInstruction(w.e.Return, w.e.SS, w.e.SP+eclTextReturnDelta)
			}
			a := l.ecl.Stats
			m := msgs[wk]
			if m == nil {
				m = &message{file: w.file, step: w.e.Step}
				msgs[wk] = m
			}
			m.calls++
			m.callTexts = append(m.callTexts, w.text)
			m.hits += a.Hits - before.Hits
			m.misses += a.Misses - before.Misses
			m.over += a.Overflows - before.Overflows
			m.orig.WriteString(w.text)
			if _, p := l.ecl.window(w.e); p != nil {
				var rows []string
				for _, ln := range p.Lines {
					if p.Shows(ln.Row) {
						rows = append(rows, fmt.Sprintf("r%d.%d:%s", ln.Row, ln.Col, string(ln.Text)))
					}
				}
				m.shown = strings.Join(rows, " | ")
			}
			if a.Hits > before.Hits {
				if _, p := l.ecl.window(w.e); p != nil {
					k := [4]uint8{w.e.Left, w.e.Top, w.e.Right, w.e.Bottom}
					m := int(w.e.Bottom) - int(p.endRow)
					if old, seen := minMargin[k]; !seen || m < old {
						minMargin[k] = m
					}
					if m == 0 {
						atEdge[k]++
					}
				}
			}
			st.calls++
			st.hit += a.Hits - before.Hits
			st.miss += a.Misses - before.Misses
			st.pass += a.Passthrough - before.Passthrough
			st.first += a.NameFirstOnly - before.NameFirstOnly
			st.none += a.NameUnannotated - before.NameUnannotated
			if d := a.Overflows - before.Overflows; d > 0 {
				st.over += d
				perWindow[[4]uint8{w.e.Left, w.e.Top, w.e.Right, w.e.Bottom}] += d
			}
			if report != nil && a.Hits == before.Hits {
				key, _, _ := l.ecl.catalog.Lookup(w.e.Original)
				outcome := "miss"
				if a.Overflows > before.Overflows {
					outcome = "overflow"
				}
				fmt.Fprintf(report, "W\t%s\t%s\t%d\t%s\t%d\t%d\t%d\t%d\t%d,%d\t%d\t%s\n", l.lang, w.file, w.e.Step, key,
					w.e.Left, w.e.Top, w.e.Right, w.e.Bottom, w.e.CursorCol, w.e.CursorRow, len(w.e.Original), outcome)
			}
		}
		flushAll()
		t.Logf("%s ECL：呼叫 %d、命中 %d、未命中 %d、溢出 %d、passthrough %d、加註降段 first %d／none %d", l.lang, st.calls, st.hit, st.miss, st.over, st.pass, st.first, st.none)
		var keys []string
		for k, n := range perWindow {
			keys = append(keys, fmt.Sprintf("L%d T%d R%d B%d：%d", k[0], k[1], k[2], k[3], n))
		}
		sort.Strings(keys)
		if len(keys) > 0 {
			t.Logf("%s 溢出依視窗：%s", l.lang, strings.Join(keys, "；"))
		}
		var margins []string
		for _, x := range wl {
			if m, ok := minMargin[x.k]; ok {
				margins = append(margins, fmt.Sprintf("L%d T%d R%d B%d 最小餘列 %d（貼底 %d 次）", x.k[0], x.k[1], x.k[2], x.k[3], m, atEdge[x.k]))
			}
		}
		t.Logf("%s 各視窗頁尾餘列：%s", l.lang, strings.Join(margins, "；"))
	}

	for _, l := range r.lanes {
		if l.hmenu == nil {
			continue
		}
		var calls, hit, miss, over int
		slackHist := map[int]int{}
		minSlack := 1 << 30
		var rowsMeasured int
		prevFile := ""
		for _, h := range hs {
			if h.lossy {
				continue
			}
			if h.file != prevFile {
				l.hmenu.ObserveDiscontinuity()
				prevFile = h.file
			}
			before := l.hmenu.Stats
			l.hmenu.ObserveEntry(h.e)
			if l.hmenu.InCall() {
				l.hmenu.ObserveInstruction(h.e.Return, h.e.SS, h.e.SP+hmenuReturnDelta)
			}
			a := l.hmenu.Stats
			if a.Hits > before.Hits {
				// Half units left on each row of the menu (spec 039 §3.4:
				// the row holds 2×(40−start column), the item gap is one unit).
				used, start := map[int]int{}, map[int]int{}
				for i, r := range h.e.Items {
					line := strings.Count(string(h.e.Text[:r[0]-1]), "@")
					t, _ := l.hmenu.catalog.lookup(strings.TrimSpace(string(h.e.Text[r[0]-1 : r[1]])))
					if _, ok := start[line]; !ok {
						last := strings.LastIndexByte(string(h.e.Text[:r[0]-1]), '@')
						start[line] = int(h.e.Col) + int(r[0]-1) - (last + 1)
					}
					if i > 0 && used[line] > 0 {
						used[line]++
					}
					for _, ch := range t {
						used[line] += runeUnits(ch)
					}
				}
				for line, u := range used {
					slack := 2*(40-start[line]) - u
					slackHist[slack]++
					minSlack = min(minSlack, slack)
					rowsMeasured++
				}
			}
			calls++
			hit += a.Hits - before.Hits
			miss += a.Misses - before.Misses
			over += a.Overflows - before.Overflows
			if report != nil && a.Hits == before.Hits {
				sum := sha256.Sum256(h.e.Text)
				outcome := "miss"
				if a.Overflows > before.Overflows {
					outcome = "overflow"
				}
				fmt.Fprintf(report, "H\t%s\t%s\t0\t%x\t0\t0\t0\t0\t%d,%d\t%d\t%s\n", l.lang, h.file, sum[:6], h.e.Col, h.e.Row, len(h.e.Text), outcome)
			}
		}
		var buckets [4]int // slack <2, 2-5, 6-11, >=12 half units
		for sl, n := range slackHist {
			switch {
			case sl < 2:
				buckets[0] += n
			case sl < 6:
				buckets[1] += n
			case sl < 12:
				buckets[2] += n
			default:
				buckets[3] += n
			}
		}
		t.Logf("%s 水平選單：呼叫 %d、命中 %d、未命中 %d、溢出 %d；餘量（半形單位）最小 %d，列數 %d：<2 %d、2–5 %d、6–11 %d、≥12 %d",
			l.lang, calls, hit, miss, over, minSlack, rowsMeasured, buckets[0], buckets[1], buckets[2], buckets[3])
	}
}

// The engine-dispatch (D) records go through the real watcher of every lane.
// The record has the caller as segment:offset; the allowed callers are keyed
// by overlay unit and offset, so a record is matched on the offset alone
// (callers listed by one offset only; ambiguous offsets are skipped and
// counted).  Wrapped-fragment joins are reported with the (n1, n2) of each.
func TestReplayPhase257EngineDispatch(t *testing.T) {
	text, twFont, fonts, dir := traceInputs(t)
	var langs []string
	for l := range fonts {
		langs = append(langs, l)
	}
	sort.Strings(langs)
	r, err := LoadLiveRuntimeOptions(LiveOptions{TextDir: text, FontPath: twFont, Langs: langs, LangFonts: fonts})
	if err != nil {
		t.Fatal(err)
	}
	_, _, ds, _ := parseTraceDir(t, dir)
	// BUCKROGERS_TRACE_ENGINE: the engine result of every dispatcher call
	// the replay feeds, with the two width gates (spec 046 §3.5).
	var engOut *bufio.Writer
	if p := os.Getenv("BUCKROGERS_TRACE_ENGINE"); p != "" {
		f, err := os.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		engOut = bufio.NewWriter(f)
		defer engOut.Flush()
		fmt.Fprintln(engOut, "kind\tlang\tcaller\toriginal\tok\ttranslation\tunits\tgate2n\tfrontUnits\ttableGateFail")
	}
	for _, l := range r.lanes {
		w := l.engDisp
		if w == nil {
			continue
		}
		byOffset := map[uint16][]CodeKey{}
		for k := range w.allow {
			byOffset[k.Offset] = append(byOffset[k.Offset], k)
		}
		for k := range w.shared {
			byOffset[k.Offset] = append(byOffset[k.Offset], k)
		}
		type join struct {
			n1, n2 int
			res    string
		}
		var joins []join
		w.SetJoinTrace(func(n1, n2 int, res string) { joins = append(joins, join{n1, n2, res}) })
		var fed, skipped, ambiguous, lossy int
		prevFile := ""
		for _, d := range ds {
			if d.lossy {
				lossy++
				continue
			}
			if d.file != prevFile {
				w.ObserveDiscontinuity()
				prevFile = d.file
			}
			_, off, ok := strings.Cut(d.caller, ":")
			o, err := strconv.ParseUint(off, 16, 16)
			if !ok || err != nil {
				skipped++
				continue
			}
			keys := byOffset[uint16(o)]
			if len(keys) != 1 {
				if len(keys) == 0 {
					skipped++
				} else {
					ambiguous++
				}
				continue
			}
			ret := Address{Segment: 0x1000, Offset: 0x10}
			if engOut != nil && w.catalog != nil {
				zh, ok, units, front, tableFail := engineGate(w.catalog, d.text)
				gate := "-"
				if ok {
					gate = "pass"
					if units > 2*len(d.text) {
						gate = "fail"
					}
				}
				fmt.Fprintf(engOut, "eng\t%s\t%s\t%q\t%v\t%q\t%d\t%s\t%d\t%v\n", l.lang, d.caller, d.text, ok, zh, units, gate, front, tableFail)
			}
			w.ObserveEntryParty(keys[0], 1, 0x100, ret, [6]uint16{0, 0, d.bg, d.fg, d.row, d.col}, []byte(d.text), nil, nil)
			if w.inCall {
				w.ObserveInstruction(ret, 1, 0x100+engineDispatchReturnDelta)
			}
			fed++
		}
		var ok, nofit, nofrag int
		var oks, nofits []string
		for _, j := range joins {
			pair := fmt.Sprintf("n1=%d n2=%d", j.n1, j.n2)
			switch j.res {
			case "ok":
				ok++
				oks = append(oks, pair)
			case "nofit":
				nofit++
				nofits = append(nofits, pair)
			default:
				nofrag++
			}
		}
		// Hits and misses are not reported: the combat name column needs the
		// party snapshot of the machine, which a record does not have.
		t.Logf("%s 引擎片段：D %d 次，送入 %d（呼叫端不在清單 %d、偏移不唯一 %d、無法比對 %d）；兩列拆分嘗試 %d：成功 %d、拆後放不下 %d、組不成已知片段 %d；成功者 %s",
			l.lang, len(ds), fed, skipped, ambiguous, lossy, len(joins), ok, nofit, nofrag, strings.Join(oks, "；"))
		if len(nofits) > 0 {
			t.Logf("%s 拆後放不下：%s", l.lang, strings.Join(nofits, "；"))
		}
	}
}

// engineGate is the engine result of an original string with the numbers the
// two width gates look at: the translation width (the generic path draws it
// only when it is at most 2×len(original)), and for a table row the width of
// the front column (029 §2.6: front+1+last must fit the row).
func engineGate(c *EngineTextCatalog, s string) (zh string, ok bool, units, front int, tableFail bool) {
	zh, ok = c.Translate(s)
	units, front = -1, -1
	if ok {
		units = stringUnits(zh)
	}
	if m := engineTableRow.FindStringSubmatch(s); m != nil {
		if fz, fok := c.translateLine(m[1]); fok {
			front = stringUnits(fz)
			tableFail = front+1+len(m[3]) > 2*len(s)
		}
	}
	return
}
