package strikecommander

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// strike_commander 規格 008 §4.2 的觸發案例 L、M1、M2、N（長測試，各開、關一次）。
// 只在下列環境變數都設定時執行，否則 t.Skip：
//   - DOSGOLEM_SC_ROOT、DOSGOLEM_SC_ROOT_SAV1（workplace/i018/root-sav，只有 ABC.sav）、
//     DOSGOLEM_SC_ROOT_SAV2（workplace/i018/root-sav2，另有 gensave.tmp）：SaveRoot 每次由它複製；
//   - DOSGOLEM_SC_STATE_DIR（i018-st/m242.state、i018-cold/c800.state）；
//   - DOSGOLEM_SC_FONT_DIR、DOSGOLEM_SC_CATALOG、DOSGOLEM_SC_GFX_CATALOG：反鋸齒圖集（011 §3.9）與兩份正式 catalog；
//   - DOSGOLEM_SC_I018：workplace/i018（游標欄核對用的 PAK）。
//
// 事件步數取自盤點工具的 strike_commander workplace/i018/a6、a4、a5、m1 的 ip.tsv（同一輸入的步數，
// 底板與按鈕呼叫點的返回位址列），獨立於本規格的實作。DOSGOLEM_SC_OUT 有設定時寫出 shot 到 <OUT>/case-l 等。

const (
	scRootSav1Env = "DOSGOLEM_SC_ROOT_SAV1"
	scRootSav2Env = "DOSGOLEM_SC_ROOT_SAV2"
)

// mWant 是一筆期望的繪圖事件。
type mWant struct {
	steps uint64
	file  ArtFile
	k     int
	x, y  int
}

type menuCase struct {
	c       slCase
	rootEnv string
	want    []mWant
}

const (
	m242SHA = "93bc8986d5e148c0f751e8736b42f25a06e223ef007135cc453b08094586a4ca" // RE 018 狀態檔表
	c800SHA = "8957dec8ef9c9e447d488d9b35f832bc5ee2e69fbfd2eec25013f785138b562d"
)

func wantMM(steps uint64, k int) mWant {
	x, y := tMMPos(k % 5)
	return mWant{steps, ArtMainMenuButtons, k, x, y}
}

func wantPlate(steps uint64) mWant { return mWant{steps, ArtMainMenuPlate, 0, 45, 24} }

var menuCases = map[string]menuCase{
	// a6/ip.tsv：第一次主選單（底板、k2、k3、k4）、VIEW OBJECTS 按下 k9、放開 k4；OBJECT VIEWER 的檔不產生事件；
	// EXIT 後重畫（底板、k2、k3、k4）。
	"L": {slCase{name: "L", state: "i018-st/m242.state", sha: m242SHA, script: "testdata/case_l.script"}, scRootEnv, []mWant{
		wantPlate(2432322885), wantMM(2432366211, 2), wantMM(2432372490, 3), wantMM(2432374925, 4),
		wantMM(2450001938, 9), wantMM(2455000861, 4),
		wantPlate(2530421954), wantMM(2530462289, 2), wantMM(2530466633, 3), wantMM(2530469068, 4),
	}},
	// a4/ip.tsv：CONTINUE 灰，LOAD 可選；按灰色 CONTINUE 沒有事件。
	"M1": {slCase{name: "M1", state: "i018-st/m242.state", sha: m242SHA, script: "testdata/case_m1.script"}, scRootSav1Env, []mWant{
		wantPlate(2432602472), wantMM(2432645812, 1), wantMM(2432650762, 2), wantMM(2432655106, 3), wantMM(2432659569, 4),
	}},
	// a5/ip.tsv：五項都可選；LOAD 按下 k6、放開 k1；之後的讀檔對話框是 SAVELOAD.SHP 第 42 組（記 skip，不產生事件）。
	"M2": {slCase{name: "M2", state: "i018-st/m242.state", sha: m242SHA, script: "testdata/case_m2.script"}, scRootSav2Env, []mWant{
		wantPlate(2432804587), wantMM(2432847927, 0), wantMM(2432855255, 1), wantMM(2432860205, 2), wantMM(2432864549, 3), wantMM(2432866984, 4),
		wantMM(2450001927, 6), wantMM(2460001107, 1),
	}},
	// m1/ip.tsv：k0、k1；按下 k2；在按鈕外放開 k0。背景與底板（另兩個檔）不產生事件。
	"N": {slCase{name: "N", state: "i018-cold/c800.state", sha: c800SHA, script: "testdata/case_n.script"}, scRootEnv, []mWant{
		{878638865, ArtTomenuButtons, 0, 89, 33}, {878651762, ArtTomenuButtons, 1, 90, 71},
		{930001117, ArtTomenuButtons, 2, 89, 33}, {950000911, ArtTomenuButtons, 0, 89, 33},
	}},
}

// tArtBand 回一列圖面的字帶矩形 [x0, x1] × [y0, y1]（1×、含端點）與原版基準（事件的條目解碼值，測試自己查表）。
type tArtBand struct {
	key            string
	x0, y0, x1, y1 int
	order          int
	ref            []uint8 // 字帶寬 × 高，逐列
}

// menuArtBands 依 shot 前最後一筆事件，為 shot 的每個圖面 Key 建 tArtBand（008 §3.2 的表，測試內獨立寫出）。
func menuArtBands(t *testing.T, f tMenuFiles, res *ReceiptResult, s ShotResult, keys []string) []tArtBand {
	t.Helper()
	var out []tArtBand
	for _, key := range keys {
		var ev *ArtEvent
		var file []byte
		var band [4]int
		switch {
		case strings.HasPrefix(key, "mainmenu-plate/"):
			var n int
			fmt.Sscanf(key, "mainmenu-plate/%d", &n)
			for i := range res.Arts {
				if a := &res.Arts[i]; a.Steps < s.Steps && a.File == ArtMainMenuPlate {
					ev = a
				}
			}
			file, band = f.plate, [4]int{12, 11 + 17*n, 219, 22 + 17*n}
		case strings.HasPrefix(key, "mainmenu/"), strings.HasPrefix(key, "tomenu/"):
			for i := range res.Arts {
				if a := &res.Arts[i]; a.Steps < s.Steps && artKey(*a) == key {
					ev = a
				}
			}
			if ev != nil && ev.File == ArtTomenuButtons {
				file, band = f.tmButtons, tTM[ev.K].band
			} else {
				file, band = f.buttons, [4]int{2, 2, 208, 12}
			}
		default:
			t.Errorf("shot %s：不認得的圖面 %s", s.Name, key)
			continue
		}
		if ev == nil {
			t.Errorf("shot %s：圖面 %s 找不到事件", s.Name, key)
			continue
		}
		ent := mustDecode(t, file, ev.K)
		b := tArtBand{key: key, x0: ev.X + band[0], y0: ev.Y + band[1], x1: ev.X + band[2], y1: ev.Y + band[3], order: ev.Order}
		for y := band[1]; y <= band[3]; y++ {
			b.ref = append(b.ref, ent.Pix[y*ent.W+band[0]:y*ent.W+band[2]+1]...)
		}
		out = append(out, b)
	}
	return out
}

// checkMenuColumns：shot 時每列圖面，畫格與原版基準不同的欄（游標或已換畫面），合成圖要等於原版放大圖；
// 落在 Order 較大的另一列圖面字帶內的像素除外（底板列被按鈕蓋住，008 §3.4）。
func checkMenuColumns(t *testing.T, f tMenuFiles, res *ReceiptResult, s ShotResult, sc int) {
	t.Helper()
	bands := menuArtBands(t, f, res, s, s.ArtKeys[sc])
	W := 320 * sc
	later := func(b tArtBand, x, y int) bool {
		for _, o := range bands {
			if o.order > b.order && x >= o.x0 && x <= o.x1 && y >= o.y0 && y <= o.y1 {
				return true
			}
		}
		return false
	}
	for _, b := range bands {
		bw := b.x1 - b.x0 + 1
		hidden := 0
		for col := 0; col < bw; col++ {
			differs := false
			for y := b.y0; y <= b.y1 && !differs; y++ {
				differs = s.Indexed[y*320+b.x0+col] != b.ref[(y-b.y0)*bw+col]
			}
			if !differs {
				continue
			}
			hidden++
		scan:
			for y := b.y0 * sc; y < (b.y1+1)*sc; y++ {
				for x := (b.x0 + col) * sc; x < (b.x0+col+1)*sc; x++ {
					if later(b, x/sc, y/sc) {
						continue
					}
					i := 4 * (y*W + x)
					if string(s.Composites[sc][i:i+4]) != string(s.Plain[sc][i:i+4]) {
						t.Errorf("shot %s %d 倍：圖面 %s 第 %d 欄與基準不同，合成圖卻不等於原版", s.Name, sc, b.key, col)
						break scan
					}
				}
			}
		}
		if sc == 2 {
			t.Logf("shot %s：圖面 %s 停畫 %d／%d 欄", s.Name, b.key, hidden, bw)
		}
	}
}

// bandShown 回 shot 的某列圖面字帶在 sc 倍時顯示中文的欄數（合成圖與原版放大圖不同的欄）。
func bandShown(s *ShotResult, sc int, key string) int {
	var x0, y0, w, h int
	switch {
	case strings.HasPrefix(key, "mainmenu-plate/"):
		var n int
		fmt.Sscanf(key, "mainmenu-plate/%d", &n)
		x0, y0, w, h = 45+12, 24+11+17*n, 208, 12
	case strings.HasPrefix(key, "mainmenu/"):
		var x, y, n int
		fmt.Sscanf(key, "mainmenu/%d,%d/%d", &x, &y, &n)
		x0, y0, w, h = x+2, y+2, 207, 11
	case strings.HasPrefix(key, "tomenu/"):
		var x, y, n int
		fmt.Sscanf(key, "tomenu/%d,%d/%d", &x, &y, &n)
		e := tTM[n]
		x0, y0, w, h = x+e.band[0], y+e.band[1], e.band[2]-e.band[0]+1, e.band[3]-e.band[1]+1
	}
	return shownCols(s, sc, x0, y0, w, h)
}

// wantBand 斷言 shot 的圖面 key 在兩個倍率都存在，顯示欄數符合 cond，並印出欄數。
func wantBand(t *testing.T, s *ShotResult, key, what string, cond func(int) bool) {
	t.Helper()
	for _, sc := range []int{2, 3} {
		n := bandShown(s, sc, key)
		t.Logf("shot %s %d 倍：%s 顯示中文 %d 欄", s.Name, sc, key, n)
		if !contains(s.ArtKeys[sc], key) && !cond(0) {
			t.Errorf("shot %s %d 倍：沒有圖面 %s（%v）", s.Name, sc, key, s.ArtKeys[sc])
			continue
		}
		if !cond(n) {
			t.Errorf("shot %s %d 倍：%s 顯示 %d 欄，要%s", s.Name, sc, key, n, what)
		}
	}
}

func positive(n int) bool { return n > 0 }
func zero(n int) bool     { return n == 0 }

func checkMenuCase(t *testing.T, name string, mc menuCase, res *ReceiptResult) {
	t.Helper()
	if len(res.Arts) != len(mc.want) {
		t.Errorf("繪圖事件 %d 筆，要 %d", len(res.Arts), len(mc.want))
	}
	for i, w := range mc.want {
		if i >= len(res.Arts) {
			break
		}
		a := res.Arts[i]
		if a.Steps != w.steps || a.File != w.file || a.K != w.k || a.X != w.x || a.Y != w.y {
			t.Errorf("第 %d 筆事件：第 %d 步 %v k%d (%d,%d)，要第 %d 步 %v k%d (%d,%d)", i, a.Steps, a.File, a.K, a.X, a.Y, w.steps, w.file, w.k, w.x, w.y)
		}
	}
	f := loadMenuFiles(t)
	for _, s := range res.Shots {
		for _, sc := range []int{2, 3} {
			cc := s.Check[sc]
			t.Logf("shot %s（第 %d 步）%d 倍：差異 %d、框外 %d，圖面 %v", s.Name, s.Steps, sc, cc.Diffs, cc.Outside, s.ArtKeys[sc])
			if cc.Outside != 0 {
				t.Errorf("shot %s %d 倍：字帶外有 %d 個差異像素", s.Name, sc, cc.Outside)
			}
			checkMenuColumns(t, f, res, s, sc)
		}
	}
	shot := func(n string) *ShotResult { return shotNamed(t, res, n) }
	plate := func(n int) string { return fmt.Sprintf("mainmenu-plate/%d", n) }
	btn := func(n int) string { x, y := tMMPos(n); return fmt.Sprintf("mainmenu/%d,%d/%d", x, y, n) }
	switch name {
	case "L":
		// 2440M：CONTINUE、LOAD 灰色中文；k2 到 k4 按鈕中文。
		s := shot("menu")
		wantBand(t, s, plate(0), "大於 0", positive)
		wantBand(t, s, plate(1), "大於 0", positive)
		for n := 2; n <= 4; n++ {
			wantBand(t, s, btn(n), "大於 0", positive)
		}
		// 2531M（EXIT 後重畫的事件在 2530421954 到 2530469068）：§4.2 預期灰字列與 k2 到 k4 顯示欄數大於 0，
		// 但這一步最後一次回掃的原版畫格整張是色號 0（主選單不在畫面上，實作首次取得的證據，寫進報告）。
		// 這裡斷言觀察到的事實：畫格全 0，圖面全停畫（合成圖等於原版放大圖），並印出各列的顯示欄數。
		s = shot("redraw")
		zeros := 0
		for _, c := range s.Indexed {
			if c == 0 {
				zeros++
			}
		}
		t.Logf("shot redraw：原版畫格色號 0 的像素 %d／64000", zeros)
		if zeros != len(s.Indexed) {
			t.Errorf("shot redraw：原版畫格不是整張色號 0（%d／64000），要重新判讀 §4.2 的 2531M 期望", zeros)
		}
		for _, key := range []string{plate(0), plate(1), btn(2), btn(3), btn(4)} {
			wantBand(t, s, key, "0（畫格全 0）", zero)
		}
		// 2452M：VIEW OBJECTS 按下態中文（該 Key 最後一筆事件是 k9）。
		s = shot("press_view")
		wantBand(t, s, btn(4), "大於 0", positive)
		if ev := lastEventBefore(res, s.Steps, btn(4)); ev == nil || ev.K != 9 {
			t.Errorf("shot press_view：%s 最後一筆事件 %+v，要 k9", btn(4), ev)
		}
		// 進 OBJECT VIEWER 後主選單圖面全停畫：主選單矩形 (45,24)–(277,127) 內合成圖等於原版放大圖。
		for _, sn := range []string{"objview", "exit_press"} {
			s := shot(sn)
			for _, sc := range []int{2, 3} {
				if n := diffsIn(s.Composites[sc], s.Plain[sc], sc, 45, 24, 278, 128); n != 0 {
					t.Errorf("shot %s %d 倍：主選單矩形內差 %d 像素，要 0（圖面 %v）", sn, sc, n, s.ArtKeys[sc])
				}
			}
		}
	case "M1":
		s := shot("menu")
		wantBand(t, s, plate(0), "大於 0", positive)
		wantBand(t, s, btn(1), "大於 0", positive)
		for n := 2; n <= 4; n++ {
			wantBand(t, s, btn(n), "大於 0", positive)
		}
	case "M2":
		s := shot("menu")
		for n := 0; n <= 4; n++ {
			wantBand(t, s, btn(n), "大於 0", positive)
		}
		// 讀檔對話框（第 42 組）：記 skip，不產生 SAVELOAD 事件與圖面（§5）。
		g2 := 0
		for _, e := range res.Log {
			if e.Kind == "art" && strings.Contains(e.Detail, "第 2 組") {
				g2++
			}
		}
		t.Logf("SAVELOAD 第 2 組 skip 紀錄 %d 筆", g2)
		if g2 == 0 {
			t.Error("讀檔對話框沒有第 2 組的 skip 紀錄")
		}
		for _, sn := range []string{"after_load", "after_load2"} {
			for _, sc := range []int{2, 3} {
				for _, k := range shot(sn).ArtKeys[sc] {
					if strings.HasPrefix(k, "saveload/") {
						t.Errorf("shot %s %d 倍：有 SAVELOAD 圖面 %s", sn, sc, k)
					}
				}
			}
		}
	case "N":
		s := shot("menu")
		wantBand(t, s, "tomenu/89,33/0", "大於 0", positive)
		wantBand(t, s, "tomenu/90,71/1", "大於 0", positive)
		s = shot("press_sc")
		wantBand(t, s, "tomenu/89,33/0", "大於 0", positive)
		if ev := lastEventBefore(res, s.Steps, "tomenu/89,33/0"); ev == nil || ev.K != 2 {
			t.Errorf("shot press_sc：最後一筆事件 %+v，要 k2", ev)
		}
		// 放開後進 OPTTEST：圖面 0 欄顯示（或已被全螢幕 blit 清除）。
		for _, sn := range []string{"after_up", "after_up2"} {
			wantBand(t, shot(sn), "tomenu/89,33/0", "0", zero)
			wantBand(t, shot(sn), "tomenu/90,71/1", "0", zero)
		}
	}
	if dir := os.Getenv(scOutEnv); dir != "" {
		out := filepath.Join(dir, "case-"+strings.ToLower(name))
		for i := range res.Shots {
			if err := writeSaveLoadShot(out, &res.Shots[i]); err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("shot 輸出寫到 %s", out)
	}
}

func TestMenuCase(t *testing.T) {
	for _, e := range []string{scRootEnv, scRootSav1Env, scRootSav2Env, scStateDirEnv, scFontDirEnv, scCatalogEnv, scGfxCatalogEnv, i018Env} {
		if os.Getenv(e) == "" {
			t.Skip(e + " 未設定：008 §4.2 需要三個設定樹、狀態檔、圖集、兩份正式 catalog 與 i018")
		}
	}
	for _, name := range []string{"L", "M1", "M2", "N"} {
		mc := menuCases[name]
		t.Run(name, func(t *testing.T) {
			t0 := time.Now()
			on := runSaveLoadCaseRoot(t, mc.c, true, mc.rootEnv)
			t.Logf("開：%d 步，耗時 %v", on.Digest.Steps-on.Boot.Steps, time.Since(t0))
			for _, a := range on.Arts {
				t.Logf("繪圖事件 第 %d 步：%v k%d (%d,%d) Order %d", a.Steps, a.File, a.K, a.X, a.Y, a.Order)
			}
			checkMenuCase(t, name, mc, on)
			writeStop(t, "case-"+strings.ToLower(name), on)
			t1 := time.Now()
			off := runSaveLoadCaseRoot(t, mc.c, false, mc.rootEnv)
			t.Logf("關：%d 步，耗時 %v", off.Digest.Steps-off.Boot.Steps, time.Since(t1))
			a, b := on.Digest, off.Digest
			t.Logf("開：Memory %x CPU %x Indexed %x Palette %x", a.MemorySHA256, a.CPUSHA256, a.IndexedSHA256, a.PaletteSHA256)
			t.Logf("關：Memory %x CPU %x Indexed %x Palette %x", b.MemorySHA256, b.CPUSHA256, b.IndexedSHA256, b.PaletteSHA256)
			if a.Steps != b.Steps || a.MemorySHA256 != b.MemorySHA256 || a.CPUSHA256 != b.CPUSHA256 ||
				a.IndexedSHA256 != b.IndexedSHA256 || a.PaletteSHA256 != b.PaletteSHA256 || a.ExtSHA256 != b.ExtSHA256 {
				t.Fatal("圖面開、關的 Digest 不同")
			}
		})
	}
}
