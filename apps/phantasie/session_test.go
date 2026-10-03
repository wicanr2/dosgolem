package phantasie

import (
	"os"
	"path/filepath"
	"testing"
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
