package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wicanr2/dosgolem/internal/cpu386"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// 只觀察實際指令的段讀取，不改Bus身分或既有hook返回值。
type orMemoryReadObserver struct {
	target, linear         uint32
	selector               uint16
	armed, active, matched bool
	remaining              int
}

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
	keyboardRequested := os.Getenv("DOSGOLEM_MOO2_HARDWARE_ESCAPE_AT_48000000") == "1"
	keyboardStep := 48000000
	if setting := os.Getenv("DOSGOLEM_MOO2_HARDWARE_ESCAPE_STEP"); setting != "" {
		if keyboardRequested {
			fmt.Fprintln(os.Stderr, "兩種硬體Esc排程設定互斥")
			os.Exit(2)
		}
		digits := true
		for _, ch := range setting {
			digits = digits && ch >= '0' && ch <= '9'
		}
		value, parseErr := strconv.Atoi(setting)
		if !digits || parseErr != nil || value < 1 || value >= maxSteps {
			fmt.Fprintln(os.Stderr, "DOSGOLEM_MOO2_HARDWARE_ESCAPE_STEP 必須為小於maxSteps的十進位正整數")
			os.Exit(2)
		}
		keyboardRequested, keyboardStep = true, value
		fmt.Printf("hardware_keyboard_schedule step=%d source=explicit_environment max_steps=%d\n", keyboardStep, maxSteps)
	}
	newGameClick := os.Getenv("DOSGOLEM_MOO2_NEW_GAME_CLICK_AFTER_DISPLAY40") == "1"
	if newGameClick && (!keyboardRequested || keyboardStep != 44000000 && keyboardStep != 46000000 || maxSteps != 50000000 || os.Getenv("DOSGOLEM_MOO2_CALENDAR_EPOCH") != "1996-01-01" || os.Getenv("DOSGOLEM_MOO2_MOUSE_EVENT") == "1" || os.Getenv("DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION") == "1") {
		fmt.Fprintln(os.Stderr, "NEW GAME點擊要求44M或46M Esc、1996-01-01、50M cap且無早期滑鼠事件")
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

	if setting := os.Getenv("DOSGOLEM_MOO2_CALENDAR_EPOCH"); setting != "" {
		epoch, err := time.Parse("2006-01-02", setting)
		if err != nil || len(setting) != 10 || epoch.Format("2006-01-02") != setting {
			fmt.Fprintln(os.Stderr, "DOSGOLEM_MOO2_CALENDAR_EPOCH 必須為 YYYY-MM-DD 的有效DOS日期")
			os.Exit(2)
		}
		if err := services.SetCalendarEpoch(epoch.Year(), int(epoch.Month()), epoch.Day()); err != nil {
			panic(err)
		}
		fmt.Printf("calendar_initial epoch=%s source=explicit_environment virtual_micros=0 approximation=platform-spec\n", setting)
	}
	fmt.Printf("separate_dos_arena=%t dos_arena_base=0x%X\n", m.DOSArenaBase != 0, m.DOSArenaBase)
	mousePositionSet := false
	var irq1Target, irq1Previous uint64
	var dtaSelector uint16
	var dtaOffset uint32
	dtaSet := false
	loopStep := 0
	calendarConsumerSteps := 0
	wordAddMemorySeen, wordAddMemorySteps := 0, 0
	cmpWordSamples, cmpWordSteps, cmpWordTotal := 0, 0, 0
	cmpWordBoundarySeen := false
	mouseExchangeSamples, mouseExchangeSteps := 0, 0
	phaseFrames := 0
	var phaseLastDisplay uint64
	phasePrefix := os.Getenv("DOSGOLEM_MOO2_VBE_FRAME_PREFIX")
	menuDisplay40Seen := false
	findQuestionConsumerSteps := 0
	byteAddSamples, byteAddConsumerBudget, byteAddConsumerSamples := 0, 0, 0
	var byteAddWatchLinear uint64
	byteAddWatchReadable := false
	negDwordSamples, negDwordConsumerBudget, negDwordConsumerSamples := 0, 0, 0
	var negDwordWatchLinear uint64
	var negDwordWatchSelector uint16
	var negDwordWatchOffset uint32
	negDwordWatchReadable := false
	defer func() {
		fmt.Printf("cmp_word_immediate_totals observed_site=14E3DE total=%d sample_groups=%d boundary212_observed=%t\n", cmpWordTotal, cmpWordSamples, cmpWordBoundarySeen)
	}()

	m.CPU.IntHook = func(c *cpu386.CPU, number uint8) bool {
		if number == 0x21 && uint8(c.R[cpu386.EAX]>>8) == 0x2a {
			beforeR, beforeSeg, beforeFlags := c.R, c.Seg, c.EFlags
			epoch, micros, configured := services.CalendarState()
			handled := services.Handle(c, number)
			fmt.Printf("calendar_date_service outer_step=%d address_space=dosgolem_high_le eip=%X epoch=%s virtual_micros=%d configured=%t before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X handled=%t\n", loopStep, c.EIP-2, epoch, micros, configured, beforeR, c.R, beforeSeg, c.Seg, beforeFlags, c.EFlags, handled)
			if handled {
				calendarConsumerSteps = 8
			}
			return handled
		}

		if number == 0x31 && uint16(c.R[cpu386.EAX]) == 0x0300 && uint16(c.R[cpu386.EBX]) == 0x0066 {
			ports, ok := services.DPMI.RealModeIO.(*machine.LEOPLPorts)
			if !ok {
				return services.Handle(c, number)
			}
			before := ports.State()
			beforeR, beforeSeg, beforeFlags := c.R, c.Seg, c.EFlags
			handled := services.Handle(c, number)
			after := ports.State()
			if after.DSPAuto8Commands != before.DSPAuto8Commands && after.DSPAuto8Commands <= 3 {
				start := uint32(after.DMAPage[1])*65536 + uint32(after.DMABase[2])
				size := uint32(after.DMABase[3]) + 1
				var sourceHash [32]byte
				var sourcePrefix []byte
				sourceReadable := uint64(start)+uint64(size) <= uint64(len(m.Mem))
				if sourceReadable {
					sourceHash = sha256.Sum256(m.Mem[start : start+size])
					sourcePrefix = append([]byte(nil), m.Mem[start:start+min(size, 16)]...)
				}
				fmt.Printf("sb16_c6_call address_space=dosgolem_high_le eip=0x%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=0x%X after_flags=0x%X handled=%t command=%X before_state=%+v after_state=%+v source_timing=after_real_mode_return source_linear=0x%X source_bytes=%d source_sha256=%x source_prefix=%X pcm_prefix=%X source_readable=%t real_mode=%+v\n", c.EIP-2, beforeR, c.R, beforeSeg, c.Seg, beforeFlags, c.EFlags, handled, after.DSPAuto8Command, before, after, start, size, sourceHash, sourcePrefix, ports.PCM[:min(len(ports.PCM), 16)], sourceReadable, services.DPMI.RealModeLast)
			}
			return handled
		}
		if number == 0x21 && (uint8(c.R[cpu386.EAX]>>8) == 0x25 || uint8(c.R[cpu386.EAX]>>8) == 0x35) {
			beforeR, beforeSeg, beforeFlags := c.R, c.Seg, c.EFlags
			handled := services.Handle(c, number)
			if handled && c.EFlags&cpu386.CF == 0 && uint8(beforeR[cpu386.EAX]) == 9 {
				if uint8(beforeR[cpu386.EAX]>>8) == 0x25 {
					irq1Target = uint64(beforeSeg[cpu386.SegDS])<<32 | uint64(beforeR[cpu386.EDX])
				} else {
					irq1Previous = uint64(c.Seg[cpu386.SegES])<<32 | uint64(c.R[cpu386.EBX])
				}
			}
			fmt.Printf("dos_vector_service eip=0x%X input=%X input_seg=%X input_flags=0x%X handled=%t output=%X output_seg=%X output_flags=0x%X\n", c.EIP-2, beforeR, beforeSeg, beforeFlags, handled, c.R, c.Seg, c.EFlags)
			return handled
		}
		if number == 0x31 && uint16(c.R[cpu386.EAX]) >= 0x200 && uint16(c.R[cpu386.EAX]) <= 0x205 {
			beforeR, beforeSeg, beforeFlags := c.R, c.Seg, c.EFlags
			handled := services.Handle(c, number)
			fmt.Printf("dpmi_vector_service eip=0x%X input=%X input_seg=%X input_flags=0x%X handled=%t output=%X output_seg=%X output_flags=0x%X\n", c.EIP-2, beforeR, beforeSeg, beforeFlags, handled, c.R, c.Seg, c.EFlags)
			return handled
		}
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
			if readOK && strings.ContainsRune(string(pattern), '?') {
				beforeR, beforeSeg, beforeFlags := c.R, c.Seg, c.EFlags
				handled := services.Handle(c, number)
				var afterDTA []byte
				desc, known := c.Descriptors[dtaSelector]
				linear := uint64(desc.Base) + uint64(dtaOffset)
				if known && dtaSet && uint64(dtaOffset)+43 <= uint64(desc.Limit)+1 && linear+43 <= uint64(len(m.Mem)) {
					afterDTA = append([]byte(nil), m.Mem[linear:linear+43]...)
				}
				fmt.Printf("find_question_result outer_step=%d address_space=dosgolem_high_le callsite=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X before_dta=%X after_dta=%X handled=%t\n", loopStep, c.EIP-2, beforeR, c.R, beforeSeg, c.Seg, beforeFlags, c.EFlags, dta, afterDTA, handled)
				if handled {
					findQuestionConsumerSteps = 12
				}
				return handled
			}
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
			state := m.VBEState()
			if handled && state.Active && state.DisplaySets == 40 {
				menuDisplay40Seen = true
			}
			// 規格317：保留既有16張，另只讀擷取已見的第40換頁。
			if phasePrefix != "" && handled && loopStep >= keyboardStep && state.Active && (phaseFrames < 16 || state.DisplaySets == 40) && state.DisplaySets != phaseLastDisplay {
				phaseFrames++
				phaseLastDisplay = state.DisplaySets
				beforeR, beforeSeg, beforeEIP, beforeFlags := c.R, c.Seg, c.EIP, c.EFlags
				control, status, stack, depth := c.FPUControl, c.FPUStatus, c.FPUStack, c.FPUDepth
				pixels, rgb := m.VBEIndexed(), m.VBERGB()
				path := fmt.Sprintf("%s-%03d.png", phasePrefix, state.DisplaySets)
				if len(pixels) != 640*480 || len(rgb) != 640*480*3 {
					panic("VBE換頁快照尺寸錯誤")
				}
				out := image.NewNRGBA(image.Rect(0, 0, 640, 480))
				for pixel := range pixels {
					copy(out.Pix[pixel*4:pixel*4+3], rgb[pixel*3:pixel*3+3])
					out.Pix[pixel*4+3] = 255
				}
				f, err := os.Create(path)
				if err != nil {
					panic(err)
				}
				encodeErr, closeErr := png.Encode(f, out), f.Close()
				if encodeErr != nil {
					panic(encodeErr)
				}
				if closeErr != nil {
					panic(closeErr)
				}
				encoded, err := os.ReadFile(path)
				if err != nil {
					panic(err)
				}
				readonly := c.R == beforeR && c.Seg == beforeSeg && c.EIP == beforeEIP && c.EFlags == beforeFlags && c.FPUControl == control && c.FPUStatus == status && c.FPUDepth == depth && m.VBEState() == state
				for j := range stack {
					readonly = readonly && math.Float64bits(stack[j]) == math.Float64bits(c.FPUStack[j])
				}
				if !readonly {
					panic("VBE換頁觀測改變原始狀態")
				}
				micros := uint64(0)
				if ports, ok := services.DPMI.RealModeIO.(*machine.LEOPLPorts); ok {
					micros = ports.BIOSClock.Micros
				}
				fmt.Printf("menu_slide_phase outer_step=%d address_space=dosgolem_high_le callsite=%X virtual_micros=%d display=%d state=%+v r=%X seg=%X flags=%X indexed_sha256=%x rgb_sha256=%x png_sha256=%x path=%s readonly=%t\n", loopStep, c.EIP-2, micros, state.DisplaySets, state, c.R, c.Seg, c.EFlags, sha256.Sum256(pixels), sha256.Sum256(rgb), sha256.Sum256(encoded), path, readonly)
			}
			return handled
		}
		if number == 0x33 {
			beforeR, beforeFlags := c.R, c.EFlags
			beforeSeg := c.Seg
			oldSelector, oldOffset := services.MouseCallbackTarget()
			oldMask, oldPending, oldActive, oldStarted, oldCompleted := services.MouseCallbackState()
			handled := services.Handle(c, number)
			if uint16(beforeR[cpu386.EAX]) == 0x14 && mouseExchangeSamples < 3 {
				mouseExchangeSamples++
				mouseExchangeSteps = 5
				newSelector, newOffset := services.MouseCallbackTarget()
				newMask, newPending, newActive, newStarted, newCompleted := services.MouseCallbackState()
				fmt.Printf("mouse_exchange_service input_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X old_target=%04X:%08X new_target=%04X:%08X old_mask=%X new_mask=%X old_pending=%d new_pending=%d old_active=%t new_active=%t old_started=%d new_started=%d old_completed=%d new_completed=%d handled=%t\n", c.EIP-2, beforeR, c.R, beforeSeg, c.Seg, beforeFlags, c.EFlags, oldSelector, oldOffset, newSelector, newOffset, oldMask, newMask, oldPending, newPending, oldActive, newActive, oldStarted, newStarted, oldCompleted, newCompleted, handled)
			}
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
	mouseEventInjected := false
	dumpPlatform := func(label string, step int) {
		if ports, ok := services.DPMI.RealModeIO.(*machine.LEOPLPorts); ok {
			keyboardReads, enqueued := uint64(0), 0
			keyboardWaiting := false
			if m.Keyboard != nil {
				keyboardReads, enqueued, keyboardWaiting = m.Keyboard.Reads, len(m.Keyboard.Enqueued), m.Keyboard.Waiting
			}
			mask, pending, active, started, completed := services.MouseCallbackState()
			seg9, off9 := services.DPMI.RealModeVector(9)
			fmt.Printf("late_startup_platform label=%s outer_step=%d address_space=dosgolem_high_le eip=0x%X r=%X seg=%X flags=0x%X bios_clock=%+v device=%+v irq7_deliveries=%d pcm16_bytes=%d keyboard_installed=%t keyboard_reads=%d keyboard_waiting=%t keyboard_enqueued=%d bda_queue_bytes=%X keyboard_port_reads=%d/%d/%d rm09=%04X:%04X absolute_ivt09=%X mouse_mask=%X mouse_pending=%d mouse_active=%t mouse_started=%d mouse_completed=%d watch_base=0x2A8E40 watch_bytes=%X\n", label, step, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags, ports.BIOSClock, ports.State(), ports.IRQ7Deliveries, len(ports.PCM16), m.Keyboard != nil, keyboardReads, keyboardWaiting, enqueued, m.Mem[0x41a:0x41e], ports.Reads[0x60], ports.Reads[0x61], ports.Reads[0x64], seg9, off9, m.Mem[0x24:0x28], mask, pending, active, started, completed, m.Mem[0x2a8e40:0x2a8e58])
			fmt.Printf("irq7_passdown_state label=%s outer_step=%d started=%d completed=%d trace=%+v\n", label, step, ports.IRQ7Passdowns, ports.IRQ7Returns, ports.IRQ7Last)
		}
	}
	dumpVBE := func() {
		fmt.Printf("controlled_mouse_event_requested=%t injected=%t\n", os.Getenv("DOSGOLEM_MOO2_MOUSE_EVENT") == "1" || os.Getenv("DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION") == "1", mouseEventInjected)
		irqActive, irqFailed, irqStarted, irqCompleted := services.IRQ0State()
		fmt.Printf("protected_irq0 active=%t failed=%t started=%d completed=%d\n", irqActive, irqFailed, irqStarted, irqCompleted)
		if ports, ok := services.DPMI.RealModeIO.(*machine.LEOPLPorts); ok {
			seg8, off8 := services.DPMI.RealModeVector(8)
			seg1c, off1c := services.DPMI.RealModeVector(0x1c)
			input := make([]byte, 4)
			readable := true
			for j := uint32(0); j < 4; j++ {
				value, ok := m.CPU.ReadSegment8(m.CPU.Seg[cpu386.SegDS], 0x271148+j)
				input[j], readable = value, readable && ok
			}
			fmt.Printf("platform_clock_state pit=%+v clock=%+v dpmi_rm08=%04X:%04X dpmi_rm1c=%04X:%04X absolute_ivt08=% X absolute_ivt1c=% X wait_ds=0x%X wait_offset=0x271148 wait_bytes=% X readable=%t\n", ports.PIT0, ports.BIOSClock, seg8, off8, seg1c, off1c, m.Mem[0x20:0x24], m.Mem[0x70:0x74], m.CPU.Seg[cpu386.SegDS], input, readable)
		}
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
	shlSeen := map[uint32]int{}
	adcSeen := map[uint32]int{}
	wordXorSeen := map[uint32]int{}
	wordXchgSeen := map[uint32]int{}
	lateCallerSeen := map[uint32]int{}
	previousStepHook := m.CPU.StepHook
	m.CPU.StepHook = func(c *cpu386.CPU) (bool, error) {
		if (c.EIP == 0x2520b7 || c.EIP == 0x2520b9 || c.EIP == 0x2520bb || c.EIP == 0x2520c5 || c.EIP == 0x2520c7 || c.EIP == 0x2520c9 || c.EIP == 0x2520cb || c.EIP == 0x2520d0 || c.EIP == 0x2520d6) && shlSeen[c.EIP] < 3 {
			shlSeen[c.EIP]++
			active, failed, started, completed := services.IRQ0State()
			fmt.Printf("dword_shl_one_state outer_step=%d address_space=dosgolem_high_le eip=0x%X r=%X seg=%X flags=0x%X irq0_active=%t irq0_failed=%t irq0_started=%d irq0_completed=%d test_input_bytes=%X destination_bytes=%X bytes=% X\n", loopStep, c.EIP, c.R, c.Seg, c.EFlags, active, failed, started, completed, m.Mem[0x272d28:0x272d2c], m.Mem[0x272d40:0x272d48], m.Mem[c.EIP:c.EIP+64])
		}
		if (c.EIP == 0x25179f || c.EIP == 0x2517a1 || c.EIP == 0x2517a8 || c.EIP == 0x2517ab) && adcSeen[c.EIP] < 3 {
			adcSeen[c.EIP]++
			active, failed, started, completed := services.IRQ0State()
			fmt.Printf("dword_adc_state outer_step=%d address_space=dosgolem_high_le eip=0x%X r=%X seg=%X flags=0x%X irq0_active=%t irq0_failed=%t irq0_started=%d irq0_completed=%d indexed_source_bytes=%X bytes=% X\n", loopStep, c.EIP, c.R, c.Seg, c.EFlags, active, failed, started, completed, m.Mem[0x272d40:0x272d48], m.Mem[c.EIP:c.EIP+64])
		}
		if (c.EIP == 0x24678c || c.EIP == 0x246790 || c.EIP == 0x246791 || c.EIP == 0x246792) && wordXorSeen[c.EIP] < 3 {
			wordXorSeen[c.EIP]++
			active, failed, started, completed := services.IRQ0State()
			fmt.Printf("word_xor_imm_state outer_step=%d address_space=dosgolem_high_le eip=0x%X r=%X seg=%X flags=0x%X irq0_active=%t irq0_failed=%t irq0_started=%d irq0_completed=%d stack_destination_bytes=%X bytes=% X\n", loopStep, c.EIP, c.R, c.Seg, c.EFlags, active, failed, started, completed, m.Mem[0x2723f8:0x272400], m.Mem[c.EIP:c.EIP+64])
		}
		if (c.EIP == 0x256171 || c.EIP == 0x256173 || c.EIP == 0x256176) && wordXchgSeen[c.EIP] < 3 {
			wordXchgSeen[c.EIP]++
			active, failed, started, completed := services.IRQ0State()
			fmt.Printf("word_xchg_state outer_step=%d address_space=dosgolem_high_le eip=0x%X r=%X seg=%X flags=0x%X irq0_active=%t irq0_failed=%t irq0_started=%d irq0_completed=%d bytes=% X\n", loopStep, c.EIP, c.R, c.Seg, c.EFlags, active, failed, started, completed, m.Mem[c.EIP:c.EIP+64])
		}
		if loopStep >= 49000000 && (c.EIP == 0x231ae4 || c.EIP == 0x231aeb || c.EIP == 0x22fcd2) && lateCallerSeen[c.EIP] < 2 {
			lateCallerSeen[c.EIP]++
			dumpPlatform("caller", loopStep)
		}
		if previousStepHook != nil {
			return previousStepHook(c)
		}
		return false, nil
	}
	c6CallerSeen := map[uint32]int{}
	orReads := &orMemoryReadObserver{remaining: 16}
	type sample struct {
		step               int
		eip, esp, esi, eax uint32
	}
	ring := make([]sample, 0, 32)
	irq7FirstPrinted := false
	var dma8ControlsPrinted uint64
	dma8ConsumerSteps := 0
	keyboardQueued := false
	var keyboardPrinted uint64
	xorALSeen, xorALConsumerSteps := 0, 0
	newGamePressed, newGameReleased := false, false
	var newGamePressMicros uint64
	newGameCallbackSamples := 0
	buttonReadSamples := 0
	buttonReadStepActive := false
	buttonReadMatched := false
	var buttonReadWidth uint32
	var buttonReadWord [2]byte
	var buttonRequests [3][2]uint64
	var buttonPositiveCounts [3]int
	var buttonHook8, buttonHook16 uintptr
	buttonPositivePending := false
	var buttonPositiveWidth, buttonPositiveOffset uint32
	var buttonPositiveSelector uint16
	var buttonPositiveLinear uint64
	var buttonPositiveBytes [2]byte
	var eventReadSelector uint16
	var eventReadOffset uint32
	var eventReadLinear uint64
	var eventReadWindow [16]byte
	var eventReadCounter [4]byte
	eventConsumerBudget, eventConsumerSamples := 0, 0
	observeButtonRequest := func(selector uint16, offset, width uint32) {
		if !buttonReadStepActive || !newGamePressed {
			return
		}
		descriptor, known := m.CPU.Descriptors[selector]
		_, _, callbackActive, _, _ := services.MouseCallbackState()
		callbackIndex := 0
		if callbackActive {
			callbackIndex = 1
		}
		buttonRequests[width][callbackIndex]++
		linear := uint64(descriptor.Base) + uint64(offset)
		valid := known && uint64(offset)+uint64(width) <= uint64(descriptor.Limit)+1 && linear+uint64(width) <= uint64(len(m.Mem))
		if valid && !callbackActive && buttonPositiveCounts[width] < 2 && !buttonPositivePending {
			buttonPositivePending, buttonPositiveWidth = true, width
			buttonPositiveSelector, buttonPositiveOffset, buttonPositiveLinear = selector, offset, linear
			buttonPositiveBytes = [2]byte{}
			copy(buttonPositiveBytes[:width], m.Mem[linear:linear+uint64(width)])
		}
		eventTarget := linear <= 0x2a1229 && linear+uint64(width) > 0x2a121a || linear <= 0x2a11ef && linear+uint64(width) > 0x2a11ec
		if valid && !callbackActive && buttonReadSamples < 32 && eventTarget {
			buttonReadMatched, buttonReadWidth = true, width
			copy(buttonReadWord[:], m.Mem[0x2a121a:0x2a121c])
			eventReadSelector, eventReadOffset, eventReadLinear = selector, offset, linear
			copy(eventReadWindow[:], m.Mem[0x2a121a:0x2a122a])
			copy(eventReadCounter[:], m.Mem[0x2a11ec:0x2a11f0])
		}
	}
	defer func() {
		if newGameClick {
			mask, pending, active, started, completed := services.MouseCallbackState()
			fmt.Printf("new_game_mouse_terminal pressed=%t released=%t samples=%d mask=%X pending=%d active=%t started=%d completed=%d\n", newGamePressed, newGameReleased, newGameCallbackSamples, mask, pending, active, started, completed)
			hook8Matches := m.CPU.SegmentRead8 != nil && reflect.ValueOf(m.CPU.SegmentRead8).Pointer() == buttonHook8
			hook16Matches := m.CPU.SegmentRead16 != nil && reflect.ValueOf(m.CPU.SegmentRead16).Pointer() == buttonHook16
			fmt.Printf("new_game_button_read_control normal8=%d callback8=%d normal16=%d callback16=%d positives8=%d positives16=%d target_samples=%d hook_code_matches=%t/%t\n", buttonRequests[1][0], buttonRequests[1][1], buttonRequests[2][0], buttonRequests[2][1], buttonPositiveCounts[1], buttonPositiveCounts[2], buttonReadSamples, hook8Matches, hook16Matches)
		}
	}()

	// 規格326：只有正常點擊與明示圖像輸出才觀測後段，不改CPU或輸入。
	postClickVisits := make(map[uint32]uint64)
	postClickSteps := 0
	dumpPostClickProgress := func(step int) {
		if phasePrefix == "" || !newGameClick || !newGamePressed || !newGameReleased || step < 49500000 || step > 50000000 || step%100000 != 0 {
			return
		}
		c := m.CPU
		r, seg, eip, flags := c.R, c.Seg, c.EIP, c.EFlags
		control, status, stack, depth := c.FPUControl, c.FPUStatus, c.FPUStack, c.FPUDepth
		state, ramBefore := m.VBEState(), sha256.Sum256(m.Mem)
		var instruction [16]byte
		if uint64(eip)+16 <= uint64(len(m.Mem)) {
			copy(instruction[:], m.Mem[eip:eip+16])
		}
		readWindow := func(selector uint16, offset uint32, out []byte) bool {
			desc, known := c.Descriptors[selector]
			linear := uint64(desc.Base) + uint64(offset)
			if !known || uint64(offset)+uint64(len(out)) > uint64(desc.Limit)+1 || linear+uint64(len(out)) > uint64(len(m.Mem)) {
				return false
			}
			copy(out, m.Mem[linear:linear+uint64(len(out))])
			return true
		}
		var rawStack [64]byte
		var rawESI [16]byte
		stackOffset := r[cpu386.EBP] - 0x38
		stackReadable := r[cpu386.EBP] >= 0x38 && readWindow(seg[cpu386.SegSS], stackOffset, rawStack[:])
		esiReadable := readWindow(seg[cpu386.SegDS], r[cpu386.ESI], rawESI[:])
		pixels, rgb := m.VBEIndexed(), m.VBERGB()
		if len(pixels) != 640*480 || len(rgb) != 640*480*3 {
			panic("後段VBE快照尺寸錯誤")
		}
		out := image.NewNRGBA(image.Rect(0, 0, 640, 480))
		for pixel := range pixels {
			copy(out.Pix[pixel*4:pixel*4+3], rgb[pixel*3:pixel*3+3])
			out.Pix[pixel*4+3] = 255
		}
		path := fmt.Sprintf("%s-post-click-%08d.png", phasePrefix, step)
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
		encoded, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}
		ramAfter := sha256.Sum256(m.Mem)
		readonly := c.R == r && c.Seg == seg && c.EIP == eip && c.EFlags == flags && c.FPUControl == control && c.FPUStatus == status && c.FPUDepth == depth && m.VBEState() == state && ramBefore == ramAfter
		for j := range stack {
			readonly = readonly && math.Float64bits(stack[j]) == math.Float64bits(c.FPUStack[j])
		}
		if !readonly {
			panic("後段進度觀測改變原始狀態")
		}
		type frequency struct {
			address uint32
			count   uint64
		}
		hits := make([]frequency, 0, len(postClickVisits))
		for address, count := range postClickVisits {
			hits = append(hits, frequency{address, count})
		}
		sort.Slice(hits, func(i, j int) bool {
			if hits[i].count != hits[j].count {
				return hits[i].count > hits[j].count
			}
			return hits[i].address < hits[j].address
		})
		hot := make([]string, 0, min(10, len(hits)))
		for _, hit := range hits[:min(10, len(hits))] {
			hot = append(hot, fmt.Sprintf("%X:%d", hit.address, hit.count))
		}
		micros := uint64(0)
		if ports, ok := services.DPMI.RealModeIO.(*machine.LEOPLPorts); ok {
			micros = ports.BIOSClock.Micros
		}
		fmt.Printf("post_click_progress outer_step=%d address_space=dosgolem_high_le eip=%X instruction_bytes=%X r=%X seg=%X flags=%X virtual_micros=%d state=%+v stack_selector=%X stack_offset=%X stack_readable=%t raw_stack=%X esi_selector=%X esi_offset=%X esi_readable=%t raw_esi=%X observed_steps=%d unique_sites=%d hot_sites=[%s] indexed_sha256=%x rgb_sha256=%x png_sha256=%x ram_before_sha256=%x ram_after_sha256=%x path=%s readonly=%t\n", step, eip, instruction, r, seg, flags, micros, state, seg[cpu386.SegSS], stackOffset, stackReadable, rawStack, seg[cpu386.SegDS], r[cpu386.ESI], esiReadable, rawESI, postClickSteps, len(hits), strings.Join(hot, " "), sha256.Sum256(pixels), sha256.Sum256(rgb), sha256.Sum256(encoded), ramBefore, ramAfter, path, readonly)
		clear(postClickVisits)
		postClickSteps = 0
	}

	for i := 0; i < maxSteps; i++ {
		loopStep = i
		dumpPostClickProgress(i)
		if newGameClick && menuDisplay40Seen {
			ports, ok := services.DPMI.RealModeIO.(*machine.LEOPLPorts)
			mask, pending, active, started, completed := services.MouseCallbackState()
			selector, offset := services.MouseCallbackTarget()
			if ok && ports.BIOSClock != nil && m.CPU.EFlags&cpu386.IF != 0 && pending == 0 && !active {
				phase, x, buttons := "", uint16(1000), uint16(1)
				if !newGamePressed && selector == 8 && offset == 0x2136d1 && mask&3 == 3 {
					phase = "press"
				} else if newGamePressed && !newGameReleased && completed >= 1 && ports.BIOSClock.Micros >= newGamePressMicros+20000 && mask&1 != 0 {
					phase, x, buttons = "release", 1002, 0
				}
				if phase != "" {
					if err := services.InjectMouseEvent(x, 229, buttons, 0, 0); err != nil {
						panic(err)
					}
					fmt.Printf("new_game_mouse_input phase=%s outer_step=%d virtual_micros=%d x=%d y=229 buttons=%d delta=0/0 mask=%X target=%X:%X started=%d completed=%d eip=%X r=%X seg=%X flags=%X\n", phase, i, ports.BIOSClock.Micros, x, buttons, mask, selector, offset, started, completed, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags)
					if phase == "press" {
						newGamePressed, newGamePressMicros = true, ports.BIOSClock.Micros
						// 規格319：初始化完成後接現存鏈，保留普通RAM的fallback。
						original8, original16 := m.CPU.SegmentRead8, m.CPU.SegmentRead16
						m.CPU.SegmentRead8 = func(selector uint16, offset uint32) (uint8, bool) {
							var value uint8
							var ok bool
							if original8 != nil {
								value, ok = original8(selector, offset)
							}
							observeButtonRequest(selector, offset, 1)
							return value, ok
						}
						m.CPU.SegmentRead16 = func(selector uint16, offset uint32) (uint16, bool) {
							var value uint16
							var ok bool
							if original16 != nil {
								value, ok = original16(selector, offset)
							}
							observeButtonRequest(selector, offset, 2)
							return value, ok
						}
						buttonHook8, buttonHook16 = reflect.ValueOf(m.CPU.SegmentRead8).Pointer(), reflect.ValueOf(m.CPU.SegmentRead16).Pointer()
					} else {
						newGameReleased = true
					}
				}
			}
		}
		if i == 0 || i == 42347255 || i == 42603292 || i == 48000000 {
			dumpPlatform("sample", i)
		}
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
		if (m.CPU.EIP == 0x257662 || m.CPU.EIP == 0x257665 || m.CPU.EIP == 0x25766a || m.CPU.EIP == 0x25766d || m.CPU.EIP == 0x257670) && seen[m.CPU.EIP] <= 3 {
			fmt.Printf("dword_ror_state step=%d address_space=dosgolem_high_le eip=0x%X r=%X seg=%X flags=0x%X bytes=% X\n", i, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags, m.Mem[m.CPU.EIP:m.CPU.EIP+16])
		}
		if m.CPU.EIP == 0x2454b0 || m.CPU.EIP == 0x2454b3 || m.CPU.EIP == 0x2454b6 || m.CPU.EIP == 0x2454b8 || m.CPU.EIP == 0x2454e7 {
			if ports, ok := services.DPMI.RealModeIO.(*machine.LEOPLPorts); ok && ports.State().DSPAuto8Commands > 0 && c6CallerSeen[m.CPU.EIP] < 3 {
				c6CallerSeen[m.CPU.EIP]++
				fmt.Printf("sb16_c6_caller step=%d address_space=dosgolem_high_le eip=0x%X r=%X seg=%X flags=0x%X device=%+v\n", i, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags, ports.State())
			}
		}
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
			copy(ring, ring[1:])
			ring = ring[:len(ring)-1]
		}
		ring = append(ring, sample{i, m.CPU.EIP, m.CPU.R[cpu386.ESP], m.CPU.R[cpu386.ESI], m.CPU.R[cpu386.EAX]})
		if i < 24 {
			v, _ := m.Read16(0x21996)
			fmt.Printf("trace step=%d eip=0x%X edx=0x%X flags=0x%X timer_word=0x%X\n", i, m.CPU.EIP, m.CPU.R[cpu386.EDX], m.CPU.EFlags, v)
		}
		if (m.CPU.EIP == 0x23c36b || m.CPU.EIP == 0x23c371 || m.CPU.EIP == 0x23c398) && seen[m.CPU.EIP] <= 3 {
			addr := m.CPU.R[cpu386.ESI] + 0x384
			if !orReads.armed && m.CPU.EIP == 0x23c36b {
				if descriptor, ok := m.CPU.Descriptors[m.CPU.Seg[cpu386.SegDS]]; ok {
					orReads.target, orReads.linear, orReads.selector, orReads.armed = addr, descriptor.Base+addr, m.CPU.Seg[cpu386.SegDS], true
					originalRead := m.CPU.SegmentRead8
					m.CPU.SegmentRead8 = func(selector uint16, offset uint32) (uint8, bool) {
						// 排除未變的高兩byte；觀察低兩byte／完整dword的取址端。
						if orReads.active && selector == orReads.selector && offset >= orReads.target && offset-orReads.target < 2 {
							orReads.matched = true
						}
						if originalRead != nil {
							return originalRead(selector, offset)
						}
						return 0, false
					}
				}
			}
			var value uint32
			readable := true
			for j := uint32(0); j < 4; j++ {
				b, ok := m.CPU.ReadSegment8(m.CPU.Seg[cpu386.SegDS], addr+j)
				value |= uint32(b) << (j * 8)
				readable = readable && ok
			}
			code := make([]byte, 64)
			for j := range code {
				code[j], _ = m.CPU.ReadSegment8(m.CPU.Seg[cpu386.SegCS], 0x23c398+uint32(j))
			}
			fmt.Printf("or_dword_memory_state step=%d address_space=dosgolem_high_le eip=0x%X r=%X seg=%X flags=0x%X destination_ds_offset=0x%X dword=%08X readable=%t next_code_23C398=%X\n", i, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags, addr, value, readable, code)
		}
		if (m.CPU.EIP == 0x25489c || m.CPU.EIP == 0x25489f || m.CPU.EIP == 0x2548a2) && seen[m.CPU.EIP] <= 3 {
			fmt.Printf("xor_dword_imm8_state step=%d address_space=dosgolem_high_le eip=0x%X r=%X seg=%X flags=0x%X\n", i, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags)
		}
		if (m.CPU.EIP == 0x2548a2 || m.CPU.EIP == 0x2548a5 || m.CPU.EIP == 0x2548a7 || m.CPU.EIP == 0x2548ae) && seen[m.CPU.EIP] <= 3 {
			low, lowOK := m.CPU.ReadSegment8(m.CPU.Seg[cpu386.SegDS], 0x2726d0)
			high, highOK := m.CPU.ReadSegment8(m.CPU.Seg[cpu386.SegDS], 0x2726d1)
			fmt.Printf("bsf_dword_state step=%d address_space=dosgolem_high_le eip=0x%X r=%X seg=%X flags=0x%X stored_ds_offset=0x2726D0 word=%04X readable=%t\n", i, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags, uint16(low)|uint16(high)<<8, lowOK && highOK)
		}
		if (m.CPU.EIP == 0x25488f || m.CPU.EIP == 0x254891 || m.CPU.EIP == 0x254894 || m.CPU.EIP == 0x254896) && seen[m.CPU.EIP] <= 3 {
			addr := m.CPU.R[cpu386.EDI]
			var value uint32
			readable := true
			for j := uint32(0); j < 4; j++ {
				part, ok := m.CPU.ReadSegment8(m.CPU.Seg[cpu386.SegES], addr+j)
				value |= uint32(part) << (8 * j)
				readable = readable && ok
			}
			fmt.Printf("scasd_state step=%d address_space=dosgolem_high_le eip=0x%X r=%X seg=%X flags=0x%X operand_es_offset=0x%X dword=%08X readable=%t\n", i, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags, addr, value, readable)
			if m.CPU.EIP == 0x25488f {
				count := m.CPU.R[cpu386.ECX]
				if count > 2048 {
					count = 2048
				}
				data := make([]byte, 0, count*4)
				complete := true
				for j := uint32(0); j < count*4; j++ {
					part, ok := m.CPU.ReadSegment8(m.CPU.Seg[cpu386.SegES], addr+j)
					data = append(data, part)
					complete = complete && ok
				}
				fmt.Printf("scasd_memory step=%d es=0x%X offset=0x%X dwords=%d readable=%t hex=%X\n", i, m.CPU.Seg[cpu386.SegES], addr, count, complete, data)
			}
		}
		if (m.CPU.EIP == 0x254a04 || m.CPU.EIP == 0x254a06 || m.CPU.EIP == 0x254a09 || m.CPU.EIP == 0x254a0c) && seen[m.CPU.EIP] <= 3 {
			addr := m.CPU.R[cpu386.EDI] + m.CPU.R[cpu386.EDX]
			value, readable := m.CPU.ReadSegment8(m.CPU.Seg[cpu386.SegDS], addr)
			fmt.Printf("byte_shl_cl_state step=%d address_space=dosgolem_high_le eip=0x%X r=%X seg=%X flags=0x%X operand_ds_offset=0x%X byte=%02X readable=%t\n", i, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags, addr, value, readable)
		}
		if (m.CPU.EIP == 0x2545ef || m.CPU.EIP == 0x2545f1 || m.CPU.EIP == 0x2545f3) && seen[m.CPU.EIP] <= 3 {
			fmt.Printf("byte_neg_state step=%d address_space=dosgolem_high_le eip=0x%X r=%X seg=%X flags=0x%X\n", i, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags)
		}
		if (m.CPU.EIP == 0x254510 || m.CPU.EIP == 0x254513 || m.CPU.EIP == 0x25451a) && seen[m.CPU.EIP] <= 3 {
			fmt.Printf("dword_rol_state step=%d address_space=dosgolem_high_le eip=0x%X r=%X seg=%X flags=0x%X\n", i, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags)
		}
		if (m.CPU.EIP == 0x254499 || m.CPU.EIP == 0x25449f || m.CPU.EIP == 0x2544a1 || m.CPU.EIP == 0x2544cb) && seen[m.CPU.EIP] <= 3 {
			fmt.Printf("dword_test_state step=%d address_space=dosgolem_high_le eip=0x%X r=%X seg=%X flags=0x%X\n", i, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags)
		}
		if (m.CPU.EIP == 0x254275 || m.CPU.EIP == 0x254278) && seen[m.CPU.EIP] <= 3 {
			fmt.Printf("byte_xor_state step=%d address_space=dosgolem_high_le eip=0x%X r=%X seg=%X flags=0x%X\n", i, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags)
		}
		if (m.CPU.EIP == 0x25425f || m.CPU.EIP == 0x254266) && seen[m.CPU.EIP] <= 3 {
			value, readable := m.CPU.ReadSegment8(m.CPU.Seg[cpu386.SegDS], 0x2726c0)
			fmt.Printf("byte_add_state step=%d address_space=dosgolem_high_le eip=0x%X r=%X seg=%X flags=0x%X operand_ds_offset=0x2726C0 byte=%02X readable=%t\n", i, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags, value, readable)
		}
		if (m.CPU.EIP == 0x254249 || m.CPU.EIP == 0x254250) && seen[m.CPU.EIP] <= 3 {
			value, readable := m.CPU.ReadSegment8(m.CPU.Seg[cpu386.SegDS], 0x2726c0)
			fmt.Printf("byte_sub_state step=%d address_space=dosgolem_high_le eip=0x%X r=%X seg=%X flags=0x%X operand_ds_offset=0x2726C0 byte=%02X readable=%t\n", i, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags, value, readable)
		}
		if (m.CPU.EIP == 0x239b3a || m.CPU.EIP == 0x239b3c || m.CPU.EIP == 0x239b3e || m.CPU.EIP == 0x239b40 || m.CPU.EIP == 0x239b42 || m.CPU.EIP == 0x239b44 || m.CPU.EIP == 0x239b46) && seen[m.CPU.EIP] <= 3 {
			if ports, ok := services.DPMI.RealModeIO.(*machine.LEOPLPorts); ok {
				fmt.Printf("pit_count_latch_state step=%d eip=0x%X r=%X seg=%X flags=0x%X pit=%+v clock=%+v\n", i, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags, ports.PIT0, ports.BIOSClock)
			}
		}
		if (m.CPU.EIP == 0x239ae8 || m.CPU.EIP == 0x239aea || m.CPU.EIP == 0x239af0 || m.CPU.EIP == 0x239af6) && seen[m.CPU.EIP] <= 3 {
			if ports, ok := services.DPMI.RealModeIO.(*machine.LEOPLPorts); ok {
				fmt.Printf("pit_mode2_state step=%d eip=0x%X r=%X seg=%X flags=0x%X pit=%+v clock=%+v\n", i, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags, ports.PIT0, ports.BIOSClock)
			}
		}
		if (m.CPU.EIP == 0x239a47 || m.CPU.EIP == 0x239a49) && seen[m.CPU.EIP] <= 3 {
			fmt.Printf("vtd_entry_state step=%d eip=0x%X r=%X seg=%X flags=0x%X\n", i, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags)
		}
		if m.CPU.EIP == 0x239a42 && seen[m.CPU.EIP] <= 3 {
			fmt.Printf("xor_word_input step=%d eip=0x%X r=%X seg=%X flags=0x%X\n", i, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags)
		}
		if (m.CPU.EIP == 0x21c2d6 || m.CPU.EIP == 0x21c2da) && seen[m.CPU.EIP] <= 3 {
			fmt.Printf("sign_branch_input step=%d eip=0x%X r=%X seg=%X flags=0x%X\n", i, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags)
		}
		orReads.active = orReads.armed && orReads.remaining > 0 && m.CPU.EIP != 0x23c36b
		orReads.matched = false
		var readEIP, readFlags uint32
		var readR [8]uint32
		var readSeg [6]uint16
		if orReads.active {
			readEIP, readR, readSeg, readFlags = m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags
		}
		observeXORAL := m.CPU.EIP == 0x247be1 && xorALSeen < 3
		observeXORConsumer := !observeXORAL && xorALConsumerSteps > 0
		xorBeforeEIP, xorBeforeR, xorBeforeSeg, xorBeforeFlags := m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags
		var xorStack [4]byte
		var xorInstructionBytes []byte
		xorStackReadable := true
		if observeXORAL || observeXORConsumer {
			if uint64(xorBeforeEIP)+8 <= uint64(len(m.Mem)) {
				xorInstructionBytes = append([]byte(nil), m.Mem[xorBeforeEIP:xorBeforeEIP+8]...)
			}
			for j := range xorStack {
				v, ok := m.CPU.ReadSegment8(m.CPU.Seg[cpu386.SegSS], m.CPU.R[cpu386.ESP]+uint32(j))
				xorStack[j], xorStackReadable = v, xorStackReadable && ok
			}
		}
		if keyboardRequested && !keyboardQueued && i == keyboardStep {
			for _, scan := range []byte{1, 0x81} {
				if err := services.QueueHardwareScan(scan); err != nil {
					panic(err)
				}
			}
			keyboardQueued = true
			fmt.Printf("hardware_keyboard_input outer_step=%d scan=01/81 source=controller_queue eip=%X r=%X seg=%X flags=%X\n", i, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags)
		}
		observeDMA8Consumer := dma8ConsumerSteps > 0
		var dma8BeforeR [8]uint32
		var dma8BeforeSeg [6]uint16
		var dma8BeforeEIP, dma8BeforeFlags uint32
		var dma8InstructionBytes [16]byte
		if observeDMA8Consumer {
			dma8BeforeR, dma8BeforeSeg, dma8BeforeEIP, dma8BeforeFlags = m.CPU.R, m.CPU.Seg, m.CPU.EIP, m.CPU.EFlags
			copy(dma8InstructionBytes[:], m.Mem[dma8BeforeEIP:dma8BeforeEIP+16])
		}

		observeCalendarConsumer := calendarConsumerSteps > 0
		var calendarBeforeR [8]uint32
		var calendarBeforeSeg [6]uint16
		var calendarBeforeEIP, calendarBeforeFlags uint32
		var calendarBytes [16]byte
		var calendarStackBefore, calendarStackAfter [32]byte
		calendarStackReadable := true
		if observeCalendarConsumer {
			calendarBeforeR, calendarBeforeSeg, calendarBeforeEIP, calendarBeforeFlags = m.CPU.R, m.CPU.Seg, m.CPU.EIP, m.CPU.EFlags
			copy(calendarBytes[:], m.Mem[calendarBeforeEIP:calendarBeforeEIP+16])
			for j := range calendarStackBefore {
				b, ok := m.CPU.ReadSegment8(calendarBeforeSeg[cpu386.SegSS], calendarBeforeR[cpu386.ESP]+uint32(j))
				calendarStackBefore[j], calendarStackReadable = b, calendarStackReadable && ok
			}
		}
		observeWordAddMemory := m.CPU.EIP == 0x210c7e && wordAddMemorySeen < 2
		if observeWordAddMemory {
			wordAddMemorySeen++
			wordAddMemorySteps = 4
		}
		var wordAddBeforeR [8]uint32
		var wordAddBeforeSeg [6]uint16
		var wordAddBeforeEIP, wordAddBeforeFlags uint32
		var wordAddBytes [16]byte
		var wordAddBeforeMemory, wordAddAfterMemory [8]byte
		wordAddReadable := true
		if wordAddMemorySteps > 0 {
			wordAddBeforeR, wordAddBeforeSeg, wordAddBeforeEIP, wordAddBeforeFlags = m.CPU.R, m.CPU.Seg, m.CPU.EIP, m.CPU.EFlags
			copy(wordAddBytes[:], m.Mem[wordAddBeforeEIP:wordAddBeforeEIP+16])
			for j := range wordAddBeforeMemory {
				b, ok := m.CPU.ReadSegment8(wordAddBeforeSeg[cpu386.SegDS], 0x29bea0+uint32(j))
				wordAddBeforeMemory[j], wordAddReadable = b, wordAddReadable && ok
			}
		}
		if m.CPU.EIP == 0x14e3de {
			cmpWordTotal++
			if cmpWordSamples < 2 || (!cmpWordBoundarySeen && uint16(m.CPU.R[cpu386.ECX]) == 212) {
				cmpWordSamples++
				if uint16(m.CPU.R[cpu386.ECX]) == 212 {
					cmpWordBoundarySeen = true
				}
				cmpWordSteps = 2
			}
		}
		var cmpWordBeforeR [8]uint32
		var cmpWordBeforeSeg [6]uint16
		var cmpWordBeforeEIP, cmpWordBeforeFlags uint32
		var cmpWordBytes [16]byte
		var cmpWordStackBefore, cmpWordStackAfter [32]byte
		cmpWordStackReadable := true
		if cmpWordSteps > 0 {
			cmpWordBeforeR, cmpWordBeforeSeg, cmpWordBeforeEIP, cmpWordBeforeFlags = m.CPU.R, m.CPU.Seg, m.CPU.EIP, m.CPU.EFlags
			copy(cmpWordBytes[:], m.Mem[cmpWordBeforeEIP:cmpWordBeforeEIP+16])
			for j := range cmpWordStackBefore {
				b, ok := m.CPU.ReadSegment8(cmpWordBeforeSeg[cpu386.SegSS], cmpWordBeforeR[cpu386.ESP]+uint32(j))
				cmpWordStackBefore[j], cmpWordStackReadable = b, cmpWordStackReadable && ok
			}
		}
		mouseExchangeObserve := mouseExchangeSteps > 0
		mouseExchangeBefore := m.CPU.R
		mouseExchangeSeg := m.CPU.Seg
		mouseExchangeEIP, mouseExchangeFlags := m.CPU.EIP, m.CPU.EFlags
		var mouseExchangeBytes [16]byte
		var mouseExchangeStack [32]byte
		mouseExchangeReadable := true
		if mouseExchangeObserve {
			copy(mouseExchangeBytes[:], m.Mem[mouseExchangeEIP:mouseExchangeEIP+16])
			for j := range mouseExchangeStack {
				b, ok := m.CPU.ReadSegment8(mouseExchangeSeg[cpu386.SegSS], mouseExchangeBefore[cpu386.ESP]+uint32(j))
				mouseExchangeStack[j], mouseExchangeReadable = b, mouseExchangeReadable && ok
			}
		}
		_, clickPending, clickActive, clickStarted, clickCompleted := services.MouseCallbackState()
		observeClick := newGameClick && newGamePressed && newGameCallbackSamples < 256 && (clickPending != 0 || clickActive)
		var clickBeforeR [8]uint32
		var clickBeforeSeg [6]uint16
		var clickBeforeEIP, clickBeforeFlags uint32
		var clickBytes [16]byte
		observeEventConsumer := newGameClick && eventConsumerBudget > 0 && eventConsumerSamples < 32
		observeFindQuestion := findQuestionConsumerSteps > 0
		observeByteAdd := newGameClick && newGamePressed && byteAddSamples < 32 && (m.CPU.EIP == 0x17122b || m.CPU.EIP == 0x171231)
		observeByteAddConsumer := byteAddConsumerBudget > 0 && byteAddConsumerSamples < 32
		observeNegDword := newGameClick && newGamePressed && negDwordSamples < 8 && m.CPU.EIP == 0x2130f3
		observeNegDwordConsumer := negDwordConsumerBudget > 0 && negDwordConsumerSamples < 32
		if observeClick || observeEventConsumer || observeFindQuestion || observeByteAdd || observeByteAddConsumer || observeNegDword || observeNegDwordConsumer || newGameClick && newGamePressed && buttonReadSamples < 32 {
			clickBeforeR, clickBeforeSeg = m.CPU.R, m.CPU.Seg
			clickBeforeEIP, clickBeforeFlags = m.CPU.EIP, m.CPU.EFlags
			copy(clickBytes[:], m.Mem[clickBeforeEIP:clickBeforeEIP+16])
		}
		var byteAddBeforeWindow [5]byte
		var byteAddSelector uint16
		var byteAddOffset uint32
		byteAddWindowReadable := false
		if observeByteAdd && clickBeforeEIP == 0x171231 {
			byteAddSelector = clickBeforeSeg[cpu386.SegDS]
			byteAddOffset = clickBeforeR[cpu386.ESI] + clickBeforeR[cpu386.EAX]
			desc, known := m.CPU.Descriptors[byteAddSelector]
			linear := uint64(desc.Base) + uint64(byteAddOffset)
			if known && byteAddOffset >= 2 && uint64(byteAddOffset)+3 <= uint64(desc.Limit)+1 && linear+3 <= uint64(len(m.Mem)) {
				byteAddWindowReadable = true
				byteAddWatchLinear, byteAddWatchReadable = linear-2, true
				copy(byteAddBeforeWindow[:], m.Mem[linear-2:linear+3])
			}
		}
		var negDwordBeforeWindow [16]byte
		if observeNegDword {
			negDwordWatchSelector = clickBeforeSeg[cpu386.SegSS]
			negDwordWatchOffset = clickBeforeR[cpu386.EBP] - 0x28
			negDwordWatchReadable = false
			desc, known := m.CPU.Descriptors[negDwordWatchSelector]
			if known && clickBeforeR[cpu386.EBP] >= 0x2c {
				start := clickBeforeR[cpu386.EBP] - 0x2c
				linear := uint64(desc.Base) + uint64(start)
				if uint64(start)+16 <= uint64(desc.Limit)+1 && linear+16 <= uint64(len(m.Mem)) {
					negDwordWatchLinear, negDwordWatchReadable = linear, true
				}
			}
		}
		if (observeNegDword || observeNegDwordConsumer) && negDwordWatchReadable {
			copy(negDwordBeforeWindow[:], m.Mem[negDwordWatchLinear:negDwordWatchLinear+16])
		}
		buttonReadStepActive = newGameClick
		buttonReadMatched = false
		buttonPositivePending = false
		if phasePrefix != "" && newGameClick && newGameReleased && i >= 49500000 {
			postClickVisits[m.CPU.EIP]++
			postClickSteps++
		}
		stepErr := m.CPU.Step()
		buttonReadStepActive = false
		if observeNegDword || observeNegDwordConsumer {
			var after [16]byte
			if negDwordWatchReadable {
				copy(after[:], m.Mem[negDwordWatchLinear:negDwordWatchLinear+16])
			}
			label := "neg_dword_consumer"
			if observeNegDword {
				label = "neg_dword_step"
				negDwordSamples++
			} else {
				negDwordConsumerSamples++
				negDwordConsumerBudget--
			}
			fmt.Printf("%s outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X selector=%X offset=%X window_linear=%X window_readable=%t before_window=%X after_window=%X error=%v\n", label, i, clickBeforeEIP, m.CPU.EIP, clickBeforeR, m.CPU.R, clickBeforeSeg, m.CPU.Seg, clickBeforeFlags, m.CPU.EFlags, clickBytes, negDwordWatchSelector, negDwordWatchOffset, negDwordWatchLinear, negDwordWatchReadable, negDwordBeforeWindow, after, stepErr)
			if observeNegDword && stepErr == nil {
				negDwordConsumerBudget = 3
			}
		}
		if observeByteAddConsumer {
			byteAddConsumerSamples++
			byteAddConsumerBudget--
			var window [5]byte
			if byteAddWatchReadable {
				copy(window[:], m.Mem[byteAddWatchLinear:byteAddWatchLinear+5])
			}
			fmt.Printf("byte_add_consumer outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X window_readable=%t raw_window=%X error=%v\n", i, clickBeforeEIP, m.CPU.EIP, clickBeforeR, m.CPU.R, clickBeforeSeg, m.CPU.Seg, clickBeforeFlags, m.CPU.EFlags, clickBytes, byteAddWatchReadable, window, stepErr)
		}
		if observeByteAdd {
			byteAddSamples++
			var after [5]byte
			if byteAddWindowReadable {
				copy(after[:], m.Mem[byteAddWatchLinear:byteAddWatchLinear+5])
			}
			fmt.Printf("byte_add_step outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X selector=%X offset=%X window_readable=%t before_window=%X after_window=%X error=%v\n", i, clickBeforeEIP, m.CPU.EIP, clickBeforeR, m.CPU.R, clickBeforeSeg, m.CPU.Seg, clickBeforeFlags, m.CPU.EFlags, clickBytes, byteAddSelector, byteAddOffset, byteAddWindowReadable, byteAddBeforeWindow, after, stepErr)
			if stepErr == nil {
				byteAddConsumerBudget = 3
			}
		}
		if observeFindQuestion {
			findQuestionConsumerSteps--
			fmt.Printf("find_question_consumer outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X error=%v\n", i, clickBeforeEIP, m.CPU.EIP, clickBeforeR, m.CPU.R, clickBeforeSeg, m.CPU.Seg, clickBeforeFlags, m.CPU.EFlags, clickBytes, stepErr)
		}
		if buttonPositivePending {
			buttonPositiveCounts[buttonPositiveWidth]++
			fmt.Printf("new_game_button_read_positive outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X width=%d selector=%X offset=%X linear=%X input_bytes=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X error=%v\n", i, clickBeforeEIP, m.CPU.EIP, buttonPositiveWidth, buttonPositiveSelector, buttonPositiveOffset, buttonPositiveLinear, buttonPositiveBytes[:buttonPositiveWidth], clickBeforeR, m.CPU.R, clickBeforeSeg, m.CPU.Seg, clickBeforeFlags, m.CPU.EFlags, clickBytes, stepErr)
		}
		if buttonReadMatched {
			buttonReadSamples++
			fmt.Printf("new_game_button_read outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X width=%d buttons_word=%X released=%t before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X error=%v\n", i, clickBeforeEIP, m.CPU.EIP, buttonReadWidth, buttonReadWord, newGameReleased, clickBeforeR, m.CPU.R, clickBeforeSeg, m.CPU.Seg, clickBeforeFlags, m.CPU.EFlags, clickBytes, stepErr)
			fmt.Printf("new_game_event_read outer_step=%d selector=%X offset=%X linear=%X width=%d before_2a121a=%X after_2a121a=%X before_2a11ec=%X after_2a11ec=%X\n", i, eventReadSelector, eventReadOffset, eventReadLinear, buttonReadWidth, eventReadWindow, m.Mem[0x2a121a:0x2a122a], eventReadCounter, m.Mem[0x2a11ec:0x2a11f0])
			eventConsumerBudget = 4
		}
		if observeEventConsumer {
			eventConsumerSamples++
			eventConsumerBudget--
			fmt.Printf("new_game_event_consumer outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X raw_2a121a=%X raw_2a11ec=%X error=%v\n", i, clickBeforeEIP, m.CPU.EIP, clickBeforeR, m.CPU.R, clickBeforeSeg, m.CPU.Seg, clickBeforeFlags, m.CPU.EFlags, clickBytes, m.Mem[0x2a121a:0x2a122a], m.Mem[0x2a11ec:0x2a11f0], stepErr)
		}
		if observeClick {
			newGameCallbackSamples++
			mask, pending, active, started, completed := services.MouseCallbackState()
			fmt.Printf("new_game_mouse_callback outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X before_started=%d before_completed=%d mask=%X pending=%d active=%t started=%d completed=%d error=%v\n", i, clickBeforeEIP, m.CPU.EIP, clickBeforeR, m.CPU.R, clickBeforeSeg, m.CPU.Seg, clickBeforeFlags, m.CPU.EFlags, clickBytes, clickStarted, clickCompleted, mask, pending, active, started, completed, stepErr)
			if completed != clickCompleted {
				fmt.Printf("new_game_button_callback_return outer_step=%d completed=%d buttons_word=%X\n", i, completed, m.Mem[0x2a121a:0x2a121c])
				fmt.Printf("new_game_event_callback_return outer_step=%d completed=%d raw_2a121a=%X raw_2a11ec=%X\n", i, completed, m.Mem[0x2a121a:0x2a122a], m.Mem[0x2a11ec:0x2a11f0])
			}
		}
		if mouseExchangeObserve {
			mouseExchangeSteps--
			fmt.Printf("mouse_exchange_caller outer_step=%d input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X stack=%X stack_readable=%t error=%v\n", i, mouseExchangeEIP, m.CPU.EIP, mouseExchangeBefore, m.CPU.R, mouseExchangeSeg, m.CPU.Seg, mouseExchangeFlags, m.CPU.EFlags, mouseExchangeBytes, mouseExchangeStack, mouseExchangeReadable, stepErr)
		}
		if cmpWordSteps > 0 {
			cmpWordSteps--
			for j := range cmpWordStackAfter {
				b, ok := m.CPU.ReadSegment8(cmpWordBeforeSeg[cpu386.SegSS], cmpWordBeforeR[cpu386.ESP]+uint32(j))
				cmpWordStackAfter[j], cmpWordStackReadable = b, cmpWordStackReadable && ok
			}
			fmt.Printf("cmp_word_immediate_observation outer_step=%d input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X stack_selector=%X stack_offset=%X stack_readable=%t before_stack=%X after_stack=%X error=%v\n", i, cmpWordBeforeEIP, m.CPU.EIP, cmpWordBeforeR, m.CPU.R, cmpWordBeforeSeg, m.CPU.Seg, cmpWordBeforeFlags, m.CPU.EFlags, cmpWordBytes, cmpWordBeforeSeg[cpu386.SegSS], cmpWordBeforeR[cpu386.ESP], cmpWordStackReadable, cmpWordStackBefore, cmpWordStackAfter, stepErr)
		}

		if wordAddMemorySteps > 0 {
			wordAddMemorySteps--
			for j := range wordAddAfterMemory {
				b, ok := m.CPU.ReadSegment8(wordAddBeforeSeg[cpu386.SegDS], 0x29bea0+uint32(j))
				wordAddAfterMemory[j], wordAddReadable = b, wordAddReadable && ok
			}
			fmt.Printf("word_add_memory_observation outer_step=%d input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X memory_selector=%X memory_offset=29BEA0 memory_readable=%t before_memory=%X after_memory=%X error=%v\n", i, wordAddBeforeEIP, m.CPU.EIP, wordAddBeforeR, m.CPU.R, wordAddBeforeSeg, m.CPU.Seg, wordAddBeforeFlags, m.CPU.EFlags, wordAddBytes, wordAddBeforeSeg[cpu386.SegDS], wordAddReadable, wordAddBeforeMemory, wordAddAfterMemory, stepErr)
		}

		if observeCalendarConsumer {
			calendarConsumerSteps--
			for j := range calendarStackAfter {
				b, ok := m.CPU.ReadSegment8(calendarBeforeSeg[cpu386.SegSS], calendarBeforeR[cpu386.ESP]+uint32(j))
				calendarStackAfter[j], calendarStackReadable = b, calendarStackReadable && ok
			}
			fmt.Printf("calendar_date_caller outer_step=%d input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X stack_selector=%X stack_offset=%X stack_readable=%t before_stack=%X after_stack=%X error=%v\n", i, calendarBeforeEIP, m.CPU.EIP, calendarBeforeR, m.CPU.R, calendarBeforeSeg, m.CPU.Seg, calendarBeforeFlags, m.CPU.EFlags, calendarBytes, calendarBeforeSeg[cpu386.SegSS], calendarBeforeR[cpu386.ESP], calendarStackReadable, calendarStackBefore, calendarStackAfter, stepErr)
		}

		if observeDMA8Consumer {
			dma8ConsumerSteps--
			fmt.Printf("dma8_control_caller outer_step=%d input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X error=%v\n", i, dma8BeforeEIP, m.CPU.EIP, dma8BeforeR, m.CPU.R, dma8BeforeSeg, m.CPU.Seg, dma8BeforeFlags, m.CPU.EFlags, dma8InstructionBytes, stepErr)
		}

		if ports, ok := services.DPMI.RealModeIO.(*machine.LEOPLPorts); ok && ports.DMA8PauseCommands+ports.DMA8ResumeCommands != dma8ControlsPrinted {
			dma8ControlsPrinted = ports.DMA8PauseCommands + ports.DMA8ResumeCommands
			dma8ConsumerSteps = 3
			fmt.Printf("dma8_control_return outer_step=%d pause_commands=%d resume_commands=%d after_eip=%X r=%X seg=%X flags=%X last=%+v error=%v\n", i, ports.DMA8PauseCommands, ports.DMA8ResumeCommands, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags, ports.DMA8ControlLast, stepErr)
			dumpPlatform("dma8_control_return", i)
		}

		if _, _, _, _, _, _, started, completed, last := services.HardwareIRQ1State(); last != nil && started != keyboardPrinted {
			keyboardPrinted = started
			fmt.Printf("hardware_keyboard_return outer_step=%d started=%d completed=%d after_eip=%X r=%X seg=%X flags=%X last=%+v raw_2a42ac=%X error=%v\n", i, started, completed, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags, last, m.Mem[0x2a42ac:0x2a42ec], stepErr)
		}
		if observeXORAL || observeXORConsumer {
			phase := "consumer"
			if observeXORAL {
				phase = "xor"
				xorALSeen++
				xorALConsumerSteps = 3
			} else {
				xorALConsumerSteps--
			}
			fmt.Printf("xor_al_immediate_observation phase=%s step=%d address_space=dosgolem_high_le input_eip=0x%X after_eip=0x%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=0x%X after_flags=0x%X input_stack=%X stack_readable=%t instruction_bytes=%X error=%v\n", phase, i, xorBeforeEIP, m.CPU.EIP, xorBeforeR, m.CPU.R, xorBeforeSeg, m.CPU.Seg, xorBeforeFlags, m.CPU.EFlags, xorStack, xorStackReadable, xorInstructionBytes, stepErr)
		}
		if ports, ok := services.DPMI.RealModeIO.(*machine.LEOPLPorts); ok && !irq7FirstPrinted && ports.IRQ7Returns != 0 {
			irq7FirstPrinted = true
			dumpPlatform("first_irq7_return", i)
		}
		orReads.active = false
		if orReads.matched {
			orReads.remaining--
			fmt.Printf("or_dword_memory_consumer step=%d address_space=dosgolem_high_le input_eip=0x%X after_eip=0x%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=0x%X after_flags=0x%X destination_linear=0x%X dword_bytes=%X instruction_bytes=%X error=%v\n", i, readEIP, m.CPU.EIP, readR, m.CPU.R, readSeg, m.CPU.Seg, readFlags, m.CPU.EFlags, orReads.linear, m.Mem[orReads.linear:orReads.linear+4], m.Mem[readEIP:readEIP+16], stepErr)
		}
		if err := stepErr; err != nil {
			dumpPlatform("stop", i)
			if ports, ok := services.DPMI.RealModeIO.(*machine.LEOPLPorts); ok {
				fmt.Printf("protected_dma_pcm bytes=%d sha256=%x prefix=%X\n", len(ports.PCM), sha256.Sum256(ports.PCM), ports.PCM[:min(16, len(ports.PCM))])
				if vector, readErr := m.Read32(0x0f * 4); readErr == nil && vector != 0 {
					address := uint32(uint16(vector>>16))*16 + uint32(uint16(vector))
					if uint64(address)+16 <= uint64(len(m.Mem)) {
						fmt.Printf("irq7_real_entry address_space=dosgolem_real_mode_linear vector=%08X linear=0x%X bytes=%X\n", vector, address, m.Mem[address:address+16])
					}
				}
			}
			var instructionError *cpu386.Error
			if errors.As(err, &instructionError) && uint64(instructionError.EIP)+16 <= uint64(len(m.Mem)) {
				fmt.Printf("guest_cpu_stop address_space=dosgolem_high_le eip=0x%X bytes=% X\n", instructionError.EIP, m.Mem[instructionError.EIP:instructionError.EIP+16])
			}
			mask, pending, active, started, completed := services.MouseCallbackState()
			fmt.Printf("mouse_callback mask=%X pending=%d active=%t started=%d completed=%d\n", mask, pending, active, started, completed)
			fmt.Printf("step_error step=%d eip=0x%X eax=0x%X ebx=0x%X ecx=0x%X edx=0x%X es=0x%X ds=0x%X ss=0x%X flags=0x%X dos_calls=%d error=%v\n", i, m.CPU.EIP, m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], m.CPU.R[cpu386.ECX], m.CPU.R[cpu386.EDX], m.CPU.Seg[cpu386.SegES], m.CPU.Seg[cpu386.SegDS], m.CPU.Seg[cpu386.SegSS], m.CPU.EFlags, services.Calls(), err)
			fmt.Printf("dpmi_real_mode_last=%+v dpmi_unimplemented=%v\n", services.DPMI.RealModeLast, services.DPMI.Unimplemented)
			dumpVBE()
			fmt.Printf("stop_bytes=% X\n", m.Mem[ring[len(ring)-1].eip:ring[len(ring)-1].eip+16])
			for _, s := range ring {
				fmt.Printf("tail step=%d eip=0x%X esp=0x%X esi=0x%X eax=0x%X\n", s.step, s.eip, s.esp, s.esi, s.eax)
			}
			if os.Getenv("DOSGOLEM_MOO2_IRQ7_PROTOTYPE") == "1" {
				if ports, ok := services.DPMI.RealModeIO.(*machine.LEOPLPorts); ok {
					runIRQ7Prototype(m, services.DPMI, ports)
				}
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
	dumpPostClickProgress(maxSteps)
	fmt.Printf("step_limit=%d eip=0x%X unique_sites=%d\n", maxSteps, m.CPU.EIP, len(seen))
	dumpPlatform("terminal", maxSteps)
	fmt.Printf("step_limit_registers r=%X seg=%X flags=0x%X bytes=% X\n", m.CPU.R, m.CPU.Seg, m.CPU.EFlags, m.Mem[m.CPU.EIP:m.CPU.EIP+16])
	for _, s := range ring {
		fmt.Printf("step_limit_tail step=%d eip=0x%X esp=0x%X esi=0x%X eax=0x%X bytes=% X\n", s.step, s.eip, s.esp, s.esi, s.eax, m.Mem[s.eip:s.eip+8])
	}
	fmt.Printf("step_limit_memory image_bytes=%d dpmi_calls=%v dpmi_unimplemented=%v dos_blocks=%v linear_blocks=%v\n",
		len(m.Mem), services.DPMI.Calls, services.DPMI.Unimplemented, services.DPMI.DOSMemory(), services.DPMI.Blocks())
	dumpVBE()
	if os.Getenv("DOSGOLEM_MOO2_IRQ1_PROTOTYPE") == "1" {
		runIRQ1Prototype(m, services.DPMI, irq1Target, irq1Previous)
	}
}
