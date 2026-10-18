package strikecommander

import (
	"strings"
	"testing"
)

// composeCatalog 是 002 §3.1 第 1 項的夾具：兩個範本、九個短代號、九個長名稱（譯文同正式 catalog），
// 加上 Talk to Gwen（LABL key 但不是武器長名稱）。
func composeCatalog() map[string]string {
	return map[string]string{
		TemplateCount:                "%s（%d）",
		TemplateRemove:               "卸下%s（%d）",
		"AIM-9J":                     "ＡＩＭ－９Ｊ",
		"AIM-9M":                     "ＡＩＭ－９Ｍ",
		"AGM-65D":                    "ＡＧＭ－６５Ｄ",
		"Durandal":                   "杜蘭達",
		"MK-20":                      "ＭＫ－２０",
		"MK-82":                      "ＭＫ－８２",
		"GBU-15":                     "ＧＢＵ－１５",
		"LAU-3":                      "ＬＡＵ－３",
		"AIM-120":                    "ＡＩＭ－１２０",
		"AIM-120 AMRAAM":             "ＡＩＭ－１２０先進中程飛彈",
		"AIM-9J rear aspect missile": "ＡＩＭ－９Ｊ尾追式飛彈",
		"AIM-9M all aspect missile":  "ＡＩＭ－９Ｍ全向式飛彈",
		"AGM-65D Maverick":           "ＡＧＭ－６５Ｄ小牛飛彈",
		"Durandal anti-runway bomb":  "杜蘭達跑道破壞炸彈",
		"MK-20 cluster bomb":         "ＭＫ－２０集束炸彈",
		"MK-82 500 lb. bomb":         "ＭＫ－８２五百磅炸彈",
		"GBU-15E Paveway":            "ＧＢＵ－１５Ｅ雷射導引炸彈",
		"LAU-3 rocket pod":           "ＬＡＵ－３火箭發射器",
		"Talk to Gwen":               "與葛溫交談",
	}
}

func withCatalog(edit func(c map[string]string)) map[string]string {
	c := composeCatalog()
	edit(c)
	return c
}

// --- 002 §3.1 第 1 項：ResolveLabel ---

func TestResolveLabelHit(t *testing.T) {
	reversed := withCatalog(func(c map[string]string) { c[TemplateCount] = "（%d）%s"; c[TemplateRemove] = "（%d）卸下%s" })
	cases := []struct {
		name, key string
		cat       map[string]string
		text      string
		rule      string
	}{
		{"整串 key", "Talk to Gwen", nil, "與葛溫交談", RuleExact},
		{"整串是長名稱", "AIM-9J rear aspect missile", nil, "ＡＩＭ－９Ｊ尾追式飛彈", RuleExact},
		{"單獨的短代號", "AIM-9J", nil, "ＡＩＭ－９Ｊ", RuleExact},
		{"數量", "AIM-9J rear aspect missile (42)", nil, "ＡＩＭ－９Ｊ尾追式飛彈（４２）", RuleCount},
		{"卸下", "Remove AIM-9J (2)", nil, "卸下ＡＩＭ－９Ｊ（２）", RuleRemove},
		{"卸下 Durandal", "Remove Durandal (17)", nil, "卸下杜蘭達（１７）", RuleRemove},
		{"五位數", "MK-82 500 lb. bomb (32767)", nil, "ＭＫ－８２五百磅炸彈（３２７６７）", RuleCount},
		{"數量 0", "LAU-3 rocket pod (0)", nil, "ＬＡＵ－３火箭發射器（０）", RuleCount},
		{"順序不同：數量", "AIM-9J rear aspect missile (42)", reversed, "（４２）ＡＩＭ－９Ｊ尾追式飛彈", RuleCount},
		{"順序不同：卸下", "Remove AIM-120 (1)", reversed, "（１）卸下ＡＩＭ－１２０", RuleRemove},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cat := c.cat
			if cat == nil {
				cat = composeCatalog()
			}
			text, rule, reason := ResolveLabel(c.key, cat)
			if string(text) != c.text || rule != c.rule || reason != "" {
				t.Fatalf("ResolveLabel(%q) = %q %q %q，要 %q %q", c.key, string(text), rule, reason, c.text, c.rule)
			}
		})
	}
}

func TestResolveLabelMiss(t *testing.T) {
	noCount := withCatalog(func(c map[string]string) { delete(c, TemplateCount) })
	noRemove := withCatalog(func(c map[string]string) { delete(c, TemplateRemove) })
	badRemove := withCatalog(func(c map[string]string) { c[TemplateRemove] = "卸下%s（）" })
	badCount := withCatalog(func(c map[string]string) { c[TemplateCount] = "%s（%d%d）" })
	noName := withCatalog(func(c map[string]string) { delete(c, "AIM-9J") })
	cases := []struct {
		name, key string
		cat       map[string]string
		reason    string
	}{
		// 整串是範本 key：第 1 步排除，也不符兩式（數量不是數字）。
		{"整串是範本 %s (%d)", TemplateCount, nil, MissTranslation},
		{"整串是範本 Remove", TemplateRemove, nil, MissTranslation},
		{"Foo (3)", "Foo (3)", nil, MissTemplate},
		{"LABL key 但不是長名稱", "Talk to Gwen (3)", nil, MissTemplate},
		{"短代號配數量範本", "AIM-9J (3)", nil, MissTemplate},
		{"範本 key 帶數量", "%s (%d) (3)", nil, MissTemplate},
		{"Remove 非短代號", "Remove Foo (3)", nil, MissTemplate},
		{"Remove 長名稱", "Remove AIM-9J rear aspect missile (3)", nil, MissTemplate},
		{"已帶數量再組", "AIM-9J rear aspect missile (42) (41)", nil, MissTemplate},
		{"缺 %s (%d)", "AIM-9J rear aspect missile (42)", noCount, MissTemplate},
		{"缺 Remove %s (%d)", "Remove AIM-9J (2)", noRemove, MissTemplate},
		{"範本譯文缺 %d", "Remove AIM-9J (2)", badRemove, MissTemplate},
		{"範本譯文兩個 %d", "AIM-9J rear aspect missile (42)", badCount, MissTemplate},
		{"缺短代號譯文", "Remove AIM-9J (2)", noName, MissTemplate},
		// 下列兩式都不符：數量必須 1 到 5 位數字（規格 §3.1 列為 template，與 §2.2 第 4 步不一致，見報告）。
		{"六位數", "AIM-9J rear aspect missile (123456)", nil, MissTranslation},
		{"空數量", "AIM-9J rear aspect missile ()", nil, MissTranslation},
		{"無空白", "AIM-9J rear aspect missile(42)", nil, MissTranslation},
		{"ASCII 負號", "AIM-9J rear aspect missile (-1)", nil, MissTranslation},
		{"尾端雜訊", "AIM-9J rear aspect missile (42)x", nil, MissTranslation},
		{"Remove 尾端空白", "Remove AIM-9J (2) ", nil, MissTranslation},
		// 下列兩串不符 removeRe，但符合 countRe（(.+) 吃下整段前綴），依 §2.2 第 4 步記 template
		// （規格 §3.1 列為 translation，與 §2.2 第 4 步不一致，見報告）。
		{"前綴雜訊", "xRemove AIM-9J (2)", nil, MissTemplate},
		{"小寫 remove", "remove AIM-9J (2)", nil, MissTemplate},
		{"不在 catalog", "Talk to Janet", nil, MissTranslation},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cat := c.cat
			if cat == nil {
				cat = composeCatalog()
			}
			text, rule, reason := ResolveLabel(c.key, cat)
			if text != nil || rule != "" || reason != c.reason {
				t.Fatalf("ResolveLabel(%q) = %q %q %q，要 miss %q", c.key, string(text), rule, reason, c.reason)
			}
		})
	}
}

// --- 002 §3.1 第 4 項：Overlay.Sync ---

func composedOverlay(t *testing.T, cat map[string]string) *Overlay {
	t.Helper()
	ov, err := NewOverlay(testLabelFont(t), cat, nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	return ov
}

func TestSyncComposed(t *testing.T) {
	cat := map[string]string{
		TemplateCount:                "%s（%d）",
		TemplateRemove:               "卸下%s（%d）",
		"AIM-9J":                     "ＡＩＭ－９Ｊ",
		"AIM-9J rear aspect missile": "ＡＩＭ－９Ｊ尾追式飛彈",
	}
	for _, c := range []struct{ key, text string }{
		{"AIM-9J rear aspect missile (42)", "ＡＩＭ－９Ｊ尾追式飛彈（４２）"},
		{"Remove AIM-9J (2)", "卸下ＡＩＭ－９Ｊ（２）"},
	} {
		t.Run(c.key, func(t *testing.T) {
			ov := composedOverlay(t, cat)
			ov.Sync([]Label{{Key: c.key, X0: 100, X1: 200, Y: 175, Font: measuredFont}}, 1)
			st := ov.Stamps()
			if len(st) != 2 || len(ov.Misses) != 0 || len(ov.Skips) != 0 {
				t.Fatalf("stamps %d misses %+v skips %+v", len(st), ov.Misses, ov.Skips)
			}
			if string(st[1].Text) != c.text || st[1].Key != c.key+".text" || st[0].Key != c.key+".cover" {
				t.Fatalf("文字 stamp %q %q，要 %q", st[1].Key, string(st[1].Text), c.text)
			}
		})
	}
}

func TestSyncComposedWidth(t *testing.T) {
	// 名稱譯文 n 字加「（４２）」4 字：36 字名稱共 40 字（2 倍寬 320）過，37 字名稱共 41 字不過。
	for _, c := range []struct {
		n             int
		stamps, skips int
	}{{36, 2, 0}, {37, 0, 1}} {
		cat := map[string]string{TemplateCount: "%s（%d）", "MK-20 cluster bomb": strings.Repeat("測", c.n)}
		ov := composedOverlay(t, cat)
		ov.Sync([]Label{{Key: "MK-20 cluster bomb (42)", X0: 100, X1: 200, Y: 175, Font: measuredFont}}, 1)
		if len(ov.Stamps()) != c.stamps || len(ov.Misses) != 0 || len(ov.Skips) != c.skips {
			t.Fatalf("名稱 %d 字：stamps %d misses %+v skips %+v", c.n, len(ov.Stamps()), ov.Misses, ov.Skips)
		}
	}
}

func TestSyncComposedGlyphMiss(t *testing.T) {
	// 測試字型缺 ７：(17) 記 glyph miss、不產生 stamp；(16) 照常產生。
	cat := map[string]string{TemplateRemove: "卸下%s（%d）", "AIM-9J": "ＡＩＭ－９Ｊ"}
	ov := composedOverlay(t, cat)
	ov.Sync([]Label{{Key: "Remove AIM-9J (17)", X0: 100, X1: 200, Y: 175, Font: measuredFont}}, 1)
	if len(ov.Stamps()) != 0 || len(ov.Misses) != 1 || ov.Misses[0].Reason != MissGlyph || string(ov.Misses[0].Runes) != "７" {
		t.Fatalf("(17)：stamps %d misses %+v", len(ov.Stamps()), ov.Misses)
	}
	ov.Sync([]Label{{Key: "Remove AIM-9J (16)", X0: 100, X1: 200, Y: 175, Font: measuredFont}}, 2)
	if len(ov.Stamps()) != 2 {
		t.Fatalf("(16)：stamps %d misses %+v", len(ov.Stamps()), ov.Misses)
	}
}

func TestSyncComposedTemplateMiss(t *testing.T) {
	ov := composedOverlay(t, map[string]string{"AIM-9J": "ＡＩＭ－９Ｊ"})
	ov.Sync([]Label{{Key: "Remove AIM-9J (2)", X0: 100, X1: 200, Y: 175, Font: measuredFont}}, 1)
	if len(ov.Stamps()) != 0 || len(ov.Misses) != 1 || ov.Misses[0].Reason != MissTemplate {
		t.Fatalf("缺範本：stamps %d misses %+v", len(ov.Stamps()), ov.Misses)
	}
}
