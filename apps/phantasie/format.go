package phantasie

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// 格式引擎（docs/spec/003 §7）。
//
// 兩套語意共用同一個解析器：
//   - FormatEnglish 重現原版 sub_5032 的輸出（位元組串），用於組句驗證與測試；
//   - FormatTarget 是目標語言的版面語意，寬度以半格（h）計，一格 = 2 h。
//
// 支援的規格只有：旗標 -、寬度（首位 1 至 9）、精度 .P（至少一位數字）、長度 l（只配 d、u）、
// 類型 s d u c，以及獨立的 %%。其餘（旗標 0 + # *、長度 h、%ls %lc、%x %X %o %e %f %g、%n$、
// %5% %-%、尾端 %）一律回 error，與 tools/catalog_lib.py 的 CONV 收斂到同一個語法。
// 三個共用向量檔（tests/vectors）由 format_test.go 逐列驗證。

// feMaxField 是寬度與精度的上限（含）。原版畫面一列 40 格，超過它的規格不可能是有意義的格式；
// 設上限是為了不讓壞格式字串（例如 %99999999d）撐大輸出。Python 端沒有此上限（見回報）。
const feMaxField = 999

// Conv 是一個轉換規格。
type Conv struct {
	Left  bool // 旗標 -
	Width int  // 0 表示沒寫
	Prec  int  // -1 表示沒寫
	Long  bool // 長度 l（只配 d、u）
	Type  byte // 's' 'd' 'u' 'c'
}

// FmtSeg 是格式字串的一個片段：Conv 為 nil 時是字面文字（%% 已讀成 %），否則是轉換。
type FmtSeg struct {
	Lit  string
	Conv *Conv
}

// ParseFormat 把格式字串切成片段。相鄰的字面文字（含 %% 讀成的 %）併成一個片段，
// 所以 "100%%%d" 得到字面 "100%" 與一個 %d；空字串得到零個片段。
// 字面文字原樣保留（可含 UTF-8 譯文）。不支援的規格回 error。
func ParseFormat(format string) ([]FmtSeg, error) {
	var segs []FmtSeg
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			segs = append(segs, FmtSeg{Lit: lit.String()})
			lit.Reset()
		}
	}
	i := 0
	for i < len(format) {
		c := format[i]
		if c != '%' {
			// 逐位元組複製；遇到 % 之前都不是規格，多位元組的 UTF-8 字元不會被拆開（% 是 ASCII）。
			lit.WriteByte(c)
			i++
			continue
		}
		start := i
		i++
		if i >= len(format) {
			return nil, fmt.Errorf("格式字串尾端的 %%：%q", format)
		}
		if format[i] == '%' {
			lit.WriteByte('%')
			i++
			continue
		}
		conv := Conv{Prec: -1}
		if format[i] == '-' {
			conv.Left = true
			i++
		}
		// 寬度：首位必須是 1 至 9（0 是旗標 0，不支援）。
		if i < len(format) && format[i] >= '1' && format[i] <= '9' {
			n, next, err := feDigits(format, i)
			if err != nil {
				return nil, fmt.Errorf("規格 %q：寬度%w", format[start:min(next, len(format))], err)
			}
			conv.Width = n
			i = next
		}
		// 精度：. 之後至少一位數字。
		if i < len(format) && format[i] == '.' {
			i++
			if i >= len(format) || format[i] < '0' || format[i] > '9' {
				return nil, fmt.Errorf("規格 %q：精度缺數字", format[start:min(i, len(format))])
			}
			n, next, err := feDigits(format, i)
			if err != nil {
				return nil, fmt.Errorf("規格 %q：精度%w", format[start:min(next, len(format))], err)
			}
			conv.Prec = n
			i = next
		}
		if i < len(format) && format[i] == 'l' {
			conv.Long = true
			i++
		}
		if i >= len(format) {
			return nil, fmt.Errorf("規格不完整：%q", format[start:])
		}
		conv.Type = format[i]
		i++
		switch conv.Type {
		case 's', 'c':
			if conv.Long {
				return nil, fmt.Errorf("規格 %q：長度 l 只配 d、u", format[start:i])
			}
		case 'd', 'u':
		default:
			// 含 %5%、%-%、%l%（% 只允許獨立的 %%）與 x X o e f g n 等未支援類型。
			return nil, fmt.Errorf("不支援的規格 %q", format[start:i])
		}
		flush()
		c2 := conv
		segs = append(segs, FmtSeg{Conv: &c2})
	}
	flush()
	return segs, nil
}

// feDigits 從 s[i] 起讀十進位數字，回數值與下一個位置；超過 feMaxField 回 error。
func feDigits(s string, i int) (n, next int, err error) {
	next = i
	for next < len(s) && s[next] >= '0' && s[next] <= '9' {
		n = n*10 + int(s[next]-'0')
		if n > feMaxField {
			// 為了不溢位，超限即停（呼叫端只拿 next 來截取訊息）。
			return 0, next + 1, fmt.Errorf("超過上限 %d", feMaxField)
		}
		next++
	}
	return n, next, nil
}

// HasConversions 回格式字串是否含任何轉換（%% 不算）。
func HasConversions(segs []FmtSeg) bool {
	for _, s := range segs {
		if s.Conv != nil {
			return true
		}
	}
	return false
}

// CountS 回 %s 轉換的個數。
func CountS(segs []FmtSeg) int {
	n := 0
	for _, s := range segs {
		if s.Conv != nil && s.Conv.Type == 's' {
			n++
		}
	}
	return n
}

// feArgs 依序消耗格式字串之後的 16 位元字組與 %s 字串（003 §5.2「引數讀取」）：
// %s %d %u %c 各 1 個字組（%s 的字組是指標，內容不使用），%ld %lu 各 2 個字組（低字組在前）。
type feArgs struct {
	words []uint16
	wi    int
}

func (a *feArgs) word(conv *Conv) (uint16, error) {
	if a.wi >= len(a.words) {
		return 0, fmt.Errorf("字組不足：第 %d 個字組不存在（共 %d 個），轉換 %%%c", a.wi+1, len(a.words), conv.Type)
	}
	w := a.words[a.wi]
	a.wi++
	return w, nil
}

// text 取出 %d %u %c 的原始文字（尚未套用精度與寬度）。%c 是引數的低位元組，為 0 時保留 NUL。
func (a *feArgs) text(conv *Conv) (string, error) {
	switch conv.Type {
	case 'c':
		w, err := a.word(conv)
		if err != nil {
			return "", err
		}
		return string([]byte{byte(w)}), nil
	case 's':
		_, err := a.word(conv) // 指標字組，內容不使用
		return "", err
	}
	lo, err := a.word(conv)
	if err != nil {
		return "", err
	}
	if !conv.Long {
		if conv.Type == 'd' {
			return strconv.Itoa(int(int16(lo))), nil
		}
		return strconv.FormatUint(uint64(lo), 10), nil
	}
	hi, err := a.word(conv)
	if err != nil {
		return "", err
	}
	v := uint32(hi)<<16 | uint32(lo)
	if conv.Type == 'd' {
		return strconv.FormatInt(int64(int32(v)), 10), nil
	}
	return strconv.FormatUint(uint64(v), 10), nil
}

// fePad 把寬度 hw 的文字補到 want h：不足補 ASCII 空白，left 補右側，否則補左側。
func fePad(text string, hw, want int, left bool) string {
	if hw >= want {
		return text
	}
	pad := strings.Repeat(" ", want-hw)
	if left {
		return text + pad
	}
	return pad + text
}

// FormatEnglish 重現原版 sub_5032 的英文語意（003 §7.1）。words 是格式字串之後的 16 位元字組
// （%s 取 1 個字組，其內容不使用，字串內容由 strs 依 %s 出現順序給）；%d 16 位元有號、%u 無號、
// %ld %lu 兩個字組（低在前）、%c 取低位元組。回傳位元組串（以 Go string 保存，%c 為 0 時保留 NUL；
// 呼叫端截到第一個 NUL 的規則不在此函式）。
//
// 寬度與精度都以位元組計：%s 與數字的精度 .P 最多輸出 P 個位元組（數字含負號、不補零），
// %c 忽略精度；寬度不足補空白，旗標 - 補右側，否則補左側。字組或 strs 不足回 error，多餘的引數忽略。
func FormatEnglish(format string, words []uint16, strs []string) (string, error) {
	segs, err := ParseFormat(format)
	if err != nil {
		return "", err
	}
	args := feArgs{words: words}
	var out strings.Builder
	si := 0
	for _, seg := range segs {
		if seg.Conv == nil {
			out.WriteString(seg.Lit)
			continue
		}
		c := seg.Conv
		var text string
		if c.Type == 's' {
			if _, err := args.text(c); err != nil {
				return "", err
			}
			if si >= len(strs) {
				return "", fmt.Errorf("字串引數不足：第 %d 個 %%s 沒有對應的字串（共 %d 個）", si+1, len(strs))
			}
			text = strs[si]
			si++
		} else if text, err = args.text(c); err != nil {
			return "", err
		}
		if c.Prec >= 0 && c.Type != 'c' && len(text) > c.Prec {
			text = text[:c.Prec]
		}
		out.WriteString(fePad(text, len(text), c.Width, c.Left))
	}
	return out.String(), nil
}

// TargetStr 是目標語言格式化的 %s 引數：原樣保留的 ASCII（Translated=false）或譯文（Translated=true）。
type TargetStr struct {
	Text       string
	Translated bool
}

// HalfWidth 回字串的半格寬度（全形字 2，其他 1）。wide 為 nil 時每個字元都算 1。
// 不特別處理置中標記：Result.Zh 本來就不含它（003 §5.2）。
func HalfWidth(s string, wide WideFunc) int {
	if wide == nil {
		return utf8.RuneCountInString(s)
	}
	n := 0
	for _, r := range s {
		if wide(r) {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// feTruncateH 回 s 中寬度不超過 maxH 的最長整字前綴（遇到第一個放不下的字就停）。
func feTruncateH(s string, maxH int, wide WideFunc) string {
	acc := 0
	for i, r := range s {
		w := 1
		if wide(r) {
			w = 2
		}
		if acc+w > maxH {
			return s[:i]
		}
		acc += w
	}
	return s
}

// feTruncateRunes 回 s 的前 n 個字元。
func feTruncateRunes(s string, n int) string {
	for i := range s {
		if n == 0 {
			return s[:i]
		}
		n--
	}
	return s
}

// FormatTarget 重現目標語言語意（003 §7.2）：tpl 是已含轉換規格的譯文模板，輸出以半格（h）計寬。
//   - 欄位寬度 W 一律是 2W h（%s、數字、%c 都一樣），旗標 - 補右側，否則補左側；
//   - %s 精度 .P：譯文保留不超過 2P h 的最長整字前綴，原樣保留的 ASCII 是前 P 個字元（與原版相同）；
//   - 數字精度是最多 P 個字元（含負號、不補零），不是欄位的 h 數；
//   - %c 字元 1 h 且佔一格：無寬度時字元靠左、右側補 1 個空白（2 h），有寬度時輸出 2W h，%c 忽略精度。
//
// %c 的低位元組為 0 時輸出一個 NUL（1 h），與 FormatEnglish 一致，截斷由呼叫端負責；
// 低位元組 >= 0x80 無法成為譯文字串中的 ASCII 字元，回 error（呼叫端視為 badarg）。
// 字組不足、strs 不足、wide 為 nil 都回 error。
func FormatTarget(tpl string, words []uint16, strs []TargetStr, wide WideFunc) (string, error) {
	if wide == nil {
		return "", errors.New("FormatTarget：wide 不可為 nil")
	}
	segs, err := ParseFormat(tpl)
	if err != nil {
		return "", err
	}
	args := feArgs{words: words}
	var out strings.Builder
	si := 0
	for _, seg := range segs {
		if seg.Conv == nil {
			out.WriteString(seg.Lit)
			continue
		}
		c := seg.Conv
		left := c.Left
		width := c.Width
		var text string
		switch c.Type {
		case 's':
			if _, err := args.text(c); err != nil {
				return "", err
			}
			if si >= len(strs) {
				return "", fmt.Errorf("字串引數不足：第 %d 個 %%s 沒有對應的字串（共 %d 個）", si+1, len(strs))
			}
			a := strs[si]
			si++
			text = a.Text
			if c.Prec >= 0 {
				if a.Translated {
					text = feTruncateH(text, 2*c.Prec, wide)
				} else {
					text = feTruncateRunes(text, c.Prec)
				}
			}
		case 'c':
			if text, err = args.text(c); err != nil {
				return "", err
			}
			if text[0] >= 0x80 {
				return "", fmt.Errorf("%%c 的位元組 0x%02X 不是 ASCII，無法排進譯文", text[0])
			}
			if width == 0 {
				// 原版一個字元佔一格，其後欄位的位置依賴它：字元靠左，右側補 1 h。
				width = 1
				left = true
			}
		default:
			if text, err = args.text(c); err != nil {
				return "", err
			}
			if c.Prec >= 0 && len(text) > c.Prec {
				text = text[:c.Prec]
			}
		}
		out.WriteString(fePad(text, HalfWidth(text, wide), 2*width, left))
	}
	return out.String(), nil
}
