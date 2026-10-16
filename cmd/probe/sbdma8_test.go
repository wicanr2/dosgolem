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

// docs/spec/444-legacy-sb16-dma8-irq7 §3 第 2 項（probe 部分）。

func TestSBDMA8FlagsDefaultOff(t *testing.T) {
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	on, log := sbDMA8Flags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatal(err)
	}
	if *on || *log != "" {
		t.Error("-sb-dma8、-sb-dma-log 預設應關閉")
	}
	if err := fs.Parse([]string{"-sb-dma8", "-sb-dma-log", "x.tsv"}); err != nil || !*on || *log != "x.tsv" {
		t.Errorf("旗標應可開啟：%v", err)
	}
}

func TestConfigureSBDMA8(t *testing.T) {
	m := machine.New()
	if err := configureSBDMA8(m, false, true, ""); err == nil || m.SBDMA8() {
		t.Error("單給 -sb-dma8（沒有 -sound-blaster）應回錯誤（probe 以 die 結束）")
	}
	m = machine.New()
	m.SetSoundBlaster(true)
	if err := configureSBDMA8(m, true, false, "x.tsv"); err == nil {
		t.Error("-sb-dma-log 沒有 -sb-dma8 應回錯誤")
	}
	if err := configureSBDMA8(m, true, true, "x.tsv"); err != nil || !m.SBDMA8() {
		t.Errorf("-sound-blaster -sb-dma8 應開啟：%v", err)
	}
	m = machine.New()
	if err := configureSBDMA8(m, false, false, ""); err != nil || m.SBDMA8() {
		t.Errorf("都不給：不開啟、不回錯誤：%v", err)
	}
}

func TestSBDMALogTSV(t *testing.T) {
	var b bytes.Buffer
	if err := writeSBDMAHeader(&b); err != nil {
		t.Fatal(err)
	}
	if err := writeSBDMARow(&b, machine.SBDMAEvent{Kind: "silence-like", Format: "silence"}); err != nil {
		t.Fatal(err)
	}
	if err := writeSBDMARow(&b, machine.SBDMAEvent{Kind: "start", Format: "pcm8", SHA256: strings.Repeat("ab", 32)}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")
	want := []string{"Kind", "Step", "Cycles", "CPUMode", "CS", "IP", "Command", "Format", "Length", "Samples",
		"RateNum", "RateDen", "TimeConstant", "TCKnown", "SpeakerOn", "IRQ0Every", "PITDiv", "Clock",
		"Channel", "DMAMode", "Page", "Addr", "Count", "Phys", "Due", "Transferred", "SHA256"}
	if lines[0] != strings.Join(want, "\t") {
		t.Errorf("第一列應為 §2.8 的欄名：%q", lines[0])
	}
	for i, l := range lines {
		if n := len(strings.Split(l, "\t")); n != len(want) {
			t.Errorf("第 %d 列應有 %d 欄（SHA256 為空時尾端 tab 仍在）：%d", i, len(want), n)
		}
	}
}

// §3 第 14 項經 internal/state（probe 的 -load-state 路徑）：版本 4 的第二個值落在機器段內。
func synthSBState(t *testing.T, sb8 bool) string {
	t.Helper()
	m := machine.New()
	m.SetSoundBlaster(true)
	if err := configureSBDMA8(m, true, sb8, ""); err != nil {
		t.Fatal(err)
	}
	if err := m.LoadCOM([]byte{0xB4, 0x4C, 0xCD, 0x21}); err != nil {
		t.Fatal(err)
	}
	d := dos.New(m, "/anchor")
	d.Install()
	p := filepath.Join(t.TempDir(), "s.gz")
	if err := state.Save(p, m, d); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSBDMA8AgainstStateFiles(t *testing.T) {
	on, off := synthSBState(t, true), synthSBState(t, false)
	for _, tc := range []struct {
		name     string
		path     string
		sb8, bad bool
	}{
		{"選項開的狀態檔、給 -sb-dma8", on, true, false},
		{"選項開的狀態檔、漏給 -sb-dma8", on, false, true},
		{"選項關的狀態檔、多給 -sb-dma8", off, true, true},
		{"選項關的狀態檔、不給", off, false, false},
	} {
		m := machine.New()
		m.SetSoundBlaster(true)
		if err := configureSBDMA8(m, true, tc.sb8, ""); err != nil {
			t.Fatal(err)
		}
		d := dos.New(m, "/anchor")
		d.Install()
		err := state.Load(tc.path, m, d)
		if tc.bad != (err != nil) {
			t.Errorf("%s：錯誤 %v，預期失敗＝%t", tc.name, err, tc.bad)
		}
	}
}
