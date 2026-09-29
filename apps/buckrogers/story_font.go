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

// setStoryLayerFont 把 layer 內既有 stamp 改指向新字型；stamp 數與順序不變。
func setStoryLayerFont(l *xlate.Layer, f *xlate.Font) {
	for _, s := range l.Stamps {
		s.Font = f
	}
}

func (o *RuntimeStoryOpeningOverlay) SetFont(base *xlate.Font) error {
	if o == nil {
		return fmt.Errorf("buckrogers: 首屏 SetFont 無 presenter")
	}
	n, err := NewRuntimeStoryOpeningOverlay(o.text, base, o.scale)
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
	n, err := NewRuntimeStoryPage2Overlay(o.text, base, o.scale)
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
	n, err := NewRuntimeStoryPage3Overlay(o.text, base, o.scale)
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
	n, err := NewRuntimeStoryPage4Overlay(o.text, base, o.scale)
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
	n, err := NewRuntimeStoryPage5Overlay(o.text, base, o.scale)
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
	n, err := NewRuntimeStoryPage6Overlay(o.text, base, o.scale)
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
	n, err := NewRuntimeStoryPage7Overlay(o.text, base, o.scale)
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
	n, err := NewRuntimeStoryPage8Overlay(o.text, base, o.scale)
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
	n, err := NewRuntimeStoryPage9Overlay(map[string]string{"story.page9.line.001": o.text}, base, o.scale)
	if err != nil {
		return err
	}
	o.font = n.font
	setStoryLayerFont(o.layer, o.font)
	return nil
}
