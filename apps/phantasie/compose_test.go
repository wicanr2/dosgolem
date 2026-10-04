package phantasie

import (
	"fmt"
	"reflect"
	"testing"
)

// 組句關聯（docs/spec/001 §3.4、§9、§10 第 1 項「組句關聯」）的單元測試。
// 期望值都是依規格手算的字面值，不用被測函式的輸出推導。DGROUP 記憶體以 cpMem 模擬。
// 本檔的輔助型別與函式一律加前綴 cp。

// cpMem 是 DGROUP 的模擬：位址 0 至 FFFF 的位元組。put 寫入字串與結尾 NUL（NUL 之後的舊內容保留，與真機相同）。
type cpMem struct{ b []byte }

func newCpMem() *cpMem { return &cpMem{b: make([]byte, 0x10200)} }

func (m *cpMem) put(ptr uint16, s string) {
	copy(m.b[ptr:], s)
	m.b[int(ptr)+len(s)] = 0
}

// read 符合 StrReader：讀到 NUL 為止；讀滿 max 仍無 NUL 時回 truncated。
func (m *cpMem) read(ptr uint16, max int) (string, bool) {
	for i := 0; i < max; i++ {
		if m.b[int(ptr)+i] == 0 {
			return string(m.b[int(ptr) : int(ptr)+i]), false
		}
	}
	return string(m.b[int(ptr) : int(ptr)+max]), true
}

const (
	cpLine   = 0x638E // 行緩衝區 DS:638E，組句的 dest
	cpInner  = 0x7000 // 內層組句緩衝區
	cpInner2 = 0x7100 // 更內層的組句緩衝區
	cpFmt    = 0x1000 // 靜態格式字串的指標
)

// cpEnv 把 ComposeStore 與模擬記憶體綁在一起，並按原版的時序驅動：
// 掛點在函式入口觸發（記憶體還是舊內容），函式執行後才寫入新內容。
type cpEnv struct {
	c    *ComposeStore
	mem  *cpMem
	step uint64
}

func newCpEnv() *cpEnv { return &cpEnv{c: NewComposeStore(), mem: newCpMem()} }

// sprintf 模擬 sprintf(dest, fmt, ...)：S 入口呼叫 OnSprintf，其後原版把 shown 寫進 DS:dest。
func (e *cpEnv) sprintf(dest uint16, format string, args [12]uint16, strs []ArgStr, shown string) {
	e.step++
	e.c.OnSprintf(e.step, dest, cpFmt, KindStatic, format, args, strs)
	e.mem.put(dest, shown)
}

// strcat 模擬 strcat(dest, src)：T 入口以 DS 目前內容為 cur 呼叫 OnStrcat，其後原版把 shown 寫進 DS:dest。
func (e *cpEnv) strcat(dest uint16, src ArgStr, shown string) AppendResult {
	cur, _ := e.mem.read(dest, 80)
	r := e.c.OnStrcat(dest, cur, src)
	e.mem.put(dest, shown)
	return r
}

// cpW 把字組依序填進 12 個字組的引數陣列。
func cpW(ws ...uint16) [12]uint16 {
	var a [12]uint16
	copy(a[:], ws)
	return a
}

func cpStat(ptr uint16, content string) ArgStr {
	return ArgStr{Ptr: ptr, Content: content, Kind: KindStatic}
}

func cpBuf(ptr uint16, content string) ArgStr {
	return ArgStr{Ptr: ptr, Content: content, Kind: KindBuffer}
}

// cpHit 要求 Match 命中，回快照。
func cpHit(t *testing.T, c *ComposeStore, dest uint16, cur string) *Piece {
	t.Helper()
	p, m := c.Match(dest, cur)
	if p == nil || m != MissNone {
		t.Fatalf("Match(%04X, %q) = (%v, miss %d)，要命中", dest, cur, p, m)
	}
	return p
}

// cpMiss 要求 Match 以指定原因失敗。
func cpMiss(t *testing.T, c *ComposeStore, dest uint16, cur string, want Miss) {
	t.Helper()
	p, m := c.Match(dest, cur)
	if p != nil || m != want {
		t.Errorf("Match(%04X, %q) = (%v, miss %d)，要 (nil, miss %d)", dest, cur, p, m, want)
	}
}

// cpMisses 比對 Associate 回的 miss 次數（空與空視為相同）。
func cpMisses(t *testing.T, got, want map[Miss]int) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("misses = %v，要 %v", got, want)
	}
}

func cpContents(as []ArgStr) []string {
	out := make([]string, len(as))
	for i, a := range as {
		out[i] = a.Content
	}
	return out
}

// cpEvFmt 是以 ptr 為格式字串指標的事件。
func cpEvFmt(kind Kind, ptr uint16) *EventRecord {
	return &EventRecord{FmtKind: kind, FmtPtr: ptr}
}

// cpEvArgs 是戰鬥指令列這類事件：格式字串是靜態 %s，引數是各指標。
func cpEvArgs(args ...ArgStr) *EventRecord {
	return &EventRecord{FmtKind: KindStatic, FmtPtr: cpFmt, Format: "%s", ArgStrs: args}
}

// ---------------------------------------------------------------------------
// OnSprintf：記錄與組合字串

func TestComposeSprintfCombined(t *testing.T) {
	cases := []struct {
		name   string
		format string
		args   [12]uint16
		strs   []ArgStr
		want   string
	}{
		{"字面", "You win", cpW(), nil, "You win"},
		{"%d 正數", "HP %d", cpW(25), nil, "HP 25"},
		{"%d 是 16 位元有號", "%d", cpW(0xFFF6), nil, "-10"},
		{"%u 是 16 位元無號", "%u", cpW(0xFFF6), nil, "65526"},
		{"%c 取低位元組", "%c%c", cpW(0x41, 0x1242), nil, "AB"},
		{"%ld 低字組在前", "%ld", cpW(0x86A0, 0x0001), nil, "100000"},
		{"%ld 負數", "%ld", cpW(0xFFFF, 0xFFFF), nil, "-1"},
		{"%lu", "%lu", cpW(0xFFFF, 0xFFFF), nil, "4294967295"},
		{"%s 與 %d", "%s hits for %d", cpW(0x2000, 12), []ArgStr{cpStat(0x2000, "Gnoll")}, "Gnoll hits for 12"},
		{"%s 之後的 %ld 取兩個字組", "%s %ld", cpW(0x2000, 0x4240, 0x000F), []ArgStr{cpStat(0x2000, "Elf")}, "Elf 1000000"},
		{"兩個 %s 依序取字串", "%s/%s", cpW(0x2000, 0x2010), []ArgStr{cpStat(0x2000, "a"), cpStat(0x2010, "b")}, "a/b"},
		{"%-5s 靠左補白", "[%-5s]", cpW(0x2000), []ArgStr{cpStat(0x2000, "ab")}, "[ab   ]"},
		{"%5d 靠右補白", "[%5d]", cpW(42), nil, "[   42]"},
		{"%.3s 精度截斷", "%.3s", cpW(0x2000), []ArgStr{cpStat(0x2000, "Dragon")}, "Dra"},
		{"多餘的字組忽略", "HP %d", cpW(25, 99), nil, "HP 25"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newCpEnv()
			e.sprintf(cpLine, tc.format, tc.args, tc.strs, tc.want)
			p := cpHit(t, e.c, cpLine, tc.want)
			if p.Literal != tc.want {
				t.Errorf("Literal = %q，要 %q", p.Literal, tc.want)
			}
			if p.Fmt != tc.format || p.FmtKind != KindStatic {
				t.Errorf("Fmt = %q、FmtKind = %v，要 %q、static", p.Fmt, p.FmtKind, tc.format)
			}
			if p.Args != tc.args {
				t.Errorf("Args = %v，要 %v", p.Args, tc.args)
			}
			if got, want := cpContents(p.ArgStrs), cpContents(tc.strs); !reflect.DeepEqual(got, want) {
				t.Errorf("ArgStrs 內容 = %q，要 %q", got, want)
			}
			if len(p.Appends) != 0 {
				t.Errorf("剛記錄的 Appends = %v，要空", p.Appends)
			}
		})
	}
}

func TestComposeSprintfNulCut(t *testing.T) {
	e := newCpEnv()
	// %c 的引數為 0 時原版輸出 NUL，DS 內的字串在此結束：組合字串是 "A"
	e.sprintf(cpLine, "A%cB", cpW(0), nil, "A\x00B")
	p := cpHit(t, e.c, cpLine, "A")
	if p.Literal != "A" {
		t.Errorf("Literal = %q，要 \"A\"", p.Literal)
	}
	// 目前內容也截到第一個 NUL
	cpHit(t, e.c, cpLine, "A\x00B")
	// 沒有 NUL 的 "AB" 與 "A" 不同（記錄是它的前綴）
	cpMiss(t, e.c, cpLine, "AB", MissPrefix)

	// 追加接在截斷之後：組合字串是 "A" 加 "Z"
	if r := e.strcat(cpLine, cpStat(0x5000, "Z"), "AZ"); r != AppendDone {
		t.Fatalf("OnStrcat = %d，要 AppendDone", r)
	}
	if p := cpHit(t, e.c, cpLine, "AZ"); p.Literal != "AZ" {
		t.Errorf("追加後 Literal = %q，要 \"AZ\"", p.Literal)
	}
}

func TestComposeSprintfOverwriteKeepsNewest(t *testing.T) {
	e := newCpEnv()
	e.sprintf(cpLine, "first", cpW(), nil, "first")
	e.sprintf(cpInner, "other", cpW(), nil, "other")
	// 追加之後再覆寫：新記錄沒有舊的 Appends
	e.strcat(cpLine, cpStat(0x5000, " x"), "first x")
	e.sprintf(cpLine, "second", cpW(), nil, "second")

	cpMiss(t, e.c, cpLine, "first", MissContent)
	cpMiss(t, e.c, cpLine, "first x", MissContent)
	p := cpHit(t, e.c, cpLine, "second")
	if p.Fmt != "second" || p.Literal != "second" || len(p.Appends) != 0 {
		t.Errorf("覆寫後的記錄 = Fmt %q、Literal %q、Appends %v，要 second、second、空", p.Fmt, p.Literal, p.Appends)
	}
	// 另一個 dest 的記錄不受影響
	cpHit(t, e.c, cpInner, "other")
}

func TestComposeSprintfCopiesArgStrs(t *testing.T) {
	e := newCpEnv()
	strs := []ArgStr{cpStat(0x2000, "Elf")}
	e.sprintf(cpLine, "%s:", cpW(0x2000), strs, "Elf:")
	// 呼叫端事後改動自己的切片，不影響記錄
	strs[0].Content = "changed"
	p := cpHit(t, e.c, cpLine, "Elf:")
	if got := p.ArgStrs[0].Content; got != "Elf" {
		t.Errorf("記錄的 ArgStrs[0].Content = %q，要 \"Elf\"", got)
	}
	// OnSprintf 不改動呼叫端的 strs（關聯結果只存在記錄內）
	e2 := newCpEnv()
	e2.sprintf(cpInner, "Orc", cpW(), nil, "Orc")
	in := []ArgStr{cpBuf(cpInner, "Orc")}
	e2.sprintf(cpLine, "%s!", cpW(cpInner), in, "Orc!")
	if in[0].Piece != nil {
		t.Errorf("呼叫端 strs[0].Piece 被改成 %v，要維持 nil", in[0].Piece)
	}
}

// S 不做去重（001 §3.2）：IRQ0 使同一個 sprintf 入口觸發兩次（同 step、同引數），
// 記錄以 dest 為鍵覆寫，兩次的結果相同，不多佔記錄、不丟巢狀 Piece。
func TestComposeSprintfIRQ0DoubleTrigger(t *testing.T) {
	e := newCpEnv()
	e.sprintf(cpInner, "Orc", cpW(), nil, "Orc")
	strs := []ArgStr{cpBuf(cpInner, "Orc")}
	for i := 0; i < 2; i++ {
		e.c.OnSprintf(10, cpLine, cpFmt, KindStatic, "%s hits", cpW(cpInner), strs)
	}
	e.mem.put(cpLine, "Orc hits")

	p := cpHit(t, e.c, cpLine, "Orc hits")
	if p.Fmt != "%s hits" || p.Literal != "Orc hits" || len(p.Appends) != 0 {
		t.Errorf("記錄 = Fmt %q、Literal %q、Appends %q，要 \"%%s hits\"、\"Orc hits\"、空", p.Fmt, p.Literal, cpContents(p.Appends))
	}
	if n := p.ArgStrs[0].Piece; n == nil || n.Literal != "Orc" {
		t.Errorf("巢狀 Piece = %+v，要 Literal \"Orc\"", n)
	}
	// 之後的追加只記一筆
	if r := e.strcat(cpLine, cpStat(0x5000, " you"), "Orc hits you"); r != AppendDone {
		t.Fatalf("OnStrcat = %d，要 AppendDone", r)
	}
	if p := cpHit(t, e.c, cpLine, "Orc hits you"); len(p.Appends) != 1 {
		t.Errorf("Appends = %q，要單筆", cpContents(p.Appends))
	}
}

func TestComposeSprintfCapacity(t *testing.T) {
	dest := func(i int) uint16 { return uint16(0x2000 + i*0x20) }
	text := func(i int) string { return fmt.Sprintf("rec%02d", i) }
	fill := func(n int) *cpEnv {
		e := newCpEnv()
		for i := 0; i < n; i++ {
			e.sprintf(dest(i), text(i), cpW(), nil, text(i))
		}
		return e
	}
	hit := func(t *testing.T, e *cpEnv, from, to int) {
		t.Helper()
		for i := from; i <= to; i++ {
			if p, m := e.c.Match(dest(i), text(i)); p == nil {
				t.Errorf("記錄 %d（dest %04X）應還在，Match 回 miss %d", i, dest(i), m)
			}
		}
	}

	t.Run("64 筆全留", func(t *testing.T) {
		e := fill(64)
		hit(t, e, 0, 63)
	})
	t.Run("第 65 筆淘汰最舊的", func(t *testing.T) {
		e := fill(65)
		cpMiss(t, e.c, dest(0), text(0), MissDest)
		hit(t, e, 1, 64)
	})
	t.Run("同 dest 覆寫不佔容量", func(t *testing.T) {
		e := fill(64)
		for k := 0; k < 10; k++ {
			e.sprintf(dest(5), text(5), cpW(), nil, text(5))
		}
		hit(t, e, 0, 63)
	})
	t.Run("覆寫過的 dest 變最新，不被淘汰", func(t *testing.T) {
		e := fill(64)
		e.sprintf(dest(0), "new0", cpW(), nil, "new0") // dest 0 變最新
		e.sprintf(dest(64), text(64), cpW(), nil, text(64))
		if p, _ := e.c.Match(dest(0), "new0"); p == nil {
			t.Errorf("覆寫過的 dest 0 被淘汰了")
		}
		cpMiss(t, e.c, dest(1), text(1), MissDest) // 現在最舊的是 dest 1
		hit(t, e, 2, 64)
	})
	t.Run("連續超出時依序淘汰", func(t *testing.T) {
		e := fill(67)
		for i := 0; i < 3; i++ {
			cpMiss(t, e.c, dest(i), text(i), MissDest)
		}
		hit(t, e, 3, 66)
	})
	t.Run("格式錯誤移除的記錄讓出容量", func(t *testing.T) {
		e := fill(64)
		e.sprintf(dest(10), "%x", cpW(), nil, "junk") // 移除 dest 10
		e.sprintf(dest(64), text(64), cpW(), nil, text(64))
		cpMiss(t, e.c, dest(10), text(10), MissDest)
		hit(t, e, 0, 9)
		hit(t, e, 11, 64) // 沒有任何記錄被多淘汰
	})
	t.Run("格式錯誤且 dest 沒記錄時不佔容量", func(t *testing.T) {
		e := fill(64)
		e.sprintf(dest(70), "%x", cpW(), nil, "junk")
		hit(t, e, 0, 63)
	})
}

func TestComposeSprintfBadFormat(t *testing.T) {
	thirteen := ""
	for i := 0; i < 13; i++ {
		thirteen += "%d"
	}
	cases := []struct {
		name   string
		format string
		strs   []ArgStr
	}{
		{"未支援的 %x", "%x", nil},
		{"尾端的 %", "100%", nil},
		{"%5% 不支援", "%5%", nil},
		{"%s 的字串引數不足", "%s %s", []ArgStr{cpStat(0x2000, "a")}},
		{"%s 沒有任何字串引數", "%s", nil},
		{"字組不足（13 個 %d，只有 12 個字組）", thirteen, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newCpEnv()
			// 先有舊記錄，格式錯誤的新呼叫要把它移除
			e.sprintf(cpLine, "good", cpW(), nil, "good")
			cpHit(t, e.c, cpLine, "good")
			e.sprintf(cpLine, tc.format, cpW(), tc.strs, "good")
			cpMiss(t, e.c, cpLine, "good", MissDest)

			// 新 dest 上的格式錯誤不留記錄
			e.sprintf(cpInner, tc.format, cpW(), tc.strs, "good")
			cpMiss(t, e.c, cpInner, "good", MissDest)
		})
	}
}

func TestComposeSprintfNestedCreatedAtS(t *testing.T) {
	e := newCpEnv()
	e.sprintf(cpInner2, "Orc", cpW(), nil, "Orc")
	e.sprintf(cpInner, "%s the Great", cpW(cpInner2), []ArgStr{cpBuf(cpInner2, "Orc")}, "Orc the Great")

	pb := cpHit(t, e.c, cpInner, "Orc the Great")
	pc := pb.ArgStrs[0].Piece
	if pc == nil || pc.Fmt != "Orc" || pc.Literal != "Orc" {
		t.Fatalf("內層 Piece = %+v，要 Fmt \"Orc\"、Literal \"Orc\"", pc)
	}

	// S 之後內層緩衝區被重用：記錄被新內容覆寫，外層記錄仍保有 S 時建立的 Piece
	e.sprintf(cpInner2, "Imp", cpW(), nil, "Imp")
	pb = cpHit(t, e.c, cpInner, "Orc the Great")
	if pc := pb.ArgStrs[0].Piece; pc == nil || pc.Literal != "Orc" {
		t.Errorf("內層覆寫後外層記錄的巢狀 Piece = %+v，要保留 Literal \"Orc\"", pc)
	}
	// 內層 dest 的新記錄是新內容
	cpHit(t, e.c, cpInner2, "Imp")

	// 三層：最外層記錄經內層的快照，連更內層的 Piece 也帶著
	e.sprintf(cpInner2, "Orc", cpW(), nil, "Orc") // 還原內層，供下一層 S 時關聯
	e.sprintf(cpInner, "%s the Great", cpW(cpInner2), []ArgStr{cpBuf(cpInner2, "Orc")}, "Orc the Great")
	e.sprintf(cpLine, "%s hits", cpW(cpInner), []ArgStr{cpBuf(cpInner, "Orc the Great")}, "Orc the Great hits")
	pa := cpHit(t, e.c, cpLine, "Orc the Great hits")
	pb2 := pa.ArgStrs[0].Piece
	if pb2 == nil || pb2.Literal != "Orc the Great" {
		t.Fatalf("第二層 Piece = %+v，要 Literal \"Orc the Great\"", pb2)
	}
	if pc2 := pb2.ArgStrs[0].Piece; pc2 == nil || pc2.Literal != "Orc" {
		t.Errorf("第三層 Piece = %+v，要 Literal \"Orc\"", pc2)
	}
}

// S 時只對 buffer 種類的 %s 引數建立關聯，且內容要與記錄吻合、不含 %。
// 依據：003 §5.1 的 buffer 區間含「sprintf 目的」，不是 buffer 的指標不會是組句緩衝區。
func TestComposeSprintfNestedOnlyMatchingBuffers(t *testing.T) {
	newInner := func() *cpEnv {
		e := newCpEnv()
		e.sprintf(cpInner, "Orc the Great", cpW(), nil, "Orc the Great")
		return e
	}
	piece := func(a ArgStr) *Piece {
		e := newInner()
		out := a.Content + "!" // 外層的組合字串就是引數內容加 "!"
		e.sprintf(cpLine, "%s!", cpW(cpInner), []ArgStr{a}, out)
		return cpHit(t, e.c, cpLine, out).ArgStrs[0].Piece
	}
	if got := piece(cpBuf(cpInner, "Orc the Great")); got == nil {
		t.Error("內容吻合的 buffer 引數沒有關聯")
	}
	for _, k := range []Kind{KindStatic, KindOther, KindMonster, KindTown} {
		if got := piece(ArgStr{Ptr: cpInner, Content: "Orc the Great", Kind: k}); got != nil {
			t.Errorf("Kind %v 的引數不該關聯，得到 %+v", k, got)
		}
	}
	if got := piece(cpBuf(cpInner, "Orc the Greatest")); got != nil {
		t.Errorf("內容不符的引數不該關聯，得到 %+v", got)
	}
	if got := piece(cpBuf(0x7777, "Orc the Great")); got != nil {
		t.Errorf("指標沒有記錄的引數不該關聯，得到 %+v", got)
	}
	// 含 % 的內層緩衝區不關聯。外層以 %.3s 取前三個字元，組合字串 "100!" 不含 %，外層本身可以命中。
	e := newCpEnv()
	e.sprintf(cpInner, "100%%", cpW(), nil, "100%")
	e.sprintf(cpLine, "%.3s!", cpW(cpInner), []ArgStr{cpBuf(cpInner, "100%")}, "100!")
	if got := cpHit(t, e.c, cpLine, "100!"); got.ArgStrs[0].Piece != nil {
		t.Errorf("含 %% 的內層不該關聯，得到 %+v", got.ArgStrs[0].Piece)
	}
}

// ---------------------------------------------------------------------------
// Match

func TestComposeMatchMisses(t *testing.T) {
	e := newCpEnv()
	e.sprintf(cpLine, "Orc hits you", cpW(), nil, "Orc hits you")
	e.sprintf(cpInner, "100%%", cpW(), nil, "100%")

	cpHit(t, e.c, cpLine, "Orc hits you")

	// 無記錄
	cpMiss(t, e.c, 0x7777, "Orc hits you", MissDest)
	// 記錄是目前字串的前綴：疑似有未追蹤的追加
	cpMiss(t, e.c, cpLine, "Orc hits you hard", MissPrefix)
	cpMiss(t, e.c, cpLine, "Orc hits youX", MissPrefix)
	// 內容不符：不同、較短（目前字串是記錄的前綴，方向相反）、大小寫不同、空
	cpMiss(t, e.c, cpLine, "Orc hits me", MissContent)
	cpMiss(t, e.c, cpLine, "Orc hits", MissContent)
	cpMiss(t, e.c, cpLine, "orc hits you", MissContent)
	cpMiss(t, e.c, cpLine, "", MissContent)
	// 含 %：與記錄相同也不關聯
	cpMiss(t, e.c, cpInner, "100%", MissPercent)
}

func TestComposeMatchNulCutBothSides(t *testing.T) {
	e := newCpEnv()
	e.sprintf(cpLine, "abc", cpW(), nil, "abc")
	// 目前字串截到第一個 NUL：NUL 之後的內容（含 %）不比對
	cpHit(t, e.c, cpLine, "abc\x00zz")
	cpHit(t, e.c, cpLine, "abc\x00%zz")
	cpHit(t, e.c, cpLine, "abc\x00")
	// NUL 在中間時，截斷後的 "ab" 不等於 "abc"
	cpMiss(t, e.c, cpLine, "ab\x00c", MissContent)
}

func TestComposeMatchSnapshot(t *testing.T) {
	e := newCpEnv()
	e.sprintf(cpLine, "%s:", cpW(0x2000), []ArgStr{cpStat(0x2000, "Elf")}, "Elf:")
	p := cpHit(t, e.c, cpLine, "Elf:")

	// 之後追加：快照的組合字串與 Appends 不變
	if r := e.strcat(cpLine, cpStat(0x5000, " Fight"), "Elf: Fight"); r != AppendDone {
		t.Fatalf("OnStrcat = %d，要 AppendDone", r)
	}
	if p.Literal != "Elf:" || len(p.Appends) != 0 {
		t.Errorf("追加之後快照 = Literal %q、Appends %v，要 \"Elf:\"、空", p.Literal, p.Appends)
	}
	// 之後覆寫：快照不變
	e.sprintf(cpLine, "Orc:", cpW(), nil, "Orc:")
	if p.Fmt != "%s:" || p.Literal != "Elf:" || p.ArgStrs[0].Content != "Elf" {
		t.Errorf("覆寫之後快照 = Fmt %q、Literal %q、ArgStrs %v，要 \"%%s:\"、\"Elf:\"、[Elf]", p.Fmt, p.Literal, cpContents(p.ArgStrs))
	}

	// 改動快照不影響之後取得的快照與記錄
	e2 := newCpEnv()
	e2.sprintf(cpLine, "%s:", cpW(0x2000), []ArgStr{cpStat(0x2000, "Elf")}, "Elf:")
	e2.strcat(cpLine, cpStat(0x5000, " Fight"), "Elf: Fight")
	q := cpHit(t, e2.c, cpLine, "Elf: Fight")
	q.Literal = "tampered"
	q.ArgStrs[0].Content = "tampered"
	q.Appends[0].Content = "tampered"
	q.Appends = append(q.Appends, cpStat(0, "extra"))
	r := cpHit(t, e2.c, cpLine, "Elf: Fight")
	if r.Literal != "Elf: Fight" || r.ArgStrs[0].Content != "Elf" || len(r.Appends) != 1 || r.Appends[0].Content != " Fight" {
		t.Errorf("改動快照後再取 = Literal %q、ArgStrs %v、Appends %v，要原內容",
			r.Literal, cpContents(r.ArgStrs), cpContents(r.Appends))
	}
}

// ---------------------------------------------------------------------------
// OnStrcat

func TestComposeStrcatAppend(t *testing.T) {
	e := newCpEnv()
	e.sprintf(cpLine, "%s:", cpW(0x2000), []ArgStr{cpStat(0x2000, "Elf")}, "Elf:")

	if r := e.strcat(cpLine, ArgStr{Ptr: 0x5000, Content: " Fight", Kind: KindStatic}, "Elf: Fight"); r != AppendDone {
		t.Fatalf("第一次追加 = %d，要 AppendDone", r)
	}
	// 組合字串變長：舊內容不再命中，新內容命中
	cpMiss(t, e.c, cpLine, "Elf:", MissContent)
	p := cpHit(t, e.c, cpLine, "Elf: Fight")
	if p.Literal != "Elf: Fight" {
		t.Errorf("Literal = %q，要 \"Elf: Fight\"", p.Literal)
	}
	if len(p.Appends) != 1 || p.Appends[0] != (ArgStr{Ptr: 0x5000, Content: " Fight", Kind: KindStatic}) {
		t.Errorf("Appends = %+v，要單筆 {5000, \" Fight\", static}", p.Appends)
	}

	// 第二次追加：依追加順序
	if r := e.strcat(cpLine, cpBuf(0x5010, " Spell"), "Elf: Fight Spell"); r != AppendDone {
		t.Fatalf("第二次追加 = %d，要 AppendDone", r)
	}
	p = cpHit(t, e.c, cpLine, "Elf: Fight Spell")
	if got, want := cpContents(p.Appends), []string{" Fight", " Spell"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Appends 內容 = %q，要 %q", got, want)
	}
	if p.Appends[1].Kind != KindBuffer || p.Appends[1].Ptr != 0x5010 {
		t.Errorf("Appends[1] = %+v，要保留 Ptr 5010、Kind buffer", p.Appends[1])
	}
	// 原 Fmt 與 ArgStrs 不變
	if p.Fmt != "%s:" || !reflect.DeepEqual(cpContents(p.ArgStrs), []string{"Elf"}) {
		t.Errorf("追加後 Fmt = %q、ArgStrs = %q，要 \"%%s:\"、[Elf]", p.Fmt, cpContents(p.ArgStrs))
	}
}

// IRQ0 在 Step 內送出，處理常式 IRET 後同一位址的 hook 再觸發一次：同一個 OnStrcat 連呼兩次，
// 兩次的 cur 都是第一次執行前的 DS 內容。第二次的組合字串已變長，不得重複追加（001 §3.2）。
func TestComposeStrcatIRQ0DoubleTrigger(t *testing.T) {
	e := newCpEnv()
	e.sprintf(cpLine, "Elf:", cpW(), nil, "Elf:")

	src := cpStat(0x5000, " Fight")
	r1 := e.c.OnStrcat(cpLine, "Elf:", src)
	r2 := e.c.OnStrcat(cpLine, "Elf:", src) // DS 還沒更新（strcat 尚未執行）
	if r1 != AppendDone {
		t.Errorf("第一次 = %d，要 AppendDone", r1)
	}
	if r2 != AppendMismatch {
		t.Errorf("第二次 = %d，要 AppendMismatch", r2)
	}
	e.mem.put(cpLine, "Elf: Fight")

	p := cpHit(t, e.c, cpLine, "Elf: Fight")
	if len(p.Appends) != 1 || p.Literal != "Elf: Fight" {
		t.Errorf("Appends = %q、Literal = %q，要單筆 \" Fight\"、\"Elf: Fight\"", cpContents(p.Appends), p.Literal)
	}
	// 之後正常的追加不受影響
	if r := e.strcat(cpLine, cpStat(0x5010, " Spell"), "Elf: Fight Spell"); r != AppendDone {
		t.Errorf("之後的追加 = %d，要 AppendDone", r)
	}
	if p := cpHit(t, e.c, cpLine, "Elf: Fight Spell"); len(p.Appends) != 2 {
		t.Errorf("Appends = %q，要兩筆", cpContents(p.Appends))
	}
}

func TestComposeStrcatNoRecord(t *testing.T) {
	e := newCpEnv()
	// 從沒記過的 dest（例如 overlay 載入器追加 .ovr 的堆疊緩衝區）
	if r := e.c.OnStrcat(0x7777, "OV1", cpStat(0x5000, ".OVR")); r != AppendNoRec {
		t.Errorf("無記錄的 dest = %d，要 AppendNoRec", r)
	}
	cpMiss(t, e.c, 0x7777, "OV1.TEST", MissDest) // 沒有因此建立記錄

	// 記錄被格式錯誤的 sprintf 移除之後同樣是無記錄
	e.sprintf(cpLine, "good", cpW(), nil, "good")
	e.sprintf(cpLine, "%x", cpW(), nil, "good")
	if r := e.strcat(cpLine, cpStat(0x5000, "!"), "good!"); r != AppendNoRec {
		t.Errorf("記錄已移除的 dest = %d，要 AppendNoRec", r)
	}
}

func TestComposeStrcatEmptySource(t *testing.T) {
	e := newCpEnv()
	e.sprintf(cpLine, "Elf:", cpW(), nil, "Elf:")
	// 來源為空字串：原始碼註解載明回 AppendDone，但不追加、不改變組合字串
	for i := 0; i < 2; i++ {
		if r := e.strcat(cpLine, cpStat(0x5000, ""), "Elf:"); r != AppendDone {
			t.Errorf("空來源 = %d，要 AppendDone", r)
		}
	}
	p := cpHit(t, e.c, cpLine, "Elf:")
	if len(p.Appends) != 0 || p.Literal != "Elf:" {
		t.Errorf("空來源後 Appends = %q、Literal = %q，要空、\"Elf:\"", cpContents(p.Appends), p.Literal)
	}
	// 之後正常追加
	if r := e.strcat(cpLine, cpStat(0x5000, " Fight"), "Elf: Fight"); r != AppendDone {
		t.Fatalf("之後的追加 = %d", r)
	}
	if p := cpHit(t, e.c, cpLine, "Elf: Fight"); len(p.Appends) != 1 {
		t.Errorf("Appends = %q，要單筆", cpContents(p.Appends))
	}
}

func TestComposeStrcatContentMismatch(t *testing.T) {
	e := newCpEnv()
	e.sprintf(cpLine, "Elf:", cpW(), nil, "Elf:")
	// DS 內容與記錄的組合字串不符（緩衝區被未追蹤的程式碼改過）：不追加
	for _, cur := range []string{"Orc:", "Elf: x", "Elf", ""} {
		if r := e.c.OnStrcat(cpLine, cur, cpStat(0x5000, " Fight")); r != AppendMismatch {
			t.Errorf("cur %q：OnStrcat = %d，要 AppendMismatch", cur, r)
		}
	}
	p := cpHit(t, e.c, cpLine, "Elf:")
	if len(p.Appends) != 0 {
		t.Errorf("不符時 Appends = %q，要空", cpContents(p.Appends))
	}
	// cur 也截到第一個 NUL：NUL 之後的殘留內容不影響判定
	if r := e.c.OnStrcat(cpLine, "Elf:\x00junk", cpStat(0x5000, " Fight")); r != AppendDone {
		t.Errorf("cur 含 NUL 與殘留：OnStrcat = %d，要 AppendDone", r)
	}
}

func TestComposeAppendsOnTopOfNestedPieces(t *testing.T) {
	// 追加不改動 S 時建立的巢狀 Piece
	e := newCpEnv()
	e.sprintf(cpInner, "Orc", cpW(), nil, "Orc")
	e.sprintf(cpLine, "%s hits", cpW(cpInner), []ArgStr{cpBuf(cpInner, "Orc")}, "Orc hits")
	e.strcat(cpLine, cpStat(0x5000, " you"), "Orc hits you")
	p := cpHit(t, e.c, cpLine, "Orc hits you")
	if p.ArgStrs[0].Piece == nil || p.ArgStrs[0].Piece.Literal != "Orc" || len(p.Appends) != 1 {
		t.Errorf("追加後 = 巢狀 %+v、Appends %q，要巢狀 Literal \"Orc\"、單筆追加", p.ArgStrs[0].Piece, cpContents(p.Appends))
	}
}

// ---------------------------------------------------------------------------
// PieceCombined、MissCounter

func TestComposePieceCombined(t *testing.T) {
	cases := []struct {
		name   string
		p      Piece
		want   string
		wantOK bool
	}{
		{"字面", Piece{Fmt: "Hello"}, "Hello", true},
		{"%s 與 %d", Piece{Fmt: "%s-%d", Args: cpW(0x2000, 7), ArgStrs: []ArgStr{{Content: "Elf"}}}, "Elf-7", true},
		{"接上各追加字串", Piece{Fmt: "%s", Args: cpW(0x2000), ArgStrs: []ArgStr{{Content: "Elf"}},
			Appends: []ArgStr{{Content: " X"}, {Content: "Y"}}}, "Elf XY", true},
		{"NUL 先截斷，再接追加", Piece{Fmt: "ab%cde", Args: cpW(0), Appends: []ArgStr{{Content: "Z"}}}, "abZ", true},
		{"格式錯誤", Piece{Fmt: "%x"}, "", false},
		{"字串引數不足", Piece{Fmt: "%s"}, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := PieceCombined(&tc.p)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("PieceCombined = (%q, %v)，要 (%q, %v)", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestComposeMissCounterNames(t *testing.T) {
	// 001 §9 的計數器名稱
	cases := []struct {
		m    Miss
		want string
	}{
		{MissDest, "composed_miss_dest"},
		{MissPrefix, "composed_miss_prefix"},
		{MissContent, "composed_miss_content"},
		{MissPercent, "composed_miss_percent"},
		{MissNone, ""},
	}
	for _, tc := range cases {
		if got := MissCounter(tc.m); got != tc.want {
			t.Errorf("MissCounter(%d) = %q，要 %q", tc.m, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Associate

func TestComposeAssociateFmtPtrComposed(t *testing.T) {
	e := newCpEnv()
	e.sprintf(cpLine, "%s hits %s for %d", cpW(0x2000, 0x2010, 7),
		[]ArgStr{cpStat(0x2000, "Gnoll"), cpStat(0x2010, "you")}, "Gnoll hits you for 7")

	rec := cpEvFmt(KindBuffer, cpLine)
	composed, misses := e.c.Associate(rec, e.mem.read)
	if composed != 1 {
		t.Errorf("composed = %d，要 1", composed)
	}
	cpMisses(t, misses, nil)
	if rec.Composed == nil {
		t.Fatal("Composed 為 nil，要整句組句關聯")
	}
	c := rec.Composed
	if c.Fmt != "%s hits %s for %d" || c.Literal != "Gnoll hits you for 7" || c.Args[2] != 7 {
		t.Errorf("Composed = Fmt %q、Literal %q、Args[2] %d，要 \"%%s hits %%s for %%d\"、\"Gnoll hits you for 7\"、7", c.Fmt, c.Literal, c.Args[2])
	}
	if got, want := cpContents(c.ArgStrs), []string{"Gnoll", "you"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Composed.ArgStrs = %q，要 %q", got, want)
	}
}

// 戰鬥指令列：25A5(0, 列, "%s", 638E)，格式字串是靜態 %s，引數是含 strcat 追加的組句緩衝區（001 §3.4 A 時第 2 點）。
func TestComposeAssociateBattleCommandLine(t *testing.T) {
	e := newCpEnv()
	e.sprintf(cpLine, "%s:", cpW(0x2000), []ArgStr{cpStat(0x2000, "Elf")}, "Elf:")
	e.strcat(cpLine, cpStat(0x5000, " Fight"), "Elf: Fight")
	e.strcat(cpLine, cpStat(0x5010, " Spell"), "Elf: Fight Spell")

	rec := cpEvArgs(cpBuf(cpLine, "Elf: Fight Spell"))
	composed, misses := e.c.Associate(rec, e.mem.read)
	cpMisses(t, misses, nil)
	if composed != 0 || rec.Composed != nil {
		t.Errorf("格式字串是靜態 %%s：composed = %d、Composed = %v，要 0、nil", composed, rec.Composed)
	}
	p := rec.ArgStrs[0].Piece
	if p == nil {
		t.Fatal("引數的 Piece 為 nil，要關聯到含追加的組句")
	}
	if p.Fmt != "%s:" || p.Literal != "Elf: Fight Spell" {
		t.Errorf("Piece = Fmt %q、Literal %q，要 \"%%s:\"、\"Elf: Fight Spell\"", p.Fmt, p.Literal)
	}
	if got, want := cpContents(p.Appends), []string{" Fight", " Spell"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Piece.Appends = %q，要 %q", got, want)
	}
	if got, want := cpContents(p.ArgStrs), []string{"Elf"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Piece.ArgStrs = %q，要 %q", got, want)
	}
}

// 沒有 strcat 掛點時，戰鬥指令列的組合字串停在 "Elf:"，目前內容多了追加的指令字：記 composed_miss_prefix（001 §10 第 5 項）。
func TestComposeAssociateBattleLineWithoutStrcatIsPrefixMiss(t *testing.T) {
	e := newCpEnv()
	e.sprintf(cpLine, "%s:", cpW(0x2000), []ArgStr{cpStat(0x2000, "Elf")}, "Elf:")
	e.mem.put(cpLine, "Elf: Fight Spell") // strcat 執行了，但沒有呼叫 OnStrcat

	rec := cpEvArgs(cpBuf(cpLine, "Elf: Fight Spell"))
	composed, misses := e.c.Associate(rec, e.mem.read)
	if composed != 0 || rec.ArgStrs[0].Piece != nil {
		t.Errorf("composed = %d、Piece = %v，要 0、nil", composed, rec.ArgStrs[0].Piece)
	}
	cpMisses(t, misses, map[Miss]int{MissPrefix: 1})

	// 計入診斷計數器：名稱依規格 §9
	ct := NewCounters()
	for m, n := range misses {
		ct.Add(MissCounter(m), uint64(n))
	}
	if got := ct.String(); got != "composed_miss_prefix=1" {
		t.Errorf("計數器 = %q，要 composed_miss_prefix=1", got)
	}
}

func TestComposeAssociateMisses(t *testing.T) {
	cases := []struct {
		name  string
		setup func(e *cpEnv)
		cur   string // 目前 DS:638E 的內容
		want  Miss
	}{
		{"dest 沒有記錄", func(e *cpEnv) {}, "Hello", MissDest},
		{"前綴相符", func(e *cpEnv) { e.sprintf(cpLine, "Elf:", cpW(), nil, "Elf:") }, "Elf: Fight", MissPrefix},
		{"內容不符", func(e *cpEnv) { e.sprintf(cpLine, "Elf:", cpW(), nil, "Elf:") }, "Orc:", MissContent},
		{"含 %", func(e *cpEnv) { e.sprintf(cpLine, "100%%", cpW(), nil, "100%") }, "100%", MissPercent},
	}
	for _, tc := range cases {
		t.Run(tc.name+"（格式字串指標）", func(t *testing.T) {
			e := newCpEnv()
			tc.setup(e)
			e.mem.put(cpLine, tc.cur)
			rec := cpEvFmt(KindBuffer, cpLine)
			composed, misses := e.c.Associate(rec, e.mem.read)
			if composed != 0 || rec.Composed != nil {
				t.Errorf("composed = %d、Composed = %v，要 0、nil", composed, rec.Composed)
			}
			cpMisses(t, misses, map[Miss]int{tc.want: 1})
		})
		t.Run(tc.name+"（%s 引數）", func(t *testing.T) {
			e := newCpEnv()
			tc.setup(e)
			e.mem.put(cpLine, tc.cur)
			rec := cpEvArgs(cpBuf(cpLine, tc.cur))
			composed, misses := e.c.Associate(rec, e.mem.read)
			if composed != 0 || rec.ArgStrs[0].Piece != nil {
				t.Errorf("composed = %d、Piece = %v，要 0、nil", composed, rec.ArgStrs[0].Piece)
			}
			cpMisses(t, misses, map[Miss]int{tc.want: 1})
		})
	}
}

// 只有 buffer 種類的指標才檢查，也只有它們計 miss：靜態字串、怪物、城鎮、other 都不是組句緩衝區。
func TestComposeAssociateOnlyBufferKind(t *testing.T) {
	e := newCpEnv()
	e.sprintf(cpLine, "Elf:", cpW(), nil, "Elf:")

	for _, k := range []Kind{KindStatic, KindOther, KindMonster, KindTown} {
		// 指標恰好是吻合的 dest：不關聯
		rec := cpEvFmt(k, cpLine)
		composed, misses := e.c.Associate(rec, e.mem.read)
		if composed != 0 || rec.Composed != nil {
			t.Errorf("FmtKind %v：composed = %d、Composed = %v，要 0、nil", k, composed, rec.Composed)
		}
		cpMisses(t, misses, nil)

		rec = &EventRecord{FmtKind: KindStatic, FmtPtr: cpFmt, ArgStrs: []ArgStr{{Ptr: cpLine, Content: "Elf:", Kind: k}}}
		_, misses = e.c.Associate(rec, e.mem.read)
		if rec.ArgStrs[0].Piece != nil {
			t.Errorf("引數 Kind %v：Piece = %v，要 nil", k, rec.ArgStrs[0].Piece)
		}
		cpMisses(t, misses, nil)

		// 指標沒有記錄：不是缺陷，不計 miss
		rec = cpEvFmt(k, 0x7777)
		_, misses = e.c.Associate(rec, e.mem.read)
		cpMisses(t, misses, nil)
		rec = &EventRecord{FmtKind: KindStatic, FmtPtr: cpFmt, ArgStrs: []ArgStr{{Ptr: 0x7777, Content: "x", Kind: k}}}
		_, misses = e.c.Associate(rec, e.mem.read)
		cpMisses(t, misses, nil)
	}

	// 對照：buffer 種類的引數指標沒有記錄時計 MissDest，兩個 buffer 引數各計一次
	rec := cpEvArgs(cpBuf(0x7777, "x"), cpBuf(0x7778, "y"), cpStat(0x7779, "z"))
	_, misses := e.c.Associate(rec, e.mem.read)
	cpMisses(t, misses, map[Miss]int{MissDest: 2})
}

// 同一個事件內，格式字串與每個 %s 引數各自檢查。
func TestComposeAssociateMultipleArgs(t *testing.T) {
	e := newCpEnv()
	e.sprintf(cpInner, "Orc", cpW(), nil, "Orc")
	e.sprintf(cpInner2, "Elf", cpW(), nil, "Elf")

	rec := &EventRecord{
		FmtKind: KindStatic, FmtPtr: cpFmt, Format: "%s %s %s",
		ArgStrs: []ArgStr{cpBuf(cpInner, "Orc"), cpStat(0x2000, "mid"), cpBuf(cpInner2, "Gone")},
	}
	composed, misses := e.c.Associate(rec, e.mem.read)
	if composed != 0 {
		t.Errorf("composed = %d，要 0", composed)
	}
	if p := rec.ArgStrs[0].Piece; p == nil || p.Literal != "Orc" {
		t.Errorf("引數 0 的 Piece = %+v，要 Literal \"Orc\"", p)
	}
	if rec.ArgStrs[1].Piece != nil {
		t.Errorf("靜態引數的 Piece = %+v，要 nil", rec.ArgStrs[1].Piece)
	}
	if rec.ArgStrs[2].Piece != nil {
		t.Errorf("內容不符的引數 Piece = %+v，要 nil", rec.ArgStrs[2].Piece)
	}
	cpMisses(t, misses, map[Miss]int{MissContent: 1})
}

// cpNestedEnv 建三層組句：C（cpInner2，"Orc"）→ B（cpInner，"Orc the Great"）→ A（cpLine，"Orc the Great hits"）。
func cpNestedEnv() *cpEnv {
	e := newCpEnv()
	e.sprintf(cpInner2, "Orc", cpW(), nil, "Orc")
	e.sprintf(cpInner, "%s the Great", cpW(cpInner2), []ArgStr{cpBuf(cpInner2, "Orc")}, "Orc the Great")
	e.sprintf(cpLine, "%s hits", cpW(cpInner), []ArgStr{cpBuf(cpInner, "Orc the Great")}, "Orc the Great hits")
	return e
}

// 巢狀 Piece 在 S 時建立；A 時只驗證其指標目前內容，不符就清掉。驗證作用在複本，不改動原 Piece 樹。
func TestComposeAssociateNestedValidated(t *testing.T) {
	const aText = "Orc the Great hits"
	cases := []struct {
		name  string
		tweak func(e *cpEnv)
		wantB bool // 第二層 B 保留
		wantC bool // 第三層 C 保留
	}{
		{"全部吻合", func(e *cpEnv) {}, true, true},
		{"C 的 DS 內容被覆寫", func(e *cpEnv) { e.mem.put(cpInner2, "Imp") }, true, false},
		{"C 的 DS 內容被追加", func(e *cpEnv) { e.mem.put(cpInner2, "Orc!") }, true, false},
		{"B 的 DS 內容被覆寫（連帶 C 一起清掉）", func(e *cpEnv) { e.mem.put(cpInner, "zzz") }, false, false},
		{"S 之後 C 的記錄被覆寫，但 DS 內容仍吻合：保留 S 時的 Piece", func(e *cpEnv) {
			e.c.OnSprintf(99, cpInner2, cpFmt, KindStatic, "Imp", cpW(), nil)
		}, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := cpNestedEnv()
			tc.tweak(e)

			rec := cpEvFmt(KindBuffer, cpLine)
			composed, misses := e.c.Associate(rec, e.mem.read)
			if composed != 1 || rec.Composed == nil {
				t.Fatalf("composed = %d、Composed = %v，要 1、非空", composed, rec.Composed)
			}
			cpMisses(t, misses, nil)
			b := rec.Composed.ArgStrs[0].Piece
			if (b != nil) != tc.wantB {
				t.Fatalf("B 的 Piece 存在 = %v，要 %v", b != nil, tc.wantB)
			}
			if b != nil {
				if b.Literal != "Orc the Great" {
					t.Errorf("B.Literal = %q，要 \"Orc the Great\"", b.Literal)
				}
				if c := b.ArgStrs[0].Piece; (c != nil) != tc.wantC {
					t.Errorf("C 的 Piece 存在 = %v，要 %v", c != nil, tc.wantC)
				} else if c != nil && c.Literal != "Orc" {
					t.Errorf("C.Literal = %q，要 \"Orc\"", c.Literal)
				}
			}

			// 原 Piece 樹不改動：直接向記錄取快照，三層仍在
			p := cpHit(t, e.c, cpLine, aText)
			pb := p.ArgStrs[0].Piece
			if pb == nil || pb.ArgStrs[0].Piece == nil {
				t.Fatalf("驗證之後記錄內的 Piece 樹被改動：B = %+v", pb)
			}
			if pb.ArgStrs[0].Piece.Literal != "Orc" {
				t.Errorf("記錄內 C.Literal = %q，要 \"Orc\"", pb.ArgStrs[0].Piece.Literal)
			}
			// 回傳的是複本：保留下來的巢狀 Piece 不是記錄內的同一個物件
			if b != nil && b == pb {
				t.Errorf("回傳的 B 與記錄內的 B 是同一個物件，要複本")
			}
			if b != nil && b.ArgStrs[0].Piece != nil && b.ArgStrs[0].Piece == pb.ArgStrs[0].Piece {
				t.Errorf("回傳的 C 與記錄內的 C 是同一個物件，要複本")
			}
		})
	}
}

// 引數路徑取得的 Piece 同樣驗證巢狀：戰鬥指令列的組句緩衝區含內層 Piece。
func TestComposeAssociateArgPathValidatesNested(t *testing.T) {
	for _, intact := range []bool{true, false} {
		e := cpNestedEnv()
		if !intact {
			e.mem.put(cpInner, "zzz")
		}
		rec := cpEvArgs(cpBuf(cpLine, "Orc the Great hits"))
		_, misses := e.c.Associate(rec, e.mem.read)
		cpMisses(t, misses, nil)
		p := rec.ArgStrs[0].Piece
		if p == nil {
			t.Fatalf("intact=%v：引數的 Piece 為 nil", intact)
		}
		if got := p.ArgStrs[0].Piece != nil; got != intact {
			t.Errorf("intact=%v：巢狀 Piece 存在 = %v，要 %v", intact, got, intact)
		}
	}
}

func TestComposeAssociateTwiceIsRepeatable(t *testing.T) {
	// 同一份記錄可對多個事件關聯：第一次的結果不影響第二次
	e := newCpEnv()
	e.sprintf(cpLine, "Elf:", cpW(), nil, "Elf:")
	for i := 0; i < 2; i++ {
		rec := cpEvFmt(KindBuffer, cpLine)
		composed, misses := e.c.Associate(rec, e.mem.read)
		cpMisses(t, misses, nil)
		if composed != 1 || rec.Composed == nil || rec.Composed.Literal != "Elf:" {
			t.Errorf("第 %d 次：composed = %d、Composed = %v，要 1、Literal \"Elf:\"", i+1, composed, rec.Composed)
		}
	}
}
