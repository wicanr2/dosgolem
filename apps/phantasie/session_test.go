package phantasie

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// 工作階段冒煙測試（docs/spec/005 §3）：需要原版（缺檔 SKIP，不算驗收）。
// 容器內路徑：DOSGOLEM_TEST_ROOT 是原版目錄（例如 /orig/phantasi），
// /phantasie-data/text 與 /phantasie-data/fonts 由 DOSGOLEM_EXTRA_MOUNT 掛入（字型由 tools/build_fonts.sh 建置）。
func TestSessionSmoke(t *testing.T) {
	root := os.Getenv("DOSGOLEM_TEST_ROOT")
	if root == "" {
		t.Skip("未設 DOSGOLEM_TEST_ROOT：沒有原版，跳過（不算驗收）")
	}
	data := "/phantasie-data"
	if _, err := os.Stat(filepath.Join(data, "fonts", "zh-TW.golemfnt")); err != nil {
		t.Skipf("沒有 %s/fonts/zh-TW.golemfnt：跳過（不算驗收）", data)
	}
	s, err := StartSession(SessionOptions{
		Root: root, TextDir: filepath.Join(data, "text"), FontDir: filepath.Join(data, "fonts"),
		Langs: []string{"zh-TW", "zh-CN", "ja", "ko"}, Scratch: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if len(s.Failed) != 0 {
		t.Fatalf("語言載入失敗：%v", s.Failed)
	}
	// 跑到標題選單的第一次讀鍵（與 tests/routes/title.route 的檢查點同一點）。
	if err := s.O.Run(6_500_000); err != nil {
		t.Fatal(err)
	}
	s.Frame()
	if !s.Hk.Armed() || s.Hk.Failed() {
		t.Fatalf("鉤子未就緒：armed=%v 診斷=%q", s.Hk.Armed(), s.Hk.Diag)
	}
	if n := len(s.Ov.Layer.Stamps); n == 0 {
		t.Fatalf("標題畫面沒有疊字；計數：%s", s.Ov.C)
	}
	if got := s.Ov.C.Get("unpaired"); got != 0 {
		t.Errorf("unpaired = %d，期望 0", got)
	}
	cyc := s.DisplayCycle()
	want := []string{"zh-TW", "zh-CN", "en", "ja", "ko"}
	if len(cyc) != len(want) {
		t.Fatalf("語言循環 %v，期望 %v", cyc, want)
	}
	for i := range want {
		if cyc[i] != want[i] {
			t.Fatalf("語言循環 %v，期望 %v", cyc, want)
		}
	}
	for _, w := range want[1:] {
		got, err := s.NextDisplay()
		if err != nil || got != w {
			t.Fatalf("NextDisplay = %q, %v，期望 %q", got, err, w)
		}
		s.Frame()
		if w != "en" && len(s.Ov.Layer.Stamps) == 0 {
			t.Errorf("切到 %s 後疊字消失", w)
		}
	}
}

// 真實 FONT 的 recolor 驗證（docs/spec/001 §10 第 1 項）：用遊戲記憶體內的 FONT 為 MONK、NO、HALBERD、MAGIC 1
// 建出畫面（墨色 3、底色 0，以及反白後墨色 0、底色 3），recolor 得到的 FG、BG 必須與畫面一致，
// 且 xlate.Colors 的多數色規則在 MONK 上判反（對照：兩種規則確實不同）。需要原版，缺檔 SKIP。
func TestSessionRealFontRecolor(t *testing.T) {
	root := os.Getenv("DOSGOLEM_TEST_ROOT")
	if root == "" {
		t.Skip("未設 DOSGOLEM_TEST_ROOT：沒有原版，跳過（不算驗收）")
	}
	data := "/phantasie-data"
	if _, err := os.Stat(filepath.Join(data, "fonts", "zh-TW.golemfnt")); err != nil {
		t.Skipf("沒有 %s/fonts/zh-TW.golemfnt：跳過（不算驗收）", data)
	}
	s, err := StartSession(SessionOptions{Root: root, TextDir: filepath.Join(data, "text"),
		FontDir: filepath.Join(data, "fonts"), Langs: []string{"zh-TW"}, Scratch: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.O.Run(6_500_000); err != nil {
		t.Fatal(err)
	}
	s.Frame()
	font := s.Ov.font
	if len(font) != FontSize {
		t.Fatalf("遊戲的 FONT 長度 %d，期望 %d", len(font), FontSize)
	}
	black, white := [3]uint8{0, 0, 0}, [3]uint8{255, 255, 255}
	for _, text := range []string{"MONK", "NO", "HALBERD", "MAGIC 1"} {
		for _, inverted := range []bool{false, true} {
			ink, bg := uint8(3), uint8(0)
			if inverted {
				ink, bg = 0, 3
			}
			ov := NewOverlay()
			ov.SetFont(font)
			const col, row = 3, 5
			rec := &EventRecord{ID: "g1", Col: col, Row: row, Text: text, Cells: []byte(text)}
			ov.records["g1"] = rec
			indexed := make([]uint8, screenW*screenH)
			rgb := make([]uint8, screenW*screenH*3)
			var region []uint8
			for y := row * 8; y < row*8+8; y++ {
				for x := col * 8; x < (col+len(text))*8; x++ {
					k := (x - col*8) / 8
					c := bg
					if ov.glyphPixel(text[k], y-row*8, (x-col*8)%8) != 0 {
						c = ink
					}
					indexed[y*screenW+x] = c
					region = append(region, c)
				}
			}
			for i, c := range indexed {
				v := black
				if c == 3 {
					v = white
				}
				rgb[3*i], rgb[3*i+1], rgb[3*i+2] = v[0], v[1], v[2]
			}
			st := &xlate.Stamp{Key: "g1", X: col * 8, Y: row * 8, Cells: 2 * len(text), CellW: 4, CellH: 8, State: xlate.Pending}
			ov.Layer.Stamps = []*xlate.Stamp{st}
			ov.Frame(indexed, rgb)
			wantFG, wantBG := black, white
			if !inverted {
				wantFG, wantBG = white, black
			}
			if st.FG != wantFG || st.BG != wantBG {
				t.Errorf("%q 反白=%v：FG=%v BG=%v，期望 FG=%v BG=%v（recolor_fallback=%d）",
					text, inverted, st.FG, st.BG, wantFG, wantBG, ov.C.Get("recolor_fallback"))
			}
			if text == "MONK" && !inverted {
				mbg, _ := xlate.Colors(region)
				if mbg != ink {
					t.Errorf("對照失敗：xlate.Colors 在 MONK 上應判反（背景判成墨色 %d），得 %d", ink, mbg)
				}
			}
		}
	}
}

// KeyGate 的去重與讀鍵計數（docs/spec/005 §2、§8）：需要原版，缺檔 SKIP。
func TestSessionKeyGate(t *testing.T) {
	root := os.Getenv("DOSGOLEM_TEST_ROOT")
	if root == "" {
		t.Skip("未設 DOSGOLEM_TEST_ROOT：沒有原版，跳過（不算驗收）")
	}
	data := "/phantasie-data"
	if _, err := os.Stat(filepath.Join(data, "fonts", "zh-TW.golemfnt")); err != nil {
		t.Skipf("沒有 %s/fonts/zh-TW.golemfnt：跳過（不算驗收）", data)
	}
	s, err := StartSession(SessionOptions{Root: root, TextDir: filepath.Join(data, "text"),
		FontDir: filepath.Join(data, "fonts"), Langs: []string{"zh-TW"}, Scratch: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// 標題選單的第一次讀鍵之後，送一個 Return（進城鎮）再一個 Esc。
	s.Gate.Press(0, "Return")
	s.Gate.PressAfterReads(2, "Esc")
	if err := s.O.Run(8_000_000); err != nil {
		t.Fatal(err)
	}
	if s.Gate.Err != "" {
		t.Fatalf("讀鍵函式簽章檢查失敗：%s", s.Gate.Err)
	}
	if s.Gate.Gated != 2 || s.Gate.Pending() != 0 {
		t.Fatalf("送出 %d 個鍵、還有 %d 個待送，期望 2 與 0", s.Gate.Gated, s.Gate.Pending())
	}
	if s.Gate.Reads < 4 {
		t.Fatalf("讀鍵入口只有 %d 次，期望至少 4 次（第一個鍵、跳過 2 次、第二個鍵）", s.Gate.Reads)
	}
	if s.O.KeysConsumed() > s.Gate.Gated {
		t.Fatalf("原版讀走 %d 個鍵，多於送出的 %d 個（重複送鍵）", s.O.KeysConsumed(), s.Gate.Gated)
	}
}
