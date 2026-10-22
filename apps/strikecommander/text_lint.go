package strikecommander

import (
	"fmt"
	"strings"
)

// 反鋸齒圖集路徑的 lint（strike_commander 規格 011 §3.7）。結構檢查由各 catalog 的核心函式做，
// 字元、寬度與垂直檢查依正式圖集的像素寬度與墨跡列。

// textScales 是覆繪的倍率（011 §3.1：1 倍不覆繪）。
var textScales = [...]int{2, 3}

// 各路徑的字級（011 §3.1），以倍率索引；0 表示該倍率不畫。
var (
	pxLabel  = map[int]int{2: 22, 3: 32}
	pxBoard  = map[int]int{2: 16, 3: 22}
	pxArt    = map[int]int{2: 16, 3: 22}
	pxTomenu = map[int]int{2: 22, 3: 32}
	pxHUD    = map[int]int{3: 15}
	pxMFD    = map[int]int{2: 16, 3: 22}
	pxPlaque = map[int]int{2: 16, 3: 22}
	pxSub    = map[int]int{2: 22, 3: 33}
)

// 字帶列數（011 §3.1）：標籤 [Y+1, Y+15)、計分板 [Y+1, Y+9)、HUD [Y+1, Y+6)、MFD [Y−1, Y+7)、橫幅 [y−2, y+7)。
const (
	bandRowsLabel  = 14
	bandRowsBoard  = 8
	bandRowsHUD    = 5
	bandRowsMFD    = 8
	bandRowsPlaque = 9
)

// boardWidth 是計分板英文游標寬（原版像素，011 §3.7，強推論）。
var boardWidth = map[string]int{"GROUND": 73, "AIR": 31}

// hudLintRange 是 HUD 狀態字的可用範圍 [L, R]（原版像素，含端點；011 §3.6.4）。NONE 以 Xs[k] = 134 計（§3.7，強推論）。
var hudLintRange = map[string][2]int{"GEAR": {183, 213}, "FLAPS": {183, 212}, "BRAKE": {183, 211},
	"DGFT": {118, 149}, "STRF": {118, 149}, "SRM": {118, 149}, "NONE": {134, 149}}

// mfdRequestStatus 是 MFD k > 0 的 key；它的可用寬以 Xs[2] = 230 計（011 §3.7，強推論）。
const (
	mfdRequestStatus = "request status"
	mfdK0Width       = 87
	mfdRequestWidth  = 306 - 230
)

// plaqueInkWidth 是橫幅英文墨跡寬（原版像素，011 §3.7；RE 021 §2.3，後三者為推算），順序同 flightAllowed[KindPlaque]。
var plaqueInkWidth = map[string]int{"time burst": 69, "enemies near": 87, "chase plane": 83, "external view": 91,
	"victim view on": 95, "weapon camera on": 121}

// textCharErrors 是 011 §3.7 的共通規則：text（已拿掉代換符號、範本佔位符）不得含 ASCII 20h–7Eh；
// N(text) 每字都在圖集字集內。where 是錯誤訊息前綴。
func textCharErrors(bad badFunc, where string, text []rune, fonts *TextFonts) {
	for _, c := range text {
		if c >= 0x20 && c <= 0x7E {
			bad("%s 含半形字 %q（U+%04X）", where, c, c)
		}
	}
	for _, c := range NarrowAlnum(text) {
		if !fonts.Chars[c] {
			bad("%s 缺字 %q（U+%04X）", where, c, c)
		}
	}
}

// textAdvance 回 N(text) 在字級 px 的前進寬；圖集缺該字級或缺字時 ok 為假（缺字另由 textCharErrors 報）。
func textAdvance(fonts *TextFonts, px int, text []rune) (int, bool) {
	f := fonts.Font(px)
	if f == nil {
		return 0, false
	}
	w, miss := f.Advance(NarrowAlnum(text))
	return w, len(miss) == 0
}

// checkBandRows 檢查字帶置中的路徑（011 §3.5 垂直、§3.7 末列）：墨跡列（有外框取兩層聯集）依 ey 放，
// 落在裁切框列 [−s, H + s)（相對字帶上緣）內；px > H 時記錯（執行期會 Skip）。
func checkBandRows(bad badFunc, where string, fonts *TextFonts, px, s, rows int, outline bool, text []rune) {
	f := fonts.Font(px)
	if f == nil {
		bad("%s 圖集缺字級 %d", where, px)
		return
	}
	h := rows * s
	if px > h {
		bad("%s %d 倍字級 %d 大於字帶高 %d", where, s, px, h)
		return
	}
	it, ib, ok, miss := textInkRows(f, NarrowAlnum(text), outline)
	if !ok || len(miss) > 0 {
		return
	}
	ey := (h - px) / 2
	if ey+it < -s || ey+ib > h+s {
		bad("%s %d 倍墨跡列 %d..%d 超出裁切框列 %d..%d", where, s, ey+it, ey+ib-1, -s, h+s-1)
	}
}

// checkWidth 記 W + extra > limit 的錯誤。
func checkWidth(bad badFunc, where string, fonts *TextFonts, px, s, extra, limit int, text []rune) {
	if w, ok := textAdvance(fonts, px, text); ok && w+extra > limit {
		if extra > 0 {
			bad("%s %d 倍寬 %d + %d 超過 %d", where, s, w, extra, limit)
		} else {
			bad("%s %d 倍寬 %d 超過 %d", where, s, w, limit)
		}
	}
}

// LintLabels 是熱點標籤 catalog 的 lint（011 §3.7）。結構、孤兒 key、漏譯與防呆由 lintLabelCore 檢查。
//   - 標籤：W + 2 ≤ 320s（2 倍 22、3 倍 32）；組合標籤以每個長名稱與短代號代入、%d 以 88888 計；
//   - GROUND、AIR：W ≤ (x1 − x0) × s（2 倍 16、3 倍 22），x1 − x0 取 73、31；
//   - 墨跡列（標籤含外框層）依 ey 放在裁切框列內。
func LintLabels(b []byte, fonts *TextFonts, labl map[string]bool) LintReport {
	rep, cat := lintLabelCore(b, labl, func(bad badFunc, r catalogRow, text string) {
		where := fmt.Sprintf("第 %d 列 key %q", r.line, r.key)
		rs := []rune(text)
		textCharErrors(bad, where, rs, fonts)
		if isTemplateKey(r.key) {
			return // 範本本身不單獨上畫面，代入後再量（下面）。
		}
		lintLabelGeometry(bad, where, r.key, rs, fonts)
	})
	bad := func(format string, a ...any) { rep.Errors = append(rep.Errors, fmt.Sprintf(format, a...)) }
	for _, c := range []struct {
		tpl   string
		names []string
	}{{TemplateCount, WeaponLongNames[:]}, {TemplateRemove, WeaponShortNames[:]}} {
		for _, n := range c.names {
			if t, ok := composeLabel(cat, c.tpl, n, "88888"); ok {
				lintLabelGeometry(bad, fmt.Sprintf("組合標籤 %q 代入 %q", c.tpl, n), "", t, fonts)
			}
		}
	}
	return rep
}

func lintLabelGeometry(bad badFunc, where, key string, rs []rune, fonts *TextFonts) {
	for _, s := range textScales {
		if bw, ok := boardWidth[key]; ok {
			checkWidth(bad, where, fonts, pxBoard[s], s, 0, bw*s, rs)
			checkBandRows(bad, where, fonts, pxBoard[s], s, bandRowsBoard, false, rs)
			continue
		}
		checkWidth(bad, where, fonts, pxLabel[s], s, 2, 320*s, rs)
		checkBandRows(bad, where, fonts, pxLabel[s], s, bandRowsLabel, true, rs)
	}
}

// artLintState 是一個圖面元素的一種狀態（一般或按下）的字樣趟：dys 是各趟 dy（含字色趟的 0），dx 是最大 |dx|。
type artLintState struct {
	name string
	dys  []int
	dx   int
}

// artLintElement 是 lint 用的一個圖面元素：字帶（原版像素，含端點）、字級與各狀態的趟表。
type artLintElement struct {
	name   string
	band   box
	px     map[int]int
	states []artLintState
}

// artLintElements 回使用 catalog key k 的全部圖面元素（011 §3.6.3、§3.7；LOAD GAME 兩處都檢查）。
// 存讀檔的高光趟是否存在要看執行期的色號，lint 一律以有高光（dx −1）計。
func artLintElements(k string) []artLintElement {
	var out []artLintElement
	for _, n := range []int{0, 1, 2, 4, 6, 8, 15, 16, 17, 19} {
		el := artElements[n]
		if el.key != k {
			continue
		}
		st := []artLintState{{"一般", []int{0}, 1}}
		if pressedState(n + 1) {
			st = append(st, artLintState{"按下", []int{0}, 1})
		}
		out = append(out, artLintElement{fmt.Sprintf("存讀檔 k%d", n), el.band, pxArt, st})
	}
	for n, key := range mmKeys {
		if key != k {
			continue
		}
		out = append(out, artLintElement{fmt.Sprintf("主選單 n%d", n), mmBand, pxArt, []artLintState{
			{"一般", []int{mmShadow.dy, mmHighlight.dy, 0}, 1}, {"按下", []int{-1, 1, 0}, 0}}})
		out = append(out, artLintElement{fmt.Sprintf("底板灰字 n%d", n), plateBand(n), pxArt, []artLintState{
			{"一般", []int{1, 0}, 0}}})
	}
	for n, el := range tmElements {
		if el.key != k {
			continue
		}
		st := artLintState{"一般", []int{tmEdge.dy, 0}, tmEdge.dx}
		out = append(out, artLintElement{fmt.Sprintf("TOMENU k%d", n), el.band, pxTomenu, []artLintState{st, {"按下", st.dys, st.dx}}})
	}
	return out
}

// artVerticalRange 是 011 §3.6.3 第 3 步：lo = −min(it + dy)、hi = bh·s − max(ib + dy)（dy 取遍 dys）。
func artVerticalRange(it, ib, bhOut int, dys []int) (lo, hi int) {
	mn, mx := it+dys[0], ib+dys[0]
	for _, dy := range dys[1:] {
		mn, mx = min(mn, it+dy), max(mx, ib+dy)
	}
	return -mn, bhOut - mx
}

// LintArtText 是圖形字 catalog 的 lint（011 §3.7）。結構由 lintArtCore 檢查；每個元素、每個狀態、每個倍率：
// W + 2m ≤ 字帶寬 × s；§3.6.3 的 lo ≤ hi（墨跡含各趟 dy 能放進字帶）。
func LintArtText(b []byte, fonts *TextFonts) LintReport {
	return lintArtCore(b, func(bad badFunc, r catalogRow, k string) {
		where := fmt.Sprintf("第 %d 列 key %q", r.line, k)
		rs := []rune(r.tr)
		textCharErrors(bad, where, rs, fonts)
		nrs := NarrowAlnum(rs)
		for _, el := range artLintElements(k) {
			bw, bh := el.band.x1-el.band.x0+1, el.band.y1-el.band.y0+1
			for _, s := range textScales {
				f := fonts.Font(el.px[s])
				if f == nil {
					bad("%s %s 圖集缺字級 %d", where, el.name, el.px[s])
					continue
				}
				w, miss := f.Advance(nrs)
				if len(miss) > 0 {
					continue
				}
				it, ib, ok, _ := textInkRows(f, nrs, false)
				for _, st := range el.states {
					if w+2*st.dx > bw*s {
						bad("%s %s %s %d 倍寬 %d + 2×%d 超過字帶寬 %d", where, el.name, st.name, s, w, st.dx, bw*s)
					}
					if !ok {
						continue
					}
					if lo, hi := artVerticalRange(it, ib, bh*s, st.dys); lo > hi {
						bad("%s %s %s %d 倍墨跡列 %d..%d（趟 dy %v）放不進字帶高 %d", where, el.name, st.name, s, it, ib-1, st.dys, bh*s)
					}
				}
			}
		}
	})
}

// LintFlightText 是飛行 catalog 的 lint（011 §3.7）。結構與 glossary 由 lintFlightCore 檢查；字元照共通規則；寬度：
//   - hud（只 3 倍、15 像素）：W + 2 ≤ (R − L + 1) × 3；
//   - mfd：k = 0 的 key W ≤ 87s，request status W ≤ (306 − 230)s；
//   - plaque：W ≤ 英文墨跡寬 × s；
//   - 墨跡列（hud 含外框層）依 ey 放在裁切框列內。
func LintFlightText(kind Kind, b []byte, fonts *TextFonts, glossary []GlossaryRow) []error {
	return lintFlightCore(kind, b, glossary, func(bad badFunc, r catalogRow) {
		where := fmt.Sprintf("第 %d 列 key %q", r.line, r.key)
		rs := []rune(r.tr)
		textCharErrors(bad, where, rs, fonts)
		for _, s := range textScales {
			switch kind {
			case KindHUD:
				px := pxHUD[s]
				if px == 0 {
					continue // 2 倍 HUD 照原版英文（AGENTS 2026-10-20）
				}
				if lr, ok := hudLintRange[r.key]; ok {
					checkWidth(bad, where, fonts, px, s, 2, (lr[1]-lr[0]+1)*s, rs)
				}
				checkBandRows(bad, where, fonts, px, s, bandRowsHUD, true, rs)
			case KindMFD:
				lim := mfdK0Width
				if r.key == mfdRequestStatus {
					lim = mfdRequestWidth
				}
				checkWidth(bad, where, fonts, pxMFD[s], s, 0, lim*s, rs)
				checkBandRows(bad, where, fonts, pxMFD[s], s, bandRowsMFD, false, rs)
			case KindPlaque:
				if iw, ok := plaqueInkWidth[r.key]; ok {
					checkWidth(bad, where, fonts, pxPlaque[s], s, 0, iw*s, rs)
				}
				checkBandRows(bad, where, fonts, pxPlaque[s], s, bandRowsPlaque, false, rs)
			}
		}
	})
}

// subWorstRunes 是 lint 代入代換符號的最壞值（011 §3.7）：$S $N $C $W 各 19 個 ？，$P、$T 各為 -32768。
func subWorstRunes(sym rune) []rune {
	if sym == 'P' || sym == 'T' {
		return []rune("-32768")
	}
	return []rune(strings.Repeat("？", 19))
}

// withWorstSubs 把代換符號換成最壞值。
func withWorstSubs(s string) []rune {
	rs := []rune(s)
	var out []rune
	for i := 0; i < len(rs); i++ {
		if rs[i] == '$' && i+1 < len(rs) && strings.ContainsRune(subSymbols, rs[i+1]) {
			out = append(out, subWorstRunes(rs[i+1])...)
			i++
			continue
		}
		out = append(out, rs[i])
	}
	return out
}

// subTextMaxLines 是字幕 lint 的上限（011 §3.6.7、§3.7）：超過 3 行失敗、3 行列出。
const subTextMaxLines = 3

// subAvailWidth 是字幕可用寬：[10, 310) 原版像素（011 §3.6.7）。
func subAvailWidth(s int) int { return 300 * s }

// LintSpeechText 是字幕 catalog 的 lint（011 §3.7）。結構、符號、source、譯名由 lintSpeechCore 檢查；字元照共通規則
// （代換符號除外）；行數：2、3 倍各以 wrapPixels 量「說話人顯示名＋：＋譯文」（代換取最壞值，再做 N()），
// 超過 3 行失敗、3 行列出。
func LintSpeechText(indexB, catalogB, enB, glossaryB []byte, fonts *TextFonts) SpeechLintReport {
	chars := func(bad badFunc, r catalogRow) {
		textCharErrors(bad, fmt.Sprintf("譯文第 %d 列 key %q", r.line, r.key), withoutSymbols(r.tr), fonts)
	}
	lines := func(rep *SpeechLintReport, bad badFunc, r catalogRow, speaker string, hasSpeaker bool) {
		full := withWorstSubs(r.tr)
		if hasSpeaker {
			full = append(append(withWorstSubs(speaker), '：'), full...)
		}
		full = NarrowAlnum(full)
		most := 0
		for _, s := range textScales {
			f := fonts.Font(pxSub[s])
			if f == nil {
				bad("譯文第 %d 列 key %q 圖集缺字級 %d", r.line, r.key, pxSub[s])
				continue
			}
			ls, _ := wrapPixels(full, f, subAvailWidth(s))
			if len(ls) > subTextMaxLines {
				bad("譯文第 %d 列 key %q %d 倍換行後 %d 行，超過 %d 行", r.line, r.key, s, len(ls), subTextMaxLines)
			}
			most = max(most, len(ls))
		}
		if most == subTextMaxLines {
			rep.Over2 = append(rep.Over2, fmt.Sprintf("%s（%d 行）", r.key, most))
		}
	}
	return lintSpeechCore(indexB, catalogB, enB, glossaryB, chars, lines)
}

// CheckTextExtra 檢查圖集收入固定附加字 FontExtraRunes 經 N() 後的每一字（011 §3.7、§3.2.2；002 §2.5）。
// 附加字是組合標籤代入的全形數字與括號，畫出前經 N()，所以查 N() 後的字。
func CheckTextExtra(fonts *TextFonts) error {
	if fonts == nil {
		return fmt.Errorf("strikecommander: 沒有圖集")
	}
	var miss []rune
	for _, r := range NarrowAlnum([]rune(FontExtraRunes)) {
		ok := fonts.Chars[r]
		for _, px := range TextSizes {
			if f := fonts.Font(px); f == nil || f.Glyphs[r] == nil {
				ok = false
			}
		}
		if !ok {
			miss = append(miss, r)
		}
	}
	if len(miss) != 0 {
		return fmt.Errorf("strikecommander: 圖集缺固定附加字 %q", string(miss))
	}
	return nil
}
