package strikecommander

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// strike_commander 規格 014 分期 (b) 的單元測試：墨跡台帳（§4.2.2）、台帳自有實例（U26，負對照 N29）、
// k0 內建字的獨立算法、台帳自備的 TRE／PAK 讀取器、判定與合成點排程。

// --- 判定 ---

func TestExpoOnScreenThreshold(t *testing.T) {
	idx := make([]byte, 64000)
	e := &ledgerEntry{}
	for i := 0; i < 10; i++ {
		e.addPt(10+i, 20, 15)
	}
	if onScreen(e, idx) {
		t.Fatal("0 點相符卻在畫面上")
	}
	for i := 0; i < 4; i++ {
		idx[20*320+10+i] = 15
	}
	if onScreen(e, idx) {
		t.Fatal("4／10 點（未過半）卻在畫面上")
	}
	idx[20*320+14] = 15
	if !onScreen(e, idx) {
		t.Fatal("5／10 點（過半、≥ 4）不在畫面上")
	}
	small := &ledgerEntry{}
	for i := 0; i < 3; i++ {
		small.addPt(10+i, 20, 15)
	}
	if onScreen(small, idx) {
		t.Fatal("3 點的條目不足 4 點，不得算在畫面上")
	}
}

// TestExpoJudgeEntry：照畫要「合成圖 ≠ 原版放大圖」且「合成圖 == 台帳合成圖」；文字為 s×s 區塊、圖面為字帶整欄；游標點另計。
func TestExpoJudgeEntry(t *testing.T) {
	pal := tPal()
	idx := make([]byte, 64000)
	for i := range idx {
		idx[i] = 30
	}
	e := &ledgerEntry{id: 7, kind: ledLabel, key: "k"}
	for x := 100; x < 110; x++ {
		idx[50*320+x] = 15
		e.addPt(x, 50, 15)
	}
	e.finish()
	for _, sc := range []int{2, 3} {
		plain := ScaleIndexedRGBA(idx, pal, sc)
		W := 320 * sc
		paintBlock := func(img []byte, x, y int, v byte) {
			for dy := 0; dy < sc; dy++ {
				for dx := 0; dx < sc; dx++ {
					img[4*((y*sc+dy)*W+x*sc+dx)] = v
				}
			}
		}
		// 合成圖等於原版：全部露出。
		ex := judgeEntry(e, idx, nil, sc, plain, plain, plain, nil, nil)
		if ex.Visible != 10 || ex.Exposed != 10 {
			t.Fatalf("%d 倍 未覆繪：%+v", sc, ex)
		}
		// 合成圖蓋了前 6 點、台帳合成圖相同：露出 4。
		comp := append([]byte(nil), plain...)
		for x := 100; x < 106; x++ {
			paintBlock(comp, x, 50, 1)
		}
		ex = judgeEntry(e, idx, nil, sc, comp, plain, comp, nil, nil)
		if ex.Exposed != 4 {
			t.Fatalf("%d 倍 部分覆繪：%+v", sc, ex)
		}
		// 台帳合成圖不同（stamp 像素不對）：蓋了也算露出。
		lc := append([]byte(nil), comp...)
		paintBlock(lc, 101, 50, 2)
		ex = judgeEntry(e, idx, nil, sc, comp, plain, lc, nil, nil)
		if ex.Exposed != 5 {
			t.Fatalf("%d 倍 stamp 像素不同：%+v", sc, ex)
		}
		// 同色筆畫整塊蓋住（合成圖 == 原版 == 台帳）：字面規則露出；落在台帳回報的照畫遮蓋範圍內時不算露出。
		cover := make([]bool, 64000)
		cover[50*320+100] = true
		ex = judgeEntry(e, idx, nil, sc, comp, plain, comp, nil, cover)
		if ex.ExposedLit != 4 || ex.Exposed != 4 {
			t.Fatalf("%d 倍 遮蓋範圍外：%+v", sc, ex)
		}
		for x := 106; x < 110; x++ {
			cover[50*320+x] = true
		}
		ex = judgeEntry(e, idx, nil, sc, comp, plain, comp, nil, cover)
		if ex.ExposedLit != 4 || ex.Exposed != 0 {
			t.Fatalf("%d 倍 遮蓋範圍內同色：%+v", sc, ex)
		}
		lc2 := append([]byte(nil), comp...)
		paintBlock(lc2, 107, 50, 9)
		if ex := judgeEntry(e, idx, nil, sc, comp, plain, lc2, nil, cover); ex.Exposed != 1 {
			t.Fatalf("%d 倍 遮蓋範圍內但台帳不同：%+v", sc, ex)
		}
		// 游標 11×11 涵蓋 x 100..110：露出點全落在游標內。
		ex = judgeEntry(e, idx, nil, sc, plain, plain, plain, &Point{105, 50}, nil)
		if ex.Cursor != ex.Exposed {
			t.Fatalf("%d 倍 游標：%+v", sc, ex)
		}
		// 呈現像已不是記錄色的點不算。
		idx2 := append([]byte(nil), idx...)
		idx2[50*320+100] = 30
		if ex := judgeEntry(e, idx2, nil, sc, plain, plain, plain, nil, nil); ex.Visible != 9 || ex.Exposed != 9 {
			t.Fatalf("%d 倍 已改色的點：%+v", sc, ex)
		}
		// 圖面：區塊是字帶 [48, 53) 的整欄；欄內任一列與台帳合成圖不同就不算照畫。
		a := &ledgerEntry{id: 8, kind: ledArt, key: "a", art: true, by0: 48, by1: 53}
		a.addPt(100, 50, 15)
		a.finish()
		comp = append([]byte(nil), plain...)
		for y := 48; y < 53; y++ {
			paintBlock(comp, 100, y, 1)
		}
		if ex := judgeEntry(a, idx, nil, sc, comp, plain, comp, nil, nil); ex.Exposed != 0 {
			t.Fatalf("%d 倍 圖面整欄照畫：%+v", sc, ex)
		}
		lc = append([]byte(nil), comp...)
		paintBlock(lc, 100, 52, 2)
		if ex := judgeEntry(a, idx, nil, sc, comp, plain, lc, nil, nil); ex.Exposed != 1 {
			t.Fatalf("%d 倍 圖面欄內一列不同：%+v", sc, ex)
		}
	}
}

// TestExpoContrast：色盤淡出成全黑時露出點沒有對比（分類「色盤不可見」）；正常色盤時有。
func TestExpoContrast(t *testing.T) {
	idx := make([]byte, 64000)
	for i := range idx {
		idx[i] = 30
	}
	e := &ledgerEntry{kind: ledLabel}
	for x := 100; x < 110; x++ {
		idx[50*320+x] = 15
		e.addPt(x, 50, 15)
	}
	pal := tPal()
	plain := ScaleIndexedRGBA(idx, pal, 2)
	if ex := judgeEntry(e, idx, &pal, 2, plain, plain, plain, nil, nil); ex.Exposed != 10 || ex.Contrast != 10 {
		t.Fatalf("正常色盤：%+v", ex)
	}
	var black [256][3]uint8
	plain = ScaleIndexedRGBA(idx, black, 2)
	if ex := judgeEntry(e, idx, &black, 2, plain, plain, plain, nil, nil); ex.Exposed != 10 || ex.Contrast != 0 {
		t.Fatalf("全黑色盤：%+v", ex)
	}
}

// TestLedgerReplace：同位置（墨跡外接框重疊）的新條目取代舊條目；逐點相同的條目不重複加入；不重疊的保留。
func TestLedgerReplace(t *testing.T) {
	l := &expoLedger{}
	mk := func(key string, x0, x1, y int, steps uint64) *ledgerEntry {
		e := &ledgerEntry{kind: ledLabel, key: key, steps: steps}
		for x := x0; x < x1; x++ {
			e.addPt(x, y, 15)
		}
		return e
	}
	l.add(mk("a", 10, 20, 50, 1))
	l.add(mk("b", 30, 40, 50, 2))
	v := l.ver
	l.add(mk("a", 10, 20, 50, 3)) // 逐點相同
	if len(l.entries) != 2 || l.ver != v || len(l.recs) != 2 {
		t.Fatalf("相同條目重複加入：%d 筆、ver %d→%d", len(l.entries), v, l.ver)
	}
	l.add(mk("c", 15, 35, 50, 4)) // 與 a、b 都重疊
	if len(l.entries) != 1 || l.entries[0].key != "c" || l.recs[0].Replaced != 4 || l.recs[1].Replaced != 4 || len(l.retiring) != 2 {
		t.Fatalf("取代：%+v %+v", l.entries, l.recs)
	}
	// a 又畫回原位：從退場清單收回（不另建條目），c 進退場清單。
	l.add(mk("a", 10, 20, 50, 6))
	if len(l.entries) != 1 || l.entries[0].key != "a" || len(l.recs) != 3 || len(l.retiring) != 2 || l.recs[0].Replaced != 0 {
		t.Fatalf("收回：%+v %+v %d", l.entries, l.recs, len(l.retiring))
	}
	l.add(mk("d", 100, 110, 150, 5))
	if len(l.entries) != 2 {
		t.Fatalf("不重疊的條目被移除：%d", len(l.entries))
	}
}

func TestExpoNextPoint(t *testing.T) {
	x := &expoRun{}
	c, ok := x.nextPoint(1000, 1000+expoInterval-1)
	if ok {
		t.Fatalf("區間內沒有合成點卻回 %d", c)
	}
	c, ok = x.nextPoint(1000, 1000+3*expoInterval)
	if !ok || c != 1000+expoInterval {
		t.Fatalf("第一個合成點 %d %t", c, ok)
	}
	if ph := int(((c - x.boot) / expoInterval) % expoPhases); ph != 1 {
		t.Fatalf("相位 %d", ph)
	}
	x.next = c + expoInterval // checkpoint 的排程部分
	if c2, ok := x.nextPoint(c, c+expoInterval); !ok || c2 != c+expoInterval {
		t.Fatalf("下一個合成點 %d %t", c2, ok)
	}
	// 同步數：合成點在 (steps, target]，target 本身可以是合成點。
	if c3, ok := x.nextPoint(c, c+expoInterval-1); ok {
		t.Fatalf("(steps, target] 外的合成點 %d", c3)
	}
	n1 := &expoRun{mut: expoMutN1}
	if _, ok := n1.nextPoint(0, 1<<40); ok {
		t.Fatal("N1 模式不得有固定合成點")
	}
}

// --- k0 內建字的獨立算法 ---

// TestK0InkIndependent：獨立算法的四塊與共用遮罩對照：標題 ⊆ dialogMask(k0)，差只在 x 134（judgeBox 的第二次勘誤延伸）；
// SAVE、CANCEL、QUIT TO DOS 平移後 ⊇ k2、k6、k8 的 dialogMask（多出的只在往下延的陰影列），且同點同色。第 0、1 組。
func TestK0InkIndependent(t *testing.T) {
	file := loadSHP(t)
	for _, g := range []int{0, 1} {
		ent := mustDecode(t, file, g*21)
		es := k0Ink(ent.Pix, 80, 30, 1)
		if len(es) != 4 {
			t.Fatalf("組 %d：%d 塊", g, len(es))
		}
		set := func(e *ledgerEntry) map[int32]uint8 {
			m := map[int32]uint8{}
			for i, p := range e.pts {
				m[p] = e.cols[i]
			}
			return m
		}
		// 標題。
		_, _, mask := dialogMask(mkEvent(t, file, g*21, 80, 30, 1))
		title := set(es[0])
		shared := map[int32]uint8{}
		for i, on := range mask {
			if on {
				shared[int32((30+i/182)*320+80+i%182)] = ent.Pix[i]
			}
		}
		extra, missing := 0, 0
		for p, c := range title {
			if sc, ok := shared[p]; !ok || sc != c {
				extra++
			}
		}
		for p := range shared {
			if _, ok := title[p]; !ok && p%320-80 != 134 {
				missing++
			}
		}
		// 第 1 組（帳篷）的條目色號與第 0 組不同（004 §3.4「依位置認定，不換算色號」），規格 014 §4.2.2 的獨立算法以
		// Entry 色號對第 0 組的底色集合判定，會多收底紋點；只記錄，不斷言（回報主代理）。
		t.Logf("組 %d 標題：獨立算法 %d 點、共用遮罩 %d 點；獨立算法多出 %d、缺 %d（x 134 除外）", g, len(title), len(shared), extra, missing)
		if g == 0 && (extra != 0 || missing != 0) {
			t.Fatalf("組 0 標題與共用遮罩不符：多 %d、缺 %d", extra, missing)
		}
		if g == 0 && len(title) < 100 {
			t.Fatalf("組 0 標題墨跡只有 %d 點", len(title))
		}
		// 三個按鈕。
		for i, d := range k0Derived {
			got := set(es[1+i])
			ev := mkEvent(t, file, g*21+d.k, 80+d.dx, 30+d.dy, 1)
			el, _, m := dialogMask(ev)
			n := 0
			for j, on := range m {
				if !on {
					continue
				}
				n++
				p := int32((ev.Y+j/el.w)*320 + ev.X + j%el.w)
				if c, ok := got[p]; !ok || c != ev.Entry[j] {
					t.Fatalf("組 %d %s：共用遮罩點 (%d,%d) 不在獨立算法或色不同", g, el.key, j%el.w, j/el.w)
				}
			}
			if n == 0 {
				t.Fatalf("組 %d %s 共用遮罩是空的", g, el.key)
			}
			_, ink, _ := elementFor(d.k)
			for p := range got {
				ry := int(p/320) - ev.Y
				if ry > ink.y1+1 {
					t.Fatalf("組 %d %s 多出的點在第 %d 列（墨跡框 %d 之下超過 1 列）", g, el.key, ry, ink.y1)
				}
			}
		}
	}
}

// --- 台帳自備的 TRE／PAK 讀取器 ---

func readOuterPak(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(i018Dir(t), "dec/MAINMENU.bin"))
	if err != nil {
		t.Skip("讀不到 MAINMENU.bin：" + err.Error())
	}
	if tSHA(b) != tMainMenuPAKSHA {
		t.Fatalf("MAINMENU.bin SHA-256 %s", tSHA(b))
	}
	return b
}

// writeTRE 寫一份合成的 TRE：names 依序成目錄（74 bytes 一筆：旗標、65 bytes 名稱、位移、長度），資料都指向同一份 pak。
func writeTRE(t *testing.T, path string, names []string, pak []byte) {
	t.Helper()
	n := len(names)
	dataOff := 8 + 74*n
	b := make([]byte, dataOff+len(pak))
	binary.LittleEndian.PutUint32(b, uint32(n))
	binary.LittleEndian.PutUint32(b[4:], 0xDEADBEEF) // 第二個 u32 不使用
	for k, name := range names {
		r := b[8+74*k:]
		r[0] = 1
		copy(r[1:66], name)
		binary.LittleEndian.PutUint32(r[66:], uint32(dataOff))
		binary.LittleEndian.PutUint32(r[70:], uint32(len(pak)))
	}
	copy(b[dataOff:], pak)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLedgerReadMainMenu(t *testing.T) {
	pak := readOuterPak(t)
	const good = `..\..\DATA\GAMEFLOW\MAINMENU.PAK`
	resid := good + "\x00junk!" // NUL 之後的殘留
	f := loadMenuFiles(t)
	check := func(dir, wantTRE string) {
		t.Helper()
		m, err := ledgerReadMainMenu(dir)
		if err != nil {
			t.Fatalf("%s：%v", dir, err)
		}
		plate := mustDecode(t, f.plate, 0)
		if m.tre != wantTRE || !reflect.DeepEqual(m.buttons, f.buttons) || !reflect.DeepEqual(m.plate, plate.Pix) {
			t.Fatalf("讀到 %s，要 %s；按鈕檔或底板與 i018 不同", m.tre, wantTRE)
		}
	}
	d := t.TempDir()
	writeTRE(t, filepath.Join(d, "BIGTRE.TRE"), []string{"..\\..\\X.PAK", resid}, pak)
	check(d, "BIGTRE.TRE")
	// 名稱大小寫不同仍命中。
	d = t.TempDir()
	writeTRE(t, filepath.Join(d, "BIGTRE.TRE"), []string{strings.ToLower(good)}, pak)
	check(d, "BIGTRE.TRE")
	// BIGTRE 不合格時改用 TOBIGTRE。
	bad := append([]byte(nil), pak...)
	bad[100] ^= 0xFF
	d = t.TempDir()
	writeTRE(t, filepath.Join(d, "BIGTRE.TRE"), []string{good}, bad)
	writeTRE(t, filepath.Join(d, "TOBIGTRE.TRE"), []string{good}, pak)
	check(d, "TOBIGTRE.TRE")
	// 各種不合格。
	for name, mk := range map[string]func(dir string){
		"缺檔":     func(string) {},
		"名稱缺":    func(dir string) { writeTRE(t, filepath.Join(dir, "BIGTRE.TRE"), []string{"..\\..\\X.PAK"}, pak) },
		"命中兩筆":   func(dir string) { writeTRE(t, filepath.Join(dir, "BIGTRE.TRE"), []string{good, resid}, pak) },
		"外層雜湊不符": func(dir string) { writeTRE(t, filepath.Join(dir, "BIGTRE.TRE"), []string{good}, bad) },
		"長度不符": func(dir string) {
			writeTRE(t, filepath.Join(dir, "BIGTRE.TRE"), []string{good}, append(append([]byte(nil), pak...), 0))
		},
	} {
		d := t.TempDir()
		mk(d)
		if _, err := ledgerReadMainMenu(d); err == nil {
			t.Errorf("%s：沒有回錯", name)
		} else {
			t.Logf("%s：%v", name, err)
		}
	}
	// 外層解析：首 16 bytes 得三條目、型別 E0；型別改成 00 時拒絕（不走 pakEntries）。
	if got, want := pak[:16], []byte{0x1b, 0x60, 0, 0, 0x10, 0, 0, 0xe0, 0x06, 0x3d, 0, 0xe0, 0x97, 0x5e, 0, 0xe0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("首 16 bytes %x", got)
	}
	r, err := ledgerOuterPak(pak)
	if err != nil || r != ledgerSubRange {
		t.Fatalf("外層 %v %v", r, err)
	}
	if _, err := pakEntries(pak); err == nil {
		t.Fatal("pakEntries 應拒絕型別 E0 的外層（規格 014 N19 的前提）")
	}
	typ := append([]byte(nil), pak...)
	typ[11] = 0
	if _, err := ledgerOuterPak(typ); err == nil {
		t.Fatal("型別非 E0 沒有拒絕")
	}
}

// TestLedgerPlateInk：底板灰字以規則 Q 算五行，平移到 (45,24)；每行有墨跡、落在該行字帶，色為底板條目。
func TestLedgerPlateInk(t *testing.T) {
	f := loadMenuFiles(t)
	pak := readOuterPak(t)
	d := t.TempDir()
	writeTRE(t, filepath.Join(d, "BIGTRE.TRE"), []string{`..\..\DATA\GAMEFLOW\MAINMENU.PAK`}, pak)
	l := &expoLedger{root: d}
	l.onShapeArt(9, ArtMainMenuPlate, f.plate, 0, tPlateX, tPlateY)
	if l.readerErr != "" || len(l.entries) != mmLines || l.plates != 1 {
		t.Fatalf("讀取器 %q、條目 %d", l.readerErr, len(l.entries))
	}
	plate := mustDecode(t, f.plate, 0)
	for n, e := range l.entries {
		pb := plateBand(n)
		if e.key != mmKeys[n] || len(e.pts) < 50 {
			t.Fatalf("第 %d 行 %q %d 點", n, e.key, len(e.pts))
		}
		for i, p := range e.pts {
			x, y := int(p%320)-tPlateX, int(p/320)-tPlateY
			if !pb.in(x, y) || plate.Pix[y*plateW+x] != e.cols[i] {
				t.Fatalf("第 %d 行 (%d,%d) 不在字帶或色不同", n, x, y)
			}
		}
	}
	// 缺檔：記錯誤，沒有條目。
	l = &expoLedger{root: t.TempDir()}
	l.onShapeArt(9, ArtMainMenuPlate, f.plate, 0, tPlateX, tPlateY)
	if l.readerErr == "" || len(l.entries) != 0 {
		t.Fatalf("缺檔：%q %d", l.readerErr, len(l.entries))
	}
}

// --- U26：台帳的 stamp 像素只由自有實例產生（負對照 N29） ---

type u26Verdict struct {
	Scale, Entry     int
	Key              string
	Visible, Exposed int
}

// u26Fixture 建一個標籤加 k0 對話框的畫格與台帳；obs、pres 是觀察用與呈現用實例（呼叫端決定內容對錯）。
type u26Fixture struct {
	idx  []byte
	pal  [256][3]uint8
	lo   *LabelObserver
	led  *expoLedger
	file []byte
	ev0  ArtEvent
	l1   Label
}

func newU26Fixture(t *testing.T) u26Fixture {
	t.Helper()
	fonts := flightTextFonts(t)
	file := loadSHP(t)
	idx := make([]byte, 64000)
	for i := range idx {
		idx[i] = 30
	}
	paint(t, idx, file, 0, 80, 30)
	l1 := Label{Key: "Talk to Gwen", X0: 114, X1: 204, Y: 170, Font: measuredFont, Kind: KindLabel, Steps: 5}
	before := make([]byte, 14*320)
	for i := range before {
		before[i] = 30
	}
	for x := 116; x < 200; x += 7 {
		idx[175*320+x] = 15
	}
	l1.Before = before
	l1.After = append([]byte(nil), idx[171*320:185*320]...)
	l1.Ref = refFromFrame(idx, l1)
	led := &expoLedger{ovs: map[int]*Overlay{}, fovs: map[int]*FlightOverlay{}}
	for _, sc := range []int{2, 3} {
		ov, err := NewOverlayText(fonts, testCatalog(), testGfx(), sc)
		if err != nil {
			t.Fatal(err)
		}
		led.ovs[sc] = ov
	}
	ev0 := mkEvent(t, file, 0, 80, 30, 1)
	led.onPutsReturn(l1)
	led.onCommit(Commit{Labels: []Label{l1}})
	led.onShapeArt(1, ArtSaveLoad, file, 0, 80, 30)
	led.onArt(ev0)
	lo := NewLabelObserver()
	lo.visible, lo.gen = []Label{l1}, 1
	return u26Fixture{idx: idx, pal: tPal(), lo: lo, led: led, file: file, ev0: ev0, l1: l1}
}

// u26Instances 建一組觀察用或呈現用實例：wrong 為真時譯文互換、圖面事件的條目打亂（內容故意錯）。
func u26Instances(t *testing.T, f u26Fixture, wrong bool) map[int]*Overlay {
	t.Helper()
	cat, gfx := testCatalog(), testGfx()
	ev := f.ev0
	if wrong {
		cat["Talk to Gwen"] = cat["Transport to Strike Base"]
		gfx["SAVE GAME"] = gfx["QUIT TO DOS"]
		ev.Entry = append([]uint8(nil), ev.Entry...)
		for i := range ev.Entry {
			ev.Entry[i] ^= 0x55
		}
	}
	out := map[int]*Overlay{}
	for _, sc := range []int{2, 3} {
		ov, err := NewOverlayText(flightTextFonts(t), cat, gfx, sc)
		if err != nil {
			t.Fatal(err)
		}
		ov.AddDialogArt(ev)
		out[sc] = ov
	}
	return out
}

// u26Verdicts 以固定的受檢合成圖（呈現用實例內容正確時的現行合成），回台帳每個條目、每個倍率的判定。
func u26Verdicts(t *testing.T, f u26Fixture, obs, pres map[int]*Overlay, comp map[int][]byte, mut expoMut) []u26Verdict {
	t.Helper()
	nilMove := (*Point)(nil)
	x := &expoRun{mut: mut, lo: f.lo, scales: []int{2, 3}, led: f.led, obsOvs: obs, presOvs: pres, obsFovs: map[int]*FlightOverlay{}, presFovs: map[int]*FlightOverlay{}, lastMove: &nilMove}
	var out []u26Verdict
	for _, sc := range []int{2, 3} {
		plain := ScaleIndexedRGBA(f.idx, f.pal, sc)
		x.presentCompose(sc, f.idx, f.pal) // 呈現用實例照常出圖（內容錯時也只改它自己的狀態）
		lc, cover := x.ledgerCompose(sc, f.idx, f.pal)
		for _, e := range f.led.entries {
			if !onScreen(e, f.idx) {
				continue
			}
			ex := judgeEntry(e, f.idx, nil, sc, comp[sc], plain, lc, nil, cover)
			out = append(out, u26Verdict{sc, e.id, e.key, ex.Visible, ex.Exposed})
		}
	}
	return out
}

func u26Mut(t *testing.T) expoMut {
	if os.Getenv("DOSGOLEM_SC_EXPO_MUT") == "n29" {
		t.Log("負對照 N29：台帳改用觀察用實例取 stamp 像素")
		return expoMutObsStamps
	}
	return 0
}

// TestU26LedgerOwnInstances（規格 014 U26）：台帳的照畫判定只由自有實例決定。受檢合成圖固定（內容正確的現行合成），
// 觀察用與呈現用實例換成內容故意錯的，判定不變。另：標籤與 k0 標題照畫；k0 內建的 SAVE、CANCEL、QUIT TO DOS 在基線露出
// （現行合成沒有衍生列，台帳自己規劃了）。DOSGOLEM_SC_EXPO_MUT=n29 時本測試必須失敗。
func TestU26LedgerOwnInstances(t *testing.T) {
	f := newU26Fixture(t)
	mut := u26Mut(t)
	good := u26Instances(t, f, false)
	comp := map[int][]byte{}
	for _, sc := range []int{2, 3} {
		comp[sc] = composeScale(scaleIn{Overlay: good[sc], LabelsV: f.lo.Visible(), LabelsG: f.lo.Generation(), Scale: sc, Frame: f.idx, Palette: f.pal}).Comp
	}
	v0 := u26Verdicts(t, f, u26Instances(t, f, false), u26Instances(t, f, false), comp, mut)
	v1 := u26Verdicts(t, f, u26Instances(t, f, true), u26Instances(t, f, true), comp, mut)
	if !reflect.DeepEqual(v0, v1) {
		t.Fatalf("觀察用、呈現用實例內容改錯後台帳判定改變：\n正確 %+v\n錯誤 %+v", v0, v1)
	}
	want := map[string]bool{"Talk to Gwen": false, "SAVE GAME": false, "SAVE": true, "CANCEL": true, "QUIT TO DOS": true} // true：露出
	seen := map[string]int{}
	for _, v := range v0 {
		w, ok := want[v.Key]
		if !ok {
			t.Fatalf("多出的條目 %+v", v)
		}
		if v.Visible == 0 || (v.Exposed > 0) != w {
			t.Fatalf("%d 倍 %q：%+v，要露出 %t", v.Scale, v.Key, v, w)
		}
		seen[v.Key]++
	}
	for k := range want {
		if seen[k] != 2 {
			t.Fatalf("%q 的判定 %d 筆，要 2（兩個倍率）", k, seen[k])
		}
	}
}

// TestN29Detected：負對照 N29 的鑑別力（同 U26 的序列）：台帳改用觀察用實例時，觀察用實例內容錯會改變判定。
func TestN29Detected(t *testing.T) {
	f := newU26Fixture(t)
	good := u26Instances(t, f, false)
	comp := map[int][]byte{}
	for _, sc := range []int{2, 3} {
		comp[sc] = composeScale(scaleIn{Overlay: good[sc], LabelsV: f.lo.Visible(), LabelsG: f.lo.Generation(), Scale: sc, Frame: f.idx, Palette: f.pal}).Comp
	}
	v0 := u26Verdicts(t, f, u26Instances(t, f, false), u26Instances(t, f, false), comp, expoMutObsStamps)
	v1 := u26Verdicts(t, f, u26Instances(t, f, true), u26Instances(t, f, true), comp, expoMutObsStamps)
	if reflect.DeepEqual(v0, v1) {
		t.Fatal("台帳改用觀察用實例後，觀察用實例內容錯仍不改判定：U26 沒有鑑別力")
	}
}

// TestU26OverlapOrder（U26 後半）：重疊的圖面以 Order 最高者的像素為準。k2（Order 1）與 k4（Order 2）同位置，畫格是 k4：
// 受檢合成圖兩列都有時 LOAD 照畫；只有 k2 時 LOAD 露出。
func TestU26OverlapOrder(t *testing.T) {
	file := loadSHP(t)
	fonts := flightTextFonts(t)
	idx := make([]byte, 64000)
	paint(t, idx, file, 4, 134, 127)
	pal := tPal()
	led := &expoLedger{ovs: map[int]*Overlay{}, fovs: map[int]*FlightOverlay{}}
	mkOv := func(evs ...ArtEvent) map[int]*Overlay {
		m := map[int]*Overlay{}
		for _, sc := range []int{2, 3} {
			ov, err := NewOverlayText(fonts, testCatalog(), testGfx(), sc)
			if err != nil {
				t.Fatal(err)
			}
			for _, ev := range evs {
				ov.AddDialogArt(ev)
			}
			m[sc] = ov
		}
		return m
	}
	k2, k4 := mkEvent(t, file, 2, 134, 127, 1), mkEvent(t, file, 4, 134, 127, 2)
	led.ovs = mkOv(k2, k4)
	led.onShapeArt(2, ArtSaveLoad, file, 4, 134, 127)
	if len(led.entries) != 1 || led.entries[0].key != "LOAD" {
		t.Fatalf("台帳條目 %+v", led.entries)
	}
	nilMove := (*Point)(nil)
	lo := NewLabelObserver()
	for _, c := range []struct {
		name    string
		pres    map[int]*Overlay
		exposed bool
	}{{"兩列", mkOv(k2, k4), false}, {"只有 k2", mkOv(k2), true}} {
		x := &expoRun{lo: lo, scales: []int{2, 3}, led: led, presOvs: c.pres, presFovs: map[int]*FlightOverlay{}, lastMove: &nilMove}
		for _, sc := range []int{2, 3} {
			comp, plain := x.presentCompose(sc, idx, pal)
			lc, cover := x.ledgerCompose(sc, idx, pal)
			ex := judgeEntry(led.entries[0], idx, nil, sc, comp, plain, lc, nil, cover)
			if ex.Visible == 0 || (ex.Exposed > 0) != c.exposed {
				t.Fatalf("%s %d 倍：%+v，要露出 %t", c.name, sc, ex, c.exposed)
			}
		}
	}
}

// TestLedgerK15YesNo：k15 另記框內 YES、NO（RE 017 §3：k15 內 (32,28)–(52,32)、(84,28)–(99,32)），色為 k15 條目。
func TestLedgerK15YesNo(t *testing.T) {
	file := loadSHP(t)
	l := &expoLedger{}
	l.onShapeArt(1, ArtSaveLoad, file, 15, 101, 85)
	got := map[string]*ledgerEntry{}
	for _, e := range l.entries {
		got[e.key] = e
	}
	ent := mustDecode(t, file, 15)
	for key, b := range map[string]box{"YES": {32, 28, 52, 32}, "NO": {84, 28, 99, 32}} {
		e := got[key]
		if e == nil || e.kind != ledConfirm || len(e.pts) < 20 {
			t.Fatalf("%s：%+v", key, e)
		}
		for i, p := range e.pts {
			x, y := int(p%320)-101, int(p/320)-85
			if !b.in(x, y) || ent.Pix[y*132+x] != e.cols[i] {
				t.Fatalf("%s (%d,%d) 不在 RE 017 的框或色不同", key, x, y)
			}
		}
	}
	if e := got["ARE YOU SURE"]; e == nil || e.kind != ledArt {
		t.Fatal("缺 ARE YOU SURE")
	}
}
