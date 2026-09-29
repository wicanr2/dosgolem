package translit

import "strings"

// spellingPhones 是拼寫後備（規格 037 §3.2 第 3 層）：字母組合 → 近似 ARPAbet。
// 規則刻意簡單、決定性；品質最差，結果以 TierSpelling 標示。元音一律視為重讀，
// 所以附註規則 9（非重讀按拼寫）不會再作用——拼寫後備本來就是按拼寫譯。
func spellingPhones(w string) []ph {
	// 詞尾不發音的 e：前面是輔音且字裡還有別的元音（jane、steele）。
	// 「元音＋單一輔音＋不發音 e」的元音讀長音（jane → [eɪ]）。
	long := -1
	if n := len(w); n >= 3 && w[n-1] == 'e' && !strings.ContainsRune("aeiouy", rune(w[n-2])) &&
		strings.ContainsAny(w[:n-2], "aeiouy") {
		if n >= 4 && strings.ContainsRune("aeiou", rune(w[n-3])) && !strings.ContainsRune("aeiouy", rune(w[n-4])) {
			long = n - 3
		}
		w = w[:n-1]
	}
	var out []ph
	c := func(s ...string) {
		for _, x := range s {
			if n := len(out); n > 0 && out[n-1].sym == x && !out[n-1].vowel {
				continue // 重複輔音（ll、nn）只讀一次
			}
			out = append(out, ph{x, -1, false})
		}
	}
	v := func(s string) { out = append(out, ph{s, 1, true}) }
	isV := func(i int) bool { return i < len(w) && i >= 0 && strings.ContainsRune("aeiouy", rune(w[i])) }
	at := func(i int, s string) bool { return strings.HasPrefix(w[i:], s) }

	for i := 0; i < len(w); {
		ch := w[i]
		switch {
		// 元音二合字母
		case at(i, "ee"), at(i, "ea"):
			v("IY")
			i += 2
		case at(i, "oo"):
			v("UW")
			i += 2
		case at(i, "ou"):
			v("AW")
			i += 2
		case at(i, "ai"), at(i, "ay"), at(i, "ei"), at(i, "ey"):
			v("EY")
			i += 2
		case at(i, "oa"), at(i, "ow"):
			v("OW")
			i += 2
		case at(i, "au"), at(i, "aw"):
			v("AO")
			i += 2
		case at(i, "oi"), at(i, "oy"):
			v("OY")
			i += 2
		case at(i, "ue"), at(i, "ew"), at(i, "ui"):
			v("UW")
			i += 2
		case at(i, "ie") && i+2 == len(w):
			v("IY")
			i += 2
		// r 色元音
		case (ch == 'e' || ch == 'i' || ch == 'u') && i+1 < len(w) && w[i+1] == 'r' && !isV(i+2):
			v("ER")
			i += 2
		case ch == 'a' && i+1 < len(w) && w[i+1] == 'r' && !isV(i+2):
			v("AA")
			c("R")
			i += 2
		case ch == 'o' && i+1 < len(w) && w[i+1] == 'r' && !isV(i+2):
			v("AO")
			c("R")
			i += 2
		// 單一元音
		case i == long:
			v(map[byte]string{'a': "EY", 'e': "IY", 'i': "AY", 'o': "OW", 'u': "UW"}[ch])
			i++
		case ch == 'a':
			v("AE")
			i++
		case ch == 'e':
			v("EH")
			i++
		case ch == 'i':
			v("IH")
			i++
		case ch == 'o':
			v("AO")
			i++
		case ch == 'u':
			v("AH")
			i++
		case ch == 'y':
			if isV(i + 1) {
				c("Y")
			} else {
				v("IY")
			}
			i++
		// 輔音組合
		case at(i, "tch"):
			c("CH")
			i += 3
		case at(i, "sch"):
			c("S", "K")
			i += 3
		case at(i, "ch"):
			c("CH")
			i += 2
		case at(i, "sh"):
			c("SH")
			i += 2
		case at(i, "th"):
			c("TH")
			i += 2
		case at(i, "ph"):
			c("F")
			i += 2
		case at(i, "gh"):
			if i == 0 {
				c("G")
			}
			i += 2
		case at(i, "ck"):
			c("K")
			i += 2
		case at(i, "qu"):
			c("K", "W")
			i += 2
		case at(i, "wh"):
			c("W")
			i += 2
		case i == 0 && (at(i, "wr") || at(i, "kn")):
			c(map[byte]string{'w': "R", 'k': "N"}[ch])
			i += 2
		case at(i, "ng") && !isV(i+2):
			c("NG")
			i += 2
		case ch == 'x':
			c("K", "S")
			i++
		case ch == 'c':
			if isV(i+1) && strings.ContainsRune("eiy", rune(w[i+1])) {
				c("S")
			} else {
				c("K")
			}
			i++
		case ch == 'g':
			if i > 0 && isV(i+1) && strings.ContainsRune("eiy", rune(w[i+1])) {
				c("JH")
			} else {
				c("G")
			}
			i++
		case ch == 'j':
			c("JH")
			i++
		case ch == 'q':
			c("K")
			i++
		case ch == 'h':
			if isV(i + 1) {
				c("HH")
			}
			i++
		case ch == 'w':
			if isV(i + 1) {
				c("W")
			}
			i++
		default:
			c(map[byte]string{
				'b': "B", 'd': "D", 'f': "F", 'k': "K", 'l': "L", 'm': "M", 'n': "N",
				'p': "P", 'r': "R", 's': "S", 't': "T", 'v': "V", 'z': "Z",
			}[ch])
			i++
		}
	}
	return out
}
