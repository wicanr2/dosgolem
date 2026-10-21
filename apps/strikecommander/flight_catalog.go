package strikecommander

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// 飛行字串的 catalog、解析與 lint（strike_commander 規格 009 §3.4、§3.10）。
// 三個檔：text/strike_hud.<lang>.tsv（hud）、strike_mfd（mfd）、strike_plaque（plaque），格式同 001 §2.7。

// FlightCatalogs 是三個種類的 catalog（catalog key → 譯文）。
type FlightCatalogs struct {
	HUD, MFD, Plaque map[string]string
}

// Table 回種類的 catalog；不是飛行種類時回 nil。
func (c FlightCatalogs) Table(k Kind) map[string]string {
	switch k {
	case KindHUD:
		return c.HUD
	case KindMFD:
		return c.MFD
	case KindPlaque:
		return c.Plaque
	}
	return nil
}

var flightDigitRe = regexp.MustCompile(`^([0-9]{1,5}) (.+)$`)

// ResolveFlight 是 §3.4 的解析：精確命中時 k = 0；hud、mfd 另接受數字前綴 `^[0-9]{1,5} `，k 是數字長度加 1。
// 其他回 ok 為假。觀察器與 Overlay 共用。
func ResolveFlight(kind Kind, s string, cats FlightCatalogs) (catKey string, k int, ok bool) {
	t := cats.Table(kind)
	if t == nil {
		return "", 0, false
	}
	if _, hit := t[s]; hit {
		return s, 0, true
	}
	if kind == KindHUD || kind == KindMFD {
		if m := flightDigitRe.FindStringSubmatch(s); m != nil {
			if _, hit := t[m[2]]; hit {
				return m[2], len(m[1]) + 1, true
			}
		}
	}
	return "", 0, false
}

// flightAllowed 是各檔允許的 key（§1.1、§3.10）。
var flightAllowed = map[Kind][]string{
	KindHUD:    {"GEAR", "FLAPS", "BRAKE", "DGFT", "STRF", "SRM", "NONE"},
	KindMFD:    {"Comm Mode:", "Select Frequency", "NO RECEIVERS", "request status", "NORM", "GUN", "FUEL:", "AIR", "GND"},
	KindPlaque: {"time burst", "enemies near", "chase plane", "external view", "victim view on", "weapon camera on"},
}

// flightMaxRunes 是字數上限（§3.10）：hud 照 §3.7.1 表（NONE 以 4 為上限）；mfd 10；plaque 依英文墨跡寬。
var flightMaxRunes = map[Kind]map[string]int{
	KindHUD: {"GEAR": 6, "FLAPS": 5, "BRAKE": 5, "DGFT": 8, "STRF": 8, "SRM": 8, "NONE": 4},
	KindPlaque: {"time burst": 8, "enemies near": 10, "chase plane": 10, "external view": 11,
		"victim view on": 11, "weapon camera on": 15},
}

const flightMFDMaxRunes = 10

// GlossaryRow 是 text/glossary.tsv 的一列（四欄 en、zh-TW、basis、note）。
type GlossaryRow struct {
	En, ZhTW, Basis, Note string
}

const glossaryHeader = "en\tzh-TW\tbasis\tnote"

// ParseGlossaryRows 讀 text/glossary.tsv（第一列欄名、每列四欄）。規格 009 §3.10 稱 ParseGlossary；
// 同名函式已由 010 用於另一種回傳型別（speech_lint.go），這裡改名。
func ParseGlossaryRows(b []byte) ([]GlossaryRow, error) {
	if !utf8.Valid(b) {
		return nil, fmt.Errorf("strikecommander: glossary 不是合法 UTF-8")
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 || lines[0] != glossaryHeader {
		return nil, fmt.Errorf("strikecommander: glossary 第 1 列欄名要是 en、zh-TW、basis、note")
	}
	var out []GlossaryRow
	for i, ln := range lines[1:] {
		c := strings.Split(ln, "\t")
		if len(c) != 4 {
			return nil, fmt.Errorf("strikecommander: glossary 第 %d 列欄數 %d，要 4", i+2, len(c))
		}
		out = append(out, GlossaryRow{c[0], c[1], c[2], c[3]})
	}
	return out, nil
}

// normalizeFullwidthASCII 把全形 ASCII（U+FF01 到 U+FF5E，含全形括號）換成對應半形（§3.10 glossary 一致性）。
func normalizeFullwidthASCII(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 0xFF01 && r <= 0xFF5E {
			r -= 0xFEE0
		}
		b.WriteRune(r)
	}
	return b.String()
}

// LintFlight 依 §3.10 檢查一份飛行 catalog。wide 在 hud 用 HUD12 的寬度表，mfd、plaque 用正式字型的。
func LintFlight(kind Kind, b []byte, wide *WideTable, glossary []GlossaryRow) []error {
	return lintFlightCore(kind, b, glossary, func(bad badFunc, r catalogRow) {
		for _, c := range r.tr {
			if c < 0x80 || (wide.Has(c) && !wide.Wide(c)) {
				bad("第 %d 列 key %q 含非全形字 %q（U+%04X）", r.line, r.key, c, c)
			}
		}
		n := utf8.RuneCountInString(r.tr)
		limit := flightMFDMaxRunes
		if kind != KindMFD {
			limit = flightMaxRunes[kind][r.key]
		}
		if limit > 0 && n > limit {
			bad("第 %d 列 key %q 譯文 %d 字，上限 %d", r.line, r.key, n, limit)
		}
	})
}

// lintFlightCore 是 LintFlight 與 LintFlightText（strike_commander 規格 011 §3.7）共用的結構與 glossary 檢查；
// 每個譯文非空的列交給 check 檢查字元與寬度。
func lintFlightCore(kind Kind, b []byte, glossary []GlossaryRow, check func(bad badFunc, r catalogRow)) []error {
	var errs []error
	bad := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	allowed, okKind := flightAllowed[kind]
	if !okKind {
		bad("種類 %q 不是飛行種類", kind)
		return errs
	}
	header, rows, err := splitCatalog(b)
	if err != nil {
		bad("%v", err)
		return errs
	}
	if header != catalogHeader {
		bad("第 1 列欄名要是 key、translation、source")
	}
	allow := map[string]bool{}
	for _, k := range allowed {
		allow[k] = true
	}
	digit := regexp.MustCompile(`^[0-9]{1,5} `)
	seen := map[string]int{}
	for _, r := range rows {
		if r.cols != 3 {
			bad("第 %d 列欄數 %d，要 3", r.line, r.cols)
			continue
		}
		if first, dup := seen[r.key]; dup {
			bad("第 %d 列 key %q 與第 %d 列重複", r.line, r.key, first)
		} else {
			seen[r.key] = r.line
		}
		if digit.MatchString(r.key) {
			bad("第 %d 列 key %q 以數字前綴開頭（數字前綴由解析處理）", r.line, r.key)
		}
		if kind == KindHUD && (strings.Contains(r.key, "Vulcan") || strings.Contains(r.key, "AIM-")) {
			bad("第 %d 列 key %q 是武器名；HUD 只譯狀態字（AGENTS 2026-10-20）", r.line, r.key)
		}
		if !allow[r.key] {
			bad("第 %d 列 key %q 不在 %s 允許的集合（孤兒 key）", r.line, r.key, kind)
		}
		if r.tr == "" {
			bad("第 %d 列 key %q 譯文是空的", r.line, r.key)
			continue
		}
		check(bad, r)
		for _, g := range glossary {
			if g.En != r.key {
				continue
			}
			if normalizeFullwidthASCII(g.ZhTW) != normalizeFullwidthASCII(r.tr) && !strings.Contains(g.Note, "語境不同") {
				bad("第 %d 列 key %q 的譯文 %q 與 glossary 的 %q 不一致", r.line, r.key, r.tr, g.ZhTW)
			}
		}
	}
	var miss []string
	for _, k := range allowed {
		if _, ok := seen[k]; !ok {
			miss = append(miss, k)
		}
	}
	if len(miss) != 0 {
		bad("漏譯 %d：%q", len(miss), miss)
	}
	return errs
}
