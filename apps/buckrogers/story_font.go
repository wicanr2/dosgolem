package buckrogers

import (
	"fmt"

	"github.com/wicanr2/dosgolem/xlate"
)

// 規格 031 §3.4（Buck repo）：劇情逐頁家族在取得原版字形表後改用 `.orig-ascii`
// 衍生字型。SetFont 只換字型，不換 presenter、layer 或 stamp 身分：
//
//   - base 是 16×16 的 `.orig-ascii` 基底；各頁以自己建構子的既有倍率規則重新衍生
//     （3× 的 22×22 衍生與名稱規則、第 4 頁不衍生）並重驗缺字。做法是用同一個
//     建構子建一個暫用 presenter，只取它的字型，確保規則與建構時完全一致。
//   - 驗證全部通過後才更新 presenter 字型與 layer 內既有 stamp 的 Font；任何失敗
//     都保留原字型（失敗即不變）。
//   - generation 由 live family／receipt runner 持有，這裡不碰；下一次 Draw 即以
//     新字型畫出，不需要重新 Apply。

// 規格 039 §3.6：原版字形不再用於劇情家族；runtime 與 receipt runner 已不呼叫
// SetFont。保留 SetFont 只為 API 相容：它只換全形段字型，半形段一律用由未經 031
// 替換的底字型衍生的半形字型（建構時取得，SetFont 不改）。

// setStoryLayerFont 把 layer 內既有全形段 stamp 改指向新字型；半形段不動，
// stamp 數與順序不變。
func setStoryLayerFont(l *xlate.Layer, f *xlate.Font) {
	for _, s := range l.Stamps {
		if s.CellW != halfUnitPx {
			s.Font = f
		}
	}
}

// storyFullOnly 回傳只含全形字的譯文副本（半形字刪除；刪空的列留一個 U+3000）。
// SetFont 用它重建暫用 presenter：只驗證與衍生全形段字型，不以新底字型衍生半形字。
func storyFullOnly(text map[string]string) map[string]string {
	out := make(map[string]string, len(text))
	for k, s := range text {
		var b []rune
		for _, r := range s {
			if !isHalfwidth(r) {
				b = append(b, r)
			}
		}
		if len(b) == 0 {
			b = []rune{ideographicSpace}
		}
		out[k] = string(b)
	}
	return out
}

// storyTextCheck 驗證劇情譯文（規格 039 §3.4）：每列不超過 maxUnits 半形單位，
// 全形字與半形字各自在對應字型內（U+0020、U+3000 不需要字模）。
func storyTextCheck(text map[string]string, fonts segmentFonts, maxUnits int) error {
	for key, s := range text {
		if u := stringUnits(s); u > maxUnits {
			return fmt.Errorf("%s 譯文 %d 單位超過 %d 單位", key, u, maxUnits)
		}
		if miss := fonts.missingRunes([]rune(s)); len(miss) != 0 {
			return fmt.Errorf("缺字 U+%04X", miss[0])
		}
	}
	return nil
}

// storyRowStamps 把一列劇情譯文補位到 cells×2 單位後拆成寬度段（規格 039 §3.3），
// 段 key 為 `<列 key>#<段序>`。
func storyRowStamps(key string, x, y, cells int, text []rune, fonts segmentFonts, bg, fg [3]uint8) []*xlate.Stamp {
	return fonts.segmentStamps(key, x, y, padUnits(text, 2*cells), nil, func(int) ([3]uint8, [3]uint8) { return bg, fg })
}

func (o *RuntimeStoryOpeningOverlay) SetFont(base *xlate.Font) error {
	if o == nil {
		return fmt.Errorf("buckrogers: 首屏 SetFont 無 presenter")
	}
	n, err := NewRuntimeStoryOpeningOverlay(storyFullOnly(o.text), base, o.scale)
	if err != nil {
		return err
	}
	o.font = n.font
	setStoryLayerFont(o.layer, o.font)
	return nil
}

func (o *RuntimeStoryPage2Overlay) SetFont(base *xlate.Font) error {
	if o == nil {
		return fmt.Errorf("buckrogers: 第 2 頁 SetFont 無 presenter")
	}
	n, err := NewRuntimeStoryPage2Overlay(storyFullOnly(o.text), base, o.scale)
	if err != nil {
		return err
	}
	o.font = n.font
	setStoryLayerFont(o.layer, o.font)
	return nil
}

func (o *RuntimeStoryPage3Overlay) SetFont(base *xlate.Font) error {
	if o == nil {
		return fmt.Errorf("buckrogers: 第 3 頁 SetFont 無 presenter")
	}
	n, err := NewRuntimeStoryPage3Overlay(storyFullOnly(o.text), base, o.scale)
	if err != nil {
		return err
	}
	o.font = n.font
	setStoryLayerFont(o.layer, o.font)
	return nil
}

// SetFont：第 4 頁 3× 本來就不衍生（直接用 16×16 基底），沿用同一規則。
func (o *RuntimeStoryPage4Overlay) SetFont(base *xlate.Font) error {
	if o == nil {
		return fmt.Errorf("buckrogers: 第 4 頁 SetFont 無 presenter")
	}
	n, err := NewRuntimeStoryPage4Overlay(storyFullOnly(o.text), base, o.scale)
	if err != nil {
		return err
	}
	o.font = n.font
	setStoryLayerFont(o.layer, o.font)
	return nil
}

func (o *RuntimeStoryPage5Overlay) SetFont(base *xlate.Font) error {
	if o == nil {
		return fmt.Errorf("buckrogers: 第 5 頁 SetFont 無 presenter")
	}
	n, err := NewRuntimeStoryPage5Overlay(storyFullOnly(o.text), base, o.scale)
	if err != nil {
		return err
	}
	o.font = n.font
	setStoryLayerFont(o.layer, o.font)
	return nil
}

func (o *RuntimeStoryPage6Overlay) SetFont(base *xlate.Font) error {
	if o == nil {
		return fmt.Errorf("buckrogers: 第 6 頁 SetFont 無 presenter")
	}
	n, err := NewRuntimeStoryPage6Overlay(storyFullOnly(o.text), base, o.scale)
	if err != nil {
		return err
	}
	o.font = n.font
	setStoryLayerFont(o.layer, o.font)
	return nil
}

func (o *RuntimeStoryPage7Overlay) SetFont(base *xlate.Font) error {
	if o == nil {
		return fmt.Errorf("buckrogers: 第 7 頁 SetFont 無 presenter")
	}
	n, err := NewRuntimeStoryPage7Overlay(storyFullOnly(o.text), base, o.scale)
	if err != nil {
		return err
	}
	o.font = n.font
	setStoryLayerFont(o.layer, o.font)
	return nil
}

func (o *RuntimeStoryPage8Overlay) SetFont(base *xlate.Font) error {
	if o == nil {
		return fmt.Errorf("buckrogers: 第 8 頁 SetFont 無 presenter")
	}
	n, err := NewRuntimeStoryPage8Overlay(storyFullOnly(o.text), base, o.scale)
	if err != nil {
		return err
	}
	o.font = n.font
	setStoryLayerFont(o.layer, o.font)
	return nil
}

// SetFont：第 9 頁只換 presenter 字型；owner 與 watcher 不變（§3.4 只對
// StoryPage9Owner.Presenter 呼叫）。
func (o *RuntimeStoryPage9Overlay) SetFont(base *xlate.Font) error {
	if o == nil {
		return fmt.Errorf("buckrogers: 第 9 頁 SetFont 無 presenter")
	}
	n, err := NewRuntimeStoryPage9Overlay(storyFullOnly(map[string]string{"story.page9.line.001": o.text}), base, o.scale)
	if err != nil {
		return err
	}
	o.font = n.font
	setStoryLayerFont(o.layer, o.font)
	return nil
}
