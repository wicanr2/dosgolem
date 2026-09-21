package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/internal/state"
)

func TestMenuCatalogFlagsArePaired(t *testing.T) {
	for _, tc := range []struct {
		events, translations string
		valid                bool
	}{
		{"", "", true},
		{"events.tsv", "translations.tsv", true},
		{"events.tsv", "", false},
		{"", "translations.tsv", false},
	} {
		if got := validateMenuCatalogFlags(tc.events, tc.translations) == nil; got != tc.valid {
			t.Fatalf("events=%q translations=%q valid=%v，要 %v", tc.events, tc.translations, got, tc.valid)
		}
	}
}

func TestAllCatalogFlagsArePaired(t *testing.T) {
	tests := []struct {
		menuEvents, menuTranslations, genderEvents, genderTranslations string
		classEvents, classTranslations                                 string
		rosterEvents, rosterTranslations                               string
		characterSheetEvents, characterSheetTranslations               string
		valid                                                          bool
	}{
		{"", "", "", "", "", "", "", "", "", "", true},
		{"m.tsv", "mt.tsv", "", "", "", "", "", "", "", "", true},
		{"", "", "g.tsv", "gt.tsv", "", "", "", "", "", "", true},
		{"m.tsv", "mt.tsv", "g.tsv", "gt.tsv", "c.tsv", "ct.tsv", "r.tsv", "rt.tsv", "s.tsv", "st.tsv", true},
		{"", "", "", "", "c.tsv", "ct.tsv", "", "", "", "", true},
		{"", "", "", "", "", "", "r.tsv", "rt.tsv", "", "", true},
		{"", "", "", "", "", "", "", "", "s.tsv", "st.tsv", true},
		{"m.tsv", "", "", "", "", "", "", "", "", "", false},
		{"", "", "g.tsv", "", "", "", "", "", "", "", false},
		{"", "", "", "gt.tsv", "", "", "", "", "", "", false},
		{"", "", "", "", "c.tsv", "", "", "", "", "", false},
		{"", "", "", "", "", "ct.tsv", "", "", "", "", false},
		{"", "", "", "", "", "", "r.tsv", "", "", "", false},
		{"", "", "", "", "", "", "", "rt.tsv", "", "", false},
		{"", "", "", "", "", "", "", "", "s.tsv", "", false},
		{"", "", "", "", "", "", "", "", "", "st.tsv", false},
	}
	for _, tc := range tests {
		got := validateCatalogFlags(tc.menuEvents, tc.menuTranslations, tc.genderEvents, tc.genderTranslations,
			tc.classEvents, tc.classTranslations, tc.rosterEvents, tc.rosterTranslations,
			tc.characterSheetEvents, tc.characterSheetTranslations) == nil
		if got != tc.valid {
			t.Fatalf("flags %#v valid=%v，要 %v", tc, got, tc.valid)
		}
	}
}

func TestNamePromptCatalogFlagsArePaired(t *testing.T) {
	for _, tc := range []struct {
		name, events, translations string
		wantOK                     bool
	}{
		{"皆省略", "", "", true},
		{"皆提供", "events", "translations", true},
		{"缺譯文", "events", "", false},
		{"缺事件", "", "translations", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateNamePromptCatalogFlags(tc.events, tc.translations) == nil; got != tc.wantOK {
				t.Fatalf("有效性 = %v，要 %v", got, tc.wantOK)
			}
		})
	}
}

func TestCareerSkillCatalogFlagsArePaired(t *testing.T) {
	for _, tc := range []struct {
		events, translations string
		wantOK               bool
	}{
		{"", "", true}, {"events", "translations", true},
		{"events", "", false}, {"", "translations", false},
	} {
		if got := validateCareerSkillCatalogFlags(tc.events, tc.translations) == nil; got != tc.wantOK {
			t.Fatalf("events=%q translations=%q valid=%v，要 %v", tc.events, tc.translations, got, tc.wantOK)
		}
	}
}

func TestValidateOverlayDrawAllowsEmptyTerminalFrame(t *testing.T) {
	for _, tc := range []struct {
		name    string
		active  []string
		missing []rune
		drew    bool
		wantOK  bool
	}{
		{"有 stamp 且有繪製", []string{"prompt"}, nil, true, true},
		{"清除後空終態", nil, nil, false, true},
		{"有 stamp 卻未繪製", []string{"prompt"}, nil, false, false},
		{"無 stamp 卻聲稱繪製", nil, nil, true, false},
		{"缺字", []string{"prompt"}, []rune{'缺'}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateOverlayDraw(tc.active, tc.missing, tc.drew) == nil; got != tc.wantOK {
				t.Fatalf("有效性 = %v，要 %v", got, tc.wantOK)
			}
		})
	}
}

func TestOverlayFlagsRequireCompleteExplicitTwoOrThreeScale(t *testing.T) {
	valid := []struct {
		menuEvents, menuRects, genderEvents, genderRects   string
		classEvents, classRects, rosterEvents, rosterRects string
		characterSheetEvents, characterSheetRects          string
		namePromptEvents, namePromptRects                  string
		font, out                                          string
		scale                                              int
		want                                               bool
	}{
		{"", "", "", "", "", "", "", "", "", "", "", "", "", "", 0, true},
		{"m.e", "m.r", "", "", "", "", "r.e", "r.r", "", "", "", "", "f.bin", "o.rgba", 2, true},
		{"m.e", "m.r", "g.e", "g.r", "c.e", "c.r", "", "", "s.e", "s.r", "", "", "f.bin", "o.rgba", 3, true},
		{"", "", "", "", "", "", "", "", "s.e", "s.r", "", "", "f.bin", "o.rgba", 2, true},
		{"", "", "", "", "", "", "", "", "", "", "n.e", "n.r", "f.bin", "o.rgba", 2, true},
		{"m.e", "", "", "", "", "", "", "", "", "", "", "", "f.bin", "o.rgba", 2, false},
		{"", "m.r", "", "", "", "", "", "", "", "", "", "", "f.bin", "o.rgba", 2, false},
		{"m.e", "m.r", "g.e", "", "", "", "", "", "", "", "", "", "f.bin", "o.rgba", 2, false},
		{"", "", "", "", "", "", "", "", "s.e", "", "", "", "f.bin", "o.rgba", 2, false},
		{"", "", "", "", "", "", "", "", "", "s.r", "", "", "f.bin", "o.rgba", 2, false},
		{"", "", "", "", "", "", "", "", "", "", "n.e", "", "f.bin", "o.rgba", 2, false},
		{"m.e", "m.r", "", "", "", "", "", "", "", "", "", "", "f.bin", "", 2, false},
		{"m.e", "m.r", "", "", "", "", "", "", "", "", "", "", "f.bin", "o.rgba", 1, false},
		{"m.e", "m.r", "", "", "", "", "", "", "", "", "", "", "f.bin", "o.rgba", 4, false},
	}
	for _, tc := range valid {
		got := validateOverlayFlags(tc.menuEvents, tc.menuRects, tc.genderEvents, tc.genderRects,
			tc.classEvents, tc.classRects, tc.rosterEvents, tc.rosterRects,
			tc.characterSheetEvents, tc.characterSheetRects,
			tc.namePromptEvents, tc.namePromptRects,
			tc.font, tc.out, tc.scale) == nil
		if got != tc.want {
			t.Fatalf("flags=%#v valid=%v，要 %v", tc, got, tc.want)
		}
	}
}

func TestEmitReceiptWritesIdenticalBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.json")
	var stdout bytes.Buffer
	value := struct {
		Count int `json:"count"`
	}{Count: 14}
	if err := emitReceipt(&stdout, path, value); err != nil {
		t.Fatal(err)
	}
	file, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stdout.Bytes(), file) || string(file) != "{\"count\":14}\n" {
		t.Fatalf("stdout=%q file=%q", stdout.Bytes(), file)
	}
}

func TestEmitReceiptRejectsOutputDirectory(t *testing.T) {
	if err := emitReceipt(&bytes.Buffer{}, t.TempDir(), struct{}{}); err == nil {
		t.Fatal("receipt-out 指向目錄時必須失敗")
	}
}

func TestConfigureScratch(t *testing.T) {
	d := dos.New(machine.New(), t.TempDir())
	if err := configureScratch(d, ""); err != nil || d.Scratch != "" {
		t.Fatalf("空 scratch = %q, %v", d.Scratch, err)
	}
	dir := t.TempDir()
	if err := configureScratch(d, dir); err != nil || d.Scratch != dir {
		t.Fatalf("scratch = %q, %v，要 %q", d.Scratch, err, dir)
	}
	file := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := configureScratch(d, file); err == nil {
		t.Fatal("一般檔案不可作為 scratch")
	}
	if err := configureScratch(d, filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("不存在的 scratch 必須失敗")
	}
}

func TestUnimplementedReportIsOptionalAndSorted(t *testing.T) {
	d := dos.New(machine.New(), t.TempDir())
	d.Unimplemented[dos.Call{Int: 0x21, AH: 0x22, AL: 0x00}] = 1
	d.Unimplemented[dos.Call{Int: 0x16, AH: 0x99, AL: 0x01}] = 3
	if got := unimplementedReport(false, d); got != nil {
		t.Fatalf("旗標關閉時 report = %#v，要 nil", got)
	}
	got := unimplementedReport(true, d)
	want := d.UnimplementedReport()
	if len(got) != 2 || len(want) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("report = %#v，要 %#v", got, want)
	}
	if empty := unimplementedReport(true, dos.New(machine.New(), t.TempDir())); len(empty) != 0 {
		t.Fatalf("空集合 report = %#v，要空", empty)
	}
}

func TestSaveTerminalState(t *testing.T) {
	m := machine.New()
	d := dos.New(m, t.TempDir())
	d.Install()
	m.Steps = 12345
	m.Write8(0x23456, 0xA5)
	if err := saveTerminalState("", m, d); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "terminal.state")
	if err := saveTerminalState(path, m, d); err != nil {
		t.Fatal(err)
	}
	m2 := machine.New()
	d2 := dos.New(m2, t.TempDir())
	d2.Install()
	if err := state.Load(path, m2, d2); err != nil {
		t.Fatal(err)
	}
	if m2.Steps != m.Steps || m2.Read8(0x23456) != 0xA5 {
		t.Fatalf("回讀 state：steps=%d byte=%02X", m2.Steps, m2.Read8(0x23456))
	}
	if err := saveTerminalState(t.TempDir(), m, d); err == nil {
		t.Fatal("state-out 指向目錄時必須失敗")
	}
}

func TestParseScheduledBIOSKey(t *testing.T) {
	got, err := parseScheduledBIOSKey("100010000:50:00")
	if err != nil || got != (scheduledBIOSKey{100010000, 0x50, 0x00}) {
		t.Fatalf("parse = %#v, %v", got, err)
	}
	for _, value := range []string{
		"", "0:50:00", "1", "1:50", "1:50:00:00", "x:50:00",
		"1:5:00", "1:050:00", "1:GG:00", "1:5A:00", "1:50:0A",
	} {
		if _, err := parseScheduledBIOSKey(value); err == nil {
			t.Errorf("應拒絕 %q", value)
		}
	}
}

func TestMergeBIOSKeySchedule(t *testing.T) {
	keys, err := mergeBIOSKeySchedule(20, []scheduledBIOSKey{{30, 0x48, 0}, {10, 0x50, 0}}, 40)
	if err != nil {
		t.Fatal(err)
	}
	want := []scheduledBIOSKey{{10, 0x50, 0}, {20, 0x1C, 0x0D}, {30, 0x48, 0}}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("keys[%d] = %#v，要 %#v", i, keys[i], want[i])
		}
	}
	for _, tc := range []struct {
		enter uint64
		keys  []scheduledBIOSKey
		until uint64
	}{
		{10, []scheduledBIOSKey{{10, 0x50, 0}}, 20},
		{0, []scheduledBIOSKey{{20, 0x50, 0}}, 20},
	} {
		if _, err := mergeBIOSKeySchedule(tc.enter, tc.keys, tc.until); err == nil {
			t.Fatal("重複 step 或 until 邊界應拒絕")
		}
	}
}

func TestWriteIndexedScreen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "screen.bin")
	data := make([]byte, 320*200)
	data[12345] = 0x0F
	if err := writeIndexedScreen(path, data); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || len(got) != len(data) || got[12345] != 0x0F {
		t.Fatalf("screen len=%d err=%v", len(got), err)
	}
	if err := writeIndexedScreen(path, data[:len(data)-1]); err == nil {
		t.Fatal("短 framebuffer 應拒絕")
	}
}
