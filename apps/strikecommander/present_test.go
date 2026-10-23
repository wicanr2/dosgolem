package strikecommander

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// strike_commander 規格 014 分期 (c) 的單元測試（§4.1 U1、U2、U6、U8、U9、U13、U14、U19、U23、U27、U28 與分期 (c) 的負對照）。
// 合成 View：flight_test.go 的 fv 加上實體 VGA、色盤、送達計數與批次讀。期望值在測試內獨立寫出。
//
// 負對照有兩種跑法：TestN…Detected 以突變旗標直接驗「規則關掉，對應檢查必須失敗」；DOSGOLEM_SC_PRESENT_MUT 設定時，
// 對應的 U 測試本身以該突變執行（應失敗）。

type pview struct {
	*fv
	vga   []byte
	pal   [256][3]uint8
	deliv uint64
	mode  uint8
}

func (v *pview) Deliveries() uint64     { return v.deliv }
func (v *pview) Palette() [256][3]uint8 { return v.pal }
func (v *pview) ReadBlock(a uint32, dst []byte) bool {
	for i := range dst {
		dst[i] = v.mem[a+uint32(i)]
	}
	return true
}
func (v *pview) IndexedLayout() machine.IndexedLayout {
	return machine.IndexedLayout{Kind: machine.IndexedLinear, Mode: v.mode, W: 320, H: 200}
}
func (v *pview) IndexedInto(dst []byte) (machine.IndexedLayout, bool) {
	l := v.IndexedLayout()
	if len(dst) < 64000 {
		return l, false
	}
	copy(dst, v.vga)
	return l, true
}

// 測試注入的 2D 窗條目（絕對 CS）：代分期 (d) 的 shape 窗（例：STRIKE 暫停 shape 窗），只供單元測試。
const (
	t2dCS    = 0x7000
	t2dEntry = 0x0100
	t2dRet   = 0x0180
	tpBase   = 0x0241 // OPTTEST 與 STRIKE 的合成 base
	tSrcDesc = 0xA000
	tDstDesc = 0xA100
	tList2   = 0xE900 // 倒序列表
)

var testRoutines = append([]presRoutine{{name: "t2d", prog: progAbs, seg: t2dCS, entryIP: t2dEntry, retIP: t2dRet,
	ipLo: t2dEntry, ipHi: t2dRet, completion: true, timeout: presTimeoutDefault, kind: PresShape}}, presRoutines...)

type presRig struct {
	t  *testing.T
	w  *world
	v  *pview
	so *SessionObserver
	p  *Presenter
}

func presMutEnv(t *testing.T) presentMut {
	m, err := parsePresentMut(os.Getenv("DOSGOLEM_SC_PRESENT_MUT"))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func newPresRig(t *testing.T, mut presentMut) *presRig {
	t.Helper()
	w := newWorld(t, tpBase)
	w.o.base, w.o.baseKnown, w.o.baseOK = tpBase, true, true
	w.o.cu.SetBase(tpBase, true)
	v := &pview{fv: w.v, vga: make([]byte, 64000), pal: tPal(), mode: 0x13}
	// 畫外緩衝 FFFF:0010 起（列表 qList），倒序列表 tList2。
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			w.setPx(x, y, uint8((x+3*y)%251))
		}
		v.w16(qDS, tList2+uint16(2*y), uint16(0x10+(199-y)*320))
	}
	// blit 的來源與目的描述元（全螢幕）。
	for _, d := range []uint16{tSrcDesc, tDstDesc} {
		v.w16(qDS, d, 0xFFFF)
		v.w16(qDS, d+2, qList)
		v.w16(qDS, d+4, 0)
		v.w16(qDS, d+6, 0)
		v.w16(qDS, d+8, 319)
		v.w16(qDS, d+10, 199)
	}
	cats := flightTestCats()
	so := &SessionObserver{Labels: NewLabelObserver(), Flight: w.o}
	so.Labels.base, so.Labels.baseKnown, so.Labels.baseOK = tpBase, true, true
	p, err := NewPresenter(PresenterConfig{Fonts: flightTextFonts(t), Catalog: testCatalog(), GfxCatalog: map[string]string{},
		FlightCatalogs: &cats, Scales: []int{2, 3}, Receipt: true, mut: mut, routines: testRoutines})
	if err != nil {
		t.Fatal(err)
	}
	p.Attach(so)
	return &presRig{t: t, w: w, v: v, so: so, p: p}
}

func (r *presRig) at(cs, ip uint16) {
	r.v.steps += 100
	r.v.cs, r.v.ip = cs, ip
	r.so.Labels.Observe(r.v)
	r.so.Flight.Observe(r.v)
	r.p.Observe(r.v)
}

// frame 是一次回掃（idx 為 nil 時用目前的 VGA）；不推進步數（同一步的回掃）。
func (r *presRig) frame(idx []byte) {
	if idx == nil {
		idx = append([]byte(nil), r.v.vga...)
	}
	r.so.steps = r.v.steps
	r.so.Frame(idx, r.v.pal)
}

func (r *presRig) write(off int, c uint8, cs, ip uint16, ctx machine.VideoWriteCtx) {
	r.v.vga[off] = c
	r.so.VideoWrite(machine.VideoWrite{Step: r.v.steps, CS: cs, IP: ip, Offset: uint32(off), Value: c, Ctx: ctx})
}

// win2d 是一次注入的 2D 窗：入口、窗內寫入、配對返回。
func (r *presRig) win2d(sp uint16, writes func()) {
	r.v.ss, r.v.sp = tSS, sp
	r.at(t2dCS, t2dEntry)
	if writes != nil {
		writes()
	}
	r.v.ss, r.v.sp = tSS, sp
	r.at(t2dCS, t2dRet)
}

func (r *presRig) cuIn(ip, cx, sp uint16) {
	r.v.cx, r.v.ss, r.v.sp = cx, tSS, sp
	r.at(tpBase+segFJemm, ip)
}

func (r *presRig) cuRet(ip, sp uint16) {
	r.v.ss, r.v.sp = tSS, sp
	r.at(tpBase+segFJemm, ip)
}

// fb 回畫外緩衝的整張色號（依 qList）。
func (r *presRig) fb() []byte {
	out := make([]byte, 64000)
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			out[y*320+x] = r.w.getPx(x, y)
		}
	}
	return out
}

// copyFB 模擬 CU 把畫外緩衝搬上 VGA（不經 VideoWrite：CU 窗內的寫入一律歸屬，這裡只關心內容）。
func (r *presRig) copyFB() { copy(r.v.vga, r.fb()) }

func (r *presRig) text(key string, y int, seg, list uint16) FlightText {
	return FlightText{Key: key, CatKey: key, Kind: KindHUD, X0: 190, Xs: []int{190, 194, 198, 202}, X1: 206, Y: y, B0: y + 1, BH: 5,
		DS: qDS, Seg: seg, List: list, Steps: r.v.steps}
}

func (r *presRig) inject(ts ...FlightText) { r.w.o.pending = append(r.w.o.pending, ts...) }

func (r *presRig) count(k PresentKind) int { return r.p.counts.Captures[k] }

func flightKeys(ts []FlightText) string {
	var b bytes.Buffer
	for _, t := range ts {
		b.WriteString(t.Key + ";")
	}
	return b.String()
}

// ---- U1：blit 窗（全螢幕與非全螢幕） ----

func (r *presRig) blit(sp uint16, partial bool, during func()) {
	r.v.ds = qDS
	r.v.w16(tSS, sp+4, tSrcDesc)
	r.v.w16(tSS, sp+6, tDstDesc)
	if partial {
		r.v.w16(qDS, tDstDesc+6, 24)
		r.v.w16(qDS, tDstDesc+10, 151)
	} else {
		r.v.w16(qDS, tDstDesc+6, 0)
		r.v.w16(qDS, tDstDesc+10, 199)
	}
	r.v.ss, r.v.sp = tSS, sp
	r.at(tpBase+segBlit, ipBlitEntry)
	if during != nil {
		during()
	}
	r.v.ss, r.v.sp = tSS, sp
	r.at(tpBase+segBlit, ipBlitRet)
}

func TestU1BlitCompletion(t *testing.T) {
	for _, partial := range []bool{false, true} {
		r := newPresRig(t, presMutEnv(t))
		lo := r.so.Labels
		lo.pending = []Label{{Key: "Fly Mission", X0: 100, X1: 150, Y: 100, Font: measuredFont, Kind: KindLabel, Seg: 0xFFFF, List: qList, Steps: r.v.steps}}
		bcs := uint16(tpBase + segBlit)
		var before int
		r.blit(0xFC00, partial, func() {
			for i := 0; i < 320; i++ {
				r.write(24*320+i, 7, bcs, 0x0074, machine.VWV86)
			}
			before = len(r.p.captures)
			r.frame(nil) // 回掃落在 blit 窗內：不產生呈現像
			if len(r.p.captures) != before {
				t.Fatalf("partial=%t：blit 窗內的回掃產生了呈現像 %v", partial, r.p.captures)
			}
		})
		want := PresBlit
		if partial {
			want = PresPartial
		}
		info := r.p.Current()
		if len(r.p.captures) != before+1 || info.Kind != want || info.SourceCS != bcs || info.SourceIP != ipBlitRet {
			t.Fatalf("partial=%t：blit 返回的呈現像 %+v（共 %d 個），要 %s", partial, info, len(r.p.captures), want)
		}
		// 全螢幕：提交後的標籤集合；非全螢幕不提交，集合為空。
		var keys []string
		for _, l := range r.p.cur.labels {
			keys = append(keys, l.Key)
		}
		if !partial && (len(keys) != 1 || keys[0] != "Fly Mission") {
			t.Fatalf("全螢幕 blit 返回的呈現像標籤 %v，要 [Fly Mission]", keys)
		}
		if !bytes.Equal(r.p.cur.indexed, r.v.vga) {
			t.Fatalf("partial=%t：呈現像色號不等於返回時的 VGA", partial)
		}
		r.frame(nil)
		if len(r.p.captures) != before+1 {
			t.Fatalf("partial=%t：blit 返回之後的回掃產生了呈現像", partial)
		}
		if c := r.p.Counts(); c.Unattributed != 0 || c.Stale != 0 {
			t.Fatalf("partial=%t：未歸屬 %d、陳舊 %d，要 0", partial, c.Unattributed, c.Stale)
		}
	}
}

// ---- 窗的重複入口（U3 的 blit 部分；N24） ----

func checkDupEntry(t *testing.T, mut presentMut) []string {
	r := newPresRig(t, mut)
	var bad []string
	sp := uint16(0xFC00)
	r.v.ds = qDS
	r.v.w16(tSS, sp+4, tSrcDesc)
	r.v.w16(tSS, sp+6, tDstDesc)
	r.v.ss, r.v.sp = tSS, sp
	r.at(tpBase+segBlit, ipBlitEntry)
	r.v.deliv++ // IRQ 送達：同一 PC 再被觀察一次
	r.at(tpBase+segBlit, ipBlitEntry)
	if c := r.p.counts; c.DuplicateDelivered != 1 || c.DuplicateEntry != 0 || len(r.p.stack) != 1 {
		bad = append(bad, fmt.Sprintf("送達後的重複入口：Delivered %d、Entry %d、深度 %d，要 1、0、1", c.DuplicateDelivered, c.DuplicateEntry, len(r.p.stack)))
	}
	r.at(tpBase+segBlit, ipBlitEntry) // 沒有送達的重複入口：異常
	if c := r.p.counts; c.DuplicateEntry != 1 || len(r.p.stack) != 1 {
		bad = append(bad, fmt.Sprintf("沒有送達的重複入口：Entry %d、深度 %d，要 1、1", c.DuplicateEntry, len(r.p.stack)))
	}
	r.at(tpBase+segBlit, ipBlitRet)
	r.v.deliv++
	r.at(tpBase+segBlit, ipBlitRet) // 返回點的 IRQ 重複觀察
	if c := r.p.counts; c.ReturnDuplicateDelivered != 1 || c.ReturnWithoutEntry != 0 {
		bad = append(bad, fmt.Sprintf("返回點的重複觀察：%d、ReturnWithoutEntry %d，要 1、0", c.ReturnDuplicateDelivered, c.ReturnWithoutEntry))
	}
	return bad
}

func TestPresentDupEntry(t *testing.T) {
	for _, b := range checkDupEntry(t, presMutEnv(t)) {
		t.Error(b)
	}
}

func TestN24Detected(t *testing.T) {
	if len(checkDupEntry(t, presMutN24)) == 0 {
		t.Fatal("N24（重複入口不看 Deliveries）沒有被檢查抓到")
	}
}

// 巢狀、跳層與逾時（§3.2.1 第 3、4 條）。
func TestPresentNestSkipTimeout(t *testing.T) {
	r := newPresRig(t, 0)
	r.v.ss, r.v.sp = tSS, 0xF000
	r.at(t2dCS, t2dEntry)
	r.v.sp = 0xEF00
	r.at(t2dCS, t2dEntry) // 內層（不同 SP）
	if len(r.p.stack) != 2 || r.p.counts.MaxDepth != 2 {
		t.Fatalf("巢狀深度 %d", len(r.p.stack))
	}
	r.v.sp = 0xF000
	r.at(t2dCS, t2dRet) // 外層返回：內層被跳過
	if c := r.p.counts; c.WindowSkipped != 1 || len(r.p.stack) != 0 {
		t.Fatalf("跳層 %d、深度 %d", c.WindowSkipped, len(r.p.stack))
	}
	r.at(t2dCS, t2dEntry)
	r.v.steps += presTimeoutDefault
	r.at(0x1234, 0x5678)
	if c := r.p.counts; c.WindowTimeout != 1 || len(r.p.stack) != 0 {
		t.Fatalf("逾時 %d、深度 %d", c.WindowTimeout, len(r.p.stack))
	}
}

// ---- U2：佇列 ----

func TestU2Queue(t *testing.T) {
	file := loadSHP(t)
	r := newPresRig(t, 0)
	fonts := flightTextFonts(t)
	gfx := testGfx()
	imm := map[int]*Overlay{}
	for _, sc := range []int{2, 3} {
		ov, err := NewOverlayText(fonts, testCatalog(), gfx, sc)
		if err != nil {
			t.Fatal(err)
		}
		imm[sc] = ov
	}
	p2, err := NewPresenter(PresenterConfig{Fonts: fonts, Catalog: testCatalog(), GfxCatalog: gfx, Scales: []int{2, 3}, routines: testRoutines})
	if err != nil {
		t.Fatal(err)
	}
	p2.Attach(r.so) // 換成有 gfx catalog 的呈現層（同一個觀察器）
	r.p = p2
	type step struct {
		ev    *ArtEvent
		clear bool
		inWin bool // 事件在開著的 2D 窗內
	}
	k0, k2, k15, k17 := mkEvent(t, file, 0, 80, 30, 1), mkEvent(t, file, 2, 134, 127, 2), mkEvent(t, file, 15, 100, 80, 3), mkEvent(t, file, 17, 118, 105, 4)
	k0b := mkEvent(t, file, 0, 80, 30, 5)
	seq := []step{{ev: &k0}, {ev: &k2}, {clear: true}, {ev: &k0}, {ev: &k2, inWin: true}, {ev: &k15}, {ev: &k17}, {ev: &k0b}}
	apply := func(s step) {
		for _, ov := range imm {
			if s.clear {
				ov.ClearArt()
			} else {
				ov.AddDialogArt(*s.ev)
			}
		}
	}
	check := func(where string) {
		for _, sc := range []int{2, 3} {
			a, b := fmt.Sprint(imm[sc].ArtKeys()), fmt.Sprint(r.p.ovs[sc].ArtKeys())
			if a != b || fmt.Sprint(imm[sc].Art()) != fmt.Sprint(r.p.ovs[sc].Art()) {
				t.Fatalf("%s %d 倍：即時套用 %s，完成點套用 %s", where, sc, a, b)
			}
		}
	}
	lo := r.so.Labels
	for i, s := range seq {
		if s.inWin {
			// 窗內的事件：窗還開著時的取像不套用它。
			r.v.ss, r.v.sp = tSS, 0xF000
			r.at(t2dCS, t2dEntry)
			lo.OnArt(*s.ev)
			r.p.capture(r.v, PresShape, 0, 0, nil, nil, nil)
			for _, sc := range []int{2, 3} {
				if fmt.Sprint(r.p.ovs[sc].ArtKeys()) != fmt.Sprint(imm[sc].ArtKeys()) {
					t.Fatalf("第 %d 項：所屬窗未關閉的項目被套用了", i)
				}
			}
			r.write(0, 1, t2dCS, t2dEntry+4, machine.VWV86)
			r.at(t2dCS, t2dRet)
			apply(s)
			check(fmt.Sprintf("第 %d 項（窗內，關窗取像後）", i))
			continue
		}
		if s.clear {
			lo.OnClearArt(r.v.steps)
		} else {
			lo.OnArt(*s.ev)
		}
		apply(s)
		r.at(0x1234, 0x0010) // 沒有同一步的 blit 窗：ClearArt 沒有所屬窗
		r.p.capture(r.v, PresShape, 0, 0, nil, nil, nil)
		check(fmt.Sprintf("第 %d 項", i))
	}
	if len(r.p.queue) != 0 {
		t.Fatalf("佇列剩 %d 項", len(r.p.queue))
	}
}

// ---- U6、U23、N27、N30、N34：CU 與 BufCU ----

func checkCU(t *testing.T, mut presentMut) []string {
	var bad []string
	fail := func(f string, a ...any) { bad = append(bad, fmt.Sprintf(f, a...)) }
	r := newPresRig(t, mut)
	fb := r.fb()
	// 1. 本次候選（倒序列表）：BufCU 照列表讀滿 200 列。
	r.inject(r.text("GEAR", 10, 0xFFFF, tList2))
	r.cuIn(ipCUDiff, 1, tCUSP)
	if r.count(PresCU) != 1 {
		fail("被接受的入口沒有取 BufCU（cu %d）", r.count(PresCU))
		return bad
	}
	for y := 0; y < 200; y++ {
		if !bytes.Equal(r.p.cur.indexed[y*320:(y+1)*320], fb[(199-y)*320:(200-y)*320]) {
			fail("BufCU 第 %d 列不是列表指的第 %d 列", y, 199-y)
			break
		}
	}
	if flightKeys(r.p.cur.flight) != "GEAR;" {
		fail("CU 入口的飛行集合 %s，要候選 GEAR", flightKeys(r.p.cur.flight))
	}
	// 重複入口、CH ≠ 0 不取像。
	r.cuIn(ipCUDiff, 1, tCUSP)
	if r.count(PresCU) != 1 || r.p.counts.CUDuplicate != 1 {
		fail("重複入口取了像或沒計數：cu %d、dup %d", r.count(PresCU), r.p.counts.CUDuplicate)
	}
	// 返回：VGA 等於 BufCU（模擬 CU 照倒序列表搬上 VGA）→ 不計 CUResidue、不另取。
	copy(r.v.vga, r.p.cur.indexed)
	r.cuRet(ipCUDiffRet, tCUSP)
	r.at(0x1234, 0x0010)
	if c := r.p.counts; c.CUResidue != 0 || r.count(PresCUReturn) != 0 {
		fail("VGA 等於 BufCU 卻計 CUResidue %d、cu-return %d", c.CUResidue, r.count(PresCUReturn))
	}
	if !r.p.fbDesc.valid || r.p.fbDesc.list != tList2 {
		fail("比對相等、Seg = FFFF 的描述元沒有成為退用描述元：%+v", r.p.fbDesc)
	}
	r.frame(nil) // 升格

	// 2. 本次候選非空時用本次候選的描述元（不用較早的退用描述元）。
	r.inject(r.text("FLAPS", 15, 0xFFFF, qList))
	r.cuIn(ipCUDiff, 1, tCUSP)
	if !bytes.Equal(r.p.cur.indexed, fb) {
		fail("本次候選非空時沒有用本次候選的描述元（qList）")
	}
	// CU 外直寫一塊 → 返回後計 CUResidue 並另取 cu-return，其色號等於 VGA。
	r.copyFB()
	r.write(5*320+5, 99, 0x5555, 0x0001, machine.VWV86)
	r.cuRet(ipCUDiffRet, tCUSP)
	r.at(0x1234, 0x0010)
	if c := r.p.counts; c.CUResidue != 1 || c.CUResidueFallbackDesc != 0 {
		fail("直寫後 CUResidue %d、FallbackDesc %d，要 1、0", c.CUResidue, c.CUResidueFallbackDesc)
	}
	if r.count(PresCUReturn) != 1 || !bytes.Equal(r.p.cur.indexed, r.v.vga) {
		fail("CUResidue 後沒有 cu-return 呈現像或色號不等於 VGA（cu-return %d）", r.count(PresCUReturn))
	}
	if flightKeys(r.p.cur.flight) != "FLAPS;" {
		fail("cu-return 的飛行集合 %s，要該 CU 的返回快照 FLAPS", flightKeys(r.p.cur.flight))
	}
	r.frame(nil)

	// 3. 本次候選為空：用退用描述元（倒序列表）讀 BufCU，不走 cu-return。
	cuBefore, retBefore := r.count(PresCU), r.count(PresCUReturn)
	r.cuIn(ipCUDiff, 1, tCUSP)
	if r.count(PresCU) != cuBefore+1 || r.p.counts.CUFallbackDescUsed != 1 {
		fail("本次候選為空時沒有用退用描述元取 BufCU（cu %d→%d、用退用 %d）", cuBefore, r.count(PresCU), r.p.counts.CUFallbackDescUsed)
	} else if !bytes.Equal(r.p.cur.indexed[:320], fb[199*320:]) {
		fail("退用描述元讀的 BufCU 第 0 列不是倒序列表的第 199 列")
	}
	// 返回時 VGA 是正序（≠ 退用描述元讀的 BufCU）→ CUResidueFallbackDesc 並改 cu-return。
	r.copyFB()
	r.cuRet(ipCUDiffRet, tCUSP)
	r.at(0x1234, 0x0010)
	if c := r.p.counts; c.CUResidueFallbackDesc != 1 {
		fail("退用描述元的 BufCU 與 VGA 不同，CUResidueFallbackDesc %d，要 1", c.CUResidueFallbackDesc)
	}
	if r.count(PresCUReturn) != retBefore+1 {
		fail("CUResidueFallbackDesc 後沒有 cu-return 呈現像")
	}
	r.frame(nil)

	// 4. 模式設定清除退用描述元 → 本次候選為空、沒有合格描述元 → cu-return。
	r.p.VideoModeChange(machine.ModeChange{})
	if r.p.fbDesc.valid {
		fail("模式設定後退用描述元沒有清除")
	}
	cuBefore, retBefore = r.count(PresCU), r.count(PresCUReturn)
	r.cuIn(ipCUDiff, 1, tCUSP)
	r.copyFB()
	r.cuRet(ipCUDiffRet, tCUSP)
	r.at(0x1234, 0x0010)
	if r.count(PresCU) != cuBefore || r.count(PresCUReturn) != retBefore+1 || !bytes.Equal(r.p.cur.indexed, r.v.vga) {
		fail("沒有合格描述元時要走 cu-return：cu %d→%d、cu-return %d→%d", cuBefore, r.count(PresCU), retBefore, r.count(PresCUReturn))
	}
	r.frame(nil)

	// 5. Seg ≠ FFFF 的描述元比對相等也不成為退用描述元。
	r.v.w16(qDS, 0xEC00, 0) // 段 8000 的列表：每列都指 8000:0000（內容全 0）
	r.inject(r.text("BRAKE", 20, 0x8000, 0xEC00))
	r.cuIn(ipCUDiff, 1, tCUSP)
	copy(r.v.vga, r.p.cur.indexed)
	r.cuRet(ipCUDiffRet, tCUSP)
	r.at(0x1234, 0x0010)
	if r.p.fbDesc.valid {
		fail("Seg ≠ FFFF 的描述元成為退用描述元")
	}
	r.frame(nil)

	// 6. base 改變清除退用描述元。
	r.inject(r.text("GEAR", 10, 0xFFFF, qList))
	r.cuIn(ipCUDiff, 1, tCUSP)
	r.copyFB()
	r.cuRet(ipCUDiffRet, tCUSP)
	r.at(0x1234, 0x0010)
	r.frame(nil)
	if !r.p.fbDesc.valid {
		fail("相等的 FFFF 描述元沒有成為退用描述元")
	}
	r.w.o.base = tpBase + 0x10
	r.w.o.cu.SetBase(tpBase+0x10, true)
	installFlightSigs(r.v.fv, tpBase+0x10)
	r.v.cx, r.v.ss, r.v.sp = 1, tSS, tCUSP
	r.at(tpBase+0x10+segFJemm, ipCUDiff)
	if r.p.fbDesc.valid || r.p.counts.CUNoDesc == 0 {
		fail("base 改變後退用描述元沒有清除")
	}
	return bad
}

func TestU6U23CU(t *testing.T) {
	for _, b := range checkCU(t, presMutEnv(t)) {
		t.Error(b)
	}
}

func TestN27N30N34Detected(t *testing.T) {
	for _, m := range []presentMut{presMutN27, presMutN30a, presMutN30b, presMutN34} {
		if len(checkCU(t, m)) == 0 {
			t.Errorf("突變 %s 沒有被 U6／U23 抓到", m)
		}
	}
}

// 作廢、無窗返回、CH ≠ 0（U6 其餘各條）。
func TestU6VoidAndNoWindow(t *testing.T) {
	r := newPresRig(t, 0)
	r.inject(r.text("GEAR", 10, 0xFFFF, qList))
	r.cuIn(ipCUDiff, 1, tCUSP)
	r.cuIn(ipCUDiff, 1, tCUSP-0x20) // 不同 SS:SP 的新入口：舊的作廢
	if c := r.p.counts; c.CUVoided != 1 || c.CUCapturedNotCopied != 1 {
		t.Fatalf("作廢 %d、CUCapturedNotCopied %d，要 1、1", c.CUVoided, c.CUCapturedNotCopied)
	}
	r.cuRet(ipCUDiffRet, tCUSP-0x20)
	r.at(0x1234, 0x0010)
	r.v.vga[0] = 200 // 與呈現像不同，CU 作廢後的下一次回掃另計
	r.p.staleVoid = true
	r.frame(nil)
	if c := r.p.counts; c.StaleAfterCUVoid != 1 || c.Stale != 0 {
		t.Fatalf("StaleAfterCUVoid %d、Stale %d", c.StaleAfterCUVoid, c.Stale)
	}
	cu := r.count(PresCU)
	r.cuIn(ipCUPlain, 0x0100, tCUSP) // CH ≠ 0
	r.cuRet(ipCUPlainRet, tCUSP)
	if r.count(PresCU) != cu || r.p.counts.CUReturnCHNonZero != 1 || r.p.counts.CUReturnNoWindow != 0 {
		t.Fatalf("CH ≠ 0：cu %d→%d、CHNonZero %d、NoWindow %d", cu, r.count(PresCU), r.p.counts.CUReturnCHNonZero, r.p.counts.CUReturnNoWindow)
	}
	r.cuRet(ipCUDiffRet, 0xF000) // 什麼都對不上
	if r.p.counts.CUReturnNoWindow != 1 {
		t.Fatalf("無窗返回 CUReturnNoWindow %d，要 1", r.p.counts.CUReturnNoWindow)
	}
}

// ---- U28、N33、N35：CU 返回無窗的分類 ----

func checkCUReturnClass(t *testing.T, mut presentMut) []string {
	var bad []string
	r := newPresRig(t, mut)
	// Delivered：被接受的返回之後，同一 PC、同 SS:SP、送達計數增加。
	r.inject(r.text("GEAR", 10, 0xFFFF, qList))
	r.cuIn(ipCUDiff, 1, tCUSP)
	r.copyFB()
	r.cuRet(ipCUDiffRet, tCUSP)
	r.v.deliv++
	r.cuRet(ipCUDiffRet, tCUSP)
	if c := r.p.counts; c.CUReturnDelivered != 1 || c.CUReturnNoWindow != 0 {
		bad = append(bad, fmt.Sprintf("Delivered %d、NoWindow %d，要 1、0", c.CUReturnDelivered, c.CUReturnNoWindow))
	}
	// ViaPlain：00A5 入口 → 00A7 返回被接受 → 00D8、SP = 00A7 的 SP + 14。
	sp := uint16(tCUSP - 0x40)
	r.cuIn(ipCUPlain, 1, sp)
	r.cuRet(ipCUPlainRet, sp)
	r.cuRet(ipCUDiffRet, sp+cuViaPlainSPDelta)
	if c := r.p.counts; c.CUReturnViaPlain != 1 || c.CUReturnNoWindow != 0 {
		bad = append(bad, fmt.Sprintf("ViaPlain %d、NoWindow %d，要 1、0", c.CUReturnViaPlain, c.CUReturnNoWindow))
	}
	// SS:SP 不符：NoWindow。
	r.cuIn(ipCUPlain, 1, sp)
	r.cuRet(ipCUPlainRet, sp)
	r.cuRet(ipCUDiffRet, sp+12)
	if c := r.p.counts; c.CUReturnNoWindow != 1 {
		bad = append(bad, fmt.Sprintf("SP 差 12：NoWindow %d，要 1", c.CUReturnNoWindow))
	}
	// 其間有新入口：NoWindow。
	r.cuIn(ipCUPlain, 1, sp)
	r.cuRet(ipCUPlainRet, sp)
	r.cuIn(ipCUDiff, 1, tCUSP)
	r.cuRet(ipCUDiffRet, tCUSP)
	r.cuRet(ipCUDiffRet, sp+cuViaPlainSPDelta)
	if c := r.p.counts; c.CUReturnNoWindow != 2 || c.CUReturnViaPlain != 1 {
		bad = append(bad, fmt.Sprintf("其間有新入口：NoWindow %d、ViaPlain %d，要 2、1", c.CUReturnNoWindow, c.CUReturnViaPlain))
	}
	return bad
}

func TestU28CUReturnClass(t *testing.T) {
	for _, b := range checkCUReturnClass(t, presMutEnv(t)) {
		t.Error(b)
	}
}

func TestN33N35Detected(t *testing.T) {
	for _, m := range []presentMut{presMutN33, presMutN35} {
		if len(checkCUReturnClass(t, m)) == 0 {
			t.Errorf("突變 %s 沒有被 U28 抓到", m)
		}
	}
}

// ---- U19、N22、N22b：飛行集合依狀態 ----

func checkU19(t *testing.T, mut presentMut) []string {
	var bad []string
	r := newPresRig(t, mut)
	w2d := func() {
		r.win2d(0xF000, func() { r.write(3*320, 9, t2dCS, t2dEntry+8, machine.VWV86) })
	}
	// 先讓可見集合是 B。
	r.inject(r.text("BRAKE", 20, 0xFFFF, qList))
	r.cuIn(ipCUDiff, 1, tCUSP)
	r.copyFB()
	r.cuRet(ipCUDiffRet, tCUSP)
	r.frame(nil)
	if flightKeys(r.w.o.Visible()) != "BRAKE;" {
		t.Fatalf("前置：可見集合 %s", flightKeys(r.w.o.Visible()))
	}
	// 序列一：CU（候選 A）→ 返回 → 2D 窗返回（尚未升格）：用返回快照 A，不用可見集合 B。
	r.inject(r.text("GEAR", 10, 0xFFFF, qList))
	r.cuIn(ipCUDiff, 1, tCUSP)
	r.copyFB()
	r.cuRet(ipCUDiffRet, tCUSP)
	w2d()
	if r.p.cur.info.Kind != PresShape || flightKeys(r.p.cur.flight) != "GEAR;" {
		bad = append(bad, fmt.Sprintf("序列一：%s 的飛行集合 %s，要返回快照 GEAR", r.p.cur.info.Kind, flightKeys(r.p.cur.flight)))
	}
	r.frame(nil) // 升格 A
	// 序列二：CU（候選 A2）→ 回掃落在返回那一步（升格並清空候選）→ 2D 窗返回：用升格後的可見集合，不為空。
	r.inject(r.text("FLAPS", 15, 0xFFFF, qList))
	r.cuIn(ipCUDiff, 1, tCUSP)
	r.copyFB()
	r.cuRet(ipCUDiffRet, tCUSP)
	r.frame(nil)
	w2d()
	if flightKeys(r.p.cur.flight) != "FLAPS;" {
		bad = append(bad, fmt.Sprintf("序列二：飛行集合 %s，要升格後的 FLAPS", flightKeys(r.p.cur.flight)))
	}
	// 序列三：升格之後再一個 2D 窗：用 Visible()。
	w2d()
	if flightKeys(r.p.cur.flight) != "FLAPS;" {
		bad = append(bad, fmt.Sprintf("序列三：飛行集合 %s，要 Visible() FLAPS", flightKeys(r.p.cur.flight)))
	}
	return bad
}

func TestU19FlightSet(t *testing.T) {
	for _, b := range checkU19(t, presMutEnv(t)) {
		t.Error(b)
	}
}

func TestN22N22bDetected(t *testing.T) {
	for _, m := range []presentMut{presMutN22, presMutN22b} {
		if len(checkU19(t, m)) == 0 {
			t.Errorf("突變 %s 沒有被 U19 抓到", m)
		}
	}
}

// ---- U8、U9：Compose 只由呈現像決定；前端狀態 ----

func TestU8ComposeStable(t *testing.T) {
	r := newPresRig(t, 0)
	r.so.Labels.pending = []Label{{Key: "Fly Mission", X0: 100, X1: 150, Y: 100, Font: measuredFont, Kind: KindLabel, Seg: 0xFFFF, List: qList, Steps: r.v.steps}}
	r.blit(0xFC00, false, func() { r.write(100, 3, tpBase+segBlit, 0x74, machine.VWV86) })
	seq := r.p.Current().Seq
	first := map[int][]byte{}
	stamps := map[int]string{}
	for _, sc := range []int{2, 3} {
		c, ok, err := r.p.Compose(sc)
		if !ok || err != nil {
			t.Fatalf("Compose(%d)：%t %v", sc, ok, err)
		}
		first[sc] = append([]byte(nil), c...)
		stamps[sc] = fmt.Sprintf("%+v", r.p.ovs[sc].Stamps())
	}
	same := func(where string) {
		for _, sc := range []int{2, 3} {
			r.p.cache = map[int]struct {
				key  composeCacheKey
				rgba []byte
			}{} // 不靠快取：每次重算
			c, _, _ := r.p.Compose(sc)
			if !bytes.Equal(c, first[sc]) {
				t.Fatalf("%s：%d 倍 Compose 改變", where, sc)
			}
			if s := fmt.Sprintf("%+v", r.p.ovs[sc].Stamps()); s != stamps[sc] {
				t.Fatalf("%s：%d 倍呈現用 Overlay 的 stamp 改變", where, sc)
			}
		}
		if r.p.Current().Seq != seq {
			t.Fatalf("%s：產生了新的呈現像", where)
		}
	}
	same("第一次")
	// 觀察器即時狀態改變（提交、圖面事件、清除），但沒有完成點。
	lo := r.so.Labels
	lo.visible, lo.gen = []Label{{Key: "Talk to Gwen", X0: 10, X1: 90, Y: 50, Font: measuredFont, Kind: KindLabel}}, lo.gen+1
	lo.OnCommit(Commit{Labels: lo.visible, Generation: lo.gen})
	lo.OnArt(ArtEvent{File: ArtSaveLoad, K: 2, X: 134, Y: 127, W: 75, H: 12})
	lo.OnClearArt(r.v.steps)
	r.at(0x1234, 0x0010)
	same("插入提交、圖面事件與清除之後")
	// 回掃（不同的畫面）不改呈現用 Overlay 的 stamp。
	for i := 0; i < 5; i++ {
		idx := make([]byte, 64000)
		for j := range idx {
			idx[j] = uint8(i + j)
		}
		r.frame(idx)
	}
	same("五次回掃之後")
}

func newTestSubs(t *testing.T, fonts *TextFonts) *Subtitles {
	s := &Subtitles{Text: fonts, enabled: true}
	c := &Subtitle{Key: "k1", Start: 0, State: SubShowing, Text: "起落架", Lines: map[int][]SubLine{}}
	for _, sc := range []int{2, 3} {
		lines, _, _, _ := subTextLines([]rune(c.Text), fonts.Font(pxSub[sc]), sc)
		c.Lines[sc] = lines
	}
	s.Subs = []*Subtitle{c}
	return s
}

func TestU9FrontendState(t *testing.T) {
	r := newPresRig(t, 0)
	subs := newTestSubs(t, r.p.cfg.Fonts)
	subs.haveFrame, subs.frameT = true, r.v.steps
	r.p.subs = subs
	r.p.SetSubtitles(false)
	r.blit(0xFC00, false, func() { r.write(100, 3, tpBase+segBlit, 0x74, machine.VWV86) })
	seq := r.p.Current().Seq
	if r.p.cur.subPtr == nil || r.p.cur.subBlocked {
		t.Fatalf("字幕狀態：%v、阻擋 %t", r.p.cur.subPtr, r.p.cur.subBlocked)
	}
	off2, _, _ := r.p.Compose(2)
	off2 = append([]byte(nil), off2...)
	off3, _, _ := r.p.Compose(3)
	if len(off2) != 640*400*4 || len(off3) != 960*600*4 {
		t.Fatalf("倍率：%d、%d", len(off2), len(off3))
	}
	r.p.SetSubtitles(true)
	on2, _, _ := r.p.Compose(2)
	if bytes.Equal(on2, off2) {
		t.Fatal("切字幕開關後 Compose 沒有改變")
	}
	r.p.SetSubtitles(false)
	again, _, _ := r.p.Compose(2)
	if !bytes.Equal(again, off2) {
		t.Fatal("切回字幕關後 Compose 不等於原本")
	}
	if r.p.Current().Seq != seq {
		t.Fatal("前端狀態改變了 Seq")
	}
}

// ---- U13、U14：色盤與字幕呈現像 ----

func TestU13Palette(t *testing.T) {
	r := newPresRig(t, 0)
	r.blit(0xFC00, false, func() { r.write(100, 3, tpBase+segBlit, 0x74, machine.VWV86) })
	r.write(200, 4, 0x5555, 0x10, machine.VWV86) // 未歸屬寫入：旗標設起
	if !r.p.needFallback {
		t.Fatal("未歸屬寫入沒有設旗標")
	}
	img := r.p.cur
	r.v.pal[7] = [3]uint8{1, 2, 3}
	r.frame(img.indexed)
	if c := r.p.Current(); c.Kind != PresPalette || r.p.cur.palette != r.v.pal || &r.p.cur.indexed[0] != &img.indexed[0] {
		t.Fatalf("只變色盤：%+v", c)
	}
	if !r.p.needFallback {
		t.Fatal("palette 呈現像清掉了未歸屬旗標")
	}
	r.frame(img.indexed)
	if r.count(PresPalette) != 1 {
		t.Fatalf("色盤沒變的回掃又產生 palette 呈現像（%d）", r.count(PresPalette))
	}
}

func TestU14Subtitle(t *testing.T) {
	r := newPresRig(t, 0)
	subs := newTestSubs(t, r.p.cfg.Fonts)
	sub0 := subs.Subs[0]
	subs.Subs = nil
	r.p.subs = subs
	// 呈現像的字幕帶不黑（第 160 列有一點），回掃的畫格字幕帶全黑：阻擋要以呈現像判。
	r.blit(0xFC00, false, func() { r.write(160*320+5, 3, tpBase+segBlit, 0x74, machine.VWV86) })
	black := make([]byte, 64000)
	subs.haveFrame, subs.frameT = true, r.v.steps
	subs.Subs = []*Subtitle{sub0}
	subs.Frame(black, r.v.steps) // 觀察器以回掃畫格判：不阻擋
	r.frame(black)
	if c := r.p.Current(); c.Kind != PresSubtitle || r.p.cur.subPtr != sub0 || !r.p.cur.subBlocked {
		t.Fatalf("字幕條出現：%+v，subPtr %v、阻擋 %t（要以呈現像色號判為阻擋）", c, r.p.cur.subPtr, r.p.cur.subBlocked)
	}
	n := r.count(PresSubtitle)
	r.frame(black)
	if r.count(PresSubtitle) != n {
		t.Fatal("字幕狀態沒變的回掃又產生 subtitle 呈現像")
	}
	sub0.State = SubEnded
	r.frame(black)
	if r.count(PresSubtitle) != n+1 || r.p.cur.subPtr != nil {
		t.Fatal("字幕條結束沒有產生 subtitle 呈現像")
	}
}

// ---- U27：重讀抽查 ----

func TestU27Recheck(t *testing.T) {
	r := newPresRig(t, 0)
	cycle := func() { r.win2d(0xF000, func() { r.write(1, 5, t2dCS, t2dEntry+2, machine.VWV86) }) }
	for i := 0; i < presRecheckEvery; i++ {
		cycle()
	}
	r.at(0x1234, 0x0010)
	if c := r.p.counts; c.RecaptureChecked != 1 || c.RecaptureMismatch != 0 {
		t.Fatalf("第 16 個完成點：比對 %d、不符 %d", c.RecaptureChecked, c.RecaptureMismatch)
	}
	for i := 0; i < presRecheckEvery; i++ {
		cycle()
	}
	r.write(2, 6, 0x5555, 0x10, machine.VWV86) // 插入一筆寫入：不比
	r.at(0x1234, 0x0010)
	if c := r.p.counts; c.RecaptureChecked != 1 || c.RecaptureSkipped != 1 {
		t.Fatalf("插入寫入後：比對 %d、略過 %d", c.RecaptureChecked, c.RecaptureSkipped)
	}
	for i := 0; i < presRecheckEvery; i++ {
		cycle()
	}
	r.v.vga[3] ^= 0xFF // 不經 VideoWrite 的改變：抽查必須抓到
	r.at(0x1234, 0x0010)
	if c := r.p.counts; c.RecaptureMismatch != 1 {
		t.Fatalf("內容改變卻沒有 RecaptureMismatch（%d）", c.RecaptureMismatch)
	}
}

// ---- 陳舊斷言與 N2、N6 的機制（長測試另驗曝光） ----

func TestPresentStale(t *testing.T) {
	r := newPresRig(t, 0)
	r.frame(nil)
	if r.p.counts.StaleNoImage != 1 {
		t.Fatalf("還沒有呈現像的回掃：無呈現像 %d", r.p.counts.StaleNoImage)
	}
	r.win2d(0xF000, func() { r.write(1, 5, t2dCS, t2dEntry+2, machine.VWV86) })
	r.frame(nil)
	if r.p.counts.Stale != 0 {
		t.Fatal("取像之後畫面沒變卻判陳舊")
	}
	r.write(9, 9, 0x5555, 0x10, machine.VWV86) // 未歸屬寫入（分期 (c) 不做退路）
	r.frame(nil)
	if r.p.counts.Stale != 1 || r.p.stale[len(r.p.stale)-1].Diff != 1 {
		t.Fatalf("未被取像的改變：陳舊 %d", r.p.counts.Stale)
	}
	// 回掃落在寫 VGA 的窗內：不檢查。
	r.v.ss, r.v.sp = tSS, 0xF000
	r.at(t2dCS, t2dEntry)
	r.write(10, 9, t2dCS, t2dEntry+2, machine.VWV86)
	checked := r.p.counts.StaleChecked
	r.frame(nil)
	if r.p.counts.StaleChecked != checked {
		t.Fatal("寫 VGA 的窗開著時做了陳舊斷言")
	}
}

func TestN6Mechanism(t *testing.T) {
	r := newPresRig(t, presMutN6)
	r.v.vga[0] = 77 // VGA 是上一格，畫外緩衝是新的
	r.inject(r.text("GEAR", 10, 0xFFFF, qList))
	r.cuIn(ipCUDiff, 1, tCUSP)
	if r.p.cur == nil || r.p.cur.indexed[0] != 77 || flightKeys(r.p.cur.flight) != "GEAR;" {
		t.Fatal("N6：CU 入口應讀 VGA（上一格）配本次候選")
	}
}

func TestN2Mechanism(t *testing.T) {
	r := newPresRig(t, presMutN2)
	r.win2d(0xF000, func() { r.write(1, 5, t2dCS, t2dEntry+2, machine.VWV86) })
	if r.count(PresShape) != 0 {
		t.Fatal("N2：完成點不應取像")
	}
	r.frame(nil)
	if r.count(PresFrame) != 1 {
		t.Fatal("N2：回掃應取像")
	}
}

func TestN9Mechanism(t *testing.T) {
	for _, m := range []presentMut{0, presMutN9} {
		r := newPresRig(t, m)
		r.blit(0xFC00, true, func() { r.write(24*320, 7, tpBase+segBlit, 0x74, machine.VWV86) })
		got := r.p.counts.Unattributed
		if (m == 0) != (got == 0) {
			t.Fatalf("突變 %s：非全螢幕 blit 的寫入未歸屬 %d", m, got)
		}
	}
}

// 呈現層模式下台帳自有 FlightOverlay 在 CU 入口收候選（exposure.go attachPresenter）：CU 入口的呈現像用候選集合，
// 台帳若等升格才收，會把照畫的中文判成露出（第一輪 Q 長測試約 59 個假「其他」）。N1（現行合成）不收。
func TestLedgerTakesCandidatesAtCUEntry(t *testing.T) {
	for _, mut := range []expoMut{0, expoMutN1} {
		r := newPresRig(t, 0)
		cats := flightTestCats()
		var lastMove *Point
		cfg := ReceiptConfig{Fonts: r.p.cfg.Fonts, Catalog: testCatalog(), GfxCatalog: map[string]string{}, FlightCatalogs: &cats,
			Scales: []int{2, 3}, expo: &ExpoConfig{mut: mut}}
		x, err := attachExpo(cfg, r.so, map[int]*Overlay{}, map[int]*FlightOverlay{}, &lastMove)
		if err != nil {
			t.Fatal(err)
		}
		x.attachPresenter(r.p)
		r.inject(r.text("GEAR", 10, 0xFFFF, qList))
		r.cuIn(ipCUDiff, 1, tCUSP)
		got := flightKeys(x.led.flights)
		if mut == 0 && (got != "GEAR;" || r.p.Current().Kind != PresCU) {
			t.Fatalf("呈現層模式：CU 入口之後台帳集合 %q、呈現像 %s，要 GEAR、cu", got, r.p.Current().Kind)
		}
		if mut != 0 && got != "" {
			t.Fatalf("N1：CU 入口之後台帳集合 %q，要等升格", got)
		}
		r.copyFB()
		r.cuRet(ipCUDiffRet, tCUSP)
		r.frame(nil) // 升格：兩種模式都收
		if got := flightKeys(x.led.flights); got != "GEAR;" {
			t.Fatalf("突變 %s：升格後台帳集合 %q", mut, got)
		}
	}
}

func TestParsePresentMut(t *testing.T) {
	m, err := parsePresentMut("n33,n34")
	if err != nil || m != presMutN33|presMutN34 {
		t.Fatalf("%v %v", m, err)
	}
	if _, err := parsePresentMut("n99"); err == nil {
		t.Fatal("未知突變沒有回錯")
	}
}
