package buckrogers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// Buck repo spec 046 §3.5: the engine joiner before spec 046, frozen.
//
// frozenTranslateLine is a copy of EngineTextCatalog.translateLine as it was
// at commit cc242ea (before the Korean and Japanese join rules).  The digest
// below was computed from the real function at that commit on the corpus
// below, so a matching digest shows the copy is faithful; the other tests
// then require the current code to give the same answers as the copy for
// every language that has no rule of its own (zh-TW, zh-CN, zz, empty).
// Nothing here may be edited to follow later changes.

func frozenTranslateLine(c *EngineTextCatalog, s string) (string, bool) {
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
		for i, v := range slots {
			t = strings.ReplaceAll(t, "{"+strconv.Itoa(i)+"}", strings.TrimSpace(zh[v]))
		}
		return t, true
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

// engineFreezeFragments are the fragment originals of the corpus catalog and
// their zh-TW translations.
var engineFreezeFragments = [][2]string{
	{" makes ", "進行"}, {"his", "他的"}, {" tactics roll.", "戰術判定。"}, {"Hitting for ", "造成"},
	{" points ", "點"}, {"of damage", "傷害"}, {" is hit FOR ", "被擊中"}, {" points of Damage.", "點傷害。"},
	{"Drop ", "丟棄"}, {" forever? ", "永久嗎？"}, {" and you record ", "並且記錄"}, {" as logbook entry ", "為手札編號"},
	{"Use antidote on ", "使用解毒劑於"}, {"(from behind) ", "（背後）"}, {" is", "是"}, {" gains experience!", "獲得經驗！"},
	{", Move Left = ", "，剩餘移動 ="},
}

// engineFreezeCatalog builds the corpus catalog for a language code; the
// Chinese texts are the same for every code, only Lang differs.
func engineFreezeCatalog(t *testing.T, lang string) *EngineTextCatalog {
	t.Helper()
	var origs, pairs []string
	for _, f := range engineFreezeFragments {
		origs = append(origs, f[0])
		pairs = append(pairs, f[0], f[1])
	}
	h := func(s string) string { x := sha256.Sum256([]byte(s)); return fmt.Sprintf("frag.%x", x[:6]) }
	c, err := LoadEngineTextCatalog(EngineTextFiles{
		Lang:           lang,
		FragmentEvents: engineTSV("frag", origs...),
		FragmentText:   engineZh("frag", pairs...),
		ItemEvents:     engineTSV("item", "Bolt", "Gun", "Laser", "Pistol"),
		ItemText:       engineZh("item", "Bolt", "爆能", "Gun", "槍", "Laser", "雷射", "Pistol", "手槍"),
		MonsterEvents:  engineTSV("monster", "TERRINE WARRIOR", "NEO WARRIOR"),
		MonsterText:    engineZh("monster", "TERRINE WARRIOR", "特林戰士", "NEO WARRIOR", "NEO 戰士"),
		TemplateEvents: []byte("event_key\tsignature\n" +
			"tpl.drop\t" + h("Drop ") + "|*|" + h(" forever? ") + "\n" +
			"tpl.antidote\t" + h("Use antidote on ") + "|_\n" +
			"tpl.logbook\t" + h(" and you record ") + "|_|" + h(" as logbook entry ") + "|_\n"),
		TemplateText: []byte("key\ttranslation\tsource\n" +
			"tpl.drop\t永久丟棄{0}嗎？\tecl-batch-editorial\n" +
			"tpl.antidote\t對{0}使用解毒劑\tecl-batch-editorial\n" +
			"tpl.logbook\t，已記入手札，編號 {1}\tecl-batch-editorial\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// engineFreezeCorpus is a fixed pseudo-random set of engine strings: names,
// numbers (some with a period), fragments, item names, spaces and monster
// names with punctuation, 1 to 6 pieces each.
func engineFreezeCorpus() []string {
	seed := uint64(0x5deece66d)
	next := func(n int) int {
		seed = seed*6364136223846793005 + 1442695040888963407
		return int((seed >> 33) % uint64(n))
	}
	pieces := []string{"FLAVIUS", "ROARKE", "CELESTE ", " PIERRE", "3", "38.", "7", "12 ", "NEO WARRIOR.", "TERRINE WARRIOR", "Bolt Gun", "Laser Pistol", "Bolt Gun (72)", " (5)", "IT", "x", "5.", "  ", " "}
	for _, f := range engineFreezeFragments {
		pieces = append(pieces, f[0], f[0])
	}
	var out []string
	for n := 0; n < 3000; n++ {
		var b strings.Builder
		for p, np := 0, 1+next(6); p < np; p++ {
			b.WriteString(pieces[next(len(pieces))])
		}
		out = append(out, b.String())
	}
	// Whole-string cases the pieces rarely produce.
	out = append(out, "Use antidote on FLAVIUS", "Use antidote on 5.", "Drop Bolt Gun forever? ", " and you record IT as logbook entry 38.", " and you record IT as logbook entry 38",
		"NEO WARRIOR.", "NEO WARRIOR", "TERRINE WARRIOR!", "FLAVIUS makes his tactics roll.", "(from behind) Hitting for 3 points of damage")
	return out
}

func engineFreezeDigest(c *EngineTextCatalog, fn func(*EngineTextCatalog, string) (string, bool)) string {
	h := sha256.New()
	for i, s := range engineFreezeCorpus() {
		z, ok := fn(c, s)
		fmt.Fprintf(h, "%d|%q|%v|%q\n", i, s, ok, z)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Digest of the real pre-046 translateLine (commit cc242ea) on the corpus.
// Do not edit.
const engineFreezeWant = "b7b9bc5f39afd8b0fc417f17ff7b33ac46f01535060807b11be1662394ac82f1"

func TestFrozenTranslateLineMatchesPre046(t *testing.T) {
	c := engineFreezeCatalog(t, "")
	t.Logf("語料：%d 個引擎字串", len(engineFreezeCorpus()))
	if got := engineFreezeDigest(c, frozenTranslateLine); got != engineFreezeWant {
		t.Errorf("凍結副本摘要 %s，應為 %s", got, engineFreezeWant)
	}
}
