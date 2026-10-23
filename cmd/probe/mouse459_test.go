package main

import (
	"bytes"
	"flag"
	"fmt"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// docs/spec/459 §3.8：隱藏的診斷旗標 -diag-mouse-irq12-direct 與報告欄位。
func TestMouseIRQ12DirectFlag(t *testing.T) {
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	var out bytes.Buffer
	fs.SetOutput(&out)
	other := fs.Bool("other", false, "別的旗標")
	v := mouseIRQ12DirectFlag(fs)
	hideFlags(fs, mouseIRQ12DirectName)
	if *v || *other {
		t.Fatal("預設應為 false")
	}
	if err := fs.Parse([]string{"-" + mouseIRQ12DirectName}); err != nil || !*v {
		t.Fatalf("應能解析：%v %t", err, *v)
	}
	fs.Usage()
	if strings.Contains(out.String(), mouseIRQ12DirectName) || !strings.Contains(out.String(), "-other") {
		t.Errorf("-h 不應列出隱藏旗標、其他旗標照列：%q", out.String())
	}

	m := machine.New()
	if err := m.SetExtendedMemoryKB(4096); err != nil {
		t.Fatal(err)
	}
	if err := m.SetProtectedModeProfile(true); err != nil {
		t.Fatal(err)
	}
	if err := configureMouseIRQ12Direct(m, true); err != nil || !m.MouseIRQ12Direct() {
		t.Fatalf("configureMouseIRQ12Direct：%v %t", err, m.MouseIRQ12Direct())
	}
	if err := m.LoadCOM([]byte{0xEB, 0xFE}); err != nil {
		t.Fatal(err)
	}
	if err := configureMouseIRQ12Direct(m, true); err == nil {
		t.Error("載入程式後設定應回錯誤")
	}
	var rep bytes.Buffer
	reportPIC459(&rep, m)
	l, _ := m.StuckLimit()
	for _, want := range []string{"負對照，非正式結果", fmt.Sprintf("L=%d（10 × IRQ0Every", l), "從片 IMR=EFh"} {
		if !strings.Contains(rep.String(), want) {
			t.Errorf("報告應含 %q：%s", want, rep.String())
		}
	}
}
