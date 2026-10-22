package strikecommander

import (
	"reflect"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// strike_commander 規格 012 §3.8.3 分期 (e) 的驗收：composeScale 與抽出前兩段程式（refShotScales、refStopScales，
// 自 6cede04 的 receipt.go:587–617、657–678 照抄）以同一輸入比較全部輸出，含 Flight 為 nil 的缺鍵。

// refShotScales 是 6cede04 receipt.go:587–617（截圖點）的原文，只把迴圈包成函式。
func refShotScales(cfg ReceiptConfig, so *SessionObserver, sr *ShotResult, overlays map[int]*Overlay, fovs map[int]*FlightOverlay, flight *FlightObserver) {
	for _, sc := range cfg.Scales {
		ov := overlays[sc]
		ov.Sync(so.Labels.Visible(), so.Labels.Generation())
		comp, _ := ov.Draw(so.LastIndexed, so.LastPalette)
		plain := ScaleIndexedRGBA(so.LastIndexed, so.LastPalette, sc)
		sr.Composites[sc], sr.Plain[sc] = comp, plain
		// 009 §3.11 接點第 4 條：Overlay.Draw → fov.Sync、fov.Draw → CheckComposite → 010 的字幕。
		var covers []FlightCover
		boxes := ov.TextBoxes()
		if fov := fovs[sc]; fov != nil {
			fov.Sync(flight.Visible(), flight.Generation())
			fov.Draw(comp, so.LastIndexed, so.LastPalette)
			covers = fov.Covers()
			boxes = append(boxes, fov.TextBoxes()...)
			sr.FlightHidden[sc], sr.FlightCovers[sc], sr.FlightReport[sc] = append([]string(nil), fov.Hidden...), covers, fov.Report()
		}
		sr.Check[sc] = CheckCompositeText(comp, plain, sc, ov.Stamps(), ov.Art(), boxes, covers...)
		sr.ArtKeys[sc] = ov.ArtKeys()
		sr.Hidden[sc] = append([]string(nil), ov.Hidden...)
		sr.BG[sc] = ov.BGReport()
		sr.Covers[sc] = map[string][4]int{}
		for _, s := range ov.Stamps() {
			if s.Font == nil {
				sr.Covers[sc][s.Key] = [4]int{s.X, s.Y, s.X + s.Cells*s.CellW, s.Y + s.CellH}
			}
		}
		if sr.Arts == nil {
			sr.Arts = map[int][]ArtReportEntry{}
		}
		sr.Arts[sc] = ov.ArtReport() // 011 §3.8 arts.tsv
	}
}

// refStopScales 是 6cede04 receipt.go:657–678（停止點）的原文。
func refStopScales(cfg ReceiptConfig, so *SessionObserver, res *ReceiptResult, overlays map[int]*Overlay, fovs map[int]*FlightOverlay, flight *FlightObserver) {
	for _, sc := range cfg.Scales {
		ov := overlays[sc]
		ov.Sync(so.Labels.Visible(), so.Labels.Generation())
		comp, _ := ov.Draw(res.Frame, res.Palette)
		res.BG[sc] = ov.BGReport()
		plain := ScaleIndexedRGBA(res.Frame, res.Palette, sc)
		res.Composites[sc], res.Plain[sc] = comp, plain
		var covers []FlightCover
		boxes := ov.TextBoxes()
		if fov := fovs[sc]; fov != nil {
			fov.Sync(flight.Visible(), flight.Generation())
			fov.Draw(comp, res.Frame, res.Palette)
			covers = fov.Covers()
			boxes = append(boxes, fov.TextBoxes()...)
			res.FlightReport[sc], res.FlightHidden[sc] = fov.Report(), append([]string(nil), fov.Hidden...)
		}
		res.CompositeCheck[sc] = CheckCompositeText(comp, plain, sc, ov.Stamps(), ov.Art(), boxes, covers...)
		if res.StopArts == nil {
			res.StopArts = map[int][]ArtReportEntry{}
		}
		res.StopArts[sc] = ov.ArtReport()
	}
}

// newShotScales、newStopScales 是 RunReceipt 現行的兩段迴圈（receipt.go 截圖點與停止點）。
func newShotScales(cfg ReceiptConfig, so *SessionObserver, sr *ShotResult, overlays map[int]*Overlay, fovs map[int]*FlightOverlay, flight *FlightObserver) {
	for _, sc := range cfg.Scales {
		in := scaleIn{Overlay: overlays[sc], LabelsV: so.Labels.Visible(), LabelsG: so.Labels.Generation(), Scale: sc, Frame: so.LastIndexed, Palette: so.LastPalette}
		if fov := fovs[sc]; fov != nil {
			in.Flight, in.FlightV, in.FlightG = fov, flight.Visible(), flight.Generation()
		}
		storeShotScale(sr, sc, composeScale(in), in.Flight != nil)
	}
}

func newStopScales(cfg ReceiptConfig, so *SessionObserver, res *ReceiptResult, overlays map[int]*Overlay, fovs map[int]*FlightOverlay, flight *FlightObserver) {
	for _, sc := range cfg.Scales {
		in := scaleIn{Overlay: overlays[sc], LabelsV: so.Labels.Visible(), LabelsG: so.Labels.Generation(), Scale: sc, Frame: res.Frame, Palette: res.Palette}
		if fov := fovs[sc]; fov != nil {
			in.Flight, in.FlightV, in.FlightG = fov, flight.Visible(), flight.Generation()
		}
		storeStopScale(res, sc, composeScale(in), in.Flight != nil)
	}
}

// composeFixture 是一組輸入：每次 build 都建新的 Overlay 與 FlightOverlay（兩者有狀態，參考實作與新函式不能共用）。
type composeFixture struct {
	cfg      ReceiptConfig
	so       *SessionObserver
	overlays map[int]*Overlay
	fovs     map[int]*FlightOverlay
	flight   *FlightObserver
}

// composeCase 描述一組輸入的組成。
type composeCase struct {
	name        string
	flight      bool  // 掛 FlightObserver
	fovScales   []int // 有 FlightOverlay 的倍率（flight 為 true 時；其餘倍率缺 fov）
	art         bool  // 加存讀檔圖面（需要 SAVELOAD.SHP）
	hideFlight  bool  // 改畫格一點讓 GEAR 比對不符（Hidden）
	labelHidden bool  // 第二個標籤的 Ref 與畫格不同（Hidden）
}

func buildComposeFixture(t *testing.T, c composeCase) composeFixture {
	t.Helper()
	fonts := flightTextFonts(t)
	w := newWorld(t, 0x0241)
	w.fillBand(0, 200, 30)
	w.hud("GEAR", 190, 10)
	w.hud("FLAPS", 190, 15)
	w.puts(putsOpt{kind: KindMFD, s: "NORM", x: 222, y: 126, color: 220})
	w.cuFrame()
	idx := w.indexed()
	// 標籤：Talk to Gwen 的 Ref 取自畫格（照畫）；Fly Mission 的 Ref 依 labelHidden 取自改過一點的畫格。
	l1 := Label{Key: "Talk to Gwen", X0: 114, X1: 204, Y: 170, Font: measuredFont}
	l2 := Label{Key: "Fly Mission", X0: 20, X1: 108, Y: 40, Font: measuredFont}
	for x := 116; x < 200; x += 7 {
		idx[175*320+x] = 15 // 英文墨跡
	}
	l1.Ref = refFromFrame(idx, l1)
	other := append([]byte(nil), idx...)
	if c.labelHidden {
		other[45*320+50] = 77
	}
	l2.Ref = refFromFrame(other, l2)
	if c.hideFlight {
		idx[15*320+195] = 99
	}
	var file []byte
	if c.art {
		file = loadSHP(t)
		paint(t, idx, file, 0, 80, 30)
	}
	pal := testPal()
	lo := NewLabelObserver()
	lo.visible, lo.gen = []Label{l1, l2}, 3
	f := composeFixture{
		cfg:      ReceiptConfig{Scales: []int{2, 3}},
		so:       &SessionObserver{Labels: lo, LastIndexed: idx, LastPalette: pal},
		overlays: map[int]*Overlay{},
		fovs:     map[int]*FlightOverlay{},
	}
	cats := flightTestCats()
	for _, sc := range f.cfg.Scales {
		ov, err := NewOverlayText(fonts, testCatalog(), testGfx(), sc)
		if err != nil {
			t.Fatal(err)
		}
		if c.art {
			e0 := tElemFor(t, 0)
			ov.AddDialogArt(mkEvent(t, file, 0, e0.x, e0.y, 1))
		}
		f.overlays[sc] = ov
	}
	if c.flight {
		f.flight = w.o
		for _, sc := range c.fovScales {
			fo, err := NewFlightOverlayText(fonts, cats, sc)
			if err != nil {
				t.Fatal(err)
			}
			f.fovs[sc] = fo
		}
	}
	return f
}

func newShotResult(f composeFixture) ShotResult {
	sr := ShotResult{Composites: map[int][]byte{}, Plain: map[int][]byte{}, Check: map[int]CompositeCheck{}, ArtKeys: map[int][]string{}, Hidden: map[int][]string{}, Covers: map[int]map[string][4]int{}, BG: map[int][]BGEntry{}}
	if f.flight != nil {
		sr.FlightHidden, sr.FlightCovers, sr.FlightReport = map[int][]string{}, map[int][]FlightCover{}, map[int][]FlightEntry{}
	}
	return sr
}

func newStopResult(f composeFixture) *ReceiptResult {
	res := &ReceiptResult{}
	if f.flight != nil {
		res.FlightReport, res.FlightHidden = map[int][]FlightEntry{}, map[int][]string{}
	}
	res.Frame, res.Palette = f.so.LastIndexed, f.so.LastPalette
	res.Composites, res.Plain, res.CompositeCheck = map[int][]byte{}, map[int][]byte{}, map[int]CompositeCheck{}
	res.BG = map[int][]BGEntry{}
	return res
}

var composeCases = []composeCase{
	{name: "標籤，無飛行"},
	{name: "標籤 Hidden，無飛行", labelHidden: true},
	{name: "飛行兩倍率", flight: true, fovScales: []int{2, 3}},
	{name: "飛行 Hidden", flight: true, fovScales: []int{2, 3}, hideFlight: true, labelHidden: true},
	{name: "飛行只有 3 倍 fov", flight: true, fovScales: []int{3}},
	{name: "圖面", art: true},
	{name: "圖面與飛行", art: true, flight: true, fovScales: []int{2, 3}, hideFlight: true},
}

// TestComposeScaleMatchesReference：同一輸入下，截圖點與停止點的新舊兩段結果逐欄位相同（reflect.DeepEqual 分得出 nil 與空值、缺鍵與空值）。
func TestComposeScaleMatchesReference(t *testing.T) {
	for _, c := range composeCases {
		t.Run(c.name, func(t *testing.T) {
			// 截圖點。
			a, b := buildComposeFixture(t, c), buildComposeFixture(t, c)
			srA, srB := newShotResult(a), newShotResult(b)
			refShotScales(a.cfg, a.so, &srA, a.overlays, a.fovs, a.flight)
			newShotScales(b.cfg, b.so, &srB, b.overlays, b.fovs, b.flight)
			if !reflect.DeepEqual(srA, srB) {
				diffShot(t, srA, srB)
			}
			// 停止點。
			a, b = buildComposeFixture(t, c), buildComposeFixture(t, c)
			resA, resB := newStopResult(a), newStopResult(b)
			refStopScales(a.cfg, a.so, resA, a.overlays, a.fovs, a.flight)
			newStopScales(b.cfg, b.so, resB, b.overlays, b.fovs, b.flight)
			if !reflect.DeepEqual(resA, resB) {
				t.Errorf("停止點不同：BG %t、Composites %t、CompositeCheck %t、StopArts %t、FlightReport %t、FlightHidden %t",
					reflect.DeepEqual(resA.BG, resB.BG), reflect.DeepEqual(resA.Composites, resB.Composites),
					reflect.DeepEqual(resA.CompositeCheck, resB.CompositeCheck), reflect.DeepEqual(resA.StopArts, resB.StopArts),
					reflect.DeepEqual(resA.FlightReport, resB.FlightReport), reflect.DeepEqual(resA.FlightHidden, resB.FlightHidden))
			}
			// 兩邊的 Overlay 狀態也相同（Hidden、Misses、Skips 由 Sync、Draw 改）。
			for _, sc := range a.cfg.Scales {
				oa, ob := a.overlays[sc], b.overlays[sc]
				if !reflect.DeepEqual(oa.Hidden, ob.Hidden) || !reflect.DeepEqual(oa.Misses, ob.Misses) || !reflect.DeepEqual(oa.Skips, ob.Skips) {
					t.Errorf("%d 倍 Overlay 狀態不同", sc)
				}
			}
			// 輸入要有鑑別力：遮蓋 stamp、差異像素與（掛飛行時）飛行字串都要出現。
			if len(srB.Covers[2]) == 0 || srB.Check[2].Diffs == 0 || srB.Check[3].Diffs == 0 {
				t.Errorf("輸入沒有遮蓋 stamp 或差異：Covers %v、Check %+v", srB.Covers[2], srB.Check)
			}
			if c.flight && len(srB.FlightReport[3]) == 0 {
				t.Error("飛行字串沒有出現")
			}
			if c.labelHidden && len(srB.Hidden[2]) == 0 {
				t.Error("標籤 Hidden 沒有出現")
			}
			if c.hideFlight && len(srB.FlightHidden[3]) == 0 {
				t.Error("飛行 Hidden 沒有出現")
			}
			if c.art && len(srB.ArtKeys[2]) == 0 {
				t.Error("圖面沒有出現")
			}
		})
	}
}

// TestComposeScaleByValue（strike_commander 規格 014 U10）：composeScale 只用 scaleIn 帶來的標籤集合與世代，
// 不讀觀察器。觀察器仍有兩個標籤、scaleIn 只帶第一個時，結果等於觀察器本身只有第一個標籤的參考實作。
func TestComposeScaleByValue(t *testing.T) {
	for _, c := range []composeCase{{name: "標籤"}, {name: "飛行", flight: true, fovScales: []int{2, 3}}} {
		t.Run(c.name, func(t *testing.T) {
			a, b := buildComposeFixture(t, c), buildComposeFixture(t, c)
			a.so.Labels.visible = a.so.Labels.visible[:1]
			srA, srB := newShotResult(a), newShotResult(b)
			refShotScales(a.cfg, a.so, &srA, a.overlays, a.fovs, a.flight)
			for _, sc := range b.cfg.Scales {
				in := scaleIn{Overlay: b.overlays[sc], LabelsV: b.so.Labels.Visible()[:1], LabelsG: b.so.Labels.Generation(), Scale: sc, Frame: b.so.LastIndexed, Palette: b.so.LastPalette}
				if fov := b.fovs[sc]; fov != nil {
					in.Flight, in.FlightV, in.FlightG = fov, b.flight.Visible(), b.flight.Generation()
				}
				storeShotScale(&srB, sc, composeScale(in), in.Flight != nil)
			}
			if len(b.so.Labels.Visible()) != 2 {
				t.Fatalf("觀察器的可見集合被改：%d", len(b.so.Labels.Visible()))
			}
			if !reflect.DeepEqual(srA, srB) {
				diffShot(t, srA, srB)
			}
		})
	}
}

// TestComposeScaleMissingKeys：Flight 為 nil 的倍率不寫 FlightHidden、FlightCovers、FlightReport 與停止點
// FlightReport、FlightHidden 的鍵（缺鍵，不寫空值）；沒掛 FlightObserver 時這些 map 本身是 nil。
func TestComposeScaleMissingKeys(t *testing.T) {
	f := buildComposeFixture(t, composeCase{flight: true, fovScales: []int{3}})
	sr := newShotResult(f)
	newShotScales(f.cfg, f.so, &sr, f.overlays, f.fovs, f.flight)
	for name, ok := range map[string]bool{
		"FlightHidden": hasKey(sr.FlightHidden, 2), "FlightCovers": hasKey(sr.FlightCovers, 2), "FlightReport": hasKey(sr.FlightReport, 2),
	} {
		if ok {
			t.Errorf("截圖點 %s 有 2 倍鍵", name)
		}
	}
	if !hasKey(sr.FlightReport, 3) || !hasKey(sr.FlightCovers, 3) || !hasKey(sr.FlightHidden, 3) {
		t.Error("截圖點缺 3 倍鍵")
	}
	f = buildComposeFixture(t, composeCase{flight: true, fovScales: []int{3}})
	res := newStopResult(f)
	newStopScales(f.cfg, f.so, res, f.overlays, f.fovs, f.flight)
	if hasKey(res.FlightReport, 2) || hasKey(res.FlightHidden, 2) || !hasKey(res.FlightReport, 3) || !hasKey(res.FlightHidden, 3) {
		t.Errorf("停止點的鍵：FlightReport %d、FlightHidden %d", len(res.FlightReport), len(res.FlightHidden))
	}
	f = buildComposeFixture(t, composeCase{})
	sr = newShotResult(f)
	newShotScales(f.cfg, f.so, &sr, f.overlays, f.fovs, f.flight)
	if sr.FlightHidden != nil || sr.FlightCovers != nil || sr.FlightReport != nil {
		t.Error("沒掛 FlightObserver 時截圖點的飛行 map 不是 nil")
	}
	// StampCovers 一律非 nil（現行 sr.Covers[sc] = map[string][4]int{}）。
	if sr.Covers[2] == nil || sr.Covers[3] == nil {
		t.Error("Covers 的值是 nil map")
	}
}

func hasKey[V any](m map[int]V, k int) bool {
	_, ok := m[k]
	return ok
}

func diffShot(t *testing.T, a, b ShotResult) {
	t.Helper()
	for name, eq := range map[string]bool{
		"Composites": reflect.DeepEqual(a.Composites, b.Composites), "Plain": reflect.DeepEqual(a.Plain, b.Plain),
		"Check": reflect.DeepEqual(a.Check, b.Check), "ArtKeys": reflect.DeepEqual(a.ArtKeys, b.ArtKeys),
		"Hidden": reflect.DeepEqual(a.Hidden, b.Hidden), "Covers": reflect.DeepEqual(a.Covers, b.Covers),
		"BG": reflect.DeepEqual(a.BG, b.BG), "FlightHidden": reflect.DeepEqual(a.FlightHidden, b.FlightHidden),
		"FlightCovers": reflect.DeepEqual(a.FlightCovers, b.FlightCovers), "FlightReport": reflect.DeepEqual(a.FlightReport, b.FlightReport),
		"Arts": reflect.DeepEqual(a.Arts, b.Arts),
	} {
		if !eq {
			t.Errorf("截圖點 %s 不同", name)
		}
	}
	t.Error("截圖點 ShotResult 不同")
}

// TestComposeScaleHDNotYet：HD 路徑在分期 (g) 才實作；in.HD 非 nil 時 panic，不默默當成 HD 關。
func TestComposeScaleHDNotYet(t *testing.T) {
	f := buildComposeFixture(t, composeCase{})
	defer func() {
		if recover() == nil {
			t.Error("HD 非 nil 沒有 panic")
		}
	}()
	composeScale(scaleIn{Overlay: f.overlays[2], LabelsV: f.so.Labels.Visible(), LabelsG: f.so.Labels.Generation(), Scale: 2, Frame: f.so.LastIndexed, Palette: f.so.LastPalette, HD: &hdSource{}})
}

// TestCheckCompositeMask：allow 為真的 1 倍像素放行其 s×s 輸出像素，先於矩形判斷；nil 與 CheckCompositeText 相同。
func TestCheckCompositeMask(t *testing.T) {
	pal := testPal()
	idx := make([]byte, 64000)
	for _, sc := range []int{2, 3} {
		plain := ScaleIndexedRGBA(idx, pal, sc)
		comp := append([]byte(nil), plain...)
		w := 320 * sc
		set := func(ox, oy int) { comp[4*(oy*w+ox)] ^= 0xFF }
		// 一點落在 1 倍 (10, 20) 的 s×s 區塊最後一個子像素；一點在文字框內；一點在飛行遮蓋內。
		set(10*sc+sc-1, 20*sc+sc-1)
		set(100*sc, 50*sc)
		set(200*sc+1, 150*sc)
		text := []xlate.AlphaRect{{X0: 100 * sc, Y0: 50 * sc, X1: 100*sc + 1, Y1: 50*sc + 1}}
		fl := FlightCover{Key: "k", X0: 200, Y0: 150, X1: 201, Y1: 151}
		base := CheckCompositeText(comp, plain, sc, nil, nil, text, fl)
		if base.Diffs != 3 || base.Outside != 1 {
			t.Fatalf("%d 倍 CheckCompositeText %+v，要 Diffs 3、Outside 1", sc, base)
		}
		if got := CheckCompositeMask(comp, plain, sc, nil, nil, text, nil, fl); got != base {
			t.Fatalf("%d 倍 allow nil：%+v，要 %+v", sc, got, base)
		}
		allow := make([]bool, 64000)
		allow[20*320+11] = true // 鄰點：不放行
		if got := CheckCompositeMask(comp, plain, sc, nil, nil, text, allow, fl); got.Outside != 1 || got.Diffs != 3 {
			t.Fatalf("%d 倍 鄰點 allow：%+v", sc, got)
		}
		allow[20*320+10] = true
		if got := CheckCompositeMask(comp, plain, sc, nil, nil, text, allow, fl); got.Outside != 0 || got.Diffs != 3 {
			t.Fatalf("%d 倍 allow 放行：%+v，要 Diffs 3、Outside 0", sc, got)
		}
		// allow 放行的點也在矩形內時仍只計一次允許（不重複、不影響 Diffs）。
		allow[50*320+100] = true
		if got := CheckCompositeMask(comp, plain, sc, nil, nil, text, allow, fl); got.Outside != 0 || got.Diffs != 3 {
			t.Fatalf("%d 倍 allow 與矩形重疊：%+v", sc, got)
		}
	}
	defer func() {
		if recover() == nil {
			t.Error("allow 長度錯沒有 panic")
		}
	}()
	CheckCompositeMask(nil, nil, 2, nil, nil, nil, make([]bool, 10))
}
