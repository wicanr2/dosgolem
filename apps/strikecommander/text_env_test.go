package strikecommander

import (
	"os"
	"sync"
	"testing"
)

// 長測試的圖集環境（strike_commander 規格 011 §3.9）：DOSGOLEM_SC_FONT_DIR 與 DOSGOLEM_SC_FONT_EXPECTED。

var envText struct {
	once  sync.Once
	fonts *TextFonts
	err   error
}

// applyTextEnv 以 DOSGOLEM_SC_FONT_DIR 的圖集設定長測試的 ReceiptConfig.Fonts（預期表取 DOSGOLEM_SC_FONT_EXPECTED）。
// 長測試的 skip 條件已要求 DOSGOLEM_SC_FONT_DIR；讀不到圖集或雜湊不符時失敗。圖集只載入一次。
func applyTextEnv(t *testing.T, cfg *ReceiptConfig) {
	t.Helper()
	envText.once.Do(func() {
		envText.fonts, envText.err = LoadTextFonts(os.Getenv(scFontDirEnv), os.Getenv(scFontExpectedEnv))
	})
	if envText.err != nil {
		t.Fatal(envText.err)
	}
	cfg.Fonts = envText.fonts
	t.Logf("圖集 anchor %s", cfg.Fonts.Anchor)
}

// writeFontsTSV 寫 <dir>/<prefix>-fonts.tsv（011 §3.8）；圖集還沒載入時不寫。
func writeFontsTSV(t *testing.T, dir, prefix string) {
	t.Helper()
	if envText.fonts == nil {
		return
	}
	if err := WriteFontsTSV(dir, prefix, envText.fonts, os.Getenv(scFontExpectedEnv)); err != nil {
		t.Fatal(err)
	}
}
