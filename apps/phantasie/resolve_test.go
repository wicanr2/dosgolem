package phantasie

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// 本檔驗證 Resolver.Resolve（docs/spec/003 §5、§9、§13 第 3、4 項；001 §5）。
// 期望值是依規格推導出來的字面值，不用被測函式的輸出當期望。
// 測試輔助函式一律加前綴 rs，避免與同套件其他測試撞名。

// rsC 是置中標記：003 §3 規定記憶體內以私用區字元 U+E000 表示。NewCatalog 收的是已解析的資料，
// 所以假 catalog 的置中譯文要用它，不是檔案內的 \c。
const rsC = ""

// 已知摘要鍵，以主機 sha256sum 取前 12 位十六進位算出來後寫死（不經 Digest）。
const (
	rsDigRO      = "h:0049aa51a630" // sha256("RO")
	rsDigScroll  = "h:ce8ed418ebde" // sha256("  SCROLL TITLE")，開頭兩個空白保留
	rsDigWarrior = "h:956e39907ed9" // sha256("WARRIOR")
)

// rsWide 是測試字型的寬度函式：CJK 統一表意文字（U+4E00 至 U+9FFF）2 h，U+2026 也是 2 h，其餘 1 h。
// U+2026 不在 CJK 區間，用來證明寬度來自傳進去的函式。
func rsWide(r rune) bool { return (r >= 0x4E00 && r <= 0x9FFF) || r == 0x2026 }

// rsEllipsisNarrow 與 rsWide 相同，但 U+2026 是 1 h。
func rsEllipsisNarrow(r rune) bool { return r >= 0x4E00 && r <= 0x9FFF }

func rsSp(n int) string { return strings.Repeat(" ", n) }

// rsUI 回測試用的 ui 表：基底加上 kv 成對覆寫。每個案例各自一份，不共用。
func rsUI(kv ...string) map[string]string {
	m := map[string]string{
		"NEW GAME":    "新遊戲",
		"  LEADING":   "  開頭",
		"+Sound":      "開音效",
		"-Sound":      "關音效",
		"TITLE":       rsC + "標題",
		"BLANKME":     "<blank>",
		"EMPTY":       "",
		"WARRIOR":     "戰士",
		"FIGHTER":     "戰士",
		"GOBLIN":      "哥布林",
		"PERNOR":      "佩爾諾",
		"FIGHT":       "戰鬥",
		"RUN":         "逃跑",
		"DRAGON":      "…A龍",
		"100% DONE":   "百分之百完成",
		"LEAVING %s":  "離開 %s",
		"ARRIVING %s": rsC + "抵達 %s",
		"EMPTYTPL %s": "",
		"LEVEL%3u %s": "等級%3u %s",
		"HP %d/%d":    "體力 %d/%d",
		"%d%% DONE":   "完成 %d%%",
		"MARK %c ON":  "標記 %c 開",
		"%s HITS %s":  "%s 擊中 %s",
		"%s VS %s":    "%s 對 %s",
		"ACTION %s:":  "行動 %s:",
		"WHOOP %s":    rsC + "哇 %s",
		"THEN %s":     "然後 %s",
		"FOE %.1s":    "敵 %.1s",
		"BO%sB":       "甲",
	}
	// 基底刻意不放 "%s" 之類的恆等鍵：它會改變恆等路徑，要測的案例自己用 kv 加。
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return m
}

// rsProse 回測試用的 prose 表：基底加上 kv 成對覆寫。
func rsProse(kv ...string) map[string]string {
	m := map[string]string{
		rsDigRO:      "羅",
		rsDigScroll:  rsC + "卷軸標題",
		rsDigWarrior: "勇者", // 與 ui 的「戰士」不同，用來看出查的是哪個家族
	}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return m
}

var rsProtected = []string{"PROTECTED HINT", "PROTECTED FMT %d", "PROTECTED INNER %s", "PROTECTED %x"}

// 事件與引數的建構輔助。
func rsLit(text string, k Kind) EventRecord { return EventRecord{Text: text, Format: text, FmtKind: k} }

func rsTpl(format string, k Kind, args ...ArgStr) EventRecord {
	return EventRecord{Format: format, FmtKind: k, ArgStrs: args}
}

func rsW(w ...uint16) [12]uint16 {
	var a [12]uint16
	copy(a[:], w)
	return a
}

func rsWith(rec EventRecord, w ...uint16) EventRecord {
	rec.Args = rsW(w...)
	return rec
}

func rsArg(content string, k Kind) ArgStr { return ArgStr{Ptr: 0x1234, Content: content, Kind: k} }

func rsPieceArg(content string, p *Piece) ArgStr {
	return ArgStr{Ptr: 0x638E, Content: content, Kind: KindBuffer, Piece: p}
}

func rsSorted(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

// rsEqSet 以排序後逐項比較（Hits、Missed 是鍵集合）；nil 與空切片視為相同，重複項會被看出來。
func rsEqSet(a, b []string) bool {
	a, b = rsSorted(a), rsSorted(b)
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

// rsCase 是一個 Resolve 案例。zh、center、hits 只在 ok 時比對；key、missed 一律比對。
type rsCase struct {
	name   string
	ui     map[string]string
	prose  map[string]string
	prot   []string
	wide   WideFunc // nil 表示 rsWide
	rec    EventRecord
	ok     bool
	why    Reason
	zh     string
	center bool
	key    string
	hits   []string
	missed []string
}

func rsResolver(c rsCase) *Resolver {
	w := c.wide
	if w == nil {
		w = rsWide
	}
	return &Resolver{Cat: NewCatalog(c.ui, c.prose, c.prot), Wide: w}
}

func rsRun(t *testing.T, cases []rsCase) {
	t.Helper()
	for _, c := range cases {
		c := c
		if c.ui == nil {
			c.ui = rsUI()
		}
		if c.prose == nil {
			c.prose = rsProse()
		}
		t.Run(c.name, func(t *testing.T) {
			rec := c.rec
			res := rsResolver(c).Resolve(&rec)
			if res.OK != c.ok || res.Why != c.why {
				t.Fatalf("OK=%v Why=%v，要 OK=%v Why=%v（Zh=%q）", res.OK, res.Why, c.ok, c.why, string(res.Zh))
			}
			if res.OK {
				if string(res.Zh) != c.zh {
					t.Errorf("Zh = %q，要 %q", string(res.Zh), c.zh)
				}
				if res.Center != c.center {
					t.Errorf("Center = %v，要 %v", res.Center, c.center)
				}
				if strings.ContainsRune(string(res.Zh), '') || strings.Contains(string(res.Zh), `\c`) {
					t.Errorf("Zh = %q 含置中標記", string(res.Zh))
				}
				if !rsEqSet(res.Hits, c.hits) {
					t.Errorf("Hits = %q，要 %q", res.Hits, c.hits)
				}
			}
			if res.Key != c.key {
				t.Errorf("Key = %q，要 %q", res.Key, c.key)
			}
			if !rsEqSet(res.Missed, c.missed) {
				t.Errorf("Missed = %q，要 %q", res.Missed, c.missed)
			}
		})
	}
}

func TestResolveCenterMarkIsPrivateUse(t *testing.T) {
	if string(CenterMark) != "" {
		t.Errorf("CenterMark = %U，003 §3 規定是 U+E000", CenterMark)
	}
}

func TestResolveReasonStrings(t *testing.T) {
	cases := []struct {
		r    Reason
		want string
	}{
		{WhyNone, "ok"}, {WhyProtected, "protected"}, {WhyBadFormat, "badformat"}, {WhyNoKey, "nokey"},
		{WhyBlankMissing, "blank_missing"}, {WhyPassthrough, "passthrough"}, {WhyIdentity, "identity"}, {WhyBadArg, "badarg"},
	}
	for _, c := range cases {
		if got := c.r.String(); got != c.want {
			t.Errorf("Reason(%d).String() = %q，要 %q", c.r, got, c.want)
		}
	}
}

// 字面事件（沒有轉換）：003 §5.2「字面」分支與 §13 第 3 項。
func TestResolveLiteral(t *testing.T) {
	rsRun(t, []rsCase{
		{name: "靜態字面命中", rec: rsLit("NEW GAME", KindStatic),
			ok: true, zh: "新遊戲", hits: []string{"NEW GAME"}},
		{name: "尾端空白先規範化再查", rec: rsLit("NEW GAME      ", KindStatic),
			ok: true, zh: "新遊戲", hits: []string{"NEW GAME"}},
		{name: "開頭空白保留為鍵的一部分", rec: rsLit("  LEADING   ", KindStatic),
			ok: true, zh: "  開頭", hits: []string{"  LEADING"}},
		{name: "少了開頭空白就是另一個鍵", rec: rsLit("LEADING", KindStatic),
			why: WhyNoKey, key: "LEADING"},
		{name: "字面鍵取 Text 不取 Format（P 類修補只改 Text）", rec: EventRecord{Format: "+Sound", Text: "-Sound ", FmtKind: KindStatic},
			ok: true, zh: "關音效", hits: []string{"-Sound"}},
		{name: "靜態缺譯填 Key", rec: rsLit("UNKNOWN", KindStatic),
			why: WhyNoKey, key: "UNKNOWN"},
		{name: "buffer 缺譯不填 Key", rec: rsLit("UNKNOWN", KindBuffer),
			why: WhyNoKey},
		{name: "other 種類的字面事件不查 ui", rec: rsLit("NEW GAME", KindOther),
			why: WhyNoKey},
		{name: "other 種類的字面事件不查 prose", rec: rsLit("RO", KindOther),
			why: WhyNoKey},
		{name: "buffer 查 prose 摘要鍵且不查 ui", ui: rsUI("RO", "不該用這個"), rec: rsLit("RO", KindBuffer),
			ok: true, zh: "羅", hits: []string{rsDigRO}},
		{name: "buffer 的尾端空白不影響摘要", rec: rsLit("RO     ", KindBuffer),
			ok: true, zh: "羅", hits: []string{rsDigRO}},
		{name: "static 不查 prose", rec: rsLit("RO", KindStatic),
			why: WhyNoKey, key: "RO"},
		{name: "<blank> 譯文 OK 且 Zh 為空", rec: rsLit("BLANKME", KindStatic),
			ok: true, zh: "", hits: []string{"BLANKME"}},
		{name: "ui 空字串譯文 blank_missing 且填 Key", rec: rsLit("EMPTY", KindStatic),
			why: WhyBlankMissing, key: "EMPTY"},
		{name: "prose 空字串譯文 blank_missing 不填 Key", prose: rsProse(rsDigRO, ""), rec: rsLit("RO", KindBuffer),
			why: WhyBlankMissing},
		{name: "置中標記只由 Center 表達", rec: rsLit("TITLE", KindStatic),
			ok: true, zh: "標題", center: true, hits: []string{"TITLE"}},
		{name: "prose 卷軸標題的置中", rec: rsLit("  SCROLL TITLE  ", KindBuffer),
			ok: true, zh: "卷軸標題", center: true, hits: []string{rsDigScroll}},
		{name: "%% 讀作 % 後當字面鍵", rec: EventRecord{Format: "100%% DONE", Text: "100% DONE", FmtKind: KindStatic},
			ok: true, zh: "百分之百完成", hits: []string{"100% DONE"}},
	})
}

// 原文沒有字母：passthrough，不填 Key，不列入鍵集合（003 §5.2、規則補充）。
func TestResolvePassthrough(t *testing.T) {
	rsRun(t, []rsCase{
		{name: "分隔線", rec: rsLit("----------", KindStatic), why: WhyPassthrough},
		{name: "星號", rec: rsLit("* * *", KindStatic), why: WhyPassthrough},
		{name: "數字", rec: rsLit("12345", KindStatic), why: WhyPassthrough},
		{name: "buffer 種類也是", rec: rsLit("----------", KindBuffer), why: WhyPassthrough},
		{name: "other 種類也是", rec: rsLit("----------", KindOther), why: WhyPassthrough},
		{name: "單一 %c 事件（沒有 %s 轉換的恆等模板）", rec: rsWith(rsTpl("%c", KindStatic), '+'), why: WhyPassthrough},
		{name: "只有數字轉換", rec: rsWith(rsTpl("%-5d|", KindStatic), 3), why: WhyPassthrough},
		{name: "12 個字組剛好夠用不算 badarg", rec: rsTpl("%ld%ld%ld%ld%ld%ld", KindStatic), why: WhyPassthrough},
	})
}

// 模板事件（含轉換）：003 §5.2「含轉換」分支、§7.2 的版面語意。
func TestResolveTemplate(t *testing.T) {
	rsRun(t, []rsCase{
		{name: "模板命中且數字欄寬以 2W h 補白",
			rec: rsWith(rsTpl("LEVEL%3u %s", KindStatic, rsArg("WARRIOR", KindStatic)), 5, 0x8000),
			ok:  true, zh: "等級" + rsSp(5) + "5 戰士", hits: []string{"LEVEL%3u %s", "WARRIOR"}},
		{name: "格式字串尾端空白先 rstrip 再當鍵",
			rec: rsWith(rsTpl("HP %d/%d  ", KindStatic), 30, 45),
			ok:  true, zh: "體力 30/45", hits: []string{"HP %d/%d"}},
		{name: "%% 在模板內讀作 %",
			rec: rsWith(rsTpl("%d%% DONE", KindStatic), 50),
			ok:  true, zh: "完成 50%", hits: []string{"%d%% DONE"}},
		{name: "%c 佔一格：字元加 1 h 空白",
			rec: rsWith(rsTpl("MARK %c ON", KindStatic), 'A'),
			ok:  true, zh: "標記 A  開", hits: []string{"MARK %c ON"}},
		{name: "模板缺譯：Key 是 rstrip 後的格式字串",
			rec: rsWith(rsTpl("ZZ %d  ", KindStatic), 1),
			why: WhyNoKey, key: "ZZ %d"},
		{name: "模板譯文為空字串視同缺",
			rec: rsTpl("EMPTYTPL %s", KindStatic, rsArg("WARRIOR", KindStatic)),
			why: WhyNoKey, key: "EMPTYTPL %s"},
		{name: "模板的置中標記",
			rec: rsTpl("ARRIVING %s", KindStatic, rsArg("PERNOR    ", KindTown)),
			ok:  true, zh: "抵達 佩爾諾", center: true, hits: []string{"ARRIVING %s", "PERNOR"}},
		{name: "城鎮名引數（補空白）查 ui",
			rec: rsTpl("LEAVING %s", KindStatic, rsArg("PERNOR    ", KindTown)),
			ok:  true, zh: "離開 佩爾諾", hits: []string{"LEAVING %s", "PERNOR"}},
		{name: "城鎮名缺譯：保留原字串並列入 Missed，事件仍 OK",
			rec: rsTpl("LEAVING %s", KindStatic, rsArg("XYZTOWN   ", KindTown)),
			ok:  true, zh: "離開 XYZTOWN   ", hits: []string{"LEAVING %s"}, missed: []string{"XYZTOWN"}},
		{name: "有 %s 引數的缺譯模板含英文字面時不當恆等",
			rec: rsTpl("ZZ %s", KindStatic, rsArg("WARRIOR", KindStatic)),
			why: WhyNoKey, key: "ZZ %s"},
		{name: "欄位寬度與精度：部分替換（一個引數缺譯、另一個替換）",
			rec: rsTpl("%-10.9s%-9.8s", KindStatic, rsArg("FIGHTER", KindStatic), rsArg("ELF", KindStatic)),
			ok:  true, zh: "戰士" + rsSp(16) + "ELF" + rsSp(15), hits: []string{"FIGHTER"}, missed: []string{"ELF"}},
		{name: "原樣保留的 ASCII 名字以 P 個字元為上限，譯文以 2P h 為上限",
			rec: rsTpl("%-10.9s%-9.8s", KindStatic, rsArg("ABCDEFGHIJ", KindOther), rsArg("FIGHTER", KindStatic)),
			ok:  true, zh: "ABCDEFGHI" + rsSp(11) + "戰士" + rsSp(14), hits: []string{"FIGHTER"}},
		{name: "兩個缺譯引數都列入 Missed",
			rec: rsTpl("%s VS %s", KindStatic, rsArg("AAA", KindStatic), rsArg("BBB", KindMonster)),
			ok:  true, zh: "AAA 對 BBB", hits: []string{"%s VS %s"}, missed: []string{"AAA", "BBB"}},
		{name: "一個替換一個缺譯",
			rec: rsTpl("%s VS %s", KindStatic, rsArg("WARRIOR", KindStatic), rsArg("BBB", KindMonster)),
			ok:  true, zh: "戰士 對 BBB", hits: []string{"%s VS %s", "WARRIOR"}, missed: []string{"BBB"}},
		{name: "Hits 與 Missed 是集合：同一鍵只列一次",
			rec: rsTpl("%s VS %s", KindStatic, rsArg("GOBLIN", KindMonster), rsArg("GOBLIN", KindStatic)),
			ok:  true, zh: "哥布林 對 哥布林", hits: []string{"%s VS %s", "GOBLIN"}},
		{name: "Missed 重複的缺譯鍵只列一次",
			rec: rsTpl("%s VS %s", KindStatic, rsArg("AAA", KindStatic), rsArg("AAA", KindStatic)),
			ok:  true, zh: "AAA 對 AAA", hits: []string{"%s VS %s"}, missed: []string{"AAA"}},
		{name: "buffer 引數缺譯不列入 Missed",
			rec: rsTpl("LEAVING %s", KindStatic, rsArg("ZZZ", KindBuffer)),
			ok:  true, zh: "離開 ZZZ", hits: []string{"LEAVING %s"}},
		{name: "other 引數不查表也不列入 Missed",
			rec: rsTpl("LEAVING %s", KindStatic, rsArg("WARRIOR", KindOther)),
			ok:  true, zh: "離開 WARRIOR", hits: []string{"LEAVING %s"}},
		{name: "空白引數原樣保留且不計 Missed",
			rec: rsTpl("LEAVING %s", KindStatic, rsArg("   ", KindStatic)),
			ok:  true, zh: "離開    ", hits: []string{"LEAVING %s"}},
		{name: "空內容引數原樣保留且不計 Missed",
			rec: rsTpl("LEAVING %s", KindStatic, rsArg("", KindMonster)),
			ok:  true, zh: "離開 ", hits: []string{"LEAVING %s"}},
		{name: "引數譯文為空字串視同缺：保留原字串並列入 Missed",
			rec: rsTpl("LEAVING %s", KindStatic, rsArg("EMPTY", KindStatic)),
			ok:  true, zh: "離開 EMPTY", hits: []string{"LEAVING %s"}, missed: []string{"EMPTY"}},
	})
}

// 恆等模板（沒有英文字面）：003 §5.2、規則補充。
func TestResolveIdentity(t *testing.T) {
	rsRun(t, []rsCase{
		{name: "靜態引數被替換：OK",
			rec: rsTpl("%s", KindStatic, rsArg("WARRIOR", KindStatic)),
			ok:  true, zh: "戰士", hits: []string{"WARRIOR"}},
		{name: "怪物名查 ui",
			rec: rsTpl("%s", KindStatic, rsArg("GOBLIN", KindMonster)),
			ok:  true, zh: "哥布林", hits: []string{"GOBLIN"}},
		{name: "城鎮名查 ui",
			rec: rsTpl("%s", KindStatic, rsArg("PERNOR    ", KindTown)),
			ok:  true, zh: "佩爾諾", hits: []string{"PERNOR"}},
		{name: "靜態引數缺譯：identity，Missed 列出，不填 Key",
			rec: rsTpl("%s", KindStatic, rsArg("ZZZ", KindStatic)),
			why: WhyIdentity, missed: []string{"ZZZ"}},
		{name: "怪物名缺譯：identity，Missed 列出",
			rec: rsTpl("%s", KindStatic, rsArg("BEHOLDER", KindMonster)),
			why: WhyIdentity, missed: []string{"BEHOLDER"}},
		{name: "城鎮名缺譯：identity，Missed 列出（規範化後）",
			rec: rsTpl("%s", KindStatic, rsArg("XYZTOWN   ", KindTown)),
			why: WhyIdentity, missed: []string{"XYZTOWN"}},
		{name: "引數譯文為空字串：identity，Missed 列出",
			rec: rsTpl("%s", KindStatic, rsArg("EMPTY", KindStatic)),
			why: WhyIdentity, missed: []string{"EMPTY"}},
		{name: "負對照：玩家名（other）恰等於 ui 鍵不翻譯",
			rec: rsTpl("%s", KindStatic, rsArg("WARRIOR", KindOther)),
			why: WhyIdentity},
		{name: "負對照：玩家名（other）恰等於 prose 鍵不翻譯",
			rec: rsTpl("%s", KindStatic, rsArg("RO", KindOther)),
			why: WhyIdentity},
		{name: "buffer 引數查 prose 摘要鍵，不查 ui",
			ui:  rsUI("RO", "不該用這個", "WARRIOR", "不該用這個"),
			rec: rsTpl("%s", KindStatic, rsArg("RO", KindBuffer)),
			ok:  true, zh: "羅", hits: []string{rsDigRO}},
		{name: "buffer 引數與 ui 同名時用 prose 的譯文",
			rec: rsTpl("%s", KindStatic, rsArg("WARRIOR", KindBuffer)),
			ok:  true, zh: "勇者", hits: []string{rsDigWarrior}},
		{name: "buffer 引數缺譯：identity，不列入 Missed",
			rec: rsTpl("%s", KindStatic, rsArg("ZZZ", KindBuffer)),
			why: WhyIdentity},
		{name: "static 引數不查 prose：缺 ui 鍵就缺譯",
			rec: rsTpl("%s", KindStatic, rsArg("RO", KindStatic)),
			why: WhyIdentity, missed: []string{"RO"}},
		{name: "卷軸標題 %s 事件：Center 為真且 Zh 不含標記",
			rec: rsTpl("%s", KindStatic, rsArg("  SCROLL TITLE  ", KindBuffer)),
			ok:  true, zh: "卷軸標題", center: true, hits: []string{rsDigScroll}},
		{name: "靜態 %s 引數的置中譯文（單一 %s）",
			rec: rsTpl("%s", KindStatic, rsArg("TITLE", KindStatic)),
			ok:  true, zh: "標題", center: true, hits: []string{"TITLE"}},
		{name: "資料引數帶置中標記但不是單一 %s：badarg",
			rec: rsTpl("LEAVING %s", KindStatic, rsArg("TITLE", KindStatic)),
			why: WhyBadArg},
		{name: "資料引數帶置中標記但恆等模板有兩個 %s：badarg",
			rec: rsTpl("%s %s", KindStatic, rsArg("TITLE", KindStatic), rsArg("BOB", KindOther)),
			why: WhyBadArg},
		{name: "恆等模板有兩個 %s 且其一被替換：OK",
			rec: rsTpl("%s %s", KindStatic, rsArg("BOB", KindOther), rsArg("GOBLIN", KindMonster)),
			ok:  true, zh: "BOB 哥布林", hits: []string{"GOBLIN"}},
		{name: "兩個 %s 都沒替換：identity",
			rec: rsTpl("%s %s", KindStatic, rsArg("BOB", KindOther), rsArg("ZZZ", KindStatic)),
			why: WhyIdentity, missed: []string{"ZZZ"}},
	})
}

// FmtKind 不是 static：003 §5.1 末段、§5.2 的 p.FmtKind 不是 static 分支。
func TestResolveFmtKindNotStatic(t *testing.T) {
	rsRun(t, []rsCase{
		{name: "other 格式字串含英文字面：nokey 且不填 Key（名字含 % 不得查 ui）",
			rec: rsTpl("BO%sB", KindOther, rsArg("WARRIOR", KindStatic)),
			why: WhyNoKey},
		{name: "buffer 格式字串含英文字面：nokey 且不填 Key",
			rec: rsTpl("BO%sB", KindBuffer, rsArg("WARRIOR", KindStatic)),
			why: WhyNoKey},
		{name: "other 格式字串名字含 %d：同樣不查 ui",
			ui:  rsUI("AL%dX", "不該用這個"),
			rec: rsWith(rsTpl("AL%dX", KindOther), 7),
			why: WhyNoKey},
		{name: "buffer 的恆等模板 %s 仍替換引數",
			rec: rsTpl("%s", KindBuffer, rsArg("WARRIOR", KindStatic)),
			ok:  true, zh: "戰士", hits: []string{"WARRIOR"}},
		{name: "other 的恆等模板 %s 仍替換引數",
			rec: rsTpl("%s", KindOther, rsArg("GOBLIN", KindMonster)),
			ok:  true, zh: "哥布林", hits: []string{"GOBLIN"}},
		{name: "other 的恆等模板不查 ui 鍵（即使 ui 有同名模板鍵）",
			ui:  rsUI("%-5d|", "不該用這個"),
			rec: rsWith(rsTpl("%-5d|", KindOther), 3),
			why: WhyPassthrough},
		{name: "other 的恆等模板沒有被替換引數：identity",
			rec: rsTpl("%s", KindOther, rsArg("ZZZ", KindStatic)),
			why: WhyIdentity, missed: []string{"ZZZ"}},
	})
}

// 組句：Composed、巢狀 Piece、Appends（003 §5.2 第 2 步、resolvePiece、§13 第 3 項）。
func TestResolveComposed(t *testing.T) {
	battle := func(apps ...ArgStr) *Piece {
		return &Piece{
			Fmt: "ACTION %s:", FmtKind: KindStatic,
			ArgStrs: []ArgStr{rsArg("GOBLIN", KindMonster)},
			Appends: apps,
		}
	}
	rsRun(t, []rsCase{
		{name: "Composed 取代事件自己的格式字串與引數",
			rec: EventRecord{Format: "%s", FmtKind: KindStatic, Composed: &Piece{
				Fmt: "%s HITS %s", FmtKind: KindStatic,
				ArgStrs: []ArgStr{rsArg("GOBLIN", KindMonster), rsArg("WARRIOR", KindStatic)},
			}},
			ok: true, zh: "哥布林 擊中 戰士", hits: []string{"%s HITS %s", "GOBLIN", "WARRIOR"}},
		{name: "Composed 的字面 Piece 取 Literal 當字面鍵",
			rec: EventRecord{Format: "X", Text: "X", FmtKind: KindStatic, Composed: &Piece{
				Fmt: "FIGHT", FmtKind: KindStatic, Literal: "FIGHT",
			}},
			ok: true, zh: "戰鬥", hits: []string{"FIGHT"}},
		{name: "Composed 的 FmtKind 不是 static 且含英文字面：nokey",
			rec: EventRecord{Format: "%s", FmtKind: KindStatic, Composed: &Piece{
				Fmt: "%s HITS %s", FmtKind: KindBuffer,
				ArgStrs: []ArgStr{rsArg("GOBLIN", KindMonster), rsArg("WARRIOR", KindStatic)},
			}},
			why: WhyNoKey},
		{name: "Composed 加 Appends 依序接在格式化結果之後",
			rec: EventRecord{Format: "%s", FmtKind: KindStatic,
				Composed: battle(rsArg("FIGHT", KindStatic), rsArg("RUN", KindStatic))},
			ok: true, zh: "行動 哥布林:戰鬥逃跑", hits: []string{"ACTION %s:", "GOBLIN", "FIGHT", "RUN"}},
		{name: "Appends 缺譯：保留原字串並列入 Missed",
			rec: EventRecord{Format: "%s", FmtKind: KindStatic,
				Composed: battle(rsArg("ZZZ", KindStatic))},
			ok: true, zh: "行動 哥布林:ZZZ", hits: []string{"ACTION %s:", "GOBLIN"}, missed: []string{"ZZZ"}},
		{name: "戰鬥指令列：外層 %s 的引數是帶 Appends 的組句 Piece",
			rec: rsTpl("%s", KindStatic,
				rsPieceArg("ACTION GOBLIN:FIGHT RUN", battle(rsArg("FIGHT", KindStatic), rsArg("RUN", KindStatic)))),
			ok: true, zh: "行動 哥布林:戰鬥逃跑", hits: []string{"ACTION %s:", "GOBLIN", "FIGHT", "RUN"}},
		{name: "內層模板缺譯：外層恆等 identity，內層的鍵列入 Missed 且不填 Key",
			rec: rsTpl("%s", KindStatic,
				rsPieceArg("ZZ GOBLIN:", &Piece{Fmt: "ZZ %s:", FmtKind: KindStatic, ArgStrs: []ArgStr{rsArg("GOBLIN", KindMonster)}})),
			why: WhyIdentity, missed: []string{"ZZ %s:"}},
		{name: "內層成功但引數缺譯：外層 OK，Missed 帶出",
			rec: rsTpl("%s", KindStatic,
				rsPieceArg("ACTION BEHOLDER:", &Piece{Fmt: "ACTION %s:", FmtKind: KindStatic, ArgStrs: []ArgStr{rsArg("BEHOLDER", KindMonster)}})),
			ok: true, zh: "行動 BEHOLDER:", hits: []string{"ACTION %s:"}, missed: []string{"BEHOLDER"}},
		{name: "內層的置中不傳遞",
			rec: rsTpl("%s", KindStatic,
				rsPieceArg("WHOOP GOBLIN", &Piece{Fmt: "WHOOP %s", FmtKind: KindStatic, ArgStrs: []ArgStr{rsArg("GOBLIN", KindMonster)}})),
			ok: true, zh: "哇 哥布林", center: false, hits: []string{"WHOOP %s", "GOBLIN"}},
		{name: "組句 Piece 當含英文字面模板的 %s 引數",
			rec: rsTpl("THEN %s", KindStatic,
				rsPieceArg("WHOOP GOBLIN", &Piece{Fmt: "WHOOP %s", FmtKind: KindStatic, ArgStrs: []ArgStr{rsArg("GOBLIN", KindMonster)}})),
			ok: true, zh: "然後 哇 哥布林", hits: []string{"THEN %s", "WHOOP %s", "GOBLIN"}},
		{name: "巢狀兩層：最內層成功帶出鍵",
			rec: rsTpl("THEN %s", KindStatic,
				rsPieceArg("THEN WHOOP GOBLIN", &Piece{Fmt: "THEN %s", FmtKind: KindStatic, ArgStrs: []ArgStr{
					rsPieceArg("WHOOP GOBLIN", &Piece{Fmt: "WHOOP %s", FmtKind: KindStatic, ArgStrs: []ArgStr{rsArg("GOBLIN", KindMonster)}}),
				}})),
			ok: true, zh: "然後 然後 哇 哥布林", hits: []string{"THEN %s", "WHOOP %s", "GOBLIN"}},
		{name: "Appends 被換成譯文算替換：恆等模板 OK",
			rec: EventRecord{Format: "%s", FmtKind: KindStatic, Composed: &Piece{
				Fmt: "%s ", FmtKind: KindStatic,
				ArgStrs: []ArgStr{rsArg("BOB", KindOther)},
				Appends: []ArgStr{rsArg("RUN", KindStatic)},
			}},
			ok: true, zh: "BOB 逃跑", hits: []string{"RUN"}},
		{name: "Appends 也沒被替換：identity，缺譯的追加字串列入 Missed",
			rec: EventRecord{Format: "%s", FmtKind: KindStatic, Composed: &Piece{
				Fmt: "%s ", FmtKind: KindStatic,
				ArgStrs: []ArgStr{rsArg("BOB", KindOther)},
				Appends: []ArgStr{rsArg("ZZZ", KindStatic)},
			}},
			why: WhyIdentity, missed: []string{"ZZZ"}},
		{name: "Appends 的 other 種類不查表",
			rec: EventRecord{Format: "%s", FmtKind: KindStatic, Composed: &Piece{
				Fmt: "%s ", FmtKind: KindStatic,
				ArgStrs: []ArgStr{rsArg("GOBLIN", KindMonster)},
				Appends: []ArgStr{rsArg("RUN", KindOther)},
			}},
			ok: true, zh: "哥布林 RUN", hits: []string{"GOBLIN"}},
	})
}

// 保護清單：003 §9。事件文字、格式字串、組句與引數 Piece（遞迴）的格式字串任一命中都回 protected。
func TestResolveProtected(t *testing.T) {
	prot := func(c rsCase) rsCase {
		c.prot = rsProtected
		c.why = WhyProtected
		return c
	}
	deep := &Piece{Fmt: "ACTION %s:", FmtKind: KindStatic, ArgStrs: []ArgStr{
		rsPieceArg("x", &Piece{Fmt: "PROTECTED INNER %s", FmtKind: KindStatic}),
	}}
	rsRun(t, []rsCase{
		prot(rsCase{name: "事件文字命中（即使 ui 有譯文）",
			ui:  rsUI("PROTECTED HINT", "不該用這個"),
			rec: rsLit("PROTECTED HINT", KindStatic)}),
		prot(rsCase{name: "事件文字尾端空白規範化後命中",
			rec: rsLit("PROTECTED HINT      ", KindStatic)}),
		prot(rsCase{name: "格式字串命中（Text 是格式化後的文字，不在清單）",
			ui:  rsUI("PROTECTED FMT %d", "不該用這個"),
			rec: EventRecord{Format: "PROTECTED FMT %d", Text: "PROTECTED FMT 5", FmtKind: KindStatic, Args: rsW(5)}}),
		prot(rsCase{name: "格式字串命中且同時是不支援的規格：保護優先於 badformat",
			rec: EventRecord{Format: "PROTECTED %x", Text: "PROTECTED 1F", FmtKind: KindStatic}}),
		prot(rsCase{name: "組句的格式字串命中",
			rec: EventRecord{Format: "%s", FmtKind: KindStatic, Composed: &Piece{
				Fmt: "PROTECTED FMT %d", FmtKind: KindStatic, Args: rsW(5)}}}),
		prot(rsCase{name: "事件 %s 引數的 Piece 格式字串命中",
			rec: rsTpl("%s", KindStatic, rsPieceArg("x", &Piece{Fmt: "PROTECTED INNER %s", FmtKind: KindStatic}))}),
		prot(rsCase{name: "事件引數 Piece 的巢狀引數 Piece 命中（兩層）",
			rec: rsTpl("%s", KindStatic, rsPieceArg("x", deep))}),
		prot(rsCase{name: "組句 Piece 的引數 Piece 命中",
			rec: EventRecord{Format: "%s", FmtKind: KindStatic, Composed: deep}}),
		prot(rsCase{name: "組句 Piece 的巢狀引數 Piece 命中（三層）",
			rec: EventRecord{Format: "%s", FmtKind: KindStatic, Composed: &Piece{
				Fmt: "THEN %s", FmtKind: KindStatic, ArgStrs: []ArgStr{rsPieceArg("x", deep)}}}}),
		{name: "子字串不算命中", prot: rsProtected, rec: rsLit("PROTECTED HINT 2", KindStatic),
			why: WhyNoKey, key: "PROTECTED HINT 2"},
		{name: "開頭空白保留：多一個開頭空白不是同一鍵", prot: rsProtected, rec: rsLit(" PROTECTED HINT", KindStatic),
			why: WhyNoKey, key: " PROTECTED HINT"},
		{name: "只比對格式字串：非 Piece 的 %s 引數內容不比對（003 §9）", prot: rsProtected,
			rec: rsTpl("%s", KindStatic, rsArg("PROTECTED HINT", KindStatic)),
			why: WhyIdentity, missed: []string{"PROTECTED HINT"}},
		{name: "清單沒有命中時照常翻譯", prot: rsProtected, rec: rsLit("NEW GAME", KindStatic),
			ok: true, zh: "新遊戲", hits: []string{"NEW GAME"}},
	})
}

// 未支援規格與尾端 %：badformat（003 §7.1）。
func TestResolveBadFormat(t *testing.T) {
	formats := []string{"50%", "%x", "%05d", "%2$s", "%*d", "%hd", "%e", "%+d", "%5%", "%-%", "%ls", "%"}
	var cases []rsCase
	for _, f := range formats {
		for _, k := range []Kind{KindStatic, KindBuffer, KindOther} {
			cases = append(cases, rsCase{name: "事件 " + f + " " + k.String(), rec: rsTpl(f, k), why: WhyBadFormat})
		}
		cases = append(cases, rsCase{name: "組句 " + f,
			rec: EventRecord{Format: "%s", FmtKind: KindStatic, Composed: &Piece{Fmt: f, FmtKind: KindStatic}},
			why: WhyBadFormat})
	}
	rsRun(t, cases)
}

// badarg：%s 引數含非 20h 至 7Eh、字組不足、資料引數帶置中標記（001 §5、003 §5.2）。
func TestResolveBadArg(t *testing.T) {
	var cases []rsCase
	for _, b := range []string{"\x00", "\x01", "\x1f", "\x7f", "\x80", "\xff", "\t", "\n"} {
		cases = append(cases, rsCase{name: fmt.Sprintf("引數含 %q", b),
			rec: rsTpl("LEAVING %s", KindStatic, rsArg("AB"+b, KindStatic)), why: WhyBadArg})
	}
	cases = append(cases,
		rsCase{name: "邊界 20h 與 7Eh 合法",
			rec: rsTpl("LEAVING %s", KindStatic, rsArg(" ~", KindOther)),
			ok:  true, zh: "離開  ~", hits: []string{"LEAVING %s"}},
		rsCase{name: "第二個引數含控制字元：整個事件不翻譯",
			rec: rsTpl("%s VS %s", KindStatic, rsArg("GOBLIN", KindMonster), rsArg("A\x02", KindStatic)),
			why: WhyBadArg},
		rsCase{name: "恆等模板的引數含控制字元",
			rec: rsTpl("%s", KindStatic, rsArg("WAR\x03", KindStatic)), why: WhyBadArg},
		rsCase{name: "字組不足：7 個 %ld 要 14 個字組，只有 12 個",
			rec: rsTpl("%ld%ld%ld%ld%ld%ld%ld", KindStatic), why: WhyBadArg},
		rsCase{name: "字組不足：13 個 %d（含英文字面的 catalog 模板）",
			ui:  rsUI("N "+strings.Repeat("%d", 13), "數 "+strings.Repeat("%d", 13)),
			rec: rsTpl("N "+strings.Repeat("%d", 13), KindStatic), why: WhyBadArg},
		rsCase{name: "%s 引數少於 %s 轉換（防禦：不得 panic）",
			rec: rsTpl("LEAVING %s", KindStatic), why: WhyBadArg},
		rsCase{name: "%c 的位元組不是 ASCII 無法排進譯文",
			rec: rsWith(rsTpl("MARK %c ON", KindStatic), 0x80), why: WhyBadArg},
	)
	rsRun(t, cases)
}

// %c 為 0 時原版輸出含 NUL，顯示截到第一個 NUL（003 §5.2「引數讀取」）。
// 規格寫在 Resolve 的引數讀取；實作的 FormatTarget 把「截斷由呼叫端負責」，Resolve 沒有截斷（見回報的發現）。
func TestResolveCharNULTruncatesAtFirstNUL(t *testing.T) {
	rsRun(t, []rsCase{
		{name: "%c 為 0：譯文顯示截到第一個 NUL",
			rec: rsWith(rsTpl("MARK %c ON", KindStatic), 0),
			ok:  true, zh: "標記 ", hits: []string{"MARK %c ON"}},
	})
}

// 寬度函式由 Resolver.Wide 提供：%s 精度對譯文以 2P h 計（003 §7.2），不看碼點範圍。
func TestResolveUsesResolverWide(t *testing.T) {
	rsRun(t, []rsCase{
		{name: "U+2026 在字型是全形：2 h 預算只放得下它",
			rec: rsTpl("FOE %.1s", KindStatic, rsArg("DRAGON", KindStatic)),
			ok:  true, zh: "敵 …", hits: []string{"FOE %.1s", "DRAGON"}},
		{name: "U+2026 在字型是半形：2 h 預算放得下 … 與 A",
			wide: rsEllipsisNarrow,
			rec:  rsTpl("FOE %.1s", KindStatic, rsArg("DRAGON", KindStatic)),
			ok:   true, zh: "敵 …A", hits: []string{"FOE %.1s", "DRAGON"}},
	})
}

// 規格 003 規則補充第 3 條：恆等模板「不論 tpl 來自 catalog 或後備」。catalog 內有譯文等於鍵的冗餘鍵時，
// 判準仍是「解析後的 tpl 與格式字串相等且沒有任何引數被替換」。
// 這三個測試依規格寫期望；實作只在後備分支設 identity（resolve.go 的 identity 變數），預期失敗，見回報的發現。
func TestResolveRedundantIdentityKeyStillIdentity(t *testing.T) {
	rsRun(t, []rsCase{
		{name: "ui 有冗餘鍵 %s -> %s，引數沒被替換：identity",
			ui:  rsUI("%s", "%s"),
			rec: rsTpl("%s", KindStatic, rsArg("BOB", KindOther)),
			why: WhyIdentity},
		{name: "ui 有冗餘鍵 %c -> %c，沒有 %s 轉換：passthrough",
			ui:  rsUI("%c", "%c"),
			rec: rsWith(rsTpl("%c", KindStatic), '+'),
			why: WhyPassthrough},
		{name: "ui 有冗餘鍵 %s -> %s：引數被替換仍 OK",
			ui:  rsUI("%s", "%s"),
			rec: rsTpl("%s", KindStatic, rsArg("GOBLIN", KindMonster)),
			ok:  true, zh: "哥布林", hits: []string{"%s", "GOBLIN"}},
	})
}

func TestResolveRedundantIdentityKeyScrollCenter(t *testing.T) {
	rsRun(t, []rsCase{
		{name: "ui 有冗餘鍵 %s -> %s：卷軸標題仍置中（tpl 恆等且單一 %s）",
			ui:  rsUI("%s", "%s"),
			rec: rsTpl("%s", KindStatic, rsArg("  SCROLL TITLE  ", KindBuffer)),
			ok:  true, zh: "卷軸標題", center: true, hits: []string{"%s", rsDigScroll}},
	})
}
