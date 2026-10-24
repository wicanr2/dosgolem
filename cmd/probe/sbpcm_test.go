package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

func pcmOptions(args ...string) sbPCMOptions {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	o := sbPCMFlags(fs)
	if err := fs.Parse(args); err != nil {
		panic(err)
	}
	return o
}

func TestSBPCMDumpWAVRawAndReport(t *testing.T) {
	for _, withLog := range []bool{false, true} {
		t.Run(fmt.Sprint(withLog), func(t *testing.T) {
			m := machine.New()
			m.SetSoundBlaster(true)
			if err := m.SetSBDMA8(true); err != nil {
				t.Fatal(err)
			}
			if err := m.LoadCOM([]byte{0xeb, 0xfe}); err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			wav := filepath.Join(dir, "test.wav")
			var log bytes.Buffer
			if withLog {
				m.OnSBDMA = func(e machine.SBDMAEvent) { fmt.Fprintln(&log, e.Kind) }
			}
			d, err := openSBPCM(m, pcmOptions("-dump-sbpcm-wav", wav, "-dump-sbpcm-raw", dir, "-sbpcm-sps", "240000", "-sbpcm-linear-check"))
			if err != nil {
				t.Fatal(err)
			}
			data := []byte{0, 64, 128, 255}
			copy(m.Mem[0x12000:], data)
			e := machine.SBDMAEvent{Kind: "start", Step: 1, Page: 1, Addr: 0x2000}
			if withLog {
				m.OnSBDMA(e)
			} else {
				m.SBDMALog = append(m.SBDMALog, e)
			}
			hash := fmt.Sprintf("%x", sha256.Sum256(data))
			d.apply(machine.SBPCMEvent{Kind: "start", Step: 1, Start: 1, Due: 11, Clock: "steps", Command: 0x14, Length: 4, SHA256: hash, Data: data})
			d.flush(5, true)
			d.apply(machine.SBPCMEvent{Kind: "complete", Step: 11, Clock: "steps"})
			var report bytes.Buffer
			if err := d.close(16, &report); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(wav)
			if err != nil {
				t.Fatal(err)
			}
			// 樣本 0: start 前向下取整，取 -32768；樣本 1 的 k=1、
			// f=39321，(-64*26215+0*39321)*256>>16 = -6554。
			want := []int16{-32768, -6554, 0}
			if len(b) != 44+2*len(want) || binary.LittleEndian.Uint32(b[40:]) != 6 || binary.LittleEndian.Uint32(b[4:]) != 42 {
				t.Fatalf("header/len %v %d", b[:44], len(b))
			}
			for i, v := range want {
				if got := int16(binary.LittleEndian.Uint16(b[44+2*i:])); got != v {
					t.Fatalf("sample %d: %d want %d", i, got, v)
				}
			}
			raw, err := os.ReadFile(filepath.Join(dir, "1-"+hash[:16]+".u8"))
			if err != nil || !bytes.Equal(raw, data) {
				t.Fatalf("raw: %v %v", raw, err)
			}
			for _, text := range []string{"firstSample=0", "endStep=16", "SBPCMMutated=0", "linearMismatch=0"} {
				if !strings.Contains(report.String(), text) {
					t.Fatal(report.String())
				}
			}
			if withLog && log.String() != "start\n" {
				t.Fatal("既有 SBDMA 回呼被改變")
			}
		})
	}
}

func TestSBPCMFlagsAndValidation(t *testing.T) {
	o := pcmOptions()
	if *o.sps != 11550000 || *o.wav != "" || *o.raw != "" || *o.linear {
		t.Fatal("defaults")
	}
	m := machine.New()
	if d, err := openSBPCM(m, o); err != nil || d != nil {
		t.Fatal("預設不可安裝觀察器")
	}
	if _, err := openSBPCM(m, pcmOptions("-sbpcm-linear-check")); err == nil {
		t.Fatal("缺 DMA8 應失敗")
	}
	m.SetSoundBlaster(true)
	if err := m.SetSBDMA8(true); err != nil {
		t.Fatal(err)
	}
	if _, err := openSBPCM(m, pcmOptions("-sbpcm-linear-check", "-sbpcm-sps", "0")); err == nil {
		t.Fatal("SPS=0 應失敗")
	}
}
