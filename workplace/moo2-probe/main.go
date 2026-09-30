package main

import (
	"fmt"
	"os"

	"github.com/wicanr2/dosgolem/internal/cpu386"
	"github.com/wicanr2/dosgolem/internal/machine"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: moo2-probe <original-exe>")
		os.Exit(2)
	}
	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	h, err := machine.InspectLEInMZ(b, 0x26654)
	if err != nil {
		panic(err)
	}
	fmt.Printf("header mz_base=0x%X offset=0x%X module_flags=0x%X data_pages_offset=0x%X data_pages_file_offset=0x%X entry_object=%d entry_offset=0x%X\n", h.MZBase, h.Offset, h.ModuleFlags, h.DataPagesOffset, h.MZBase+h.DataPagesOffset, h.EIPObject, h.EIP)
	m, err := machine.LoadLEInMZ(b, 0x26654)
	if err != nil {
		fmt.Printf("load_error=%v\n", err)
		os.Exit(1)
	}
	services := machine.NewMOO2StartupDOS(nil)
	services.AttachMachine(m)
	m.CPU.IntHook = services.Handle
	fmt.Printf("diagnostic_only_moo2_adapter=true; startup_returns_from_dosbox_x_auxiliary=true; synthetic_environment=true\n")
	fmt.Printf("loaded=true entry=0x%X esp=0x%X bytes=%d\n", m.CPU.EIP, m.CPU.R[cpu386.ESP], len(m.Mem))
	fmt.Printf("entry_bytes=% X\n", m.Mem[m.CPU.EIP:m.CPU.EIP+16])
	fmt.Printf("entry_window=% X\n", m.Mem[m.CPU.EIP:m.CPU.EIP+80])
	seen := map[uint32]int{}
	type sample struct {
		step               int
		eip, esp, esi, eax uint32
	}
	ring := make([]sample, 0, 32)
	for i := 0; i < 200000; i++ {
		if i >= 5500 {
			if i == 5500 || m.CPU.R[cpu386.ESI] != ring[len(ring)-1].esi || m.CPU.R[cpu386.EAX] != ring[len(ring)-1].eax {
				fmt.Printf("change step=%d eip=0x%X esi=0x%X eax=0x%X bytes=% X\n", i, m.CPU.EIP, m.CPU.R[cpu386.ESI], m.CPU.R[cpu386.EAX], m.Mem[m.CPU.EIP:m.CPU.EIP+8])
			}
		}
		seen[m.CPU.EIP]++
		if (m.CPU.EIP == 0x146903 || m.CPU.EIP == 0x14822d || m.CPU.EIP == 0x15c1df) && seen[m.CPU.EIP] <= 3 {
			esi := m.CPU.R[cpu386.ESI]
			var source string
			if uint64(esi) < uint64(len(m.Mem)) {
				source = fmt.Sprintf("%02X", m.Mem[esi])
			} else {
				source = "越界"
			}
			fmt.Printf("checkpoint step=%d eip=0x%X eax=0x%X ebx=0x%X ecx=0x%X edx=0x%X esi=0x%X edi=0x%X ebp=0x%X esp=0x%X ds=0x%X es=0x%X ss=0x%X flags=0x%X source_flat=%s dos_calls=%d\n", i, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], m.CPU.R[cpu386.ECX], m.CPU.R[cpu386.EDX], esi, m.CPU.R[cpu386.EDI], m.CPU.R[cpu386.EBP], m.CPU.R[cpu386.ESP], m.CPU.Seg[cpu386.SegDS], m.CPU.Seg[cpu386.SegES], m.CPU.Seg[cpu386.SegSS], m.CPU.EFlags, source, services.Calls())
		}
		if len(ring) == cap(ring) {
			ring = ring[1:]
		}
		ring = append(ring, sample{i, m.CPU.EIP, m.CPU.R[cpu386.ESP], m.CPU.R[cpu386.ESI], m.CPU.R[cpu386.EAX]})
		if i < 24 {
			v, _ := m.Read16(0x21996)
			fmt.Printf("trace step=%d eip=0x%X edx=0x%X flags=0x%X timer_word=0x%X\n", i, m.CPU.EIP, m.CPU.R[cpu386.EDX], m.CPU.EFlags, v)
		}
		if err := m.CPU.Step(); err != nil {
			fmt.Printf("step_error step=%d eip=0x%X eax=0x%X ebx=0x%X ecx=0x%X edx=0x%X es=0x%X ds=0x%X ss=0x%X flags=0x%X dos_calls=%d error=%v\n", i, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], m.CPU.R[cpu386.ECX], m.CPU.R[cpu386.EDX], m.CPU.Seg[cpu386.SegES], m.CPU.Seg[cpu386.SegDS], m.CPU.Seg[cpu386.SegSS], m.CPU.EFlags, services.Calls(), err)
			fmt.Printf("stop_bytes=% X\n", m.Mem[ring[len(ring)-1].eip:ring[len(ring)-1].eip+16])
			for _, s := range ring {
				fmt.Printf("tail step=%d eip=0x%X esp=0x%X esi=0x%X eax=0x%X\n", s.step, s.eip, s.esp, s.esi, s.eax)
			}
			return
		}
		if services.Exited {
			fmt.Printf("dos_exit step=%d code=%d after_eip=0x%X console=%q\n",
				i, services.ExitCode, m.CPU.EIP, services.Console)
			return
		}
	}
	fmt.Printf("step_limit=200000 eip=0x%X unique_sites=%d\n", m.CPU.EIP, len(seen))
}
