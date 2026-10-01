package main

import (
	"crypto/sha256"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/wicanr2/dosgolem/internal/cpu386"
	"github.com/wicanr2/dosgolem/internal/machine"
)

func main() {
	emptyFixture := len(os.Args) == 3 && os.Args[2] == "--empty-mox-set"
	fileFixture := len(os.Args) == 4 && os.Args[2] == "--mox-set"
	gameDirectory := len(os.Args) == 4 && os.Args[2] == "--game-dir"
	if len(os.Args) != 2 && !emptyFixture && !fileFixture && !gameDirectory {
		fmt.Fprintln(os.Stderr, "usage: moo2-probe <original-exe> [--empty-mox-set | --mox-set <local-file> | --game-dir <local-directory>]")
		os.Exit(2)
	}
	maxSteps := 8000000
	if setting := os.Getenv("DOSGOLEM_MOO2_MAX_STEPS"); setting != "" {
		value, parseErr := strconv.Atoi(setting)
		if parseErr != nil || value < 1 || value > 50000000 {
			fmt.Fprintln(os.Stderr, "DOSGOLEM_MOO2_MAX_STEPS 必須為 1 至 50000000 的十進位整數")
			os.Exit(2)
		}
		maxSteps = value
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
	var m *machine.LEMachine
	if os.Getenv("DOSGOLEM_MOO2_SEPARATE_DOS") == "1" {
		m, err = machine.LoadLEInMZWithDOSArena(b, 0x26654)
	} else {
		m, err = machine.LoadLEInMZ(b, 0x26654)
	}
	if err != nil {
		fmt.Printf("load_error=%v\n", err)
		os.Exit(1)
	}
	var files machine.ReadOnlyFileProvider
	if gameDirectory {
		provider, err := machine.OpenDirectoryReadOnlyFiles(os.Args[3])
		if err != nil {
			panic(err)
		}
		defer provider.Close()
		files = provider
		info, err := os.Stat(filepath.Join(os.Args[3], "MOX.SET"))
		if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
			panic("遊戲資料目錄須有小於 1 MiB 的一般 MOX.SET 檔案")
		}
		data, err := os.ReadFile(filepath.Join(os.Args[3], "MOX.SET"))
		if err != nil {
			panic(err)
		}
		fmt.Printf("game_dir_mox_set size=%d sha256=%x mtime_utc=%s\n",
			len(data), sha256.Sum256(data), info.ModTime().UTC().Format(time.RFC3339))
	} else if emptyFixture || fileFixture {
		var fixtureData []byte
		stamp := time.Date(1996, 1, 1, 0, 0, 0, 0, time.UTC)
		if fileFixture {
			info, err := os.Stat(os.Args[3])
			if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
				panic("MOX.SET 輸入須是小於 1 MiB 的一般檔案")
			}
			fixtureData, err = os.ReadFile(os.Args[3])
			if err != nil {
				panic(err)
			}
			stamp = info.ModTime().UTC()
		}
		root, err := os.MkdirTemp("", "moo2-mox-set-")
		if err != nil {
			panic(err)
		}
		defer os.RemoveAll(root)
		path := filepath.Join(root, "MOX.SET")
		if err := os.WriteFile(path, fixtureData, 0o600); err != nil {
			panic(err)
		}
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			panic(err)
		}
		provider, err := machine.OpenDirectoryReadOnlyFiles(root)
		if err != nil {
			panic(err)
		}
		defer provider.Close()
		files = provider
		fmt.Printf("controlled_fixture=MOX.SET size=%d sha256=%x mtime_utc=%s\n",
			len(fixtureData), sha256.Sum256(fixtureData), stamp.Format(time.RFC3339))
	}
	services := machine.NewMOO2StartupDOS(files)
	if err := services.AttachMachine(m); err != nil {
		panic(err)
	}
	fmt.Printf("separate_dos_arena=%t dos_arena_base=0x%X\n", m.DOSArenaBase != 0, m.DOSArenaBase)
	mousePositionSet := false
	var dtaSelector uint16
	var dtaOffset uint32
	dtaSet := false
	m.CPU.IntHook = func(c *cpu386.CPU, number uint8) bool {
		if number == 0x21 && uint8(c.R[cpu386.EAX]>>8) == 0x1a {
			selector, offset := c.Seg[cpu386.SegDS], c.R[cpu386.EDX]
			handled := services.Handle(c, number)
			if handled {
				dtaSelector, dtaOffset, dtaSet = selector, offset, true
			}
			fmt.Printf("find_dta eip=0x%X selector=0x%X offset=0x%X handled=%t\n", c.EIP-2, selector, offset, handled)
			return handled
		}
		if number == 0x21 && uint8(c.R[cpu386.EAX]>>8) == 0x4e {
			pattern := make([]byte, 0, 128)
			readOK := true
			for i := uint32(0); i < 128; i++ {
				if c.R[cpu386.EDX] > ^uint32(0)-i {
					readOK = false
					break
				}
				ch, ok := c.ReadSegment8(c.Seg[cpu386.SegDS], c.R[cpu386.EDX]+i)
				if !ok {
					readOK = false
					break
				}
				pattern = append(pattern, ch)
				if ch == 0 {
					break
				}
			}
			dta := make([]byte, 0, 43)
			for i := uint32(0); dtaSet && i < 43 && dtaOffset <= ^uint32(0)-i; i++ {
				ch, ok := c.ReadSegment8(dtaSelector, dtaOffset+i)
				if !ok {
					break
				}
				dta = append(dta, ch)
			}
			fmt.Printf("find_request eip=0x%X r=%X seg=%X flags=0x%X pattern_hex=%X read_ok=%t dta_set=%t dta_selector=0x%X dta_offset=0x%X dta_hex=%X\n", c.EIP-2, c.R, c.Seg, c.EFlags, pattern, readOK, dtaSet, dtaSelector, dtaOffset, dta)
		}
		if number == 0x10 && c.R[cpu386.EAX] == 0x4f05 {
			beforeR := c.R
			handled := services.Handle(c, number)
			fmt.Printf("vbe_window eip=0x%X input=%X handled=%t output=%X state=%+v\n", c.EIP-2, beforeR, handled, c.R, m.VBEState())
			return handled
		}
		if number == 0x10 && c.R[cpu386.EAX] == 0x4f07 {
			beforeR := c.R
			handled := services.Handle(c, number)
			fmt.Printf("vbe_display_start eip=0x%X input=%X handled=%t output=%X state=%+v\n", c.EIP-2, beforeR, handled, c.R, m.VBEState())
			return handled
		}
		if number == 0x33 {
			beforeR, beforeFlags := c.R, c.EFlags
			handled := services.Handle(c, number)
			if handled && uint16(beforeR[cpu386.EAX]) == 4 {
				mousePositionSet = true
			}
			fmt.Printf("mouse_service eip=0x%X input=%X input_flags=0x%X handled=%t output=%X output_flags=0x%X\n", c.EIP-2, beforeR, beforeFlags, handled, c.R, c.EFlags)
			return handled
		}
		if number != 0x31 || uint16(c.R[cpu386.EAX]) != 0x0100 {
			return services.Handle(c, number)
		}
		beforeEIP, requested := c.EIP, uint16(c.R[cpu386.EBX])
		handled := services.Handle(c, number)
		fmt.Printf("dpmi_dos_alloc eip=0x%X requested_paras=%d handled=%t ax=0x%X bx=0x%X dx=0x%X cf=%t\n",
			beforeEIP, requested, handled, uint16(c.R[cpu386.EAX]), uint16(c.R[cpu386.EBX]),
			uint16(c.R[cpu386.EDX]), c.EFlags&cpu386.CF != 0)
		return handled
	}
	fmt.Printf("diagnostic_only_moo2_adapter=true; startup_returns_from_dosbox_x_auxiliary=true; synthetic_environment=true\n")
	fmt.Printf("loaded=true entry=0x%X esp=0x%X bytes=%d\n", m.CPU.EIP, m.CPU.R[cpu386.ESP], len(m.Mem))
	fmt.Printf("entry_bytes=% X\n", m.Mem[m.CPU.EIP:m.CPU.EIP+16])
	fmt.Printf("entry_window=% X\n", m.Mem[m.CPU.EIP:m.CPU.EIP+80])
	dumpVBE := func() {
		pixels := m.VBEIndexed()
		fmt.Printf("vbe_video state=%+v indexed_bytes=%d indexed_sha256=%x\n", m.VBEState(), len(pixels), sha256.Sum256(pixels))
		if path := os.Getenv("DOSGOLEM_MOO2_VBE_PNG"); path != "" && len(pixels) == 640*480 {
			rgb := m.VBERGB()
			out := image.NewNRGBA(image.Rect(0, 0, 640, 480))
			for pixel := range pixels {
				copy(out.Pix[pixel*4:pixel*4+3], rgb[pixel*3:pixel*3+3])
				out.Pix[pixel*4+3] = 255
			}
			file, err := os.Create(path)
			if err != nil {
				panic(err)
			}
			encodeErr, closeErr := png.Encode(file, out), file.Close()
			if encodeErr != nil {
				panic(encodeErr)
			}
			if closeErr != nil {
				panic(closeErr)
			}
			fmt.Printf("vbe_png path=%s rgb_sha256=%x\n", path, sha256.Sum256(rgb))
		}
	}
	seen := map[uint32]int{}
	type sample struct {
		step               int
		eip, esp, esi, eax uint32
	}
	ring := make([]sample, 0, 32)
	mouseEventInjected := false
	for i := 0; i < maxSteps; i++ {
		mouseEventRequested := os.Getenv("DOSGOLEM_MOO2_MOUSE_EVENT") == "1" || (os.Getenv("DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION") == "1" && mousePositionSet)
		if mouseEventRequested && !mouseEventInjected && m.CPU.EFlags&cpu386.IF != 0 {
			mask, _, _, _, _ := services.MouseCallbackState()
			if mask != 0 {
				if err := services.InjectMouseEvent(657, 189, 0, 0, 0); err != nil {
					panic(err)
				}
				fmt.Printf("controlled_mouse_event step=%d x=657 y=189 buttons=0 delta=0/0\n", i)
				mouseEventInjected = true
			}
		}
		if m.CPU.EIP == 0x100cf {
			value, err := m.Read16(0x191cbe)
			fmt.Printf("empty_mox_cmp step=%d eip=0x%X ds=0x%X source_linear=0x191CBE source_word=0x%X read_error=%v flags=0x%X\n",
				i, m.CPU.EIP, m.CPU.Seg[cpu386.SegDS], value, err, m.CPU.EFlags)
		}
		if i >= 5500 && (i < 20000 || i%10000 == 0) {
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
		// 固定 1.31 的比較輸入觀測；不注入狀態，最多保存前三次。
		if m.CPU.EIP == 0x234c9a && seen[m.CPU.EIP] <= 3 {
			var destination uint32
			readable := true
			for j := uint32(0); j < 4; j++ {
				value, ok := m.CPU.ReadSegment8(m.CPU.Seg[cpu386.SegDS], 0x270d9b+j)
				destination |= uint32(value) << (8 * j)
				readable = readable && ok
			}
			fmt.Printf("cmp_memory_sample step=%d eip=0x%X ds=0x%X destination_offset=0x270D9B destination=0x%X readable=%t ecx=0x%X flags=0x%X\n",
				i, m.CPU.EIP, m.CPU.Seg[cpu386.SegDS], destination, readable, m.CPU.R[cpu386.ECX], m.CPU.EFlags)
		}
		if m.CPU.EIP == 0x228dce && seen[m.CPU.EIP] <= 3 {
			low, lowOK := m.CPU.ReadSegment8(m.CPU.Seg[cpu386.SegDS], m.CPU.R[cpu386.EDI])
			high, highOK := m.CPU.ReadSegment8(m.CPU.Seg[cpu386.SegDS], m.CPU.R[cpu386.EDI]+1)
			fmt.Printf("cmp_word_sample step=%d eip=0x%X ds=0x%X destination_offset=0x%X destination=0x%X readable=%t eax=0x%X flags=0x%X\n",
				i, m.CPU.EIP, m.CPU.Seg[cpu386.SegDS], m.CPU.R[cpu386.EDI], uint16(low)|uint16(high)<<8, lowOK && highOK, m.CPU.R[cpu386.EAX], m.CPU.EFlags)
		}
		if m.CPU.EIP == 0x21ca3d && seen[m.CPU.EIP] <= 3 {
			destination := m.CPU.R[cpu386.EAX] + 4
			low, lowOK := m.CPU.ReadSegment8(m.CPU.Seg[cpu386.SegDS], destination)
			high, highOK := m.CPU.ReadSegment8(m.CPU.Seg[cpu386.SegDS], destination+1)
			fmt.Printf("inc_word_sample step=%d eip=0x%X ds=0x%X destination_offset=0x%X destination=0x%X readable=%t eax=0x%X flags=0x%X\n",
				i, m.CPU.EIP, m.CPU.Seg[cpu386.SegDS], destination, uint16(low)|uint16(high)<<8, lowOK && highOK, m.CPU.R[cpu386.EAX], m.CPU.EFlags)
		}
		if len(ring) == cap(ring) {
			ring = ring[1:]
		}
		ring = append(ring, sample{i, m.CPU.EIP, m.CPU.R[cpu386.ESP], m.CPU.R[cpu386.ESI], m.CPU.R[cpu386.EAX]})
		if i < 24 {
			v, _ := m.Read16(0x21996)
			fmt.Printf("trace step=%d eip=0x%X edx=0x%X flags=0x%X timer_word=0x%X\n", i, m.CPU.EIP, m.CPU.R[cpu386.EDX], m.CPU.EFlags, v)
		}
		if (m.CPU.EIP == 0x21c2d6 || m.CPU.EIP == 0x21c2da) && seen[m.CPU.EIP] <= 3 {
			fmt.Printf("sign_branch_input step=%d eip=0x%X r=%X seg=%X flags=0x%X\n", i, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags)
		}
		if err := m.CPU.Step(); err != nil {
			mask, pending, active, started, completed := services.MouseCallbackState()
			fmt.Printf("mouse_callback mask=%X pending=%d active=%t started=%d completed=%d\n", mask, pending, active, started, completed)
			fmt.Printf("step_error step=%d eip=0x%X eax=0x%X ebx=0x%X ecx=0x%X edx=0x%X es=0x%X ds=0x%X ss=0x%X flags=0x%X dos_calls=%d error=%v\n", i, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], m.CPU.R[cpu386.ECX], m.CPU.R[cpu386.EDX], m.CPU.Seg[cpu386.SegES], m.CPU.Seg[cpu386.SegDS], m.CPU.Seg[cpu386.SegSS], m.CPU.EFlags, services.Calls(), err)
			fmt.Printf("dpmi_real_mode_last=%+v dpmi_unimplemented=%v\n", services.DPMI.RealModeLast, services.DPMI.Unimplemented)
			dumpVBE()
			fmt.Printf("stop_bytes=% X\n", m.Mem[ring[len(ring)-1].eip:ring[len(ring)-1].eip+16])
			for _, s := range ring {
				fmt.Printf("tail step=%d eip=0x%X esp=0x%X esi=0x%X eax=0x%X\n", s.step, s.eip, s.esp, s.esi, s.eax)
			}
			return
		}
		branchEIP := ring[len(ring)-1].eip
		if (branchEIP == 0x21c2d6 || branchEIP == 0x21c2da) && seen[branchEIP] <= 3 {
			fmt.Printf("sign_branch_result step=%d input_eip=0x%X after_eip=0x%X r=%X seg=%X flags=0x%X\n", i, branchEIP, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags)
		}
		if services.Exited {
			fmt.Printf("dos_exit step=%d code=%d after_eip=0x%X console=%q\n",
				i, services.ExitCode, m.CPU.EIP, services.Console)
			fmt.Printf("dos_exit_memory image_bytes=%d dpmi_calls=%v dpmi_unimplemented=%v dos_blocks=%v linear_blocks=%v\n",
				len(m.Mem), services.DPMI.Calls, services.DPMI.Unimplemented, services.DPMI.DOSMemory(), services.DPMI.Blocks())
			return
		}
	}
	fmt.Printf("step_limit=%d eip=0x%X unique_sites=%d\n", maxSteps, m.CPU.EIP, len(seen))
	fmt.Printf("step_limit_registers r=%X seg=%X flags=0x%X bytes=% X\n", m.CPU.R, m.CPU.Seg, m.CPU.EFlags, m.Mem[m.CPU.EIP:m.CPU.EIP+16])
	for _, s := range ring {
		fmt.Printf("step_limit_tail step=%d eip=0x%X esp=0x%X esi=0x%X eax=0x%X bytes=% X\n", s.step, s.eip, s.esp, s.esi, s.eax, m.Mem[s.eip:s.eip+8])
	}
	fmt.Printf("step_limit_memory image_bytes=%d dpmi_calls=%v dpmi_unimplemented=%v dos_blocks=%v linear_blocks=%v\n",
		len(m.Mem), services.DPMI.Calls, services.DPMI.Unimplemented, services.DPMI.DOSMemory(), services.DPMI.Blocks())
	dumpVBE()
}
