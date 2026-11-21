package dkk

import "testing"

func newTranslator(t *testing.T) *Translator {
	t.Helper()
	ui := catalogOf(t, "Long Bow", "長弓", "Long", "長", "Mace", "釘頭錘", "Spell:", "法術：", "Hold Person", "人類定身術",
		"Dart", "飛鏢", "of", "之", "Hornet's Nest", "黃蜂巢", "Paladine", "帕拉丁", "Cleric", "牧師",
		" <No Spell Effects>", "<無法術效果>")
	names := catalogOf(t, "DREAD WOLF", "恐狼")
	tr := &Translator{Exact: []Lookuper{ui}, Names: names, Words: ui}
	tpl := "template\ttranslation\tnote\n" +
		"Hitting for {n} points  of damage\t造成 {n} 點傷害\t\n" +
		"{n},{n} N {n}:{n}\t{1},{2} 北 {3}:{4}\t\n" +
		"Spells in Grimoire\t法術書中的法術\t\n"
	if err := tr.LoadTemplates([]byte(tpl)); err != nil {
		t.Fatal(err)
	}
	return tr
}

func TestTranslatorTemplates(t *testing.T) {
	tr := newTranslator(t)
	for in, want := range map[string]string{
		"Hitting for 15 points  of damage": "造成 15 點傷害",
		"6,6 N 06:08":                      "6,6 北 06:08",
	} {
		if e, ok := tr.Lookup([]byte(in)); !ok || e.Translation != want {
			t.Errorf("%q → %q，要 %q", in, e.Translation, want)
		}
	}
	if _, ok := tr.Lookup([]byte("Hitting for many points  of damage")); ok {
		t.Error("沒有數字的字串不走模板")
	}
}

func TestTranslatorItemNames(t *testing.T) {
	tr := newTranslator(t)
	for in, want := range map[string]string{
		"Long Bow +3 ":           "長弓 +3",
		"Mace +4 ":               "釘頭錘 +4",
		"Dart of Hornet's Nest ": "飛鏢之黃蜂巢",
	} {
		if e, ok := tr.Lookup([]byte(in)); !ok || e.Translation != want {
			t.Errorf("%q → %q，要 %q", in, e.Translation, want)
		}
	}
	if _, ok := tr.Lookup([]byte("Long Unknown +1 ")); ok {
		t.Error("任一詞組查不到就整串不處理")
	}
	if _, ok := tr.Lookup([]byte("Long Unknown")); ok {
		t.Error("不以空白結尾的詞組也要每段都查得到")
	}
}

// TestTranslatorG3bRules 是 G3b 盤點補的規則（規格 004 §2.1、§2.2、§2.6、規格 001 §2.2）。
func TestTranslatorG3bRules(t *testing.T) {
	tr := newTranslator(t)
	for in, want := range map[string]string{
		"Paladine Cleric":    "帕拉丁牧師",     // 不以空白結尾的詞組
		"Long Bow +3":        "長弓 +3",     // 反白列不帶結尾空白
		" Yes Mace +4 ":      " 是 釘頭錘 +4", // 清單列：前導空白保留
		"Yes Mace +4":        "是 釘頭錘 +4",  // 反白列
		"No  Long Bow +3":    "否 長弓 +3",
		"Spells in Grimoire": "法術書中的法術", // 沒有數字的模板
		"<No Spell Effects>": "<無法術效果>", // 反白列少了前導空白
	} {
		if e, ok := tr.Lookup([]byte(in)); !ok || e.Translation != want {
			t.Errorf("%q → %q，要 %q", in, e.Translation, want)
		}
	}
	if _, ok := tr.Lookup([]byte("Yes Unknown")); ok {
		t.Error("備妥列的物品名查不到就整串不處理")
	}
}

func TestTranslatorNamesAndLabels(t *testing.T) {
	tr := newTranslator(t)
	if e, ok := tr.Lookup([]byte("DREAD WOLF")); !ok || e.Translation != "恐狼" {
		t.Errorf("怪物名：%+v", e)
	}
	if e, ok := tr.Lookup([]byte("Spell:Hold Person")); !ok || e.Translation != "法術：人類定身術" {
		t.Errorf("標籤：%+v", e)
	}
	if _, ok := tr.Lookup([]byte("Spell:Unknown")); ok {
		t.Error("標籤後段查不到就不處理")
	}
	if _, ok := tr.Lookup([]byte("ANNA")); ok {
		t.Error("玩家名不該命中")
	}
}
