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

// 規格328：原Bus每個真正請求只轉呼叫一次；只在CPU.Step內計數。
type publishBusObserver struct {
	// 343 BEGIN write_state
	setleWatch bool
	// 343 END write_state

	cpu386.Bus
	active                                        bool
	step, steps                                   int
	eip                                           uint32
	targets                                       [2]uint32
	sourceStart, sourceEnd                        uint32
	reads, writes, errors, sourceReads, vbeWrites uint64
	targetReads, targetWrites                     [2]uint64
}

func (b *publishBusObserver) Read8(addr uint32) (uint8, error) {
	value, err := b.Bus.Read8(addr)
	if b.active {
		b.reads++
		if err != nil {
			b.errors++
		} else {
			if addr >= b.sourceStart && addr < b.sourceEnd {
				b.sourceReads++
				if b.sourceReads <= 4 {
					fmt.Printf("post_click_publish_event outer_step=%d input_eip=%X kind=source_read linear=%X value=%X error=%v\n", b.step, b.eip, addr, value, err)
				}
			}
			for j, target := range b.targets {
				if addr == target {
					b.targetReads[j]++
					if b.targetReads[j] <= 4 {
						fmt.Printf("post_click_publish_event outer_step=%d input_eip=%X kind=target_read target=%d linear=%X value=%X error=%v\n", b.step, b.eip, j, addr, value, err)
					}
				}
			}
		}
	}
	return value, err
}
func (b *publishBusObserver) Write8(addr uint32, value uint8) error {
	err := b.Bus.Write8(addr, value)
	// 343 BEGIN write_event
	if b.active && b.setleWatch {
		fmt.Printf("setle_bus_write outer_step=%d address_space=dosgolem_high_le input_eip=%X linear=%X value=%X error=%v\n", b.step, b.eip, addr, value, err)
	}
	// 343 END write_event

	if b.active {
		b.writes++
		if err != nil {
			b.errors++
		} else {
			if addr >= 0xa0000 && addr < 0xb0000 {
				b.vbeWrites++
			}
			for j, target := range b.targets {
				if addr == target {
					b.targetWrites[j]++
					if b.targetWrites[j] <= 4 {
						fmt.Printf("post_click_publish_event outer_step=%d input_eip=%X kind=target_write target=%d linear=%X value=%X error=%v\n", b.step, b.eip, j, addr, value, err)
					}
				}
			}
		}
	}
	return err
}

func main() {
	emptyFixture := len(os.Args) == 3 && os.Args[2] == "--empty-mox-set"
	fileFixture := len(os.Args) == 4 && os.Args[2] == "--mox-set"
	gameDirectory := len(os.Args) == 4 && os.Args[2] == "--game-dir"
	if len(os.Args) != 2 && !emptyFixture && !fileFixture && !gameDirectory {
		fmt.Fprintln(os.Stderr, "usage: moo2-probe <original-exe> [--empty-mox-set | --mox-set <local-file> | --game-dir <local-directory>]")
		os.Exit(2)
	}
	// 346 BEGIN universe160_parse
	universe160Setting := os.Getenv("DOSGOLEM_MOO2_UNIVERSE_CONTINUE_160M")
	universe160 := universe160Setting == "1"
	if universe160Setting != "" && !universe160 {
		fmt.Fprintln(os.Stderr, "UNIVERSE_CONTINUE_160M要求值1")
		os.Exit(2)
	}
	// 346 END universe160_parse

	// 352 BEGIN 180_parse
	universe180Setting := os.Getenv("DOSGOLEM_MOO2_UNIVERSE_CONTINUE_180M")
	universe180 := universe180Setting == "1"
	if universe180Setting != "" && !universe180 || universe180 && !universe160 {
		fmt.Fprintln(os.Stderr, "UNIVERSE_CONTINUE_180M要求值1與原160M旗標")
		os.Exit(2)
	}
	// 352 END 180_parse

	maxSteps := 8000000
	if setting := os.Getenv("DOSGOLEM_MOO2_MAX_STEPS"); setting != "" {
		value, parseErr := strconv.Atoi(setting)
		if parseErr != nil || value < 1 || value > 100000000 && !((value == 120000000 || universe160 && value == 160000000 || universe180 && value == 180000000) && os.Getenv("DOSGOLEM_MOO2_BANNER_RED_CLICK") == "1") {
			fmt.Fprintln(os.Stderr, "DOSGOLEM_MOO2_MAX_STEPS 必須為 1 至 100000000；完整BANNER_RED_CLICK情境可固定120000000")
			os.Exit(2)
		}
		maxSteps = value
	}
	// 352 BEGIN 180_cap
	if universe180 && maxSteps != 180000000 {
		fmt.Fprintln(os.Stderr, "UNIVERSE_CONTINUE_180M要求固定180000000與完整BANNER_RED_CLICK情境")
		os.Exit(2)
	}
	if universe180 {
		fmt.Printf("generation_continuation_config baseline_steps=160000000 max_steps=180000000 source=explicit_environment\n")
	}
	// 352 END 180_cap

	// 346 BEGIN universe160_cap
	if universe160 && maxSteps != 160000000 && !(universe180 && maxSteps == 180000000) {
		fmt.Fprintln(os.Stderr, "UNIVERSE_CONTINUE_160M要求固定160000000與完整BANNER_RED_CLICK情境")
		os.Exit(2)
	}
	if universe160 {
		fmt.Printf("universe_continuation_config baseline_steps=120000000 max_steps=%d source=explicit_environment\n", maxSteps)
	}
	// 346 END universe160_cap

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
	if newGameClick && (!keyboardRequested || keyboardStep != 44000000 && keyboardStep != 46000000 || !(maxSteps == 50000000 || (maxSteps == 100000000 || (maxSteps == 120000000 && os.Getenv("DOSGOLEM_MOO2_BANNER_RED_CLICK") == "1" || (universe160 && maxSteps == 160000000 || universe180 && maxSteps == 180000000))) && keyboardStep == 44000000) || os.Getenv("DOSGOLEM_MOO2_CALENDAR_EPOCH") != "1996-01-01" || os.Getenv("DOSGOLEM_MOO2_MOUSE_EVENT") == "1" || os.Getenv("DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION") == "1") {
		fmt.Fprintln(os.Stderr, "NEW GAME點擊要求44M或46M Esc與50M cap，或44M Esc與100M cap；1996-01-01且無早期滑鼠事件")
		os.Exit(2)
	}

	readyClickSetting := os.Getenv("DOSGOLEM_MOO2_MENU_READY_CLICK")
	readyClick := readyClickSetting == "1"
	if readyClickSetting != "" && (!readyClick || !newGameClick || keyboardStep != 44000000 || maxSteps != 100000000 && !(maxSteps == 120000000 && os.Getenv("DOSGOLEM_MOO2_BANNER_RED_CLICK") == "1" || (universe160 && maxSteps == 160000000 || universe180 && maxSteps == 180000000)) || os.Getenv("DOSGOLEM_MOO2_SEPARATE_DOS") != "1" || os.Getenv("DOSGOLEM_MOO2_VBE_FRAME_PREFIX") == "") {
		fmt.Fprintln(os.Stderr, "MENU_READY_CLICK要求值1、舊點擊旗標、44M Esc、100M cap、1996-01-01、SEPARATE_DOS=1與frame prefix")
		os.Exit(2)
	}

	setupAcceptSetting := os.Getenv("DOSGOLEM_MOO2_SETUP_ACCEPT_CLICK")
	setupAccept := setupAcceptSetting == "1"
	if setupAcceptSetting != "" && (!setupAccept || !readyClick) {
		fmt.Fprintln(os.Stderr, "SETUP_ACCEPT_CLICK要求值1與完整MENU_READY_CLICK固定情境")
		os.Exit(2)
	}

	raceHumansSetting := os.Getenv("DOSGOLEM_MOO2_RACE_HUMANS_CLICK")
	raceHumans := raceHumansSetting == "1"
	if raceHumansSetting != "" && (!raceHumans || !setupAccept) {
		fmt.Fprintln(os.Stderr, "RACE_HUMANS_CLICK要求值1與完整SETUP_ACCEPT_CLICK固定情境")
		os.Exit(2)
	}

	rulerAcceptSetting := os.Getenv("DOSGOLEM_MOO2_RULER_NAME_ACCEPT_CLICK")
	rulerAccept := rulerAcceptSetting == "1"
	if rulerAcceptSetting != "" && (!rulerAccept || !raceHumans) {
		fmt.Fprintln(os.Stderr, "RULER_NAME_ACCEPT_CLICK要求值1與完整RACE_HUMANS_CLICK固定情境")
		os.Exit(2)
	}

	// 339 BEGIN parse
	bannerRedSetting := os.Getenv("DOSGOLEM_MOO2_BANNER_RED_CLICK")
	bannerRed := bannerRedSetting == "1"
	if bannerRedSetting != "" && (!bannerRed || !rulerAccept) {
		fmt.Fprintln(os.Stderr, "BANNER_RED_CLICK要求值1與完整RULER_NAME_ACCEPT_CLICK固定情境")
		os.Exit(2)
	}
	bannerPressed, bannerReleased, bannerStoreSeen := false, false, false
	var bannerPressMicros, bannerPressStarted uint64
	bannerPolled := false
	// 339 END parse

	// 345 BEGIN universe_state
	genAnchors := [3]int{114000000, 117000000, 119900000}
	var genSeen, genWaiting [3]bool
	var genBP, genRA [3]uint32
	var genSelector [3]uint16
	var genStarts, genCounts [3]int
	genGroup, genBudget := -1, 0
	var genCode [16]byte
	var genFrame [96]byte
	var genStack [4]byte
	defer func() {
		fmt.Printf("universe_loop_totals groups=%v waiting=%v starts=%v samples=%v remaining=%d max_groups=3 max_steps_per_group=192\n", genSeen, genWaiting, genStarts, genCounts, genBudget)
	}()
	// 345 END universe_state

	// 344 BEGIN cc_state
	ccSeen, ccBudget := false, 0
	// 344 END cc_state

	// 343 BEGIN setle_state
	setleSeen, setleBudget := false, 0
	// 343 END setle_state

	// 342 BEGIN test_state
	testMemorySeen, testMemoryBudget := false, 0
	// 342 END test_state

	// 353 BEGIN and_state
	andWordMemorySeen, andWordMemoryBudget := false, 0
	// 353 END and_state

	// 354 BEGIN xchg_state
	// 357 BEGIN imul_word_state
	imulWordSeen, imulWordBudget := false, 0
	// 357 END imul_word_state

	// 356 BEGIN set_memory_state
	setMemorySeen, setMemoryBudget := false, 0
	// 356 END set_memory_state

	// 355 BEGIN add_source_state
	addSourceSeen, addSourceBudget := false, 0
	// 355 END add_source_state

	xchgByteMemorySeen, xchgByteMemoryBudget := false, 0
	// 354 END xchg_state

	// 341 BEGIN gate_state
	bannerSelectionGateSeen := false
	var bannerGateR [8]uint32
	var bannerGateSeg [6]uint16
	var bannerGateFlags uint32
	var bannerGateStack [4]byte
	bannerGateReadable, bannerGateReadonly := false, false
	// 341 END gate_state

	// 340 BEGIN return_state
	bannerGUIButtonSeen := false
	var bannerReturnR [8]uint32
	var bannerReturnSeg [6]uint16
	var bannerReturnFlags uint32
	var bannerReturnStack [4]byte
	bannerReturnReadable, bannerReturnReadonly := false, false
	// 340 END return_state

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
			// 339 BEGIN poll
			if bannerRed && bannerPressed && !bannerReleased && !bannerPolled && handled && uint16(beforeR[cpu386.EAX]) == 3 && c.EIP-2 == 0x24c31b && !oldActive && uint16(c.R[cpu386.EBX]) == 1 && uint16(c.R[cpu386.ECX]) == 276 && uint16(c.R[cpu386.EDX]) == 190 {
				bannerPolled = true
				micros := uint64(0)
				if ports, ok := services.DPMI.RealModeIO.(*machine.LEOPLPorts); ok && ports.BIOSClock != nil {
					micros = ports.BIOSClock.Micros
				}
				fmt.Printf("banner_red_pressed_poll outer_step=%d address_space=dosgolem_high_le callsite=%X virtual_micros=%d before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X callback_active=%t handled=%t\n", loopStep, c.EIP-2, micros, beforeR, c.R, beforeSeg, c.Seg, beforeFlags, c.EFlags, oldActive, handled)
			}
			// 339 END poll

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

	// 規格334：正式7筆表就緒後的一次額外正常輸入，不改舊情境。

	// 規格335唯讀診斷：首個原SCASW與一條MOV消費，最多兩步。
	scasWordSeen, scasWordBudget := false, 0
	var scasWordSelector, scasWordStackSelector uint16
	var scasWordOffset, scasWordStackOffset uint32
	var scasWordSource []byte

	// 規格336：80M原17筆表與設定頁一致後，一次正常ACCEPT輸入。

	// 339 BEGIN state
	// 規格339：99M原旗幟頁／十筆表一致後，一次正常紅色選擇。
	defer func() {
		if bannerRed {
			mask, pending, active, started, completed := services.MouseCallbackState()
			fmt.Printf("banner_red_terminal pressed=%t released=%t store_seen=%t mask=%X pending=%d active=%t started=%d completed=%d\n", bannerPressed, bannerReleased, bannerStoreSeen, mask, pending, active, started, completed)
		}
	}()
	// 339 END state

	// 規格338：95M原名稱頁／三筆表與候選bytes一致後，一次正常ACCEPT輸入。
	rulerPressed, rulerReleased, rulerStoreSeen := false, false, false
	var rulerPressMicros, rulerPressStarted uint64
	defer func() {
		if rulerAccept {
			mask, pending, active, started, completed := services.MouseCallbackState()
			fmt.Printf("ruler_name_accept_terminal pressed=%t released=%t store_seen=%t mask=%X pending=%d active=%t started=%d completed=%d\n", rulerPressed, rulerReleased, rulerStoreSeen, mask, pending, active, started, completed)
		}
	}()

	// 規格337：90M原16筆選族表與畫面一致後，一次正常Humans輸入。
	racePressed, raceReleased, raceStoreSeen := false, false, false
	var racePressMicros, racePressStarted uint64
	defer func() {
		if raceHumans {
			mask, pending, active, started, completed := services.MouseCallbackState()
			fmt.Printf("race_humans_terminal pressed=%t released=%t store_seen=%t mask=%X pending=%d active=%t started=%d completed=%d\n", racePressed, raceReleased, raceStoreSeen, mask, pending, active, started, completed)
		}
	}()
	setupPressed, setupReleased, setupStoreSeen := false, false, false
	var setupPressMicros, setupPressStarted uint64
	defer func() {
		if setupAccept {
			mask, pending, active, started, completed := services.MouseCallbackState()
			fmt.Printf("setup_accept_terminal pressed=%t released=%t store_seen=%t mask=%X pending=%d active=%t started=%d completed=%d\n", setupPressed, setupReleased, setupStoreSeen, mask, pending, active, started, completed)
		}
	}()
	readyPressed, readyReleased, readyStoreSeen := false, false, false
	var readyPressMicros, readyPressStarted uint64
	defer func() {
		if readyClick {
			mask, pending, active, started, completed := services.MouseCallbackState()
			fmt.Printf("ready_menu_terminal pressed=%t released=%t store_seen=%t mask=%X pending=%d active=%t started=%d completed=%d\n", readyPressed, readyReleased, readyStoreSeen, mask, pending, active, started, completed)
		}
	}()
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

	// 規格328：正常後段的Bus包裝與VBE獨立計數對帳。
	var publishBus *publishBusObserver
	var publishBegin machine.MOO2VBEState
	defer func() {
		if publishBus != nil {
			fmt.Printf("post_click_publish_totals begin_step=49500000 steps=%d reads=%d writes=%d errors=%d targets=%X target_reads=%v target_writes=%v source_start=%X source_end=%X source_reads=%d vbe_writes=%d bus_matches=%t begin_state=%+v end_state=%+v\n", publishBus.steps, publishBus.reads, publishBus.writes, publishBus.errors, publishBus.targets, publishBus.targetReads, publishBus.targetWrites, publishBus.sourceStart, publishBus.sourceEnd, publishBus.sourceReads, publishBus.vbeWrites, m.CPU.Bus == publishBus, publishBegin, m.VBEState())
		}
	}()

	// 規格327：只讀真正來源與有界消費，不使用CPU讀取hook。
	var sourceGroups [4]int
	sourceActive, sourceBudget, sourceGroup := false, 0, 0
	sourceKind := ""
	var sourceSelector, sourceStackSelector uint16
	var sourceBase, sourceIndex, sourceOffset, sourceStackOffset uint32
	peekSourceWindow := func(selector uint16, offset uint32, out []byte) bool {
		desc, known := m.CPU.Descriptors[selector]
		linear := uint64(desc.Base) + uint64(offset)
		if !known || uint64(offset)+uint64(len(out)) > uint64(desc.Limit)+1 || linear+uint64(len(out)) > uint64(len(m.Mem)) {
			return false
		}
		copy(out, m.Mem[linear:linear+uint64(len(out))])
		return true
	}
	defer func() {
		if newGameClick && phasePrefix != "" {
			fmt.Printf("post_click_source_totals groups=%v active=%t remaining=%d max_regular_groups=6 max_steps_per_group=33\n", sourceGroups, sourceActive, sourceBudget)
		}
	}()

	// 規格330：事件返回與首個上層Jcc的有界唯讀觀察。
	var activationSeen [2]bool
	activationGroup, activationBudget := -1, 0
	activationReturned := false
	var activationSelector, activationStackSelector uint16
	var activationStackOffset uint32
	activationPeek := func(read func()) bool {
		c := m.CPU
		r, seg, eip, flags, bus := c.R, c.Seg, c.EIP, c.EFlags, c.Bus
		control, status, stack, depth := c.FPUControl, c.FPUStatus, c.FPUStack, c.FPUDepth
		state := m.VBEState()
		read()
		same := c.R == r && c.Seg == seg && c.EIP == eip && c.EFlags == flags && c.Bus == bus && c.FPUControl == control && c.FPUStatus == status && c.FPUDepth == depth && m.VBEState() == state
		for j := range stack {
			same = same && math.Float64bits(stack[j]) == math.Float64bits(c.FPUStack[j])
		}
		if !same {
			panic("事件返回快照改變原始狀態")
		}
		return same
	}

	// 346 BEGIN universe160_checkpoint
	universeCheckpointSeen := false
	dumpUniverseCheckpoint := func(step int) {
		if !universe160 || step != 120000000 || universeCheckpointSeen {
			return
		}
		universeCheckpointSeen = true
		c := m.CPU
		r, seg, eip, flags := c.R, c.Seg, c.EIP, c.EFlags
		control, status, stack, depth := c.FPUControl, c.FPUStatus, c.FPUStack, c.FPUDepth
		state := m.VBEState()
		var code [16]byte
		var rgbHash [32]byte
		ram := sha256.Sum256(m.Mem)
		readonly := activationPeek(func() {
			copy(code[:], m.Mem[eip:eip+16])
			rgbHash = sha256.Sum256(m.VBERGB())
		}) && ram == sha256.Sum256(m.Mem)
		mask, pending, active, started, completed := services.MouseCallbackState()
		irqActive, irqFailed, irqStarted, irqCompleted := services.IRQ0State()
		micros := uint64(0)
		if ports, ok := services.DPMI.RealModeIO.(*machine.LEOPLPorts); ok {
			micros = ports.BIOSClock.Micros
		}
		fmt.Printf("universe_continuation_checkpoint outer_step=%d address_space=dosgolem_high_le eip=%X r=%X seg=%X flags=%X fpu_control=%X fpu_status=%X fpu_depth=%d fpu_stack=%v state=%+v instruction_bytes=%X virtual_micros=%d rgb_sha256=%x ram_before_sha256=%x ram_after_sha256=%x readonly=%t callback_mask=%X callback_pending=%d callback_active=%t callback_started=%d callback_completed=%d irq_active=%t irq_failed=%t irq_started=%d irq_completed=%d gen_waiting=%v gen_samples=%v\n", step, eip, r, seg, flags, control, status, depth, stack, state, code, micros, rgbHash, ram, sha256.Sum256(m.Mem), readonly, mask, pending, active, started, completed, irqActive, irqFailed, irqStarted, irqCompleted, genWaiting, genCounts)
		dumpPlatform("universe_120m", step)
	}
	// 346 END universe160_checkpoint

	// 347 BEGIN text_state
	textSeen := [2][3]bool{}
	textSources := [2]uint32{}
	textUntil := [2]int{}
	textLabels := [2]string{"Generating Universe ...", "Placing home worlds ..."}
	observeProgressText := func(step int) {
		if !universe160 {
			return
		}
		c := m.CPU
		group, phase := -1, -1
		switch c.EIP {
		case 0x17dca5:
			group, phase = 0, 0
		case 0x17dcaa:
			group, phase = 0, 1
		case 0x16c8a3:
			group, phase = 1, 0
		case 0x16c8a8:
			group, phase = 1, 1
		}
		if phase == 1 && !textSeen[group][phase] {
			textSources[group] = c.R[0]
			textUntil[group] = step + 256
		}
		if group < 0 {
			for j := range textUntil {
				if textSeen[j][1] && !textSeen[j][2] && step <= textUntil[j] {
					var destination [48]byte
					readable := false
					stable := activationPeek(func() { readable = peekSourceWindow(c.Seg[cpu386.SegDS], 0x2842f4, destination[:]) })
					if stable && readable && c.EIP == [2]uint32{0x17dcba, 0x16c8b6}[j] && c.R[6] == textSources[j]+uint32(len(textLabels[j])+1) && c.R[7] == 0x2842f4+uint32(len(textLabels[j])+1) && strings.HasPrefix(string(destination[:]), textLabels[j]+"\x00") {
						group, phase = j, 2
						break
					}
				}
			}
		}
		if group < 0 || phase < 0 || textSeen[group][phase] {
			return
		}
		textSeen[group][phase] = true
		var source, destination [48]byte
		var stack [96]byte
		var code [16]byte
		sourceOK, destinationOK, stackOK := false, false, false
		ram := sha256.Sum256(m.Mem)
		readonly := activationPeek(func() {
			sourceOK = peekSourceWindow(c.Seg[cpu386.SegDS], textSources[group], source[:])
			destinationOK = peekSourceWindow(c.Seg[cpu386.SegDS], 0x2842f4, destination[:])
			stackOK = peekSourceWindow(c.Seg[cpu386.SegSS], c.R[4], stack[:])
			copy(code[:], m.Mem[c.EIP:c.EIP+16])
		}) && ram == sha256.Sum256(m.Mem)
		control, status, depth := c.FPUControl, c.FPUStatus, c.FPUDepth
		fmt.Printf("progress_text_observation group=%d phase=%s outer_step=%d address_space=dosgolem_high_le eip=%X r=%X seg=%X flags=%X fpu_control=%X fpu_status=%X fpu_depth=%d instruction_bytes=%X source_offset=%X source_readable=%t source_window=%X destination_offset=2842F4 destination_readable=%t destination_window=%X stack_offset=%X stack_readable=%t stack_window=%X readonly=%t\n", group, [3]string{"lookup_call", "lookup_return", "copy_complete"}[phase], step, c.EIP, c.R, c.Seg, c.EFlags, control, status, depth, code, textSources[group], sourceOK, source, destinationOK, destination, c.R[4], stackOK, stack, readonly)
	}
	defer func() {
		if universe160 {
			fmt.Printf("progress_text_totals seen=%v source_offsets=%X copy_deadlines=%v max_groups=2 max_events_per_group=3 max_copy_steps=256\n", textSeen, textSources, textUntil)
		}
	}()
	// 347 END text_state

	// 348 BEGIN home_return_state
	homeReturnSeen := [11]bool{}
	homeCallerSP, homeCallerBP := uint32(0), uint32(0)
	homeDump := func(step, phase int) {
		c := m.CPU
		var code [16]byte
		var stack [96]byte
		var callerWord, localWord [2]byte
		var returnSlot [4]byte
		codeOK, stackOK, callerOK, localOK, slotOK := false, false, false, false, false
		ram := sha256.Sum256(m.Mem)
		readonly := activationPeek(func() {
			codeOK = peekSourceWindow(c.Seg[cpu386.SegCS], c.EIP, code[:])
			stackOK = peekSourceWindow(c.Seg[cpu386.SegSS], c.R[4], stack[:])
			if homeCallerBP >= 8 {
				callerOK = peekSourceWindow(c.Seg[cpu386.SegSS], homeCallerBP-8, callerWord[:])
			}
			if c.R[5] >= 8 {
				localOK = peekSourceWindow(c.Seg[cpu386.SegSS], c.R[5]-8, localWord[:])
			}
			if homeCallerSP >= 4 {
				slotOK = peekSourceWindow(c.Seg[cpu386.SegSS], homeCallerSP-4, returnSlot[:])
			}
		}) && ram == sha256.Sum256(m.Mem)
		homeReturnSeen[phase] = true
		fmt.Printf("home_return_observation phase=%d outer_step=%d address_space=dosgolem_high_le eip=%X r=%X seg=%X flags=%X fpu_control=%X fpu_status=%X fpu_depth=%d instruction_bytes=%X code_readable=%t stack_offset=%X stack_readable=%t stack_window=%X caller_sp=%X caller_bp=%X caller_word_offset=%X caller_word=%X caller_word_readable=%t local_word=%X local_word_readable=%t return_slot=%X return_slot_readable=%t ram_before_sha256=%x ram_after_sha256=%x readonly=%t\n", phase, step, c.EIP, c.R, c.Seg, c.EFlags, c.FPUControl, c.FPUStatus, c.FPUDepth, code, codeOK, c.R[4], stackOK, stack, homeCallerSP, homeCallerBP, homeCallerBP-8, callerWord, callerOK, localWord, localOK, returnSlot, slotOK, ram, sha256.Sum256(m.Mem), readonly)
	}
	observeHomeReturn := func(step int) {
		if !universe160 {
			return
		}
		c := m.CPU
		phase := -1
		if !homeReturnSeen[0] && c.EIP == 0x16b985 {
			homeCallerSP, homeCallerBP = c.R[4], c.R[5]
			phase = 0
		} else if homeReturnSeen[0] && !homeReturnSeen[9] {
			switch c.EIP {
			case 0x16c78e:
				if c.R[4] == homeCallerSP-4 {
					phase = 1
				}
			case 0x16c8b6:
				if c.R[5] == homeCallerSP-28 {
					phase = 2
				}
			case 0x16c8e1:
				if c.R[5] == homeCallerSP-28 {
					phase = 3
				}
			case 0x16bf57:
				if c.R[5] == homeCallerSP-28 {
					phase = 4
				}
			case 0x16bd81:
				if c.R[4] == homeCallerSP-24 {
					phase = 5
				}
			case 0x16bd86:
				if c.R[4] == homeCallerSP-4 {
					phase = 6
				}
			case 0x16b98a:
				if c.R[4] == homeCallerSP {
					phase = 7
				}
			case 0x16b98f:
				if c.R[4] == homeCallerSP {
					phase = 8
				}
			case 0x16b995, 0x16bb0c:
				if homeReturnSeen[8] && c.R[4] == homeCallerSP {
					phase = 9
				}
			}
		}
		if phase >= 0 && !homeReturnSeen[phase] {
			homeDump(step, phase)
		}
	}
	defer func() {
		if universe160 {
			homeDump(loopStep+1, 10)
			fmt.Printf("home_return_totals seen=%v max_events=11 budget=%d caller_sp=%X caller_bp=%X\n", homeReturnSeen, maxSteps, homeCallerSP, homeCallerBP)
		}
	}()
	// 348 END home_return_state

	// 349 BEGIN generation_state
	generationSeen := [17]bool{}
	generationSP, generationBP, generationInput := uint32(0), uint32(0), uint32(0)
	generationSites := [16]uint32{0x16bade, 0x16ad13, 0x16ad1b, 0x16ad34, 0x16ad39, 0x16ad56, 0x16ad5b, 0x16adfb, 0x16ae00, 0x16ae0c, 0x16ae7f, 0x16ae84, 0x16b01f, 0x16bae3, 0x16baec, 0}
	generationDump := func(step, phase int) {
		c := m.CPU
		var code [16]byte
		var stack [96]byte
		var frame [128]byte
		var input [64]byte
		var returnSlot [4]byte
		codeOK, stackOK, frameOK, inputOK, slotOK := false, false, false, false, false
		ram := sha256.Sum256(m.Mem)
		readonly := activationPeek(func() {
			codeOK = peekSourceWindow(c.Seg[cpu386.SegCS], c.EIP, code[:])
			stackOK = peekSourceWindow(c.Seg[cpu386.SegSS], c.R[4], stack[:])
			if generationSP >= 88 {
				frameOK = peekSourceWindow(c.Seg[cpu386.SegSS], generationSP-88, frame[:])
			}
			inputOK = peekSourceWindow(c.Seg[cpu386.SegDS], generationInput, input[:])
			if generationSP >= 4 {
				slotOK = peekSourceWindow(c.Seg[cpu386.SegSS], generationSP-4, returnSlot[:])
			}
		}) && ram == sha256.Sum256(m.Mem)
		generationSeen[phase] = true
		fmt.Printf("generation_entry_observation phase=%d outer_step=%d address_space=dosgolem_high_le eip=%X r=%X seg=%X flags=%X fpu_control=%X fpu_status=%X fpu_depth=%d instruction_bytes=%X code_readable=%t stack_offset=%X stack_readable=%t stack_window=%X caller_sp=%X caller_bp=%X callee_bp=%X frame_offset=%X frame_readable=%t frame_window=%X input_offset=%X input_readable=%t input_window=%X return_slot=%X return_slot_readable=%t ram_before_sha256=%x ram_after_sha256=%x readonly=%t\n", phase, step, c.EIP, c.R, c.Seg, c.EFlags, c.FPUControl, c.FPUStatus, c.FPUDepth, code, codeOK, c.R[4], stackOK, stack, generationSP, generationBP, generationSP-24, generationSP-88, frameOK, frame, generationInput, inputOK, input, returnSlot, slotOK, ram, sha256.Sum256(m.Mem), readonly)
	}
	observeGenerationEntry := func(step int) {
		if !universe160 || !homeReturnSeen[9] {
			return
		}
		c := m.CPU
		phase := -1
		if !generationSeen[0] && c.EIP == generationSites[0] {
			generationSP, generationBP, generationInput = c.R[4], c.R[5], c.R[0]
			phase = 0
		} else if generationSeen[0] && !generationSeen[15] {
			for j := 1; j < len(generationSites)-1; j++ {
				if c.EIP != generationSites[j] || generationSeen[j] {
					continue
				}
				valid := c.R[5] == generationSP-24
				if j == 1 || j == 12 {
					valid = c.R[4] == generationSP-4
				}
				if j == 13 || j == 14 {
					valid = c.R[4] == generationSP && c.R[5] == generationBP
				}
				if valid {
					phase = j
					break
				}
			}
			if generationSeen[14] && !generationSeen[15] && (c.EIP == 0x16bb00 || c.EIP == 0x16baee) && c.R[4] == generationSP && c.R[5] == generationBP {
				phase = 15
			}
		}
		if phase >= 0 && !generationSeen[phase] {
			generationDump(step, phase)
		}
	}
	defer func() {
		if universe160 {
			generationDump(loopStep+1, 16)
			fmt.Printf("generation_entry_totals seen=%v max_events=17 budget=%d caller_sp=%X caller_bp=%X callee_bp=%X input_offset=%X returned=%t\n", generationSeen, maxSteps, generationSP, generationBP, generationSP-24, generationInput, generationSeen[13])
		}
	}()
	// 349 END generation_state

	// 350 BEGIN iteration_state
	iterationSeen := [4][5]bool{}
	iterationIndices := [4]uint16{}
	iterationGroup, iterationGroups, iterationEvents := -1, 0, 0
	iterationFull := false
	iterationDump := func(step, group, phase int) {
		c := m.CPU
		var code, boundCode [16]byte
		var stack [96]byte
		var frame [48]byte
		var bound [2]byte
		codeOK, boundCodeOK, stackOK, frameOK, boundOK := false, false, false, false, false
		boundOffset := uint32(0)
		ram := sha256.Sum256(m.Mem)
		readonly := activationPeek(func() {
			codeOK = peekSourceWindow(c.Seg[cpu386.SegCS], c.EIP, code[:])
			boundCodeOK = peekSourceWindow(c.Seg[cpu386.SegCS], 0x16af1c, boundCode[:])
			if boundCodeOK && boundCode[0] == 0x66 && boundCode[1] == 0x3b && boundCode[2] == 0x35 {
				boundOffset = uint32(boundCode[3]) | uint32(boundCode[4])<<8 | uint32(boundCode[5])<<16 | uint32(boundCode[6])<<24
				boundOK = peekSourceWindow(c.Seg[cpu386.SegDS], boundOffset, bound[:])
			}
			stackOK = peekSourceWindow(c.Seg[cpu386.SegSS], c.R[4], stack[:])
			if generationSP >= 56 {
				frameOK = peekSourceWindow(c.Seg[cpu386.SegSS], generationSP-56, frame[:])
			}
		}) && ram == sha256.Sum256(m.Mem)
		iterationEvents++
		fmt.Printf("generation_iteration_observation group=%d phase=%d outer_step=%d address_space=dosgolem_high_le eip=%X r=%X seg=%X flags=%X fpu_control=%X fpu_status=%X fpu_depth=%d instruction_bytes=%X code_readable=%t bound_instruction_bytes=%X bound_code_readable=%t bound_offset=%X bound_raw=%X bound_readable=%t bound_signed=%d stack_offset=%X stack_readable=%t stack_window=%X frame_offset=%X frame_readable=%t frame_window=%X ram_before_sha256=%x ram_after_sha256=%x readonly=%t\n", group, phase, step, c.EIP, c.R, c.Seg, c.EFlags, c.FPUControl, c.FPUStatus, c.FPUDepth, code, codeOK, boundCode, boundCodeOK, boundOffset, bound, boundOK, int16(uint16(bound[0])|uint16(bound[1])<<8), c.R[4], stackOK, stack, generationSP-56, frameOK, frame, ram, sha256.Sum256(m.Mem), readonly)
	}
	observeGenerationIteration := func(step int) {
		if !universe160 || !generationSeen[2] || generationSeen[13] || m.CPU.R[5] != generationSP-24 {
			return
		}
		c := m.CPU
		if c.EIP == 0x16ad7d {
			index := uint16(c.R[6])
			if iterationGroup < 0 || iterationIndices[iterationGroup] != index {
				if iterationGroups >= len(iterationSeen) {
					iterationFull = true
					return
				}
				iterationGroup = iterationGroups
				iterationGroups++
				iterationIndices[iterationGroup] = index
			}
		}
		if iterationGroup < 0 || iterationFull {
			return
		}
		phase := -1
		switch c.EIP {
		case 0x16ad7d:
			phase = 0
		case 0x16af1c:
			phase = 1
		case 0x16af23:
			phase = 2
		case 0x16ae07:
			phase = 3
		case 0x16ae0c:
			if iterationSeen[iterationGroup][3] {
				phase = 4
			}
		}
		if phase >= 0 && !iterationSeen[iterationGroup][phase] {
			iterationSeen[iterationGroup][phase] = true
			iterationDump(step, iterationGroup, phase)
		}
	}
	defer func() {
		if universe160 {
			iterationDump(loopStep+1, -1, 5)
			fmt.Printf("generation_iteration_totals groups=%d indices=%v seen=%v events=%d max_groups=4 max_events_per_group=5 max_events=21 full=%t budget=%d outer_returned=%t\n", iterationGroups, iterationIndices, iterationSeen, iterationEvents, iterationFull, maxSteps, generationSeen[13])
		}
	}()
	// 350 END iteration_state

	// 351 BEGIN completion_state
	completionSeen := [12]bool{}
	completionHeads, completionEvents := 0, 0
	completionFull := false
	completionDump := func(step, group, phase int) {
		c := m.CPU
		var code, boundCode [16]byte
		var stack [96]byte
		var frame [48]byte
		var bound [2]byte
		codeOK, boundCodeOK, stackOK, frameOK, boundOK := false, false, false, false, false
		boundOffset := uint32(0)
		ram := sha256.Sum256(m.Mem)
		readonly := activationPeek(func() {
			codeOK = peekSourceWindow(c.Seg[cpu386.SegCS], c.EIP, code[:])
			boundCodeOK = peekSourceWindow(c.Seg[cpu386.SegCS], 0x16af1c, boundCode[:])
			if boundCodeOK && boundCode[0] == 0x66 && boundCode[1] == 0x3b && boundCode[2] == 0x35 {
				boundOffset = uint32(boundCode[3]) | uint32(boundCode[4])<<8 | uint32(boundCode[5])<<16 | uint32(boundCode[6])<<24
				boundOK = peekSourceWindow(c.Seg[cpu386.SegDS], boundOffset, bound[:])
			}
			stackOK = peekSourceWindow(c.Seg[cpu386.SegSS], c.R[4], stack[:])
			if generationSP >= 56 {
				frameOK = peekSourceWindow(c.Seg[cpu386.SegSS], generationSP-56, frame[:])
			}
		}) && ram == sha256.Sum256(m.Mem)
		completionEvents++
		fmt.Printf("generation_completion_observation sequence=%d phase=%d outer_step=%d address_space=dosgolem_high_le eip=%X r=%X seg=%X flags=%X fpu_control=%X fpu_status=%X fpu_depth=%d instruction_bytes=%X code_readable=%t bound_instruction_bytes=%X bound_code_readable=%t bound_offset=%X bound_raw=%X bound_readable=%t bound_signed=%d stack_offset=%X stack_readable=%t stack_window=%X frame_offset=%X frame_readable=%t frame_window=%X ram_before_sha256=%x ram_after_sha256=%x readonly=%t\n", group, phase, step, c.EIP, c.R, c.Seg, c.EFlags, c.FPUControl, c.FPUStatus, c.FPUDepth, code, codeOK, boundCode, boundCodeOK, boundOffset, bound, boundOK, int16(uint16(bound[0])|uint16(bound[1])<<8), c.R[4], stackOK, stack, generationSP-56, frameOK, frame, ram, sha256.Sum256(m.Mem), readonly)
	}
	observeGenerationCompletion := func(step int) {
		if !universe160 || !generationSeen[2] {
			return
		}
		c := m.CPU
		phase := -1
		if c.EIP == 0x16bae3 && c.R[4] == generationSP && c.R[5] == generationBP {
			phase = 11
		} else if !generationSeen[13] {
			if c.EIP == 0x16b01f && c.R[4] == generationSP-4 {
				phase = 10
			} else if c.R[5] == generationSP-24 {
				switch c.EIP {
				case 0x16ad7d:
					if completionHeads < 72 {
						completionDump(step, completionHeads, 0)
						completionHeads++
					} else {
						completionFull = true
					}
				case 0x16af1c:
					if uint16(c.R[6]) == 36 {
						phase = 1
					}
				case 0x16af23:
					if uint16(c.R[6]) == 36 {
						phase = 2
					}
				case 0x16af29:
					phase = 3
				case 0x16af35:
					phase = 4
				case 0x16af68:
					phase = 5
				case 0x16af6e:
					phase = 6
				case 0x16b014:
					phase = 7
				case 0x16b018:
					phase = 8
				case 0x16b01a:
					phase = 9
				}
			}
		}
		if phase > 0 && !completionSeen[phase] {
			completionSeen[phase] = true
			completionDump(step, -1, phase)
		}
	}
	defer func() {
		if universe160 {
			completionDump(loopStep+1, -1, 12)
			fmt.Printf("generation_completion_totals heads=%d seen=%v events=%d max_heads=72 max_boundaries=11 max_events=84 full=%t budget=%d outer_returned=%t\n", completionHeads, completionSeen, completionEvents, completionFull, maxSteps, generationSeen[13])
		}
	}()
	// 351 END completion_state

	// 352 BEGIN 180_checkpoint
	continuationCheckpointSeen := false
	dumpGenerationContinuationCheckpoint := func(step int) {
		if !universe180 || step != 160000000 || continuationCheckpointSeen {
			return
		}
		continuationCheckpointSeen = true
		c := m.CPU
		var code [16]byte
		var stack [96]byte
		var frame [128]byte
		var input [64]byte
		var returnSlot [4]byte
		codeOK, stackOK, frameOK, inputOK, slotOK := false, false, false, false, false
		var indexedHash, rgbHash [32]byte
		ram := sha256.Sum256(m.Mem)
		readonly := activationPeek(func() {
			codeOK = peekSourceWindow(c.Seg[cpu386.SegCS], c.EIP, code[:])
			stackOK = peekSourceWindow(c.Seg[cpu386.SegSS], c.R[4], stack[:])
			if generationSP >= 88 {
				frameOK = peekSourceWindow(c.Seg[cpu386.SegSS], generationSP-88, frame[:])
			}
			inputOK = peekSourceWindow(c.Seg[cpu386.SegDS], generationInput, input[:])
			if generationSP >= 4 {
				slotOK = peekSourceWindow(c.Seg[cpu386.SegSS], generationSP-4, returnSlot[:])
			}
			indexedHash = sha256.Sum256(m.VBEIndexed())
			rgbHash = sha256.Sum256(m.VBERGB())
			dumpPlatform("continuation_160m", step)
		}) && ram == sha256.Sum256(m.Mem)
		irqActive, irqFailed, irqStarted, irqCompleted := services.IRQ0State()
		fmt.Printf("generation_continuation_checkpoint outer_step=%d address_space=dosgolem_high_le eip=%X r=%X seg=%X flags=%X fpu_control=%X fpu_status=%X fpu_depth=%d fpu_stack=%v instruction_bytes=%X code_readable=%t stack_offset=%X stack_readable=%t stack_window=%X caller_sp=%X caller_bp=%X callee_bp=%X frame_offset=%X frame_readable=%t frame_window=%X input_offset=%X input_readable=%t input_window=%X return_slot=%X return_slot_readable=%t state=%+v indexed_sha256=%x rgb_sha256=%x ram_before_sha256=%x ram_after_sha256=%x readonly=%t irq_active=%t irq_failed=%t irq_started=%d irq_completed=%d\n", step, c.EIP, c.R, c.Seg, c.EFlags, c.FPUControl, c.FPUStatus, c.FPUDepth, c.FPUStack, code, codeOK, c.R[4], stackOK, stack, generationSP, generationBP, generationSP-24, generationSP-88, frameOK, frame, generationInput, inputOK, input, returnSlot, slotOK, m.VBEState(), indexedHash, rgbHash, ram, sha256.Sum256(m.Mem), readonly, irqActive, irqFailed, irqStarted, irqCompleted)
	}
	// 352 END 180_checkpoint

	// 規格331：只觀察非零臂caller，callee等待原程式正常返回。
	branchSeen, branchActive, branchWaiting := false, false, false
	branchStart, branchSamples, branchCallStep := 0, 0, 0
	var branchSelector, branchStackSelector, branchReturnSelector uint16
	var branchStackOffset, branchReturnEIP, branchReturnESP uint32
	branchStop := ""
	// 規格332：舊96步終態先保存，後續以獨立前綴有界觀察。
	branchTail, branchOldTerminal := false, ""
	branchMaxSamples := 96
	branchTerminal := func(label string, limit int) string {
		return fmt.Sprintf("%s seen=%t active=%t waiting=%t start=%d samples=%d max_samples=%d max_outer_steps=8192 return_eip=%X return_selector=%X return_esp=%X stop=%s\n", label, branchSeen, branchActive, branchWaiting, branchStart, branchSamples, limit, branchReturnEIP, branchReturnSelector, branchReturnESP, branchStop)
	}
	defer func() {
		if newGameClick && phasePrefix != "" {
			if branchOldTerminal != "" {
				fmt.Print(branchOldTerminal)
			} else {
				fmt.Print(branchTerminal("new_game_button_branch_terminal", 96))
			}
			if branchTail {
				fmt.Print(branchTerminal("new_game_button_tail_terminal", 384))
			}
		}
	}()

	// 規格326：只有正常點擊與明示圖像輸出才觀測後段，不改CPU或輸入。
	postClickVisits := make(map[uint32]uint64)
	postClickSteps := 0

	// 規格336唯讀診斷：設定頁原表三時點，不沿舊caller框架猜測。
	dumpSetupTable := func(step int) {
		if !readyClick || step != 80000000 && step != 90000000 && step != 100000000 && !(bannerRed && maxSteps == 120000000 && step == 120000000 || universe160 && step >= 120000000 && step <= maxSteps && step%10000000 == 0) {
			return
		}
		c := m.CPU
		r, seg, eip, flags, bus := c.R, c.Seg, c.EIP, c.EFlags, c.Bus
		control, status, depth, fpu := c.FPUControl, c.FPUStatus, c.FPUDepth, c.FPUStack
		state, ramBefore := m.VBEState(), sha256.Sum256(m.Mem)
		mask, pending, active, started, completed := services.MouseCallbackState()
		irqActive, irqFailed, irqStarted, irqCompleted := services.IRQ0State()
		var globals [192]byte
		var header, event [16]byte
		g := peekSourceWindow(seg[cpu386.SegDS], 0x26c480, globals[:])
		h := peekSourceWindow(seg[cpu386.SegDS], 0x29be0e, header[:])
		ev := peekSourceWindow(seg[cpu386.SegDS], 0x2a121a, event[:])
		pointer := uint32(globals[0]) | uint32(globals[1])<<8 | uint32(globals[2])<<16 | uint32(globals[3])<<24
		count := uint16(header[0]) | uint16(header[1])<<8
		bias := uint32(header[4]) | uint32(header[5])<<8 | uint32(header[6])<<16 | uint32(header[7])<<24
		var records []byte
		t := false
		if g && h && count <= 64 {
			records = make([]byte, int(count)*55)
			t = peekSourceWindow(seg[cpu386.SegDS], pointer, records)
		}
		ramAfter := sha256.Sum256(m.Mem)
		am, ap, aa, ast, ac := services.MouseCallbackState()
		ia, iff, ist, ic := services.IRQ0State()
		readonly := c.R == r && c.Seg == seg && c.EIP == eip && c.EFlags == flags && c.Bus == bus && c.FPUControl == control && c.FPUStatus == status && c.FPUDepth == depth && m.VBEState() == state && ramBefore == ramAfter && mask == am && pending == ap && active == aa && started == ast && completed == ac && irqActive == ia && irqFailed == iff && irqStarted == ist && irqCompleted == ic
		for j := range fpu {
			readonly = readonly && math.Float64bits(fpu[j]) == math.Float64bits(c.FPUStack[j])
		}
		if !readonly {
			panic("設定頁唯讀表快照改變原始狀態")
		}
		fmt.Printf("setup_table_snapshot outer_step=%d address_space=dosgolem_high_le eip=%X r=%X seg=%X flags=%X fpu_control=%X fpu_status=%X fpu_depth=%d fpu_stack=%v state=%+v ds=%X globals_readable=%t globals=%X header_readable=%t header=%X table_pointer=%X table_count=%d table_bias=%X table_stride=55 table_readable=%t records=%X event_readable=%t event=%X callback_mask=%X callback_pending=%d callback_active=%t callback_started=%d callback_completed=%d irq_active=%t irq_failed=%t irq_started=%d irq_completed=%d ram_before_sha256=%x ram_after_sha256=%x readonly=%t\n", step, eip, r, seg, flags, control, status, depth, fpu, state, seg[cpu386.SegDS], g, globals, h, header, pointer, count, bias, t, records, ev, event, mask, pending, active, started, completed, irqActive, irqFailed, irqStarted, irqCompleted, ramBefore, ramAfter, readonly)
	}

	// 規格333：只保存原表與原caller框架，等待已驗CALL的自然返回。
	menuWaitStarted, menuWaitDone, menuBeginPending := false, false, false
	menuSnapshots, menuStart, menuLastStep := 0, 0, 0
	var menuDS, menuSS uint16
	var menuFrame, menuReturnESP uint32
	dumpMenuTable := func(step int, kind string) {
		if !menuWaitStarted || menuSnapshots >= 16 {
			return
		}
		c := m.CPU
		r, seg, eip, flags, bus := c.R, c.Seg, c.EIP, c.EFlags, c.Bus
		control, status, depth, fpu := c.FPUControl, c.FPUStatus, c.FPUDepth, c.FPUStack
		state, ramBefore := m.VBEState(), sha256.Sum256(m.Mem)
		mask, pending, active, started, completed := services.MouseCallbackState()
		irqActive, irqFailed, irqStarted, irqCompleted := services.IRQ0State()
		var globals [192]byte
		var header, code [16]byte
		var frame [320]byte
		var ret [4]byte
		globalsReadable := peekSourceWindow(menuDS, 0x26c480, globals[:])
		headerReadable := peekSourceWindow(menuDS, 0x29be0e, header[:])
		frameReadable := peekSourceWindow(menuSS, menuFrame, frame[:])
		returnReadable := menuReturnESP >= 4 && peekSourceWindow(menuSS, menuReturnESP-4, ret[:])
		codeReadable := uint64(0x20ddf2)+16 <= uint64(len(m.Mem))
		if codeReadable {
			copy(code[:], m.Mem[0x20ddf2:0x20ddf2+16])
		}
		pointer := uint32(globals[0]) | uint32(globals[1])<<8 | uint32(globals[2])<<16 | uint32(globals[3])<<24
		count := uint16(header[0]) | uint16(header[1])<<8
		bias := uint32(header[4]) | uint32(header[5])<<8 | uint32(header[6])<<16 | uint32(header[7])<<24
		var records []byte
		tableReadable := false
		if globalsReadable && headerReadable && count <= 16 {
			records = make([]byte, int(count)*55)
			tableReadable = peekSourceWindow(menuDS, pointer, records)
		}
		ramAfter := sha256.Sum256(m.Mem)
		afterMask, afterPending, afterActive, afterStarted, afterCompleted := services.MouseCallbackState()
		afterIRQActive, afterIRQFailed, afterIRQStarted, afterIRQCompleted := services.IRQ0State()
		readonly := c.R == r && c.Seg == seg && c.EIP == eip && c.EFlags == flags && c.Bus == bus && c.FPUControl == control && c.FPUStatus == status && c.FPUDepth == depth && m.VBEState() == state && ramBefore == ramAfter && mask == afterMask && pending == afterPending && active == afterActive && started == afterStarted && completed == afterCompleted && irqActive == afterIRQActive && irqFailed == afterIRQFailed && irqStarted == afterIRQStarted && irqCompleted == afterIRQCompleted
		for j := range fpu {
			readonly = readonly && math.Float64bits(fpu[j]) == math.Float64bits(c.FPUStack[j])
		}
		if !readonly {
			panic("原表唯讀快照改變原始狀態")
		}
		menuSnapshots++
		fmt.Printf("new_game_menu_table_snapshot kind=%s outer_step=%d address_space=dosgolem_high_le eip=%X r=%X seg=%X flags=%X fpu_control=%X fpu_status=%X fpu_depth=%d fpu_stack=%v state=%+v ds=%X globals_readable=%t globals=%X header_readable=%t header=%X table_pointer=%X table_count=%d table_bias=%X table_stride=55 table_readable=%t records=%X frame_selector=%X frame_offset=%X frame_readable=%t frame=%X return_eip=20DDF7 return_esp=%X return_selector=%X return_readable=%t return_bytes=%X code_offset=20DDF2 code_readable=%t code=%X waiting=%t callback_mask=%X callback_pending=%d callback_active=%t callback_started=%d callback_completed=%d irq_active=%t irq_failed=%t irq_started=%d irq_completed=%d ram_before_sha256=%x ram_after_sha256=%x readonly=%t samples=%d\n", kind, step, eip, r, seg, flags, control, status, depth, fpu, state, menuDS, globalsReadable, globals, headerReadable, header, pointer, count, bias, tableReadable, records, menuSS, menuFrame, frameReadable, frame, menuReturnESP, menuSS, returnReadable, ret, codeReadable, code, !menuWaitDone, mask, pending, active, started, completed, irqActive, irqFailed, irqStarted, irqCompleted, ramBefore, ramAfter, readonly, menuSnapshots)
	}
	observeMenuTable := func(step int) {
		menuLastStep = step
		if !menuWaitStarted || menuWaitDone {
			return
		}
		if m.CPU.EIP == 0x20ddf7 && m.CPU.Seg[cpu386.SegSS] == menuSS && m.CPU.R[cpu386.ESP] == menuReturnESP {
			menuWaitDone = true
			dumpMenuTable(step, "natural_return")
		} else if menuBeginPending {
			menuBeginPending = false
			dumpMenuTable(step, "call_begin")
		} else if step >= 49500000 && step <= 50000000 && step%100000 == 0 || step > 50000000 && step%10000000 == 0 {
			dumpMenuTable(step, "checkpoint")
		}
	}
	defer func() {
		if menuWaitStarted {
			dumpMenuTable(menuLastStep, "terminal")
			fmt.Printf("new_game_menu_table_terminal seen=true start=%d samples=%d max_samples=16 waiting=%t return_eip=20DDF7 return_selector=%X return_esp=%X\n", menuStart, menuSnapshots, !menuWaitDone, menuSS, menuReturnESP)
		}
	}()

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

	dumpExtendedProgress := func(step int) {
		if maxSteps != 100000000 && !(bannerRed && maxSteps == 120000000) && !universe160 || phasePrefix == "" || !newGameClick || !newGameReleased || step < 50000000 || step > maxSteps || step%10000000 != 0 {
			return
		}
		c := m.CPU
		r, seg, eip, flags, bus := c.R, c.Seg, c.EIP, c.EFlags, c.Bus
		control, status, stack, depth := c.FPUControl, c.FPUStatus, c.FPUStack, c.FPUDepth
		state, ramBefore := m.VBEState(), sha256.Sum256(m.Mem)
		pixels, rgb := m.VBEIndexed(), m.VBERGB()
		if len(pixels) != 640*480 || len(rgb) != 640*480*3 {
			panic("續行VBE快照尺寸錯誤")
		}
		out := image.NewNRGBA(image.Rect(0, 0, 640, 480))
		for pixel := range pixels {
			copy(out.Pix[pixel*4:pixel*4+3], rgb[pixel*3:pixel*3+3])
			out.Pix[pixel*4+3] = 255
		}
		path := fmt.Sprintf("%s-extended-%08d.png", phasePrefix, step)
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
		readonly := c.R == r && c.Seg == seg && c.EIP == eip && c.EFlags == flags && c.Bus == bus && c.FPUControl == control && c.FPUStatus == status && c.FPUDepth == depth && m.VBEState() == state && ramBefore == ramAfter
		for j := range stack {
			readonly = readonly && math.Float64bits(stack[j]) == math.Float64bits(c.FPUStack[j])
		}
		if !readonly {
			panic("續行快照改變原始狀態")
		}
		micros := uint64(0)
		if ports, ok := services.DPMI.RealModeIO.(*machine.LEOPLPorts); ok {
			micros = ports.BIOSClock.Micros
		}
		fmt.Printf("extended_new_game_checkpoint outer_step=%d address_space=dosgolem_high_le eip=%X r=%X seg=%X flags=%X fpu_control=%X fpu_status=%X fpu_depth=%d fpu_stack=%v virtual_micros=%d state=%+v indexed_sha256=%x rgb_sha256=%x png_sha256=%x ram_before_sha256=%x ram_after_sha256=%x path=%s readonly=%t\n", step, eip, r, seg, flags, control, status, depth, stack, micros, state, sha256.Sum256(pixels), sha256.Sum256(rgb), sha256.Sum256(encoded), ramBefore, ramAfter, path, readonly)
		if publishBus != nil {
			mask, pending, active, started, completed := services.MouseCallbackState()
			fmt.Printf("extended_new_game_counters outer_step=%d publish_steps=%d reads=%d writes=%d errors=%d target_reads=%v target_writes=%v source_reads=%d vbe_writes=%d bus_matches=%t callback_mask=%X callback_pending=%d callback_active=%t callback_started=%d callback_completed=%d callback_samples=%d button_requests=%v positives=%v target_samples=%d\n", step, publishBus.steps, publishBus.reads, publishBus.writes, publishBus.errors, publishBus.targetReads, publishBus.targetWrites, publishBus.sourceReads, publishBus.vbeWrites, m.CPU.Bus == publishBus, mask, pending, active, started, completed, newGameCallbackSamples, buttonRequests, buttonPositiveCounts, buttonReadSamples)
		}
	}

	for i := 0; i < maxSteps; i++ {
		loopStep = i
		observeMenuTable(i)
		dumpPostClickProgress(i)
		dumpExtendedProgress(i)
		dumpSetupTable(i)
		// 346 BEGIN universe160_call
		dumpUniverseCheckpoint(i)
		// 346 END universe160_call

		// 352 BEGIN 180_call
		dumpGenerationContinuationCheckpoint(i)
		// 352 END 180_call

		// 347 BEGIN text_call
		observeProgressText(i)
		// 347 END text_call

		// 348 BEGIN home_return_call
		observeHomeReturn(i)
		// 348 END home_return_call

		// 349 BEGIN generation_call
		observeGenerationEntry(i)
		// 349 END generation_call

		// 350 BEGIN iteration_call
		observeGenerationIteration(i)
		// 350 END iteration_call

		// 351 BEGIN completion_call
		observeGenerationCompletion(i)
		// 351 END completion_call

		if i == 49500000 && newGameClick && newGameReleased && phasePrefix != "" {
			selector := m.CPU.Seg[cpu386.SegDS]
			desc, known := m.CPU.Descriptors[selector]
			valid := known && uint64(0x499303)+1 <= uint64(desc.Limit)+1 && uint64(desc.Base)+0x499303+1 <= uint64(len(m.Mem)) && uint64(0x3ddd71)+1 <= uint64(desc.Limit)+1 && uint64(desc.Base)+0x3ddd71+1 <= uint64(len(m.Mem))
			if valid {
				publishBegin = m.VBEState()
				publishBus = &publishBusObserver{Bus: m.CPU.Bus, targets: [2]uint32{desc.Base + 0x499300, desc.Base + 0x499303}, sourceStart: desc.Base + 0x3ddd67, sourceEnd: desc.Base + 0x3ddd72}
				m.CPU.Bus = publishBus
			}
			fmt.Printf("post_click_publish_begin outer_step=%d selector=%X descriptor_base=%X descriptor_limit=%X readable=%t state=%+v\n", i, selector, desc.Base, desc.Limit, valid, m.VBEState())
		}
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

		if readyClick && (!readyPressed && i == 50000000 || readyPressed && !readyReleased) {
			ports, ok := services.DPMI.RealModeIO.(*machine.LEOPLPorts)
			mask, pending, active, started, completed := services.MouseCallbackState()
			selector, offset := services.MouseCallbackTarget()
			irqActive, irqFailed, irqStarted, irqCompleted := services.IRQ0State()
			available := ok && ports.BIOSClock != nil && m.CPU.EFlags&cpu386.IF != 0 && pending == 0 && !active && !irqActive && !irqFailed && selector == 8 && offset == 0x2136d1 && mask == 0x2b
			phase, x, buttons := "", uint16(1000), uint16(1)
			if !readyPressed {
				c := m.CPU
				r, seg, eip, flags, bus := c.R, c.Seg, c.EIP, c.EFlags, c.Bus
				ramBefore := sha256.Sum256(m.Mem)
				var globals [192]byte
				var header [16]byte
				var records [385]byte
				g := peekSourceWindow(seg[cpu386.SegDS], 0x26c480, globals[:])
				h := peekSourceWindow(seg[cpu386.SegDS], 0x29be0e, header[:])
				t := peekSourceWindow(seg[cpu386.SegDS], 0x298848, records[:])
				readonly := c.R == r && c.Seg == seg && c.EIP == eip && c.EFlags == flags && c.Bus == bus && sha256.Sum256(m.Mem) == ramBefore
				valid := readonly && available && newGamePressed && newGameReleased && started == 2 && completed == 2 && seg[cpu386.SegDS] == 0x188 && g && h && t && globals[0] == 0x48 && globals[1] == 0x88 && globals[2] == 0x29 && globals[3] == 0 && header[0] == 7 && header[1] == 0 && header[4] == 0 && header[5] == 0 && header[6] == 0 && header[7] == 0 && fmt.Sprintf("%x", sha256.Sum256(records[:])) == "776c6e6e61a5b17529ff383cae79a194edc17c7cf0b9a11f3dc8fc8841339183"
				fmt.Printf("ready_menu_precondition outer_step=%d ds=%X globals=%X header=%X records=%X table_pointer=298848 table_count=7 table_stride=55 table_readable=%t readonly=%t callback_mask=%X callback_pending=%d callback_active=%t callback_started=%d callback_completed=%d irq_active=%t irq_failed=%t irq_started=%d irq_completed=%d eip=%X r=%X seg=%X flags=%X valid=%t\n", i, seg[cpu386.SegDS], globals, header, records, g && h && t, readonly, mask, pending, active, started, completed, irqActive, irqFailed, irqStarted, irqCompleted, eip, r, seg, flags, valid)
				if !valid {
					panic("正式選單點擊的原表或輸入條件不符")
				}
				phase = "press"
			} else if available && completed >= readyPressStarted+1 && ports.BIOSClock.Micros >= readyPressMicros+20000 {
				phase, x, buttons = "release", 1002, 0
			}
			if phase != "" {
				if err := services.InjectMouseEvent(x, 229, buttons, 0, 0); err != nil {
					panic(err)
				}
				fmt.Printf("ready_menu_mouse_input phase=%s outer_step=%d virtual_micros=%d x=%d y=229 buttons=%d delta=0/0 mask=%X target=%X:%X started=%d completed=%d eip=%X r=%X seg=%X flags=%X\n", phase, i, ports.BIOSClock.Micros, x, buttons, mask, selector, offset, started, completed, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags)
				if phase == "press" {
					readyPressed, readyPressMicros, readyPressStarted = true, ports.BIOSClock.Micros, started
				} else {
					readyReleased = true
				}
			}
		}

		if setupAccept && (!setupPressed && i == 80000000 || setupPressed && !setupReleased) {
			ports, ok := services.DPMI.RealModeIO.(*machine.LEOPLPorts)
			mask, pending, active, started, completed := services.MouseCallbackState()
			selector, offset := services.MouseCallbackTarget()
			irqActive, irqFailed, irqStarted, irqCompleted := services.IRQ0State()
			available := ok && ports.BIOSClock != nil && m.CPU.EFlags&cpu386.IF != 0 && pending == 0 && !active && !irqActive && !irqFailed && selector == 8 && offset == 0x2136d1 && mask == 0x2b
			phase, x, buttons := "", uint16(960), uint16(1)
			if !setupPressed {
				c := m.CPU
				r, seg, eip, flags := c.R, c.Seg, c.EIP, c.EFlags
				ramBefore := sha256.Sum256(m.Mem)
				var globals [192]byte
				var header [16]byte
				var records [935]byte
				g, h, t := false, false, false
				var rgbHash [32]byte
				readonly := activationPeek(func() {
					g = peekSourceWindow(seg[cpu386.SegDS], 0x26c480, globals[:])
					h = peekSourceWindow(seg[cpu386.SegDS], 0x29be0e, header[:])
					t = peekSourceWindow(seg[cpu386.SegDS], 0x298848, records[:])
					rgbHash = sha256.Sum256(m.VBERGB())
				}) && sha256.Sum256(m.Mem) == ramBefore
				valid := readonly && available && readyPressed && readyReleased && readyStoreSeen && started == 4 && completed == 4 && seg[cpu386.SegDS] == 0x188 && g && h && t && globals[0] == 0x48 && globals[1] == 0x88 && globals[2] == 0x29 && globals[3] == 0 && header[0] == 17 && header[1] == 0 && header[4] == 0 && header[5] == 0 && header[6] == 0 && header[7] == 0 && fmt.Sprintf("%x", sha256.Sum256(records[:])) == "2a18a0213dcb1d539de3175c8356b8c8b886c1d61b38f63795b3fa1859a3f52f" && fmt.Sprintf("%x", rgbHash) == "3bbe6c339cfbc74e84b3210bccfdc4574073b4d308ef9b90a1720cc5c74e623e"
				fmt.Printf("setup_accept_precondition outer_step=%d address_space=dosgolem_high_le ds=%X globals=%X header=%X records=%X table_pointer=298848 table_count=17 table_stride=55 table_readable=%t rgb_sha256=%x readonly=%t callback_mask=%X callback_pending=%d callback_active=%t callback_started=%d callback_completed=%d irq_active=%t irq_failed=%t irq_started=%d irq_completed=%d eip=%X r=%X seg=%X flags=%X valid=%t\n", i, seg[cpu386.SegDS], globals, header, records, g && h && t, rgbHash, readonly, mask, pending, active, started, completed, irqActive, irqFailed, irqStarted, irqCompleted, eip, r, seg, flags, valid)
				if !valid {
					panic("設定頁ACCEPT點擊的原表或輸入條件不符")
				}
				phase = "press"
			} else if available && completed >= setupPressStarted+1 && ports.BIOSClock.Micros >= setupPressMicros+20000 {
				phase, x, buttons = "release", 962, 0
			}
			if phase != "" {
				if err := services.InjectMouseEvent(x, 400, buttons, 0, 0); err != nil {
					panic(err)
				}
				fmt.Printf("setup_accept_mouse_input phase=%s outer_step=%d virtual_micros=%d x=%d y=400 buttons=%d delta=0/0 mask=%X target=%X:%X started=%d completed=%d eip=%X r=%X seg=%X flags=%X\n", phase, i, ports.BIOSClock.Micros, x, buttons, mask, selector, offset, started, completed, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags)
				if phase == "press" {
					setupPressed, setupPressMicros, setupPressStarted = true, ports.BIOSClock.Micros, started
				} else {
					setupReleased = true
				}
			}
		}

		if raceHumans && (!racePressed && i == 90000000 || racePressed && !raceReleased) {
			ports, ok := services.DPMI.RealModeIO.(*machine.LEOPLPorts)
			mask, pending, active, started, completed := services.MouseCallbackState()
			selector, offset := services.MouseCallbackTarget()
			irqActive, irqFailed, irqStarted, irqCompleted := services.IRQ0State()
			available := ok && ports.BIOSClock != nil && m.CPU.EFlags&cpu386.IF != 0 && pending == 0 && !active && !irqActive && !irqFailed && selector == 8 && offset == 0x2136d1 && mask == 0x2b
			phase, x, buttons := "", uint16(824), uint16(1)
			if !racePressed {
				c := m.CPU
				r, seg, eip, flags := c.R, c.Seg, c.EIP, c.EFlags
				ramBefore := sha256.Sum256(m.Mem)
				var globals [192]byte
				var header [16]byte
				var records [880]byte
				g, h, t := false, false, false
				var rgbHash [32]byte
				readonly := activationPeek(func() {
					g = peekSourceWindow(seg[cpu386.SegDS], 0x26c480, globals[:])
					h = peekSourceWindow(seg[cpu386.SegDS], 0x29be0e, header[:])
					t = peekSourceWindow(seg[cpu386.SegDS], 0x298848, records[:])
					rgbHash = sha256.Sum256(m.VBERGB())
				}) && sha256.Sum256(m.Mem) == ramBefore
				valid := readonly && available && setupPressed && setupReleased && setupStoreSeen && started == 6 && completed == 6 && seg[cpu386.SegDS] == 0x188 && g && h && t && globals[0] == 0x48 && globals[1] == 0x88 && globals[2] == 0x29 && globals[3] == 0 && header[0] == 16 && header[1] == 0 && header[4] == 0 && header[5] == 0 && header[6] == 0 && header[7] == 0 && fmt.Sprintf("%x", sha256.Sum256(records[:])) == "f03515b12cb289bfcfe49b46d5cf8619f8f4e300ccfd1bd4ca43107f1c313ade" && fmt.Sprintf("%x", rgbHash) == "9d8c0a1acb3b96200296f789bb13c6877067f6165ec608a7e019506036832dfc"
				fmt.Printf("race_humans_precondition outer_step=%d address_space=dosgolem_high_le ds=%X globals=%X header=%X records=%X table_pointer=298848 table_count=16 table_stride=55 table_readable=%t rgb_sha256=%x readonly=%t callback_mask=%X callback_pending=%d callback_active=%t callback_started=%d callback_completed=%d irq_active=%t irq_failed=%t irq_started=%d irq_completed=%d eip=%X r=%X seg=%X flags=%X valid=%t\n", i, seg[cpu386.SegDS], globals, header, records, g && h && t, rgbHash, readonly, mask, pending, active, started, completed, irqActive, irqFailed, irqStarted, irqCompleted, eip, r, seg, flags, valid)
				if !valid {
					panic("選族Humans點擊的原表或輸入條件不符")
				}
				phase = "press"
			} else if available && completed >= racePressStarted+1 && ports.BIOSClock.Micros >= racePressMicros+20000 {
				phase, x, buttons = "release", 826, 0
			}
			if phase != "" {
				if err := services.InjectMouseEvent(x, 352, buttons, 0, 0); err != nil {
					panic(err)
				}
				fmt.Printf("race_humans_mouse_input phase=%s outer_step=%d virtual_micros=%d x=%d y=352 buttons=%d delta=0/0 mask=%X target=%X:%X started=%d completed=%d eip=%X r=%X seg=%X flags=%X\n", phase, i, ports.BIOSClock.Micros, x, buttons, mask, selector, offset, started, completed, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags)
				if phase == "press" {
					racePressed, racePressMicros, racePressStarted = true, ports.BIOSClock.Micros, started
				} else {
					raceReleased = true
				}
			}
		}

		if rulerAccept && (!rulerPressed && i == 95000000 || rulerPressed && !rulerReleased) {
			ports, ok := services.DPMI.RealModeIO.(*machine.LEOPLPorts)
			mask, pending, active, started, completed := services.MouseCallbackState()
			selector, offset := services.MouseCallbackTarget()
			irqActive, irqFailed, irqStarted, irqCompleted := services.IRQ0State()
			available := ok && ports.BIOSClock != nil && m.CPU.EFlags&cpu386.IF != 0 && pending == 0 && !active && !irqActive && !irqFailed && selector == 8 && offset == 0x2136d1 && (mask == 0x2b || rulerPressed && mask == 1)
			phase, x, buttons := "", uint16(640), uint16(1)
			if !rulerPressed {
				c := m.CPU
				r, seg, eip, flags := c.R, c.Seg, c.EIP, c.EFlags
				ramBefore := sha256.Sum256(m.Mem)
				var globals [192]byte
				var header [16]byte
				var records [165]byte
				var candidate [32]byte
				candidateReadable := false
				g, h, t := false, false, false
				var rgbHash [32]byte
				readonly := activationPeek(func() {
					g = peekSourceWindow(seg[cpu386.SegDS], 0x26c480, globals[:])
					h = peekSourceWindow(seg[cpu386.SegDS], 0x29be0e, header[:])
					t = peekSourceWindow(seg[cpu386.SegDS], 0x298848, records[:])
					candidateReadable = peekSourceWindow(seg[cpu386.SegDS], 0x28439d, candidate[:])
					rgbHash = sha256.Sum256(m.VBERGB())
				}) && sha256.Sum256(m.Mem) == ramBefore
				valid := readonly && available && eip == 0x215d9e && flags == 0x202 && r == [8]uint32{0xa0, 0x178, 0x2c0864, 7, 0x2bd94c, 0x2bd970, 0x2843a5, 0x28439d} && seg == [6]uint16{8, 0x188, 0x188, 0, 0x20, 0x188} && c.FPUControl == 0x127f && c.FPUStatus == 0 && c.FPUDepth == 0 && candidateReadable && fmt.Sprintf("%X", candidate) == "5374726164657200000000000000000000000000000000000000000000000000" && racePressed && raceReleased && raceStoreSeen && started == 8 && completed == 8 && irqStarted == 21395 && irqCompleted == 21395 && seg[cpu386.SegDS] == 0x188 && g && h && t && globals[0] == 0x48 && globals[1] == 0x88 && globals[2] == 0x29 && globals[3] == 0 && header[0] == 3 && header[1] == 0 && header[4] == 0 && header[5] == 0 && header[6] == 0 && header[7] == 0 && fmt.Sprintf("%x", sha256.Sum256(records[:])) == "f3dc28cf153edf625b154c5864b3040cddd52fc9ade2a0cfd2a256b80789d0a7" && fmt.Sprintf("%x", rgbHash) == "2646a7ef25939869ede60aa35bcd97d649b906fd2cc287d08deb6c2d8058cc12"
				fmt.Printf("ruler_name_accept_precondition outer_step=%d address_space=dosgolem_high_le ds=%X globals=%X header=%X records=%X candidate_offset=28439D candidate_readable=%t candidate=%X table_pointer=298848 table_count=3 table_stride=55 table_readable=%t rgb_sha256=%x readonly=%t callback_mask=%X callback_pending=%d callback_active=%t callback_started=%d callback_completed=%d irq_active=%t irq_failed=%t irq_started=%d irq_completed=%d eip=%X r=%X seg=%X flags=%X valid=%t\n", i, seg[cpu386.SegDS], globals, header, records, candidateReadable, candidate, g && h && t, rgbHash, readonly, mask, pending, active, started, completed, irqActive, irqFailed, irqStarted, irqCompleted, eip, r, seg, flags, valid)
				if !valid {
					panic("名稱ACCEPT點擊的原表或輸入條件不符")
				}
				phase = "press"
			} else if available && completed >= rulerPressStarted+1 && ports.BIOSClock.Micros >= rulerPressMicros+20000 {
				phase, x, buttons = "release", 642, 0
			}
			if phase != "" {
				if err := services.InjectMouseEvent(x, 239, buttons, 0, 0); err != nil {
					panic(err)
				}
				fmt.Printf("ruler_name_accept_mouse_input phase=%s outer_step=%d virtual_micros=%d x=%d y=239 buttons=%d delta=0/0 mask=%X target=%X:%X started=%d completed=%d eip=%X r=%X seg=%X flags=%X\n", phase, i, ports.BIOSClock.Micros, x, buttons, mask, selector, offset, started, completed, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags)
				if phase == "press" {
					rulerPressed, rulerPressMicros, rulerPressStarted = true, ports.BIOSClock.Micros, started
				} else {
					rulerReleased = true
				}
			}
		}
		// 339 BEGIN input
		if bannerRed && (!bannerPressed && i == 99000000 || bannerPressed && !bannerReleased) {
			ports, ok := services.DPMI.RealModeIO.(*machine.LEOPLPorts)
			mask, pending, active, started, completed := services.MouseCallbackState()
			selector, offset := services.MouseCallbackTarget()
			irqActive, irqFailed, irqStarted, irqCompleted := services.IRQ0State()
			available := ok && ports.BIOSClock != nil && m.CPU.EFlags&cpu386.IF != 0 && pending == 0 && !active && !irqActive && !irqFailed && selector == 8 && offset == 0x2136d1 && (mask == 0x2b || bannerPressed && mask == 1)
			phase, x, buttons := "", uint16(276), uint16(1)
			if !bannerPressed {
				c := m.CPU
				r, seg, eip, flags := c.R, c.Seg, c.EIP, c.EFlags
				ramBefore := sha256.Sum256(m.Mem)
				var globals [192]byte
				var header [16]byte
				var records [550]byte
				g, h, t := false, false, false
				var rgbHash [32]byte
				readonly := activationPeek(func() {
					g = peekSourceWindow(seg[cpu386.SegDS], 0x26c480, globals[:])
					h = peekSourceWindow(seg[cpu386.SegDS], 0x29be0e, header[:])
					t = peekSourceWindow(seg[cpu386.SegDS], 0x298848, records[:])
					rgbHash = sha256.Sum256(m.VBERGB())
				}) && sha256.Sum256(m.Mem) == ramBefore
				state := m.VBEState()
				valid := readonly && available && eip == 0x238599 && flags == 0x206 && r == [8]uint32{0x35b824, 0x47, 0x55, 9, 0x2bd9b8, 0x2bda14, 0x6abc38, 0x35b82d} && seg == [6]uint16{8, 0x188, 0x188, 0, 0x20, 0x188} && c.FPUControl == 0x127f && c.FPUStatus == 0 && c.FPUDepth == 0 && c.FPUStack == [8]float64{} && rulerPressed && rulerReleased && started == 10 && completed == 10 && irqStarted == 22553 && irqCompleted == 22553 && ports.BIOSClock.Micros == 177207342 && state.Active && state.Bank == 4 && state.StartY == 0 && state.BankSets == 1511 && state.Writes == 35010112 && state.DisplaySets == 48 && g && h && t && fmt.Sprintf("%x", sha256.Sum256(globals[:])) == "66d6e38de2de33b12053cb7686087b7e6882007822e92b2e5262980149310594" && fmt.Sprintf("%x", sha256.Sum256(header[:])) == "0943dfdc49b8bd9a5899e8155bd7b98553f3a0dfa5402eeb37ad6a41261711d9" && fmt.Sprintf("%x", sha256.Sum256(records[:])) == "978afb91aaed9e0cb37672a66352e2ae515b382d5b6f472da9238e09fb650dfb" && fmt.Sprintf("%x", rgbHash) == "8c2c573c08ebf4bce0b4e166d4268fb6fcf90f35dd27fbbb2de614b26f187f15"
				fmt.Printf("banner_red_precondition outer_step=%d address_space=dosgolem_high_le ds=%X globals=%X header=%X records=%X table_pointer=298848 table_count=10 table_stride=55 table_readable=%t rgb_sha256=%x readonly=%t callback_mask=%X callback_pending=%d callback_active=%t callback_started=%d callback_completed=%d irq_active=%t irq_failed=%t irq_started=%d irq_completed=%d eip=%X r=%X seg=%X flags=%X valid=%t\n", i, seg[cpu386.SegDS], globals, header, records, g && h && t, rgbHash, readonly, mask, pending, active, started, completed, irqActive, irqFailed, irqStarted, irqCompleted, eip, r, seg, flags, valid)
				if !valid {
					panic("旗幟紅色點擊的原表或輸入條件不符")
				}
				phase = "press"
			} else if available && bannerPolled && bannerGUIButtonSeen && bannerSelectionGateSeen && completed >= bannerPressStarted+1 && ports.BIOSClock.Micros >= bannerPressMicros+20000 {
				phase, x, buttons = "release", 278, 0
			}
			if phase != "" {
				if err := services.InjectMouseEvent(x, 190, buttons, 0, 0); err != nil {
					panic(err)
				}
				fmt.Printf("banner_red_mouse_input phase=%s outer_step=%d virtual_micros=%d x=%d y=190 buttons=%d delta=0/0 mask=%X target=%X:%X started=%d completed=%d eip=%X r=%X seg=%X flags=%X\n", phase, i, ports.BIOSClock.Micros, x, buttons, mask, selector, offset, started, completed, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags)
				if phase == "press" {
					bannerPressed, bannerPressMicros, bannerPressStarted = true, ports.BIOSClock.Micros, started
				} else {
					bannerReleased = true
				}
			}
		}
		// 339 END input

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

		branchBegin, branchResumed, branchSkipped := false, false, 0
		_, _, branchCallback, _, _ := services.MouseCallbackState()
		branchIRQActive, branchIRQFailed, branchIRQStarted, branchIRQCompleted := services.IRQ0State()
		if phasePrefix != "" && newGameClick && newGameReleased && !branchCallback && !branchIRQActive && !branchSeen && m.CPU.EIP == 0x20db87 {
			branchSeen, branchActive, branchBegin, branchStart = true, true, true, i
			branchSelector, branchStackSelector = m.CPU.Seg[cpu386.SegDS], m.CPU.Seg[cpu386.SegSS]
			branchStackOffset = m.CPU.R[cpu386.EBP] - 160
		}
		if !branchTail && branchSeen && !branchActive && branchSamples == 96 && branchStop == "sample_budget" && phasePrefix != "" && newGameClick && newGameReleased && !branchCallback && !branchIRQActive && m.CPU.EIP == 0x20dcbd {
			branchOldTerminal = branchTerminal("new_game_button_branch_terminal", 96)
			branchTail, branchActive, branchBegin = true, true, true
			branchMaxSamples, branchSamples, branchStart, branchStop = 384, 0, i, ""
			branchWaiting = false
			branchSelector, branchStackSelector = m.CPU.Seg[cpu386.SegDS], m.CPU.Seg[cpu386.SegSS]
			branchStackOffset = m.CPU.R[cpu386.EBP] - 160
		}
		if branchActive && i-branchStart >= 8192 {
			branchActive, branchStop = false, "outer_budget"
		}
		if branchActive && branchWaiting && m.CPU.EIP == branchReturnEIP && m.CPU.Seg[cpu386.SegSS] == branchReturnSelector && m.CPU.R[cpu386.ESP] == branchReturnESP && !branchCallback && !branchIRQActive {
			branchWaiting, branchResumed, branchSkipped = false, true, i-branchCallStep-1
		}
		if branchActive && !branchWaiting && (branchCallback || branchIRQActive || branchIRQFailed) {
			branchActive, branchStop = false, "interrupt_before"
		}
		observeBranch := branchActive && !branchWaiting && branchSamples < branchMaxSamples
		var branchBeforeEvent [16]byte
		var branchBeforeCounter, branchBeforeTop [4]byte
		var branchBeforeGlobals [192]byte
		var branchBeforeHeader [16]byte
		var branchBeforeSource [8]byte
		var branchBeforeStack [320]byte
		var branchBeforePointer [64]byte
		branchEventReadable, branchCounterReadable, branchGlobalsReadable, branchStackReadable, branchTopReadable, branchPointerReadable, branchHeaderReadable, branchSourceReadable := false, false, false, false, false, false, false, false
		branchBeforeReadonly := true
		branchPointerOffset := m.CPU.R[cpu386.EBX]
		branchSourceOffset := m.CPU.R[cpu386.EAX]
		if observeBranch {
			branchBeforeReadonly = activationPeek(func() {
				branchEventReadable = peekSourceWindow(branchSelector, 0x2a121a, branchBeforeEvent[:])
				branchCounterReadable = peekSourceWindow(branchSelector, 0x2a11ec, branchBeforeCounter[:])
				branchGlobalsReadable = peekSourceWindow(branchSelector, 0x26c480, branchBeforeGlobals[:])
				branchHeaderReadable = peekSourceWindow(branchSelector, 0x29be0e, branchBeforeHeader[:])
				branchSourceReadable = peekSourceWindow(branchSelector, branchSourceOffset, branchBeforeSource[:])
				branchStackReadable = branchStackOffset <= ^uint32(0)-320 && peekSourceWindow(branchStackSelector, branchStackOffset, branchBeforeStack[:])
				branchTopReadable = peekSourceWindow(m.CPU.Seg[cpu386.SegSS], m.CPU.R[cpu386.ESP], branchBeforeTop[:])
				branchPointerReadable = peekSourceWindow(branchSelector, branchPointerOffset, branchBeforePointer[:])
			})
		}

		activationBegin := false
		_, _, activationCallback, _, _ := services.MouseCallbackState()
		if phasePrefix != "" && newGameClick && newGamePressed && newGameReleased && !activationCallback && activationBudget == 0 {
			group, offset := -1, uint32(0)
			if m.CPU.EIP == 0x213c60 {
				group, offset = 0, 0x2a1228
			}
			if m.CPU.EIP == 0x213a7c {
				group, offset = 1, 0x2a1226
			}
			if group >= 0 && !activationSeen[group] {
				var value [2]byte
				known := false
				activationPeek(func() { known = peekSourceWindow(m.CPU.Seg[cpu386.SegDS], offset, value[:]) })
				if known && value == [2]byte{1, 0} {
					activationSeen[group], activationGroup, activationBudget, activationBegin = true, group, 48, true
					activationReturned = false
					activationSelector, activationStackSelector = m.CPU.Seg[cpu386.SegDS], m.CPU.Seg[cpu386.SegSS]
					activationStackOffset = m.CPU.R[cpu386.ESP] - 16
				}
			}
		}
		observeActivation := activationBudget > 0
		var activationBeforeEvent [16]byte
		var activationBeforeCounter, activationBeforeGlobal, activationReturnBytes [4]byte
		var activationBeforeStack [64]byte
		activationEventReadable, activationCounterReadable, activationGlobalReadable, activationStackReadable, activationReturnReadable := false, false, false, false, false
		activationBeforeReadonly := true
		if observeActivation {
			activationBeforeReadonly = activationPeek(func() {
				activationEventReadable = peekSourceWindow(activationSelector, 0x2a121a, activationBeforeEvent[:])
				activationCounterReadable = peekSourceWindow(activationSelector, 0x2a11ec, activationBeforeCounter[:])
				activationGlobalReadable = peekSourceWindow(activationSelector, 0x26c518, activationBeforeGlobal[:])
				activationStackReadable = activationStackOffset <= ^uint32(0)-64 && peekSourceWindow(activationStackSelector, activationStackOffset, activationBeforeStack[:])
				activationReturnReadable = peekSourceWindow(m.CPU.Seg[cpu386.SegSS], m.CPU.R[cpu386.ESP], activationReturnBytes[:])
			})
		}

		sourceBegin := false
		if phasePrefix != "" && newGameClick && newGameReleased && i >= 49500000 && !sourceActive && m.CPU.EIP == 0x213345 {
			selector := m.CPU.Seg[cpu386.SegDS]
			var global [4]byte
			var value [1]byte
			known := peekSourceWindow(selector, 0x29be74, global[:])
			base := uint32(global[0]) | uint32(global[1])<<8 | uint32(global[2])<<16 | uint32(global[3])<<24
			index := m.CPU.R[cpu386.EAX]
			offset := base + index
			known = known && peekSourceWindow(selector, offset, value[:])
			kind := 3
			if known {
				kind = 0
				if value[0] == 0x80 {
					kind = 1
				} else if value[0] > 0x80 {
					kind = 2
				}
			}
			limit := 2
			if kind == 3 {
				limit = 1
			}
			if sourceGroups[kind] < limit {
				sourceGroups[kind]++
				sourceGroup++
				sourceKind = []string{"lt80", "eq80", "gt80", "unreadable"}[kind]
				sourceSelector, sourceBase, sourceIndex, sourceOffset = selector, base, index, offset
				sourceStackSelector = m.CPU.Seg[cpu386.SegSS]
				sourceStackOffset = m.CPU.R[cpu386.EBP] - 0x40
				sourceActive, sourceBudget, sourceBegin = true, 33, true
			}
		}
		observeSource := sourceActive && sourceBudget > 0
		var sourceBeforeGlobal [4]byte
		var sourceBeforeWindow, destinationBeforeWindow [5]byte
		var sourceBeforeStack [80]byte
		sourceGlobalReadable, sourceWindowReadable, sourceStackReadable, destinationReadable := false, false, false, false
		var destinationSelector uint16
		var destinationOffset uint32
		if observeSource {
			sourceGlobalReadable = peekSourceWindow(sourceSelector, 0x29be74, sourceBeforeGlobal[:])
			sourceWindowReadable = sourceOffset >= 2 && peekSourceWindow(sourceSelector, sourceOffset-2, sourceBeforeWindow[:])
			sourceStackReadable = m.CPU.R[cpu386.EBP] >= 0x40 && peekSourceWindow(sourceStackSelector, sourceStackOffset, sourceBeforeStack[:])
			if m.CPU.EIP == 0x213336 {
				destinationSelector = m.CPU.Seg[cpu386.SegDS]
				destinationOffset = m.CPU.R[cpu386.EDX]
				destinationReadable = destinationOffset >= 2 && peekSourceWindow(destinationSelector, destinationOffset-2, destinationBeforeWindow[:])
			}
		}

		_, clickPending, clickActive, clickStarted, clickCompleted := services.MouseCallbackState()

		scasWordBegin := readyClick && !scasWordSeen && m.CPU.EIP == 0x1f3640
		if scasWordBegin {
			scasWordSeen, scasWordBudget = true, 2
			scasWordSelector, scasWordStackSelector = m.CPU.Seg[cpu386.SegES], m.CPU.Seg[cpu386.SegSS]
			scasWordOffset, scasWordStackOffset = m.CPU.R[cpu386.EDI], m.CPU.R[cpu386.EBP]-4
			if m.CPU.R[cpu386.ECX] <= 256 && m.CPU.EFlags&cpu386.DF == 0 {
				scasWordSource = make([]byte, int(m.CPU.R[cpu386.ECX])*2)
			}
		}
		observeScasWord := scasWordBudget > 0
		var scasWordBefore []byte
		var scasWordBeforeStack [4]byte
		scasWordSourceReadable, scasWordStackReadable := false, false
		scasWordReadonly := false
		if observeScasWord {
			r, seg, eip, flags, bus := m.CPU.R, m.CPU.Seg, m.CPU.EIP, m.CPU.EFlags, m.CPU.Bus
			ramBefore := sha256.Sum256(m.Mem)
			scasWordBefore = append([]byte(nil), scasWordSource...)
			scasWordSourceReadable = scasWordSource != nil && peekSourceWindow(scasWordSelector, scasWordOffset, scasWordBefore)
			scasWordStackReadable = peekSourceWindow(scasWordStackSelector, scasWordStackOffset, scasWordBeforeStack[:])
			scasWordReadonly = m.CPU.R == r && m.CPU.Seg == seg && m.CPU.EIP == eip && m.CPU.EFlags == flags && m.CPU.Bus == bus && sha256.Sum256(m.Mem) == ramBefore
		}

		// 339 BEGIN store_pre

		observeBannerStore := bannerRed && bannerPressed && !bannerStoreSeen && m.CPU.EIP == 0x20dddb
		bannerBeforeR, bannerBeforeSeg, bannerBeforeEIP, bannerBeforeFlags := m.CPU.R, m.CPU.Seg, m.CPU.EIP, m.CPU.EFlags
		var bannerBeforeWord [2]byte
		var bannerBytes [16]byte
		bannerWordReadable := false
		if observeBannerStore {
			bannerWordReadable = peekSourceWindow(bannerBeforeSeg[cpu386.SegDS], 0x26c4a6, bannerBeforeWord[:])
			if uint64(bannerBeforeEIP)+16 <= uint64(len(m.Mem)) {
				copy(bannerBytes[:], m.Mem[bannerBeforeEIP:bannerBeforeEIP+16])
			}
		}
		// 339 END store_pre

		observeRulerStore := rulerAccept && rulerPressed && !rulerStoreSeen && m.CPU.EIP == 0x20dddb
		rulerBeforeR, rulerBeforeSeg, rulerBeforeEIP, rulerBeforeFlags := m.CPU.R, m.CPU.Seg, m.CPU.EIP, m.CPU.EFlags
		var rulerBeforeWord [2]byte
		var rulerBytes [16]byte
		var rulerBeforeCandidate [32]byte
		rulerBeforeCandidateReadable := false
		rulerWordReadable := false
		if observeRulerStore {
			rulerBeforeCandidateReadable = peekSourceWindow(rulerBeforeSeg[cpu386.SegDS], 0x28439d, rulerBeforeCandidate[:])
			rulerWordReadable = peekSourceWindow(rulerBeforeSeg[cpu386.SegDS], 0x26c4a6, rulerBeforeWord[:])
			if uint64(rulerBeforeEIP)+16 <= uint64(len(m.Mem)) {
				copy(rulerBytes[:], m.Mem[rulerBeforeEIP:rulerBeforeEIP+16])
			}
		}

		observeRaceStore := raceHumans && racePressed && !raceStoreSeen && m.CPU.EIP == 0x20dddb
		raceBeforeR, raceBeforeSeg, raceBeforeEIP, raceBeforeFlags := m.CPU.R, m.CPU.Seg, m.CPU.EIP, m.CPU.EFlags
		var raceBeforeWord [2]byte
		var raceBytes [16]byte
		raceWordReadable := false
		if observeRaceStore {
			raceWordReadable = peekSourceWindow(raceBeforeSeg[cpu386.SegDS], 0x26c4a6, raceBeforeWord[:])
			if uint64(raceBeforeEIP)+16 <= uint64(len(m.Mem)) {
				copy(raceBytes[:], m.Mem[raceBeforeEIP:raceBeforeEIP+16])
			}
		}
		observeSetupStore := setupAccept && setupPressed && !setupStoreSeen && m.CPU.EIP == 0x20dddb
		setupBeforeR, setupBeforeSeg, setupBeforeEIP, setupBeforeFlags := m.CPU.R, m.CPU.Seg, m.CPU.EIP, m.CPU.EFlags
		var setupBeforeWord [2]byte
		var setupBytes [16]byte
		setupWordReadable := false
		if observeSetupStore {
			setupWordReadable = peekSourceWindow(setupBeforeSeg[cpu386.SegDS], 0x26c4a6, setupBeforeWord[:])
			if uint64(setupBeforeEIP)+16 <= uint64(len(m.Mem)) {
				copy(setupBytes[:], m.Mem[setupBeforeEIP:setupBeforeEIP+16])
			}
		}
		observeReadyStore := readyClick && readyPressed && !readyStoreSeen && m.CPU.EIP == 0x20dddb
		var readyBeforeWord [2]byte
		readyWordReadable := false
		if observeReadyStore {
			readyWordReadable = peekSourceWindow(m.CPU.Seg[cpu386.SegDS], 0x26c4a6, readyBeforeWord[:])
		}
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
		if observeScasWord || observeReadyStore || observeClick || observeEventConsumer || observeFindQuestion || observeByteAdd || observeByteAddConsumer || observeNegDword || observeNegDwordConsumer || observeSource || observeActivation || observeBranch || newGameClick && newGamePressed && buttonReadSamples < 32 {
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
		if publishBus != nil {
			publishBus.active, publishBus.step, publishBus.eip = true, i, m.CPU.EIP
			publishBus.steps++
		}
		// 341 BEGIN gate_pre
		observeBannerGate := bannerRed && bannerPressed && bannerGUIButtonSeen && !bannerSelectionGateSeen && m.CPU.EIP == 0x214104
		if observeBannerGate {
			bannerGateR, bannerGateSeg, bannerGateFlags = m.CPU.R, m.CPU.Seg, m.CPU.EFlags
			ram := sha256.Sum256(m.Mem)
			bannerGateReadonly = activationPeek(func() {
				bannerGateReadable = peekSourceWindow(bannerGateSeg[cpu386.SegSS], bannerGateR[cpu386.ESP], bannerGateStack[:])
			}) && sha256.Sum256(m.Mem) == ram
		}
		// 341 END gate_pre

		// 340 BEGIN return_pre
		observeBannerReturn := bannerRed && bannerPressed && !bannerGUIButtonSeen && m.CPU.EIP == 0x214104
		if observeBannerReturn {
			bannerReturnR, bannerReturnSeg, bannerReturnFlags = m.CPU.R, m.CPU.Seg, m.CPU.EFlags
			ram := sha256.Sum256(m.Mem)
			bannerReturnReadonly = activationPeek(func() {
				bannerReturnReadable = peekSourceWindow(bannerReturnSeg[cpu386.SegSS], bannerReturnR[cpu386.ESP], bannerReturnStack[:])
			}) && sha256.Sum256(m.Mem) == ram
		}
		// 340 END return_pre

		// 342 BEGIN test_pre
		if bannerRed && !testMemorySeen && m.CPU.EIP == 0x184694 {
			testMemorySeen, testMemoryBudget = true, 3
		}
		observeTestMemory := testMemoryBudget > 0
		testR, testSeg, testEIP, testFlags := m.CPU.R, m.CPU.Seg, m.CPU.EIP, m.CPU.EFlags
		var testCode [16]byte
		var testSource, testStack [4]byte
		testOffset := testR[cpu386.EDX] + 0x265219
		testSourceReadable, testStackReadable := false, false
		var testRAM [32]byte
		testReadonly := false
		if observeTestMemory {
			testRAM = sha256.Sum256(m.Mem)
			testReadonly = activationPeek(func() {
				copy(testCode[:], m.Mem[testEIP:testEIP+16])
				testSourceReadable = peekSourceWindow(testSeg[cpu386.SegDS], testOffset, testSource[:])
				testStackReadable = peekSourceWindow(testSeg[cpu386.SegSS], testR[cpu386.ESP], testStack[:])
			}) && sha256.Sum256(m.Mem) == testRAM
		}
		// 342 END test_pre

		// 353 BEGIN and_pre
		if bannerRed && !andWordMemorySeen && m.CPU.EIP == 0x103bf9 {
			andWordMemorySeen, andWordMemoryBudget = true, 3
		}
		observeAndWordMemory := andWordMemoryBudget > 0
		andWordR, andWordSeg, andWordEIP, andWordFlags := m.CPU.R, m.CPU.Seg, m.CPU.EIP, m.CPU.EFlags
		var andWordCode [16]byte
		var andWordSource [8]byte
		var andWordStack [4]byte
		andWordOffset := andWordR[cpu386.EBX] + 0x0c - 1
		andWordSourceReadable, andWordStackReadable := false, false
		var andWordRAM [32]byte
		andWordReadonly := false
		var andWordRAMCopy []byte
		if observeAndWordMemory {
			andWordRAM = sha256.Sum256(m.Mem)
			andWordRAMCopy = append([]byte(nil), m.Mem...)
			andWordReadonly = activationPeek(func() {
				copy(andWordCode[:], m.Mem[andWordEIP:andWordEIP+16])
				andWordSourceReadable = peekSourceWindow(andWordSeg[cpu386.SegDS], andWordOffset, andWordSource[:])
				andWordStackReadable = peekSourceWindow(andWordSeg[cpu386.SegSS], andWordR[cpu386.ESP], andWordStack[:])
			}) && sha256.Sum256(m.Mem) == andWordRAM
		}
		// 353 END and_pre

		// 354 BEGIN xchg_pre
		if bannerRed && !xchgByteMemorySeen && m.CPU.EIP == 0x223e93 {
			xchgByteMemorySeen, xchgByteMemoryBudget = true, 2
		}
		observeXchgByteMemory := xchgByteMemoryBudget > 0
		xchgByteR, xchgByteSeg, xchgByteEIP, xchgByteFlags := m.CPU.R, m.CPU.Seg, m.CPU.EIP, m.CPU.EFlags
		var xchgByteCode [16]byte
		var xchgByteSource [3]byte
		var xchgByteDestination [3]byte
		xchgByteOffset := xchgByteR[cpu386.ESI] - 1
		xchgByteSourceReadable, xchgByteDestinationReadable := false, false
		var xchgByteRAM [32]byte
		xchgByteReadonly := false
		var xchgByteRAMCopy []byte
		if observeXchgByteMemory {
			xchgByteRAM = sha256.Sum256(m.Mem)
			xchgByteRAMCopy = append([]byte(nil), m.Mem...)
			xchgByteReadonly = activationPeek(func() {
				copy(xchgByteCode[:], m.Mem[xchgByteEIP:xchgByteEIP+16])
				xchgByteSourceReadable = peekSourceWindow(xchgByteSeg[cpu386.SegDS], xchgByteOffset, xchgByteSource[:])
				xchgByteDestinationReadable = peekSourceWindow(xchgByteSeg[cpu386.SegES], xchgByteR[cpu386.EDI]-1, xchgByteDestination[:])
			}) && sha256.Sum256(m.Mem) == xchgByteRAM
		}
		// 354 END xchg_pre

		// 357 BEGIN imul_word_pre
		if bannerRed && !imulWordSeen && m.CPU.EIP == 0x1cf90a {
			imulWordSeen, imulWordBudget = true, 3
		}
		observeImulWord := imulWordBudget > 0
		imulWordR, imulWordSeg, imulWordEIP, imulWordFlags := m.CPU.R, m.CPU.Seg, m.CPU.EIP, m.CPU.EFlags
		var imulWordCode [24]byte
		var imulWordSource [4]byte
		var imulWordDestination [20]byte
		imulWordOffset := imulWordR[cpu386.EBX] + 0x3f
		imulWordSourceReadable, imulWordDestinationReadable := false, false
		var imulWordRAM [32]byte
		imulWordReadonly := false
		var imulWordRAMCopy []byte
		if observeImulWord {
			imulWordRAM = sha256.Sum256(m.Mem)
			imulWordRAMCopy = append([]byte(nil), m.Mem...)
			imulWordReadonly = activationPeek(func() {
				copy(imulWordCode[:], m.Mem[imulWordEIP:imulWordEIP+24])
				imulWordSourceReadable = peekSourceWindow(imulWordSeg[cpu386.SegDS], imulWordOffset, imulWordSource[:])
				imulWordDestinationReadable = peekSourceWindow(imulWordSeg[cpu386.SegSS], imulWordR[cpu386.EBP]-44, imulWordDestination[:])
			}) && sha256.Sum256(m.Mem) == imulWordRAM
		}
		// 357 END imul_word_pre

		// 356 BEGIN set_memory_pre
		if bannerRed && !setMemorySeen && m.CPU.EIP == 0x1ce387 {
			setMemorySeen, setMemoryBudget = true, 3
		}
		observeSetMemory := setMemoryBudget > 0
		setMemoryR, setMemorySeg, setMemoryEIP, setMemoryFlags := m.CPU.R, m.CPU.Seg, m.CPU.EIP, m.CPU.EFlags
		var setMemoryCode [16]byte
		var setMemorySource [3]byte
		setMemoryOffset := setMemoryR[cpu386.EBP] - 13
		setMemorySourceReadable := false
		var setMemoryRAM [32]byte
		setMemoryReadonly := false
		var setMemoryRAMCopy []byte
		if observeSetMemory {
			setMemoryRAM = sha256.Sum256(m.Mem)
			setMemoryRAMCopy = append([]byte(nil), m.Mem...)
			setMemoryReadonly = activationPeek(func() {
				copy(setMemoryCode[:], m.Mem[setMemoryEIP:setMemoryEIP+16])
				setMemorySourceReadable = peekSourceWindow(setMemorySeg[cpu386.SegSS], setMemoryOffset, setMemorySource[:])
			}) && sha256.Sum256(m.Mem) == setMemoryRAM
		}
		// 356 END set_memory_pre

		// 355 BEGIN add_source_pre
		if bannerRed && !addSourceSeen && m.CPU.EIP == 0x1cdd0f {
			addSourceSeen, addSourceBudget = true, 5
		}
		observeAddSource := addSourceBudget > 0
		addSourceR, addSourceSeg, addSourceEIP, addSourceFlags := m.CPU.R, m.CPU.Seg, m.CPU.EIP, m.CPU.EFlags
		var addSourceCode [16]byte
		var addSourceSource [34]byte
		var addSourceDestination [3]byte
		addSourceOffset := addSourceR[cpu386.EBP] - 33
		addSourceSourceReadable, addSourceDestinationReadable := false, false
		var addSourceRAM [32]byte
		addSourceReadonly := false
		var addSourceRAMCopy []byte
		if observeAddSource {
			addSourceRAM = sha256.Sum256(m.Mem)
			addSourceRAMCopy = append([]byte(nil), m.Mem...)
			addSourceReadonly = activationPeek(func() {
				copy(addSourceCode[:], m.Mem[addSourceEIP:addSourceEIP+16])
				addSourceSourceReadable = peekSourceWindow(addSourceSeg[cpu386.SegSS], addSourceOffset, addSourceSource[:])
				addSourceDestinationReadable = peekSourceWindow(addSourceSeg[cpu386.SegDS], addSourceR[cpu386.EBX]+6, addSourceDestination[:])
			}) && sha256.Sum256(m.Mem) == addSourceRAM
		}
		// 355 END add_source_pre

		// 343 BEGIN setle_pre
		if bannerRed && !setleSeen && m.CPU.EIP == 0x17d536 {
			setleSeen, setleBudget = true, 2
		}
		observeSetle := setleBudget > 0
		setleR, setleSeg, setleEIP, setleFlags := m.CPU.R, m.CPU.Seg, m.CPU.EIP, m.CPU.EFlags
		var setleCode [16]byte
		var setleWindow, setleStack [4]byte
		setleOffset := setleR[cpu386.EBP] - 5
		setleReadable, setleStackReadable := false, false
		var setleRAM [32]byte
		setleReadonly := false
		if observeSetle {
			setleRAM = sha256.Sum256(m.Mem)
			setleReadonly = activationPeek(func() {
				copy(setleCode[:], m.Mem[setleEIP:setleEIP+16])
				setleReadable = peekSourceWindow(setleSeg[cpu386.SegSS], setleOffset, setleWindow[:])
				setleStackReadable = peekSourceWindow(setleSeg[cpu386.SegSS], setleR[cpu386.ESP], setleStack[:])
			}) && sha256.Sum256(m.Mem) == setleRAM
		}
		if publishBus != nil {
			publishBus.setleWatch = observeSetle
		}
		// 343 END setle_pre

		// 344 BEGIN cc_pre
		if bannerRed && !ccSeen && m.CPU.EIP == 0x17d5a0 {
			ccSeen, ccBudget = true, 2
		}
		observeCC := ccBudget > 0
		ccR, ccSeg, ccEIP, ccFlags := m.CPU.R, m.CPU.Seg, m.CPU.EIP, m.CPU.EFlags
		var ccCode [16]byte
		var ccRAM [32]byte
		var ccFPU [8]uint64
		ccControl, ccStatus, ccDepth := m.CPU.FPUControl, m.CPU.FPUStatus, m.CPU.FPUDepth
		ccReadonly := false
		ccVBEUnchanged := func() bool { return false }
		if observeCC {
			ccRAM = sha256.Sum256(m.Mem)
			ccVBE := m.VBEState()
			ccVBEUnchanged = func() bool { return ccVBE == m.VBEState() }
			ccReadonly = activationPeek(func() {
				copy(ccCode[:], m.Mem[ccEIP:ccEIP+16])
				for j := range ccFPU {
					ccFPU[j] = math.Float64bits(m.CPU.FPUStack[j])
				}
			}) && sha256.Sum256(m.Mem) == ccRAM
		}
		// 344 END cc_pre

		// 345 BEGIN universe_pre
		if bannerRed && genBudget == 0 && m.CPU.EIP == 0x17fcc3 {
			for j, anchor := range genAnchors {
				if !genSeen[j] && i >= anchor {
					genSeen[j], genWaiting[j], genStarts[j], genGroup, genBudget = true, true, i, j, 192
					genBP[j], genSelector[j] = m.CPU.R[cpu386.EBP], m.CPU.Seg[cpu386.SegSS]
					var frame [96]byte
					var code [224]byte
					readable := false
					ram := sha256.Sum256(m.Mem)
					readonly := activationPeek(func() {
						readable = peekSourceWindow(genSelector[j], genBP[j]-48, frame[:])
						copy(code[:], m.Mem[0x17fc60:0x17fd40])
					}) && ram == sha256.Sum256(m.Mem)
					if readable {
						genRA[j] = uint32(frame[60]) | uint32(frame[61])<<8 | uint32(frame[62])<<16 | uint32(frame[63])<<24
					}
					fmt.Printf("universe_loop_arm group=%d anchor=%d outer_step=%d address_space=dosgolem_high_le eip=%X r=%X seg=%X flags=%X frame_selector=%X frame_offset=%X frame_readable=%t frame=%X candidate_return=%X code_start=17FC60 code=%X readonly=%t\n", j, anchor, i, m.CPU.EIP, m.CPU.R, m.CPU.Seg, m.CPU.EFlags, genSelector[j], genBP[j]-48, readable, frame, genRA[j], code, readonly)
					break
				}
			}
		}
		observeGen := genBudget > 0
		genR, genSeg, genEIP, genFlags := m.CPU.R, m.CPU.Seg, m.CPU.EIP, m.CPU.EFlags
		genFrameReadable, genStackReadable, genReadonly := false, false, false
		var genOpcode byte
		if observeGen || genWaiting[0] || genWaiting[1] || genWaiting[2] {
			if uint64(genEIP) < uint64(len(m.Mem)) {
				genOpcode = m.Mem[genEIP]
			}
		}
		genRAMChecked := observeGen && (genCounts[genGroup] == 0 || genBudget == 1)
		if observeGen {
			var ram [32]byte
			if genRAMChecked {
				ram = sha256.Sum256(m.Mem)
			}
			genReadonly = activationPeek(func() {
				copy(genCode[:], m.Mem[genEIP:genEIP+16])
				genFrameReadable = peekSourceWindow(genSelector[genGroup], genBP[genGroup]-48, genFrame[:])
				genStackReadable = peekSourceWindow(genSeg[cpu386.SegSS], genR[cpu386.ESP], genStack[:])
			})
			if genRAMChecked {
				genReadonly = genReadonly && ram == sha256.Sum256(m.Mem)
			}
		}
		// 345 END universe_pre

		stepErr := m.CPU.Step()
		if publishBus != nil {
			publishBus.active = false
		}
		buttonReadStepActive = false

		// 345 BEGIN universe_post
		if observeGen {
			genBudget--
			genCounts[genGroup]++
			var frameAfter [96]byte
			var ram [32]byte
			if genRAMChecked {
				ram = sha256.Sum256(m.Mem)
			}
			readable := false
			readonly := activationPeek(func() {
				readable = peekSourceWindow(genSelector[genGroup], genBP[genGroup]-48, frameAfter[:])
			}) && genReadonly
			if genRAMChecked {
				readonly = readonly && ram == sha256.Sum256(m.Mem)
			}
			_, pending, active, started, completed := services.MouseCallbackState()
			irqActive, irqFailed, irqStarted, irqCompleted := services.IRQ0State()
			fmt.Printf("universe_loop_step group=%d sample=%d outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X frame_selector=%X frame_offset=%X frame_readable=%t after_frame_readable=%t before_frame=%X after_frame=%X stack_selector=%X stack_offset=%X stack_readable=%t stack=%X callback_pending=%d callback_active=%t callback_started=%d callback_completed=%d irq_active=%t irq_failed=%t irq_started=%d irq_completed=%d readonly=%t ram_checked=%t remaining=%d error=%v\n", genGroup, genCounts[genGroup], i, genEIP, m.CPU.EIP, genR, m.CPU.R, genSeg, m.CPU.Seg, genFlags, m.CPU.EFlags, genCode, genSelector[genGroup], genBP[genGroup]-48, genFrameReadable, readable, genFrame, frameAfter, genSeg[cpu386.SegSS], genR[cpu386.ESP], genStackReadable, genStack, pending, active, started, completed, irqActive, irqFailed, irqStarted, irqCompleted, readonly, genRAMChecked, genBudget, stepErr)
			if stepErr != nil {
				genBudget = 0
			}
		}
		if stepErr == nil && (genOpcode == 0xc3 || genOpcode == 0xc2) {
			for j := range genWaiting {
				if !genWaiting[j] || m.CPU.EIP != genRA[j] {
					continue
				}
				genWaiting[j] = false
				var stack [4]byte
				var code [3]byte
				readable := false
				ram := sha256.Sum256(m.Mem)
				readonly := activationPeek(func() {
					readable = peekSourceWindow(genSeg[cpu386.SegSS], genR[cpu386.ESP], stack[:])
					copy(code[:], m.Mem[genEIP:genEIP+3])
				}) && ram == sha256.Sum256(m.Mem)
				rawTarget := uint32(stack[0]) | uint32(stack[1])<<8 | uint32(stack[2])<<16 | uint32(stack[3])<<24
				pop := uint32(4)
				if genOpcode == 0xc2 {
					pop += uint32(code[1]) | uint32(code[2])<<8
				}
				wantR := genR
				wantR[cpu386.ESP] += pop
				valid := readable && rawTarget == genRA[j] && genR[cpu386.ESP] == genBP[j]+12 && m.CPU.R == wantR && m.CPU.Seg == genSeg && m.CPU.EFlags == genFlags
				fmt.Printf("universe_loop_return group=%d outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X frame_ebp=%X stack_selector=%X stack_offset=%X stack_readable=%t stack=%X raw_target=%X readonly=%t valid=%t error=%v\n", j, i, genEIP, m.CPU.EIP, genR, m.CPU.R, genSeg, m.CPU.Seg, genFlags, m.CPU.EFlags, code, genBP[j], genSeg[cpu386.SegSS], genR[cpu386.ESP], readable, stack, rawTarget, readonly, valid, stepErr)
			}
		}
		// 345 END universe_post

		// 344 BEGIN cc_post
		if observeCC {
			ccBudget--
			var afterFPU [8]uint64
			ram := sha256.Sum256(m.Mem)
			readonly := activationPeek(func() {
				for j := range afterFPU {
					afterFPU[j] = math.Float64bits(m.CPU.FPUStack[j])
				}
			}) && sha256.Sum256(m.Mem) == ram && ccReadonly
			_, pending, active, started, completed := services.MouseCallbackState()
			irqActive, irqFailed, irqStarted, irqCompleted := services.IRQ0State()
			fmt.Printf("setcc_consumer outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X before_fpu_control=%X after_fpu_control=%X before_fpu_status=%X after_fpu_status=%X before_fpu_depth=%d after_fpu_depth=%d before_fpu_bits=%X after_fpu_bits=%X callback_pending=%d callback_active=%t callback_started=%d callback_completed=%d irq_active=%t irq_failed=%t irq_started=%d irq_completed=%d readonly=%t step_ram_unchanged=%t step_vbe_unchanged=%t remaining=%d error=%v\n", i, ccEIP, m.CPU.EIP, ccR, m.CPU.R, ccSeg, m.CPU.Seg, ccFlags, m.CPU.EFlags, ccCode, ccControl, m.CPU.FPUControl, ccStatus, m.CPU.FPUStatus, ccDepth, m.CPU.FPUDepth, ccFPU, afterFPU, pending, active, started, completed, irqActive, irqFailed, irqStarted, irqCompleted, readonly, ccRAM == ram, ccVBEUnchanged(), ccBudget, stepErr)
			if stepErr != nil {
				ccBudget = 0
			}
		}
		// 344 END cc_post

		// 343 BEGIN setle_post
		if observeSetle {
			setleBudget--
			var windowAfter, stackAfter [4]byte
			ram := sha256.Sum256(m.Mem)
			afterReadable, afterStackReadable := false, false
			readonly := activationPeek(func() {
				afterReadable = peekSourceWindow(setleSeg[cpu386.SegSS], setleOffset, windowAfter[:])
				afterStackReadable = peekSourceWindow(setleSeg[cpu386.SegSS], setleR[cpu386.ESP], stackAfter[:])
			}) && sha256.Sum256(m.Mem) == ram && setleReadonly
			_, pending, active, started, completed := services.MouseCallbackState()
			irqActive, irqFailed, irqStarted, irqCompleted := services.IRQ0State()
			fmt.Printf("setle_consumer outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X selector=%X window_offset=%X operand_offset=%X window_readable=%t after_window_readable=%t before_window=%X after_window=%X stack_selector=%X stack_offset=%X stack_readable=%t after_stack_readable=%t before_stack=%X after_stack=%X callback_pending=%d callback_active=%t callback_started=%d callback_completed=%d irq_active=%t irq_failed=%t irq_started=%d irq_completed=%d readonly=%t step_ram_unchanged=%t remaining=%d error=%v\n", i, setleEIP, m.CPU.EIP, setleR, m.CPU.R, setleSeg, m.CPU.Seg, setleFlags, m.CPU.EFlags, setleCode, setleSeg[cpu386.SegSS], setleOffset, setleOffset+1, setleReadable, afterReadable, setleWindow, windowAfter, setleSeg[cpu386.SegSS], setleR[cpu386.ESP], setleStackReadable, afterStackReadable, setleStack, stackAfter, pending, active, started, completed, irqActive, irqFailed, irqStarted, irqCompleted, readonly, setleRAM == ram, setleBudget, stepErr)
			if stepErr != nil {
				setleBudget = 0
			}
		}
		// 343 END setle_post

		// 342 BEGIN test_post
		if observeTestMemory {
			testMemoryBudget--
			var sourceAfter, stackAfter [4]byte
			ram := sha256.Sum256(m.Mem)
			afterSourceReadable, afterStackReadable := false, false
			readonly := activationPeek(func() {
				afterSourceReadable = peekSourceWindow(testSeg[cpu386.SegDS], testOffset, sourceAfter[:])
				afterStackReadable = peekSourceWindow(testSeg[cpu386.SegSS], testR[cpu386.ESP], stackAfter[:])
			}) && sha256.Sum256(m.Mem) == ram && testReadonly
			_, pending, active, started, completed := services.MouseCallbackState()
			irqActive, irqFailed, irqStarted, irqCompleted := services.IRQ0State()
			fmt.Printf("test_memory_consumer outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X selector=%X source_offset=%X source_readable=%t after_source_readable=%t before_source=%X after_source=%X stack_selector=%X stack_offset=%X stack_readable=%t after_stack_readable=%t before_stack=%X after_stack=%X callback_pending=%d callback_active=%t callback_started=%d callback_completed=%d irq_active=%t irq_failed=%t irq_started=%d irq_completed=%d readonly=%t step_ram_unchanged=%t remaining=%d error=%v\n", i, testEIP, m.CPU.EIP, testR, m.CPU.R, testSeg, m.CPU.Seg, testFlags, m.CPU.EFlags, testCode, testSeg[cpu386.SegDS], testOffset, testSourceReadable, afterSourceReadable, testSource, sourceAfter, testSeg[cpu386.SegSS], testR[cpu386.ESP], testStackReadable, afterStackReadable, testStack, stackAfter, pending, active, started, completed, irqActive, irqFailed, irqStarted, irqCompleted, readonly, testRAM == ram, testMemoryBudget, stepErr)
			if stepErr != nil {
				testMemoryBudget = 0
			}
		}
		// 342 END test_post

		// 353 BEGIN and_post
		if observeAndWordMemory {
			andWordMemoryBudget--
			var sourceAfter [8]byte
			var stackAfter [4]byte
			ram := sha256.Sum256(m.Mem)
			afterSourceReadable, afterStackReadable := false, false
			readonly := activationPeek(func() {
				afterSourceReadable = peekSourceWindow(andWordSeg[cpu386.SegDS], andWordOffset, sourceAfter[:])
				afterStackReadable = peekSourceWindow(andWordSeg[cpu386.SegSS], andWordR[cpu386.ESP], stackAfter[:])
			}) && sha256.Sum256(m.Mem) == ram && andWordReadonly
			_, pending, active, started, completed := services.MouseCallbackState()
			irqActive, irqFailed, irqStarted, irqCompleted := services.IRQ0State()
			changes := make([]uint32, 0, 4)
			for address, value := range andWordRAMCopy {
				if value != m.Mem[address] {
					changes = append(changes, uint32(address))
				}
			}
			fmt.Printf("and_word_memory_consumer outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X selector=%X source_offset=%X source_readable=%t after_source_readable=%t before_source=%X after_source=%X stack_selector=%X stack_offset=%X stack_readable=%t after_stack_readable=%t before_stack=%X after_stack=%X callback_pending=%d callback_active=%t callback_started=%d callback_completed=%d irq_active=%t irq_failed=%t irq_started=%d irq_completed=%d readonly=%t ram_changes=%X step_ram_unchanged=%t remaining=%d error=%v\n", i, andWordEIP, m.CPU.EIP, andWordR, m.CPU.R, andWordSeg, m.CPU.Seg, andWordFlags, m.CPU.EFlags, andWordCode, andWordSeg[cpu386.SegDS], andWordOffset, andWordSourceReadable, afterSourceReadable, andWordSource, sourceAfter, andWordSeg[cpu386.SegSS], andWordR[cpu386.ESP], andWordStackReadable, afterStackReadable, andWordStack, stackAfter, pending, active, started, completed, irqActive, irqFailed, irqStarted, irqCompleted, readonly, changes, andWordRAM == ram, andWordMemoryBudget, stepErr)
			if stepErr != nil {
				andWordMemoryBudget = 0
			}
		}
		// 353 END and_post

		// 354 BEGIN xchg_post
		if observeXchgByteMemory {
			xchgByteMemoryBudget--
			var sourceAfter [3]byte
			var destinationAfter [3]byte
			ram := sha256.Sum256(m.Mem)
			afterSourceReadable, afterDestinationReadable := false, false
			readonly := activationPeek(func() {
				afterSourceReadable = peekSourceWindow(xchgByteSeg[cpu386.SegDS], xchgByteOffset, sourceAfter[:])
				afterDestinationReadable = peekSourceWindow(xchgByteSeg[cpu386.SegES], xchgByteR[cpu386.EDI]-1, destinationAfter[:])
			}) && sha256.Sum256(m.Mem) == ram && xchgByteReadonly
			_, pending, active, started, completed := services.MouseCallbackState()
			irqActive, irqFailed, irqStarted, irqCompleted := services.IRQ0State()
			changes := make([]uint32, 0, 4)
			for address, value := range xchgByteRAMCopy {
				if value != m.Mem[address] {
					changes = append(changes, uint32(address))
				}
			}
			fmt.Printf("xchg_byte_memory_consumer outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X selector=%X source_offset=%X source_readable=%t after_source_readable=%t before_source=%X after_source=%X destination_selector=%X destination_offset=%X destination_readable=%t after_destination_readable=%t before_destination=%X after_destination=%X callback_pending=%d callback_active=%t callback_started=%d callback_completed=%d irq_active=%t irq_failed=%t irq_started=%d irq_completed=%d readonly=%t ram_changes=%X step_ram_unchanged=%t remaining=%d error=%v\n", i, xchgByteEIP, m.CPU.EIP, xchgByteR, m.CPU.R, xchgByteSeg, m.CPU.Seg, xchgByteFlags, m.CPU.EFlags, xchgByteCode, xchgByteSeg[cpu386.SegDS], xchgByteOffset, xchgByteSourceReadable, afterSourceReadable, xchgByteSource, sourceAfter, xchgByteSeg[cpu386.SegES], xchgByteR[cpu386.EDI]-1, xchgByteDestinationReadable, afterDestinationReadable, xchgByteDestination, destinationAfter, pending, active, started, completed, irqActive, irqFailed, irqStarted, irqCompleted, readonly, changes, xchgByteRAM == ram, xchgByteMemoryBudget, stepErr)
			if stepErr != nil {
				xchgByteMemoryBudget = 0
			}
		}
		// 354 END xchg_post

		// 357 BEGIN imul_word_post
		if observeImulWord {
			imulWordBudget--
			var sourceAfter [4]byte
			var destinationAfter [20]byte
			ram := sha256.Sum256(m.Mem)
			afterSourceReadable, afterDestinationReadable := false, false
			readonly := activationPeek(func() {
				afterSourceReadable = peekSourceWindow(imulWordSeg[cpu386.SegDS], imulWordOffset, sourceAfter[:])
				afterDestinationReadable = peekSourceWindow(imulWordSeg[cpu386.SegSS], imulWordR[cpu386.EBP]-44, destinationAfter[:])
			}) && sha256.Sum256(m.Mem) == ram && imulWordReadonly
			_, pending, active, started, completed := services.MouseCallbackState()
			irqActive, irqFailed, irqStarted, irqCompleted := services.IRQ0State()
			changes := make([]uint32, 0, 4)
			for address, value := range imulWordRAMCopy {
				if value != m.Mem[address] {
					changes = append(changes, uint32(address))
				}
			}
			fmt.Printf("imul_word_immediate_consumer outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X selector=%X source_offset=%X source_readable=%t after_source_readable=%t before_source=%X after_source=%X destination_selector=%X destination_offset=%X destination_readable=%t after_destination_readable=%t before_destination=%X after_destination=%X callback_pending=%d callback_active=%t callback_started=%d callback_completed=%d irq_active=%t irq_failed=%t irq_started=%d irq_completed=%d readonly=%t ram_changes=%X step_ram_unchanged=%t remaining=%d error=%v\n", i, imulWordEIP, m.CPU.EIP, imulWordR, m.CPU.R, imulWordSeg, m.CPU.Seg, imulWordFlags, m.CPU.EFlags, imulWordCode, imulWordSeg[cpu386.SegDS], imulWordOffset, imulWordSourceReadable, afterSourceReadable, imulWordSource, sourceAfter, imulWordSeg[cpu386.SegSS], imulWordR[cpu386.EBP]-44, imulWordDestinationReadable, afterDestinationReadable, imulWordDestination, destinationAfter, pending, active, started, completed, irqActive, irqFailed, irqStarted, irqCompleted, readonly, changes, imulWordRAM == ram, imulWordBudget, stepErr)
			if stepErr != nil {
				imulWordBudget = 0
			}
		}
		// 357 END imul_word_post

		// 356 BEGIN set_memory_post
		if observeSetMemory {
			setMemoryBudget--
			var sourceAfter [3]byte
			ram := sha256.Sum256(m.Mem)
			afterSourceReadable := false
			readonly := activationPeek(func() {
				afterSourceReadable = peekSourceWindow(setMemorySeg[cpu386.SegSS], setMemoryOffset, sourceAfter[:])
			}) && sha256.Sum256(m.Mem) == ram && setMemoryReadonly
			_, pending, active, started, completed := services.MouseCallbackState()
			irqActive, irqFailed, irqStarted, irqCompleted := services.IRQ0State()
			changes := make([]uint32, 0, 4)
			for address, value := range setMemoryRAMCopy {
				if value != m.Mem[address] {
					changes = append(changes, uint32(address))
				}
			}
			fmt.Printf("setcc_byte_memory_consumer outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X selector=%X source_offset=%X source_readable=%t after_source_readable=%t before_source=%X after_source=%X callback_pending=%d callback_active=%t callback_started=%d callback_completed=%d irq_active=%t irq_failed=%t irq_started=%d irq_completed=%d readonly=%t ram_changes=%X step_ram_unchanged=%t remaining=%d error=%v\n", i, setMemoryEIP, m.CPU.EIP, setMemoryR, m.CPU.R, setMemorySeg, m.CPU.Seg, setMemoryFlags, m.CPU.EFlags, setMemoryCode, setMemorySeg[cpu386.SegSS], setMemoryOffset, setMemorySourceReadable, afterSourceReadable, setMemorySource, sourceAfter, pending, active, started, completed, irqActive, irqFailed, irqStarted, irqCompleted, readonly, changes, setMemoryRAM == ram, setMemoryBudget, stepErr)
			if stepErr != nil {
				setMemoryBudget = 0
			}
		}
		// 356 END set_memory_post

		// 355 BEGIN add_source_post
		if observeAddSource {
			addSourceBudget--
			var sourceAfter [34]byte
			var destinationAfter [3]byte
			ram := sha256.Sum256(m.Mem)
			afterSourceReadable, afterDestinationReadable := false, false
			readonly := activationPeek(func() {
				afterSourceReadable = peekSourceWindow(addSourceSeg[cpu386.SegSS], addSourceOffset, sourceAfter[:])
				afterDestinationReadable = peekSourceWindow(addSourceSeg[cpu386.SegDS], addSourceR[cpu386.EBX]+6, destinationAfter[:])
			}) && sha256.Sum256(m.Mem) == ram && addSourceReadonly
			_, pending, active, started, completed := services.MouseCallbackState()
			irqActive, irqFailed, irqStarted, irqCompleted := services.IRQ0State()
			changes := make([]uint32, 0, 4)
			for address, value := range addSourceRAMCopy {
				if value != m.Mem[address] {
					changes = append(changes, uint32(address))
				}
			}
			fmt.Printf("add_byte_source_consumer outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X selector=%X source_offset=%X source_readable=%t after_source_readable=%t before_source=%X after_source=%X destination_selector=%X destination_offset=%X destination_readable=%t after_destination_readable=%t before_destination=%X after_destination=%X callback_pending=%d callback_active=%t callback_started=%d callback_completed=%d irq_active=%t irq_failed=%t irq_started=%d irq_completed=%d readonly=%t ram_changes=%X step_ram_unchanged=%t remaining=%d error=%v\n", i, addSourceEIP, m.CPU.EIP, addSourceR, m.CPU.R, addSourceSeg, m.CPU.Seg, addSourceFlags, m.CPU.EFlags, addSourceCode, addSourceSeg[cpu386.SegSS], addSourceOffset, addSourceSourceReadable, afterSourceReadable, addSourceSource, sourceAfter, addSourceSeg[cpu386.SegDS], addSourceR[cpu386.EBX]+6, addSourceDestinationReadable, afterDestinationReadable, addSourceDestination, destinationAfter, pending, active, started, completed, irqActive, irqFailed, irqStarted, irqCompleted, readonly, changes, addSourceRAM == ram, addSourceBudget, stepErr)
			if stepErr != nil {
				addSourceBudget = 0
			}
		}
		// 355 END add_source_post

		// 340 BEGIN return_post
		if observeBannerReturn && m.CPU.EIP == 0x20db5b {
			expected := bannerReturnR
			expected[cpu386.ESP] += 4
			valid := bannerReturnReadonly && bannerReturnReadable && bannerReturnStack == [4]byte{0x5b, 0xdb, 0x20, 0} && bannerReturnR[cpu386.EAX]&0xffff == 1 && bannerReturnSeg[cpu386.SegCS] == 8 && bannerReturnSeg[cpu386.SegDS] == 0x188 && bannerReturnSeg[cpu386.SegSS] == 0x188 && m.CPU.R == expected && m.CPU.Seg == bannerReturnSeg && m.CPU.EFlags == bannerReturnFlags && stepErr == nil
			fmt.Printf("banner_red_gui_button_return outer_step=%d address_space=dosgolem_high_le input_eip=214104 after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X stack_selector=%X stack_offset=%X raw_return=%X readable=%t readonly=%t valid=%t error=%v\n", i, m.CPU.EIP, bannerReturnR, m.CPU.R, bannerReturnSeg, m.CPU.Seg, bannerReturnFlags, m.CPU.EFlags, bannerReturnSeg[cpu386.SegSS], bannerReturnR[cpu386.ESP], bannerReturnStack, bannerReturnReadable, bannerReturnReadonly, valid, stepErr)
			if !valid {
				panic("旗幟caller未收到原正常pressed返回")
			}
			bannerGUIButtonSeen = true
		}
		// 340 END return_post

		// 341 BEGIN gate_post
		if observeBannerGate && m.CPU.EIP == 0x20e165 {
			expected := bannerGateR
			expected[cpu386.ESP] += 4
			valid := bannerGateReadonly && bannerGateReadable && bannerGateStack == [4]byte{0x65, 0xe1, 0x20, 0} && bannerGateR[cpu386.EAX]&0xffff == 1 && bannerGateSeg[cpu386.SegCS] == 8 && bannerGateSeg[cpu386.SegDS] == 0x188 && bannerGateSeg[cpu386.SegSS] == 0x188 && m.CPU.R == expected && m.CPU.Seg == bannerGateSeg && m.CPU.EFlags == bannerGateFlags && stepErr == nil
			fmt.Printf("banner_red_selection_gate_return outer_step=%d address_space=dosgolem_high_le input_eip=214104 after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X stack_selector=%X stack_offset=%X raw_return=%X readable=%t readonly=%t valid=%t error=%v\n", i, m.CPU.EIP, bannerGateR, m.CPU.R, bannerGateSeg, m.CPU.Seg, bannerGateFlags, m.CPU.EFlags, bannerGateSeg[cpu386.SegSS], bannerGateR[cpu386.ESP], bannerGateStack, bannerGateReadable, bannerGateReadonly, valid, stepErr)
			if !valid {
				panic("旗幟後段caller未收到原正常pressed返回")
			}
			bannerSelectionGateSeen = true
		}
		// 341 END gate_post

		if observeScasWord {
			scasWordBudget--
			var afterStack [4]byte
			afterSource := make([]byte, len(scasWordBefore))
			r, seg, eip, flags, bus := m.CPU.R, m.CPU.Seg, m.CPU.EIP, m.CPU.EFlags, m.CPU.Bus
			ramBefore := sha256.Sum256(m.Mem)
			afterSourceReadable := scasWordSource != nil && peekSourceWindow(scasWordSelector, scasWordOffset, afterSource)
			afterStackReadable := peekSourceWindow(scasWordStackSelector, scasWordStackOffset, afterStack[:])
			readonly := scasWordReadonly && m.CPU.R == r && m.CPU.Seg == seg && m.CPU.EIP == eip && m.CPU.EFlags == flags && m.CPU.Bus == bus && sha256.Sum256(m.Mem) == ramBefore
			_, pending, active, started, completed := services.MouseCallbackState()
			irqActive, irqFailed, irqStarted, irqCompleted := services.IRQ0State()
			fmt.Printf("repne_scasw_consumer begin=%t outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X selector=%X source_offset=%X source_readable=%t after_source_readable=%t before_source=%X after_source=%X stack_selector=%X stack_offset=%X stack_readable=%t after_stack_readable=%t before_stack=%X after_stack=%X callback_pending=%d callback_active=%t callback_started=%d callback_completed=%d irq_active=%t irq_failed=%t irq_started=%d irq_completed=%d readonly=%t remaining=%d error=%v\n", scasWordBegin, i, clickBeforeEIP, m.CPU.EIP, clickBeforeR, m.CPU.R, clickBeforeSeg, m.CPU.Seg, clickBeforeFlags, m.CPU.EFlags, clickBytes, scasWordSelector, scasWordOffset, scasWordSourceReadable, afterSourceReadable, scasWordBefore, afterSource, scasWordStackSelector, scasWordStackOffset, scasWordStackReadable, afterStackReadable, scasWordBeforeStack, afterStack, pending, active, started, completed, irqActive, irqFailed, irqStarted, irqCompleted, readonly, scasWordBudget, stepErr)
			if stepErr != nil || scasWordBudget == 1 && m.CPU.EIP != 0x1f3643 {
				scasWordBudget = 0
			}
		}

		// 339 BEGIN store_post

		if observeBannerStore {
			bannerStoreSeen = true
			var afterWord [2]byte
			afterReadable := peekSourceWindow(bannerBeforeSeg[cpu386.SegDS], 0x26c4a6, afterWord[:])
			mask, pending, active, started, completed := services.MouseCallbackState()
			irqActive, irqFailed, irqStarted, irqCompleted := services.IRQ0State()
			fmt.Printf("banner_red_selected_store outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X selector=%X offset=26C4A6 before_readable=%t after_readable=%t before_word=%X after_word=%X callback_mask=%X callback_pending=%d callback_active=%t callback_started=%d callback_completed=%d irq_active=%t irq_failed=%t irq_started=%d irq_completed=%d error=%v\n", i, bannerBeforeEIP, m.CPU.EIP, bannerBeforeR, m.CPU.R, bannerBeforeSeg, m.CPU.Seg, bannerBeforeFlags, m.CPU.EFlags, bannerBytes, bannerBeforeSeg[cpu386.SegDS], bannerWordReadable, afterReadable, bannerBeforeWord, afterWord, mask, pending, active, started, completed, irqActive, irqFailed, irqStarted, irqCompleted, stepErr)
		}
		// 339 END store_post

		if observeRulerStore {
			rulerStoreSeen = true
			var afterWord [2]byte
			var afterCandidate [32]byte
			afterCandidateReadable := peekSourceWindow(rulerBeforeSeg[cpu386.SegDS], 0x28439d, afterCandidate[:])
			afterReadable := peekSourceWindow(rulerBeforeSeg[cpu386.SegDS], 0x26c4a6, afterWord[:])
			mask, pending, active, started, completed := services.MouseCallbackState()
			irqActive, irqFailed, irqStarted, irqCompleted := services.IRQ0State()
			fmt.Printf("ruler_name_accept_selected_store outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X selector=%X offset=26C4A6 before_readable=%t after_readable=%t before_word=%X after_word=%X callback_mask=%X callback_pending=%d callback_active=%t callback_started=%d callback_completed=%d irq_active=%t irq_failed=%t irq_started=%d irq_completed=%d candidate_offset=28439D before_candidate_readable=%t after_candidate_readable=%t before_candidate=%X after_candidate=%X error=%v\n", i, rulerBeforeEIP, m.CPU.EIP, rulerBeforeR, m.CPU.R, rulerBeforeSeg, m.CPU.Seg, rulerBeforeFlags, m.CPU.EFlags, rulerBytes, rulerBeforeSeg[cpu386.SegDS], rulerWordReadable, afterReadable, rulerBeforeWord, afterWord, mask, pending, active, started, completed, irqActive, irqFailed, irqStarted, irqCompleted, rulerBeforeCandidateReadable, afterCandidateReadable, rulerBeforeCandidate, afterCandidate, stepErr)
		}

		if observeRaceStore {
			raceStoreSeen = true
			var afterWord [2]byte
			afterReadable := peekSourceWindow(raceBeforeSeg[cpu386.SegDS], 0x26c4a6, afterWord[:])
			mask, pending, active, started, completed := services.MouseCallbackState()
			irqActive, irqFailed, irqStarted, irqCompleted := services.IRQ0State()
			fmt.Printf("race_humans_selected_store outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X selector=%X offset=26C4A6 before_readable=%t after_readable=%t before_word=%X after_word=%X callback_mask=%X callback_pending=%d callback_active=%t callback_started=%d callback_completed=%d irq_active=%t irq_failed=%t irq_started=%d irq_completed=%d error=%v\n", i, raceBeforeEIP, m.CPU.EIP, raceBeforeR, m.CPU.R, raceBeforeSeg, m.CPU.Seg, raceBeforeFlags, m.CPU.EFlags, raceBytes, raceBeforeSeg[cpu386.SegDS], raceWordReadable, afterReadable, raceBeforeWord, afterWord, mask, pending, active, started, completed, irqActive, irqFailed, irqStarted, irqCompleted, stepErr)
		}
		if observeSetupStore {
			setupStoreSeen = true
			var afterWord [2]byte
			afterReadable := peekSourceWindow(setupBeforeSeg[cpu386.SegDS], 0x26c4a6, afterWord[:])
			mask, pending, active, started, completed := services.MouseCallbackState()
			irqActive, irqFailed, irqStarted, irqCompleted := services.IRQ0State()
			fmt.Printf("setup_accept_selected_store outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X selector=%X offset=26C4A6 before_readable=%t after_readable=%t before_word=%X after_word=%X callback_mask=%X callback_pending=%d callback_active=%t callback_started=%d callback_completed=%d irq_active=%t irq_failed=%t irq_started=%d irq_completed=%d error=%v\n", i, setupBeforeEIP, m.CPU.EIP, setupBeforeR, m.CPU.R, setupBeforeSeg, m.CPU.Seg, setupBeforeFlags, m.CPU.EFlags, setupBytes, setupBeforeSeg[cpu386.SegDS], setupWordReadable, afterReadable, setupBeforeWord, afterWord, mask, pending, active, started, completed, irqActive, irqFailed, irqStarted, irqCompleted, stepErr)
		}
		if observeReadyStore {
			readyStoreSeen = true
			var afterWord [2]byte
			afterReadable := peekSourceWindow(clickBeforeSeg[cpu386.SegDS], 0x26c4a6, afterWord[:])
			mask, pending, active, started, completed := services.MouseCallbackState()
			irqActive, irqFailed, irqStarted, irqCompleted := services.IRQ0State()
			fmt.Printf("ready_menu_selected_store outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X selector=%X offset=26C4A6 before_readable=%t after_readable=%t before_word=%X after_word=%X callback_mask=%X callback_pending=%d callback_active=%t callback_started=%d callback_completed=%d irq_active=%t irq_failed=%t irq_started=%d irq_completed=%d error=%v\n", i, clickBeforeEIP, m.CPU.EIP, clickBeforeR, m.CPU.R, clickBeforeSeg, m.CPU.Seg, clickBeforeFlags, m.CPU.EFlags, clickBytes, clickBeforeSeg[cpu386.SegDS], readyWordReadable, afterReadable, readyBeforeWord, afterWord, mask, pending, active, started, completed, irqActive, irqFailed, irqStarted, irqCompleted, stepErr)
		}
		if observeBranch {
			var afterEvent [16]byte
			var afterCounter, afterTopBytes [4]byte
			var afterGlobals [192]byte
			var afterHeader [16]byte
			var afterSource [8]byte
			var afterStack [320]byte
			var afterPointer [64]byte
			afterTopReadable := false
			afterReadonly := activationPeek(func() {
				afterTopReadable = peekSourceWindow(m.CPU.Seg[cpu386.SegSS], m.CPU.R[cpu386.ESP], afterTopBytes[:])
				if branchEventReadable {
					peekSourceWindow(branchSelector, 0x2a121a, afterEvent[:])
				}
				if branchCounterReadable {
					peekSourceWindow(branchSelector, 0x2a11ec, afterCounter[:])
				}
				if branchHeaderReadable {
					peekSourceWindow(branchSelector, 0x29be0e, afterHeader[:])
				}
				if branchSourceReadable {
					peekSourceWindow(branchSelector, branchSourceOffset, afterSource[:])
				}
				if branchGlobalsReadable {
					peekSourceWindow(branchSelector, 0x26c480, afterGlobals[:])
				}
				if branchStackReadable {
					peekSourceWindow(branchStackSelector, branchStackOffset, afterStack[:])
				}
				if branchPointerReadable {
					peekSourceWindow(branchSelector, branchPointerOffset, afterPointer[:])
				}
			})
			_, _, afterCallback, afterStarted, afterCompleted := services.MouseCallbackState()
			afterIRQActive, afterIRQFailed, afterIRQStarted, afterIRQCompleted := services.IRQ0State()
			interrupted := afterIRQStarted < branchIRQStarted || afterIRQCompleted < branchIRQCompleted || afterIRQStarted-branchIRQStarted != afterIRQCompleted-branchIRQCompleted || afterIRQActive || afterIRQFailed || afterCallback
			top := uint32(branchBeforeTop[0]) | uint32(branchBeforeTop[1])<<8 | uint32(branchBeforeTop[2])<<16 | uint32(branchBeforeTop[3])<<24
			afterTop := uint32(afterTopBytes[0]) | uint32(afterTopBytes[1])<<8 | uint32(afterTopBytes[2])<<16 | uint32(afterTopBytes[3])<<24
			actualReturn := !interrupted && stepErr == nil && clickBytes[0] == 0xc3 && branchTopReadable && m.CPU.EIP == top && m.CPU.R[cpu386.ESP] == clickBeforeR[cpu386.ESP]+4
			callTarget := uint32(0)
			if clickBytes[0] == 0xe8 {
				callTarget = clickBeforeEIP + 5 + (uint32(clickBytes[1]) | uint32(clickBytes[2])<<8 | uint32(clickBytes[3])<<16 | uint32(clickBytes[4])<<24)
			}
			actualCall := !interrupted && stepErr == nil && clickBytes[0] == 0xe8 && m.CPU.EIP == callTarget && m.CPU.R[cpu386.ESP]+4 == clickBeforeR[cpu386.ESP] && afterTopReadable && afterTop == clickBeforeEIP+5
			branchSamples++
			if actualCall {
				branchWaiting, branchCallStep, branchReturnEIP, branchReturnESP, branchReturnSelector = true, i, clickBeforeEIP+5, clickBeforeR[cpu386.ESP], clickBeforeSeg[cpu386.SegSS]
			}
			unsupportedCall := clickBytes[0] == 0x9a || clickBytes[0] == 0xff && (clickBytes[1]>>3&7 == 2 || clickBytes[1]>>3&7 == 3)
			stop := ""
			if stepErr != nil {
				stop = "error"
			} else if interrupted {
				stop = "interrupt_redirect"
			} else if actualReturn {
				stop = "caller_ret"
			} else if clickBytes[0] == 0xe8 && !actualCall || unsupportedCall {
				stop = "unverified_call"
			} else if clickBytes[0] == 0xc3 && !actualReturn || clickBytes[0] == 0xc2 || clickBytes[0] == 0xca || clickBytes[0] == 0xcb {
				stop = "unverified_ret"
			} else if branchSamples == branchMaxSamples {
				stop = "sample_budget"
			}
			label := "new_game_button_branch_consumer"
			if branchTail {
				label = "new_game_button_tail_consumer"
			}
			fmt.Printf("%s begin=%t resumed=%t skipped_steps=%d outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X selector=%X event_readable=%t before_event=%X after_event=%X counter_readable=%t before_counter=%X after_counter=%X globals_offset=26C480 globals_readable=%t before_globals=%X after_globals=%X header_offset=29BE0E header_readable=%t before_header=%X after_header=%X source_offset=%X source_readable=%t before_source=%X after_source=%X stack_selector=%X stack_offset=%X stack_readable=%t before_stack=%X after_stack=%X pointer_selector=%X pointer_offset=%X pointer_readable=%t before_pointer=%X after_pointer=%X return_selector=%X return_offset=%X return_readable=%t return_bytes=%X after_return_readable=%t after_return_bytes=%X actual_return=%t actual_call=%t call_target=%X before_started=%d before_completed=%d after_started=%d after_completed=%d before_irq_active=%t before_irq_failed=%t before_irq_started=%d before_irq_completed=%d after_irq_active=%t after_irq_failed=%t after_irq_started=%d after_irq_completed=%d readonly=%t samples=%d waiting=%t stop=%s error=%v\n", label, branchBegin, branchResumed, branchSkipped, i, clickBeforeEIP, m.CPU.EIP, clickBeforeR, m.CPU.R, clickBeforeSeg, m.CPU.Seg, clickBeforeFlags, m.CPU.EFlags, clickBytes, branchSelector, branchEventReadable, branchBeforeEvent, afterEvent, branchCounterReadable, branchBeforeCounter, afterCounter, branchGlobalsReadable, branchBeforeGlobals, afterGlobals, branchHeaderReadable, branchBeforeHeader, afterHeader, branchSourceOffset, branchSourceReadable, branchBeforeSource, afterSource, branchStackSelector, branchStackOffset, branchStackReadable, branchBeforeStack, afterStack, branchSelector, branchPointerOffset, branchPointerReadable, branchBeforePointer, afterPointer, clickBeforeSeg[cpu386.SegSS], clickBeforeR[cpu386.ESP], branchTopReadable, branchBeforeTop, afterTopReadable, afterTopBytes, actualReturn, actualCall, callTarget, clickStarted, clickCompleted, afterStarted, afterCompleted, branchIRQActive, branchIRQFailed, branchIRQStarted, branchIRQCompleted, afterIRQActive, afterIRQFailed, afterIRQStarted, afterIRQCompleted, branchBeforeReadonly && afterReadonly, branchSamples, branchWaiting, stop, stepErr)
			if branchTail && actualCall && clickBeforeEIP == 0x20ddf2 && callTarget == 0x209325 && !menuWaitStarted {
				menuWaitStarted, menuBeginPending, menuStart = true, true, i
				menuDS, menuSS = branchSelector, clickBeforeSeg[cpu386.SegSS]
				menuFrame, menuReturnESP = branchStackOffset, clickBeforeR[cpu386.ESP]
			}
			if stop != "" {
				branchActive, branchStop = false, stop
			}
		}

		if observeActivation {
			var afterEvent [16]byte
			var afterCounter, afterGlobal, afterReturnBytes [4]byte
			var afterStack [64]byte
			afterReturnReadable := false
			afterReadonly := activationPeek(func() {
				afterReturnReadable = peekSourceWindow(m.CPU.Seg[cpu386.SegSS], m.CPU.R[cpu386.ESP], afterReturnBytes[:])
				if activationEventReadable {
					peekSourceWindow(activationSelector, 0x2a121a, afterEvent[:])
				}
				if activationCounterReadable {
					peekSourceWindow(activationSelector, 0x2a11ec, afterCounter[:])
				}
				if activationGlobalReadable {
					peekSourceWindow(activationSelector, 0x26c518, afterGlobal[:])
				}
				if activationStackReadable {
					peekSourceWindow(activationStackSelector, activationStackOffset, afterStack[:])
				}
			})
			_, _, afterCallback, afterStarted, afterCompleted := services.MouseCallbackState()
			returnTarget := uint32(activationReturnBytes[0]) | uint32(activationReturnBytes[1])<<8 | uint32(activationReturnBytes[2])<<16 | uint32(activationReturnBytes[3])<<24
			wasReturned := activationReturned
			actualReturn := stepErr == nil && clickBytes[0] == 0xc3 && activationReturnReadable && m.CPU.R[cpu386.ESP] == clickBeforeR[cpu386.ESP]+4 && m.CPU.EIP == returnTarget && !clickActive && !afterCallback
			activationReturned = activationReturned || actualReturn
			isJcc := clickBytes[0] >= 0x70 && clickBytes[0] <= 0x7f || clickBytes[0] == 0x0f && clickBytes[1] >= 0x80 && clickBytes[1] <= 0x8f
			callTarget := uint32(0)
			if clickBytes[0] == 0xe8 {
				callTarget = clickBeforeEIP + 5 + (uint32(clickBytes[1]) | uint32(clickBytes[2])<<8 | uint32(clickBytes[3])<<16 | uint32(clickBytes[4])<<24)
			}
			afterTop := uint32(afterReturnBytes[0]) | uint32(afterReturnBytes[1])<<8 | uint32(afterReturnBytes[2])<<16 | uint32(afterReturnBytes[3])<<24
			actualCall := clickBytes[0] == 0xe8 && stepErr == nil && m.CPU.EIP == callTarget && m.CPU.R[cpu386.ESP]+4 == clickBeforeR[cpu386.ESP] && afterReturnReadable && afterTop == clickBeforeEIP+5 && !clickActive && !afterCallback
			activationBudget--
			stop := ""
			if stepErr != nil {
				stop = "error"
			} else if wasReturned && isJcc && !clickActive && !afterCallback {
				stop = "caller_jcc"
			} else if wasReturned && actualCall {
				stop = "caller_call"
			} else if activationBudget == 0 {
				stop = "budget"
			}
			fmt.Printf("new_game_activation_consumer group=%d begin=%t outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X selector=%X event_readable=%t before_event=%X after_event=%X counter_readable=%t before_counter=%X after_counter=%X global_readable=%t before_global=%X after_global=%X stack_selector=%X stack_offset=%X stack_readable=%t before_stack=%X after_stack=%X return_selector=%X return_offset=%X return_readable=%t return_bytes=%X return_target=%X actual_return=%t returned=%t after_return_readable=%t after_return_bytes=%X actual_call=%t before_callback=%t after_callback=%t before_started=%d before_completed=%d after_started=%d after_completed=%d readonly=%t remaining=%d stop=%s error=%v\n", activationGroup, activationBegin, i, clickBeforeEIP, m.CPU.EIP, clickBeforeR, m.CPU.R, clickBeforeSeg, m.CPU.Seg, clickBeforeFlags, m.CPU.EFlags, clickBytes, activationSelector, activationEventReadable, activationBeforeEvent, afterEvent, activationCounterReadable, activationBeforeCounter, afterCounter, activationGlobalReadable, activationBeforeGlobal, afterGlobal, activationStackSelector, activationStackOffset, activationStackReadable, activationBeforeStack, afterStack, clickBeforeSeg[cpu386.SegSS], clickBeforeR[cpu386.ESP], activationReturnReadable, activationReturnBytes, returnTarget, actualReturn, activationReturned, afterReturnReadable, afterReturnBytes, actualCall, clickActive, afterCallback, clickStarted, clickCompleted, afterStarted, afterCompleted, activationBeforeReadonly && afterReadonly, activationBudget, stop, stepErr)
			if stop != "" {
				activationBudget = 0
			}
		}

		if observeSource {
			var afterGlobal [4]byte
			var afterWindow, afterDestination [5]byte
			var afterStack [80]byte
			if sourceGlobalReadable {
				peekSourceWindow(sourceSelector, 0x29be74, afterGlobal[:])
			}
			if sourceWindowReadable {
				peekSourceWindow(sourceSelector, sourceOffset-2, afterWindow[:])
			}
			if sourceStackReadable {
				peekSourceWindow(sourceStackSelector, sourceStackOffset, afterStack[:])
			}
			if destinationReadable {
				peekSourceWindow(destinationSelector, destinationOffset-2, afterDestination[:])
			}
			sourceBudget--
			stop := "none"
			if stepErr != nil {
				stop = "guest_error"
			} else if clickBeforeEIP == 0x213336 {
				stop = "destination_write"
			} else if m.CPU.EIP == 0x213345 {
				stop = "source_return"
			} else if sourceBudget == 0 {
				stop = "budget"
			}
			fmt.Printf("post_click_source_consumer group=%d kind=%s begin=%t outer_step=%d address_space=dosgolem_high_le input_eip=%X after_eip=%X before_r=%X after_r=%X before_seg=%X after_seg=%X before_flags=%X after_flags=%X instruction_bytes=%X source_selector=%X source_base=%X source_index=%X source_offset=%X global_readable=%t before_global=%X after_global=%X source_readable=%t before_source=%X after_source=%X stack_selector=%X stack_offset=%X stack_readable=%t before_stack=%X after_stack=%X destination_selector=%X destination_offset=%X destination_readable=%t before_destination=%X after_destination=%X remaining=%d stop=%s error=%v\n", sourceGroup, sourceKind, sourceBegin, i, clickBeforeEIP, m.CPU.EIP, clickBeforeR, m.CPU.R, clickBeforeSeg, m.CPU.Seg, clickBeforeFlags, m.CPU.EFlags, clickBytes, sourceSelector, sourceBase, sourceIndex, sourceOffset, sourceGlobalReadable, sourceBeforeGlobal, afterGlobal, sourceWindowReadable, sourceBeforeWindow, afterWindow, sourceStackSelector, sourceStackOffset, sourceStackReadable, sourceBeforeStack, afterStack, destinationSelector, destinationOffset, destinationReadable, destinationBeforeWindow, afterDestination, sourceBudget, stop, stepErr)
			if stop != "none" {
				sourceActive = false
				sourceBudget = 0
			}
		}

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
	observeMenuTable(maxSteps)
	dumpPostClickProgress(maxSteps)
	dumpExtendedProgress(maxSteps)
	dumpSetupTable(maxSteps)
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
