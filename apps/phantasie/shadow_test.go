package phantasie

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// 頁面影子與畫面操作的單元測試（docs/spec/002 §4、§5、§8 第 1 項）。
// 期望值全部是依規格推導的字面值，不用被測函式的輸出當期望。
// 測試用字型沒有任何原版素材：本檔的疊字只看位置、狀態與透明格，不呼叫 Draw。

// shWide 是測試字型的全形判斷：CJK 碼點寬 2 h，其餘 1 h。
func shWide(r rune) bool { return r >= 0x2E80 }

// shNew 建立只有 zh-TW 一個通道的疊字核心。pairs 是「英文鍵、譯文」交錯的 ui 條目。
func shNew(t *testing.T, pairs ...string) *Overlay {
	t.Helper()
	ui := map[string]string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		ui[pairs[i]] = pairs[i+1]
	}
	o := NewOverlay()
	o.AddLanguage(&Language{
		Name:    "zh-TW",
		Cat:     NewCatalog(ui, nil, nil),
		Font:    &xlate.Font{Name: "shfont", W: 16, H: 16, Glyphs: map[rune][]byte{}},
		Wide:    shWide,
		Enabled: true,
	})
	if o.ShadowLang() != "zh-TW" {
		t.Fatalf("shadowLang = %q，要 zh-TW", o.ShadowLang())
	}
	return o
}

// shDraw 以 Begin、End 提交一個字面事件（格式字串就是文字本身，靜態字串），回事件編號。
func shDraw(t *testing.T, o *Overlay, col, row int, text string) string {
	t.Helper()
	rec := &EventRecord{Col: col, Row: row, Text: text, Format: text, FmtKind: KindStatic, Cells: []byte(text)}
	o.Begin(rec, false)
	o.End()
	if o.records[rec.ID] == nil {
		t.Fatalf("事件 %s（%q 在 %d,%d）沒有提交疊字", rec.ID, text, col, row)
	}
	return rec.ID
}

func shStateLetter(s xlate.State) string {
	switch s {
	case xlate.Pending:
		return "P"
	case xlate.Shown:
		return "S"
	}
	return "-"
}

// shView 把一筆疊字寫成一行：鍵、位置、格數乘格寬、文字、狀態，有透明格時附 T=位元串。
func shView(s *xlate.Stamp) string {
	tr := ""
	for i := 0; i < s.Cells; i++ {
		if i < len(s.Transparent) && s.Transparent[i] {
			tr += "1"
		} else {
			tr += "0"
		}
	}
	out := fmt.Sprintf("%s X%d Y%d %dx%d [%s] %s", s.Key, s.X, s.Y, s.Cells, s.CellW, string(s.Text), shStateLetter(s.State))
	if strings.Contains(tr, "1") {
		out += " T=" + tr
	}
	return out
}

func shViews(o *Overlay) []string {
	var out []string
	for _, s := range o.Layer.Stamps {
		out = append(out, shView(s))
	}
	return out
}

func shEq(t *testing.T, what string, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("%s\n got:\n  %s\nwant:\n  %s", what, strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// shStates 依 Layer 內各事件組第一次出現的順序，回每組各疊字的狀態，例如 "g1=PP g2=S"。
func shStates(o *Overlay) string {
	var order []string
	letters := map[string]string{}
	for _, s := range o.Layer.Stamps {
		if _, ok := letters[s.Key]; !ok {
			order = append(order, s.Key)
		}
		letters[s.Key] += shStateLetter(s.State)
	}
	parts := make([]string, len(order))
	for i, k := range order {
		parts[i] = k + "=" + letters[k]
	}
	return strings.Join(parts, " ")
}

// shKeyY 回每筆疊字的「鍵:Y」，依 Layer 內順序。
func shKeyY(o *Overlay) []string {
	var out []string
	for _, s := range o.Layer.Stamps {
		out = append(out, fmt.Sprintf("%s:%d", s.Key, s.Y))
	}
	return out
}

// shDesc 把影子寫成一行一筆：事件編號、位移、Hidden。
func shDesc(s Shadow) []string {
	var out []string
	for _, e := range s {
		out = append(out, fmt.Sprintf("%s dy=%d hid=%v", e.ID, e.DY, e.Hidden))
	}
	return out
}

// shFrame 以全 0 的畫面呼叫一次 Frame（疊字轉 Shown；沒有設字型時 recolor 不動作）。
func shFrame(o *Overlay) {
	o.Frame(make([]uint8, screenW*screenH), make([]uint8, screenW*screenH*3))
}

// shInt10 以 (AH, AL, 視窗列欄) 呼叫 OnInt10：CX = 上緣列、左緣欄，DX = 下緣列、右緣欄。
func shInt10(o *Overlay, ah, al byte, top, left, bottom, right int) {
	o.OnInt10(uint16(ah)<<8|uint16(al), 0, uint16(top)<<8|uint16(left), uint16(bottom)<<8|uint16(right))
}

// ---- known 與 empties（§4） ----

func TestShadowKnownEvictsLeastRecentlyUsed(t *testing.T) {
	st := newShadowStore()
	has := func(h uint64) bool { _, ok := st.known[h]; return ok }
	for h := uint64(1); h <= 32; h++ {
		st.setKnown(h, Shadow{{ID: fmt.Sprintf("g%d", h)}})
	}
	if len(st.known) != 32 {
		t.Fatalf("容量 32：放入 32 筆後有 %d 筆", len(st.known))
	}
	// get 算用到：讀過 1 之後，第 33 筆進來時被淘汰的是 2，不是 1。
	s, state := st.get(1)
	if state != shadowKnown || len(s) != 1 || s[0].ID != "g1" {
		t.Fatalf("get(1) = %v, %v", s, state)
	}
	st.setKnown(33, Shadow{{ID: "g33"}})
	if !has(1) || has(2) || !has(33) || len(st.known) != 32 {
		t.Errorf("get 之後放第 33 筆：has(1)=%v has(2)=%v has(33)=%v 筆數 %d，要 true false true 32", has(1), has(2), has(33), len(st.known))
	}
	// set 算用到：重新 set 3 之後，第 34 筆進來時被淘汰的是 4，不是 3。
	st.setKnown(3, Shadow{{ID: "g3b"}})
	if len(st.known) != 32 {
		t.Fatalf("覆寫既有的鍵不該改變筆數，現在 %d", len(st.known))
	}
	st.setKnown(34, Shadow{{ID: "g34"}})
	if !has(3) || has(4) || !has(34) || len(st.known) != 32 {
		t.Errorf("set 之後放第 34 筆：has(3)=%v has(4)=%v has(34)=%v 筆數 %d，要 true false true 32", has(3), has(4), has(34), len(st.known))
	}
	// 最新放入的不可被淘汰；被淘汰者查不到；覆寫的內容是新的。
	if !has(33) || !has(34) {
		t.Errorf("最新放入的 33、34 不該被淘汰")
	}
	if _, state := st.get(2); state != shadowUnknown {
		t.Errorf("被淘汰的 2 應為未知，得 %v", state)
	}
	if s, state := st.get(3); state != shadowKnown || s[0].ID != "g3b" {
		t.Errorf("get(3) = %v, %v，要 g3b", s, state)
	}
}

func TestShadowEmptiesFIFO(t *testing.T) {
	st := newShadowStore()
	for h := uint64(1); h <= 64; h++ {
		st.addEmpty(h)
	}
	if len(st.empties) != 64 || len(st.fifo) != 64 || len(st.known) != 0 {
		t.Fatalf("放入 64 筆：empties %d fifo %d known %d，要 64 64 0", len(st.empties), len(st.fifo), len(st.known))
	}
	for h := uint64(1); h <= 64; h++ {
		if s, state := st.get(h); state != shadowEmpty || len(s) != 0 {
			t.Fatalf("get(%d) = %v, %v，要空影子", h, s, state)
		}
	}
	// 容量 64：第 65 筆擠掉最先進的 1。
	st.addEmpty(65)
	if _, state := st.get(1); state != shadowUnknown {
		t.Errorf("第 65 筆之後 1 應被淘汰，得 %v", state)
	}
	if _, state := st.get(2); state != shadowEmpty {
		t.Errorf("第 65 筆之後 2 應還在，得 %v", state)
	}
	// 先進先出，不是最久沒用到：讀 2 很多次，下一筆進來時照樣先淘汰 2。
	for i := 0; i < 5; i++ {
		st.get(2)
	}
	st.addEmpty(66)
	if _, state := st.get(2); state != shadowUnknown {
		t.Errorf("讀取不該改變淘汰次序：2 應被淘汰，得 %v", state)
	}
	if _, state := st.get(3); state != shadowEmpty {
		t.Errorf("3 應還在，得 %v", state)
	}
	// 重複登記不重排：3 是現在最舊的，再登記一次它仍是最舊，下一筆進來時被淘汰。
	st.addEmpty(3)
	st.addEmpty(67)
	if _, state := st.get(3); state != shadowUnknown {
		t.Errorf("重複登記不該重排：3 應被淘汰，得 %v", state)
	}
	if _, state := st.get(4); state != shadowEmpty {
		t.Errorf("4 應還在，得 %v", state)
	}
	if len(st.empties) != 64 || len(st.fifo) != 64 || len(st.known) != 0 {
		t.Errorf("empties %d fifo %d known %d，要 64 64 0（空影子不進 known）", len(st.empties), len(st.fifo), len(st.known))
	}
}

func TestShadowKnownAndEmptyAreExclusive(t *testing.T) {
	st := newShadowStore()
	st.addEmpty(7)
	st.setKnown(7, Shadow{{ID: "g1"}})
	if _, state := st.get(7); state != shadowKnown {
		t.Errorf("set 之後 7 應是 known，得 %v", state)
	}
	if len(st.empties) != 0 || len(st.fifo) != 0 {
		t.Errorf("set 之後 empties %d fifo %d，要 0 0", len(st.empties), len(st.fifo))
	}
	st.addEmpty(7)
	if s, state := st.get(7); state != shadowEmpty || len(s) != 0 {
		t.Errorf("addEmpty 之後 7 應是空影子，得 %v %v", s, state)
	}
	if _, ok := st.known[7]; ok {
		t.Errorf("空影子不該留在 known")
	}
	st.setKnown(8, Shadow{{ID: "g2"}})
	st.reset()
	if _, state := st.get(7); state != shadowUnknown {
		t.Errorf("reset 之後 7 應是未知，得 %v", state)
	}
	if _, state := st.get(8); state != shadowUnknown {
		t.Errorf("reset 之後 8 應是未知，得 %v", state)
	}
	if len(st.known) != 0 || len(st.empties) != 0 || len(st.fifo) != 0 {
		t.Errorf("reset 之後 known %d empties %d fifo %d，要全 0", len(st.known), len(st.empties), len(st.fifo))
	}
}

// ---- save1、load1 與 row24（§4 的表） ----

func TestShadowSave1Registration(t *testing.T) {
	o := shNew(t, "ABCD", "甲乙丙丁")
	// 空層：登記空影子，不進 known。
	o.OnSave1(0xA1)
	if _, state := o.sh.get(0xA1); state != shadowEmpty {
		t.Errorf("空層 save1 之後應是空影子，得 %v", state)
	}
	if _, ok := o.sh.known[0xA1]; ok {
		t.Errorf("空影子不該進 known")
	}
	// 非空層：登記到 known。
	shDraw(t, o, 3, 3, "ABCD")
	o.OnSave1(0xA2)
	s, state := o.sh.get(0xA2)
	if state != shadowKnown {
		t.Fatalf("非空層 save1 之後應是 known，得 %v", state)
	}
	shEq(t, "known[0xA2]", shDesc(s), []string{"g1 dy=0 hid=[]"})
	// 同一內容先前是空影子：改成 known，empties 移除。
	o.OnSave1(0xA1)
	if _, state := o.sh.get(0xA1); state != shadowKnown {
		t.Errorf("同一內容之後有疊字，應改成 known，得 %v", state)
	}
	if len(o.sh.empties) != 0 {
		t.Errorf("empties 應已清空，還有 %d 筆", len(o.sh.empties))
	}
	// 反過來：known 的內容在空層重新存入，改成空影子，並從 known 移除。
	o.Layer.Clear(0, 0, 320, 200)
	o.OnSave1(0xA2)
	if _, state := o.sh.get(0xA2); state != shadowEmpty {
		t.Errorf("空層重新存入後應是空影子，得 %v", state)
	}
	if _, ok := o.sh.known[0xA2]; ok {
		t.Errorf("變成空影子的內容必須從 known 移除")
	}
	if _, state := o.sh.get(0xA1); state != shadowKnown {
		t.Errorf("其他條目不受影響：0xA1 應仍是 known，得 %v", state)
	}
}

func TestShadowLoadKnownRestoresInShadowOrder(t *testing.T) {
	o := shNew(t, "MONK", "僧侶", "ABCD", "甲乙丙丁")
	shDraw(t, o, 4, 3, "MONK")   // g1：2 個全形字加 4 個半形空白
	shDraw(t, o, 20, 10, "ABCD") // g2
	shFrame(o)
	shEq(t, "存入前（Frame 之後）", shViews(o), []string{
		"g1 X32 Y24 2x8 [僧侶] S",
		"g1 X48 Y24 4x4 [    ] S",
		"g2 X160 Y80 4x8 [甲乙丙丁] S",
	})
	o.OnSave1(0xB1)
	shDraw(t, o, 0, 0, "ABCD") // g3 不在影子內
	o.OnLoad(0xB1)
	// 還原：Clear 之後依影子順序附加；全部 Pending；影子外的 g3 消失。
	shEq(t, "還原後", shViews(o), []string{
		"g1 X32 Y24 2x8 [僧侶] P",
		"g1 X48 Y24 4x4 [    ] P",
		"g2 X160 Y80 4x8 [甲乙丙丁] P",
	})
	if !reflect.DeepEqual(o.Hits("g1"), []string{"MONK"}) || !reflect.DeepEqual(o.Hits("g2"), []string{"ABCD"}) {
		t.Errorf("還原後 hits：g1 %v g2 %v，要 [MONK] [ABCD]", o.Hits("g1"), o.Hits("g2"))
	}
	if got := o.C.Get("shadow_restore"); got != 1 {
		t.Errorf("shadow_restore = %d，要 1", got)
	}
}

func TestShadowLoadEmptyShadowClears(t *testing.T) {
	o := shNew(t, "ABCD", "甲乙丙丁")
	o.OnSave1(0xC1) // 空層：空影子
	shDraw(t, o, 3, 3, "ABCD")
	o.OnLoad(0xC1)
	if len(o.Layer.Stamps) != 0 {
		t.Errorf("命中空影子應 Clear 整層，還剩 %v", shViews(o))
	}
	if _, state := o.sh.get(0xC1); state != shadowEmpty {
		t.Errorf("命中空影子之後仍是空影子，得 %v", state)
	}
	if got := o.C.Get("shadow_restore"); got != 0 {
		t.Errorf("空影子不是還原，shadow_restore = %d", got)
	}
}

func TestShadowLoadUnknownClearsAndRegistersEmpty(t *testing.T) {
	o := shNew(t, "ABCD", "甲乙丙丁")
	shDraw(t, o, 3, 3, "ABCD")
	o.OnLoad(0xD1)
	if len(o.Layer.Stamps) != 0 {
		t.Errorf("未知內容應 Clear 整層，還剩 %v", shViews(o))
	}
	if s, state := o.sh.get(0xD1); state != shadowEmpty || len(s) != 0 {
		t.Errorf("未知內容 load 之後應登記空影子，得 %v %v", s, state)
	}
	if _, ok := o.sh.known[0xD1]; ok {
		t.Errorf("空影子不進 known")
	}
}

func TestShadowRow24MergesAfterUnknownLoad(t *testing.T) {
	o := shNew(t, "ABCD", "甲乙丙丁", "TOWN", "城鎮之名")
	shDraw(t, o, 5, 5, "ABCD") // g1：load 之前的畫面
	o.OnLoad(0xE1)             // 未知：Clear 並登記空影子
	if _, state := o.sh.get(0xE1); state != shadowEmpty {
		t.Fatalf("load 未知之後 0xE1 應是空影子，得 %v", state)
	}
	shDraw(t, o, 10, 5, "ABCD") // g2：第 5 列，不是第 24 列
	shDraw(t, o, 0, 24, "TOWN") // g3：第 24 列的城名
	o.OnRow24(0xE1, 0xE2)
	s, state := o.sh.get(0xE2)
	if state != shadowKnown {
		t.Fatalf("row24 之後 0xE2 應是 known，得 %v", state)
	}
	shEq(t, "合併出的影子只含第 24 列的城名", shDesc(s), []string{"g3 dy=0 hid=[]"})
	// 以合併出的影子還原：城名疊字回來。
	o.Layer.Clear(0, 0, 320, 200)
	o.OnLoad(0xE2)
	shEq(t, "還原", shViews(o), []string{"g3 X0 Y192 4x8 [城鎮之名] P"})
}

func TestShadowRow24ReplacesRow24Entries(t *testing.T) {
	o := shNew(t, "ABCD", "甲乙丙丁", "TOWN", "城鎮之名", "HALL", "大廳之內")
	shDraw(t, o, 5, 5, "ABCD")  // g1
	shDraw(t, o, 0, 24, "TOWN") // g2：舊的城名
	o.OnSave1(0xF1)
	shDraw(t, o, 0, 24, "HALL") // g3：完全蓋住 g2，g2 從 Layer 移除
	o.OnRow24(0xF1, 0xF2)
	s, state := o.sh.get(0xF2)
	if state != shadowKnown {
		t.Fatalf("row24 之後 0xF2 應是 known，得 %v", state)
	}
	// 舊影子去掉第 24 列的 g2，再接上 Layer 內第 24 列的 g3；第 5 列的 g1 沿用舊影子。
	shEq(t, "新影子", shDesc(s), []string{"g1 dy=0 hid=[]", "g3 dy=0 hid=[]"})
	// 舊內容的影子不變。
	old, _ := o.sh.get(0xF1)
	shEq(t, "舊影子", shDesc(old), []string{"g1 dy=0 hid=[]", "g2 dy=0 hid=[]"})
}

func TestShadowRow24UnknownOldDoesNotRegister(t *testing.T) {
	o := shNew(t, "TOWN", "城鎮之名")
	shDraw(t, o, 0, 24, "TOWN")
	o.OnRow24(0x71, 0x72)
	if _, state := o.sh.get(0x72); state != shadowUnknown {
		t.Errorf("h_old 未知時不登記 h_new，得 %v", state)
	}
	if _, state := o.sh.get(0x71); state != shadowUnknown {
		t.Errorf("h_old 仍是未知，得 %v", state)
	}
}

func TestShadowRow24EmptyResultIsEmptyShadow(t *testing.T) {
	o := shNew(t, "TOWN", "城鎮之名")
	shDraw(t, o, 0, 24, "TOWN")
	o.OnSave1(0x81) // 影子只有第 24 列的 g1
	o.Layer.Clear(0, 192, 320, 200)
	o.OnRow24(0x81, 0x82)
	if s, state := o.sh.get(0x82); state != shadowEmpty || len(s) != 0 {
		t.Errorf("去掉第 24 列又沒有新的，應登記空影子，得 %v %v", s, state)
	}
	if _, ok := o.sh.known[0x82]; ok {
		t.Errorf("空影子不進 known")
	}
}

// 有效列 = Row + DY/8：第 23 列的疊字被捲到第 24 列，row24 要把它當第 24 列的條目。
func TestShadowRow24UsesEffectiveRow(t *testing.T) {
	o := shNew(t, "TOWN", "城鎮之名")
	shDraw(t, o, 0, 23, "TOWN")        // g1，Y=184
	shInt10(o, 0x07, 1, 23, 0, 24, 39) // 視窗第 23 至 24 列，下捲 1 列：Y=192
	shEq(t, "下捲後", shKeyY(o), []string{"g1:192"})
	o.OnSave1(0x91)
	s, _ := o.sh.get(0x91)
	shEq(t, "影子帶位移", shDesc(s), []string{"g1 dy=8 hid=[]"})
	o.Layer.Clear(0, 192, 320, 200)
	o.OnRow24(0x91, 0x92)
	if _, state := o.sh.get(0x92); state != shadowEmpty {
		t.Errorf("DY=8 的條目有效列是 24，row24 應把它換掉，得 %v", state)
	}
	// effRow 本身。
	cases := []struct {
		id   string
		dy   int
		want int
		ok   bool
	}{
		{"g1", 0, 23, true},
		{"g1", 8, 24, true},
		{"g1", -8, 22, true},
		{"g1", 16, 25, true},
		{"g404", 0, 0, false},
	}
	for _, c := range cases {
		got, ok := o.effRow(ShadowEntry{ID: c.id, DY: c.dy})
		if got != c.want || ok != c.ok {
			t.Errorf("effRow(%s, DY=%d) = %d, %v，要 %d, %v", c.id, c.dy, got, ok, c.want, c.ok)
		}
	}
}

// copy12 之前最多有 58 次 save1，每次 load1 未命中都登記空影子；known 內其他條目不能被擠出。
func TestShadowCopy12FloodKeepsKnown(t *testing.T) {
	o := shNew(t, "ABCD", "甲乙丙丁")
	shDraw(t, o, 3, 3, "ABCD")
	for i := uint64(0); i < 32; i++ {
		o.OnSave1(0x1000 + i) // known 滿 32 筆，0x1000 是之後要命中的
	}
	for i := uint64(0); i < 58; i++ {
		o.OnLoad(0x2000 + i) // 未知內容：Clear 並登記空影子
	}
	for i := uint64(0); i < 58; i++ {
		o.OnSave1(0x3000 + i) // 此時層已空：登記空影子
	}
	for i := uint64(0); i < 32; i++ {
		if _, ok := o.sh.known[0x1000+i]; !ok {
			t.Errorf("known 內的 0x%X 被空影子登記擠出", 0x1000+i)
		}
	}
	if len(o.sh.empties) != 64 {
		t.Errorf("empties 有 %d 筆，116 次登記後要停在容量 64", len(o.sh.empties))
	}
	o.OnLoad(0x1000) // copy12 之後的 load2
	shEq(t, "load2 命中", shViews(o), []string{"g1 X24 Y24 4x8 [甲乙丙丁] P"})
}

// ---- 由 Layer 產生影子的 Hidden、DY ----

func TestShadowHiddenOfDirect(t *testing.T) {
	mk := func(x, cells, cw int, tr ...int) *xlate.Stamp {
		s := &xlate.Stamp{X: x, Y: 24, Cells: cells, CellW: cw, CellH: 8}
		for _, i := range tr {
			if len(s.Transparent) < cells {
				s.Transparent = make([]bool, cells)
			}
			s.Transparent[i] = true
		}
		return s
	}
	clip := &EventRecord{Col: 38, Row: 3, Text: "ABCD"} // 矩形 [304, 336) 裁切成 [304, 320)
	mid := &EventRecord{Col: 2, Row: 3, Text: "ABCDEFGH"}
	cases := []struct {
		name   string
		rec    *EventRecord
		stamps []*xlate.Stamp
		want   string
	}{
		{"沒有疊字：整個事件矩形", clip, nil, "[{304 320}]"},
		{"全覆蓋", clip, []*xlate.Stamp{mk(304, 2, 8)}, "[]"},
		{"只蓋第一格", clip, []*xlate.Stamp{mk(304, 1, 8)}, "[{312 320}]"},
		{"兩段半形相鄰合併", clip, []*xlate.Stamp{mk(304, 2, 4), mk(312, 2, 4)}, "[]"},
		{"透明格不算可見", clip, []*xlate.Stamp{mk(304, 2, 8, 0)}, "[{304 312}]"},
		{"中間缺一段與尾端缺一段", mid, []*xlate.Stamp{mk(16, 2, 8), mk(40, 4, 4)}, "[{32 40} {56 80}]"},
		{"疊序與位置無關：先列右邊的段", mid, []*xlate.Stamp{mk(40, 4, 4), mk(16, 2, 8)}, "[{32 40} {56 80}]"},
	}
	for _, c := range cases {
		got := fmt.Sprint(hiddenOf(c.rec, c.stamps))
		if got != c.want {
			t.Errorf("%s：hiddenOf = %s，要 %s", c.name, got, c.want)
		}
	}
}

func TestShadowHiddenIsComplementWithAdjacentMerge(t *testing.T) {
	o := shNew(t, "ABCDEFGH", "一二三四五六七八")
	shDraw(t, o, 2, 3, "ABCDEFGH") // g1：X16..80，單一疊字 8 格
	// 清第 2、3 格（x 32..48）。
	o.Layer.Clear(32, 24, 48, 32)
	shEq(t, "清 x32..48 之後", shDesc(o.shadowFromLayer()), []string{"g1 dy=0 hid=[{32 48}]"})
	// 再清第 6 格（x 64..72）。
	o.Layer.Clear(64, 24, 72, 32)
	shEq(t, "再清 x64..72 之後", shDesc(o.shadowFromLayer()), []string{"g1 dy=0 hid=[{32 48} {64 72}]"})
	// 再清第 4 格（x 48..56）：與前一段相鄰，合併成 [32, 56)。
	o.Layer.Clear(48, 24, 56, 32)
	shEq(t, "再清 x48..56 之後", shDesc(o.shadowFromLayer()), []string{"g1 dy=0 hid=[{32 56} {64 72}]"})
	// 還原：第 2 至 4、6 格透明。
	o.OnSave1(0x41)
	o.Layer.Clear(0, 0, 320, 200)
	o.OnLoad(0x41)
	shEq(t, "還原", shViews(o), []string{"g1 X16 Y24 8x8 [一二三四五六七八] P T=00111010"})
}

// 整段被 Layer.Clear 移除的疊字不在 Layer 內，Hidden 仍要包含它的範圍，還原時不復活。
func TestShadowHiddenIncludesStampRemovedByClear(t *testing.T) {
	o := shNew(t, "MONK", "僧侶")
	shDraw(t, o, 4, 3, "MONK") // g1：全形段 X32..48、半形段 X48..64
	o.Layer.Clear(48, 24, 64, 32)
	shEq(t, "Clear 之後", shViews(o), []string{"g1 X32 Y24 2x8 [僧侶] P"})
	shEq(t, "影子", shDesc(o.shadowFromLayer()), []string{"g1 dy=0 hid=[{48 64}]"})
	o.OnSave1(0x42)
	o.Layer.Clear(0, 0, 320, 200)
	o.OnLoad(0x42)
	shEq(t, "還原後半形段不復活", shViews(o), []string{"g1 X32 Y24 2x8 [僧侶] P"})
}

// 錨定格全部失效使全形段被 drop，同事件的半形段存活；影子的 Hidden 含被 drop 的範圍。
func TestShadowHiddenAfterAnchorsDropOneStamp(t *testing.T) {
	o := shNew(t, "MONK", "僧侶")
	shDraw(t, o, 4, 3, "MONK") // g1：全形段 X32..48（2 格），半形段 X48..64
	idx := make([]uint8, screenW*screenH)
	rgb := make([]uint8, screenW*screenH*3)
	idx[25*screenW+33] = 3 // 全形段第 0 格有墨：錨定格；第 1 格與半形段都是純色
	o.Frame(idx, rgb)
	idx[25*screenW+33] = 0 // 第 0 格內容改變，連續 3 次 Frame 之後轉透明
	for i := 0; i < 3; i++ {
		o.Frame(idx, rgb)
	}
	// 唯一的錨定格失效 = 原文已不在：全形段整筆移除，半形段留下。
	shEq(t, "錨定失效之後", shViews(o), []string{"g1 X48 Y24 4x4 [    ] S"})
	shEq(t, "影子", shDesc(o.shadowFromLayer()), []string{"g1 dy=0 hid=[{32 48}]"})
	o.OnSave1(0x43)
	o.Layer.Clear(0, 0, 320, 200)
	o.OnLoad(0x43)
	shEq(t, "還原後被 drop 的全形段不復活", shViews(o), []string{"g1 X48 Y24 4x4 [    ] P"})
}

func TestShadowScrollThenRegisterKeepsY(t *testing.T) {
	o := shNew(t, "ABCD", "甲乙丙丁")
	shDraw(t, o, 2, 5, "ABCD") // Y=40
	shInt10(o, 0x06, 1, 3, 0, 10, 39)
	shEq(t, "上捲 1 列之後", shKeyY(o), []string{"g1:32"})
	o.OnSave1(0x51)
	s, _ := o.sh.get(0x51)
	shEq(t, "影子帶位移", shDesc(s), []string{"g1 dy=-8 hid=[]"})
	o.Layer.Clear(0, 0, 320, 200)
	o.OnLoad(0x51)
	shEq(t, "還原後 Y 與捲動後一致", shViews(o), []string{"g1 X16 Y32 4x8 [甲乙丙丁] P"})
	if r, ok := o.effRow(s[0]); !ok || r != 4 {
		t.Errorf("effRow = %d, %v，要 4, true", r, ok)
	}
}

// 同事件各疊字位移不一致：不入影子，計 shadow_skip。
func TestShadowInconsistentDYIsSkipped(t *testing.T) {
	o := shNew(t, "MONK", "僧侶", "ABCD", "甲乙丙丁")
	shDraw(t, o, 4, 5, "MONK")   // g1：全形段 X32..48，半形段 X48..64，Y=40
	shDraw(t, o, 20, 12, "ABCD") // g2：視窗之外，位移一致
	// 視窗只涵蓋 x32..48：全形段上移，半形段不在水平範圍內而不動。
	shInt10(o, 0x06, 1, 3, 4, 10, 5)
	shEq(t, "部分疊字被捲動", shKeyY(o), []string{"g1:32", "g1:40", "g2:96"})
	shEq(t, "影子只剩 g2", shDesc(o.shadowFromLayer()), []string{"g2 dy=0 hid=[]"})
	if got := o.C.Get("shadow_skip"); got != 1 {
		t.Errorf("shadow_skip = %d，要 1", got)
	}
	// 只剩位移不一致的事件：影子為空，save1 登記空影子。
	o.Layer.Clear(160, 96, 192, 104)
	o.OnSave1(0x61)
	if _, state := o.sh.get(0x61); state != shadowEmpty {
		t.Errorf("只有位移不一致的事件時應登記空影子，得 %v", state)
	}
	if got := o.C.Get("shadow_skip"); got != 2 {
		t.Errorf("shadow_skip = %d，要 2", got)
	}
}

// ---- 還原：Transparent、疊序、shadowLang ----

func TestShadowRestoreKeepsTransparentAndStackOrder(t *testing.T) {
	o := shNew(t, "ABCDEFGH", "一二三四五六七八", "ABCD", "甲乙丙丁")
	shDraw(t, o, 2, 3, "ABCDEFGH") // g1：X16..80
	shDraw(t, o, 4, 3, "ABCD")     // g2：X32..64，蓋在 g1 之上；g1 的第 2 至 5 格轉透明
	g1 := "g1 X16 Y24 8x8 [一二三四五六七八] P T=00111100"
	g2 := "g2 X32 Y24 4x8 [甲乙丙丁] P"
	shEq(t, "提交之後", shViews(o), []string{g1, g2})
	o.OnSave1(0x11)
	shEq(t, "影子", shDesc(o.shadowFromLayer()), []string{"g1 dy=0 hid=[{32 64}]", "g2 dy=0 hid=[]"})
	o.Layer.Clear(0, 0, 320, 200)
	o.OnLoad(0x11)
	shEq(t, "還原：較晚的事件仍在較早的事件之上，透明格照舊", shViews(o), []string{g1, g2})

	// 疊序以影子為準：把 Layer 內的順序換成 g2 在下、g1 在上，還原後仍是這個順序，
	// 而且不經 Layer.Add 的覆蓋判斷（g1 的矩形涵蓋 g2，Add 會把 g2 整筆移除）。
	o.Layer.Stamps = []*xlate.Stamp{o.Layer.Stamps[1], o.Layer.Stamps[0]}
	o.OnSave1(0x12)
	s, _ := o.sh.get(0x12)
	shEq(t, "換序後的影子", shDesc(s), []string{"g2 dy=0 hid=[]", "g1 dy=0 hid=[{32 64}]"})
	o.Layer.Clear(0, 0, 320, 200)
	o.OnLoad(0x12)
	shEq(t, "還原後保留影子的疊序", shViews(o), []string{g2, g1})
}

func TestShadowRestoreSkipsLostEntries(t *testing.T) {
	o := shNew(t, "ABCD", "甲乙丙丁")
	shDraw(t, o, 3, 3, "ABCD") // g1
	// g2 的記錄存在，但 catalog 沒有它的鍵：Resolve 失敗。
	o.records["g2"] = &EventRecord{ID: "g2", Col: 1, Row: 1, Text: "NOKEY", Format: "NOKEY", FmtKind: KindStatic, Cells: []byte("NOKEY")}
	o.restoreShadow(Shadow{{ID: "g99"}, {ID: "g1"}, {ID: "g2"}})
	shEq(t, "只還原有效的條目", shViews(o), []string{"g1 X24 Y24 4x8 [甲乙丙丁] P"})
	if got := o.C.Get("shadow_lost"); got != 2 {
		t.Errorf("shadow_lost = %d，要 2（記錄不存在與 Resolve 失敗各一）", got)
	}
}

// 影子與語言無關，還原用 shadowLang，不是顯示語言。
func TestShadowRestoreUsesShadowLangNotDisplay(t *testing.T) {
	ui := func(zh string) *Catalog { return NewCatalog(map[string]string{"MONK": zh}, nil, nil) }
	o := NewOverlay()
	o.AddLanguage(&Language{Name: "zh-TW", Cat: ui("僧侶"), Font: &xlate.Font{Name: "shfont-tw", W: 16, H: 16}, Wide: shWide, Enabled: true})
	o.AddLanguage(&Language{Name: "zh-CN", Cat: ui("僧侣"), Font: &xlate.Font{Name: "shfont-cn", W: 16, H: 16}, Wide: shWide, Enabled: true})
	o.AddLanguage(&Language{Name: "en"}) // 原版英文：沒有 catalog，不啟用
	shDraw(t, o, 4, 3, "MONK")
	o.OnSave1(0x21)

	if err := o.SetDisplay("en"); err != nil {
		t.Fatal(err)
	}
	if o.ShadowLang() != "zh-TW" || o.Display() != "en" {
		t.Fatalf("顯示 en 時 shadowLang 應維持 zh-TW，得 display=%q shadowLang=%q", o.Display(), o.ShadowLang())
	}
	o.OnLoad(0x99) // 未知：Clear
	o.OnLoad(0x21)
	shEq(t, "顯示語言為 en 仍能還原（以 shadowLang）", shViews(o), []string{
		"g1 X32 Y24 2x8 [僧侶] P",
		"g1 X48 Y24 4x4 [    ] P",
	})

	// 切到 zh-CN：同一份影子以新的語言還原，不需要重建影子。
	if err := o.SetDisplay("zh-CN"); err != nil {
		t.Fatal(err)
	}
	o.OnLoad(0x99)
	o.OnLoad(0x21)
	shEq(t, "切換語言後同一份影子還原成新語言", shViews(o), []string{
		"g1 X32 Y24 2x8 [僧侣] P",
		"g1 X48 Y24 4x4 [    ] P",
	})
	if name := o.Layer.Stamps[0].Font.Name; name != "shfont-cn" {
		t.Errorf("還原的疊字字型 = %q，要 shfont-cn", name)
	}
}

// ---- OnInt10（§5） ----

func TestInt10SetModeClearsLayerAndShadows(t *testing.T) {
	o := shNew(t, "ABCD", "甲乙丙丁")
	shDraw(t, o, 3, 3, "ABCD")
	o.OnSave1(0xAA1) // known
	o.OnLoad(0xAA3)  // 未知：Clear 並登記空影子
	shDraw(t, o, 4, 4, "ABCD")
	o.OnInt10(0x0004, 0, 0, 0) // AH=00h AL=04h
	if len(o.Layer.Stamps) != 0 {
		t.Errorf("設定視訊模式應清空疊字層，還剩 %v", shViews(o))
	}
	if _, state := o.sh.get(0xAA1); state != shadowUnknown {
		t.Errorf("known 應清空，得 %v", state)
	}
	if _, state := o.sh.get(0xAA3); state != shadowUnknown {
		t.Errorf("empties 應清空，得 %v", state)
	}
	if got := o.C.Get("op_int10_00"); got != 1 {
		t.Errorf("op_int10_00 = %d，要 1", got)
	}
}

// shWindowFixture 在第 3 至 6 列的視窗內外放幾筆疊字（都是單一全形段、單格 8 像素）。
func shWindowFixture(t *testing.T) *Overlay {
	o := shNew(t, "ABCD", "甲乙丙丁")
	shDraw(t, o, 1, 3, "ABCD") // g1：Y24
	shDraw(t, o, 1, 4, "ABCD") // g2：Y32
	shDraw(t, o, 1, 6, "ABCD") // g3：Y48
	shDraw(t, o, 1, 2, "ABCD") // g4：Y16，視窗之上
	shDraw(t, o, 1, 8, "ABCD") // g5：Y64，視窗之下
	return o
}

func TestInt10WindowClearRect(t *testing.T) {
	for _, ah := range []byte{0x06, 0x07} {
		o := shNew(t, "ABCD", "甲乙丙丁", "ABCDEFGH", "一二三四五六七八")
		shDraw(t, o, 2, 2, "ABCD")     // g1：視窗之上
		shDraw(t, o, 3, 4, "ABCD")     // g2：X24..56 Y32，整筆在視窗內
		shDraw(t, o, 0, 5, "ABCDEFGH") // g3：X0..64 Y40，與視窗只部分相交
		shDraw(t, o, 3, 6, "ABCD")     // g4：Y48，貼著視窗下緣之外
		shDraw(t, o, 10, 4, "ABCD")    // g5：X80，貼著視窗右緣之外
		shFrame(o)
		// 視窗：第 3 至 5 列、第 2 至 9 欄，AL=0：清除矩形 [16,24)-[80,48)。
		shInt10(o, ah, 0, 3, 2, 5, 9)
		shEq(t, fmt.Sprintf("AH=%02Xh AL=0", ah), shViews(o), []string{
			"g1 X16 Y16 4x8 [甲乙丙丁] S",
			"g3 X0 Y40 8x8 [一二三四五六七八] P T=00111111",
			"g4 X24 Y48 4x8 [甲乙丙丁] S",
			"g5 X80 Y32 4x8 [甲乙丙丁] S",
		})
	}
}

func TestInt10WindowClampsToScreen(t *testing.T) {
	o := shNew(t, "ABCD", "甲乙丙丁")
	shDraw(t, o, 32, 21, "ABCD") // g1：X256..288 Y168，在夾邊後的視窗內
	shDraw(t, o, 10, 21, "ABCD") // g2：X80，視窗之左
	shDraw(t, o, 32, 19, "ABCD") // g3：Y152..160，視窗之上
	shDraw(t, o, 36, 24, "ABCD") // g4：X288..320 Y192..200，在視窗內且貼著畫面右下角
	// 下緣 40、右緣 79 夾到列 24、欄 39：清除 [240,160)-[320,200)。
	shInt10(o, 0x06, 0, 20, 30, 40, 79)
	shEq(t, "夾邊後清除", shKeyY(o), []string{"g2:168", "g3:152"})
}

func TestInt10ScrollSemantics(t *testing.T) {
	cases := []struct {
		name string
		ah   byte
		al   byte
		want []string
	}{
		// 視窗第 3 至 6 列（Y 24..56，4 列）。g1 Y24、g2 Y32、g3 Y48 在內，g4 Y16、g5 Y64 在外。
		{"上捲 1 列：g1 捲出視窗上緣被移除", 0x06, 1, []string{"g2:24", "g3:40", "g4:16", "g5:64"}},
		{"下捲 1 列：g3 捲出視窗下緣被移除", 0x07, 1, []string{"g1:32", "g2:40", "g4:16", "g5:64"}},
		{"上捲 2 列", 0x06, 2, []string{"g3:32", "g4:16", "g5:64"}},
		{"下捲 3 列：只有 g1 留在視窗內", 0x07, 3, []string{"g1:48", "g4:16", "g5:64"}},
		{"AL 等於視窗列數視同清除", 0x06, 4, []string{"g4:16", "g5:64"}},
		{"AL 大於視窗列數視同清除", 0x07, 9, []string{"g4:16", "g5:64"}},
		{"AL=0 上捲是清除", 0x06, 0, []string{"g4:16", "g5:64"}},
		{"AL=0 下捲是清除", 0x07, 0, []string{"g4:16", "g5:64"}},
	}
	for _, c := range cases {
		o := shWindowFixture(t)
		shInt10(o, c.ah, c.al, 3, 0, 6, 39)
		shEq(t, c.name, shKeyY(o), c.want)
	}
}

// Layer.Scroll 只移動整筆落在視窗水平範圍內的疊字，橫向跨出視窗的不處理。
func TestInt10ScrollOnlyMovesStampsInsideWindowHorizontally(t *testing.T) {
	o := shNew(t, "ABCD", "甲乙丙丁", "ABCDEFGH", "一二三四五六七八")
	shDraw(t, o, 2, 4, "ABCD")     // g1：X16..48，在視窗 x16..80 內
	shDraw(t, o, 0, 5, "ABCDEFGH") // g2：X0..64，左緣在視窗之外
	shDraw(t, o, 9, 4, "ABCD")     // g3：X72..104，右緣在視窗之外
	shInt10(o, 0x06, 1, 3, 2, 6, 9)
	shEq(t, "上捲 1 列", shKeyY(o), []string{"g1:24", "g2:40", "g3:32"})
}

func TestInt10ClampBeforeRowCount(t *testing.T) {
	// 下緣 60 夾到 24，視窗只剩第 22 至 24 列（3 列）：AL=3 視同清除，AL=2 才是捲動。
	build := func() *Overlay {
		o := shNew(t, "ABCDEFGH", "一二三四五六七八")
		shDraw(t, o, 0, 23, "ABCDEFGH") // X0..64 Y184：左緣在視窗（x16 起）之外
		return o
	}
	o := build()
	shInt10(o, 0x06, 3, 22, 2, 60, 79)
	shEq(t, "AL=3 清除，相交的格轉透明", shViews(o), []string{"g1 X0 Y184 8x8 [一二三四五六七八] P T=00111111"})
	o = build()
	shInt10(o, 0x06, 2, 22, 2, 60, 79)
	shEq(t, "AL=2 捲動，跨出視窗左緣的疊字不動也不透明", shViews(o), []string{"g1 X0 Y184 8x8 [一二三四五六七八] P"})
}

func TestInt10WindowNoOps(t *testing.T) {
	cases := []struct {
		name                     string
		ah, al                   byte
		top, left, bottom, right int
	}{
		{"上緣大於下緣（06h）", 0x06, 0, 5, 2, 3, 9},
		{"上緣大於下緣（07h）", 0x07, 1, 5, 2, 3, 9},
		{"左緣大於右緣", 0x06, 0, 3, 9, 5, 2},
		{"夾邊後視窗不存在（列）", 0x06, 0, 30, 2, 30, 9},
		{"夾邊後視窗不存在（欄）", 0x07, 0, 3, 45, 5, 50},
	}
	for _, c := range cases {
		o := shNew(t, "ABCD", "甲乙丙丁")
		shDraw(t, o, 3, 4, "ABCD") // g1：若矩形被交換或不夾邊，它會落在清除範圍內
		shFrame(o)
		before := shViews(o)
		shInt10(o, c.ah, c.al, c.top, c.left, c.bottom, c.right)
		shEq(t, c.name+"：疊字層不動作", shViews(o), before)
	}
}

func TestInt10PaletteSelectMarksAllPending(t *testing.T) {
	o := shNew(t, "ABCD", "甲乙丙丁", "MONK", "僧侶")
	shDraw(t, o, 3, 4, "ABCD")
	shDraw(t, o, 4, 24, "MONK")
	shFrame(o)
	if got := shStates(o); got != "g1=S g2=SS" {
		t.Fatalf("Frame 之後 %q，要 g1=S g2=SS", got)
	}
	before := shKeyY(o)
	o.OnInt10(0x0B00, 0x0100, 0, 0) // AH=0Bh BH=1
	if got := shStates(o); got != "g1=P g2=PP" {
		t.Errorf("AH=0Bh 之後 %q，要所有疊字 Pending", got)
	}
	shEq(t, "AH=0Bh 不改位置也不移除", shKeyY(o), before)
	if got := o.C.Get("op_int10_0B"); got != 1 {
		t.Errorf("op_int10_0B = %d，要 1", got)
	}
}

func TestInt10OtherFunctionsDoNothing(t *testing.T) {
	o := shNew(t, "ABCD", "甲乙丙丁")
	shDraw(t, o, 3, 4, "ABCD")
	shFrame(o)
	before := shViews(o)
	for _, ah := range []byte{0x01, 0x02, 0x03, 0x05, 0x08, 0x09, 0x0C, 0x0E, 0x0F} {
		shInt10(o, ah, 1, 0, 0, 24, 39)
		shEq(t, fmt.Sprintf("AH=%02Xh 不動作", ah), shViews(o), before)
	}
}

// ---- OnInvert（§5） ----

func TestInvertWholeGroupBecomesPending(t *testing.T) {
	o := shNew(t, "MONK", "僧侶", "ABCD", "甲乙丙丁")
	shDraw(t, o, 4, 3, "MONK")  // g1：全形段 X32..48，半形段 X48..64，Y24
	shDraw(t, o, 10, 3, "ABCD") // g2：X80..112 Y24
	shDraw(t, o, 4, 4, "ABCD")  // g3：X32..64 Y32，在 g1 的下一列
	shDraw(t, o, 20, 3, "ABCD") // g4：X160 Y24
	shFrame(o)
	if got := shStates(o); got != "g1=SS g2=S g3=S g4=S" {
		t.Fatalf("Frame 之後 %q", got)
	}
	// 矩形 x32..40 y24..32 只與 g1 的全形段相交；整組（含沒相交的半形段）都改 Pending。
	o.OnInvert(3, 4, 1, false)
	if got := shStates(o); got != "g1=PP g2=S g3=S g4=S" {
		t.Errorf("invert 之後 %q，要 g1=PP g2=S g3=S g4=S", got)
	}
	if o.C.Get("op_invert") != 1 || o.C.Get("dim") != 0 || o.C.Get("op_invert2") != 0 {
		t.Errorf("invert 計數：op_invert=%d dim=%d op_invert2=%d，要 1 0 0", o.C.Get("op_invert"), o.C.Get("dim"), o.C.Get("op_invert2"))
	}
	shFrame(o)

	// 不相交：矩形貼著 g2 右緣、在 g1 上一列、在 g3 與 g1 之間的下一列之外。
	o.OnInvert(3, 14, 2, false) // x112..128 y24..32：g2 右緣是 112（半開），不相交
	o.OnInvert(2, 4, 4, false)  // y16..24：在所有疊字之上
	o.OnInvert(5, 4, 4, false)  // y40..48：在所有疊字之下
	if got := shStates(o); got != "g1=SS g2=S g3=S g4=S" {
		t.Errorf("不相交的 invert 之後 %q，要全部維持 Shown", got)
	}
	// 差一格就相交：x104..112 與 g2 相交。
	o.OnInvert(3, 13, 1, false)
	if got := shStates(o); got != "g1=SS g2=P g3=S g4=S" {
		t.Errorf("與 g2 相交的 invert 之後 %q，要只有 g2 Pending", got)
	}
	shFrame(o)

	// invert2：同樣改 Pending，另計 dim。
	o.OnInvert(4, 4, 1, true) // y32..40：g3
	if got := shStates(o); got != "g1=SS g2=S g3=P g4=S" {
		t.Errorf("invert2 之後 %q，要只有 g3 Pending", got)
	}
	if o.C.Get("op_invert2") != 1 || o.C.Get("dim") != 1 {
		t.Errorf("invert2 計數：op_invert2=%d dim=%d，要 1 1", o.C.Get("op_invert2"), o.C.Get("dim"))
	}
}

// 反白、捲動、反白、繪字、捲動、反白：清單選單的操作序列（002 §2、§8 第 1 項）。
func TestInvertScrollInvertDrawSequence(t *testing.T) {
	o := shNew(t,
		"AAAA", "甲乙丙丁", "BBBB", "戊己庚辛", "CCCC", "壬癸子丑",
		"DDDD", "寅卯辰巳", "EEEE", "午未申酉", "FFFF", "戌亥天地")
	for i, text := range []string{"AAAA", "BBBB", "CCCC", "DDDD", "EEEE"} {
		shDraw(t, o, 2, 5+i, text) // g1..g5：第 5 至 9 列
	}
	shFrame(o)
	step := func(name, wantY, wantState string) {
		t.Helper()
		shEq(t, name+"：位置", shKeyY(o), strings.Fields(wantY))
		if got := shStates(o); got != wantState {
			t.Errorf("%s：狀態 %q，要 %q", name, got, wantState)
		}
	}
	step("初始", "g1:40 g2:48 g3:56 g4:64 g5:72", "g1=S g2=S g3=S g4=S g5=S")

	o.OnInvert(6, 2, 4, false) // 反白第 6 列的選項 g2
	step("反白", "g1:40 g2:48 g3:56 g4:64 g5:72", "g1=S g2=P g3=S g4=S g5=S")
	shFrame(o)

	shInt10(o, 0x06, 1, 5, 2, 9, 17) // 清單上捲 1 列：g1 捲出視窗上緣
	step("AH=06h AL=1", "g2:40 g3:48 g4:56 g5:64", "g2=S g3=S g4=S g5=S")

	o.OnInvert(5, 2, 4, false) // 反白移到第 5 列：g2
	step("反白", "g2:40 g3:48 g4:56 g5:64", "g2=P g3=S g4=S g5=S")

	shDraw(t, o, 2, 9, "FFFF") // 繪新列 g6
	step("繪字", "g2:40 g3:48 g4:56 g5:64 g6:72", "g2=P g3=S g4=S g5=S g6=P")
	shFrame(o)
	step("Frame", "g2:40 g3:48 g4:56 g5:64 g6:72", "g2=S g3=S g4=S g5=S g6=S")

	shInt10(o, 0x07, 1, 5, 2, 9, 17) // 清單下捲 1 列：g6 捲出視窗下緣
	step("AH=07h AL=1", "g2:48 g3:56 g4:64 g5:72", "g2=S g3=S g4=S g5=S")

	o.OnInvert(6, 2, 4, false) // 反白移到第 6 列：g2
	step("反白", "g2:48 g3:56 g4:64 g5:72", "g2=P g3=S g4=S g5=S")
}
