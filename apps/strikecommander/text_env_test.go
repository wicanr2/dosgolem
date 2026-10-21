package strikecommander

import (
	"fmt"
	"os"
	"sync"
	"testing"
)

// 長測試的新路徑環境變數（strike_commander 規格 011 §3.9；分期 (d)）。

const scTextPathsEnv = "DOSGOLEM_SC_TEXT_PATHS"

var envText struct {
	once  sync.Once
	fonts *TextFonts
	paths TextPaths
	err   error
}

// applyTextEnv 依 DOSGOLEM_SC_TEXT_PATHS（逗號分隔的路徑名）設定長測試的 ReceiptConfig.Fonts 與 TextPaths；
// 未設定時不動（舊路徑）。設定了而讀不到圖集或雜湊不符時失敗。圖集只載入一次。
func applyTextEnv(t *testing.T, cfg *ReceiptConfig) {
	t.Helper()
	s := os.Getenv(scTextPathsEnv)
	if s == "" {
		return
	}
	envText.once.Do(func() {
		envText.paths, envText.err = ParseTextPaths(s)
		if envText.err != nil {
			return
		}
		dir := os.Getenv(scFontDirEnv)
		if dir == "" {
			envText.err = fmt.Errorf("%s 設定時要 %s", scTextPathsEnv, scFontDirEnv)
			return
		}
		envText.fonts, envText.err = LoadTextFonts(dir, os.Getenv(scFontExpectedEnv))
	})
	if envText.err != nil {
		t.Fatal(envText.err)
	}
	cfg.Fonts, cfg.TextPaths = envText.fonts, envText.paths
	t.Logf("新路徑 %+v，圖集 anchor %s", cfg.TextPaths, cfg.Fonts.Anchor)
}

// writeFontsTSV 在新路徑開著時寫 <dir>/<prefix>-fonts.tsv（011 §3.8）。
func writeFontsTSV(t *testing.T, dir, prefix string) {
	t.Helper()
	if os.Getenv(scTextPathsEnv) == "" || envText.fonts == nil {
		return
	}
	if err := WriteFontsTSV(dir, prefix, envText.fonts, os.Getenv(scFontExpectedEnv)); err != nil {
		t.Fatal(err)
	}
}
