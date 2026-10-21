package strikecommander

import (
	"encoding/binary"
	"strings"
	"testing"
)

// strike_commander 規格 009 §4.1 的單元測試（合成 View）。期望值在測試內獨立寫出。

// fv 是合成的 FlightView：稀疏記憶體與暫存器。
type fv struct {
	mem                            map[uint32]uint8
	steps                          uint64
	cs, ip, ss, sp, bp, ds, ax, cx uint16
}

func (v *fv) Steps() uint64        { return v.steps }
func (v *fv) CS() uint16           { return v.cs }
func (v *fv) IP() uint16           { return v.ip }
func (v *fv) SS() uint16           { return v.ss }
func (v *fv) SP() uint16           { return v.sp }
func (v *fv) BP() uint16           { return v.bp }
func (v *fv) DS() uint16           { return v.ds }
func (v *fv) AX() uint16           { return v.ax }
func (v *fv) CX() uint16           { return v.cx }
func (v *fv) Read8(a uint32) uint8 { return v.mem[a] }

func (v *fv) w8(seg, off uint16, b ...int) {
	for i, x := range b {
		v.mem[lin(seg, off+uint16(i))] = uint8(x)
	}
}
func (v *fv) w16(seg, off, x uint16) { v.w8(seg, off, int(x&0xFF), int(x>>8)) }

// wsig 寫簽章；-1 的兩個位置依序填 fill 的低、高位元組。
func (v *fv) wsig(seg, off uint16, sig []int, fill uint16) {
	wild := 0
	for i, b := range sig {
		if b < 0 {
			b = int(fill >> (8 * wild) & 0xFF)
			wild++
		}
		v.mem[lin(seg, off+uint16(i))] = uint8(b)
	}
}

// 合成世界的位址（段值與 STRIKE 實際值無關，只需互不重疊）。
const (
	qDS      = 0x5000
	tObjHUD  = 0xC57A
	tObjMFD  = 0xCA8C
	tSS      = 0x6E8D
	qSP      = 0xFD00
	qFrameP  = 0xFD40
	qQ       = 0xFD80
	qStr     = 0x9000
	tCUSP    = 0xFDE6
	tDesc    = 0xB1E8
	tOvl     = 0xEB96
	tPlqStr  = 0x9100
	qList    = 0xE654
	tFBBase  = 0xFFFF0 + 0x10 // FFFF:0010
	colorHUD = 223
)

type world struct {
	t    *testing.T
	v    *fv
	base uint16
	o    *FlightObserver
}

func flightTestCats() FlightCatalogs {
	return FlightCatalogs{
		HUD: map[string]string{"GEAR": "起落架", "FLAPS": "襟翼", "BRAKE": "煞車", "DGFT": "纏鬥", "STRF": "掃射", "SRM": "短程飛彈", "NONE": "無"},
		MFD: map[string]string{"Comm Mode:": "通訊模式：", "Select Frequency": "選擇頻道", "NO RECEIVERS": "無人接收",
			"request status": "回報狀態", "NORM": "正常", "GUN": "機砲", "FUEL:": "燃油：", "AIR": "對空", "GND": "對地"},
		Plaque: map[string]string{"time burst": "時間加速", "enemies near": "敵機接近", "chase plane": "追蹤視角",
			"external view": "外部視角", "victim view on": "目標視角開", "weapon camera on": "武器攝影機開"},
	}
}

func newWorld(t *testing.T, base uint16) *world {
	v := &fv{mem: map[uint32]uint8{}}
	installFlightSigs(v, base)
	for r := 0; r < 200; r++ {
		v.w16(qDS, qList+uint16(2*r), uint16(0x10+r*320))
	}
	for _, obj := range []uint16{tObjHUD, tObjMFD} {
		v.w16(qDS, obj+objSeg, 0xFFFF)
		v.w16(qDS, obj+objList, qList)
	}
	v.w16(qDS, tDesc, 0xFFFF)
	v.w16(qDS, tDesc+2, qList)
	v.w16(qDS, tDesc+4, 0)
	v.w16(qDS, tDesc+6, 0)
	v.w16(qDS, tDesc+8, 319)
	v.w16(qDS, tDesc+10, 199)
	v.wsig(tOvl, ipPlaque, sigPlaqueHead, 0)
	v.wsig(tOvl, ipPlaqueLoop, sigPlaqueLoop, 0)
	cats := flightTestCats()
	w := &world{t: t, v: v, base: base}
	w.o = NewFlightObserver(func(k Kind, s string) (string, int, bool) { return ResolveFlight(k, s, cats) })
	v.steps = 1_000_000
	return w
}

func installFlightSigs(v *fv, base uint16) {
	v.wsig(base+segFText, ipFPuts, sigPutsEntry, 0)
	for _, r := range flightBaseRows {
		v.wsig(base+r.seg, r.off, r.bytes, 0)
	}
	v.wsig(base+segFHUD, ipFHUDRet-uint16(len(sigHUDCall)), sigHUDCall, base+segFText)
	v.wsig(base+segFMFD, ipFMFDRet-uint16(len(sigMFDCall)), sigMFDCall, base+segFText)
}

func (w *world) px(x, y int) uint32          { return uint32(tFBBase + y*320 + x) }
func (w *world) setPx(x, y int, c uint8)     { w.v.mem[w.px(x, y)] = c }
func (w *world) getPx(x, y int) uint8        { return w.v.mem[w.px(x, y)] }
func (w *world) at(cs, ip uint16)            { w.v.steps += 100; w.v.cs, w.v.ip = cs, ip; w.o.Observe(w.v) }
func (w *world) frame()                      { w.v.steps += 100; w.o.Frame(nil, w.v.steps) }
func (w *world) cuIn(ip, cx, sp uint16)      { w.v.cx, w.v.ss, w.v.sp = cx, tSS, sp; w.at(w.base+segFJemm, ip) }
func (w *world) cuRet(ip, sp uint16)         { w.v.ss, w.v.sp = tSS, sp; w.at(w.base+segFJemm, ip) }
func (w *world) mu(cx uint16)                { w.v.cx, w.v.ss, w.v.sp = cx, tSS, 0xFDF0; w.at(w.base+segFJemm, ipMU) }
func (w *world) cu()                         { w.cuIn(ipCUDiff, 0x0001, tCUSP); w.cuRet(ipCUDiffRet, tCUSP) }
func (w *world) cuFrame()                    { w.cu(); w.frame() }
func (w *world) fillBand(y0, y1 int, c uint8) {
	for y := y0; y < y1; y++ {
		for x := 0; x < 320; x++ {
			w.setPx(x, y, c)
		}
	}
}

// putsOpt 是一次合成的 puts 呼叫。
type putsOpt struct {
	kind      Kind
	s         string
	x, y      int
	adv       int   // 每字前進寬（hud 4、mfd 6）
	color     uint8 // 墨跡色
	skipSpace bool  // 空白不呼叫 putc（F19 的情形）
	putcY     int   // 第一次 putc 的 y；0 表示同 y
	ret, seg  uint16
	callerBad bool // 返回位址之前 1 byte 錯
	noInk     bool
	extraInk  *Point // 框外多一點墨跡
	inkFn     func(i, cx int)
}

func (w *world) puts(op putsOpt) {
	v := w.v
	if op.adv == 0 {
		op.adv = 4
		if op.kind == KindMFD {
			op.adv = 6
		}
	}
	obj := uint16(tObjHUD)
	r, s := uint16(ipFHUDRet), w.base+segFHUD
	if op.kind == KindMFD {
		obj, r, s = tObjMFD, ipFMFDRet, w.base+segFMFD
	}
	if op.ret != 0 {
		r, s = op.ret, op.seg
	}
	for i := 0; i < len(op.s); i++ {
		v.w8(qDS, qStr+uint16(i), int(op.s[i]))
	}
	v.w8(qDS, qStr+uint16(len(op.s)), 0)
	v.w16(tSS, qSP+4, obj)
	v.w16(tSS, qSP+6, qStr)
	v.w16(tSS, qFrameP, qQ)
	v.w16(tSS, qFrameP+2, ipFFmtRet)
	v.w16(tSS, qFrameP+4, w.base+segFText)
	v.w16(tSS, qQ+2, r)
	v.w16(tSS, qQ+4, s)
	if op.callerBad {
		v.w8(s, r-1, 0x00)
		defer v.wsig(w.base+segFHUD, ipFHUDRet-uint16(len(sigHUDCall)), sigHUDCall, w.base+segFText)
	}
	v.w16(qDS, obj+objX, uint16(op.x))
	v.w16(qDS, obj+objY, uint16(op.y))
	v.ss, v.sp, v.bp, v.ds = tSS, qSP, qFrameP, qDS
	w.at(w.base+segFText, ipFPuts)
	inkH := 4
	if op.kind == KindMFD {
		inkH = 5
	}
	py := op.y
	if op.putcY != 0 {
		py = op.putcY
	}
	for i := 0; i < len(op.s); i++ {
		cx := op.x + i*op.adv
		v.w16(qDS, obj+objX, uint16(cx))
		v.w16(qDS, obj+objY, uint16(py))
		if !(op.skipSpace && op.s[i] == ' ') {
			v.sp = qSP - 0x10
			v.w16(tSS, qSP-0x10+4, obj)
			v.w16(tSS, qSP-0x10+6, uint16(op.s[i])) // putc 的字元參數（含空白）
			w.at(w.base+segFText, ipFPutc)
			v.sp = qSP
		}
		if op.s[i] != ' ' && !op.noInk {
			if op.inkFn != nil {
				op.inkFn(i, cx)
				continue
			}
			for dx := 0; dx < op.adv-1; dx++ {
				for dy := 1; dy <= inkH; dy++ {
					w.setPx(cx+dx, op.y+dy, op.color)
				}
			}
		}
	}
	if op.extraInk != nil {
		w.setPx(op.extraInk.X, op.extraInk.Y, op.color)
	}
	v.w16(qDS, obj+objX, uint16(op.x+len(op.s)*op.adv))
	v.ss, v.sp = tSS, qSP
	w.at(w.base+segFText, ipFPutsRet)
}

func (w *world) hud(s string, x, y int) {
	w.puts(putsOpt{kind: KindHUD, s: s, x: x, y: y, color: colorHUD})
}

// plaqueOpt 是一次合成的橫幅呼叫。
type plaqueOpt struct {
	s        string
	x, y     int
	color    uint8
	desc     uint16
	reqSP    uint16 // 返回時的 SP；0 表示同入口
	noReturn bool
}

func (w *world) plaque(op plaqueOpt) {
	v := w.v
	if op.desc == 0 {
		op.desc = tDesc
	}
	for i := 0; i < len(op.s); i++ {
		v.w8(qDS, tPlqStr+uint16(i), int(op.s[i]))
	}
	v.w8(qDS, tPlqStr+uint16(len(op.s)), 0)
	v.ss, v.sp, v.ds = tSS, qSP, qDS
	v.w16(tSS, qSP+4, 0xB1CA)
	v.w16(tSS, qSP+6, op.desc)
	v.w16(tSS, qSP+8, uint16(op.x))
	v.w16(tSS, qSP+0x0A, uint16(int16(op.y)))
	v.w16(tSS, qSP+0x0C, tPlqStr)
	v.w16(tSS, qSP+0x0E, qDS)
	w.at(tOvl, ipPlaque)
	for i := 0; i < len(op.s); i++ {
		if op.s[i] == ' ' {
			continue
		}
		for dx := 0; dx < 7; dx++ {
			for dy := 0; dy < 5; dy++ {
				w.setPx(op.x+8*i+dx, op.y+dy, op.color)
			}
		}
	}
	if op.noReturn {
		return
	}
	v.ax = uint16(op.x + 8*len(op.s))
	if op.reqSP != 0 {
		v.sp = op.reqSP
	}
	w.at(tOvl, ipPlaqueRet)
	v.sp = qSP
}

func keysOf(ts []FlightText) []string {
	var out []string
	for _, t := range ts {
		out = append(out, t.Key)
	}
	return out
}

func eqStrs(a, b []string) bool { return strings.Join(a, "|") == strings.Join(b, "|") && len(a) == len(b) }

// ---- 第 1 項：辨識 ----

func TestFlightBaseLocate(t *testing.T) {
	for _, base := range []uint16{0x0241, 0x0300} {
		w := newWorld(t, base)
		w.hud("GEAR", 190, 10)
		if b, ok := w.o.Base(); !ok || b != base {
			t.Fatalf("base %04X：定位到 %04X、啟用 %t", base, b, ok)
		}
		if len(w.o.Pending()) != 1 {
			t.Fatalf("base %04X：待提交 %d 筆，要 1", base, len(w.o.Pending()))
		}
	}
}

func TestFlightBaseRowMismatch(t *testing.T) {
	for i, r := range flightBaseRows {
		w := newWorld(t, 0x0300)
		a := lin(w.base+r.seg, r.off+uint16(len(r.bytes)-1))
		w.v.mem[a] ^= 0xFF
		w.hud("GEAR", 190, 10)
		if _, ok := w.o.Base(); ok {
			t.Errorf("第 %d 列 %s 改 1 byte 仍啟用", i, r.name)
		}
		if len(w.o.Pending()) != 0 {
			t.Errorf("第 %d 列 %s 改 1 byte 仍收字串", i, r.name)
		}
	}
}

func TestFlightBaseReverify(t *testing.T) {
	w := newWorld(t, 0x0241)
	a := lin(w.base+segFGlyph, 0x0184)
	w.v.mem[a] = 0x90
	w.hud("GEAR", 190, 10)
	if _, ok := w.o.Base(); ok {
		t.Fatal("字形方法簽章不符仍啟用")
	}
	w.v.mem[a] = 0x55
	w.v.steps += 300_000 // 50 萬步內
	w.fillBand(11, 16, 0)
	w.hud("GEAR", 190, 10)
	if _, ok := w.o.Base(); ok {
		t.Fatal("50 萬步內就重驗（節流不成立）")
	}
	w.v.steps += 300_000 // 超過 50 萬步
	w.fillBand(11, 16, 0)
	w.hud("GEAR", 190, 10)
	if _, ok := w.o.Base(); !ok {
		t.Fatal("50 萬步後重驗仍未啟用（F34：永久停用）")
	}
	if len(w.o.Pending()) != 1 {
		t.Fatalf("重驗啟用後待提交 %d 筆，要 1", len(w.o.Pending()))
	}
}

func TestFlightOPTTESTPutsNotLocated(t *testing.T) {
	w := newWorld(t, 0x0241)
	// OPTTEST 的 puts 在 034B：同樣 16 bytes 放在 034B，IP 034B 不是 STRIKE 的 0345。
	seg := uint16(0x3B83)
	w.v.wsig(seg, ipPutsEntry, sigPutsEntry, 0)
	w.at(seg, ipPutsEntry)
	if _, ok := w.o.Base(); ok || w.o.baseKnown {
		t.Fatal("OPTTEST 的 puts 位置被定位")
	}
}

// 簽章層（§3.1 第 5 條）與 F38。
func TestFlightSigLayer(t *testing.T) {
	w := newWorld(t, 0x0241)
	w.hud("GEAR", 190, 10)
	w.cuFrame()
	if len(w.o.Visible()) != 1 {
		t.Fatalf("可見 %d 筆，要 1", len(w.o.Visible()))
	}
	gen := w.o.Generation()
	w.hud("FLAPS", 190, 15) // 待提交 1 筆
	w.cuIn(ipCUDiff, 1, tCUSP)
	w.cuRet(ipCUDiffRet, tCUSP)
	w.hud("BRAKE", 190, 20) // 候選 1、待提交 1、待升格
	w.v.mem[lin(w.base+segFText, ipFPuts)] = 0x90
	w.v.steps = w.o.lastSig + SigCheckSteps - 200
	w.at(0x1234, 0x0001)
	if len(w.o.Visible()) == 0 {
		t.Fatal("不到 500 萬步就清空")
	}
	w.v.steps += 200
	w.at(0x1234, 0x0001)
	if len(w.o.Visible()) != 0 || w.o.Generation() != gen+1 || len(w.o.Pending()) != 0 || len(w.o.Candidates()) != 0 {
		t.Fatalf("簽章不符後：可見 %d、gen %d（要 %d）、待提交 %d、候選 %d", len(w.o.Visible()), w.o.Generation(), gen+1, len(w.o.Pending()), len(w.o.Candidates()))
	}
	if in, aw := w.o.CUState(); in || aw {
		t.Fatalf("簽章不符後 CU 狀態 %t %t", in, aw)
	}
}

// ---- 第 2 項：呼叫端 ----

func TestFlightCaller(t *testing.T) {
	w := newWorld(t, 0x0241)
	w.hud("GEAR", 190, 10)
	w.puts(putsOpt{kind: KindMFD, s: "NORM", x: 222, y: 126, color: 220})
	p := w.o.Pending()
	if len(p) != 2 || p[0].Kind != KindHUD || p[1].Kind != KindMFD {
		t.Fatalf("待提交 %+v", keysOf(p))
	}
	// 返回位址對、前面 1 byte 錯：不收（F12）。
	w2 := newWorld(t, 0x0241)
	w2.puts(putsOpt{kind: KindHUD, s: "GEAR", x: 190, y: 10, color: colorHUD, callerBad: true})
	if len(w2.o.Pending()) != 0 || w2.o.Counts().SkipCaller != 1 {
		t.Fatalf("呼叫端位元組錯仍收：待提交 %d、呼叫端略過 %d", len(w2.o.Pending()), w2.o.Counts().SkipCaller)
	}
	// +1462:0259（速度帶）不收。
	w3 := newWorld(t, 0x0241)
	w3.v.wsig(w3.base+segFHUD, 0x0259-12, sigHUDCall, w3.base+segFText)
	w3.puts(putsOpt{kind: KindHUD, s: "GEAR", x: 190, y: 10, color: colorHUD, ret: 0x0259, seg: w3.base + segFHUD})
	if len(w3.o.Pending()) != 0 {
		t.Fatal("+1462:0259 被收")
	}
}

// ---- 第 3 項：Xs 與作廢 ----

func TestFlightXsAndVoid(t *testing.T) {
	w := newWorld(t, 0x0241)
	w.hud("000 NONE", 118, 79)
	p := w.o.Pending()
	if len(p) != 1 {
		t.Fatalf("待提交 %d 筆", len(p))
	}
	n := p[0]
	if n.K != 4 || n.CatKey != "NONE" || len(n.Xs) != 8 || n.Xs[4] != 134 || n.Anchor() != 134 || n.X0 != 118 || n.X1 != 150 {
		t.Fatalf("000 NONE：k %d catkey %q Xs %v X0 %d X1 %d", n.K, n.CatKey, n.Xs, n.X0, n.X1)
	}
	// 空白不記入 Xs（7 筆）：作廢（F19、F21）。
	w2 := newWorld(t, 0x0241)
	w2.puts(putsOpt{kind: KindHUD, s: "000 NONE", x: 118, y: 79, color: colorHUD, skipSpace: true})
	if len(w2.o.Pending()) != 0 || w2.o.Counts().SkipVoid != 1 {
		t.Fatalf("putc 7 筆仍收：待提交 %d", len(w2.o.Pending()))
	}
	// Y ≠ y′：作廢（F20）。
	w3 := newWorld(t, 0x0241)
	w3.puts(putsOpt{kind: KindHUD, s: "GEAR", x: 190, y: 10, color: colorHUD, putcY: 11})
	if len(w3.o.Pending()) != 0 || w3.o.Counts().SkipVoid != 1 {
		t.Fatalf("Y ≠ y′ 仍收：待提交 %d", len(w3.o.Pending()))
	}
}

// ---- 第 4 項：橫幅 ----

func TestFlightPlaque(t *testing.T) {
	w := newWorld(t, 0x0241)
	w.hud("GEAR", 190, 10) // 啟用 base
	w.plaque(plaqueOpt{s: "time burst", x: 125, y: 4, color: 5})
	p := w.o.Pending()
	if len(p) != 2 || p[1].Key != "time burst" || p[1].X0 != 125 || p[1].Y != 4 || p[1].X1 != 205 || p[1].B0 != 2 || p[1].BH != 9 {
		t.Fatalf("橫幅：%+v", p)
	}
	// 含 0Ah：略過。
	w.plaque(plaqueOpt{s: "time\nburst", x: 125, y: 4, color: 5})
	if len(w.o.Pending()) != 2 || w.o.Counts().SkipString != 1 {
		t.Fatalf("含 0Ah 仍收：%d", len(w.o.Pending()))
	}
	// 描述元段不是 FFFFh：略過（F14）。
	w.v.w16(qDS, 0xB300, 0xA000)
	w.plaque(plaqueOpt{s: "chase plane", x: 118, y: 181, color: 5, desc: 0xB300})
	if len(w.o.Pending()) != 2 || w.o.Counts().SkipDesc != 1 {
		t.Fatalf("描述元不符仍收：%d", len(w.o.Pending()))
	}
	// VGA 視窗旗標為真時不讀描述元就略過（F40）。
	w.mu(0)
	w.plaque(plaqueOpt{s: "chase plane", x: 118, y: 181, color: 5, desc: 0xB300})
	if c := w.o.Counts(); c.SkipDesc != 1 || c.SkipVGA != 1 {
		t.Fatalf("VGA 旗標為真：描述元略過 %d（要 1）、VGA 略過 %d（要 1）", c.SkipDesc, c.SkipVGA)
	}
	w.mu(1)
	// 入口與返回 SP 不同：不讀後列、不收。
	w.plaque(plaqueOpt{s: "chase plane", x: 118, y: 181, color: 5, reqSP: qSP + 2})
	if len(w.o.Pending()) != 0 { // MU CX=0 已清空待提交
		t.Fatalf("SP 不同仍收：%v", keysOf(w.o.Pending()))
	}
	if w.o.plq == nil || w.o.plq.t.After != nil {
		t.Fatal("SP 不同時入口事件應仍未返回、沒有後列")
	}
	// 入口時已有未返回的事件：舊的捨棄、記 log（F39）。
	w.plaque(plaqueOpt{s: "enemies near", x: 116, y: 4, color: 5})
	logs, _ := w.o.Log()
	found := false
	for _, e := range logs {
		if e.Kind == "plaque" && strings.Contains(e.Detail, "chase plane") && strings.Contains(e.Detail, "沒有返回") {
			found = true
		}
	}
	if !found || len(w.o.Pending()) != 1 || w.o.Pending()[0].Key != "enemies near" {
		t.Fatalf("未返回的舊橫幅未捨棄：log %t、待提交 %v", found, keysOf(w.o.Pending()))
	}
	// 描述元 rect 是底板裁切窗（實測 (112,0)-(207,12)）：在畫面內就收；超出畫面不收。
	w2 := newWorld(t, 0x0241)
	w2.hud("GEAR", 190, 10)
	for i, v := range []uint16{0xFFFF, qList, 112, 0, 207, 12} {
		w2.v.w16(qDS, 0xB400+uint16(2*i), v)
	}
	w2.plaque(plaqueOpt{s: "time burst", x: 125, y: 4, color: 5, desc: 0xB400})
	if p := w2.o.Pending(); len(p) != 2 || p[1].Key != "time burst" {
		t.Fatalf("裁切窗描述元沒有收：%v、描述元略過 %d", keysOf(p), w2.o.Counts().SkipDesc)
	}
	w2.v.w16(qDS, 0xB400+8, 320) // x1 超出畫面
	w2.plaque(plaqueOpt{s: "chase plane", x: 118, y: 181, color: 5, desc: 0xB400})
	if w2.o.Counts().SkipDesc != 1 {
		t.Fatalf("rect 超出畫面仍收：描述元略過 %d", w2.o.Counts().SkipDesc)
	}
}

// ---- 第 5 項：解析 ----

func TestResolveFlight(t *testing.T) {
	c := flightTestCats()
	for _, tc := range []struct {
		kind Kind
		s    string
		key  string
		k    int
		ok   bool
	}{
		{KindHUD, "GEAR", "GEAR", 0, true},
		{KindHUD, "000 NONE", "NONE", 4, true},
		{KindMFD, "1 request status", "request status", 2, true},
		{KindHUD, "1 request status", "", 0, false},
		{KindHUD, "999 Vulcan", "", 0, false},
		{KindHUD, "1.0G", "", 0, false},
		{KindMFD, "1 Hawk", "", 0, false},
		{KindPlaque, "time burst", "time burst", 0, true},
		{KindPlaque, "1 time burst", "", 0, false},
	} {
		key, k, ok := ResolveFlight(tc.kind, tc.s, c)
		if key != tc.key || k != tc.k || ok != tc.ok {
			t.Errorf("ResolveFlight(%s, %q) = %q %d %t，要 %q %d %t", tc.kind, tc.s, key, k, ok, tc.key, tc.k, tc.ok)
		}
	}
	// 觀察器經閉包使用解析：讀數與武器名不進待提交（F11）。
	w := newWorld(t, 0x0241)
	w.hud("999 Vulcan", 118, 79)
	w.hud("1.0G", 118, 17)
	w.puts(putsOpt{kind: KindMFD, s: "1 Hawk", x: 222, y: 139, color: 220})
	if len(w.o.Pending()) != 0 || w.o.Counts().SkipResolve != 3 {
		t.Fatalf("不可譯字串進了待提交：%v、解析略過 %d", keysOf(w.o.Pending()), w.o.Counts().SkipResolve)
	}
}

// ---- 第 6 項：待提交與提交 ----

func TestFlightPendingCapDedup(t *testing.T) {
	w := newWorld(t, 0x0241)
	for i := 0; i < 33; i++ {
		w.hud("GEAR", 190, i*5) // Y 不同，不去重
	}
	p := w.o.Pending()
	if len(p) != 32 || p[0].Y != 5 || w.o.Counts().SkipOverflow != 1 {
		t.Fatalf("上限：%d 筆、最舊 Y %d", len(p), p[0].Y)
	}
	w2 := newWorld(t, 0x0241)
	w2.hud("GEAR", 190, 10)
	first := w2.o.Pending()[0].Steps
	w2.fillBand(11, 16, 0)
	w2.hud("GEAR", 190, 10)
	p = w2.o.Pending()
	if len(p) != 1 || p[0].Steps == first || w2.o.Counts().SkipDedup != 1 {
		t.Fatalf("去重（F32）：%d 筆、保留第 %d 步（第一筆 %d）", len(p), p[0].Steps, first)
	}
}

func TestFlightCUEntryRefAndStale(t *testing.T) {
	w := newWorld(t, 0x0241)
	w.hud("GEAR", 190, 10)
	w.v.steps += flightStaleSteps // 過期
	w.hud("FLAPS", 190, 15)
	w.setPx(195, 18, 77) // CU 之前改一點：Ref 要讀到
	w.cuIn(ipCUDiff, 1, tCUSP)
	c := w.o.Candidates()
	if len(c) != 1 || c[0].Key != "FLAPS" || w.o.Counts().SkipStale != 1 || len(w.o.Pending()) != 0 {
		t.Fatalf("過期（F33）：候選 %v、過期 %d", keysOf(c), w.o.Counts().SkipStale)
	}
	if c[0].Ref == nil || c[0].Ref[(18-16)*320+195] != 77 {
		t.Fatal("Ref 沒有在 CU 入口讀")
	}
}

func checkInvariant(t *testing.T, o *FlightObserver) {
	t.Helper()
	c := o.Counts()
	_, aw := o.CUState()
	pend := 0
	if aw {
		pend = 1
	}
	if c.Promotions != c.ReturnsAccepted-c.CandidatesReplacedAwaiting-pend {
		t.Errorf("不變式：升格 %d ≠ 接受返回 %d − 返回後被取代 %d − 待升格 %d", c.Promotions, c.ReturnsAccepted, c.CandidatesReplacedAwaiting, pend)
	}
}

func TestFlightCUBasic(t *testing.T) {
	w := newWorld(t, 0x0241)
	commits := 0
	var last FlightCommit
	w.o.OnCommit = func(c FlightCommit) { commits++; last = c }
	w.hud("GEAR", 190, 10)
	w.cuIn(ipCUDiff, 1, tCUSP)
	if in, _ := w.o.CUState(); !in || len(w.o.Candidates()) != 1 || len(w.o.Pending()) != 0 {
		t.Fatalf("入口後：進行中 %t、候選 %d、待提交 %d", in, len(w.o.Candidates()), len(w.o.Pending()))
	}
	w.frame()
	if commits != 0 || len(w.o.Visible()) != 0 || w.o.Counts().CUFrames != 1 {
		t.Fatalf("CU 期間的 Frame 升格了（F36）：commits %d、CU 期間回掃 %d", commits, w.o.Counts().CUFrames)
	}
	w.cuRet(ipCUDiffRet, tCUSP)
	if in, aw := w.o.CUState(); in || !aw || w.o.Counts().ReturnsAccepted != 1 {
		t.Fatalf("返回後：進行中 %t、待升格 %t", in, aw)
	}
	if len(w.o.Visible()) != 0 {
		t.Fatal("CU 返回當下就升格（F2）")
	}
	w.frame()
	if commits != 1 || last.FrameSteps != w.v.steps || len(w.o.Candidates()) != 0 || len(w.o.Visible()) != 1 {
		t.Fatalf("升格：commits %d、FrameSteps %d（要 %d）、候選 %d（F49）", commits, last.FrameSteps, w.v.steps, len(w.o.Candidates()))
	}
	if _, aw := w.o.CUState(); aw {
		t.Fatal("升格後待升格仍為真（F41）")
	}
	w.frame()
	if commits != 1 {
		t.Fatalf("沒有新 CU 又升格（F41）：%d", commits)
	}
	checkInvariant(t, w.o)
}

func TestFlightCUDuplicateReturn(t *testing.T) {
	for _, between := range []bool{false, true} {
		w := newWorld(t, 0x0241)
		w.hud("GEAR", 190, 10)
		w.cuIn(ipCUDiff, 1, tCUSP)
		w.cuRet(ipCUDiffRet, tCUSP)
		if between {
			w.frame()
		}
		w.cuRet(ipCUDiffRet, tCUSP)
		w.frame()
		if !between {
			w.frame()
		}
		c := w.o.Counts()
		if c.Promotions != 1 || c.ReturnsIgnored != 1 {
			t.Fatalf("重複返回（Frame 夾在中間 %t）：升格 %d、忽略的返回 %d（F44）", between, c.Promotions, c.ReturnsIgnored)
		}
		checkInvariant(t, w.o)
	}
}

func TestFlightCUReturnSSMismatch(t *testing.T) {
	w := newWorld(t, 0x0241)
	w.hud("GEAR", 190, 10)
	w.cuIn(ipCUDiff, 1, tCUSP)
	w.cuRet(ipCUDiffRet, tCUSP-0x10)
	w.frame()
	c := w.o.Counts()
	if c.Promotions != 0 || c.ReturnsSSMismatch != 1 {
		t.Fatalf("SS:SP 不符的返回（F43）：升格 %d、不符 %d", c.Promotions, c.ReturnsSSMismatch)
	}
	checkInvariant(t, w.o)
}

func TestFlightCUDuplicateEntry(t *testing.T) {
	w := newWorld(t, 0x0241)
	w.hud("GEAR", 190, 10)
	w.cuIn(ipCUDiff, 1, tCUSP) // 入口：GEAR 成為候選，待提交為空
	ref := append([]byte(nil), w.o.Candidates()[0].Ref...)
	w.setPx(195, 12, 99) // 改一個 band 像素
	w.v.steps += 1000
	w.cuIn(ipCUDiff, 1, tCUSP) // 同 SS:SP、相隔約 1000 步
	c := w.o.Candidates()
	if len(c) != 1 || string(c[0].Ref) != string(ref) || w.o.Counts().EntriesDuplicate != 1 {
		t.Fatalf("重複入口（F42）：候選 %d、Ref 改變 %t、重複 %d", len(c), len(c) == 1 && string(c[0].Ref) != string(ref), w.o.Counts().EntriesDuplicate)
	}
}

func TestFlightCUEntryVoided(t *testing.T) {
	for _, tc := range []struct {
		name string
		sp   uint16
		gap  uint64
	}{{"SS:SP 不同", tCUSP - 0x20, 1000}, {"相隔 25 萬步", tCUSP, 250_000}} {
		w := newWorld(t, 0x0241)
		w.hud("GEAR", 190, 10)
		w.cuIn(ipCUDiff, 1, tCUSP)
		w.hud("FLAPS", 190, 15)
		w.v.steps += tc.gap
		w.cuIn(ipCUDiff, 1, tc.sp)
		c := w.o.Candidates()
		if len(c) != 1 || c[0].Key != "FLAPS" || w.o.Counts().EntriesVoided != 1 {
			t.Fatalf("%s（F47、F50）：候選 %v、作廢 %d", tc.name, keysOf(c), w.o.Counts().EntriesVoided)
		}
		w.cuRet(ipCUDiffRet, tc.sp)
		w.frame()
		if v := w.o.Visible(); len(v) != 1 || v[0].Key != "FLAPS" {
			t.Fatalf("%s：可見 %v", tc.name, keysOf(v))
		}
	}
}

func TestFlightTwoCUOneReturn(t *testing.T) {
	w := newWorld(t, 0x0241)
	w.hud("GEAR", 190, 10)
	w.cuIn(ipCUDiff, 1, tCUSP)
	w.cuRet(ipCUDiffRet, tCUSP)
	w.hud("FLAPS", 190, 15)
	w.cuIn(ipCUDiff, 1, tCUSP)
	if _, aw := w.o.CUState(); aw || w.o.Counts().CandidatesReplacedAwaiting != 1 {
		t.Fatalf("CU2 入口：待升格 %t（F48）、返回後被取代 %d", aw, w.o.Counts().CandidatesReplacedAwaiting)
	}
	w.frame()
	if len(w.o.Visible()) != 0 {
		t.Fatal("CU2 期間升格")
	}
	w.cuRet(ipCUDiffRet, tCUSP)
	w.frame()
	if v := w.o.Visible(); len(v) != 1 || v[0].Key != "FLAPS" {
		t.Fatalf("升格的是 %v，要 CU2 的 FLAPS", keysOf(v))
	}
	checkInvariant(t, w.o)
}

func TestFlightCUTimeout(t *testing.T) {
	w := newWorld(t, 0x0241)
	w.hud("GEAR", 190, 10)
	w.cuIn(ipCUDiff, 1, tCUSP)
	w.v.steps += cuTimeoutSteps
	w.frame()
	if in, _ := w.o.CUState(); in || w.o.Counts().Timeouts != 1 || len(w.o.Visible()) != 0 {
		t.Fatalf("逾時（F45）：進行中 %t、逾時 %d、可見 %d", in, w.o.Counts().Timeouts, len(w.o.Visible()))
	}
	w.hud("FLAPS", 190, 15)
	w.cuFrame()
	if v := w.o.Visible(); len(v) != 1 || v[0].Key != "FLAPS" {
		t.Fatalf("逾時後下一次 CU：可見 %v", keysOf(v))
	}
	checkInvariant(t, w.o)
}

func TestFlightCUReset(t *testing.T) {
	cases := map[string]func(w *world){
		"簽章層": func(w *world) {
			w.o.visible = []FlightText{{Key: "x"}}
			w.v.mem[lin(w.base+segFText, ipFPuts)] = 0x90
			w.v.steps += SigCheckSteps
			w.at(0x1234, 1)
		},
		"base 改變": func(w *world) {
			installFlightSigs(w.v, 0x0300)
			w.base = 0x0300
			w.hud("BRAKE", 190, 20)
		},
		"base 停用": func(w *world) {
			installFlightSigs(w.v, 0x0300)
			w.v.mem[lin(0x0300+segFGlyph, 0x0184)] = 0x90
			w.base = 0x0300
			w.hud("BRAKE", 190, 20)
		},
	}
	for name, f := range cases {
		w := newWorld(t, 0x0241)
		w.hud("GEAR", 190, 10)
		w.cuIn(ipCUDiff, 1, tCUSP)
		f(w)
		if in, aw := w.o.CUState(); in || aw || len(w.o.Candidates()) != 0 {
			t.Errorf("%s（F46）：進行中 %t、待升格 %t、候選 %d", name, in, aw, len(w.o.Candidates()))
		}
	}
}

func TestFlightCHNonZero(t *testing.T) {
	w := newWorld(t, 0x0241)
	commits := 0
	w.o.OnCommit = func(FlightCommit) { commits++ }
	w.hud("GEAR", 190, 10)
	w.cuFrame() // 可見 GEAR
	w.hud("FLAPS", 190, 15)
	w.cuIn(ipCUDiff, 1, tCUSP) // 候選 FLAPS（未升格）
	w.cuRet(ipCUDiffRet, tCUSP)
	w.hud("BRAKE", 190, 20) // 待提交 BRAKE
	commits = 0
	// 先把候選的待升格狀態保留：直接比 CH≠0 之後的變化。
	pend, cand, vis := keysOf(w.o.Pending()), keysOf(w.o.Candidates()), keysOf(w.o.Visible())
	_, aw := w.o.CUState()
	w.cuIn(ipCUPlain, 0x0200, 0xFE56)
	w.cuRet(ipCUPlainRet, 0xFE56)
	if !eqStrs(keysOf(w.o.Pending()), pend) || !eqStrs(keysOf(w.o.Candidates()), cand) || !eqStrs(keysOf(w.o.Visible()), vis) {
		t.Fatal("CH ≠ 0 改了集合（F35）")
	}
	c := w.o.Counts()
	if c.CUCHNonZero != 1 || c.ReturnsIgnored != 1 {
		t.Fatalf("CH ≠ 0 的 CU %d、忽略的返回 %d", c.CUCHNonZero, c.ReturnsIgnored)
	}
	if _, aw2 := w.o.CUState(); aw2 != aw {
		t.Fatal("CH ≠ 0 的返回改了待升格（F44）")
	}
	// 沒有待升格的狀態下：預放待提交與候選 → 00A5（CX=0200）→ 00A7 → Frame，OnCommit 0 次。
	w2 := newWorld(t, 0x0241)
	n := 0
	w2.o.OnCommit = func(FlightCommit) { n++ }
	w2.hud("GEAR", 190, 10)
	w2.cuIn(ipCUDiff, 1, tCUSP)
	w2.v.steps += cuTimeoutSteps // 讓候選留著、進行中逾時清掉
	w2.frame()
	w2.hud("FLAPS", 190, 15)
	w2.cuIn(ipCUPlain, 0x0200, 0xFE56)
	w2.cuRet(ipCUPlainRet, 0xFE56)
	w2.frame()
	if n != 0 || len(w2.o.Candidates()) != 1 || len(w2.o.Pending()) != 1 || len(w2.o.Visible()) != 0 {
		t.Fatalf("CH ≠ 0 序列：OnCommit %d、候選 %d、待提交 %d、可見 %d", n, len(w2.o.Candidates()), len(w2.o.Pending()), len(w2.o.Visible()))
	}
}

func TestFlightPlainViaDiff(t *testing.T) {
	w := newWorld(t, 0x0241)
	w.hud("GEAR", 190, 10)
	w.cuIn(ipCUPlain, 0x0001, 0xFDD8)
	w.cuRet(ipCUPlainRet, 0xFDD8)
	w.cuRet(ipCUDiffRet, tCUSP)
	w.frame()
	c := w.o.Counts()
	if c.Promotions != 1 || c.ReturnsIgnored != 1 {
		t.Fatalf("經差分 CU 呼叫一般 CU：升格 %d、忽略的返回 %d", c.Promotions, c.ReturnsIgnored)
	}
	checkInvariant(t, w.o)
}

func TestFlightCUJemmMismatch(t *testing.T) {
	w := newWorld(t, 0x0241)
	w.hud("GEAR", 190, 10)
	w.v.mem[lin(w.base+segFJemm, 0x00C0)] ^= 0xFF
	w.cuIn(ipCUDiff, 1, tCUSP)
	if in, _ := w.o.CUState(); in || len(w.o.Candidates()) != 0 {
		t.Fatal("98 bytes 有 1 byte 不同仍觸發")
	}
}

func TestFlightDigitPrefixGeneration(t *testing.T) {
	w := newWorld(t, 0x0241)
	w.hud("000 NONE", 118, 79)
	w.cuFrame()
	gen := w.o.Generation()
	w.fillBand(80, 85, 0)
	w.hud("001 NONE", 118, 79)
	w.setPx(140, 81, 55) // 改一點，看前後列、Ref 有更新
	w.cuFrame()
	v := w.o.Visible()
	if w.o.Generation() != gen || len(v) != 1 || v[0].Key != "001 NONE" || v[0].Ref[(81-80)*320+140] != 55 {
		t.Fatalf("數字改變：gen %d（要 %d）、可見 %v", w.o.Generation(), gen, keysOf(v))
	}
	w.fillBand(80, 85, 0)
	w.hud("00 NONE", 118, 79) // Xs[k] 從 134 變 130
	w.cuFrame()
	if w.o.Generation() != gen+1 {
		t.Fatalf("Xs[k] 改變：gen %d，要 %d", w.o.Generation(), gen+1)
	}
}

// ---- 第 7 項：VGA 視窗旗標 ----

func TestFlightVGAWindow(t *testing.T) {
	w := newWorld(t, 0x0241)
	w.hud("GEAR", 190, 10)
	w.cuIn(ipCUDiff, 1, tCUSP)
	w.cuRet(ipCUDiffRet, tCUSP) // 候選 GEAR 待升格
	w.hud("FLAPS", 190, 15)     // 待提交
	w.mu(0)
	if !w.o.VGAWindow() || len(w.o.Pending()) != 0 {
		t.Fatal("MU CX=0 沒有設旗標或沒有清待提交")
	}
	w.hud("BRAKE", 190, 20)
	w.plaque(plaqueOpt{s: "time burst", x: 125, y: 4, color: 5})
	if len(w.o.Pending()) != 0 || w.o.Counts().SkipVGA != 2 {
		t.Fatalf("旗標為真仍收（F4）：%v", keysOf(w.o.Pending()))
	}
	w.frame()
	if v := w.o.Visible(); len(v) != 1 || v[0].Key != "GEAR" {
		t.Fatalf("未升格的候選沒有保留（F25）：%v", keysOf(v))
	}
	w.mu(1)
	if w.o.VGAWindow() {
		t.Fatal("CX ≠ 0 的 MU 沒有解除")
	}
	w.mu(0)
	w.cuIn(ipCUDiff, 1, tCUSP)
	if w.o.VGAWindow() {
		t.Fatal("CU 入口沒有解除（F23）")
	}
	w.cuRet(ipCUDiffRet, tCUSP)
	w.mu(0)
	installFlightSigs(w.v, 0x0300)
	w.base = 0x0300
	w.hud("GEAR", 190, 10)
	if w.o.VGAWindow() {
		t.Fatal("base 改變沒有重設旗標（F24）")
	}
}

// ---- 第 8 項：版面 ----

func hudText(catKey, key string, x0, y int) FlightText {
	t := FlightText{Key: key, CatKey: catKey, Kind: KindHUD, X0: x0, Y: y}
	for i := range key {
		t.Xs = append(t.Xs, x0+4*i)
	}
	t.X1 = x0 + 4*len(key)
	t.K = len(key) - len(catKey)
	t.B0, t.BH = flightBand(KindHUD, y)
	return t
}

func TestFlightLayout(t *testing.T) {
	// HUD 3 倍。
	g, why := flightLayout(hudText("GEAR", "GEAR", 190, 10), 3, 3)
	if why != "" || g.tx != 190 || g.ty != 11 || g.cellW != 4 || g.cellH != 4 || g.glyphX != 0 || g.glyphY != 0 || g.cx0 != 189 || g.cx1 != 207 {
		t.Fatalf("GEAR：%+v %s", g, why)
	}
	if b0, bh := flightBand(KindHUD, 10); b0 != 11 || bh != 5 {
		t.Fatalf("HUD band（F26）：%d %d", b0, bh)
	}
	if b0, bh := flightBand(KindPlaque, 4); b0+bh > 11 {
		t.Fatalf("橫幅 band %d+%d 與 GEAR band 11 重疊", b0, bh)
	}
	// 右欄 R：7 字放得下時左移到 R+1−28。
	for _, tc := range []struct {
		key string
		y   int
		r   int
	}{{"GEAR", 10, 213}, {"FLAPS", 15, 212}, {"BRAKE", 20, 211}} {
		g, why := flightLayout(hudText(tc.key, tc.key, 190, tc.y), 7, 3)
		if why != "" || g.tx != tc.r+1-28 {
			t.Errorf("%s 7 字：tx %d（要 %d）%s", tc.key, g.tx, tc.r+1-28, why)
		}
		// 8 字 w = 32：tx = max(183, R + 1 − 32) = 183，183 + 31 = 214 > R。
		if _, why := flightLayout(hudText(tc.key, tc.key, 190, tc.y), 8, 3); why == "" {
			t.Errorf("%s 8 字應 Skip", tc.key)
		}
	}
	// 左欄放不下時 Skip（F13）。
	if g, why := flightLayout(hudText("DGFT", "DGFT", 118, 74), 8, 3); why != "" || g.tx != 118 {
		t.Errorf("DGFT 8 字：%+v %s", g, why)
	}
	if _, why := flightLayout(hudText("DGFT", "DGFT", 118, 74), 9, 3); why == "" {
		t.Error("DGFT 9 字應 Skip（左欄不能左移）")
	}
	none := hudText("NONE", "000 NONE", 118, 79)
	if g, why := flightLayout(none, 1, 3); why != "" || g.tx != 134 || g.cx0 != 133 || g.cx1 != 151 {
		t.Errorf("000 NONE：%+v %s", g, why)
	}
	if _, why := flightLayout(none, 5, 3); why == "" {
		t.Error("NONE 5 字應 Skip")
	}
	// MFD。
	mfd := func(catKey, key string, x0, y int) FlightText {
		t := FlightText{Key: key, CatKey: catKey, Kind: KindMFD, X0: x0, Y: y, K: len(key) - len(catKey)}
		for i := range key {
			t.Xs = append(t.Xs, x0+6*i)
		}
		t.X1 = x0 + 6*len(key)
		t.B0, t.BH = flightBand(KindMFD, y)
		return t
	}
	for _, tc := range []struct{ sc, tx, gx, gy, cw int }{{2, 250, 0, 0, 8}, {3, 264, 1, 5, 6}} {
		// 7 字的合成例從 x 276 起：放不下時左移（§3.7.2）。
		g, why := flightLayout(mfd("GUN", "GUN", 276, 126), 7, tc.sc)
		if why != "" || g.tx != tc.tx || g.glyphX != tc.gx || g.glyphY != tc.gy || g.cellW != tc.cw || g.cellH != 8 || g.ty != 125 {
			t.Errorf("7 字合成例 %d 倍：%+v %s", tc.sc, g, why)
		}
		// GUN 譯「機砲」2 字：原位放得下，不左移（使用者 2026-10-21）。
		g, why = flightLayout(mfd("GUN", "GUN", 276, 126), 2, tc.sc)
		if why != "" || g.tx != 276 || g.cx0 != 276 {
			t.Errorf("GUN 機砲 %d 倍：%+v %s", tc.sc, g, why)
		}
		g, _ = flightLayout(mfd("NORM", "NORM", 222, 126), 2, tc.sc)
		if g.tx+2*g.cellW-1 >= 276 {
			t.Errorf("NORM %d 倍右緣 %d 與 GUN 起點 276 相交", tc.sc, g.tx+2*g.cellW-1)
		}
	}
	if _, why := flightLayout(mfd("GUN", "GUN", 276, 126), 11, 2); why == "" {
		t.Error("MFD w 88 > 87 應 Skip")
	}
	if _, why := flightLayout(mfd("GUN", "GUN", 276, 126), 10, 2); why != "" {
		t.Errorf("MFD w 80 不應 Skip：%s", why)
	}
	if g, _ := flightLayout(mfd("AIR", "AIR", 42, 121), 2, 2); g.tx != 42 {
		t.Errorf("左 MFD：%+v", g)
	}
	// 橫幅：墨跡 125..193。
	pl := FlightText{Key: "time burst", CatKey: "time burst", Kind: KindPlaque, X0: 125, Y: 4, B0: 2, BH: 9}
	pl.Before, pl.After = make([]byte, 9*320), make([]byte, 9*320)
	pl.After[2*320+125], pl.After[6*320+193] = 5, 5
	for _, tc := range []struct{ sc, tx, gx, gy int }{{2, 143, 0, 1}, {3, 147, 1, 5}} {
		g, why := flightLayout(pl, 4, tc.sc)
		if why != "" || g.tx != tc.tx || g.glyphX != tc.gx || g.glyphY != tc.gy || g.cellH != 9 || g.ty != 2 {
			t.Errorf("橫幅 %d 倍：%+v %s（F28 靠左會得 125）", tc.sc, g, why)
		}
	}
	if _, why := flightLayout(pl, 9, 2); why == "" {
		t.Error("橫幅 w 72 > 69 應 Skip（F22）")
	}
}

func TestFlightOverlayScales(t *testing.T) {
	hud, label := testHUDFont(t), flightLabelFont(t)
	if _, err := NewFlightOverlay(nil, label, flightTestCats(), 3); err == nil {
		t.Fatal("3 倍缺 HUD 字型應回錯")
	}
	if _, err := NewFlightOverlay(nil, label, flightTestCats(), 2); err != nil {
		t.Fatal(err)
	}
	w := newWorld(t, 0x0241)
	w.hud("GEAR", 190, 10)
	w.cuFrame()
	for _, sc := range []int{2, 3} {
		fo, err := NewFlightOverlay(hud, label, flightTestCats(), sc)
		if err != nil {
			t.Fatal(err)
		}
		fo.Sync(w.o.Visible(), w.o.Generation())
		if sc == 2 && (len(fo.entries) != 0 || len(fo.Misses) != 0 || len(fo.Skips) != 0 || !fo.HUDScaleOff) {
			t.Fatalf("2 倍 HUD（F1）：%d 筆、misses %v、skips %v、HUDScaleOff %t", len(fo.entries), fo.Misses, fo.Skips, fo.HUDScaleOff)
		}
		if sc == 3 && (len(fo.entries) != 1 || fo.HUDScaleOff) {
			t.Fatalf("3 倍 HUD：%d 筆", len(fo.entries))
		}
	}
}

// ---- 字型 ----

const flightHUDRunes = "起落架襟翼煞車纏鬥掃射短程飛彈無"
const flightLabelRunes = "通訊模式：選擇頻道無人接收回報狀態正常機砲（ＧＵＮ）燃油對空地時間加速敵機接近追蹤視角外部目標開武器攝影"

// buildHUD12 合成一份 12×12 GOLEMFNT：每字格列 1..11、欄 0..10 全亮（同 Fusion 的 BBX 11 11 0 -2）。
func buildHUD12(runes string) []byte {
	b := []byte("GOLEMFNT")
	b = binary.LittleEndian.AppendUint16(b, 12)
	b = binary.LittleEndian.AppendUint16(b, 12)
	b = binary.LittleEndian.AppendUint32(b, uint32(len([]rune(runes))))
	for _, r := range runes {
		b = binary.LittleEndian.AppendUint32(b, uint32(r))
		b = append(b, 0x82)
		for row := 0; row < 12; row++ {
			if row == 0 {
				b = append(b, 0, 0)
			} else {
				b = append(b, 0xFF, 0xE0)
			}
		}
	}
	return b
}

func testHUDFont(t *testing.T) *HUDFont {
	t.Helper()
	f, err := ParseHUDFont(buildHUD12(flightHUDRunes), "hud12-test")
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func flightLabelFont(t *testing.T) *LabelFont {
	t.Helper()
	f, err := ParseLabelFont(buildTestFont(flightLabelRunes, "a -"), "full16-flight-test")
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// 第 13 項（Go 端）：ParseHUDFont 拒絕 16×16。放置公式與失敗條件由 bdf_to_golemfnt.py --self-test 驗。
func TestParseHUDFont(t *testing.T) {
	if _, err := ParseHUDFont(buildTestFont("測", ""), "x"); err == nil {
		t.Fatal("ParseHUDFont 接受了 16×16")
	}
	f := testHUDFont(t)
	if f.Font.W != 12 || !f.Wide.Wide('起') {
		t.Fatalf("HUD12：%dx%d", f.Font.W, f.Font.H)
	}
}
