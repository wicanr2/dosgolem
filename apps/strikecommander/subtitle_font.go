package strikecommander

import (
	"fmt"
	"os"

	"github.com/wicanr2/dosgolem/xlate"
)

// 語音字幕的 10×10 字型 SC-SUB10（strike_commander 規格 010 第 4 版 §3.9、§3.12）。
//
// 字型以 Fusion Pixel Font 10px 等寬 zh_tw 為基礎，由 tools/font/build_sub10.sh 轉成 GOLEMFNT 子集。
// 上游授權宣告保留字型名稱，所以產物檔名、xlate.Font.Name、收據的 font_name 一律用本專案的名字 SubtitleFontName。
// 16×16 的 LabelFont（001 到 008）與 12×12 的 HUDFont（009）各自一個型別，互不替代。

// SubtitleFontName 是 SC-SUB10 的字型名稱（010 §3.9「字型名稱」）：LoadSubtitleFont 的 name 與收據的 font_name。
const SubtitleFontName = "SC-SUB10"

// subFontSize 是 SC-SUB10 的字模尺寸（010 §3.7）。
const subFontSize = 10

// SubtitleFont 是語音字幕用的 10×10 字型與它的寬度表。
type SubtitleFont struct {
	Font *xlate.Font
	Wide *WideTable
}

// ParseSubtitleFont 以同一份位元組建出 xlate.Font 與寬度表。字型必須是 10×10，不符回錯（16×16、12×12 都拒絕）。
func ParseSubtitleFont(b []byte, name string) (*SubtitleFont, error) {
	f, err := xlate.ParseFont(b)
	if err != nil {
		return nil, err
	}
	if f.W != subFontSize || f.H != subFontSize {
		return nil, fmt.Errorf("strikecommander: 字幕字型 %dx%d，要 %dx%d", f.W, f.H, subFontSize, subFontSize)
	}
	wt, err := ParseWide(b)
	if err != nil {
		return nil, err
	}
	f.Name = name
	return &SubtitleFont{Font: f, Wide: wt}, nil
}

// LoadSubtitleFont 讀檔後呼叫 ParseSubtitleFont。呼叫端的 name 一律傳 SubtitleFontName。
func LoadSubtitleFont(path, name string) (*SubtitleFont, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseSubtitleFont(b, name)
}
