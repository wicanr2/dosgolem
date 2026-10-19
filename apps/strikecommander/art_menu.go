package strikecommander

import "fmt"

// 主選單（MAINMENU.PAK）與 MENU 片頭選單（TOMENU.PAK）的元素、墨跡、無字底與字樣（strike_commander 規格 008 §3.2 到 §3.5）。
// 座標都是 1×、相對元素左上角、含端點。

// 008 §3.2 的 MAINMENU 按鈕：211×15，一般態 k 0 到 4、按下態 k + 5；兩態共用字帶與置中框。
const (
	mmW, mmH       = 211, 15
	plateW, plateH = 233, 104
	mmLines        = 5
)

// mmBand 是按鈕的字帶（兩態相同）。
var mmBand = box{2, 2, 208, 12}

// mmCenterX 是按鈕第 n 行置中框的 x（§3.3 遮罩 M 的外接框，兩態共用）；y 一律 3..10。
var mmCenterX = [mmLines][2]int{{22, 190}, {46, 166}, {18, 195}, {9, 203}, {30, 178}}

// mmKeys 是第 n 行的英文 key（底板灰字同一行同一 key）。
var mmKeys = [mmLines]string{"CONTINUE GAME", "LOAD GAME", "START NEW GAME", "TRAINING MISSION", "VIEW OBJECTS"}

func mmCenter(n int) box { return box{mmCenterX[n][0], 3, mmCenterX[n][1], 10} }

// mmPressed：MAINMENU 按鈕檔以 k ≥ 5 判按下態，n = k mod 5（§3.2）。不用 SAVELOAD 的 pressedState。
func mmPressed(k int) bool { return k >= mmLines }

// plateBand 是底板第 n 行的字帶（底板座標，含按鈕蓋住的第 1 列 y 11+17n，§3.2、§3.4）。
func plateBand(n int) box { return box{12, 11 + 17*n, 219, 22 + 17*n} }

// plateCenter 是底板第 n 行的置中框：x 為按鈕置中框加 11，y 13+17n..20+17n。
func plateCenter(n int) box {
	return box{mmCenterX[n][0] + 11, 13 + 17*n, mmCenterX[n][1] + 11, 20 + 17*n}
}

// tmElement 是 TOMENU 按鈕（一般態 k 0、1；按下態 k + 2，只換外框，§3.2）。
type tmElement struct {
	key          string
	band, center box
}

var tmElements = [2]tmElement{
	{"PLAY STRIKE COMMANDER", box{2, 2, 163, 23}, box{15, 4, 146, 21}},
	{"PLAY TACTICAL OPERATIONS", box{2, 2, 162, 23}, box{4, 4, 159, 21}},
}

// tmSizes 是 TOMENU 按鈕 k 0 到 3 的尺寸（k3 寬 166，字帶仍取 x 2..162）。
var tmSizes = [4][2]int{{166, 26}, {165, 26}, {166, 26}, {166, 26}}

// tmPressed：TOMENU 按鈕檔以 k ≥ 2 判按下態，n = k mod 2。
func tmPressed(k int) bool { return k >= 2 }

// menuElementSize 回檔別 kind 條目 e 的元素尺寸；不產生事件的條目回 false（§3.1 第 5 條）。
func menuElementSize(kind ArtFile, e int) (w, h int, ok bool) {
	switch kind {
	case ArtMainMenuButtons:
		if e >= 0 && e < 2*mmLines {
			return mmW, mmH, true
		}
	case ArtMainMenuPlate:
		if e == 0 {
			return plateW, plateH, true
		}
	case ArtTomenuButtons:
		if e >= 0 && e < len(tmSizes) {
			return tmSizes[e][0], tmSizes[e][1], true
		}
	}
	return 0, 0, false
}

// bMainMenu 是 MAINMENU 的底色集合 B = {177..181}（§3.3）。
var bMainMenu = func() (b [256]bool) {
	for c := 177; c <= 181; c++ {
		b[c] = true
	}
	return
}()

// bTomenu 是 TOMENU 的 B_T（§3.3）：木紋與亮紋。補集是字面色 {4..18, 20, 21, 178, 183, 193} 與 24。
var bTomenu = [256]bool{23: true, 25: true, 26: true, 28: true, 29: true, 31: true,
	186: true, 187: true, 188: true, 189: true, 190: true}

// ruleP 是 §3.3 規則 P：一般態 normal 與按下態 pressed（211×15）互為參照。回遮罩 M（211×15，逐列）
// 與兩態共用的無字底（mmBand 大小，逐列）。
//  1. D：字帶內兩態色號不同的點。
//  2. M：D 外接框內、色號不屬 B、八鄰接 D 的點併入一次（不在 D 的點兩態同色）。
//  3. 回填：M 內每點換成同列、字帶內、不在 M 的最近像素，等距取左；沒有退回。
func ruleP(normal, pressed []uint8) (mask []bool, bg []uint8) {
	w, b := mmW, mmBand
	d := make([]bool, w*mmH)
	x0, y0, x1, y1 := w, mmH, -1, -1
	for y := b.y0; y <= b.y1; y++ {
		for x := b.x0; x <= b.x1; x++ {
			if normal[y*w+x] != pressed[y*w+x] {
				d[y*w+x] = true
				x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
			}
		}
	}
	mask = append([]bool(nil), d...)
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if d[y*w+x] || bMainMenu[normal[y*w+x]] {
				continue
			}
		adj:
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nx, ny := x+dx, y+dy
					if (dx != 0 || dy != 0) && nx >= 0 && nx < w && ny >= 0 && ny < mmH && d[ny*w+nx] {
						mask[y*w+x] = true
						break adj
					}
				}
			}
		}
	}
	bw := b.x1 - b.x0 + 1
	bg = make([]uint8, bw*(b.y1-b.y0+1))
	for y := b.y0; y <= b.y1; y++ {
		for x := b.x0; x <= b.x1; x++ {
			c := normal[y*w+x]
			if mask[y*w+x] {
				c = nearestRow(normal, mask, w, y, x, b.x0, b.x1, c)
			}
			bg[(y-b.y0)*bw+x-b.x0] = c
		}
	}
	return mask, bg
}

// nearestRow 回第 y 列、[lo, hi] 內不在 mask 的最近像素色號，等距取左；找不到時回 def。
func nearestRow(pix []uint8, mask []bool, w, y, x, lo, hi int, def uint8) uint8 {
	for d := 1; d <= hi-lo; d++ {
		if l := x - d; l >= lo && !mask[y*w+l] {
			return pix[y*w+l]
		}
		if r := x + d; r <= hi && !mask[y*w+r] {
			return pix[y*w+r]
		}
	}
	return def
}

// ruleQ 是 §3.3 規則 Q：底板 plate（233×104）第 n 行的灰字去除。m、ref 是同位置按鈕 n 依規則 P 的遮罩與無字底。
// 回 plateBand(n) 大小（逐列）的無字底與 Q 的點數。
//  1. 參照：ref 平移到底板座標（x + 11、y + 10 + 17n），定義域 x 13..219。
//  2. Q0：列 13+17n..20+17n、x 13..219、底板色號與參照不同且不屬 B 的點。
//  3. Q = Q0 ∪（M 平移後落在同一段列的點；M 的陰影列平移後是 21、22，不換）。
//  4. 回填：Q 內每點取參照的色號。
func ruleQ(plate []uint8, n int, m []bool, ref []uint8) ([]uint8, int) {
	pb := plateBand(n)
	bw := pb.x1 - pb.x0 + 1
	out := make([]uint8, bw*(pb.y1-pb.y0+1))
	for y := pb.y0; y <= pb.y1; y++ {
		copy(out[(y-pb.y0)*bw:(y-pb.y0+1)*bw], plate[y*plateW+pb.x0:y*plateW+pb.x1+1])
	}
	refBW := mmBand.x1 - mmBand.x0 + 1
	dy := 10 + 17*n
	refAt := func(px, py int) uint8 { // 底板座標 → 按鈕無字底
		return ref[(py-dy-mmBand.y0)*refBW+px-11-mmBand.x0]
	}
	q := 0
	for py := 13 + 17*n; py <= 20+17*n; py++ {
		for px := 13; px <= 219; px++ {
			c, r := plate[py*plateW+px], refAt(px, py)
			if (c != r && !bMainMenu[c]) || m[(py-dy)*mmW+px-11] {
				out[(py-pb.y0)*bw+px-pb.x0] = r
				q++
			}
		}
	}
	return out, q
}

// glyphPass 是一趟字樣：以 col 在字模位移 (dx, dy) 個輸出像素處畫一次（§3.5）。依表中順序畫，先畫的在下。
type glyphPass struct {
	col    uint8
	dx, dy int
}

// §3.5 的字樣常數（原版統計，執行期不重算；單元測試以 §3.5 的母體重算比對）。
var (
	mmShadow    = glyphPass{150, 1, 1}                    // 一般態陰影
	mmHighlight = glyphPass{187, -1, -1}                  // 一般態亮邊
	mmNormalC   = [mmLines]uint8{139, 140, 140, 140, 140} // 一般態字色 c
	mmPressedE  = [mmLines]uint8{139, 140, 140, 140, 140} // 按下態上緣 e，d (0, −1)
	mmPressedHB = [mmLines]uint8{183, 184, 185, 184, 186} // 按下態下緣亮邊 hb，d (0, +1)
	mmPressedC  = [mmLines]uint8{148, 148, 148, 148, 149} // 按下態字色 c
	plateHP     = [mmLines]uint8{183, 182, 183, 183, 182} // 底板灰字下緣亮邊 hp，d (0, +1)
	plateC      = uint8(176)                              // 底板灰字字色
	tmEdge      = glyphPass{193, 1, 1}                    // TOMENU 亮邊
	tmC         = uint8(18)                               // TOMENU 字色
)

// menuPasses 回 008 §3.5 表中該態的各趟（最後一趟是字色 c，d (0, 0)）。
func menuPasses(ev ArtEvent) []glyphPass {
	switch ev.File {
	case ArtMainMenuButtons:
		n := ev.K % mmLines
		if mmPressed(ev.K) {
			return []glyphPass{{mmPressedE[n], 0, -1}, {mmPressedHB[n], 0, 1}, {mmPressedC[n], 0, 0}}
		}
		return []glyphPass{mmShadow, mmHighlight, {mmNormalC[n], 0, 0}}
	case ArtMainMenuPlate:
		return []glyphPass{{plateHP[ev.Line], 0, 1}, {plateC, 0, 0}}
	case ArtTomenuButtons:
		return []glyphPass{tmEdge, {tmC, 0, 0}}
	}
	return nil
}

// artSpec 是一列圖面的元素資料：catalog key、元素尺寸、字帶、置中框與圖面 Key。
type artSpec struct {
	key          string // 圖形字 catalog 的 key
	w, h         int    // 元素尺寸（Entry 的寬高）
	band, center box
	rectKey      string
}

// artSpecFor 回事件的元素資料（004 §3.3、008 §3.2、§3.6）。不是有字的元素時回 false。
func artSpecFor(ev ArtEvent) (artSpec, bool) {
	switch ev.File {
	case ArtSaveLoad:
		el, ink, ok := elementFor(ev.K)
		if !ok {
			return artSpec{}, false
		}
		return artSpec{el.key, el.w, el.h, el.band, ink, artKey(ev)}, true
	case ArtMainMenuButtons:
		if ev.K < 0 || ev.K >= 2*mmLines {
			return artSpec{}, false
		}
		n := ev.K % mmLines
		return artSpec{mmKeys[n], mmW, mmH, mmBand, mmCenter(n), artKey(ev)}, true
	case ArtMainMenuPlate:
		if ev.K != 0 || ev.Line < 0 || ev.Line >= mmLines {
			return artSpec{}, false
		}
		n := ev.Line
		return artSpec{mmKeys[n], plateW, plateH, plateBand(n), plateCenter(n), artKey(ev)}, true
	case ArtTomenuButtons:
		if ev.K < 0 || ev.K >= len(tmSizes) {
			return artSpec{}, false
		}
		el := tmElements[ev.K%2]
		return artSpec{el.key, tmSizes[ev.K][0], tmSizes[ev.K][1], el.band, el.center, artKey(ev)}, true
	}
	return artSpec{}, false
}

// mmPair 從按鈕檔解碼第 n 行的一般態與按下態（211×15）。
func mmPair(buttons []byte, n int) (normal, pressed []uint8, err error) {
	a, err := decodeEntry(buttons, n)
	if err != nil {
		return nil, nil, err
	}
	b, err := decodeEntry(buttons, n+mmLines)
	if err != nil {
		return nil, nil, err
	}
	if a.W != mmW || a.H != mmH || b.W != mmW || b.H != mmH {
		return nil, nil, fmt.Errorf("按鈕 %d、%d 尺寸 %d×%d、%d×%d，要 %d×%d", n, n+mmLines, a.W, a.H, b.W, b.H, mmW, mmH)
	}
	return a.Pix, b.Pix, nil
}

// artBackground 回事件在字帶內的無字底（字帶寬×高，逐列）與各趟字樣（004 §3.4、§3.5；008 §3.3、§3.5）。
func artBackground(ev ArtEvent) ([]uint8, []glyphPass, error) {
	switch ev.File {
	case ArtSaveLoad:
		c, h, hasH := dialogColors(ev)
		passes := []glyphPass{{c, 0, 0}}
		if hasH {
			// 高光在筆畫左側：先以 h 在字模左移 1 個輸出像素處畫一次，再以 c 畫字模（004 §3.5）。
			passes = []glyphPass{{h, -1, 0}, {c, 0, 0}}
		}
		return dialogBackground(ev), passes, nil
	case ArtMainMenuButtons:
		normal, pressed, err := mmPair(ev.Buttons, ev.K%mmLines)
		if err != nil {
			return nil, nil, err
		}
		_, bg := ruleP(normal, pressed)
		return bg, menuPasses(ev), nil
	case ArtMainMenuPlate:
		normal, pressed, err := mmPair(ev.Buttons, ev.Line)
		if err != nil {
			return nil, nil, err
		}
		m, ref := ruleP(normal, pressed)
		bg, _ := ruleQ(ev.Entry, ev.Line, m, ref)
		return bg, menuPasses(ev), nil
	case ArtTomenuButtons:
		sp, _ := artSpecFor(ev)
		bg, _ := tomenuBackground(ev.Entry, sp)
		return bg, menuPasses(ev), nil
	}
	return nil, nil, fmt.Errorf("檔別 %d 不覆繪", ev.File)
}

// tomenuMask 回 TOMENU 條目的墨跡遮罩：置中框內、色號不屬 B_T 的點（§3.3，照 004 的色號規則）。
func tomenuMask(entry []uint8, sp artSpec) []bool {
	return inkMask(entry, sp.w, sp.h, sp.center, &bTomenu)
}

// tomenuBackground 回 TOMENU 條目的無字底與退回清單：回填來源照 004 §7.1（同列、字帶內、非墨跡、色號屬 B_T 的最近位置）。
func tomenuBackground(entry []uint8, sp artSpec) ([]uint8, []int) {
	return inpaint(entry, entry, tomenuMask(entry, sp), sp.w, sp.band, &bTomenu)
}
