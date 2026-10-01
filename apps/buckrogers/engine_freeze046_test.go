package buckrogers

import (
	"strconv"
	"strings"
	"testing"
)

// Buck repo spec 047 §3.6 (3): the engine joiner of commit 6680221 (spec 046),
// frozen before spec 047 changes the zh-TW and zh-CN template path.
//
// frozenTranslateLine046 is a copy of EngineTextCatalog.translateLine as it
// was at 6680221.  It calls the current joinKo, joinJa, fillTemplateKoJa and
// monsterSlot, which spec 047 does not change.  The digests below were taken
// from the real function at that commit on the corpus of engine_freeze_test.go
// for the ja and ko catalogs; the tests then require the current code to give
// the same answers for ja and ko.  Nothing here may be edited to follow later
// changes.

func frozenTranslateLine046(c *EngineTextCatalog, s string) (string, bool) {
	if z, ok := c.monsterSlot(s); ok {
		return z, true
	}
	parts := c.Decompose(s)
	hasFixed := false
	zh := make([]string, len(parts))
	for i, p := range parts {
		switch p.Kind {
		case 'F':
			hasFixed = true
			z := c.fragText[strings.TrimSuffix(p.Key, ".uc")]
			if z == "" {
				if _, _, tmpl := c.matchTemplate(parts); !tmpl {
					return "", false
				}
			}
			zh[i] = z
		case 'I':
			hasFixed = true
			z, ok := c.itemChinese(p.Text)
			if !ok {
				return "", false
			}
			zh[i] = z
		default:
			if z, ok := c.monsterSlot(p.Text); ok {
				zh[i] = z
			} else {
				zh[i] = p.Text
			}
		}
	}
	if !hasFixed {
		return "", false
	}
	if t, slots, ok := c.matchTemplate(parts); ok {
		if c.lang == LangKo || c.lang == LangJa {
			return fillTemplateKoJa(c.lang, t, slots, parts, zh), true
		}
		for i, v := range slots {
			t = strings.ReplaceAll(t, "{"+strconv.Itoa(i)+"}", strings.TrimSpace(zh[v]))
		}
		return t, true
	}
	switch c.lang {
	case LangKo:
		return joinKo(parts, zh), true
	case LangJa:
		return joinJa(parts, zh), true
	}
	var b strings.Builder
	for i, p := range parts {
		z := zh[i]
		if p.Kind == 'F' {
			if strings.HasPrefix(p.Text, " ") && i > 0 && parts[i-1].Kind == '_' && len(zh[i-1]) > 0 && engineAlnum(zh[i-1][len(zh[i-1])-1]) {
				z = " " + z
			}
			if strings.HasSuffix(p.Text, " ") && i+1 < len(parts) && parts[i+1].Kind == '_' && len(zh[i+1]) > 0 && engineAlnum(zh[i+1][0]) {
				z += " "
			}
		}
		b.WriteString(z)
	}
	return b.String(), true
}

func TestFrozenTranslateLine046MatchesRealAtFreeze(t *testing.T) {
	for _, lang := range []string{LangJa, LangKo} {
		c := engineFreezeCatalog(t, lang)
		got := engineFreezeDigest(c, frozenTranslateLine046)
		t.Logf("%s frozen digest %s", lang, got)
		if want := engineFreeze046Want[lang]; got != want {
			t.Errorf("%s: 凍結副本摘要 %s，應為 %s", lang, got, want)
		}
	}
}

func TestEngineJoinJaKoUnchangedBySpec047(t *testing.T) {
	for _, lang := range []string{LangJa, LangKo} {
		c := engineFreezeCatalog(t, lang)
		real := engineFreezeDigest(c, func(c *EngineTextCatalog, s string) (string, bool) { return c.translateLine(s) })
		if want := engineFreeze046Want[lang]; real != want {
			t.Errorf("%s: 現行 translateLine 摘要 %s，應為 %s", lang, real, want)
		}
	}
}

// Digests of the real translateLine at 6680221 (see the header).  Do not edit.
var engineFreeze046Want = map[string]string{
	LangJa: "d48169780d8e4db6962c38c955da32d9c294f8e98274e8d224d96a78357fb71a",
	LangKo: "c02e0975f248ab967499d355b562214cb123ca933e5fa5bfed791a490785276c",
}
