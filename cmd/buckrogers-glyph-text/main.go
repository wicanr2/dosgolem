// buckrogers-glyph-text 是本機診斷工具：從 savestate 執行，逐筆記錄
// 0763:026B glyph 呼叫（含字元）、026F:029C 清除呼叫與敘事窗區的 A000
// 寫入叢集。輸出含原版文字，只能寫到不入版控的工作目錄。
package main

import (
	"bufio"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/internal/state"
)

type key struct {
	step        uint64
	scan, ascii uint8
}

type keys []key

func (k *keys) String() string { return "" }
func (k *keys) Set(v string) error {
	p := strings.Split(v, ":")
	if len(p) != 3 {
		return fmt.Errorf("STEP:SCAN:ASCII")
	}
	s, err := strconv.ParseUint(p[0], 10, 64)
	if err != nil {
		return err
	}
	a, err := strconv.ParseUint(p[1], 16, 8)
	if err != nil {
		return err
	}
	b, err := strconv.ParseUint(p[2], 16, 8)
	if err != nil {
		return err
	}
	*k = append(*k, key{s, uint8(a), uint8(b)})
	return nil
}

func main() {
	statePath := flag.String("state", "", "savestate")
	until := flag.Uint64("until", 0, "絕對步數上限")
	enterFrom := flag.Uint64("enter-from", 0, "從此步數起週期送 Enter；0 表示不送")
	enterEvery := flag.Uint64("enter-every", 5_000_000, "週期 Enter 間隔")
	shotEvery := flag.Uint64("shot-every", 0, "每隔多少步存一張 320×200 PNG；0 表示不存")
	out := flag.String("out", "", "輸出 TSV（含原文，只放 ignored 目錄）")
	shotDir := flag.String("shot-dir", ".", "PNG 目錄")
	dump := flag.String("dump", "", "結束時傾印記憶體 SEG:OFF:LEN（hex）到 -out.bin")
	stateOut := flag.String("state-out", "", "結束時存 savestate（只供本機研究）")
	finalShot := flag.String("final-shot", "", "結束時存 PNG")
	memOut := flag.String("mem-out", "", "結束時傾印 1MB 實模式記憶體（含原版資料，只放 ignored 目錄）")
	var ks keys
	flag.Var(&ks, "key", "STEP:SCAN_HEX:ASCII_HEX，可重複")
	flag.Parse()
	if *statePath == "" || *until == 0 || *out == "" {
		fmt.Fprintln(os.Stderr, "需要 -state -until -out")
		os.Exit(2)
	}
	m := machine.New()
	d := dos.New(m, ".")
	d.Install()
	if err := state.Load(*statePath, m, d); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	f, err := os.Create(*out)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	w := bufio.NewWriter(f)
	defer func() { w.Flush(); f.Close() }()

	// A000 寫入叢集：同一 CS:IP 連續寫入合併為一列。
	var burstAt [2]uint16
	var burstFirst, burstLo, burstHi uint32
	var burstStep uint64
	var burstN int
	flush := func() {
		if burstN > 0 {
			fmt.Fprintf(w, "V\t%d\t%04X:%04X\t%d\t%d\t%d\t%d\n", burstStep, burstAt[0], burstAt[1], burstN, burstFirst, burstLo, burstHi)
		}
		burstN = 0
	}
	m.ObserveVideoWrites(func(v machine.VideoWrite) {
		if v.Offset < 136*320 || v.Offset >= 200*320 {
			return
		}
		if burstN > 0 && [2]uint16{v.CS, v.IP} == burstAt && v.Step-burstStep < 200000 {
			burstN++
			if v.Offset < burstLo {
				burstLo = v.Offset
			}
			if v.Offset > burstHi {
				burstHi = v.Offset
			}
			return
		}
		flush()
		burstAt, burstFirst, burstLo, burstHi, burstStep, burstN = [2]uint16{v.CS, v.IP}, v.Offset, v.Offset, v.Offset, v.Step, 1
	})
	var hret [4]uint16
	nextKey := 0
	nextEnter := *enterFrom
	nextShot := uint64(0)
	if *shotEvery != 0 {
		nextShot = (m.Steps / *shotEvery + 1) * *shotEvery
	}
	for m.Steps < *until && !d.Exited {
		for nextKey < len(ks) && m.Steps >= ks[nextKey].step {
			m.PushBIOSKey(ks[nextKey].scan, ks[nextKey].ascii)
			fmt.Fprintf(w, "K\t%d\t%02X\t%02X\n", m.Steps, ks[nextKey].scan, ks[nextKey].ascii)
			nextKey++
		}
		if *enterFrom != 0 && m.Steps >= nextEnter {
			m.PushBIOSKey(0x1c, 0x0d)
			fmt.Fprintf(w, "K\t%d\t1C\t0D\n", m.Steps)
			nextEnter += *enterEvery
		}
		if *shotEvery != 0 && m.Steps >= nextShot {
			shot(m, fmt.Sprintf("%s/s%d.png", *shotDir, nextShot))
			nextShot += *shotEvery
		}
		cs, ip := m.CPU.Seg[cpu.CS], m.CPU.IP
		ss, sp := m.CPU.Seg[cpu.SS], m.CPU.R[cpu.SP]
		if hret[0] != 0 && cs == hret[0] && ip == hret[1] && ss == hret[2] && sp == hret[3] {
			fmt.Fprintf(w, "R\t%d\t%04X:%04X\n", m.Steps, cs, ip)
			hret = [4]uint16{}
		}
		switch {
		case cs == 0x0763 && ip == 0x026B:
			flush()
			rs, ro := m.Read16(cpu.Addr(ss, sp+2)), m.Read16(cpu.Addr(ss, sp))
			var a [7]uint16
			for i := range a {
				a[i] = m.Read16(cpu.Addr(ss, sp+4+uint16(i)*2))
			}
			ch := byte(a[1])
			c := "?"
			if ch >= 0x20 && ch < 0x7f {
				c = string(rune(ch))
			} else {
				c = fmt.Sprintf("\\x%02X", ch)
			}
			fmt.Fprintf(w, "G\t%d\t%04X:%04X\t%d\t%d\t%d\t%d\t%d\t%d\t%s\n", m.Steps, rs, ro, a[0], a[2], a[3], a[4], a[5], a[6], c)
		case cs == 0x0763 && ip == 0x0424:
			flush()
			var a [6]uint16
			for i := range a {
				a[i] = m.Read16(cpu.Addr(ss, sp+4+uint16(i)*2))
			}
			base := cpu.Addr(a[1], a[0])
			n := int(m.Read8(base))
			b := make([]byte, n)
			for i := range b {
				c := m.Read8(base + 1 + uint32(i))
				if c < 0x20 || c >= 0x7f {
					c = '~'
				}
				b[i] = c
			}
			fmt.Fprintf(w, "D\t%d\t%04X:%04X\t%d\t%d\t%d\t%d\t%s\n", m.Steps, m.Read16(cpu.Addr(ss, sp+2)), m.Read16(cpu.Addr(ss, sp)), a[2], a[3], a[4], a[5], b)
		case cs == 0x0763 && ip == 0x056C:
			flush()
			var a [9]uint16
			for i := range a {
				a[i] = m.Read16(cpu.Addr(ss, sp+4+uint16(i)*2))
			}
			base := cpu.Addr(a[1], a[0])
			n := int(m.Read8(base))
			b := make([]byte, n)
			for i := range b {
				c := m.Read8(base + 1 + uint32(i))
				if c < 0x20 || c >= 0x7f {
					c = '~'
				}
				b[i] = c
			}
			ds := m.CPU.Seg[cpu.DS]
			fmt.Fprintf(w, "W\t%d\t%04X:%04X\tflag=%d\ta=%d,%d\tL%d T%d R%d B%d\tcur=%d,%d\t%s\n", m.Steps, m.Read16(cpu.Addr(ss, sp+2)), m.Read16(cpu.Addr(ss, sp)),
				uint8(a[2]), uint8(a[3]), uint8(a[4]), uint8(a[8]), uint8(a[7]), uint8(a[6]), uint8(a[5]),
				m.Read8(cpu.Addr(ds, 0x5f3e)), m.Read8(cpu.Addr(ds, 0x5f3f)), b)
		case cs == 0x37F1 && ip == 0x0243:
			flush()
			pb, sel := m.Read16(cpu.Addr(ss, sp+4)), m.Read16(cpu.Addr(ss, sp+6))
			hret = [4]uint16{m.Read16(cpu.Addr(ss, sp+2)), m.Read16(cpu.Addr(ss, sp)), ss, sp + 8}
			at := func(off int) uint32 { return cpu.Addr(ss, uint16(int(pb)+off)) }
			n := int(m.Read8(at(-0x213)))
			b := make([]byte, n)
			for i := range b {
				c := m.Read8(at(-0x200 + 1 + i))
				if c < 0x20 || c >= 0x7f {
					c = '~'
				}
				b[i] = c
			}
			var tab []string
			for i := 0; i < 12; i++ {
				tab = append(tab, fmt.Sprintf("%d-%d", m.Read8(at(-0x23e+2*i)), m.Read8(at(-0x23d+2*i))))
			}
			fmt.Fprintf(w, "H\t%d\tsel=%d\trow=%d\tcol=%d\tlen0=%d\t%s\t|%s|\n", m.Steps, sel, m.Read8(at(-0x24a)), m.Read8(at(-0x214)), m.Read8(at(-0x200)), strings.Join(tab, ","), b)
		case cs == 0x026F && ip == 0x029C:
			flush()
			var a [6]uint16
			for i := range a {
				a[i] = m.Read16(cpu.Addr(ss, sp+uint16(i)*2))
			}
			fmt.Fprintf(w, "C\t%d\t%04X:%04X\t%d\t%d\t%d\t%d\n", m.Steps, a[1], a[0], a[2], a[3], a[4], a[5])
		}
		if err := m.Step(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			break
		}
	}
	flush()
	if *memOut != "" {
		b := make([]byte, 0x100000)
		for i := range b {
			b[i] = m.Read8(uint32(i))
		}
		os.WriteFile(*memOut, b, 0o644)
		fmt.Fprintf(w, "DS\t%04X\n", m.CPU.Seg[cpu.DS])
	}
	if *dump != "" {
		var seg, off, n uint32
		fmt.Sscanf(*dump, "%x:%x:%x", &seg, &off, &n)
		b := make([]byte, n)
		for i := range b {
			b[i] = m.Read8(cpu.Addr(uint16(seg), uint16(off+uint32(i))))
		}
		os.WriteFile(*out+".bin", b, 0o644)
	}
	if *stateOut != "" {
		if err := state.Save(*stateOut, m, d); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
	if *finalShot != "" {
		shot(m, *finalShot)
	}
	fmt.Fprintf(w, "E\t%d\t%v\n", m.Steps, d.Exited)
}

func shot(m *machine.Machine, path string) {
	idx, pal := m.Indexed(), m.Palette()
	img := image.NewRGBA(image.Rect(0, 0, 320, 200))
	for i, v := range idx[:320*200] {
		p := pal[v]
		img.Set(i%320, i/320, color.RGBA{p[0], p[1], p[2], 255})
	}
	f, err := os.Create(path)
	if err != nil {
		return
	}
	png.Encode(f, img)
	f.Close()
}
