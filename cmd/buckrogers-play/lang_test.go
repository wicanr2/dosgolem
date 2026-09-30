package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/apps/buckrogers"
	"github.com/wicanr2/dosgolem/xlate"
)

func enabledOnly(codes ...string) func(string) bool {
	return func(c string) bool {
		for _, x := range codes {
			if x == c {
				return true
			}
		}
		return false
	}
}

// Buck 規格 040 §3.4：-lang ＞ 設定檔 ＞ zh-TW；已知但未啟用、設定檔不認得都退 zh-TW 並回錯誤。
func TestStartLangPriority(t *testing.T) {
	on := enabledOnly("zh-TW", "en")
	cases := []struct {
		flag, saved, want string
		warn              bool
	}{
		{"", "", "zh-TW", false},
		{"", "en", "en", false},
		{"zh-TW", "en", "zh-TW", false},
		{"en", "zh-TW", "en", false},
		{"ja", "en", "zh-TW", true}, // -lang 已知但未啟用：不退回設定檔
		{"", "ko", "zh-TW", true},
		{"", "xx", "zh-TW", true},
		{"", "zz", "zh-TW", true}, // 測試語言不在前端
	}
	for _, c := range cases {
		got, err := startLang(c.flag, c.saved, on)
		if got != c.want || (err != nil) != c.warn {
			t.Fatalf("%+v → %q %v", c, got, err)
		}
	}
}

func TestFrontendKnownLang(t *testing.T) {
	for _, c := range []string{"zh-TW", "zh-CN", "en", "ja", "ko"} {
		if !frontendKnownLang(c) {
			t.Fatal(c)
		}
	}
	for _, c := range []string{"", "zz", "zh-tw", "EN", "fr"} {
		if frontendKnownLang(c) {
			t.Fatal(c)
		}
	}
}

func TestSettingsReadWrite(t *testing.T) {
	d := filepath.Join(t.TempDir(), "data")
	if l, err := readSettingsLang(d); l != "" || err != nil {
		t.Fatalf("不存在 %q %v", l, err)
	}
	if err := writeSettingsLang(d, "en"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(d, settingsName))
	if string(b) != "{\"lang\":\"en\"}\n" {
		t.Fatalf("%q", b)
	}
	if l, err := readSettingsLang(d); l != "en" || err != nil {
		t.Fatalf("%q %v", l, err)
	}
	if err := writeSettingsLang(d, "zh-TW"); err != nil {
		t.Fatal(err)
	}
	if l, _ := readSettingsLang(d); l != "zh-TW" {
		t.Fatal(l)
	}
	entries, _ := os.ReadDir(d)
	if len(entries) != 1 {
		t.Fatalf("留下暫存檔 %v", entries)
	}
}

// 讀寫失敗不致命：回錯誤，由呼叫端記下後用預設。
func TestSettingsFailures(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, settingsName), []byte("{lang"), 0o644)
	if l, err := readSettingsLang(d); l != "" || err == nil {
		t.Fatalf("壞 JSON %q %v", l, err)
	}
	got, err := startLang("", "", enabledOnly("zh-TW"))
	if got != "zh-TW" || err != nil {
		t.Fatal(got, err)
	}
	// 設定檔是目錄：讀失敗。
	d2 := t.TempDir()
	os.Mkdir(filepath.Join(d2, settingsName), 0o755)
	if _, err := readSettingsLang(d2); err == nil {
		t.Fatal("目錄應讀失敗")
	}
	// 資料目錄的位置是一般檔：寫失敗。
	f := filepath.Join(t.TempDir(), "file")
	os.WriteFile(f, nil, 0o644)
	if err := writeSettingsLang(f, "en"); err == nil {
		t.Fatal("應寫失敗")
	}
	if err := writeSettingsLang(d2, "en"); err == nil {
		t.Fatal("目標是目錄應寫失敗")
	}
	entries, _ := os.ReadDir(d2)
	if len(entries) != 1 {
		t.Fatalf("失敗後留下暫存檔 %v", entries)
	}
}

// 自動模式（-frames>0）與開發模式（-save）不讀也不寫。
func TestSettingsModes(t *testing.T) {
	d := t.TempDir()
	if err := writeSettingsLang(d, "en"); err != nil {
		t.Fatal(err)
	}
	for _, s := range []langSettings{newLangSettings(100, "", d), newLangSettings(0, "/tmp/save", d), newLangSettings(5, "/x", d)} {
		if l, err := s.read(); l != "" || err != nil {
			t.Fatalf("不應讀 %q %v", l, err)
		}
		if err := s.write("zh-TW"); err != nil {
			t.Fatal(err)
		}
	}
	if l, _ := readSettingsLang(d); l != "en" {
		t.Fatalf("不應寫：%q", l)
	}
	s := newLangSettings(0, "", d)
	if l, err := s.read(); l != "en" || err != nil {
		t.Fatalf("互動模式應讀 %q %v", l, err)
	}
	if err := s.write("zh-TW"); err != nil {
		t.Fatal(err)
	}
	if l, _ := readSettingsLang(d); l != "zh-TW" {
		t.Fatalf("互動模式應寫：%q", l)
	}
}

func testUI() langUI {
	return langUI{text: map[string]string{
		"lang.zh-TW": "繁體中文", "lang.zh-CN": "簡體中文", "lang.en": "英文（原版）", "lang.ja": "日文", "lang.ko": "韓文",
		langOffFiles: "缺語言檔", langOffFont: "缺字型", langOffLoad: "載入失敗",
	}}
}

func TestHelpLangLines(t *testing.T) {
	ui := testUI()
	lines := []string{"操作說明", "目前語言：{lang}", "未啟用：{off}"}
	langs := []buckrogers.LangStatus{
		{Code: "zh-TW", Enabled: true}, {Code: "zh-CN", Reason: "缺語言檔 skill-exit-confirmation.zh-CN.tsv"},
		{Code: "en", Enabled: true}, {Code: "ja", Reason: "缺語言檔 x"}, {Code: "ko", Reason: "缺字型"},
	}
	got := ui.helpLines(lines, "en", langs)
	want := []string{"操作說明", "目前語言：英文（原版）", "未啟用：簡體中文、日文（缺語言檔）；韓文（缺字型）"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("%q", got)
	}
	// 全部啟用時不顯示未啟用列。
	all := []buckrogers.LangStatus{{Code: "zh-TW", Enabled: true}, {Code: "en", Enabled: true}}
	got = ui.helpLines(lines, "zh-TW", all)
	if len(got) != 2 || got[1] != "目前語言：繁體中文" {
		t.Fatalf("%q", got)
	}
	// 載入錯誤歸成「載入失敗」；過長截斷到 38 格。
	many := []buckrogers.LangStatus{{Code: "zh-CN", Reason: "buckrogers: x"}, {Code: "ja", Reason: "字型：壞"}, {Code: "ko", Reason: "缺語言檔 y"}}
	got = ui.helpLines([]string{"未啟用：{off}{off}"}, "zh-TW", many)
	if len([]rune(got[0])) != helpColumns || !strings.HasPrefix(got[0], "未啟用：簡體中文（載入失敗）；日文（缺字型）；韓文（缺語言檔）") {
		t.Fatalf("%q", got)
	}
}

func TestLoadLangUI(t *testing.T) {
	d := t.TempDir()
	body := "key\ttranslation\tsource\nhelp.01\t說明\tfrontend-help\n" +
		"lang.zh-TW\t繁體中文\tfrontend-help\nlang.zh-CN\t簡體中文\tfrontend-help\nlang.en\t英文\tfrontend-help\n" +
		"lang.ja\t日文\tfrontend-help\nlang.ko\t韓文\tfrontend-help\nlang.off.files\t缺檔\tfrontend-help\n" +
		"lang.off.font\t缺字\tfrontend-help\nlang.off.load\t失敗\tfrontend-help\n"
	os.WriteFile(filepath.Join(d, "host-ui.zh-TW.tsv"), []byte(body), 0o644)
	font := &xlate.Font{Glyphs: map[rune][]byte{}}
	for _, r := range "繁體中文簡英日缺檔字失敗" {
		font.Glyphs[r] = nil
	}
	ui, warn, err := loadLangUI(d, font)
	if err != nil {
		t.Fatal(err)
	}
	// 「韓」缺字模：改顯示代碼，並回報。
	if ui.name("ko") != "ko" || ui.name("ja") != "日文" || len(warn) != 1 || !strings.Contains(warn[0], "lang.ko") {
		t.Fatalf("%q %q %v", ui.name("ko"), ui.name("ja"), warn)
	}
	os.WriteFile(filepath.Join(d, "host-ui.zh-TW.tsv"), []byte(strings.Replace(body, "lang.off.load\t失敗\tfrontend-help\n", "", 1)), 0o644)
	if _, _, err := loadLangUI(d, font); err == nil || !strings.Contains(err.Error(), "lang.off.load") {
		t.Fatalf("缺列應失敗：%v", err)
	}
	os.WriteFile(filepath.Join(d, "host-ui.zh-TW.tsv"), []byte(strings.Replace(body, "日文\tfrontend-help", "日文\truntime", 1)), 0o644)
	if _, _, err := loadLangUI(d, font); err == nil {
		t.Fatal("source 不符應失敗")
	}
}

// 正式 host-ui 譯文（BUCKROGERS_CHT_ROOT 指到 Buck repo 時）：每個 F4 語言都有名稱，
// 說明頁代入後每列不超過 38 格、列數不超過 15。
func TestRepoHostUILangRows(t *testing.T) {
	root := os.Getenv("BUCKROGERS_CHT_ROOT")
	if root == "" {
		t.Skip("BUCKROGERS_CHT_ROOT 未設定")
	}
	text := filepath.Join(root, "text")
	ui, _, err := loadLangUI(text, nil)
	if err != nil {
		t.Fatal(err)
	}
	font := &xlate.Font{Glyphs: map[rune][]byte{}}
	for r := rune(0); r < 0x10000; r++ {
		font.Glyphs[r] = nil
	}
	lines, err := buckrogers.LoadHelp(text, font)
	if err != nil {
		t.Fatal(err)
	}
	langs := []buckrogers.LangStatus{{Code: "zh-TW", Enabled: true}, {Code: "zh-CN", Reason: "缺語言檔 a"},
		{Code: "en", Enabled: true}, {Code: "ja", Reason: "缺語言檔 b"}, {Code: "ko", Reason: "缺語言檔 c"}}
	for _, cur := range []string{"zh-TW", "en"} {
		got := ui.helpLines(lines, cur, langs)
		if len(got) > 15 {
			t.Fatalf("%d 列", len(got))
		}
		joined := strings.Join(got, "\n")
		if strings.Contains(joined, "{") || !strings.Contains(joined, "目前語言："+ui.name(cur)) || strings.Contains(joined, "F4 到") {
			t.Fatalf("%s", joined)
		}
		for _, l := range got {
			if n := len([]rune(l)); n > helpColumns {
				t.Fatalf("%q %d 格", l, n)
			}
		}
	}
}

// 不認得的 -lang 是用法錯誤（結束碼 2），在載入任何資料之前。
func TestUnknownLangFlagIsUsageError(t *testing.T) {
	if os.Getenv("BUCKROGERS_PLAY_MAIN") == "1" {
		os.Args = []string{"buckrogers-play", "-lang", "xx"}
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestUnknownLangFlagIsUsageError$")
	cmd.Env = append(os.Environ(), "BUCKROGERS_PLAY_MAIN=1", "BUCKROGERS_NO_DIALOG=1", "XDG_DATA_HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 2 || !strings.Contains(string(out), "-lang 不認得的語言代碼") {
		t.Fatalf("%v\n%s", err, out)
	}
}
