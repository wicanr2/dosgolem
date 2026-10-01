package buckrogers

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// Buck repo spec 047 §3.3 and §3.6 (3): for zh-TW and zh-CN the engine
// joiner differs from the frozen pre-046 code (frozenTranslateLine) only for
// strings that match a template with a plain-slot value of digits plus one
// period whose placeholder is in the template.  Which strings those are is
// decided with Decompose and matchTemplate (unchanged functions) and the
// test's own regular expression; the expected output is the frozen copy's
// answer for the string without that period, plus 。 unless the sentence
// already ends with a terminator.

var zh047Fragments = [][2]string{
	{"Needs ", "需要"}, {" points in ", "點於"}, {" is destroyed in ", "被摧毀於"}, {"Count ", "計數"}, {" now? ", "現在嗎？"},
	{"Total ", "總計"}, {" done. ", "完成。"},
}

func engineCatalogP047(t *testing.T, lang string) *EngineTextCatalog {
	t.Helper()
	frags := append(append([][2]string{}, engineFreezeFragments...), zh047Fragments...)
	var origs, pairs []string
	for _, f := range frags {
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
			"tpl.logbook\t" + h(" and you record ") + "|_|" + h(" as logbook entry ") + "|_\n" +
			"tpl.needs\t" + h("Needs ") + "|_|" + h(" points in ") + "|*\n" +
			"tpl.destroyed\t_|" + h(" is destroyed in ") + "|_\n" +
			"tpl.count\t" + h("Count ") + "|_|" + h(" now? ") + "\n" +
			"tpl.total\t" + h("Total ") + "|_|" + h(" done. ") + "\n"),
		TemplateText: []byte("key\ttranslation\tsource\n" +
			"tpl.drop\t永久丟棄{0}嗎？\tecl-batch-editorial\n" +
			"tpl.antidote\t對{0}使用解毒劑\tecl-batch-editorial\n" +
			"tpl.logbook\t，已記入手札，編號 {1}\tecl-batch-editorial\n" +
			"tpl.needs\t{1}還需要{0}點\tecl-batch-editorial\n" +
			"tpl.destroyed\t{0}在{1}被摧毀\tecl-batch-editorial\n" +
			"tpl.count\t計數{0}現在嗎\tecl-batch-editorial\n" +
			"tpl.total\t總計{0}，完成\tecl-batch-editorial\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func engineCorpusP047() []string {
	out := engineFreezeCorpus()
	return append(out,
		"Needs 5 points in 2.", "Needs 5. points in 2", "Needs 5 points in FLAVIUS", "Needs 5 points in NEO WARRIOR",
		"NEO WARRIOR is destroyed in 3.", "3. is destroyed in 4.", "FLAVIUS is destroyed in 12", "7. is destroyed in FLAVIUS",
		"Count 7. now? ", "Count 7 now? ", "Count NO. now? ", "Count FLAVIUS now? ",
		"Total 9. done. ", "Total 9 done. ",
		"Drop 5. forever? ", "Drop Bolt Gun forever? ",
		"Use antidote on 5.", "Use antidote on 5", "Use antidote on NO.", "Use antidote on 12.5.",
		" and you record IT as logbook entry 38.", " and you record 4. as logbook entry 38.", " and you record IT as logbook entry 38?",
	)
}

var zh047DigitsDot = regexp.MustCompile(`^[0-9]+\.$`)

// zh047Expected is the answer the frozen copy and the §3.3 rule give together.
func zh047Expected(c *EngineTextCatalog, s string) (string, bool, bool) {
	frozen, ok := frozenTranslateLine(c, s)
	if !ok {
		return "", false, false
	}
	if _, isMonster := c.monsterSlot(s); isMonster {
		return frozen, true, false
	}
	parts := c.Decompose(s)
	tpl, slots, isTpl := c.matchTemplate(parts)
	if !isTpl {
		return frozen, true, false
	}
	stripped := false
	var b strings.Builder
	for i, p := range parts {
		text := p.Text
		for idx, v := range slots {
			if v != i || p.Kind != '_' || !strings.Contains(tpl, "{"+fmt.Sprint(idx)+"}") {
				continue
			}
			if core := strings.TrimSpace(p.Text); zh047DigitsDot.MatchString(core) {
				text = strings.Replace(p.Text, core, strings.TrimSuffix(core, "."), 1)
				stripped = true
			}
		}
		b.WriteString(text)
	}
	if !stripped {
		return frozen, true, false
	}
	z, ok := frozenTranslateLine(c, b.String())
	if !ok {
		return "", false, true
	}
	if !strings.ContainsRune("。！？」』…", []rune(z)[len([]rune(z))-1]) {
		z += "。"
	}
	return z, true, true
}

func TestEngineJoinZhDifferenceIsTemplateDigitPeriod(t *testing.T) {
	for _, lang := range []string{LangZhTW, LangZhCN} {
		c := engineCatalogP047(t, lang)
		var diff, same int
		for _, s := range engineCorpusP047() {
			want, wantOK, inDiff := zh047Expected(c, s)
			got, gotOK := c.translateLine(s)
			if got != want || gotOK != wantOK {
				t.Errorf("%s %q：(%q,%v)，應為 (%q,%v)", lang, s, got, gotOK, want, wantOK)
			}
			if inDiff {
				diff++
				if frozen, _ := frozenTranslateLine(c, s); frozen == got {
					t.Errorf("%s %q：差集字串的輸出沒有變：%q", lang, s, got)
				}
			} else {
				same++
				if frozen, fok := frozenTranslateLine(c, s); frozen != got || fok != gotOK {
					t.Errorf("%s %q：差集以外的字串輸出變了：%q → %q", lang, s, frozen, got)
				}
			}
		}
		t.Logf("%s：語料 %d 個，差集 %d、不變 %d", lang, diff+same, diff, same)
		if diff < 12 {
			t.Errorf("%s：差集只有 %d 個，語料涵蓋不足", lang, diff)
		}
	}
	// The cases of the spec, spelled out.
	c := engineCatalogP047(t, LangZhTW)
	for in, want := range map[string]string{
		" and you record IT as logbook entry 38.": "，已記入手札，編號 38。",
		" and you record IT as logbook entry 38":  "，已記入手札，編號 38",
		" and you record IT as logbook entry NO.": "，已記入手札，編號 NO.",
		"Use antidote on 5.":                      "對5使用解毒劑。",
		"Use antidote on FLAVIUS":                 "對FLAVIUS使用解毒劑",
		"Drop 5. forever? ":                       "永久丟棄5嗎？",
		"Count 7. now? ":                          "計數7現在嗎。",
	} {
		if got, _ := c.translateLine(in); got != want {
			t.Errorf("%q → %q，應為 %q", in, got, want)
		}
	}
}

// ja and ko do not change with spec 047 (the digests are in
// engine_freeze046_test.go); a template with its field at the end keeps the
// period there.
func TestEngineJoinJaKoKeepFieldPeriodAtEnd(t *testing.T) {
	for _, lang := range []string{LangJa, LangKo} {
		c := engineCatalogP047(t, lang)
		got, _ := c.translateLine(" and you record IT as logbook entry 38.")
		if !strings.Contains(got, "38.") {
			t.Errorf("%s：欄位在末尾時 ja、ko 不去句點：%q", lang, got)
		}
	}
}
