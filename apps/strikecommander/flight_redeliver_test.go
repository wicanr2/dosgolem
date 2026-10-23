package strikecommander

import (
	"reflect"
	"strings"
	"testing"
)

// strike_commander 規格 009 第 6 版 §4.1 第 19 項：入口重看。合成 View 另加 Deliveries()（包住既有 fv，自帶計數）。
// 期望值在測試內獨立寫出。

// dview 是有送達計數的合成 FlightView。
type dview struct {
	*fv
	d uint64
}

func (v *dview) Deliveries() uint64 { return v.d }

// rw 是以 dview（或不含 Deliveries 的 fv）餵觀察器的合成世界。每次 obs 是一個觀察點。
type rw struct {
	*world
	dv   *dview
	view FlightView
}

func newRW(t *testing.T, withDeliveries bool) *rw {
	w := newWorld(t, 0x0241)
	r := &rw{world: w, dv: &dview{fv: w.v}}
	r.view = r.dv
	if !withDeliveries {
		r.view = w.v
	}
	return r
}

func (r *rw) obs(cs, ip uint16) {
	r.v.steps += 100
	r.v.cs, r.v.ip = cs, ip
	r.o.Observe(r.view)
}

// other 是別的 PC 的觀察點（例：IRQ 處理常式 0018:4BDE）。
func (r *rw) other() { r.obs(0x0018, 0x4BDE) }

// deliver 是一次送達之後的觀察點（計數先加 1）。
func (r *rw) deliver() {
	r.dv.d++
	r.other()
}

// entry 是 hud 字串 s 在 (x, y) 的 puts 入口；callerBad 時 printf 的返回位址改成 +1462:0259（呼叫端不符）。
func (r *rw) entry(s string, x, y int, callerBad bool) {
	v := r.v
	for i := 0; i < len(s); i++ {
		v.w8(qDS, qStr+uint16(i), int(s[i]))
	}
	v.w8(qDS, qStr+uint16(len(s)), 0)
	v.w16(tSS, qSP+4, tObjHUD)
	v.w16(tSS, qSP+6, qStr)
	v.w16(tSS, qFrameP, qQ)
	v.w16(tSS, qFrameP+2, ipFFmtRet)
	v.w16(tSS, qFrameP+4, r.base+segFText)
	ret := uint16(ipFHUDRet)
	if callerBad {
		ret = 0x0259
	}
	v.w16(tSS, qQ+2, ret)
	v.w16(tSS, qQ+4, r.base+segFHUD)
	v.w16(qDS, tObjHUD+objX, uint16(x))
	v.w16(qDS, tObjHUD+objY, uint16(y))
	v.ss, v.sp, v.bp, v.ds = tSS, qSP, qFrameP, qDS
	r.obs(r.base+segFText, ipFPuts)
}

// putc 是第 i 字的 putc 觀察點（前進寬 4）；obj 是 [SS:SP+4]。
func (r *rw) putc(s string, i, x, y int, obj uint16) {
	v := r.v
	v.w16(qDS, tObjHUD+objX, uint16(x+4*i))
	v.w16(qDS, tObjHUD+objY, uint16(y))
	v.sp = qSP - 0x10
	v.w16(tSS, qSP-0x10+4, obj)
	v.w16(tSS, qSP-0x10+6, uint16(s[i]))
	r.obs(r.base+segFText, ipFPutc)
}

// next 是 putc 入口之後的下一個指令 02AD（入口真的執行了）。
func (r *rw) next() { r.obs(r.base+segFText, ipFPutc+1) }

// ink 畫第 i 字的墨跡（hud 方塊：3 欄、Y+1..Y+4）。
func (r *rw) ink(s string, i, x, y int) {
	if s[i] == ' ' {
		return
	}
	for dx := 0; dx < 3; dx++ {
		for dy := 1; dy <= 4; dy++ {
			r.setPx(x+4*i+dx, y+dy, colorHUD)
		}
	}
}

// char 是一字正常執行：putc 觀察點 → 02AD → 繪製。
func (r *rw) char(s string, i, x, y int) {
	r.putc(s, i, x, y, tObjHUD)
	r.next()
	r.ink(s, i, x, y)
}

// ret 是 puts 返回 03CD。
func (r *rw) ret(s string, x int) {
	r.v.w16(qDS, tObjHUD+objX, uint16(x+4*len(s)))
	r.v.ss, r.v.sp = tSS, qSP
	r.obs(r.base+segFText, ipFPutsRet)
}

func v6Counts(c FlightCounts) [3]int {
	return [3]int{c.PutcRedelivered, c.PutsRedelivered, c.PlaqueRedelivered}
}

func xsOf(x0, n int) []int {
	var out []int
	for i := 0; i < n; i++ {
		out = append(out, x0+4*i)
	}
	return out
}

func logHas(o *FlightObserver, kind, sub string) int {
	logs, _ := o.Log()
	n := 0
	for _, e := range logs {
		if e.Kind == kind && strings.Contains(e.Detail, sub) {
			n++
		}
	}
	return n
}

// flapsWith 跑 FLAPS（190, 15），第 2 字 A 依 aSeq 處理，其餘正常。
func (r *rw) flapsWith(aSeq func()) {
	const s, x, y = "FLAPS", 190, 15
	r.entry(s, x, y, false)
	r.char(s, 0, x, y)
	r.char(s, 1, x, y)
	aSeq()
	r.char(s, 3, x, y)
	r.char(s, 4, x, y)
	r.ret(s, x)
}

// 第 1 款：putc 送達 → 撤銷 → 重看。
func TestFlightRedeliverPutc(t *testing.T) {
	const s, x, y = "FLAPS", 190, 15
	r := newRW(t, true)
	var first uint64
	r.flapsWith(func() {
		r.putc(s, 2, x, y, tObjHUD) // A 觀察點（d = 0）
		first = r.v.steps
		r.deliver()                 // 下一觀察點在別的 PC（d = 1）
		r.putc(s, 2, x, y, tObjHUD) // A 再觀察（d = 1）
		r.next()                    // 02AD（d = 1）
		r.ink(s, 2, x, y)
	})
	// 同一序列無送達的對照。
	r0 := newRW(t, true)
	r0.flapsWith(func() { r0.char(s, 2, x, y) })
	p, p0 := r.o.Pending(), r0.o.Pending()
	c := r.o.Counts()
	if len(p) != 1 || len(p[0].Xs) != 5 || c.SkipVoid != 0 || v6Counts(c) != [3]int{1, 0, 0} {
		t.Fatalf("待提交 %d、SkipVoid %d、第 6 版計數 %v", len(p), c.SkipVoid, v6Counts(c))
	}
	if !reflect.DeepEqual(p[0].Xs, p0[0].Xs) || !reflect.DeepEqual(p[0].Xs, xsOf(x, 5)) {
		t.Fatalf("Xs %v，無送達 %v", p[0].Xs, p0[0].Xs)
	}
	if logHas(r.o, "redeliv", "") != 1 {
		t.Fatalf("log 的 redeliv %d 筆，要 1", logHas(r.o, "redeliv", ""))
	}
	ev, drop := r.o.Redelivered()
	if len(ev) != 1 || drop != 0 || ev[0] != (FlightRedeliverEvent{Steps: first, Kind: RedelivPutc, Key: s}) {
		t.Fatalf("事件 %+v、捨棄 %d", ev, drop)
	}
	if v6Counts(r0.o.Counts()) != [3]int{} {
		t.Fatalf("無送達的對照有第 6 版計數 %v", v6Counts(r0.o.Counts()))
	}
}

// 第 2 款：迴圈中送達在字形繪製期間（02AD 之後），下一字同 SS:SP 的 putc 照收（F70）。
func TestFlightRedeliverDuringGlyph(t *testing.T) {
	const s, x, y = "FLAPS", 190, 15
	r := newRW(t, true)
	r.entry(s, x, y, false)
	for i := 0; i < len(s); i++ {
		r.putc(s, i, x, y, tObjHUD)
		r.next()    // 計數不變
		r.deliver() // 繪製中送達
		r.ink(s, i, x, y)
	}
	r.ret(s, x)
	p := r.o.Pending()
	if len(p) != 1 || !reflect.DeepEqual(p[0].Xs, xsOf(x, 5)) || v6Counts(r.o.Counts()) != [3]int{} {
		t.Fatalf("待提交 %d、Xs %v、第 6 版計數 %v", len(p), p, v6Counts(r.o.Counts()))
	}
}

// 第 3 款：連續相同字元（000 NONE 的 0 0 0），第二個 0 之前的繪製期間有送達，兩筆都留（F71）。
func TestFlightRedeliverSameChar(t *testing.T) {
	const s, x, y = "000 NONE", 118, 79
	r := newRW(t, true)
	r.entry(s, x, y, false)
	for i := 0; i < len(s); i++ {
		r.putc(s, i, x, y, tObjHUD)
		r.next()
		if i == 0 {
			r.deliver()
		}
		r.ink(s, i, x, y)
	}
	r.ret(s, x)
	p := r.o.Pending()
	if len(p) != 1 || !reflect.DeepEqual(p[0].Xs, xsOf(x, 8)) || v6Counts(r.o.Counts()) != [3]int{} {
		t.Fatalf("待提交 %d、%+v、第 6 版計數 %v", len(p), p, v6Counts(r.o.Counts()))
	}
}

// 第 4 款：第一個 putc 送達，撤銷後 Xs 為空、Y 清除（F72），重看時重記，Y = y′、不作廢。
func TestFlightRedeliverFirstPutc(t *testing.T) {
	const s, x, y = "FLAPS", 190, 15
	r := newRW(t, true)
	r.entry(s, x, y, false)
	r.putc(s, 0, x, y, tObjHUD)
	r.deliver()
	if r.o.cur == nil || len(r.o.cur.t.Xs) != 0 || r.o.cur.t.Y != 0 {
		t.Fatalf("撤銷第一個 putc 後 Xs %v、Y %d，要空、0", r.o.cur.t.Xs, r.o.cur.t.Y)
	}
	r.putc(s, 0, x, y, tObjHUD)
	r.next()
	r.ink(s, 0, x, y)
	for i := 1; i < len(s); i++ {
		r.char(s, i, x, y)
	}
	r.ret(s, x)
	p := r.o.Pending()
	if len(p) != 1 || p[0].Y != y || r.o.Counts().SkipVoid != 0 || v6Counts(r.o.Counts()) != [3]int{1, 0, 0} {
		t.Fatalf("待提交 %d、SkipVoid %d、第 6 版計數 %v", len(p), r.o.Counts().SkipVoid, v6Counts(r.o.Counts()))
	}
}

// 第 5 款：puts 入口送達 → 入口重看。log 沒有「沒有返回」，PutsRedelivered 1，待提交 1 筆（F73）。
func TestFlightRedeliverPutsEntry(t *testing.T) {
	const s, x, y = "FLAPS", 190, 15
	r := newRW(t, true)
	r.entry(s, x, y, false)
	first := r.v.steps
	r.deliver()
	if r.o.cur != nil {
		t.Fatal("puts 入口送達後 cur 沒有丟棄")
	}
	r.entry(s, x, y, false)
	for i := 0; i < len(s); i++ {
		r.char(s, i, x, y)
	}
	r.ret(s, x)
	if n := logHas(r.o, "puts", "沒有返回"); n != 0 || len(r.o.Pending()) != 1 || v6Counts(r.o.Counts()) != [3]int{0, 1, 0} {
		t.Fatalf("「沒有返回」%d 筆、待提交 %d、第 6 版計數 %v", n, len(r.o.Pending()), v6Counts(r.o.Counts()))
	}
	if ev, _ := r.o.Redelivered(); len(ev) != 1 || ev[0] != (FlightRedeliverEvent{Steps: first, Kind: RedelivPuts, Key: s}) {
		t.Fatalf("事件 %+v", ev)
	}
}

// 第 6 款：橫幅入口送達 → 重看。PlaqueRedelivered 1，待提交 1 筆，沒有「沒有返回」（F74）。
func TestFlightRedeliverPlaque(t *testing.T) {
	r := newRW(t, true)
	// 先以一筆 hud 啟用 base。
	r.entry("GEAR", 190, 10, false)
	for i := 0; i < 4; i++ {
		r.char("GEAR", i, 190, 10)
	}
	r.ret("GEAR", 190)
	const s, x, y = "time burst", 125, 4
	v := r.v
	for i := 0; i < len(s); i++ {
		v.w8(qDS, tPlqStr+uint16(i), int(s[i]))
	}
	v.w8(qDS, tPlqStr+uint16(len(s)), 0)
	plqEntry := func() {
		v.ss, v.sp, v.ds = tSS, qSP, qDS
		v.w16(tSS, qSP+4, 0xB1CA)
		v.w16(tSS, qSP+6, tDesc)
		v.w16(tSS, qSP+8, x)
		v.w16(tSS, qSP+0x0A, y)
		v.w16(tSS, qSP+0x0C, tPlqStr)
		v.w16(tSS, qSP+0x0E, qDS)
		r.obs(tOvl, ipPlaque)
	}
	plqEntry()
	first := v.steps
	r.deliver()
	plqEntry()
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			continue
		}
		for dx := 0; dx < 7; dx++ {
			for dy := 0; dy < 5; dy++ {
				r.setPx(x+8*i+dx, y+dy, 5)
			}
		}
	}
	v.ax = uint16(x + 8*len(s))
	v.ss, v.sp = tSS, qSP
	r.obs(tOvl, ipPlaqueRet)
	if n := logHas(r.o, "plaque", "沒有返回"); n != 0 || len(r.o.Pending()) != 2 || v6Counts(r.o.Counts()) != [3]int{0, 0, 1} {
		t.Fatalf("「沒有返回」%d 筆、待提交 %v、第 6 版計數 %v", n, keysOf(r.o.Pending()), v6Counts(r.o.Counts()))
	}
	if ev, _ := r.o.Redelivered(); len(ev) != 1 || ev[0] != (FlightRedeliverEvent{Steps: first, Kind: RedelivPlaque, Key: s}) {
		t.Fatalf("事件 %+v", ev)
	}
}

// 第 7 款：重看又送達，第三次才執行。Xs 只一筆，PutcRedelivered 2。
func TestFlightRedeliverTwice(t *testing.T) {
	const s, x, y = "FLAPS", 190, 15
	r := newRW(t, true)
	r.flapsWith(func() {
		r.putc(s, 2, x, y, tObjHUD)
		r.deliver()
		r.putc(s, 2, x, y, tObjHUD)
		r.deliver()
		r.putc(s, 2, x, y, tObjHUD)
		r.next()
		r.ink(s, 2, x, y)
	})
	p := r.o.Pending()
	if len(p) != 1 || !reflect.DeepEqual(p[0].Xs, xsOf(x, 5)) || v6Counts(r.o.Counts()) != [3]int{2, 0, 0} {
		t.Fatalf("待提交 %d、%+v、第 6 版計數 %v", len(p), p, v6Counts(r.o.Counts()))
	}
}

// 第 8 款：返回點 03CD 送達後重看：待提交 1 筆，第 6 版計數不變。
func TestFlightRedeliverReturn(t *testing.T) {
	const s, x, y = "FLAPS", 190, 15
	r := newRW(t, true)
	r.entry(s, x, y, false)
	for i := 0; i < len(s); i++ {
		r.char(s, i, x, y)
	}
	r.ret(s, x)
	r.deliver()
	r.ret(s, x)
	if len(r.o.Pending()) != 1 || v6Counts(r.o.Counts()) != [3]int{} || logHas(r.o, "redeliv", "") != 0 {
		t.Fatalf("待提交 %d、第 6 版計數 %v", len(r.o.Pending()), v6Counts(r.o.Counts()))
	}
}

// 第 9 款：不記 pend 的入口（呼叫端不符的 puts 入口、obj 不符的 putc）在下一觀察點送達，計數不變、不撤銷（F75）。
func TestFlightRedeliverSkippedEntries(t *testing.T) {
	const s, x, y = "FLAPS", 190, 15
	r := newRW(t, true)
	r.entry(s, x, y, false)
	r.char(s, 0, x, y)
	r.char(s, 1, x, y)
	// 進行中插入一個呼叫端不符的 puts 入口，下一觀察點送達。
	r.entry("GEAR", 190, 10, true)
	r.deliver()
	// obj 不符的 putc，下一觀察點送達。
	r.putc(s, 1, x, y, 0x1234)
	r.deliver()
	for i := 2; i < len(s); i++ {
		r.char(s, i, x, y)
	}
	r.ret(s, x)
	c := r.o.Counts()
	p := r.o.Pending()
	if c.SkipCaller != 1 || len(p) != 1 || !reflect.DeepEqual(p[0].Xs, xsOf(x, 5)) || v6Counts(c) != [3]int{} || logHas(r.o, "redeliv", "") != 0 {
		t.Fatalf("呼叫端略過 %d、待提交 %d、第 6 版計數 %v", c.SkipCaller, len(p), v6Counts(c))
	}
}

// 第 10 款：putc 觀察點的同一步觸發簽章層清空，pend 一併清除（F76）；下一觀察點送達不 panic、計數不變。
func TestFlightRedeliverResetClearsPend(t *testing.T) {
	const s, x, y = "FLAPS", 190, 15
	r := newRW(t, true)
	r.entry("GEAR", 190, 10, false)
	for i := 0; i < 4; i++ {
		r.char("GEAR", i, 190, 10)
	}
	r.ret("GEAR", 190)
	r.cuFrame()
	if len(r.o.Visible()) != 1 {
		t.Fatalf("可見集合 %v", keysOf(r.o.Visible()))
	}
	r.entry(s, x, y, false)
	// puts 入口簽章改掉，讓 putc 觀察點那一步跑簽章層檢查而清空。
	r.v.mem[lin(r.base+segFText, ipFPuts)] ^= 0xFF
	r.v.steps += SigCheckSteps
	r.putc(s, 0, x, y, tObjHUD)
	if _, ok := r.o.Base(); ok || r.o.cur != nil {
		t.Fatalf("簽章層沒有清空（base 啟用 %t、cur %v）", ok, r.o.cur != nil)
	}
	if r.o.pendOn {
		t.Fatal("reset 之後 pend 沒有清除")
	}
	before := r.o.Counts()
	r.deliver()
	if v6Counts(r.o.Counts()) != [3]int{} || r.o.Counts() != before || logHas(r.o, "redeliv", "") != 0 {
		t.Fatalf("重設後送達改變計數：%+v", r.o.Counts())
	}
}

// 第 11 款：不提供 Deliveries 的視圖（fv）照第 5 版：第 1 款的序列 putc 6 次，作廢。
func TestFlightRedeliverNoDeliveries(t *testing.T) {
	const s, x, y = "FLAPS", 190, 15
	r := newRW(t, false)
	r.flapsWith(func() {
		r.putc(s, 2, x, y, tObjHUD)
		r.other()
		r.putc(s, 2, x, y, tObjHUD)
		r.next()
		r.ink(s, 2, x, y)
	})
	c := r.o.Counts()
	if len(r.o.Pending()) != 0 || c.SkipVoid != 1 || v6Counts(c) != [3]int{} {
		t.Fatalf("待提交 %d、SkipVoid %d、第 6 版計數 %v", len(r.o.Pending()), c.SkipVoid, v6Counts(c))
	}
}
