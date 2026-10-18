package strikecommander

import (
	"strings"
	"testing"
)

// --- 第 6 項：lint ---

func TestLint(t *testing.T) {
	wide := testLabelFont(t).Wide
	labl := map[string]bool{"Talk to Gwen": true, "Fly Mission": true, "AIM-9J rear aspect missile": true}
	head := "key\ttranslation\tsource\n"
	good := head + "Talk to Gwen\t測試譯文甲\t已觀察\nFly Mission\t測試丙\t已觀察\nAIM-9J rear aspect missile\tＡＩＭ－９Ｊ\t未觀察\n"
	if rep := Lint([]byte(good), wide, labl); !rep.OK() || len(rep.Untranslated) != 0 {
		t.Fatalf("合格 catalog 被判失敗：%+v", rep)
	}
	partial := head + "Talk to Gwen\t測試譯文甲\t已觀察\n"
	if rep := Lint([]byte(partial), wide, labl); !rep.OK() || strings.Join(rep.Untranslated, "|") != "AIM-9J rear aspect missile|Fly Mission" {
		t.Fatalf("漏譯只列出、不判失敗：%+v", rep)
	}
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
		{"超寬", head + "Talk to Gwen\t" + strings.Repeat("測", 41) + "\tx\n", "超過 320"},
		{"孤兒 key", head + "Talk to Janet\t測試\tx\n", "孤兒"},
		{"欄名", "k\tt\ts\nTalk to Gwen\t測試\tx\n", "欄名"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rep := Lint([]byte(c.tsv), wide, labl)
			if rep.OK() || !strings.Contains(strings.Join(rep.Errors, "\n"), c.want) {
				t.Fatalf("要含 %q 的失敗：%+v", c.want, rep.Errors)
			}
		})
	}
	// 40 字（2 倍寬 320）剛好可以
	ok40 := head + "Talk to Gwen\t" + strings.Repeat("測", 40) + "\tx\n"
	if rep := Lint([]byte(ok40), wide, labl); !rep.OK() {
		t.Fatalf("40 字應通過：%+v", rep.Errors)
	}
}
