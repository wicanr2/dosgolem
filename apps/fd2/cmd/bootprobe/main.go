package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/wicanr2/dosgolem/internal/cpu386"
	"github.com/wicanr2/dosgolem/internal/machine"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	exe := flag.String("exe", "/orig/FD2.EXE", "固定版本原版執行檔")
	root := flag.String("root", "/orig", "唯讀原版資料根目錄")
	frame := flag.String("frame", "", "原生320×200停點PNG輸出（不覆寫）")
	budget := flag.Int("steps", 500000, "有界指令數（1至2000000000）")
	timedKeys := flag.String("timed-keys", "", "指令時點輸入：80000000:enter，逗號分隔且嚴格遞增")
	keys := flag.String("keys", "", "等待邊界的BIOS按鍵序列，以逗號分隔：up,down,left,right,enter,esc")
	state := flag.String("state", "", "獨立可寫檔案覆蓋目錄；空值維持唯讀")
	traceTail := flag.Bool("trace-tail", false, "唯讀記錄最後2000個指令位置")
	dialogueEnters := flag.Int("dialogue-enters", 0, "等待邊界送 Enter 的有界次數")
	dialogueFrames := flag.String("dialogue-frames", "", "逐頁原生收據目錄，必須存在且不得覆寫")
	heapMiB := flag.Int("heap-mib", 32, "明示近堆預算MiB（1至64），非原版配置器位址契約")
	flag.Parse()
	if *heapMiB < 1 || *heapMiB > 64 {
		panic("近堆容量越界")
	}
	if *dialogueEnters < 0 || *dialogueEnters > 512 {
		panic("對話輸入次數越界")
	}
	if *dialogueFrames != "" {
		if info, e := os.Stat(*dialogueFrames); e != nil || !info.IsDir() {
			panic("對話收據目錄不存在")
		}
	}

	if *budget < 1 || *budget > 2000000000 {
		panic("指令預算越界")
	}
	b, err := os.ReadFile(*exe)
	if err != nil {
		panic(err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(b))
	if hash != "222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f" {
		panic("FD2 hash mismatch")
	}
	m, err := machine.LoadLE(b)
	if err != nil {
		panic(err)
	}
	if err = machine.InstallDOS4GWBIOSData(m); err != nil {
		panic(err)
	}
	var files interface {
		machine.ReadOnlyFileProvider
		Close() error
	}
	if *state == "" {
		files, err = machine.OpenDirectoryReadOnlyFiles(*root)
	} else {
		files, err = machine.OpenDirectoryOverlayFiles(*root, *state)
	}
	if err != nil {
		panic(err)
	}
	defer files.Close()
	services := machine.NewFD2StartupDOS(files)
	defer services.Close()
	fileCalls := []map[string]any{}
	m.CPU.IntHook = func(c *cpu386.CPU, n byte) bool {
		ax := uint16(c.R[cpu386.EAX])
		path := ""
		observe := n == 0x21 && (ax>>8 == 0x3d || ax>>8 == 0x3c)
		if observe {
			for i := uint32(0); i < 260; i++ {
				v, ok := c.ReadSegment8(c.Seg[cpu386.SegDS], c.R[cpu386.EDX]+i)
				if !ok || v == 0 {
					break
				}
				path += string([]byte{v})
			}
		}
		handled := services.Handle(c, n)
		if observe {
			fileCalls = append(fileCalls, map[string]any{"AX_in": fmt.Sprintf("%04X", ax), "path": path, "handled": handled, "AX_out": fmt.Sprintf("%04X", uint16(c.R[cpu386.EAX])), "carry": c.EFlags&cpu386.CF != 0})
			if len(fileCalls) > 128 {
				fileCalls = fileCalls[len(fileCalls)-128:]
			}
		}
		return handled
	}
	opl := machine.NewLEOPLPorts()
	services.DPMI.RealModeIO = opl
	m.CPU.PortIn = opl.In8
	m.CPU.PortOut = opl.Out8
	if !machine.InstallLEVideo(m, opl) {
		panic("LEVideo安裝失敗")
	}
	if _, err = machine.InstallFD2WatcomRuntimeWithHeapCapacity(m, uint32(*heapMiB)*1024*1024, services.DPMI); err != nil {
		panic(err)
	}
	if !machine.InstallLEBIOSClock(m, opl) {
		panic("BIOS clock安裝失敗")
	}
	if !machine.InstallLEBIOSKeyboard(m) {
		panic("BIOS鍵盤安裝失敗")
	}
	var input []uint16
	keyNames := map[string]uint16{"up": 0x48e0, "down": 0x50e0, "left": 0x4be0, "right": 0x4de0, "enter": 0x1c0d, "esc": 0x011b}
	if *keys != "" {
		for _, name := range strings.Split(*keys, ",") {
			key, ok := keyNames[strings.TrimSpace(name)]
			if !ok {
				panic("未知按鍵名稱")
			}
			input = append(input, key)
		}
	}
	if len(input) > 100 {
		panic("按鍵序列超過100")
	}
	type scheduledKey struct {
		Step int
		Key  uint16
		Name string
	}
	var scheduled []scheduledKey
	if *timedKeys != "" {
		for _, item := range strings.Split(*timedKeys, ",") {
			pair := strings.Split(strings.TrimSpace(item), ":")
			if len(pair) != 2 {
				panic("排程輸入格式錯誤")
			}
			n, e := strconv.Atoi(pair[0])
			key, ok := keyNames[pair[1]]
			if e != nil || !ok || n < 0 || n >= *budget || (len(scheduled) > 0 && n <= scheduled[len(scheduled)-1].Step) {
				panic("排程輸入時點／名稱無效")
			}
			scheduled = append(scheduled, scheduledKey{n, key, pair[1]})
		}
	}
	if len(scheduled)+len(input)+*dialogueEnters > 512 {
		panic("總輸入超過100")
	}
	timedIndex := 0
	delivered := []scheduledKey{}
	inputIndex := 0
	waiting := false
	steps := 0
	mainStep := -1
	var stop error
	var instructionEIP uint32
	tail := make([]uint32, 0, 2000)
	dialogueArmed, dialogueIndex := false, 0
	dialogueReceipts := []map[string]any{}

	for ; steps < *budget; steps++ {
		if m.CPU.EIP == 0x25bf4 && mainStep < 0 {
			mainStep = steps
		}
		if timedIndex < len(scheduled) && scheduled[timedIndex].Step == steps {
			if e := m.Keyboard.Enqueue(scheduled[timedIndex].Key); e != nil {
				panic(e)
			}
			delivered = append(delivered, scheduled[timedIndex])
			timedIndex++
		}
		instructionEIP = m.CPU.EIP
		if *dialogueEnters > 0 || *dialogueFrames != "" {
			if instructionEIP == 0x16c57 {
				dialogueArmed = true
			}
			if instructionEIP == 0x16cf3 && dialogueArmed {
				dialogueArmed = false
				pixels := m.Video.Indexed()
				if len(pixels) != 64000 {
					panic("對話等待沒有完整mode13畫面")
				}
				palette := m.Video.Palette()
				pal := make(color.Palette, 256)
				for i, v := range palette {
					pal[i] = color.RGBA{v[0], v[1], v[2], 255}
				}
				snapshot := image.NewPaletted(image.Rect(0, 0, 320, 200), pal)
				copy(snapshot.Pix, pixels)
				var encoded bytes.Buffer
				if err := png.Encode(&encoded, snapshot); err != nil {
					panic(err)
				}
				path := ""
				if *dialogueFrames != "" {
					path = filepath.Join(*dialogueFrames, fmt.Sprintf("dialogue-%03d.png", dialogueIndex))
					f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
					if e != nil {
						panic(e)
					}
					_, e = f.Write(encoded.Bytes())
					ce := f.Close()
					if e != nil {
						panic(e)
					}
					if ce != nil {
						panic(ce)
					}
				}
				anchor, e := m.Read32(0x53c67)
				if e != nil {
					panic(e)
				}
				dialogueReceipts = append(dialogueReceipts, map[string]any{"index": dialogueIndex, "step": steps, "eip": "0x16CF3", "path": path, "portrait_anchor": anchor, "sha256": fmt.Sprintf("%x", sha256.Sum256(encoded.Bytes())), "indexed_sha256": fmt.Sprintf("%x", sha256.Sum256(pixels))})
				if dialogueIndex == *dialogueEnters {
					waiting = true
					break
				}
				if e := m.Keyboard.Enqueue(0x1c0d); e != nil {
					panic(e)
				}
				dialogueIndex++
			}
		}

		if *traceTail && steps >= *budget-2000 {
			tail = append(tail, instructionEIP)
		}
		if stop = m.CPU.Step(); stop != nil {
			break
		}
		if m.Keyboard.Waiting {
			if inputIndex == len(input) {
				if timedIndex < len(scheduled) {
					continue
				}
				waiting = true
				break
			}
			if err := m.Keyboard.Enqueue(input[inputIndex]); err != nil {
				panic(err)
			}
			inputIndex++
		}
	}
	end := int(instructionEIP) + 16
	if end > len(m.Mem) {
		end = len(m.Mem)
	}
	raw := ""
	if int(instructionEIP) < end {
		raw = fmt.Sprintf("% X", m.Mem[instructionEIP:uint32(end)])
	}
	msg := "step budget exhausted"
	if waiting {
		msg = "waiting for BIOS keyboard input"
	}
	if stop != nil {
		msg = stop.Error()
	}
	r := map[string]any{"schema": 1, "runner": "dosgolem", "exe_sha256": hash, "steps_completed": steps, "step_budget": *budget, "main_entry_step": mainStep, "stop_eip": fmt.Sprintf("0x%X", instructionEIP), "cpu_eip_after_error": fmt.Sprintf("0x%X", m.CPU.EIP), "address_space": "dosgolem relocated LE linear", "next_bytes": raw, "error": msg, "registers": m.CPU.R, "esp": fmt.Sprintf("0x%X", m.CPU.R[cpu386.ESP]), "state_injections": []string{}, "runtime_adapter": "existing InstallFD2WatcomRuntime and FD2StartupDOS", "original_root_read_only": true, "game_screen_produced": false, "normal_player_path_verified": false}

	if instructionEIP == 0x36d98 {
		stack := make([]uint32, 4)
		valid := true
		for i := range stack {
			v, e := m.Read32(m.CPU.R[cpu386.ESP] + uint32(i*4))
			if e != nil {
				valid = false
				break
			}
			stack[i] = v
		}
		if valid {
			regs := make([]uint32, 7)
			for i := range regs {
				v, e := m.Read32(stack[2] + uint32(i*4))
				if e != nil {
					valid = false
					break
				}
				regs[i] = v
			}
			r["watcom_int386_input"] = map[string]any{"stack": stack, "regs": regs, "valid": valid, "abi": "existing cdecl REGS; read-only observation"}
		}
	}
	r["heap_capacity_bytes"] = uint32(*heapMiB) * 1024 * 1024
	r["instruction_tail"] = tail
	r["state_directory"] = *state
	r["dos_file_calls"] = fileCalls
	r["keyboard"] = m.Keyboard
	r["dialogue_enter_count"] = dialogueIndex
	r["dialogue_receipts"] = dialogueReceipts
	r["dialogue_input_method"] = "明示啟用的16C57呼叫首次16CF3等待邊界；只送BIOS Enter，不改遊戲狀態"
	r["input_schedule"] = "BIOS等待邊界及明示指令時點；未注入遊戲狀態"
	r["timed_keys_requested"] = scheduled
	r["timed_keys_delivered"] = delivered
	r["waiting_for_input"] = waiting
	pixels := m.Video.Indexed()
	nonzero := 0
	for _, v := range pixels {
		if v != 0 {
			nonzero++
		}
	}
	r["video_mode"] = m.Video.Mode
	r["video_mode_sets"] = m.Video.ModeSets
	r["video_nonzero_pixels"] = nonzero
	r["video_indexed_sha256"] = fmt.Sprintf("%x", sha256.Sum256(pixels))
	r["video_palette_writes"] = opl.Writes[0x3c9]
	if *frame != "" {
		if len(pixels) != 64000 {
			panic("尚無mode13畫面可擷取")
		}
		palette := m.Video.Palette()
		pal := make(color.Palette, 256)
		for i, v := range palette {
			pal[i] = color.RGBA{v[0], v[1], v[2], 255}
		}
		snapshot := image.NewPaletted(image.Rect(0, 0, 320, 200), pal)
		copy(snapshot.Pix, pixels)
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, snapshot); err != nil {
			panic(err)
		}
		f, err := os.OpenFile(*frame, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			panic(err)
		}
		_, err = f.Write(encoded.Bytes())
		closeErr := f.Close()
		if err != nil {
			panic(err)
		}
		if closeErr != nil {
			panic(closeErr)
		}
		r["frame"] = map[string]any{"path": *frame, "sha256": fmt.Sprintf("%x", sha256.Sum256(encoded.Bytes())), "classification": "dosgolem自身停點畫面，完整度與玩家路徑尚待驗證"}
	}

	if uint64(instructionEIP)+64 <= uint64(len(m.Mem)) {
		r["next_bytes_64"] = fmt.Sprintf("% X", m.Mem[instructionEIP:instructionEIP+64])
	}
	r["pit0"] = opl.PIT0
	r["pit_timing"] = "hardware-spec approximation: 1 us per protected/real instruction; 1193182Hz PIT; default BIOS IRQ0 only"
	r["bios_clock"] = opl.BIOSClock
	if v, e := m.Read32(0x46c); e == nil {
		r["bios_ticks"] = v
	}
	r["protected_port_routing"] = "strict platform dispatch; unknown output rejected"
	r["bios_profile"] = "existing Machine.initBDA, DOS4GW selector 0040 base 0400"
	r["dpmi_real_mode_last"] = services.DPMI.RealModeLast
	r["dpmi_real_mode_history"] = services.DPMI.RealModeHistory
	r["opl_ports"] = opl.Log
	r["opl_reads"] = opl.Reads
	r["opl_writes"] = opl.Writes
	r["vga_timing"] = "hardware-spec approximation: existing deterministic status-read ticks"
	r["device_state"] = opl.State()
	r["dma_completions"] = opl.DMACompletions
	r["irq7_deliveries"] = opl.IRQ7Deliveries
	r["pcm_observed_bytes"] = len(opl.PCM)
	ivt := map[string]string{}
	for _, n := range []uint32{8, 15, 0x66} {
		v, e := m.Read32(n * 4)
		if e == nil {
			ivt[fmt.Sprintf("%02X", n)] = fmt.Sprintf("%04X:%04X", v>>16, v&0xffff)
		}
	}
	r["memory_ivt"] = ivt
	r["pic_profile"] = "DOSBox-X observed startup IMR 21=F8 A1=2C; mask registers and IRQ7/non-specific EOI subset"
	r["sb_profile"] = "SB16 DSP 4.05; base 0220; IRQ 7; DMA 1; HDMA 5; unsupported commands rejected"
	r["dsp_timing"] = "hardware-spec approximation: immediate reset; 1 us per real-mode instruction; rational sample rate 1000000/(256-TC), reset default 22050 Hz; no audio output"
	r["opl_timing"] = "hardware-spec approximation: existing immediate timer detection model"
	r["dpmi_calls"] = services.DPMI.Calls
	r["dpmi_unimplemented"] = services.DPMI.Unimplemented
	r["dos_memory"] = services.DPMI.DOSMemory()
	seg, off := services.DPMI.RealModeVector(0x66)
	r["real_mode_vector_66"] = fmt.Sprintf("%04X:%04X", seg, off)
	if uint16(m.CPU.R[cpu386.EAX]) == 0x0300 {
		packet := make([]byte, 50)
		valid := true
		for i := range packet {
			v, ok := m.CPU.ReadSegment8(m.CPU.Seg[cpu386.SegES], m.CPU.R[cpu386.EDI]+uint32(i))
			if !ok {
				valid = false
				break
			}
			packet[i] = v
		}
		if valid {
			r["real_mode_packet_hex"] = fmt.Sprintf("% X", packet)
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		panic(err)
	}
	if stop != nil {
		os.Exit(2)
	}
}
