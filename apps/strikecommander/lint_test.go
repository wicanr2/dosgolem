package strikecommander

import (
	"strings"
	"testing"
)

// lintCatalogRows 回合格夾具的列：001 的三個 key、兩個範本、九個短代號與九個長名稱（002 §2.3），
// 以及四個 EXE 字串（003 §3.5）。譯文只用測試圖集有的字。
func lintCatalogRows() []string {
	rows := []string{
		"Talk to Gwen\t測試譯文甲\t已觀察",
		"Fly Mission\t測試丙\t已觀察",
		TemplateCount + "\t%s（%d）\t已觀察",
		TemplateRemove + "\t卸下%s（%d）\t已觀察",
	}
	for _, k := range WeaponShortNames {
		rows = append(rows, k+"\t測試\t未觀察")
	}
	for _, k := range WeaponLongNames {
		rows = append(rows, k+"\t測試譯文\t未觀察")
	}
	rows = append(rows,
		"GROUND\t地面\t已觀察",
		"AIR\t空中\t已觀察",
		"Return to office\t測試\t已觀察",
		"Return to tent\t測試\t未觀察",
	)
	return rows
}

const lintHead = "key\ttranslation\tsource\n"

// lintTSV 由合格夾具組 catalog：drop 的 key 拿掉，replace 的 key 換掉譯文。
func lintTSV(drop []string, replace map[string]string) string {
	var b strings.Builder
	b.WriteString(lintHead)
next:
	for _, r := range lintCatalogRows() {
		k := r[:strings.IndexByte(r, '\t')]
		for _, d := range drop {
			if k == d {
				continue next
			}
		}
		if tr, ok := replace[k]; ok {
			r = k + "\t" + tr + "\tx"
		}
		b.WriteString(r + "\n")
	}
	return b.String()
}

// --- 001 第 6 項、002 §3.1 第 2 項：lint（011 §3.7 的 LintLabels） ---

func TestLint(t *testing.T) {
	fonts := testTextFonts(t)
	labl := testLABL("Talk to Gwen", "Fly Mission")
	good := lintTSV(nil, nil)
	if rep := LintLabels([]byte(good), fonts, labl); !rep.OK() || len(rep.Untranslated) != 0 {
		t.Fatalf("合格 catalog 被判失敗：%+v", rep)
	}
	partial := lintHead + "Talk to Gwen\t測試譯文甲\t已觀察\n"
	if rep := LintLabels([]byte(partial), fonts, labl); !rep.OK() || len(rep.Untranslated) != len(lintCatalogRows())-1 {
		t.Fatalf("漏譯只列出、不判失敗：%+v", rep)
	}
	head := lintHead
	cases := []struct {
		name, tsv, want string
	}{
		{"重複 key", head + "Talk to Gwen\t測試\tx\nTalk to Gwen\t測\tx\n", "重複"},
		{"欄數不對", head + "Talk to Gwen\t測試\n", "欄數"},
		{"欄數過多", head + "Talk to Gwen\t測試\tx\ty\n", "欄數"},
		{"空譯文", head + "Talk to Gwen\t\tx\n", "空的"},
		{"含半形字", head + "Talk to Gwen\t測a試\tx\n", "半形"},
		{"半形空白", head + "Talk to Gwen\t測 試\tx\n", "半形"},
		{"缺字", head + "Talk to Gwen\t測乂\tx\n", "缺字"},
		// 2 倍 22 像素：30 字寬 660，加 2 超過 640。
		{"超寬", head + "Talk to Gwen\t" + strings.Repeat("測", 30) + "\tx\n", "超過 640"},
		{"孤兒 key", head + "Talk to Janet\t測試\tx\n", "孤兒"},
		{"孤兒：組合後的整串", head + "AIM-9J rear aspect missile (42)\t測試\tx\n", "孤兒"},
		{"欄名", "k\tt\ts\nTalk to Gwen\t測試\tx\n", "欄名"},
		// 002 §2.3：範本譯文。
		{"範本半形括號與空白", lintTSV(nil, map[string]string{TemplateCount: "%s (%d)"}), "半形"},
		{"範本缺 %s", lintTSV(nil, map[string]string{TemplateCount: "測（%d）"}), "0 個 %s"},
		{"範本缺 %d", lintTSV(nil, map[string]string{TemplateRemove: "卸下%s（）"}), "0 個 %d"},
		{"範本兩個 %s", lintTSV(nil, map[string]string{TemplateCount: "%s%s（%d）"}), "2 個 %s"},
		{"範本兩個 %d", lintTSV(nil, map[string]string{TemplateRemove: "卸下%s（%d%d）"}), "2 個 %d"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rep := LintLabels([]byte(c.tsv), fonts, labl)
			if rep.OK() || !strings.Contains(strings.Join(rep.Errors, "\n"), c.want) {
				t.Fatalf("要含 %q 的失敗：%+v", c.want, rep.Errors)
			}
		})
	}
	// 29 字（2 倍 638 + 2 = 640、3 倍 928 + 2 = 930）剛好可以
	ok29 := head + "Talk to Gwen\t" + strings.Repeat("測", 29) + "\tx\n"
	if rep := LintLabels([]byte(ok29), fonts, labl); !rep.OK() {
		t.Fatalf("29 字應通過：%+v", rep.Errors)
	}
	// 範本譯文的順序不限（%d 在前）。
	if rep := LintLabels([]byte(lintTSV(nil, map[string]string{TemplateCount: "（%d）%s"})), fonts, labl); !rep.OK() {
		t.Fatalf("（%%d）%%s 應通過：%+v", rep.Errors)
	}
}

// 002 §2.3：缺範本、缺短代號都列入漏譯（不判失敗）。
func TestLintUntranslatedComposed(t *testing.T) {
	fonts := testTextFonts(t)
	labl := testLABL("Talk to Gwen", "Fly Mission")
	for _, k := range []string{TemplateCount, TemplateRemove, "AIM-9J", "AIM-120", "GROUND", "Return to tent"} {
		t.Run(k, func(t *testing.T) {
			rep := LintLabels([]byte(lintTSV([]string{k}, nil)), fonts, labl)
			if !rep.OK() || strings.Join(rep.Untranslated, "|") != k {
				t.Fatalf("缺 %q：%+v", k, rep)
			}
		})
	}
}

// 002 §2.3 的防呆三條：只看 LABL 清單。
func TestLintGuards(t *testing.T) {
	fonts := testTextFonts(t)
	good := []byte(lintTSV(nil, nil))
	noLong := testLABL("Talk to Gwen", "Fly Mission")
	delete(noLong, "LAU-3 rocket pod")
	cases := []struct {
		name string
		labl map[string]bool
		want string
	}{
		{"LABL 含 Remove X (1)", testLABL("Talk to Gwen", "Fly Mission", "Remove X (1)"), "符合組合標籤"},
		{"LABL 含 X (1)", testLABL("Talk to Gwen", "Fly Mission", "X (1)"), "符合組合標籤"},
		{"LABL 含短代號", testLABL("Talk to Gwen", "Fly Mission", "AIM-9J"), "短代號"},
		{"LABL 缺長名稱", noLong, "武器長名稱"},
		// 003 §3.5：ExeKeys 與 LABL 沒有交集。
		{"LABL 含 GROUND", testLABL("Talk to Gwen", "Fly Mission", "GROUND"), "EXE 字串"},
		{"LABL 含 Return to office", testLABL("Talk to Gwen", "Fly Mission", "Return to office"), "EXE 字串"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rep := LintLabels(good, fonts, c.labl)
			if rep.OK() || !strings.Contains(strings.Join(rep.Errors, "\n"), c.want) {
				t.Fatalf("要含 %q 的失敗：%+v", c.want, rep.Errors)
			}
		})
	}
}
