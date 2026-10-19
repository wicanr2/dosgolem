package strikecommander

import (
	"os"
	"testing"
)

// strike_commander 規格 007 §4.2 到 §4.4 的案例 I、J、K 補充斷言（長測試本體在 saveload_long_test.go 的 TestSaveLoadCase）。
// 事件步數、差異範圍、游標欄、Digest 由 checkSaveLoadCase 與 TestSaveLoadCase 斷言；這裡是各 shot 的特定期望。

var slExtra = map[string]func(*testing.T, *ReceiptResult){
	"I": checkCaseI007,
	"J": checkCaseJ007,
	"K": checkCaseK007,
}

const (
	keyK2  = "saveload/134,127/2"
	keyK15 = "saveload/101,85/15"
	keyK16 = "saveload/119,96/16"
	keyK17 = "saveload/119,110/17"
	keyK19 = "saveload/170,110/19"
)

func shotNamed(t *testing.T, res *ReceiptResult, name string) *ShotResult {
	t.Helper()
	for i := range res.Shots {
		if res.Shots[i].Name == name {
			return &res.Shots[i]
		}
	}
	t.Fatalf("沒有 shot %s", name)
	return nil
}

// shownCols 回字帶 (x, y, w, h)（1×）中合成圖與原版放大圖不同的欄數（顯示中文的欄）。
func shownCols(s *ShotResult, sc, x, y, w, h int) int {
	n := 0
	for col := 0; col < w; col++ {
		if diffsIn(s.Composites[sc], s.Plain[sc], sc, x+col, y, x+col+1, y+h) != 0 {
			n++
		}
	}
	return n
}

// hasConfirmKey 回 keys 是否有一般態屬於 {15, 16, 17, 19} 的確認框圖面。
func hasConfirmKey(keys []string) []string {
	var out []string
	for _, k := range keys {
		if k == keyK15 || k == keyK16 || k == keyK17 || k == keyK19 {
			out = append(out, k)
		}
	}
	return out
}

// wantShown 斷言 shot 的某列圖面在兩個倍率都有顯示中文的欄，並印出欄數。
func wantShown(t *testing.T, s *ShotResult, key string, x, y, w, h int) {
	t.Helper()
	for _, sc := range []int{2, 3} {
		if !contains(s.ArtKeys[sc], key) {
			t.Errorf("shot %s %d 倍：沒有圖面 %s（%v）", s.Name, sc, key, s.ArtKeys[sc])
			continue
		}
		n := shownCols(s, sc, x, y, w, h)
		t.Logf("shot %s %d 倍：%s 顯示中文 %d／%d 欄", s.Name, sc, key, n, w)
		if n == 0 {
			t.Errorf("shot %s %d 倍：%s 沒有顯示中文", s.Name, sc, key)
		}
	}
}

// wantNoConfirm 斷言 shot 的圖面沒有確認框的列。
func wantNoConfirm(t *testing.T, s *ShotResult) {
	t.Helper()
	for _, sc := range []int{2, 3} {
		if k := hasConfirmKey(s.ArtKeys[sc]); len(k) != 0 {
			t.Errorf("shot %s %d 倍：還有確認框圖面 %v", s.Name, sc, k)
		}
	}
}

// lastEventBefore 回 shot 前最後一筆 Key 為 key 的事件。
func lastEventBefore(res *ReceiptResult, steps uint64, key string) *ArtEvent {
	var out *ArtEvent
	for i := range res.Arts {
		if res.Arts[i].Steps < steps && artKey(res.Arts[i]) == key {
			out = &res.Arts[i]
		}
	}
	return out
}

func checkCaseI007(t *testing.T, res *ReceiptResult) {
	// confirm：k15 提示、k17、k19 顯示中文；k2 字帶（007 §3.9 下移後）顯示中文。
	s := shotNamed(t, res, "confirm")
	wantShown(t, s, keyK15, 119, 96, 95, 9)
	wantShown(t, s, keyK17, 123, 111, 38, 9)
	wantShown(t, s, keyK19, 174, 111, 36, 9)
	wantShown(t, s, keyK2, 154, 129, 40, 9)
	// nopressed：k20 字帶游標欄停畫由 checkCursorColumns 斷言；這裡確認 k20 是最後一筆。
	s = shotNamed(t, res, "nopressed")
	if ev := lastEventBefore(res, s.Steps, keyK19); ev == nil || ev.K != 20 {
		t.Errorf("shot nopressed：%s 最後一筆不是 k20：%+v", keyK19, ev)
	}
	wantShown(t, s, keyK19, 174, 111, 36, 9)
	// no10：確認框圖面已移除。
	wantNoConfirm(t, shotNamed(t, res, "no10"))
	// savepressed：多一次 k3 事件，k3 字帶顯示中文（扣游標欄）。
	s = shotNamed(t, res, "savepressed")
	wantNoConfirm(t, s)
	if ev := lastEventBefore(res, s.Steps, keyK2); ev == nil || ev.K != 3 || ev.Steps < 4825000000 {
		t.Errorf("shot savepressed：%s 最後一筆不是 4825M 步之後的 k3：%+v", keyK2, ev)
	} else {
		t.Logf("SAVE 按下態 k3 事件：第 %d 步", ev.Steps)
	}
	wantShown(t, s, keyK2, 154, 129, 40, 9)
}

func checkCaseJ007(t *testing.T, res *ReceiptResult) {
	file := loadSHP(t)
	font, err := LoadLabelFont(os.Getenv(scFontEnv), "full16-zh-TW")
	if err != nil {
		t.Fatal(err)
	}
	gb, err := os.ReadFile(os.Getenv(scGfxCatalogEnv))
	if err != nil {
		t.Fatal(err)
	}
	gfx, err := ParseCatalog(gb)
	if err != nil {
		t.Fatal(err)
	}
	// ow、ow2：k15 字帶 (119,96)–(213,104) 的輸出像素等於只有 k16 圖面時的合成（不會漏出「確定嗎」）。
	for _, name := range []string{"ow", "ow2"} {
		s := shotNamed(t, res, name)
		wantShown(t, s, keyK16, 119, 96, 95, 9)
		wantShown(t, s, keyK2, 154, 129, 40, 9)
		for _, sc := range []int{2, 3} {
			if !contains(s.ArtKeys[sc], keyK15) {
				t.Errorf("shot %s %d 倍：沒有 k15 圖面", name, sc)
			}
			ov, err := NewOverlay(font, nil, gfx, sc)
			if err != nil {
				t.Fatal(err)
			}
			ov.AddDialogArt(mkEvent(t, file, 16, 119, 96, 1))
			only16, _ := ov.Draw(s.Indexed, s.Palette)
			if n := diffsIn(s.Composites[sc], only16, sc, 119, 96, 214, 105); n != 0 {
				t.Errorf("shot %s %d 倍：k15 字帶內有 %d 個輸出像素不等於 k16 圖面", name, sc, n)
			}
		}
	}
	// nopressed、no10 同案例 I。
	if ev := lastEventBefore(res, shotNamed(t, res, "nopressed").Steps, keyK19); ev == nil || ev.K != 20 {
		t.Errorf("shot nopressed：%s 最後一筆不是 k20：%+v", keyK19, ev)
	}
	wantNoConfirm(t, shotNamed(t, res, "no10"))
	// yespressed：k18 字帶顯示中文（扣游標欄）。
	s := shotNamed(t, res, "yespressed")
	if ev := lastEventBefore(res, s.Steps, keyK17); ev == nil || ev.K != 18 {
		t.Errorf("shot yespressed：%s 最後一筆不是 k18：%+v", keyK17, ev)
	}
	wantShown(t, s, keyK17, 123, 111, 38, 9)
	// yesredraw：k0 重畫（4995005984）之後、全螢幕 blit 之前；圖面沒有確認框的列。畫面內容照實記錄。
	s = shotNamed(t, res, "yesredraw")
	wantNoConfirm(t, s)
	for _, c := range res.ArtClears {
		if c > 4995005984 && c < s.Steps {
			t.Errorf("shot yesredraw 之前（第 %d 步）已有圖面清除", c)
		}
	}
	k15, k0 := mustDecode(t, file, 15), mustDecode(t, file, 0)
	eq15, eq0, tot := 0, 0, 0
	for y := 85; y <= 128; y++ {
		for x := 101; x <= 232; x++ {
			v := s.Indexed[y*320+x]
			tot++
			if v == k15.Pix[(y-85)*k15.W+x-101] {
				eq15++
			}
			if v == k0.Pix[(y-30)*k0.W+x-80] {
				eq0++
			}
		}
	}
	t.Logf("yesredraw（第 %d 步）：確認框矩形 %d 像素，等於 k15 解碼 %d、等於 k0 底板解碼 %d；圖面 %v；2 倍差異 %d、框外 %d",
		s.Steps, tot, eq15, eq0, s.ArtKeys[2], s.Check[2].Diffs, s.Check[2].Outside)
	t.Logf("圖面清除步數：%v", res.ArtClears)
}

func checkCaseK007(t *testing.T, res *ReceiptResult) {
	file := loadSHP(t)
	s := shotNamed(t, res, "tq")
	wantShown(t, s, keyK15, 119, 96, 95, 9)
	wantShown(t, s, keyK17, 123, 111, 38, 9)
	wantShown(t, s, keyK19, 174, 111, 36, 9)
	wantShown(t, s, keyK2, 154, 129, 40, 9)
	if ev := lastEventBefore(res, shotNamed(t, res, "tnopressed").Steps, keyK19); ev == nil || ev.K != 20 || ev.G != 1 {
		t.Errorf("shot tnopressed：%s 最後一筆不是組 1 k20：%+v", keyK19, ev)
	}
	wantNoConfirm(t, shotNamed(t, res, "tno"))
	// 007 §3.4 的 21 組字色與高光：用執行期事件的組與 k 解碼。
	want := map[int][3]int{15: {31, 87, 1}, 17: {31, 87, 1}, 19: {44, 0, 0}, 20: {44, 0, 0}}
	for _, a := range res.Arts {
		w, ok := want[a.K]
		if !ok {
			continue
		}
		if a.G != 1 {
			t.Errorf("第 %d 步 k%d：組 %d，要 1", a.Steps, a.K, a.G)
			continue
		}
		c, h, hasH := dialogColors(mkEvent(t, file, 21+a.K, a.X, a.Y, a.Order))
		if !hasH {
			h = 0
		}
		if int(c) != w[0] || int(h) != w[1] || hasH != (w[2] == 1) {
			t.Errorf("第 %d 步 e %d：c %d h %d（高光 %t），要 %d %d", a.Steps, 21+a.K, c, h, hasH, w[0], w[1])
		}
	}
}
