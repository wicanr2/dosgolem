package strikecommander

import (
	"strings"
	"testing"
)

// strike_commander 規格 009 §4.1 第 9 到 12、14、15 項。

// testPal 是各色號互不相同的色盤：c → (c, 255−c, c/2)。
func testPal() [256][3]uint8 {
	var p [256][3]uint8
	for c := range p {
		p[c] = [3]uint8{uint8(c), uint8(255 - c), uint8(c / 2)}
	}
	return p
}

func (w *world) indexed() []byte {
	idx := make([]byte, 64000)
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			idx[y*320+x] = w.getPx(x, y)
		}
	}
	return idx
}

func rgbAt(out []byte, scale, x, y int) [3]uint8 {
	i := 4 * (y*320*scale + x)
	return [3]uint8{out[i], out[i+1], out[i+2]}
}

// drawFlight 以世界目前的畫外緩衝當畫格出圖。
func drawFlight(t *testing.T, w *world, scale int) ([]byte, []byte, *FlightOverlay) {
	t.Helper()
	fo, err := NewFlightOverlayText(flightTextFonts(t), flightTestCats(), scale)
	if err != nil {
		t.Fatal(err)
	}
	fo.Sync(w.o.Visible(), w.o.Generation())
	idx := w.indexed()
	out := ScaleIndexedRGBA(idx, testPal(), scale)
	plain := append([]byte(nil), out...)
	fo.Draw(out, idx, testPal())
	if cc := CheckCompositeText(out, plain, scale, nil, nil, fo.TextBoxes(), fo.Covers()...); cc.Outside != 0 {
		t.Fatalf("%d 倍：允許區外差異 %d", scale, cc.Outside)
	}
	return out, idx, fo
}

// ---- 第 9 項：背景、字色、外框 ----

func TestFlightBackgroundOutline(t *testing.T) {
	pal := testPal()
	w := newWorld(t, 0x0241)
	w.fillBand(0, 100, 30) // 3D 場景
	w.hud("GEAR", 190, 10)
	w.hud("FLAPS", 190, 15)
	w.hud("BRAKE", 190, 20)
	w.hud("000 NONE", 118, 79)
	w.cuFrame()
	out, idx, fo := drawFlight(t, w, 3)
	if len(fo.entries) != 4 || len(fo.Hidden) != 0 {
		t.Fatalf("筆數 %d、Hidden %v", len(fo.entries), fo.Hidden)
	}
	rep := fo.Report()
	for _, e := range rep {
		if e.FG != colorHUD || !e.Outline || e.Px != 15 {
			t.Errorf("%s：fg %d（F9）、外框 %t（F7）、px %d", e.Key, e.FG, e.Outline, e.Px)
		}
	}
	// 字形（測試圖集方塊字，15 像素、asc 13）：每字填色欄 [tx + 1, tx + 14)、列 [baseline − 12, baseline + 1) 為字色；
	// 外框層是填色四周各加 1，左上角 (tx, baseline − 13) 與下緣 baseline + 1 是 0 號色。四行都完整（外框不碰鄰行字形）。
	for _, e := range rep {
		n := len([]rune(e.Display))
		for i := 0; i < n; i++ {
			x0 := e.TxOut + 15*i
			for y := e.Baseline - 12; y <= e.Baseline; y++ {
				for x := x0 + 1; x < x0+14; x++ {
					if c := rgbAt(out, 3, x, y); c != pal[colorHUD] {
						t.Fatalf("%s 第 %d 字 (%d,%d) 是 %v", e.Key, i, x, y, c)
					}
				}
			}
		}
		if c := rgbAt(out, 3, e.TxOut, e.Baseline-13); c != pal[0] {
			t.Errorf("%s 對角外框點 %v，要 0 號色", e.Key, c)
		}
		if c := rgbAt(out, 3, e.TxOut+5, e.Baseline+1); c != pal[0] {
			t.Errorf("%s 下外框 %v，要 0 號色", e.Key, c)
		}
	}
	// 背景換回前列（F6）：FLAPS 的遮蓋內、文字框外，原版放大圖有英文墨跡（≠ 30）的點，輸出都是 30。
	plain := ScaleIndexedRGBA(idx, pal, 3)
	fl := rep[1]
	if fl.Key != "FLAPS" {
		t.Fatalf("第 2 筆是 %s", fl.Key)
	}
	replaced := 0
	for y := fl.Y0 * 3; y < fl.Y1*3; y++ {
		for x := fl.X0 * 3; x < fl.X1*3; x++ {
			if x >= fl.Ink.X0 && x < fl.Ink.X1 && y >= fl.Ink.Y0 && y < fl.Ink.Y1 {
				continue
			}
			if c := rgbAt(out, 3, x, y); c != pal[30] {
				t.Fatalf("FLAPS 遮蓋內、文字框外 (%d,%d) 是 %v，要 30", x, y, c)
			}
			if rgbAt(plain, 3, x, y) != pal[30] {
				replaced++
			}
		}
	}
	if replaced == 0 {
		t.Fatal("FLAPS 遮蓋內、文字框外沒有英文墨跡，換回無鑑別力")
	}
	// NONE 的數字墨跡保留（F8）：(119, 81) 仍是墨跡色。
	if c := rgbAt(out, 3, 119*3+1, 81*3+1); c != pal[colorHUD] {
		t.Errorf("000 NONE 的數字墨跡被改：%v", c)
	}
}

func TestFlightNoOutlineMFDPlaque(t *testing.T) {
	pal := testPal()
	w := newWorld(t, 0x0241)
	w.fillBand(0, 200, 217)
	w.hud("GEAR", 190, 10) // 啟用 base
	w.puts(putsOpt{kind: KindMFD, s: "NORM", x: 222, y: 126, color: 220})
	w.fillBand(2, 11, 28)
	w.plaque(plaqueOpt{s: "time burst", x: 125, y: 4, color: 5})
	w.cuFrame()
	for _, sc := range []int{2, 3} {
		out, _, fo := drawFlight(t, w, sc)
		for _, e := range fo.Report() {
			if e.Kind == KindHUD {
				continue
			}
			if e.Outline {
				t.Errorf("%d 倍 %s 有外框旗標（F17）", sc, e.Key)
			}
			for y := e.Y0 * sc; y < e.Y1*sc; y++ {
				for x := e.X0 * sc; x < e.X1*sc; x++ {
					if rgbAt(out, sc, x, y) == pal[0] {
						t.Fatalf("%d 倍 %s 在 (%d,%d) 有 0 號色（F17）", sc, e.Key, x, y)
					}
				}
			}
			want := map[string]uint8{"NORM": 220, "time burst": 5}[e.Key]
			if e.FG != want {
				t.Errorf("%s fg %d，要 %d", e.Key, e.FG, want)
			}
		}
	}
}

func TestInkModeTie(t *testing.T) {
	tx := FlightText{BH: 1, Before: make([]byte, 320), After: make([]byte, 320)}
	for _, x := range []int{10, 11, 12} {
		tx.After[x] = 20
	}
	for _, x := range []int{13, 14, 15} {
		tx.After[x] = 10
	}
	if fg, n := inkMode(tx, 0, 320); fg != 10 || n != 6 {
		t.Fatalf("同數取小（F29）：fg %d n %d", fg, n)
	}
	tx.After[16] = 20
	if fg, _ := inkMode(tx, 0, 320); fg != 20 {
		t.Fatalf("眾數：fg %d", fg)
	}
}

// ---- 第 10 項：出圖比對 ----

func TestFlightMismatch(t *testing.T) {
	w := newWorld(t, 0x0241)
	w.fillBand(0, 200, 30)
	w.hud("GEAR", 190, 10)
	w.hud("FLAPS", 190, 15)
	w.puts(putsOpt{kind: KindMFD, s: "NORM", x: 222, y: 126, color: 220})
	w.cuFrame()
	fo, err := NewFlightOverlayText(flightTextFonts(t), flightTestCats(), 3)
	if err != nil {
		t.Fatal(err)
	}
	fo.Sync(w.o.Visible(), w.o.Generation())
	idx := w.indexed()
	// 畫面未變：都不 Hidden（MFD 的 b0 = Y − 1，列位移錯會在這裡不符）。
	if bad := fo.Match(idx); len(bad) != 0 {
		t.Fatalf("畫面未變仍不符：%v", bad)
	}
	// band 內、遮蓋範圍外改一點：不 Hidden（F3）。
	idx2 := append([]byte(nil), idx...)
	idx2[12*320+250] = 99
	if bad := fo.Match(idx2); len(bad) != 0 {
		t.Fatalf("遮蓋範圍外改一點就不符：%v", bad)
	}
	// 遮蓋範圍內改一點：只有該筆 Hidden（F16）。
	idx2[15*320+195] = 99
	out := ScaleIndexedRGBA(idx2, testPal(), 3)
	fo.Draw(out, idx2, testPal())
	if !eqStrs(fo.Hidden, []string{"GEAR"}) {
		t.Fatalf("Hidden %v，要 [GEAR]", fo.Hidden)
	}
	rep := fo.Report()
	if !rep[0].Hidden || rep[1].Hidden || rep[2].Hidden {
		t.Fatalf("Report 的 Hidden：%t %t %t", rep[0].Hidden, rep[1].Hidden, rep[2].Hidden)
	}
}

// ---- 第 11 項：墨跡框核對 ----

func TestFlightInkBox(t *testing.T) {
	w := newWorld(t, 0x0241)
	w.puts(putsOpt{kind: KindHUD, s: "GEAR", x: 190, y: 10, color: colorHUD, extraInk: &Point{240, 12}})
	if len(w.o.Pending()) != 0 || w.o.Counts().SkipInk != 1 {
		t.Fatalf("框外差異仍收（F31）：%d", len(w.o.Pending()))
	}
	w.fillBand(11, 16, 0)
	w.puts(putsOpt{kind: KindHUD, s: "FLAPS", x: 190, y: 15, color: colorHUD, noInk: true})
	if len(w.o.Pending()) != 0 || w.o.Counts().SkipInk != 2 {
		t.Fatalf("沒有差異仍收：%d", len(w.o.Pending()))
	}
	// 只在字中間畫一點：缺上緣、左緣、右緣，仍通過（inkInBox，不用 InkResult.OK）。
	w.puts(putsOpt{kind: KindHUD, s: "BRAKE", x: 190, y: 20, color: colorHUD,
		inkFn: func(i, cx int) { w.setPx(cx+1, 23, colorHUD) }})
	p := w.o.Pending()
	if len(p) != 1 || p[0].Ink.Top || p[0].Ink.Left || p[0].Ink.OK() {
		t.Fatalf("缺三緣：待提交 %d、%s", len(p), p[0].Ink)
	}
	// 第 5 版（§3.2.4.5）：三例都是舊路徑，Clip 為 nil、第 5 版計數全 0、log 沒有 clip、inkfb。
	if p[0].Clip != nil {
		t.Fatalf("舊路徑的 Clip 不是 nil：%+v", *p[0].Clip)
	}
	checkV5Quiet(t, w.o, "第 11 項三例")
	// 前列取錯時點（F5）：MFD 底在 HUD 字之後才畫（RE 021 §3.2），以 MFD 自己入口時的緩衝當前列才只含自己的墨跡。
	w2 := newWorld(t, 0x0241)
	w2.fillBand(0, 200, 30)
	w2.hud("GEAR", 190, 10)
	for y := 116; y <= 188; y++ {
		for x := 219; x <= 305; x++ {
			w2.setPx(x, y, 217)
		}
	}
	w2.puts(putsOpt{kind: KindMFD, s: "NORM", x: 222, y: 126, color: 220})
	if p := w2.o.Pending(); len(p) != 2 || w2.o.Counts().SkipInk != 0 {
		t.Fatalf("MFD 底之後的字串：待提交 %v、核對不通過 %d", keysOf(p), w2.o.Counts().SkipInk)
	}
}

// ---- 第 12 項：SessionObserver 層 ----

func TestFlightSessionObserver(t *testing.T) {
	cats := flightTestCats()
	cfg := ReceiptConfig{Scales: []int{2, 3}, Fonts: flightTextFonts(t), FlightCatalogs: &cats}
	res := &ReceiptResult{}
	so := &SessionObserver{Labels: NewLabelObserver()}
	fovs := map[int]*FlightOverlay{}
	if err := attachFlight(cfg, res, so, fovs); err != nil {
		t.Fatal(err)
	}
	w := newWorld(t, 0x0241)
	w.o = so.Flight
	var gotSteps uint64
	var lastAtCall []byte
	calls := 0
	orig := so.OnFlightFrame
	so.OnFlightFrame = func(ix []byte, st uint64) {
		calls++
		gotSteps, lastAtCall = st, so.LastIndexed
		orig(ix, st)
	}
	pal := testPal()
	frame := func() []byte {
		idx := w.indexed()
		w.v.steps += 100
		so.steps = w.v.steps
		so.Frame(idx, pal)
		return idx
	}
	w.fillBand(0, 100, 30)
	w.hud("GEAR", 190, 10)
	w.cu()
	idx1 := frame()
	if v := so.Flight.Visible(); len(v) != 1 || v[0].Key != "GEAR" || &so.LastIndexed[0] != &idx1[0] {
		t.Fatalf("第一次升格：%v", keysOf(v))
	}
	w.fillBand(11, 16, 30)
	w.hud("FLAPS", 190, 15)
	w.cu()
	// CU 入口與返回之後、Frame 之前：出圖用的 Visible() 仍是舊集合（F2）。
	if v := so.Flight.Visible(); len(v) != 1 || v[0].Key != "GEAR" {
		t.Fatalf("CU 返回後、Frame 之前 Visible %v，要 [GEAR]", keysOf(v))
	}
	idx2 := frame()
	if v := so.Flight.Visible(); len(v) != 1 || v[0].Key != "FLAPS" || &so.LastIndexed[0] != &idx2[0] {
		t.Fatalf("Frame 之後 Visible %v", keysOf(v))
	}
	if calls != 2 || gotSteps != so.steps || &lastAtCall[0] != &idx2[0] {
		t.Fatalf("OnFlightFrame：%d 次、步數 %d（要 %d）、LastIndexed 已更新 %t", calls, gotSteps, so.steps, &lastAtCall[0] == &idx2[0])
	}
	// CU 進行中的 Frame：照合成畫面計數（FLAPS 的遮蓋範圍內改一點，3 倍 1 筆、2 倍沒有 HUD stamp）。
	w.hud("BRAKE", 190, 20)
	w.cuIn(ipCUDiff, 1, tCUSP)
	w.setPx(195, 18, 99)
	frame()
	if calls != 3 || len(res.FlightCUFrames) != 1 || res.FlightCUFrames[0].Mismatch[3] != 1 || res.FlightCUFrames[0].Mismatch[2] != 0 {
		t.Fatalf("CU 期間的 Frame：%d 次、紀錄 %+v", calls, res.FlightCUFrames)
	}
	if c := so.Flight.Counts(); c.CUFrames != 1 || c.CUFrameMismatch != 1 {
		t.Fatalf("CU 期間回掃 %d、不符 %d", c.CUFrames, c.CUFrameMismatch)
	}
	// 升格後第一個 Frame 被擋下：返回後讓 BRAKE 的遮蓋範圍與 Ref 不同。
	w.cuRet(ipCUDiffRet, tCUSP)
	w.setPx(195, 23, 98)
	frame()
	if c := so.Flight.Counts(); c.FirstFrameBlocked != 1 || len(res.FlightFirstBlocked) != 3 {
		t.Fatalf("升格後第一個 Frame 被擋下 %d、紀錄 %v", c.FirstFrameBlocked, res.FlightFirstBlocked)
	}
}

// ---- 第 14 項：lint ----

func errHas(errs []error, sub string) bool {
	for _, e := range errs {
		if strings.Contains(e.Error(), sub) {
			return true
		}
	}
	return false
}

const goodHUD = "key\ttranslation\tsource\nGEAR\t起落架\ts\nFLAPS\t襟翼\ts\nBRAKE\t煞車\ts\nDGFT\t纏鬥\ts\nSTRF\t掃射\ts\nSRM\t短程飛彈\ts\nNONE\t無\ts\n"

const goodMFD = "key\ttranslation\tsource\nComm Mode:\t通訊模式：\ts\nSelect Frequency\t選擇頻道\ts\nNO RECEIVERS\t無人接收\ts\n" +
	"request status\t回報狀態\ts\nNORM\t正常\ts\nGUN\t機砲\ts\nFUEL:\t燃油：\ts\nAIR\t對空\ts\nGND\t對地\ts\n"

// TestLintFlight：LintFlightText（011 §3.7）的結構、glossary、字元與寬度規則。
func TestLintFlight(t *testing.T) {
	fonts := flightTextFonts(t)
	// FUEL: 的 glossary 用半形冒號：全形 ASCII 正規化後與 catalog 的「燃油：」相同。
	gl := []GlossaryRow{{"GUN", "機砲", "攻略無，自譯", ""}, {"FUEL:", "燃油:", "x", ""}, {"AIR", "空中", "x", "語境不同"}, {"GEAR", "起落架", "x", ""}}
	if errs := LintFlightText(KindHUD, []byte(goodHUD), fonts, gl); len(errs) != 0 {
		t.Fatalf("hud：%v", errs)
	}
	if errs := LintFlightText(KindMFD, []byte(goodMFD), fonts, gl); len(errs) != 0 {
		t.Fatalf("mfd（FUEL: 全形冒號與 glossary 半形正規化後相同、AIR 語境不同）：%v", errs)
	}
	rep := func(s, old, new string) []byte { return []byte(strings.Replace(s, old, new, 1)) }
	for _, tc := range []struct {
		name string
		kind Kind
		b    []byte
		want string
	}{
		{"欄名", KindHUD, rep(goodHUD, "key\t", "k\t"), "欄名"},
		{"欄數", KindHUD, rep(goodHUD, "GEAR\t起落架\ts", "GEAR\t起落架"), "欄數"},
		{"key 重複", KindHUD, []byte(goodHUD + "GEAR\t起落架\ts\n"), "重複"},
		{"空譯文", KindHUD, rep(goodHUD, "GEAR\t起落架", "GEAR\t"), "空的"},
		{"孤兒 key", KindHUD, []byte(goodHUD + "LOCK\t無\ts\n"), "孤兒"},
		{"漏譯", KindHUD, rep(goodHUD, "FLAPS\t襟翼\ts\n", ""), "漏譯"},
		{"數字開頭", KindHUD, []byte(goodHUD + "000 NONE\t無\ts\n"), "數字前綴"},
		{"武器名", KindHUD, []byte(goodHUD + "999 Vulcan\t無\ts\n"), "武器名"},
		// NONE 以 Xs[k] = 134 計：可用 (149 − 134 + 1) × 3 = 48；4 字 60 + 2 超過。
		{"超寬", KindHUD, rep(goodHUD, "NONE\t無", "NONE\t無無無無"), "超過"},
		{"半形字", KindHUD, rep(goodHUD, "GEAR\t起落架", "GEAR\t起落架a"), "半形"},
		{"缺字", KindHUD, rep(goodHUD, "GEAR\t起落架", "GEAR\t起落乂"), "缺字"},
		{"與 glossary 不一致", KindHUD, rep(goodHUD, "GEAR\t起落架", "GEAR\t起落"), "glossary"},
	} {
		if errs := LintFlightText(tc.kind, tc.b, fonts, gl); !errHas(errs, tc.want) {
			t.Errorf("%s：沒有報「%s」：%v", tc.name, tc.want, errs)
		}
	}
	if _, err := ParseGlossaryRows([]byte("en\tzh\tbasis\tnote\n")); err == nil {
		t.Error("ParseGlossaryRows 接受欄名不符")
	}
	if _, err := ParseGlossaryRows([]byte(glossaryHeader + "\nA\tB\tC\n")); err == nil {
		t.Error("ParseGlossaryRows 接受 3 欄")
	}
	if rows, err := ParseGlossaryRows([]byte(glossaryHeader + "\nA\tB\tC\tD\n")); err != nil || len(rows) != 1 || rows[0].Note != "D" {
		t.Errorf("ParseGlossaryRows：%v %v", rows, err)
	}
}

// ---- 第 15 項：升格統計 ----

func TestFlightFirstFrameStats(t *testing.T) {
	o := NewFlightObserver(nil)
	for _, n := range []int{1, 0, 2} {
		o.NoteFirstFrameBlocked(n)
	}
	if c := o.Counts(); c.FirstFrameBlocked != 3 || c.FirstFrameBlockedMaxRun != 1 {
		t.Fatalf("被擋 %d、最長連續 %d", c.FirstFrameBlocked, c.FirstFrameBlockedMaxRun)
	}
	o.NoteFirstFrameBlocked(1)
	if c := o.Counts(); c.FirstFrameBlockedMaxRun != 2 {
		t.Fatalf("連續兩次：%d", c.FirstFrameBlockedMaxRun)
	}
}

// 暫停序列（§4.1 第 7 項）：CU → MU CX=0 → 直寫一塊壓到某筆遮蓋範圍的像素 → Frame。被壓到的那筆 Hidden，其他照畫。
func TestFlightPauseSequence(t *testing.T) {
	w := newWorld(t, 0x0241)
	w.fillBand(0, 100, 30)
	w.hud("GEAR", 190, 10)
	w.hud("DGFT", 118, 74)
	w.cuIn(ipCUDiff, 1, tCUSP)
	w.cuRet(ipCUDiffRet, tCUSP)
	w.mu(0)
	// 底板 (112,0)-(207,12) 直寫 VGA：畫格上壓到 GEAR band 的列 11、12。
	for y := 0; y <= 12; y++ {
		for x := 112; x <= 207; x++ {
			w.setPx(x, y, 28)
		}
	}
	w.frame()
	_, _, fo := drawFlight(t, w, 3)
	if !eqStrs(fo.Hidden, []string{"GEAR"}) || len(fo.entries) != 2 {
		t.Fatalf("暫停：Hidden %v（要 [GEAR]，F25 會讓 DGFT 也不畫）、%d 筆", fo.Hidden, len(fo.entries))
	}
}
