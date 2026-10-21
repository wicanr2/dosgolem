package strikecommander

import "github.com/wicanr2/dosgolem/xlate"

// 反鋸齒文字的共用規則（strike_commander 規格 011 §3.3、§3.6.7）：半形化 N() 與像素換行。

func isASCIIAlnum(r rune) bool {
	return r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z'
}

func isASCIIDigit(r rune) bool { return r >= '0' && r <= '9' }

// NarrowAlnum 是 011 §3.3 的 N()：
//  1. 全形英數 U+FF10–FF19、U+FF21–FF3A、U+FF41–FF5A 換成對應 ASCII（減 FEE0h）；
//  2. 第 1 步之後，－（U+FF0D）的前一字或後一字是 ASCII 英數時換成 -；
//  3. 其他字不變。不改字數；回新的切片，不改 rs。
func NarrowAlnum(rs []rune) []rune {
	a := make([]rune, len(rs))
	for i, r := range rs {
		if r >= 0xFF10 && r <= 0xFF19 || r >= 0xFF21 && r <= 0xFF3A || r >= 0xFF41 && r <= 0xFF5A {
			r -= 0xFEE0
		}
		a[i] = r
	}
	out := append([]rune(nil), a...)
	for i, r := range a {
		if r != 0xFF0D {
			continue
		}
		if i > 0 && isASCIIAlnum(a[i-1]) || i+1 < len(a) && isASCIIAlnum(a[i+1]) {
			out[i] = '-'
		}
	}
	return out
}

// 換行禁則集合（011 §3.6.7 第 5 步）：H 不得在行首，T 不得在行尾。
var (
	pixWrapHead = map[rune]bool{'，': true, '。': true, '、': true, '！': true, '？': true, '；': true, '：': true,
		'」': true, '』': true, '）': true, '…': true, ' ': true}
	pixWrapTail = map[rune]bool{'「': true, '『': true, '（': true}
)

// wrapUnits 是 011 §3.6.7 換行第 1 步：ASCII 英數的最長連續段為一個單位（段內夾在兩個英數之間的 - . ' 併入）；
// ASCII - 後面緊接數字、且前一字不是 ASCII 英數時，- 與後面的數字段同一單位；其他每字一個單位。
func wrapUnits(s []rune) [][]rune {
	var us [][]rune
	for i := 0; i < len(s); {
		lead := isASCIIAlnum(s[i]) || s[i] == '-' && i+1 < len(s) && isASCIIDigit(s[i+1]) && !(i > 0 && isASCIIAlnum(s[i-1]))
		if !lead {
			us = append(us, s[i:i+1])
			i++
			continue
		}
		j := i + 1
		for j < len(s) {
			if isASCIIAlnum(s[j]) {
				j++
				continue
			}
			if (s[j] == '-' || s[j] == '.' || s[j] == '\'') && isASCIIAlnum(s[j-1]) && j+1 < len(s) && isASCIIAlnum(s[j+1]) {
				j++
				continue
			}
			break
		}
		us = append(us, s[i:j])
		i = j
	}
	return us
}

func isSpaceUnit(u []rune) bool { return len(u) == 1 && u[0] == ' ' }

// wrapPixels 是 011 §3.6.7 的像素換行；lint 與顯示共用。maxW 是可用寬（輸出像素）。
// 缺字的前進寬記 0（呼叫端另以 Advance 判缺字）。整串為空或只有空白時回 0 行。
func wrapPixels(s []rune, f *xlate.AlphaFont, maxW int) (lines [][]rune, forced bool) {
	width := func(u []rune) int { w, _ := f.Advance(u); return w }
	// 第 1、2 步：切單位；單一單位比 maxW 寬時拆成單字。
	var us [][]rune
	for _, u := range wrapUnits(s) {
		if len(u) > 1 && width(u) > maxW {
			for k := range u {
				us = append(us, u[k:k+1])
			}
			forced = true
			continue
		}
		us = append(us, u)
	}
	n := len(us)
	ws := make([]int, n)
	for k, u := range us {
		ws[k] = width(u)
	}
	emit := func(i, j int) {
		for j > i && isSpaceUnit(us[j-1]) {
			j--
		}
		var l []rune
		for k := i; k < j; k++ {
			l = append(l, us[k]...)
		}
		lines = append(lines, l)
	}
	i := 0
	for {
		// 第 3 步：丟掉行首的 U+0020；丟完到底就結束。
		for i < n && isSpaceUnit(us[i]) {
			i++
		}
		if i >= n {
			return lines, forced
		}
		// 最大的 j 使單位 i..j−1 的寬度和（行尾 U+0020 不計）≤ maxW。
		j, sum, pend := i, 0, 0
		for j < n {
			if isSpaceUnit(us[j]) {
				pend += ws[j]
				j++
				continue
			}
			if sum+pend+ws[j] > maxW {
				break
			}
			sum += pend + ws[j]
			pend = 0
			j++
		}
		if j == n {
			emit(i, n)
			return lines, forced
		}
		// 第 4 步。
		if j == i {
			j = i + 1
			forced = true
		}
		// 第 5、6 步：禁則。
		bad := func(j int) bool {
			if j >= n {
				return false // 第 4 步取到最後一個單位之後，沒有下一個單位
			}
			return pixWrapHead[us[j][0]] || pixWrapTail[us[j-1][len(us[j-1])-1]]
		}
		j0 := j
		for j > i+1 && bad(j) {
			j--
		}
		if j == i+1 && bad(j) {
			j = j0
			forced = true
		}
		// 第 7 步。
		emit(i, j)
		i = j
	}
}

// textInkRows 回字串在字級圖集 f 的墨跡列 [it, ib)（相對 em 上緣；011 §1 用語）。withStroke 為真且字型有外框層時
// 取填色層與外框層的聯集。沒有墨跡時 ok 為假；有缺字時 missing 非空。
func textInkRows(f *xlate.AlphaFont, rs []rune, withStroke bool) (it, ib int, ok bool, missing []rune) {
	const big = 1 << 20
	clip := xlate.AlphaRect{X0: -big, Y0: -big, X1: big, Y1: big}
	baseline := f.Ascent // em 上緣在第 0 列
	_, box, missing, _ := xlate.StringMask(f, rs, 0, baseline, xlate.AlphaFill, clip)
	if len(missing) > 0 {
		return 0, 0, false, missing
	}
	it, ib, ok = box.Y0, box.Y1, !box.Empty()
	if withStroke && f.HasStroke {
		if _, sb, _, _ := xlate.StringMask(f, rs, 0, baseline, xlate.AlphaStroke, clip); !sb.Empty() {
			if !ok {
				it, ib, ok = sb.Y0, sb.Y1, true
			} else {
				it, ib = min(it, sb.Y0), max(ib, sb.Y1)
			}
		}
	}
	return it, ib, ok, nil
}
