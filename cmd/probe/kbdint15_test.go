package main

import (
	"bytes"
	"flag"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/internal/state"
)

// docs/spec/447-bios-int09-int15-keyboard-intercept §3 第 4 項：-kbd-int15。

func TestKbdInt15Flag(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"-kbd-int15"}, true},
	} {
		fs := flag.NewFlagSet("probe", flag.ContinueOnError)
		on := kbdInt15Flag(fs)
		if err := fs.Parse(tc.args); err != nil {
			t.Fatal(err)
		}
		if *on != tc.want {
			t.Errorf("%v：-kbd-int15＝%v，預期 %v", tc.args, *on, tc.want)
		}
		m := machine.New()
		if m.KbdIntercept {
			t.Fatal("machine.New 之後 KbdIntercept 應預設為 false")
		}
		if err := configureKbdInt15(m, *on); err != nil {
			t.Fatal(err)
		}
		if m.KbdIntercept != tc.want {
			t.Errorf("%v：KbdIntercept＝%v，預期 %v", tc.args, m.KbdIntercept, tc.want)
		}
	}
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	kbdInt15Flag(fs)
	if u := fs.Lookup("kbd-int15").Usage; !strings.Contains(u, "447") {
		t.Errorf("說明文字應引用規格 447：%q", u)
	}
}

func TestKbdInt15RejectsHardwareKeyFeeds(t *testing.T) {
	for _, tc := range []struct {
		on           bool
		keys, keysAt string
		bad          bool
	}{
		{true, "abc", "", true},
		{true, "", "100:a", true},
		{true, "", "", false},
		{false, "abc", "100:a", false},
	} {
		if err := checkKbdInt15Keys(tc.on, tc.keys, tc.keysAt); (err != nil) != tc.bad {
			t.Errorf("%+v：錯誤 %v，預期失敗＝%t", tc, err, tc.bad)
		}
	}
}

func synthKbdState(t *testing.T, on bool) string {
	t.Helper()
	m := machine.New()
	if err := m.LoadCOM([]byte{0xB4, 0x4C, 0xCD, 0x21}); err != nil {
		t.Fatal(err)
	}
	d := dos.New(m, "/anchor")
	d.Install()
	if on {
		if err := m.EnableKbdIntercept(); err != nil {
			t.Fatal(err)
		}
	}
	p := filepath.Join(t.TempDir(), "s.gz")
	if err := state.Save(p, m, d); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestKbdInt15AgainstStateFiles(t *testing.T) {
	on, off := synthKbdState(t, true), synthKbdState(t, false)
	for _, tc := range []struct {
		name    string
		path    string
		flagOn  bool
		bad     bool
		enabled bool
	}{
		{"選項開的狀態檔、給 -kbd-int15", on, true, false, true},
		{"選項開的狀態檔、漏給 -kbd-int15", on, false, true, false},
		{"選項關的狀態檔、給 -kbd-int15", off, true, false, true},
		{"選項關的狀態檔、不給", off, false, false, false},
	} {
		m := machine.New()
		d := dos.New(m, "/anchor")
		d.Install()
		if err := state.Load(tc.path, m, d); err != nil {
			t.Fatal(err)
		}
		if m.KbdIntercept {
			t.Fatalf("%s：KbdIntercept 不進狀態檔，載入後應為 false", tc.name)
		}
		err := configureKbdInt15(m, tc.flagOn)
		if tc.bad != (err != nil) || m.KbdIntercept != tc.enabled {
			t.Errorf("%s：錯誤 %v、KbdIntercept=%v，預期失敗＝%t、%v", tc.name, err, m.KbdIntercept, tc.bad, tc.enabled)
		}
	}
	// 偵測依據的 3 bytes 是 §2.2 的 E9 91 04。
	if machine.KbdInterceptStub9() != [3]byte{0xE9, 0x91, 0x04} {
		t.Errorf("KbdInterceptStub9＝% X，應 E9 91 04", machine.KbdInterceptStub9())
	}
}

// §2.6：開啟時另印一行，數字全為 0 也印。
func TestReportKbdInt15(t *testing.T) {
	m := machine.New()
	d := dos.New(m, "/anchor")
	var b bytes.Buffer
	reportKbdInt15(&b, m, d)
	for _, w := range []string{"KeyIRQs=0", "KeyStalls=0", "TimedKeysPending=0", "KbdIntercepts=0", "KbdTranslates=0"} {
		if !strings.Contains(b.String(), w) {
			t.Errorf("報告缺 %q：%q", w, b.String())
		}
	}
}
