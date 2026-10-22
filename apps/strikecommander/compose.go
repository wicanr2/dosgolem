package strikecommander

import (
	"regexp"
	"strings"
)

// 武器掛載畫面的組合標籤（strike_commander 規格 002）。熱點標籤經同一條 082A 路徑上畫面，
// 但字串由 sprintf 以範本加名稱與數量組成，例如 "AIM-9J rear aspect missile (42)"。
// 解析由整串字串反推範本、名稱與數量，再以各自的譯文代入（002 §2.2）。
// 範本、短代號、長名稱與兩個式子是解析與 lint 共用的常數（002 §2.1）。

// 兩個範本：OPTTEST.EXE 的格式字串（檔案位移 52F10h、52F18h，DS:10C0、DS:10C8）。
const (
	TemplateCount  = "%s (%d)"
	TemplateRemove = "Remove %s (%d)"
)

// WeaponShortNames 是 DS:09AE 指標表第 0 到 8 筆的武器代號（檔案位移 527FEh，已證實：檔案位元組）。
// 是 Remove 範本 %s 位置的名稱。
var WeaponShortNames = [...]string{
	"AIM-9J", "AIM-9M", "AGM-65D", "Durandal", "MK-20", "MK-82", "GBU-15", "LAU-3", "AIM-120",
}

// WeaponLongNames 是九個武器長名稱（LABL 原文），是 "%s (%d)" 範本 %s 位置的名稱。
var WeaponLongNames = [...]string{
	"AIM-120 AMRAAM", "AIM-9J rear aspect missile", "AIM-9M all aspect missile", "AGM-65D Maverick",
	"Durandal anti-runway bomb", "MK-20 cluster bomb", "MK-82 500 lb. bomb", "GBU-15E Paveway", "LAU-3 rocket pod",
}

// FontExtraRunes 是字型固定附加的字（strike_commander font/zh-TW.extra.txt，002 §2.5）：
// 數量的全形數字與括號不一定出現在正式譯文裡。
const FontExtraRunes = "０１２３４５６７８９（）"

// 兩個式子：數量 1 到 5 位 ASCII 數字（%d 印出的非負 int）。Go 的 $ 只在字串尾成立。
var (
	removeRe = regexp.MustCompile(`^Remove (.+) \(([0-9]{1,5})\)$`)
	countRe  = regexp.MustCompile(`^(.+) \(([0-9]{1,5})\)$`)
)

// ResolveLabel 的 rule 與 Miss.Reason 的值。
const (
	RuleExact  = "exact"
	RuleRemove = "remove"
	RuleCount  = "count"

	MissTranslation = "translation"
	MissTemplate    = "template"
	MissGlyph       = "glyph"
)

func isTemplateKey(k string) bool { return k == TemplateCount || k == TemplateRemove }

func isShortName(s string) bool {
	for _, n := range WeaponShortNames {
		if s == n {
			return true
		}
	}
	return false
}

func isLongName(s string) bool {
	for _, n := range WeaponLongNames {
		if s == n {
			return true
		}
	}
	return false
}

// validTemplate：範本譯文恰好一個 %s、一個 %d（002 §2.2）。執行期載入器不跑 lint，解析自己檢查。
func validTemplate(tr string) bool {
	return strings.Count(tr, "%s") == 1 && strings.Count(tr, "%d") == 1
}

// ResolveLabel 依 002 §2.2 為觀察到的整串熱點標籤選譯文。命中時 reason 為空；
// miss 時 text 為 nil、rule 為空，reason 是 MissTemplate 或 MissTranslation。
// 只選譯文，不改原文、不寫入原版狀態；寬度與缺字由呼叫端照 001 檢查。
func ResolveLabel(key string, catalog map[string]string) (text []rune, rule string, reason string) {
	return resolveLabel(key, catalog, false)
}

// resolveLabel 的 exactOnly 只留第 1 步（002 §3.2 案例 D 的負對照）。
func resolveLabel(key string, catalog map[string]string, exactOnly bool) ([]rune, string, string) {
	// 1. 整串是 catalog key、不是範本。單獨出現的短代號也在這裡命中。
	if t, ok := catalog[key]; ok && !isTemplateKey(key) {
		return []rune(t), RuleExact, ""
	}
	if !exactOnly {
		// 2. Remove 範本：名稱是短代號。與第 3 步互斥（第 3 步只收九個長名稱），先後只為意圖明確。
		if m := removeRe.FindStringSubmatch(key); m != nil && isShortName(m[1]) {
			if t, ok := composeLabel(catalog, TemplateRemove, m[1], m[2]); ok {
				return t, RuleRemove, ""
			}
		}
		// 3. 數量範本：名稱是武器長名稱。
		if m := countRe.FindStringSubmatch(key); m != nil && isLongName(m[1]) {
			if t, ok := composeLabel(catalog, TemplateCount, m[1], m[2]); ok {
				return t, RuleCount, ""
			}
		}
	}
	// 4. miss：形式像組合標籤記 template，否則記 translation。
	if removeRe.MatchString(key) || countRe.MatchString(key) {
		return nil, "", MissTemplate
	}
	return nil, "", MissTranslation
}

// composeLabel 以範本 tpl 的譯文代入名稱的譯文與全形數量。範本譯文不合法、缺範本或缺名稱的譯文時回 false。
// %s、%d 依在譯文中的位置代入，順序不限。
func composeLabel(catalog map[string]string, tpl, name, digits string) ([]rune, bool) {
	tr, ok := catalog[tpl]
	if !ok || !validTemplate(tr) {
		return nil, false
	}
	nameTr, ok := catalog[name]
	if !ok {
		return nil, false
	}
	var b strings.Builder
	for i := 0; i < len(tr); {
		switch {
		case strings.HasPrefix(tr[i:], "%s"):
			b.WriteString(nameTr)
			i += 2
		case strings.HasPrefix(tr[i:], "%d"):
			b.WriteString(fullwidthDigits(digits))
			i += 2
		default:
			b.WriteByte(tr[i])
			i++
		}
	}
	return []rune(b.String()), true
}

// fullwidthDigits 把 ASCII 數字逐位換成全形（０ 到 ９），不補零。
func fullwidthDigits(s string) string {
	var b strings.Builder
	for _, c := range s {
		b.WriteRune('０' + (c - '0'))
	}
	return b.String()
}
