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

// docs/spec/445-legacy-sb16-dma8-pause-resume §3 第 2.18 項（probe 部分）：-sb-dma-log 的說明文字含 pause、resume；
// reportSBDMA8 印出暫停中、暫停次數、恢復次數。
func TestSBDMA8PauseReport(t *testing.T) {
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	sbDMA8Flags(fs)
	if u := fs.Lookup("sb-dma-log").Usage; !strings.Contains(u, "pause") || !strings.Contains(u, "resume") {
		t.Errorf("-sb-dma-log 的說明文字應列出 pause、resume：%s", u)
	}
	m := machine.New()
	m.SetSoundBlaster(true)
	if err := configureSBDMA8(m, true, true, ""); err != nil {
		t.Fatal(err)
	}
	if err := m.LoadCOM([]byte{0xEB, 0xFE}); err != nil {
		t.Fatal(err)
	}
	for _, w := range [][2]uint16{{0x0A, 5}, {0x83, 0}, {0x0C, 0}, {0x02, 0}, {0x02, 0x10}, {0x03, 3}, {0x03, 0}, {0x0B, 0x49}, {0x0A, 1},
		{0x22C, 0x40}, {0x22C, 0xA6}, {0x22C, 0x14}, {0x22C, 3}, {0x22C, 0}, {0x22C, 0xD0}, {0x22C, 0xD0}} {
		m.Out8(w[0], uint8(w[1]))
	}
	if err := m.SBDMA8Err(); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	reportSBDMA8(&b, m)
	for _, want := range []string{"暫停中=true", "暫停 1 次", "恢復 0 次"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("報告應含 %q：%s", want, b.String())
		}
	}
	m.Out8(0x22C, 0xD4)
	b.Reset()
	reportSBDMA8(&b, m)
	for _, want := range []string{"暫停中=false", "暫停 1 次", "恢復 1 次"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("恢復後報告應含 %q：%s", want, b.String())
		}
	}
}

// docs/spec/446-legacy-sb16-idle-control-and-busy-cycle §3 第 7 項（probe 部分）：-sb-dma-log 的說明文字含 control-idle；
// reportSBDMA8 印出 idle 控制次數與 22Ch、22Eh 的讀取次數。
func TestSBDMA8IdleReport(t *testing.T) {
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	sbDMA8Flags(fs)
	if u := fs.Lookup("sb-dma-log").Usage; !strings.Contains(u, "control-idle") {
		t.Errorf("-sb-dma-log 的說明文字應列出 control-idle：%s", u)
	}
	m := machine.New()
	m.SetSoundBlaster(true)
	if err := configureSBDMA8(m, true, true, ""); err != nil {
		t.Fatal(err)
	}
	if err := m.LoadCOM([]byte{0xEB, 0xFE}); err != nil {
		t.Fatal(err)
	}
	// 機器層測試以 In8 代替 CPU 的 in（446 §2.4「讀取的來源」）。
	for _, w := range [][2]uint16{{0x22C, 0xD0}, {0x22C, 0xD4}, {0x22C, 0x40}, {0x22C, 0xA6}, {0x22C, 0x80}, {0x22C, 3}, {0x22C, 0}, {0x22C, 0xD0}} {
		m.Out8(w[0], uint8(w[1]))
	}
	for i := 0; i < 3; i++ {
		m.In8(0x22C)
	}
	m.In8(0x22E)
	if err := m.SBDMA8Err(); err != nil {
		t.Fatal(err)
	}
	if st := m.SBDMA8Stats(); st.IdleControls != 3 || st.Pauses != 0 {
		t.Errorf("IdleControls 應為 3：%+v", st)
	}
	var b bytes.Buffer
	reportSBDMA8(&b, m)
	for _, want := range []string{"D0h／D4h 3 次", "22Ch 讀取 3 次", "22Eh 讀取 1 次", "docs/spec/446"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("報告應含 %q：%s", want, b.String())
		}
	}
}
