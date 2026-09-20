// Package xlate 是放大畫布上的通用轉譯疊字層（docs/spec/202-translation-overlay.md）。
//
// 只認識原版的色號／RGB 畫面與字型點陣圖，不認識任何遊戲的位址、字串來源或文本格式——
// 「印字常式在哪」「這一行要換成什麼字」由遊戲 adapter 決定；
// 這裡只管排版、定色、判斷疊字是否還有效、捲動與畫字，換一款遊戲照樣成立。
//
// 本套件不 import internal/…，也不含任何遊戲位址或原版素材。
package xlate

// DefaultScreenW、DefaultScreenH 是 Layer.W、Layer.H 留 0 時的預設原版畫面尺寸
// （spec 202 §2.3：「Layer.W、H 是原版畫面大小，0 當 320×200」）。
const (
	DefaultScreenW = 320
	DefaultScreenH = 200
)

// width、height 回這一層實際使用的原版畫面尺寸：Layer.W／H 是 0 時退回預設值。
func (l *Layer) width() int {
	if l.W > 0 {
		return l.W
	}
	return DefaultScreenW
}

func (l *Layer) height() int {
	if l.H > 0 {
		return l.H
	}
	return DefaultScreenH
}
