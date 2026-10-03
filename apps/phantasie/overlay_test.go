package phantasie

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// 疊字核心的單元測試（docs/spec/001 §3.2、§4、§7、§9、§10 第 1 項與第 5 項，004 §4、§5、§7 第 1 項與第 4 項）。
// 期望值都由規格推導成字面值，不拿被測函式的輸出當期望。輔助函式與型別一律加 ov 前綴。
//
// 版面速算：事件 Col、Row 的矩形左上角是 (Col×8, Row×8)，可用寬度 avail = 2×len(Text) h。
// 半形字 1 h（疊字格 4 像素），全形字 2 h（疊字格 8 像素），不足以空白補滿，所以疊字格寬度總和恆為 8×len(Text) 像素。

// ---------- 輔助 ----------

// ovWide 是測試字型的寬度函式：CJK、假名、全形為 2 h，其餘為 1 h。
func ovWide(r rune) bool { return r > 0x2E7F }

func ovSp(n int) string { return strings.Repeat(" ", n) }

// ovTWUI 是 zh-TW 的假 ui catalog（鍵是規範化英文，即去掉尾端空白）。
func ovTWUI() map[string]string {
	return map[string]string{
		"Hello":    "你好",
		"Bye":      "再見",
		"OK":       "ok",
		"Hi":       "你好世界",
		"Ho":       "a你好",
		"AB":       "甲乙",
		"XY":       "乙",
		"ABCD":     "甲",
		"ABCDEF":   "甲乙丙丁戊己",
		"WXYZ":     "子丑寅卯",
		"ABCDEFGH": "甲乙丙丁",
		"+Sound":   "+開音效",
		"-Sound":   "-關音效",
		"Mode +":   "模式開",
		"Mode -":   "模式關",
		"Sound %s": "音效%s",
		"Sound":    "音效",
		"Mode ++":  "雙開",
		"Mode -+":  "半關",
		"Mode +-":  "右關",
		"Mode --":  "雙關",
		"Bad":      "a\nb", // 含換行的譯文是版面錯誤
	}
}

// ovJAUI 是第二種語言的假 ui catalog，切段方式與 zh-TW 不同（例如 ABCDEFGH 是半形接全形）。
func ovJAUI() map[string]string {
	return map[string]string{
		"Hello":    "こんにちは",
		"Bye":      "では",
		"AB":       "はひ",
		"XY":       "ひ",
		"ABCD":     "さしす",
		"ABCDEF":   "あいうえおか",
		"WXYZ":     "さしすせ",
		"ABCDEFGH": "abあいう",
		"+Sound":   "+オン",
		"-Sound":   "-オフ",
	}
}

func ovLang(name string, ui map[string]string) *Language {
	return &Language{Name: name, Cat: NewCatalog(ui, nil, nil), Font: &xlate.Font{Name: "ovfont-" + name}, Wide: ovWide, Enabled: true}
}

// ovNew 建立 Overlay、註冊語言，並把顯示語言設為第一個。
func ovNew(t *testing.T, langs ...*Language) *Overlay {
	t.Helper()
	o := NewOverlay()
	for _, l := range langs {
		o.AddLanguage(l)
	}
	if len(langs) > 0 {
		if err := o.SetDisplay(langs[0].Name); err != nil {
			t.Fatalf("SetDisplay(%s)：%v", langs[0].Name, err)
		}
	}
	return o
}

func ovNewTW(t *testing.T) *Overlay {
	t.Helper()
	return ovNew(t, ovLang("zh-TW", ovTWUI()))
}

// 事件記錄的預設識別。同一個 Begin 到 End 之間不會重複，所以不同事件共用這組值不影響配對。
const (
	ovSP     = 0x1000
	ovBP     = 0x1010
	ovCaller = 0x2000
	ovFmtPtr = 0x3000
)

// ovRec 是字面事件：Format 等於 Text，沒有轉換。
func ovRec(col, row int, text string) *EventRecord {
	return &EventRecord{
		SP: ovSP, BP: ovBP, Caller: ovCaller, Col: col, Row: row,
		Text: text, FmtPtr: ovFmtPtr, Format: text, FmtKind: KindStatic, Cells: []byte(text),
	}
}

// ovRecS 是格式恆為單一 %s 的事件，引數內容就是 Text。
func ovRecS(col, row int, content string, kind Kind) *EventRecord {
	r := ovRec(col, row, content)
	r.Format = "%s"
	r.ArgStrs = []ArgStr{{Ptr: 0x4000, Content: content, Kind: kind}}
	return r
}

// ovRecC 是單字元事件（%c，引數是該字元）。
func ovRecC(col, row int, ch byte) *EventRecord {
	r := ovRec(col, row, string([]byte{ch}))
	r.Format = "%c"
	r.Args[0] = uint16(ch)
	return r
}

// ovFire 送一組配對的 A、B。
func ovFire(o *Overlay, r *EventRecord) *EventRecord {
	o.Begin(r, false)
	o.End()
	return r
}

// ovS 是一筆疊字的摘要。Tr 每格一個字元：T 是透明格，F 是不透明格。
type ovS struct {
	Key          string
	X, Y         int
	Cells, CellW int
	Text         string
	Tr           string
}

func ovDump(o *Overlay) []ovS {
	out := make([]ovS, 0, len(o.Layer.Stamps))
	for _, s := range o.Layer.Stamps {
		tr := make([]byte, s.Cells)
		for i := range tr {
			tr[i] = 'F'
			if i < len(s.Transparent) && s.Transparent[i] {
				tr[i] = 'T'
			}
		}
		out = append(out, ovS{s.Key, s.X, s.Y, s.Cells, s.CellW, string(s.Text), string(tr)})
	}
	return out
}

func ovFmtDump(rows []ovS) string {
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "\n  {%s X=%d Y=%d Cells=%d CellW=%d Text=%q Tr=%s}", r.Key, r.X, r.Y, r.Cells, r.CellW, r.Text, r.Tr)
	}
	if len(rows) == 0 {
		b.WriteString(" (空)")
	}
	return b.String()
}

func ovCheckDump(t *testing.T, o *Overlay, want ...ovS) {
	t.Helper()
	got := ovDump(o)
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Layer 疊字不符\n got:%s\nwant:%s", ovFmtDump(got), ovFmtDump(want))
	}
}

func ovCheckCounters(t *testing.T, o *Overlay, want map[string]uint64) {
	t.Helper()
	if got := o.C.Snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("計數器不符\n got: %s\nwant: %v", o.C.String(), want)
	}
}

func ovCheckStrs(t *testing.T, what string, got, want []string) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %q，要 %q", what, got, want)
	}
}

// ovSame 回兩個疊字切片是否逐一指向同一個物件。
func ovSame(a, b []*xlate.Stamp) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ovKeys 回 Layer 內事件組的 Key，依各組第一筆疊字的出現順序。
func ovKeys(o *Overlay) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range o.Layer.Stamps {
		if !seen[s.Key] {
			seen[s.Key] = true
			out = append(out, s.Key)
		}
	}
	return out
}

// ovOpaque 回與矩形 [x0,x1)×[y0,y1) 相交的非透明疊字格數。
func ovOpaque(o *Overlay, x0, y0, x1, y1 int) int {
	n := 0
	for _, s := range o.Layer.Stamps {
		if s.Y >= y1 || s.Y+s.CellH <= y0 {
			continue
		}
		for i := 0; i < s.Cells; i++ {
			if i < len(s.Transparent) && s.Transparent[i] {
				continue
			}
			cx0 := s.X + i*s.CellW
			if cx0 < x1 && x0 < cx0+s.CellW {
				n++
			}
		}
	}
	return n
}

// ovVisible 回 key 組目前可見的像素 x 範圍（逐像素欄掃描後合併，半開區間）。
func ovVisible(o *Overlay, key string) [][2]int {
	var vis [320]bool
	for _, s := range o.Layer.Stamps {
		if s.Key != key {
			continue
		}
		for i := 0; i < s.Cells; i++ {
			if i < len(s.Transparent) && s.Transparent[i] {
				continue
			}
			for x := s.X + i*s.CellW; x < s.X+(i+1)*s.CellW; x++ {
				if x >= 0 && x < 320 {
					vis[x] = true
				}
			}
		}
	}
	var out [][2]int
	for x := 0; x < 320; x++ {
		if !vis[x] {
			continue
		}
		if n := len(out); n > 0 && out[n-1][1] == x {
			out[n-1][1] = x + 1
			continue
		}
		out = append(out, [2]int{x, x + 1})
	}
	return out
}

// ovSpy 包住 Catalog，記錄 UI 的查詢順序，用來觀察重建發生在提交之前還是之後。
type ovSpy struct {
	*Catalog
	calls *[]string
}

func (s ovSpy) UI(key string) (string, bool) {
	*s.calls = append(*s.calls, key)
	return s.Catalog.UI(key)
}

// ovOptionRec 是選項選單的開關符號事件：格式恆為單一 %s，引數指向靜態字串 "+Sound "（呼叫端 107E）。
func ovOptionRec() *EventRecord {
	r := ovRecS(4, 6, "+Sound ", KindStatic)
	r.Caller = 0x107E
	return r
}

// ovOptionDump 是選項事件 id 的疊字（Col 4、Row 6，Y 位移 dy）：avail 14 h，"+開音效" 7 h，補 7 個空白。
func ovOptionDump(id string, dy int, text string) []ovS {
	return []ovS{
		{id, 32, 48 + dy, 1, 4, text[:1], "F"},
		{id, 36, 48 + dy, 3, 8, text[1:], "FFF"},
		{id, 60, 48 + dy, 7, 4, ovSp(7), "FFFFFFF"},
	}
}

// ---------- 事件分類 E、K、N ----------

func TestOverlayEventEmptyIgnored(t *testing.T) {
	o := ovNewTW(t)
	ovFire(o, ovRec(2, 3, "Hello"))
	before := append([]*xlate.Stamp(nil), o.Layer.Stamps...)
	dump := ovDump(o)
	ovFire(o, ovRec(2, 3, ""))
	if !ovSame(before, o.Layer.Stamps) || !reflect.DeepEqual(dump, ovDump(o)) {
		t.Fatalf("E 類不得動 Layer：\n%s", ovFmtDump(ovDump(o)))
	}
	// E 類的 B 也配對成功，不計 dup_close，也不計 blank、clipped 等。
	ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 1})
}

func TestOverlayEventBlank(t *testing.T) {
	hello := []ovS{
		{"g1", 16, 24, 2, 8, "你好", "FF"},
		{"g1", 32, 24, 6, 4, ovSp(6), "FFFFFF"},
	}
	t.Run("整列清除", func(t *testing.T) {
		o := ovNewTW(t)
		ovFire(o, ovRec(2, 3, "Hello"))
		ovCheckDump(t, o, hello...)
		ovFire(o, ovRec(2, 3, ovSp(5))) // 矩形 [16,56)×[24,32) 蓋住兩筆疊字
		ovCheckDump(t, o)
		ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 1, "blank": 1})
	})
	t.Run("部分覆蓋只標透明", func(t *testing.T) {
		o := ovNewTW(t)
		ovFire(o, ovRec(2, 3, "Hello"))
		ovFire(o, ovRec(2, 3, " ")) // 矩形 [16,24)：全形字的第 0 格
		ovCheckDump(t, o,
			ovS{"g1", 16, 24, 2, 8, "你好", "TF"},
			ovS{"g1", 32, 24, 6, 4, ovSp(6), "FFFFFF"})
		ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 1, "blank": 1})
	})
	t.Run("裁切到畫布", func(t *testing.T) {
		o := ovNewTW(t)
		ovFire(o, ovRec(0, 25, "  ")) // Row 25：y0=200，矩形落在畫布外
		ovCheckCounters(t, o, map[string]uint64{"events": 1, "blank": 1, "clipped": 1})
		o = ovNewTW(t)
		ovFire(o, ovRec(38, 0, ovSp(4))) // Col 38 起 4 個字元：x0=304，右緣 336 超過 320
		ovCheckCounters(t, o, map[string]uint64{"events": 1, "blank": 1, "clipped": 1})
	})
}

func TestOverlayEventNonPrintable(t *testing.T) {
	cases := []struct {
		name string
		text string
		want map[string]uint64
		keys []string
	}{
		{"控制字元與高位元組", "\x05\xC4\x05", map[string]uint64{"nonprintable": 1}, []string{"05", "C4"}},
		{"7F 是非可列印", "A\x7F", map[string]uint64{"nonprintable": 1}, []string{"7F"}},
		{"1F 是非可列印", "\x1f", map[string]uint64{"nonprintable": 1}, []string{"1F"}},
		{"80 是非可列印", "\x80", map[string]uint64{"nonprintable": 1}, []string{"80"}},
		{"7E 是可列印，走 T 類（沒有字母，passthrough）", "~", map[string]uint64{"passthrough": 1}, nil},
		{"20 是空白，走 K 類", " ", map[string]uint64{"blank": 1}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := ovNewTW(t)
			ovFire(o, ovRec(2, 3, "Hello"))
			ovFire(o, ovRec(2, 3, c.text))
			want := map[string]uint64{"events": 2, "translated": 1}
			for k, v := range c.want {
				want[k] = v
			}
			ovCheckCounters(t, o, want)
			ovCheckStrs(t, "KeySet(nonprintable)", o.C.KeySet("nonprintable"), c.keys)
		})
	}
	// N 類對事件矩形 Clear：矩形 [16,40)×[24,32)，全形段整筆被蓋，半形段前兩格轉透明。
	o := ovNewTW(t)
	ovFire(o, ovRec(2, 3, "Hello"))
	ovFire(o, ovRec(2, 3, "\x05\xC4\x05"))
	ovCheckDump(t, o, ovS{"g1", 32, 24, 6, 4, ovSp(6), "TTFFFF"})
}

// ---------- 事件分類 T ----------

func TestOverlayTranslateFields(t *testing.T) {
	o := ovNewTW(t)
	r := ovFire(o, ovRec(2, 3, "Hello"))
	if r.ID != "g1" {
		t.Fatalf("事件編號 = %q，要 g1", r.ID)
	}
	ovCheckDump(t, o,
		ovS{"g1", 16, 24, 2, 8, "你好", "FF"},
		ovS{"g1", 32, 24, 6, 4, ovSp(6), "FFFFFF"})
	lang := o.Language("zh-TW")
	span := 0
	for _, s := range o.Layer.Stamps {
		if s.CellH != 8 || s.GlyphX != 0 || s.GlyphY != 0 || s.GlyphScale != 1 || s.State != xlate.Pending {
			t.Fatalf("疊字欄位不符：CellH=%d GlyphX=%d GlyphY=%d GlyphScale=%d State=%v", s.CellH, s.GlyphX, s.GlyphY, s.GlyphScale, s.State)
		}
		if s.Font != lang.Font || o.Layer.FontRegistry[s.Font.Name] != s.Font {
			t.Fatalf("疊字的字型要是語言通道的字型，且登記在 FontRegistry")
		}
		if s.SwapColors || s.Owner != "" || s.Transparent != nil {
			t.Fatalf("一般事件的疊字不用 SwapColors、Owner、Transparent")
		}
		span += s.Cells * s.CellW
	}
	if span != 8*len("Hello") {
		t.Fatalf("疊字格寬度總和 = %d 像素，要 %d（補白滿 2×len(Text) h）", span, 8*len("Hello"))
	}
	ovCheckCounters(t, o, map[string]uint64{"events": 1, "translated": 1})
	rec := o.Record("g1")
	if rec == nil || rec.Text != "Hello" {
		t.Fatalf("records[g1] = %+v", rec)
	}
	ovCheckStrs(t, "Hits(g1)", o.Hits("g1"), []string{"Hello"})
	ovCheckStrs(t, "KeysShown", o.KeysShown(), []string{"Hello"})
	if want := []LogEvent{{N: 1, ID: "g1", Col: 2, Row: 3, Text: "Hello"}}; !reflect.DeepEqual(o.Log(), want) {
		t.Fatalf("稽核日誌 = %+v，要 %+v", o.Log(), want)
	}
	if !o.Drawing() {
		t.Fatal("顯示語言是 zh-TW 時要呼叫 Layer.Draw")
	}
}

func TestOverlayTranslateNarrowAndCenter(t *testing.T) {
	t.Run("全半形譯文只有一段，CellW 為 4", func(t *testing.T) {
		o := ovNewTW(t)
		ovFire(o, ovRec(0, 0, "OK")) // "ok" 2 h，avail 4 h，補 2 個空白
		ovCheckDump(t, o, ovS{"g1", 0, 0, 4, 4, "ok  ", "FFFF"})
	})
	t.Run("置中標記", func(t *testing.T) {
		o := ovNew(t, ovLang("zh-TW", map[string]string{"Hello": string(CenterMark) + "你好"}))
		ovFire(o, ovRec(2, 3, "Hello")) // avail 10 h，w=4，左補 floor((10-4)/2)=3 h，右補 3 h
		ovCheckDump(t, o,
			ovS{"g1", 16, 24, 3, 4, "   ", "FFF"},
			ovS{"g1", 28, 24, 2, 8, "你好", "FF"},
			ovS{"g1", 44, 24, 3, 4, "   ", "FFF"})
	})
}

func TestOverlayClipAndTruncate(t *testing.T) {
	t.Run("Col 加長度超過 40 裁切", func(t *testing.T) {
		o := ovNewTW(t)
		ovFire(o, ovRec(38, 1, "Hello")) // 可見 2 格，avail 4 h，"你好" 剛好 4 h
		ovCheckDump(t, o, ovS{"g1", 304, 8, 2, 8, "你好", "FF"})
		ovCheckCounters(t, o, map[string]uint64{"events": 1, "translated": 1, "clipped": 1})
	})
	t.Run("譯文超過 avail 時取最長整字前綴", func(t *testing.T) {
		o := ovNewTW(t)
		ovFire(o, ovRec(0, 0, "Hi")) // avail 4 h，譯文 8 h
		ovCheckDump(t, o, ovS{"g1", 0, 0, 2, 8, "你好", "FF"})
		ovCheckCounters(t, o, map[string]uint64{"events": 1, "translated": 1, "truncated": 1})
		ovCheckStrs(t, "KeySet(truncated)", o.C.KeySet("truncated"), []string{"Hi"})
	})
	t.Run("整字前綴不拆全形字", func(t *testing.T) {
		o := ovNewTW(t)
		ovFire(o, ovRec(0, 0, "Ho")) // "a你好"：a 1 h、你 2 h 共 3 h，好放不下，補 1 個空白
		ovCheckDump(t, o,
			ovS{"g1", 0, 0, 1, 4, "a", "F"},
			ovS{"g1", 4, 0, 1, 8, "你", "F"},
			ovS{"g1", 12, 0, 1, 4, " ", "F"})
		ovCheckCounters(t, o, map[string]uint64{"events": 1, "translated": 1, "truncated": 1})
	})
}

func TestOverlayUntranslatedClearsOldText(t *testing.T) {
	// 001 §10 第 5 項的負對照：先印中文，再以未譯英文覆寫同一位置，事件矩形內不得有未透明格。
	o := ovNewTW(t)
	ovFire(o, ovRec(2, 3, "Hello"))
	ovFire(o, ovRec(2, 3, "Missing"))
	if n := ovOpaque(o, 16, 24, 72, 32); n != 0 {
		t.Fatalf("事件矩形內仍有 %d 個未透明格：%s", n, ovFmtDump(ovDump(o)))
	}
	ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 1, "untranslated": 1})
	ovCheckStrs(t, "KeySet(untranslated)", o.C.KeySet("untranslated"), []string{"Missing"})
	ovCheckStrs(t, "KeySet(untranslated_at)", o.C.KeySet("untranslated_at"), []string{"2000|Missing"})
	if o.Record("g2") != nil {
		t.Fatal("缺譯事件不留記錄")
	}
	if len(o.Log()) != 1 {
		t.Fatalf("缺譯事件不進稽核日誌：%+v", o.Log())
	}

	// 部分覆蓋：矩形 [16,48) 蓋住全形段整筆，半形段前 4 格轉透明。
	o = ovNewTW(t)
	ovFire(o, ovRec(2, 3, "Hello"))
	ovFire(o, ovRec(2, 3, "Zork"))
	ovCheckDump(t, o, ovS{"g1", 32, 24, 6, 4, ovSp(6), "TTTTFF"})
}

func TestOverlayUntranslatedIdentityArgs(t *testing.T) {
	// 單一 %s 事件的靜態引數缺譯：走 identity，計一次 untranslated（無鍵），Missed 鍵照常列入 untranslated_args（001 §7）。
	o := ovNewTW(t)
	ovFire(o, ovRec(2, 3, "Hello"))
	ovFire(o, ovRecS(2, 3, "Zork", KindStatic))
	ovCheckDump(t, o, ovS{"g1", 32, 24, 6, 4, ovSp(6), "TTTTFF"})
	ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 1, "untranslated": 1, "untranslated_args": 1})
	ovCheckStrs(t, "KeySet(untranslated_args)", o.C.KeySet("untranslated_args"), []string{"Zork"})
	ovCheckStrs(t, "KeySet(untranslated)", o.C.KeySet("untranslated"), nil)
	ovCheckStrs(t, "KeySet(untranslated_at)", o.C.KeySet("untranslated_at"), []string{"2000|%s"})
	ovCheckStrs(t, "KeySet(untranslated_args_at)", o.C.KeySet("untranslated_args_at"), []string{"2000|%s"})

	// 模板有譯文、靜態引數缺譯：事件照常翻譯（OK），Missed 仍列入 untranslated_args。
	o = ovNewTW(t)
	r := ovRec(2, 3, "Sound Zork")
	r.Format = "Sound %s"
	r.ArgStrs = []ArgStr{{Ptr: 0x4000, Content: "Zork", Kind: KindStatic}}
	ovFire(o, r)
	ovCheckDump(t, o, // "音效Zork" 8 h，avail 20 h，補 12 個空白
		ovS{"g1", 16, 24, 2, 8, "音效", "FF"},
		ovS{"g1", 32, 24, 16, 4, "Zork" + ovSp(12), "FFFFFFFFFFFFFFFF"})
	ovCheckCounters(t, o, map[string]uint64{"events": 1, "translated": 1, "untranslated_args": 1})
	ovCheckStrs(t, "KeySet(untranslated_args)", o.C.KeySet("untranslated_args"), []string{"Zork"})
	ovCheckStrs(t, "KeySet(untranslated_args_at)", o.C.KeySet("untranslated_args_at"), []string{"2000|Sound %s"})
	ovCheckStrs(t, "Hits(g1)", o.Hits("g1"), []string{"Sound %s"})
}

func TestOverlayUntranslatedReasonsClearAndCount(t *testing.T) {
	protected := &Language{Name: "zh-TW", Cat: NewCatalog(ovTWUI(), nil, []string{"Secret"}), Font: &xlate.Font{Name: "ovfont-zh-TW"}, Wide: ovWide, Enabled: true}
	badarg := ovRecS(2, 3, "A", KindStatic)
	badarg.ArgStrs[0].Content = "A\x01" // 引數含非可列印的位元組
	cases := []struct {
		name  string
		rec   *EventRecord
		want  map[string]uint64
		wantK string // untranslated_at 的鍵，空字串表示不記
	}{
		{"保護清單", ovRec(2, 3, "Secret"), map[string]uint64{"protected": 1}, ""},
		{"沒有字母", ovRec(2, 3, "123"), map[string]uint64{"passthrough": 1}, ""},
		{"格式尾端的 %", ovRec(2, 3, "100%"), map[string]uint64{"untranslated": 1, "badformat": 1}, "2000|100%"},
		{"引數含非可列印位元組", badarg, map[string]uint64{"untranslated": 1, "badarg": 1}, "2000|%s"},
		{"譯文含換行", ovRec(2, 3, "Bad"), map[string]uint64{"untranslated": 1, "badformat": 1}, "2000|Bad"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := ovNew(t, protected)
			ovFire(o, ovRec(2, 3, "Hello"))
			ovFire(o, c.rec)
			// 一律對事件矩形 Clear，原文照常顯示。
			if n := ovOpaque(o, 16, 24, 16+8*len(c.rec.Text), 32); n != 0 {
				t.Fatalf("事件矩形內仍有 %d 個未透明格：%s", n, ovFmtDump(ovDump(o)))
			}
			want := map[string]uint64{"events": 2, "translated": 1}
			for k, v := range c.want {
				want[k] = v
			}
			ovCheckCounters(t, o, want)
			var wantK []string
			if c.wantK != "" {
				wantK = []string{c.wantK}
			}
			ovCheckStrs(t, "KeySet(untranslated_at)", o.C.KeySet("untranslated_at"), wantK)
			if o.Record("g2") != nil {
				t.Fatal("不翻譯的事件不留記錄")
			}
		})
	}
}

// ---------- A、B 配對與提交時機 ----------

func TestOverlayCommitAtEndNotBegin(t *testing.T) {
	o := ovNewTW(t)
	ovFire(o, ovRec(2, 3, "Hello"))
	before := append([]*xlate.Stamp(nil), o.Layer.Stamps...)
	beforeDump := ovDump(o)

	// 缺譯事件：提交時會 Clear，A 之後 B 之前舊疊字要繼續遮住。
	miss := ovRec(2, 3, "Missing")
	o.Begin(miss, false)
	if rec, ok := o.Open(); !ok || rec != miss || miss.ID != "g2" {
		t.Fatalf("Open() = %v %v，ID=%q", rec, ok, miss.ID)
	}
	if !ovSame(before, o.Layer.Stamps) || !reflect.DeepEqual(beforeDump, ovDump(o)) {
		t.Fatalf("A 之後、B 之前 Layer 要不變：\n%s", ovFmtDump(ovDump(o)))
	}
	ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 1})
	o.End()
	ovCheckDump(t, o)
	ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 1, "untranslated": 1})
	if _, ok := o.Open(); ok {
		t.Fatal("B 之後不再有開啟中的事件")
	}

	// 可翻譯事件：B 之前不加疊字，B 之後才有。
	bye := ovRec(10, 10, "Bye")
	o.Begin(bye, false)
	ovCheckDump(t, o)
	o.End()
	ovCheckDump(t, o,
		ovS{"g3", 80, 80, 2, 8, "再見", "FF"},
		ovS{"g3", 96, 80, 2, 4, "  ", "FF"})
}

func TestOverlayPairingDupOpen(t *testing.T) {
	o := ovNewTW(t)
	r1 := ovRec(2, 3, "Hello")
	r1d := ovRec(2, 3, "Hello") // 同一次呼叫被 IRQ0 重複觸發：SP、BP、Caller、Col、Row、FmtPtr、Text 都相同
	r1d.Step = 99               // Step 不在識別內
	if !o.Begin(r1, false) {
		t.Fatal("開啟新事件的 Begin 要回 true")
	}
	if o.Begin(r1d, false) {
		t.Fatal("重入的 Begin 要回 false")
	}
	if r1d.ID != "" {
		t.Fatalf("重入的記錄不得拿到事件編號：%q", r1d.ID)
	}
	if rec, ok := o.Open(); !ok || rec != r1 {
		t.Fatal("重入後開啟中的事件仍是第一個")
	}
	ovCheckCounters(t, o, map[string]uint64{"events": 1, "dup_open": 1})
	o.End()
	ovCheckCounters(t, o, map[string]uint64{"events": 1, "dup_open": 1, "translated": 1})
	ovCheckDump(t, o,
		ovS{"g1", 16, 24, 2, 8, "你好", "FF"},
		ovS{"g1", 32, 24, 6, 4, ovSp(6), "FFFFFF"})
	next := ovRec(10, 10, "Bye")
	o.Begin(next, false)
	if next.ID != "g2" {
		t.Fatalf("重入不得消耗事件編號：下一個是 %q，要 g2", next.ID)
	}
	o.End()

	// skip 的事件同樣去重。
	o = ovNewTW(t)
	o.Begin(ovRec(5, 5, "Bye"), true)
	o.Begin(ovRec(5, 5, "Bye"), true)
	ovCheckCounters(t, o, map[string]uint64{"events": 1, "dup_open": 1})
}

func TestOverlayPairingUnpaired(t *testing.T) {
	cases := []struct {
		name string
		mut  func(r *EventRecord)
	}{
		{"SP", func(r *EventRecord) { r.SP++ }},
		{"BP", func(r *EventRecord) { r.BP++ }},
		{"Caller", func(r *EventRecord) { r.Caller++ }},
		{"FmtPtr", func(r *EventRecord) { r.FmtPtr++ }},
		{"Col", func(r *EventRecord) { r.Col = 5 }},
		{"Row", func(r *EventRecord) { r.Row = 4 }},
		{"Text", func(r *EventRecord) { r.Text, r.Format, r.Cells = "Bye", "Bye", []byte("Bye") }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := ovNewTW(t)
			r1 := ovRec(2, 3, "Hello")
			r2 := ovRec(2, 3, "Hello")
			c.mut(r2)
			o.Begin(r1, false)
			if !o.Begin(r2, false) { // 識別不同：上一個事件缺 B，先提交
				t.Fatal("識別不同的 Begin 要開啟新事件（回 true）")
			}
			if r1.ID != "g1" || r2.ID != "g2" {
				t.Fatalf("事件編號 = %q、%q，要 g1、g2", r1.ID, r2.ID)
			}
			if rec, ok := o.Open(); !ok || rec != r2 {
				t.Fatal("新事件要成為開啟中的事件")
			}
			if keys := ovKeys(o); !reflect.DeepEqual(keys, []string{"g1"}) {
				t.Fatalf("第一個事件要已提交、第二個尚未：Layer 事件組 = %v", keys)
			}
			ovCheckCounters(t, o, map[string]uint64{"events": 2, "unpaired": 1, "translated": 1})
			o.End()
			ovCheckCounters(t, o, map[string]uint64{"events": 2, "unpaired": 1, "translated": 2})
		})
	}
}

func TestOverlayPairingDupClose(t *testing.T) {
	o := ovNewTW(t)
	o.End() // 沒有開啟中的事件
	ovCheckCounters(t, o, map[string]uint64{"dup_close": 1})
	ovFire(o, ovRec(2, 3, "Hello"))
	dump := ovDump(o)
	o.End() // 配對完成後再來一個 B
	ovCheckCounters(t, o, map[string]uint64{"dup_close": 2, "events": 1, "translated": 1})
	if !reflect.DeepEqual(dump, ovDump(o)) {
		t.Fatal("重複的 B 不得動 Layer")
	}
}

func TestOverlayPairingSkipEvents(t *testing.T) {
	// badlen、truncated_input：照常開啟並配對，提交時不動 Layer。
	o := ovNewTW(t)
	ovFire(o, ovRec(2, 3, "Hello"))
	before := append([]*xlate.Stamp(nil), o.Layer.Stamps...)
	dump := ovDump(o)

	o.Begin(ovRec(2, 3, "Missing"), true) // 不是 skip 的話會 Clear
	if _, ok := o.Open(); !ok {
		t.Fatal("skip 事件也要開啟")
	}
	o.End()
	if !ovSame(before, o.Layer.Stamps) || !reflect.DeepEqual(dump, ovDump(o)) {
		t.Fatal("skip 事件的提交不得動 Layer")
	}
	ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 1}) // 沒有 dup_close、untranslated

	o.End()
	ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 1, "dup_close": 1})

	// skip 事件缺 B：計 unpaired，被「提交」時仍不動 Layer。
	s := ovRec(20, 20, "Bye")
	o.Begin(s, true)
	o.Begin(ovRec(21, 21, "Bye"), false)
	if got := o.C.Get("unpaired"); got != 1 {
		t.Fatalf("unpaired = %d，要 1", got)
	}
	for _, st := range o.Layer.Stamps {
		if st.Key == s.ID {
			t.Fatal("skip 事件缺 B 時提交仍不得加疊字")
		}
	}
	o.End()
	if n := ovOpaque(o, 168, 168, 200, 176); n == 0 {
		t.Fatal("下一個非 skip 事件要照常提交")
	}
}

func TestOverlayNoShadowLangDoesNotMaintainLayer(t *testing.T) {
	o := NewOverlay() // 沒有任何語言
	ovFire(o, ovRec(2, 3, "Hello"))
	ovCheckDump(t, o)
	ovCheckCounters(t, o, map[string]uint64{"events": 1})

	o = NewOverlay() // 只有停用的語言
	o.AddLanguage(&Language{Name: "ja", Enabled: false})
	ovFire(o, ovRec(2, 3, "Hello"))
	ovCheckDump(t, o)
	ovCheckCounters(t, o, map[string]uint64{"events": 1})
}

// ---------- P 類：符號修補 ----------

func TestOverlayPatchIdentityPercentS(t *testing.T) {
	o := ovNewTW(t)
	ovFire(o, ovOptionRec())
	ovCheckDump(t, o, ovOptionDump("g1", 0, "+開音效")...)
	ovCheckStrs(t, "Hits(g1)", o.Hits("g1"), []string{"+Sound"})

	// 單字元 "-" 覆寫第 0 格（符號格）。
	minus := ovRecC(4, 6, '-')
	ovFire(o, minus)
	if minus.ID != "g2" {
		t.Fatalf("修補事件的編號 = %q，要 g2", minus.ID)
	}
	ovCheckDump(t, o, ovOptionDump("g2", 0, "-關音效")...)

	// 新記錄的 Text、Cells、ArgStrs[0].Content 同步改成 "-Sound "，其餘沿用。
	nw := o.Record("g2")
	if nw == nil {
		t.Fatal("沒有新記錄 g2")
	}
	if nw.ID != "g2" || nw.Text != "-Sound " || string(nw.Cells) != "-Sound " ||
		len(nw.ArgStrs) != 1 || nw.ArgStrs[0].Content != "-Sound " ||
		nw.Format != "%s" || nw.Col != 4 || nw.Row != 6 || nw.Caller != 0x107E {
		t.Fatalf("新記錄 = %+v", nw)
	}
	// 舊記錄保持原樣（影子仍以舊 ID 引用它）。
	old := o.Record("g1")
	if old == nil {
		t.Fatal("舊記錄 g1 被移除了")
	}
	if old.ID != "g1" || old.Text != "+Sound " || string(old.Cells) != "+Sound " ||
		len(old.ArgStrs) != 1 || old.ArgStrs[0].Content != "+Sound " {
		t.Fatalf("舊記錄被改動：%+v", old)
	}
	ovCheckStrs(t, "Hits(g2)", o.Hits("g2"), []string{"-Sound"})
	ovCheckStrs(t, "KeysShown", o.KeysShown(), []string{"-Sound"})
	ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 1, "patched": 1})
	// P 類的新記錄也寫進稽核日誌，Text 是修補後的內容。
	wantLog := []LogEvent{{N: 1, ID: "g1", Col: 4, Row: 6, Text: "+Sound "}, {N: 2, ID: "g2", Col: 4, Row: 6, Text: "-Sound "}}
	if !reflect.DeepEqual(o.Log(), wantLog) {
		t.Fatalf("稽核日誌 = %+v，要 %+v", o.Log(), wantLog)
	}

	// 切回 "+"：命中 "+Sound" 的譯文；上一筆記錄 g2 與更早的 g1 都不變。
	ovFire(o, ovRecC(4, 6, '+'))
	ovCheckDump(t, o, ovOptionDump("g3", 0, "+開音效")...)
	if r := o.Record("g3"); r == nil || r.Text != "+Sound " || r.ArgStrs[0].Content != "+Sound " || string(r.Cells) != "+Sound " {
		t.Fatalf("g3 = %+v", r)
	}
	if r := o.Record("g2"); r == nil || r.Text != "-Sound " || r.ArgStrs[0].Content != "-Sound " {
		t.Fatalf("g2 被改動：%+v", r)
	}
	if r := o.Record("g1"); r == nil || r.Text != "+Sound " {
		t.Fatalf("g1 被改動：%+v", r)
	}
	ovCheckStrs(t, "Hits(g3)", o.Hits("g3"), []string{"+Sound"})
	ovCheckCounters(t, o, map[string]uint64{"events": 3, "translated": 1, "patched": 2})
	if n := len(o.Log()); n != 3 || o.Log()[2].Text != "+Sound " {
		t.Fatalf("稽核日誌 = %+v", o.Log())
	}
}

func TestOverlayPatchSymbolInsideLabel(t *testing.T) {
	// 符號不在第 0 格：Text "Mode + " 的第 5 格被 "-" 覆寫，格偏移對內容偏移。
	o := ovNewTW(t)
	ovFire(o, ovRecS(4, 6, "Mode + ", KindStatic))
	ovCheckDump(t, o, // avail 14 h，"模式開" 6 h，補 8 個空白
		ovS{"g1", 32, 48, 3, 8, "模式開", "FFF"},
		ovS{"g1", 56, 48, 8, 4, ovSp(8), "FFFFFFFF"})
	ovFire(o, ovRecC(9, 6, '-')) // x0=72，落在半形補白段
	ovCheckDump(t, o,
		ovS{"g2", 32, 48, 3, 8, "模式關", "FFF"},
		ovS{"g2", 56, 48, 8, 4, ovSp(8), "FFFFFFFF"})
	nw := o.Record("g2")
	if nw == nil || nw.Text != "Mode - " || string(nw.Cells) != "Mode - " || nw.ArgStrs[0].Content != "Mode - " {
		t.Fatalf("g2 = %+v", nw)
	}
	ovCheckStrs(t, "Hits(g2)", o.Hits("g2"), []string{"Mode -"})
	ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 1, "patched": 1})
}

func TestOverlayPatchChainsOnLatestRecord(t *testing.T) {
	// 取被覆蓋疊字所屬事件組的最新記錄：舊記錄 g1 與新記錄 g2 的矩形都含第 6 格，第二次修補要建在 g2 上。
	o := ovNewTW(t)
	ovFire(o, ovRecS(4, 6, "Mode ++", KindStatic)) // 第 5、6 格各有一個符號；"雙開" 4 h，補 10 個空白
	ovCheckDump(t, o,
		ovS{"g1", 32, 48, 2, 8, "雙開", "FF"},
		ovS{"g1", 48, 48, 10, 4, ovSp(10), "FFFFFFFFFF"})
	ovFire(o, ovRecC(9, 6, '-')) // k=5
	ovCheckDump(t, o,
		ovS{"g2", 32, 48, 2, 8, "半關", "FF"},
		ovS{"g2", 48, 48, 10, 4, ovSp(10), "FFFFFFFFFF"})
	ovFire(o, ovRecC(10, 6, '-')) // k=6：建在 g2 上得 "Mode --"（建在 g1 上會是 "Mode +-"，譯文不同）
	ovCheckDump(t, o,
		ovS{"g3", 32, 48, 2, 8, "雙關", "FF"},
		ovS{"g3", 48, 48, 10, 4, ovSp(10), "FFFFFFFFFF"})
	if r := o.Record("g3"); r == nil || r.Text != "Mode --" || r.ArgStrs[0].Content != "Mode --" || string(r.Cells) != "Mode --" {
		t.Fatalf("g3 = %+v", r)
	}
	ovCheckStrs(t, "Hits(g3)", o.Hits("g3"), []string{"Mode --"})
	ovCheckCounters(t, o, map[string]uint64{"events": 3, "translated": 1, "patched": 2})
}

func TestOverlayPatchLiteralEvent(t *testing.T) {
	o := ovNewTW(t)
	ovFire(o, ovRec(4, 6, "+Sound ")) // 字面事件：Format 等於 Text，沒有轉換
	ovCheckDump(t, o, ovOptionDump("g1", 0, "+開音效")...)
	ovFire(o, ovRecC(4, 6, '-'))
	ovCheckDump(t, o, ovOptionDump("g2", 0, "-關音效")...)
	nw := o.Record("g2")
	if nw == nil || nw.Text != "-Sound " || string(nw.Cells) != "-Sound " || nw.Format != "-Sound " || len(nw.ArgStrs) != 0 {
		t.Fatalf("g2 = %+v", nw)
	}
	if old := o.Record("g1"); old == nil || old.Text != "+Sound " || old.Format != "+Sound " || string(old.Cells) != "+Sound " {
		t.Fatalf("舊記錄被改動：%+v", old)
	}
	ovCheckStrs(t, "Hits(g2)", o.Hits("g2"), []string{"-Sound"})
	ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 1, "patched": 1})
}

func TestOverlayPatchKeepsDYAndStackOrder(t *testing.T) {
	o := ovNewTW(t)
	ovFire(o, ovOptionRec())        // g1，Row 6
	ovFire(o, ovRec(2, 3, "Hello")) // g2，疊在 g1 之後
	// 原版把第 5 至 6 列的內容上捲一列：g1 的疊字 Y 由 48 變 40，記錄的 Row 仍是 6，DY = -8。
	o.Layer.Scroll(0, 40, 320, 56, -8)
	ovFire(o, ovRecC(4, 5, '-')) // 事件座標是畫面上的列 5
	ovCheckDump(t, o, append(ovOptionDump("g3", -8, "-關音效"),
		ovS{"g2", 16, 24, 2, 8, "你好", "FF"},
		ovS{"g2", 32, 24, 6, 4, ovSp(6), "FFFFFF"})...)
	// 疊序：新組在舊組原本的位置，仍在 g2 之前。
	if keys := ovKeys(o); !reflect.DeepEqual(keys, []string{"g3", "g2"}) {
		t.Fatalf("事件組次序 = %v，要 [g3 g2]", keys)
	}
	ovCheckCounters(t, o, map[string]uint64{"events": 3, "translated": 2, "patched": 1})
}

func TestOverlayPatchKeepsHiddenCells(t *testing.T) {
	o := ovNewTW(t)
	ovFire(o, ovOptionRec())
	o.Layer.Clear(60, 48, 88, 56) // 補白段整筆被原版清掉，已不可見
	ovCheckDump(t, o, ovOptionDump("g1", 0, "+開音效")[:2]...)
	ovFire(o, ovRecC(4, 6, '-'))
	// 保留可見範圍：被清掉的範圍不復活。
	ovCheckDump(t, o, ovOptionDump("g2", 0, "-關音效")[:2]...)
}

func TestOverlayPatchInconsistentDYIsLost(t *testing.T) {
	o := ovNewTW(t)
	ovFire(o, ovOptionRec())
	o.Layer.Stamps[2].Y++ // 同組各疊字的 Y 不一致
	ovFire(o, ovRecC(4, 6, '-'))
	ovCheckDump(t, o) // 整組移除
	// 移除後事件走一般流程：%c 的 "-" 沒有字母，passthrough。
	ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 1, "rebuild_lost": 1, "passthrough": 1})
	if o.Record("g2") != nil {
		t.Fatal("沒有修補就不留新記錄")
	}
}

func TestOverlayPatchFallbackForOtherForms(t *testing.T) {
	t.Run("模板含字面文字", func(t *testing.T) {
		o := ovNewTW(t)
		r := ovRec(4, 6, "Sound +")
		r.Format = "Sound %s"
		r.ArgStrs = []ArgStr{{Ptr: 0x4000, Content: "+", Kind: KindOther}}
		ovFire(o, r)
		ovCheckDump(t, o, // "音效+" 5 h，補 9 個空白；"+" 與補白同為半形，同一段
			ovS{"g1", 32, 48, 2, 8, "音效", "FF"},
			ovS{"g1", 48, 48, 10, 4, "+" + ovSp(9), "FFFFFFFFFF"})
		ovFire(o, ovRecC(10, 6, '-')) // 第 6 格是 "+"，x0=80 落在半形段的第 8 格
		ovCheckDump(t, o,
			ovS{"g1", 32, 48, 2, 8, "音效", "FF"},
			ovS{"g1", 48, 48, 10, 4, "+" + ovSp(9), "FFFFFFFFTT"})
		ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 1, "patch_fallback": 1})
		if o.Record("g1").Text != "Sound +" || o.Record("g2") != nil {
			t.Fatal("退回 Clear 時不得修補或留新記錄")
		}
	})
	t.Run("多個轉換", func(t *testing.T) {
		o := ovNewTW(t)
		r := ovRec(4, 6, "+Sound")
		r.Format = "%s%s"
		r.ArgStrs = []ArgStr{{Ptr: 0x4000, Content: "+", Kind: KindOther}, {Ptr: 0x4002, Content: "Sound", Kind: KindStatic}}
		ovFire(o, r)
		ovCheckDump(t, o, // "+音效" 5 h，補 7 個空白
			ovS{"g1", 32, 48, 1, 4, "+", "F"},
			ovS{"g1", 36, 48, 2, 8, "音效", "FF"},
			ovS{"g1", 52, 48, 7, 4, ovSp(7), "FFFFFFF"})
		ovFire(o, ovRecC(4, 6, '-'))
		ovCheckDump(t, o, // Clear [32,40)：半形 "+" 整筆被蓋，全形段第 0 格轉透明
			ovS{"g1", 36, 48, 2, 8, "音效", "TF"},
			ovS{"g1", 52, 48, 7, 4, ovSp(7), "FFFFFFF"})
		ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 1, "patch_fallback": 1})
	})
	t.Run("組句", func(t *testing.T) {
		o := ovNewTW(t)
		r := ovRecS(4, 6, "+Sound ", KindBuffer)
		r.Composed = &Piece{Fmt: "+Sound ", FmtKind: KindStatic, Literal: "+Sound "}
		ovFire(o, r)
		ovCheckDump(t, o, ovOptionDump("g1", 0, "+開音效")...)
		ovFire(o, ovRecC(4, 6, '-'))
		got := ovDump(o)
		want := ovOptionDump("g1", 0, "+開音效")
		want = []ovS{want[1], want[2]}
		want[0].Tr = "TFF" // 符號格被 Clear，全形段第 0 格轉透明
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Layer 疊字不符\n got:%s\nwant:%s", ovFmtDump(got), ovFmtDump(want))
		}
		ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 1, "patch_fallback": 1})
	})
}

func TestOverlayPatchResolveFailureClears(t *testing.T) {
	// catalog 只有 "+Sound"，沒有 "-Sound"：修補後 Resolve 失敗，退回 Clear 並計 untranslated。
	o := ovNew(t, ovLang("zh-TW", map[string]string{"+Sound": "+開音效"}))
	ovFire(o, ovOptionRec())
	ovFire(o, ovRecC(4, 6, '-'))
	if n := ovOpaque(o, 32, 48, 40, 56); n != 0 {
		t.Fatalf("符號格要被清除，仍有 %d 個未透明格", n)
	}
	got := ovDump(o)
	want := ovOptionDump("g1", 0, "+開音效")
	want = []ovS{want[1], want[2]}
	want[0].Tr = "TFF"
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Layer 疊字不符\n got:%s\nwant:%s", ovFmtDump(got), ovFmtDump(want))
	}
	ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 1, "untranslated": 1})
	ovCheckStrs(t, "KeySet(untranslated)", o.C.KeySet("untranslated"), nil) // 恆等模板沒有鍵
	ovCheckStrs(t, "KeySet(untranslated_at)", o.C.KeySet("untranslated_at"), []string{"107E|%s"})
	if o.Record("g2") != nil {
		t.Fatal("Resolve 失敗不留新記錄")
	}
}

func TestOverlayPatchNotApplicablePassesThrough(t *testing.T) {
	t.Run("沒有疊字覆蓋該格", func(t *testing.T) {
		o := ovNewTW(t)
		ovFire(o, ovRecC(4, 6, '+')) // 單字元 %c：沒有字母，走一般流程的 passthrough
		ovCheckDump(t, o)
		ovCheckCounters(t, o, map[string]uint64{"events": 1, "passthrough": 1})
	})
	t.Run("原事件在該格不是正負號", func(t *testing.T) {
		o := ovNewTW(t)
		ovFire(o, ovRec(2, 3, "Hello"))
		ovFire(o, ovRecC(2, 3, '+')) // 該格被 "你好" 覆蓋，但原文第 0 格是 H
		ovCheckDump(t, o,
			ovS{"g1", 16, 24, 2, 8, "你好", "TF"},
			ovS{"g1", 32, 24, 6, 4, ovSp(6), "FFFFFF"})
		ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 1, "passthrough": 1})
	})
	t.Run("符號位置上原文是空白", func(t *testing.T) {
		o := ovNewTW(t)
		ovFire(o, ovRecS(4, 6, "Mode + ", KindStatic))
		ovFire(o, ovRecC(8, 6, '-')) // 第 4 格，原文是空白
		ovCheckDump(t, o,
			ovS{"g1", 32, 48, 3, 8, "模式開", "FFF"},
			ovS{"g1", 56, 48, 8, 4, ovSp(8), "FFTTFFFF"})
		ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 1, "passthrough": 1})
		if o.Record("g2") != nil {
			t.Fatal("不修補就不留新記錄")
		}
	})
	t.Run("符號格的疊字已被清掉", func(t *testing.T) {
		o := ovNewTW(t)
		ovFire(o, ovOptionRec())
		o.Layer.Clear(32, 48, 40, 56) // 符號格被清掉：半形 "+" 整筆移除，全形段第 0 格透明
		ovFire(o, ovRecC(4, 6, '+'))
		ovCheckDump(t, o,
			ovS{"g1", 36, 48, 3, 8, "開音效", "TFF"},
			ovS{"g1", 60, 48, 7, 4, ovSp(7), "FFFFFFF"})
		ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 1, "passthrough": 1})
	})
	t.Run("覆蓋該格的疊字格已透明", func(t *testing.T) {
		o := ovNewTW(t)
		ovFire(o, ovRecS(4, 6, "Mode ++", KindStatic)) // "雙開" [32,48)，補白 10 格 [48,88)
		o.Layer.Clear(72, 48, 80, 56)                  // 第 5 格（x 72 至 80）的疊字格轉透明
		ovCheckDump(t, o,
			ovS{"g1", 32, 48, 2, 8, "雙開", "FF"},
			ovS{"g1", 48, 48, 10, 4, ovSp(10), "FFFFFFTTFF"})
		ovFire(o, ovRecC(9, 6, '-')) // 該格已透明，不算被覆蓋：不修補
		ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 1, "passthrough": 1})
		if o.Record("g2") != nil {
			t.Fatal("不修補就不留新記錄")
		}
		// 相鄰的第 6 格（x 80 至 88）不透明：照常修補，且保留已不可見的 [72,80)。
		ovFire(o, ovRecC(10, 6, '-'))
		ovCheckDump(t, o,
			ovS{"g3", 32, 48, 2, 8, "右關", "FF"},
			ovS{"g3", 48, 48, 10, 4, ovSp(10), "FFFFFFTTFF"})
		ovCheckCounters(t, o, map[string]uint64{"events": 3, "translated": 1, "passthrough": 1, "patched": 1})
	})
	t.Run("多字元事件不是 P 類", func(t *testing.T) {
		o := ovNewTW(t)
		ovFire(o, ovOptionRec())
		ovFire(o, ovRecS(4, 6, "-Sound ", KindStatic)) // 整個標籤重印：T 類，由 Layer.Add 取代舊疊字
		ovCheckDump(t, o, ovOptionDump("g2", 0, "-關音效")...)
		ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 2})
	})
	t.Run("其他單字元事件不修補", func(t *testing.T) {
		o := ovNewTW(t)
		ovFire(o, ovOptionRec())
		ovFire(o, ovRecC(4, 6, 'x')) // 名字回顯等：走一般流程（%c 沒有字母，passthrough）
		ovCheckDump(t, o,
			ovS{"g1", 36, 48, 3, 8, "開音效", "TFF"},
			ovS{"g1", 60, 48, 7, 4, ovSp(7), "FFFFFFF"})
		ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 1, "passthrough": 1})
	})
}

// ---------- 語言切換（004 §5、§7 第 1 項） ----------

func TestOverlaySwitchKeepsTransparentCellsAndStackOrder(t *testing.T) {
	o := ovNew(t, ovLang("zh-TW", ovTWUI()), ovLang("ja", ovJAUI()))
	ovFire(o, ovRec(0, 0, "ABCDEF")) // 較舊：x [0,48)
	ovFire(o, ovRec(4, 0, "WXYZ"))   // 較新：x [32,64)，蓋住較舊組的第 4、5 格
	ovCheckDump(t, o,
		ovS{"g1", 0, 0, 6, 8, "甲乙丙丁戊己", "FFFFTT"},
		ovS{"g2", 32, 0, 4, 8, "子丑寅卯", "FFFF"})

	if err := o.SetDisplay("ja"); err != nil {
		t.Fatal(err)
	}
	if o.Display() != "ja" || o.ShadowLang() != "ja" {
		t.Fatalf("Display=%q ShadowLang=%q", o.Display(), o.ShadowLang())
	}
	ovCheckDump(t, o, // 順序不變；較舊組被蓋到的格仍為透明
		ovS{"g1", 0, 0, 6, 8, "あいうえおか", "FFFFTT"},
		ovS{"g2", 32, 0, 4, 8, "さしすせ", "FFFF"})
	ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 2})
	ovCheckStrs(t, "KeysShown", o.KeysShown(), []string{"ABCDEF", "WXYZ"})
	for _, s := range o.Layer.Stamps {
		if s.State != xlate.Pending || s.Font != o.Language("ja").Font {
			t.Fatalf("重建的疊字要是 Pending 且用 ja 的字型：%+v", s)
		}
	}

	// 切換之後的新事件一律以新語言解析。
	ovFire(o, ovRec(10, 10, "AB"))
	if got := ovDump(o); got[len(got)-1].Text != "はひ" {
		t.Fatalf("新事件的譯文 = %q，要 はひ", got[len(got)-1].Text)
	}
}

func TestOverlaySwitchSameOriginKeepsOtherEvent(t *testing.T) {
	o := ovNew(t, ovLang("zh-TW", ovTWUI()), ovLang("ja", ovJAUI()))
	ovFire(o, ovRec(2, 3, "ABCDEFGH")) // 較長：x [16,80)
	ovFire(o, ovRec(2, 3, "XY"))       // 同原點較短：x [16,32)，蓋住較長者的第 0、1 格
	ovCheckDump(t, o,
		ovS{"g1", 16, 24, 4, 8, "甲乙丙丁", "TTFF"},
		ovS{"g1", 48, 24, 8, 4, ovSp(8), "FFFFFFFF"},
		ovS{"g2", 16, 24, 1, 8, "乙", "F"},
		ovS{"g2", 24, 24, 2, 4, "  ", "FF"})
	wantVis := [][2]int{{32, 80}}
	if got := ovVisible(o, "g1"); !reflect.DeepEqual(got, wantVis) {
		t.Fatalf("切換前 g1 的可見範圍 = %v，要 %v", got, wantVis)
	}

	if err := o.SetDisplay("ja"); err != nil {
		t.Fatal(err)
	}
	// g1 在 ja 的切段不同："ab" 在不可見範圍 [16,32) 內整筆不加入，全形段第 0 格透明。
	ovCheckDump(t, o,
		ovS{"g1", 24, 24, 3, 8, "あいう", "TFF"},
		ovS{"g1", 48, 24, 8, 4, ovSp(8), "FFFFFFFF"},
		ovS{"g2", 16, 24, 1, 8, "ひ", "F"},
		ovS{"g2", 24, 24, 2, 4, "  ", "FF"})
	if got := ovVisible(o, "g1"); !reflect.DeepEqual(got, wantVis) {
		t.Fatalf("切換後 g1 的可見範圍 = %v，要 %v", got, wantVis)
	}
	ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 2})
}

func TestOverlaySwitchKeepsDYAfterScroll(t *testing.T) {
	// 001 §10 的捲動序列：疊字經 Layer.Scroll 上移一列後切換語言，新疊字沿用 DY（Y = Row×8 + DY）。
	o := ovNew(t, ovLang("zh-TW", ovTWUI()), ovLang("ja", ovJAUI()))
	ovFire(o, ovRec(2, 3, "Hello")) // Y=24
	o.Layer.Scroll(0, 16, 320, 40, -8)
	ovCheckDump(t, o,
		ovS{"g1", 16, 16, 2, 8, "你好", "FF"},
		ovS{"g1", 32, 16, 6, 4, ovSp(6), "FFFFFF"})
	if err := o.SetDisplay("ja"); err != nil {
		t.Fatal(err)
	}
	ovCheckDump(t, o, ovS{"g1", 16, 16, 5, 8, "こんにちは", "FFFFF"})
	if r := o.Record("g1"); r == nil || r.Row != 3 {
		t.Fatalf("記錄的 Row 不隨捲動改變：%+v", r)
	}
}

func TestOverlaySwitchHiddenIsNotRevivedByTransparentFlags(t *testing.T) {
	// 004 §7 第 4 項：同一事件兩段，Clear 把第二段整筆移除後切換語言，第二段的範圍仍不可見。
	o := ovNew(t, ovLang("zh-TW", ovTWUI()), ovLang("ja", ovJAUI()))
	ovFire(o, ovRec(0, 0, "ABCD")) // "甲" 全形 1 格 [0,8)，補白半形 6 格 [8,32)
	ovCheckDump(t, o,
		ovS{"g1", 0, 0, 1, 8, "甲", "F"},
		ovS{"g1", 8, 0, 6, 4, ovSp(6), "FFFFFF"})
	o.Layer.Clear(8, 0, 32, 8) // 補白段整筆被蓋住而移除，不留透明旗標
	ovCheckDump(t, o, ovS{"g1", 0, 0, 1, 8, "甲", "F"})

	if err := o.SetDisplay("ja"); err != nil { // "さしす" 6 h 加補白 2 h
		t.Fatal(err)
	}
	if n := ovOpaque(o, 8, 0, 32, 8); n != 0 {
		t.Fatalf("已被清除的範圍 [8,32) 復活了 %d 個未透明格：%s", n, ovFmtDump(ovDump(o)))
	}
	ovCheckDump(t, o, ovS{"g1", 0, 0, 3, 8, "さしす", "FTT"}) // 補白段全部落在不可見範圍內，不加入
}

func TestOverlaySwitchWaitsForOpenEvent(t *testing.T) {
	var calls []string
	zh := ovLang("zh-TW", ovTWUI())
	ja := &Language{Name: "ja", Cat: ovSpy{NewCatalog(ovJAUI(), nil, nil), &calls}, Font: &xlate.Font{Name: "ovfont-ja"}, Wide: ovWide, Enabled: true}
	o := ovNew(t, zh, ja)
	ovFire(o, ovRec(2, 3, "Hello"))

	bye := ovRec(10, 10, "Bye")
	o.Begin(bye, false) // A 已觸發、B 未觸發
	if err := o.SetDisplay("ja"); err != nil {
		t.Fatal(err)
	}
	if o.Display() != "ja" || o.ShadowLang() != "ja" {
		t.Fatalf("Display=%q ShadowLang=%q（顯示語言與影子語言立即更新）", o.Display(), o.ShadowLang())
	}
	// 事件中途不執行重建：Layer 還是 zh-TW，也沒有查過 ja 的 catalog。
	ovCheckDump(t, o,
		ovS{"g1", 16, 24, 2, 8, "你好", "FF"},
		ovS{"g1", 32, 24, 6, 4, ovSp(6), "FFFFFF"})
	if len(calls) != 0 {
		t.Fatalf("事件中途不得重建，卻查了 %v", calls)
	}
	o.End()
	// 先提交開啟中的事件（查 Bye），再重建 Layer 內的每個事件組（依疊序：g1、g2）。
	if want := []string{"Bye", "Hello", "Bye"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("catalog 查詢順序 = %v，要 %v（B 提交完成之後才重建）", calls, want)
	}
	ovCheckDump(t, o,
		ovS{"g1", 16, 24, 5, 8, "こんにちは", "FFFFF"},
		ovS{"g2", 80, 80, 2, 8, "では", "FF"},
		ovS{"g2", 96, 80, 2, 4, "  ", "FF"})
	if o.pendingSwitch {
		t.Fatal("重建完成後 pendingSwitch 要清掉")
	}
	ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 2})

	// 沒有開啟中的事件時，切換立即重建。
	if err := o.SetDisplay("zh-TW"); err != nil {
		t.Fatal(err)
	}
	ovCheckDump(t, o,
		ovS{"g1", 16, 24, 2, 8, "你好", "FF"},
		ovS{"g1", 32, 24, 6, 4, ovSp(6), "FFFFFF"},
		ovS{"g2", 80, 80, 2, 8, "再見", "FF"},
		ovS{"g2", 96, 80, 2, 4, "  ", "FF"})
}

func TestOverlaySwitchRecordsMissingIsLost(t *testing.T) {
	o := ovNew(t, ovLang("zh-TW", ovTWUI()), ovLang("ja", ovJAUI()))
	ovFire(o, ovRec(2, 3, "Hello"))
	ovFire(o, ovRec(10, 10, "Bye"))
	delete(o.records, "g1") // 記錄已不存在
	if err := o.SetDisplay("ja"); err != nil {
		t.Fatal(err)
	}
	ovCheckDump(t, o,
		ovS{"g2", 80, 80, 2, 8, "では", "FF"},
		ovS{"g2", 96, 80, 2, 4, "  ", "FF"})
	// rebuild_lost 獨立計數，不併入 untranslated 或 switch_untranslated。
	ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 2, "rebuild_lost": 1})
}

func TestOverlaySwitchInconsistentDYIsLost(t *testing.T) {
	o := ovNew(t, ovLang("zh-TW", ovTWUI()), ovLang("ja", ovJAUI()))
	ovFire(o, ovRec(2, 3, "Hello"))
	ovFire(o, ovRec(10, 10, "Bye"))
	o.Layer.Stamps[1].Y += 4 // g1 的兩筆疊字 Y 不一致
	if err := o.SetDisplay("ja"); err != nil {
		t.Fatal(err)
	}
	if keys := ovKeys(o); !reflect.DeepEqual(keys, []string{"g2"}) {
		t.Fatalf("事件組 = %v，要只剩 [g2]", keys)
	}
	ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 2, "rebuild_lost": 1})
}

func TestOverlaySwitchUntranslatedRemovesGroup(t *testing.T) {
	jaUI := ovJAUI()
	delete(jaUI, "Hello") // 新語言缺該鍵
	ko := map[string]string{"Hello": "안녕", "Bye": "a\nb"}
	o := ovNew(t, ovLang("zh-TW", ovTWUI()), ovLang("ja", jaUI), ovLang("ko", ko))
	ovFire(o, ovRec(2, 3, "Hello"))
	ovFire(o, ovRec(10, 10, "Bye"))

	if err := o.SetDisplay("ja"); err != nil {
		t.Fatal(err)
	}
	ovCheckDump(t, o, // 缺譯的組移除，原文顯示
		ovS{"g2", 80, 80, 2, 8, "では", "FF"},
		ovS{"g2", 96, 80, 2, 4, "  ", "FF"})
	ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 2, "switch_untranslated": 1})
	ovCheckStrs(t, "KeysShown", o.KeysShown(), []string{"Bye"})

	// 譯文含換行（版面錯誤）同樣移除並計 switch_untranslated。
	if err := o.SetDisplay("ko"); err != nil {
		t.Fatal(err)
	}
	ovCheckDump(t, o)
	ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 2, "switch_untranslated": 2})
	ovCheckStrs(t, "KeysShown", o.KeysShown(), nil)
}

func TestOverlayShadowLangStart(t *testing.T) {
	// 先 AddLanguage zh-TW，再切 en：shadowLang 維持 zh-TW，Layer 照常維護。
	o := NewOverlay()
	o.AddLanguage(ovLang("zh-TW", ovTWUI()))
	o.AddLanguage(ovLang("ja", ovJAUI()))
	if o.ShadowLang() != "zh-TW" || o.Display() != "" || o.Drawing() {
		t.Fatalf("啟動值：shadowLang=%q display=%q drawing=%v", o.ShadowLang(), o.Display(), o.Drawing())
	}
	if err := o.SetDisplay("en"); err != nil {
		t.Fatal(err)
	}
	if o.ShadowLang() != "zh-TW" || o.Display() != "en" || o.Drawing() {
		t.Fatalf("切 en：shadowLang=%q display=%q drawing=%v", o.ShadowLang(), o.Display(), o.Drawing())
	}
	ovFire(o, ovRec(2, 3, "Hello")) // 顯示英文，但 Layer 仍以 zh-TW 維護
	ovCheckDump(t, o,
		ovS{"g1", 16, 24, 2, 8, "你好", "FF"},
		ovS{"g1", 32, 24, 6, 4, ovSp(6), "FFFFFF"})
	if err := o.SetDisplay("zh-TW"); err != nil {
		t.Fatal(err)
	}
	if !o.Drawing() || o.ShadowLang() != "zh-TW" {
		t.Fatal("切到 zh-TW 後要畫疊字")
	}
	if err := o.SetDisplay("ja"); err != nil {
		t.Fatal(err)
	}
	if o.ShadowLang() != "ja" {
		t.Fatalf("顯示語言切到 ja，shadowLang = %q", o.ShadowLang())
	}
	if err := o.SetDisplay("en"); err != nil {
		t.Fatal(err)
	}
	if o.ShadowLang() != "ja" {
		t.Fatalf("進入 en 時維持原值，shadowLang = %q", o.ShadowLang())
	}
	if got := o.Languages(); !reflect.DeepEqual(got, []string{"zh-TW", "ja"}) {
		t.Fatalf("Languages() = %v", got)
	}

	// 第一個已啟用的非 en 語言：en 與停用的語言都不算。
	o = NewOverlay()
	o.AddLanguage(ovLang("en", ovTWUI())) // 完整啟用的 en 也不算
	if o.ShadowLang() != "" {
		t.Fatalf("只有 en 時 shadowLang = %q，要空", o.ShadowLang())
	}
	o.AddLanguage(&Language{Name: "ko", Enabled: false})
	if o.ShadowLang() != "" {
		t.Fatalf("只有停用語言時 shadowLang = %q，要空", o.ShadowLang())
	}
	o.AddLanguage(ovLang("zh-TW", ovTWUI()))
	if o.ShadowLang() != "zh-TW" {
		t.Fatalf("shadowLang = %q，要 zh-TW", o.ShadowLang())
	}
	o.AddLanguage(ovLang("ja", ovJAUI()))
	if o.ShadowLang() != "zh-TW" {
		t.Fatalf("已有 shadowLang 時不被後加的語言取代：%q", o.ShadowLang())
	}
}

func TestOverlayDisabledLanguageCannotSwitch(t *testing.T) {
	o := ovNewTW(t)
	cases := []struct {
		name string
		lang *Language
	}{
		{"字型沒有名稱", &Language{Name: "x1", Cat: NewCatalog(ovJAUI(), nil, nil), Font: &xlate.Font{Name: ""}, Wide: ovWide, Enabled: true}},
		{"沒有字型", &Language{Name: "x2", Cat: NewCatalog(ovJAUI(), nil, nil), Wide: ovWide, Enabled: true}},
		{"沒有 catalog", &Language{Name: "x3", Font: &xlate.Font{Name: "ovfont-x3"}, Wide: ovWide, Enabled: true}},
		{"沒有寬度表", &Language{Name: "x4", Cat: NewCatalog(ovJAUI(), nil, nil), Font: &xlate.Font{Name: "ovfont-x4"}, Enabled: true}},
		{"組態停用", &Language{Name: "x5", Cat: NewCatalog(ovJAUI(), nil, nil), Font: &xlate.Font{Name: "ovfont-x5"}, Wide: ovWide, Enabled: false}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o.AddLanguage(c.lang)
			if c.lang.Enabled {
				t.Fatal("語言要被停用")
			}
			if err := o.SetDisplay(c.lang.Name); err == nil {
				t.Fatal("停用的語言不可切換")
			}
			if o.Display() != "zh-TW" || o.ShadowLang() != "zh-TW" {
				t.Fatalf("切換失敗後狀態要不變：Display=%q ShadowLang=%q", o.Display(), o.ShadowLang())
			}
		})
	}
	if _, ok := o.Layer.FontRegistry[""]; ok {
		t.Fatal("字型名稱為空的語言不得登記到 FontRegistry")
	}
	if f := o.Layer.FontRegistry["ovfont-zh-TW"]; f == nil || f != o.Language("zh-TW").Font {
		t.Fatal("啟用的語言要把字型登記到 FontRegistry")
	}
	if err := o.SetDisplay("xx"); err == nil {
		t.Fatal("沒註冊的語言不可切換")
	}
	if err := o.SetDisplay("en"); err != nil {
		t.Fatalf("en 永遠可切換：%v", err)
	}
	if got := o.Languages(); !reflect.DeepEqual(got, []string{"zh-TW", "x1", "x2", "x3", "x4", "x5"}) {
		t.Fatalf("Languages() = %v", got)
	}
}

// ---------- records 的保留與淘汰（004 §5） ----------

func TestOverlayRecordsGC(t *testing.T) {
	o := ovNewTW(t)
	ovFire(o, ovRec(2, 3, "Hello"))   // g1：之後被 g3 完整取代
	ovFire(o, ovRec(10, 10, "Bye"))   // g2：留在 Layer
	ovFire(o, ovRec(2, 3, "Hello"))   // g3：留在 Layer
	ovFire(o, ovRec(20, 20, "Hello")) // g4：Clear 後只被影子引用
	ovFire(o, ovRec(30, 5, "Bye"))    // g5：Clear 後沒人引用
	o.Layer.Clear(160, 160, 320, 168)
	o.Layer.Clear(240, 40, 320, 48)
	o.sh.setKnown(7, Shadow{{ID: "g4"}})
	if keys := ovKeys(o); !reflect.DeepEqual(keys, []string{"g2", "g3"}) {
		t.Fatalf("Layer 事件組 = %v，要 [g2 g3]", keys)
	}
	for _, id := range []string{"g1", "g2", "g3", "g4", "g5"} {
		if o.Record(id) == nil {
			t.Fatalf("gcRecords 之前 %s 的記錄還在", id)
		}
	}
	o.gcRecords()
	for id, keep := range map[string]bool{"g1": false, "g2": true, "g3": true, "g4": true, "g5": false} {
		if (o.Record(id) != nil) != keep {
			t.Fatalf("gcRecords 之後 Record(%s) 存在 = %v，要 %v", id, o.Record(id) != nil, keep)
		}
		if (o.Hits(id) != nil) != keep {
			t.Fatalf("gcRecords 之後 Hits(%s) 存在 = %v，要 %v", id, o.Hits(id) != nil, keep)
		}
	}
}

func TestOverlayRecordsGCEvery256Events(t *testing.T) {
	o := ovNewTW(t)
	for i := 0; i < 255; i++ {
		ovFire(o, ovRec(2, 3, "Hello")) // 每個事件都完整取代前一個的疊字
	}
	if n := len(o.records); n != 255 {
		t.Fatalf("255 個事件後 records = %d 筆，要 255（尚未掃描）", n)
	}
	ovFire(o, ovRec(2, 3, "Hello"))
	if n := len(o.records); n != 1 {
		t.Fatalf("第 256 個事件後 records = %d 筆，要 1（掃描後只剩被 Layer 引用的）", n)
	}
	if o.Record("g256") == nil || o.Record("g255") != nil {
		t.Fatal("要留 g256、移除 g255")
	}
}

func TestOverlayRecordsMaxEvictsOldestFirst(t *testing.T) {
	o := ovNewTW(t)
	for i := 1; i <= MaxRecords+2; i++ {
		id := "g" + strconv.Itoa(i)
		o.records[id] = &EventRecord{ID: id}
		o.hits[id] = []string{"k"}
		o.recSeq = append(o.recSeq, id)
		if i != 5 { // g5 沒人引用
			o.Layer.Stamps = append(o.Layer.Stamps, &xlate.Stamp{Key: id})
		}
	}
	o.gcRecords()
	// 先移除沒被引用的 g5，剩 4097 筆仍超過上限，再移除最舊的 g1。
	if n := len(o.records); n != MaxRecords {
		t.Fatalf("records = %d 筆，要 %d", n, MaxRecords)
	}
	for id, keep := range map[string]bool{"g1": false, "g2": true, "g4": true, "g5": false, "g6": true, "g4098": true} {
		if (o.Record(id) != nil) != keep || (o.Hits(id) != nil) != keep {
			t.Fatalf("Record(%s)、Hits(%s) 存在 = %v、%v，要 %v", id, id, o.Record(id) != nil, o.Hits(id) != nil, keep)
		}
	}
}

func TestOverlayRecordsEvictedAreLostOnSwitch(t *testing.T) {
	// 超過上限時最舊的記錄先被移除，即使仍被 Layer 引用；之後切換語言計 rebuild_lost。
	o := ovNew(t, ovLang("zh-TW", ovTWUI()), ovLang("ja", ovJAUI()))
	ovFire(o, ovRec(2, 3, "Hello"))
	ovFire(o, ovRec(10, 10, "Bye"))
	var sh Shadow
	for i := 0; i < MaxRecords; i++ {
		id := "x" + strconv.Itoa(i)
		o.records[id] = &EventRecord{ID: id}
		o.recSeq = append(o.recSeq, id)
		sh = append(sh, ShadowEntry{ID: id})
	}
	o.sh.setKnown(1, sh) // 影子引用這 4096 筆
	o.gcRecords()        // 共 4098 筆，最舊的 g1、g2 被移除
	if o.Record("g1") != nil || o.Record("g2") != nil || o.Record("x0") == nil {
		t.Fatal("要移除最舊的 g1、g2，保留 x0")
	}
	if err := o.SetDisplay("ja"); err != nil {
		t.Fatal(err)
	}
	ovCheckDump(t, o)
	ovCheckCounters(t, o, map[string]uint64{"events": 2, "translated": 2, "rebuild_lost": 2})
}

func TestOverlayLogKeepsLatestMaxLog(t *testing.T) {
	o := ovNewTW(t)
	for i := 0; i < MaxLog+4; i++ {
		ovFire(o, ovRec(2, 3, "Hello"))
	}
	log := o.Log()
	if len(log) != MaxLog {
		t.Fatalf("稽核日誌 %d 筆，要 %d", len(log), MaxLog)
	}
	// 事件編號 g1 至 g4100，保留最新的 4096 筆，最舊在前。
	if log[0].ID != "g5" || log[len(log)-1].ID != "g"+strconv.Itoa(MaxLog+4) {
		t.Fatalf("稽核日誌首尾 = %s、%s，要 g5、g%d", log[0].ID, log[len(log)-1].ID, MaxLog+4)
	}
}

func TestOverlayKeysShown(t *testing.T) {
	o := ovNewTW(t)
	ovCheckStrs(t, "空 Layer 的 KeysShown", o.KeysShown(), nil)
	ovFire(o, ovRec(2, 3, "Hello"))
	ovFire(o, ovRec(10, 10, "Bye"))
	ovFire(o, ovRec(20, 20, "Hello"))
	ovCheckStrs(t, "KeysShown", o.KeysShown(), []string{"Bye", "Hello"}) // 聯集、排序、去重
	o.Layer.Clear(80, 80, 320, 88)
	ovCheckStrs(t, "KeysShown", o.KeysShown(), []string{"Hello"})
}

// ---------- 故障注入 ----------

func TestOverlayFaultNoAdd(t *testing.T) {
	o := ovNewTW(t)
	o.Faults["noadd"] = true
	ovFire(o, ovRec(2, 3, "Hello"))
	ovCheckDump(t, o)
	// 事件照常解析、記錄與寫入稽核日誌（exposed_events 負對照需要它），只是不加進 Layer（005 §5.1）。
	ovCheckCounters(t, o, map[string]uint64{"events": 1, "translated": 1})
	if o.Record("g1") == nil || len(o.Log()) != 1 {
		t.Fatal("noadd 時仍要登記記錄與稽核日誌")
	}
	if len(o.Layer.Stamps) != 0 {
		t.Fatalf("noadd 時 Layer 必須沒有疊字，得 %d 筆", len(o.Layer.Stamps))
	}
}

func TestOverlayFaultNoClear(t *testing.T) {
	cases := []struct {
		name string
		rec  *EventRecord
		want map[string]uint64
	}{
		{"K 類", ovRec(2, 3, ovSp(5)), map[string]uint64{"blank": 1}},
		{"N 類", ovRec(2, 3, "\x05\xC4\x05"), map[string]uint64{"nonprintable": 1}},
		{"缺譯", ovRec(2, 3, "Missing"), map[string]uint64{"untranslated": 1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := ovNewTW(t)
			ovFire(o, ovRec(2, 3, "Hello"))
			before := append([]*xlate.Stamp(nil), o.Layer.Stamps...)
			dump := ovDump(o)
			o.Faults["noclear"] = true
			ovFire(o, c.rec)
			if !ovSame(before, o.Layer.Stamps) || !reflect.DeepEqual(dump, ovDump(o)) {
				t.Fatalf("noclear 時 Layer 不得動：\n%s", ovFmtDump(ovDump(o)))
			}
			want := map[string]uint64{"events": 2, "translated": 1}
			for k, v := range c.want {
				want[k] = v
			}
			ovCheckCounters(t, o, want) // 計數照常
			if n := ovOpaque(o, 16, 24, 56, 32); n == 0 {
				t.Fatal("故障注入要讓原文上的疊字留著")
			}
		})
	}
}

// ---------- 純函式 ----------

func TestOverlayGroupDY(t *testing.T) {
	rec := &EventRecord{Row: 5}
	mk := func(ys ...int) []*xlate.Stamp {
		var out []*xlate.Stamp
		for _, y := range ys {
			out = append(out, &xlate.Stamp{Y: y})
		}
		return out
	}
	cases := []struct {
		name   string
		stamps []*xlate.Stamp
		dy     int
		ok     bool
	}{
		{"沒有疊字", nil, 0, false},
		{"無位移", mk(40, 40), 0, true},
		{"下移一列", mk(48, 48, 48), 8, true},
		{"上捲一列", mk(32), -8, true},
		{"各疊字不一致", mk(48, 49), 0, false},
		{"第一筆以外不一致", mk(40, 40, 32), 0, false},
	}
	for _, c := range cases {
		dy, ok := groupDY(c.stamps, rec)
		if dy != c.dy || ok != c.ok {
			t.Errorf("%s：groupDY = (%d, %v)，要 (%d, %v)", c.name, dy, ok, c.dy, c.ok)
		}
	}
}

func TestOverlayVisibleRangesAndHiddenOf(t *testing.T) {
	// s1：四格全形 [16,48)，第 0 格透明（Transparent 比 Cells 短，缺的當不透明）；s2：兩格半形 [60,68)。
	s1 := &xlate.Stamp{X: 16, Cells: 4, CellW: 8, Transparent: []bool{true, false}}
	s2 := &xlate.Stamp{X: 60, Cells: 2, CellW: 4}
	if got, want := visibleRanges([]*xlate.Stamp{s1, s2}), []xrange{{24, 48}, {60, 68}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("visibleRanges = %v，要 %v", got, want)
	}
	// 相鄰的範圍合併。
	s3 := &xlate.Stamp{X: 48, Cells: 3, CellW: 4}
	if got, want := visibleRanges([]*xlate.Stamp{s2, s1, s3}), []xrange{{24, 68}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("visibleRanges（相鄰合併）= %v，要 %v", got, want)
	}
	if got := visibleRanges(nil); len(got) != 0 {
		t.Fatalf("visibleRanges(nil) = %v", got)
	}

	rec := &EventRecord{Col: 2, Row: 3, Text: "ABCDEFGH"} // 事件矩形 x [16,80)
	if got, want := hiddenOf(rec, []*xlate.Stamp{s1, s2}), []xrange{{16, 24}, {48, 60}, {68, 80}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("hiddenOf = %v，要 %v", got, want)
	}
	if got, want := hiddenOf(rec, nil), []xrange{{16, 80}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("hiddenOf（沒有疊字）= %v，要 %v", got, want)
	}
	full := &xlate.Stamp{X: 16, Cells: 8, CellW: 8}
	if got := hiddenOf(rec, []*xlate.Stamp{full}); len(got) != 0 {
		t.Fatalf("hiddenOf（全部可見）= %v，要空", got)
	}
	// 補集只看可見範圍，與 Transparent 旗標無關：被移除的疊字範圍也算不可見。
	if got, want := hiddenOf(rec, []*xlate.Stamp{{X: 16, Cells: 4, CellW: 8}}), []xrange{{48, 80}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("hiddenOf（後半段已移除）= %v，要 %v", got, want)
	}
	clip := &EventRecord{Col: 38, Row: 0, Text: "Hello"} // x [304,344) 裁切到 [304,320)
	if got, want := hiddenOf(clip, nil), []xrange{{304, 320}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("hiddenOf（裁切）= %v，要 %v", got, want)
	}
	if got := hiddenOf(&EventRecord{Col: 40, Text: "Hi"}, nil); got != nil {
		t.Fatalf("hiddenOf（在畫布外）= %v，要 nil", got)
	}
}

func TestOverlayReplaceGroup(t *testing.T) {
	mk := func(key string, x int) *xlate.Stamp {
		return &xlate.Stamp{Key: key, X: x, Y: 0, Cells: 1, CellW: 8, CellH: 8}
	}
	keys := func(o *Overlay) string {
		var parts []string
		for _, s := range o.Layer.Stamps {
			parts = append(parts, s.Key+"@"+strconv.Itoa(s.X))
		}
		return strings.Join(parts, " ")
	}
	o := NewOverlay()
	// a 與 b 的第一筆同原點 (0,0)；a 的另一筆在最後。
	o.Layer.Stamps = []*xlate.Stamp{mk("a", 0), mk("b", 0), mk("a", 16)}

	o.replaceGroup("a", []*xlate.Stamp{mk("a2", 0), mk("a2", 8)})
	if got, want := keys(o), "a2@0 a2@8 b@0"; got != want {
		t.Fatalf("取代 a 之後 = %q，要 %q（新疊字插在舊組第一筆的位置，同原點的 b 不受影響）", got, want)
	}
	o.replaceGroup("b", []*xlate.Stamp{mk("b2", 0)})
	if got, want := keys(o), "a2@0 a2@8 b2@0"; got != want {
		t.Fatalf("取代 b 之後 = %q，要 %q（同原點的 a2 不受影響）", got, want)
	}
	o.replaceGroup("zzz", []*xlate.Stamp{mk("n", 0)})
	if got, want := keys(o), "a2@0 a2@8 b2@0 n@0"; got != want {
		t.Fatalf("取代不存在的組 = %q，要 %q（附加在最後）", got, want)
	}
}
