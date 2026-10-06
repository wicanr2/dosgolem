package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"github.com/wicanr2/dosgolem/internal/cpu386"
	"github.com/wicanr2/dosgolem/internal/machine"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestFD2KeyNamesIncludeSecretShopFunctionChords(t *testing.T) {
	keys := fd2KeyNames()
	want := map[string]uint16{
		"shift-f1":  0x5400,
		"shift-f5":  0x5800,
		"shift-f10": 0x5d00,
		"ctrl-f1":   0x5e00,
		"ctrl-f5":   0x6200,
		"ctrl-f6":   0x6300,
		"ctrl-f10":  0x6700,
		"alt-f1":    0x6800,
		"alt-f10":   0x7100,
	}
	for name, value := range want {
		if got := keys[name]; got != value {
			t.Fatalf("%s=%04X，應為 %04X", name, got, value)
		}
	}
	if len(keys) != 36 {
		t.Fatalf("按鍵表有 %d 筆，應為 6 個基本鍵加 30 個 F 鍵 chord", len(keys))
	}
}

func TestEIPTraceWindowBoundariesAndEntryBudget(t *testing.T) {
	w := eipTraceWindow{100, 200, 2}
	for _, c := range []struct {
		step, entries int
		want          bool
	}{
		{99, 0, false}, {100, 0, true}, {200, 1, true}, {201, 0, false}, {150, 2, false},
	} {
		if got := w.allows(c.step, c.entries); got != c.want {
			t.Fatalf("step=%d entries=%d got=%v want=%v", c.step, c.entries, got, c.want)
		}
	}
	if !(eipTraceWindow{0, 0, 200000}).allows(30000000000, 199999) {
		t.Fatal("預設窗口應保留既有全程追蹤")
	}
	for _, w := range []eipTraceWindow{{-1, 0, 1}, {0, -1, 1}, {2, 1, 1}, {0, 0, 0}, {0, 0, 200001}} {
		if w.valid() {
			t.Fatalf("接受非法窗口 %+v", w)
		}
	}
	for _, w := range []eipTraceWindow{{0, 0, 200000}, {100, 100, 1}, {100, 0, 2}} {
		if !w.valid() {
			t.Fatalf("拒絕合法窗口 %+v", w)
		}
	}
}

func TestFD2FrameUnitsCaptureRawRowsWithoutChangingGuest(t *testing.T) {
	m := &machine.LEMachine{Mem: make([]byte, 0x60000)}
	base := uint32(len(m.Mem) - 128*80)
	binary.LittleEndian.PutUint32(m.Mem[0x53a45:], base)
	binary.LittleEndian.PutUint32(m.Mem[0x53beb:], 128)
	for i := 0; i < 128*80; i++ {
		m.Mem[int(base)+i] = byte(i*13 + 7)
	}
	before := append([]byte(nil), m.Mem...)
	record := map[string]any{"step": uint64(4000799496), "eip": "0x11EED", "file": "frame-000043.png"}
	appendFD2FrameUnits(record, m, true)
	if !record["frame_units_valid"].(bool) {
		t.Fatal("拒收剛好落在記憶體尾端的128筆")
	}
	rows := record["units"].([]map[string]any)
	if len(rows) != 128 {
		t.Fatalf("單位筆數%d", len(rows))
	}
	for i, row := range rows {
		raw, err := hex.DecodeString(row["raw_hex"].(string))
		expected := before[int(base)+i*80 : int(base)+(i+1)*80]
		if err != nil || !bytes.Equal(raw, expected) || row["index"] != uint32(i) {
			t.Fatalf("原始列%d損壞", i)
		}
		if row["hp"] != binary.LittleEndian.Uint16(expected[0x40:]) || row["pose"] != expected[3] || row["camp"] != expected[6] {
			t.Fatalf("原始投影%d錯誤", i)
		}
	}
	if !bytes.Equal(m.Mem, before) || record["step"] != uint64(4000799496) || record["eip"] != "0x11EED" {
		t.Fatal("觀測改變原記憶體或同時點標記")
	}
	m.Mem[base] = 0
	raw, _ := hex.DecodeString(rows[0]["raw_hex"].(string))
	if raw[0] != before[base] {
		t.Fatal("已產生收據仍引用可變原記憶體")
	}
}

func TestFD2FrameUnitsRejectInvalidNativeSources(t *testing.T) {
	for _, c := range []struct {
		name        string
		size        int
		base, count uint32
	}{
		{"count", 0x60000, 0x10000, 129},
		{"partial-record", 0x60000, 0x60000 - 79, 1},
		{"base-overflow", 0x60000, 0xfffffff0, 128},
		{"missing-globals", 128, 0, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := &machine.LEMachine{Mem: make([]byte, c.size)}
			if c.size >= 0x53beb+4 {
				binary.LittleEndian.PutUint32(m.Mem[0x53a45:], c.base)
				binary.LittleEndian.PutUint32(m.Mem[0x53beb:], c.count)
			}
			before := append([]byte(nil), m.Mem...)
			record := map[string]any{}
			appendFD2FrameUnits(record, m, true)
			if record["frame_units_valid"].(bool) || len(record["units"].([]map[string]any)) != 0 {
				t.Fatal("無效來源偽裝為可用")
			}
			if !bytes.Equal(before, m.Mem) {
				t.Fatal("非法來源觀測改寫記憶體")
			}
		})
	}
}

func TestFD2FrameUnitsDisabledPreservesMetadataAndEmptyRows(t *testing.T) {
	record := map[string]any{"step": 123, "unit_base": 456, "unit_count": 7}
	before, _ := json.Marshal(record)
	appendFD2FrameUnits(record, nil, false)
	after, _ := json.Marshal(record)
	if !bytes.Equal(before, after) {
		t.Fatal("停用旗標仍改metadata")
	}
	m := &machine.LEMachine{Mem: make([]byte, 0x60000)}
	appendFD2FrameUnits(record, m, true)
	if !record["frame_units_valid"].(bool) || len(record["units"].([]map[string]any)) != 0 {
		t.Fatal("合法零筆與非法來源混淆")
	}
}

func mapStateFixture(t *testing.T, size int) (*machine.LEMachine, *machine.LEOPLPorts) {
	t.Helper()
	m := &machine.LEMachine{Mem: make([]byte, size)}
	ports := machine.NewLEOPLPorts()
	if !machine.InstallLEVideo(m, ports) {
		t.Fatal("video install")
	}
	m.Video.Mode = 0x13
	return m, ports
}

func TestFD2MapStateCapturesWidthBitsAndIndependentPalette(t *testing.T) {
	m, ports := mapStateFixture(t, 0xb0000)
	binary.LittleEndian.PutUint32(m.Mem[0x53a45:], 0x70000)
	binary.LittleEndian.PutUint32(m.Mem[0x53beb:], 1)
	for i := 0; i < 80; i++ {
		m.Mem[0x70000+i] = byte(i * 3)
	}
	binary.LittleEndian.PutUint32(m.Mem[0x53c0f:], 0xffff8001)
	binary.LittleEndian.PutUint32(m.Mem[0x51a93:], 0xffffffff)
	binary.LittleEndian.PutUint32(m.Mem[0x51a0c:], 0xf2)
	m.Mem[0x51aab], m.Mem[0x51aac], m.Mem[0x60002] = 1, 0, 15
	binary.LittleEndian.PutUint16(m.Mem[0x60000:], 0x8001)
	binary.LittleEndian.PutUint16(m.Mem[0x46c:], 0xffff)
	ports.Out8(0x3c8, 0)
	for i := 0; i < 768; i++ {
		ports.Out8(0x3c9, byte(i%64))
	}
	memBefore := append([]byte(nil), m.Mem...)
	portsBefore, _ := json.Marshal(ports)
	record := map[string]any{"step": 4000799496, "eip": "0x11EED"}
	appendFD2MapState(record, m, true)
	if !record["map_runtime_valid"].(bool) || !record["frame_units_valid"].(bool) {
		t.Fatal("valid source rejected")
	}
	runtime := record["map_runtime"].(map[string]any)
	fields := runtime["globals"].([]map[string]any)
	if len(fields) != 16 {
		t.Fatalf("globals %d", len(fields))
	}
	values := map[string]uint32{}
	for _, f := range fields {
		raw, e := hex.DecodeString(f["raw_hex"].(string))
		if e != nil || len(raw) != int(f["width_bytes"].(uint32)) {
			t.Fatal("raw width")
		}
		values[f["address"].(string)] = f["value"].(uint32)
	}
	for a, v := range map[string]uint32{"0x53C0F": 0xffff8001, "0x51A93": 0xffffffff, "0x51A0C": 0xf2, "0x51AAB": 1, "0x51AAC": 0, "0x60002": 15, "0x60000": 0x8001, "0x46C": 0xffff} {
		if values[a] != v {
			t.Fatalf("%s bits=%x want=%x", a, values[a], v)
		}
	}
	rgb, _ := hex.DecodeString(runtime["palette_rgb_hex"].(string))
	dac, _ := hex.DecodeString(runtime["palette_dac6_hex"].(string))
	if len(rgb) != 768 || len(dac) != 768 {
		t.Fatal("palette length")
	}
	for i := range dac {
		if dac[i] != byte(i%64) || rgb[i] != (dac[i]<<2|dac[i]>>4) {
			t.Fatalf("palette %d", i)
		}
	}
	portsAfter, _ := json.Marshal(ports)
	if !bytes.Equal(m.Mem, memBefore) || !bytes.Equal(portsBefore, portsAfter) {
		t.Fatal("observation changed guest or ports")
	}
	m.Mem[0x53c0f] = 0
	if values["0x53C0F"] != 0xffff8001 {
		t.Fatal("mutable receipt")
	}
}

func TestFD2MapStateRejectsMissingAndPartialSources(t *testing.T) {
	for _, c := range []struct {
		name  string
		size  int
		mode  byte
		count uint32
	}{
		{"last-global-missing", 0x60002, 0x13, 0},
		{"short-global", 0x60001, 0x13, 0},
		{"count", 0xb0000, 0x13, 129},
		{"video-mode", 0xb0000, 3, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			m, ports := mapStateFixture(t, c.size)
			m.Video.Mode = c.mode
			binary.LittleEndian.PutUint32(m.Mem[0x53beb:], c.count)
			before := append([]byte(nil), m.Mem...)
			portBefore, _ := json.Marshal(ports)
			rec := map[string]any{}
			appendFD2MapState(rec, m, true)
			rt := rec["map_runtime"].(map[string]any)
			if rec["map_runtime_valid"].(bool) || len(rt["globals"].([]map[string]any)) != 0 || rt["palette_rgb_hex"] != "" || rt["palette_dac6_hex"] != "" {
				t.Fatal("partial source published")
			}
			portAfter, _ := json.Marshal(ports)
			if !bytes.Equal(m.Mem, before) || !bytes.Equal(portBefore, portAfter) {
				t.Fatal("invalid source changed guest")
			}
		})
	}
	for _, m := range []*machine.LEMachine{nil, {Mem: make([]byte, 0x60003)}} {
		rec := map[string]any{}
		appendFD2MapState(rec, m, true)
		if rec["map_runtime_valid"].(bool) {
			t.Fatal("missing video accepted")
		}
	}
}

func TestFD2MapStateDisabledAndExactEnd(t *testing.T) {
	rec := map[string]any{"step": 123}
	old, _ := json.Marshal(rec)
	appendFD2MapState(rec, nil, false)
	after, _ := json.Marshal(rec)
	if !bytes.Equal(old, after) {
		t.Fatal("disabled metadata changed")
	}
	m, _ := mapStateFixture(t, 0x60003)
	binary.LittleEndian.PutUint32(m.Mem[0x53a45:], 0x60003)
	appendFD2MapState(rec, m, true)
	if !rec["map_runtime_valid"].(bool) {
		t.Fatal("exact end zero units rejected")
	}
}

func TestFD2NativeHeapProfileRoutesOnlyKnownOriginalEntries(t *testing.T) {
	for _, profile := range []string{"adapter", "native"} {
		calls := 0
		c := &cpu386.CPU{}
		c.StepHook = func(*cpu386.CPU) (bool, error) { calls++; return true, nil }
		if err := applyFD2HeapProfile(c, fd2NativeHeapEXESHA256, profile); err != nil {
			t.Fatal(err)
		}
		for _, addr := range []uint32{0x36d26, 0x37426, 0x46114, 0x36d98, 0x375c0, 0x12345} {
			c.EIP = addr
			before := *c
			oldCalls := calls
			handled, err := c.StepHook(c)
			nativeEntry := profile == "native" && (addr == 0x36d26 || addr == 0x37426 || addr == 0x46114)
			if err != nil || handled == nativeEntry || calls-oldCalls != boolInt(!nativeEntry) {
				t.Fatalf("%s %X %v %v calls%d", profile, addr, handled, err, calls-oldCalls)
			}
			if c.EIP != before.EIP || c.R != before.R || c.Seg != before.Seg || c.EFlags != before.EFlags {
				t.Fatal("routing changed guest")
			}
		}
	}
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func TestFD2NativeHeapProfileRejectsUnknownInputsAndUninstalledPlatform(t *testing.T) {
	for _, profile := range []string{"", "Native", "first-fit", "native ", "other"} {
		if validateFD2HeapProfile(profile, false) == nil {
			t.Fatalf("accepted %q", profile)
		}
	}
	if validateFD2HeapProfile("native", true) == nil {
		t.Fatal("native accepted adapter capacity")
	}
	if err := applyFD2HeapProfile(&cpu386.CPU{}, fd2NativeHeapEXESHA256, "native"); err == nil {
		t.Fatal("native accepted missing platform")
	}
	c := &cpu386.CPU{EIP: 0x36d26}
	calls := 0
	c.StepHook = func(*cpu386.CPU) (bool, error) { calls++; return true, nil }
	if err := applyFD2HeapProfile(c, "wrong", "native"); err == nil {
		t.Fatal("wrong EXE accepted")
	}
	c.StepHook(c)
	if calls != 1 {
		t.Fatal("rejection changed installed hook")
	}
	r := map[string]any{"existing": 123}
	before, _ := json.Marshal(r)
	appendFD2NativeHeapReport(r, nil)
	after, _ := json.Marshal(r)
	if !bytes.Equal(before, after) {
		t.Fatal("disabled profile altered receipt")
	}
}

func TestFD2NativeHeapObserverTracksReturnsAndReuseWithoutGuestWrites(t *testing.T) {
	m := &machine.LEMachine{Mem: make([]byte, 0x1000), CPU: &cpu386.CPU{}}
	c := m.CPU
	c.Seg[cpu386.SegDS] = 0x160
	c.Seg[cpu386.SegSS] = 0x160
	c.Descriptors = map[uint16]cpu386.Descriptor{0x160: {Base: 0, Limit: 0xfff, Writable: true}}
	o := newFD2NativeHeapObserver()
	event := func(entry, param, ptr uint32, step int) {
		binary.LittleEndian.PutUint32(m.Mem[0x100:], 0x90)
		binary.LittleEndian.PutUint32(m.Mem[0x104:], param)
		c.EIP = entry
		c.R[cpu386.ESP] = 0x100
		before := append([]byte(nil), m.Mem...)
		regs, seg, flags := c.R, c.Seg, c.EFlags
		o.observe(m, step)
		if !bytes.Equal(before, m.Mem) || regs != c.R || seg != c.Seg || flags != c.EFlags || c.EIP != entry {
			t.Fatal("entry observation changed guest")
		}
		c.EIP = 0x90
		c.R[cpu386.ESP] = 0x104
		c.R[cpu386.EAX] = ptr
		before = append([]byte(nil), m.Mem...)
		regs = c.R
		o.observe(m, step+10)
		if !bytes.Equal(before, m.Mem) || regs != c.R || seg != c.Seg || flags != c.EFlags || c.EIP != 0x90 {
			t.Fatal("return observation changed guest")
		}
	}
	binary.LittleEndian.PutUint32(m.Mem[0x1fc:], 13)
	event(0x37426, 0x200, 0, 0) // 建立free block不算先配置後重用。
	event(0x36d26, 8, 0x200, 20)
	if o.reuses != 0 {
		t.Fatal("initial free mistaken for reuse")
	}
	event(0x37426, 0x200, 0, 40)
	event(0x36d26, 8, 0x200, 60)
	if o.allocations != 2 || o.frees != 2 || o.reuses != 1 || o.invalidPointers != 0 || len(o.pending) != 0 {
		t.Fatalf("observer %+v", o)
	}
	// 完整span不足仍只記錄invalid，不越界讀取，不改原始CPU路徑。
	event(0x36d26, 8, 0xfffffff0, 80)
	if o.invalidPointers != 1 {
		t.Fatal("invalid allocation pointer passed")
	}
	// 樣本滿仍持續計數，不變成原始執行停止點。
	for i := 0; i < 260; i++ {
		event(0x36d26, 8, 0x200, 100+i*20)
	}
	if len(o.events) != 256 || o.allocations != 263 {
		t.Fatal("sample cap stopped totals")
	}
}

func TestFD2MemoryChangeAddressBounds(t *testing.T) {
	for _, text := range []string{"1,", ",1", " ", "-1", "+1", "xyz", "100000000", "1,0x1", "0x0X1", "0x"} {
		if _, err := parseFD2MemoryChangeAddresses(text); err == nil {
			t.Fatalf("accepted %q", text)
		}
	}
	parts := make([]string, 17)
	for i := range parts {
		parts[i] = strconv.FormatInt(int64(i), 16)
	}
	if _, err := parseFD2MemoryChangeAddresses(strings.Join(parts, ",")); err == nil {
		t.Fatal("accepted17")
	}
	addresses, err := parseFD2MemoryChangeAddresses("0X0, ffffffff, 0x10")
	if err != nil || len(addresses) != 3 || addresses[1] != 0xffffffff {
		t.Fatalf("valid parse %v %v", addresses, err)
	}
	if addresses, err := parseFD2MemoryChangeAddresses(""); err != nil || len(addresses) != 0 {
		t.Fatal("disabled parse")
	}
}

func memoryChangeTestMachine() *machine.LEMachine {
	m := &machine.LEMachine{Mem: make([]byte, 512)}
	copy(m.Mem, []byte{
		0xc6, 0x05, 0x00, 0x01, 0x00, 0x00, 0x01,
		0xc6, 0x05, 0x00, 0x01, 0x00, 0x00, 0x01,
		0xc7, 0x05, 0x00, 0x01, 0x00, 0x00, 0x05, 0x04, 0x03, 0x02,
		0x0f, 0x0b,
	})
	m.CPU = cpu386.New(m)
	m.CPU.SetDescriptor(0x08, cpu386.Descriptor{Limit: 511})
	m.CPU.SetDescriptor(0x10, cpu386.Descriptor{Limit: 511, Writable: true})
	m.CPU.Seg[cpu386.SegCS] = 0x08
	for _, segment := range []int{cpu386.SegDS, cpu386.SegES, cpu386.SegSS} {
		m.CPU.Seg[segment] = 0x10
	}
	return m
}

func decodeMemoryChangeRecords(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	records := []map[string]any{}
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	return records
}

func TestFD2MemoryChangeUsesSuccessfulInstructionWithoutChangingGuest(t *testing.T) {
	m, plain := memoryChangeTestMachine(), memoryChangeTestMachine()
	var output bytes.Buffer
	o := &fd2MemoryChangeObserver{addresses: []uint32{0x100, 0x101}, window: eipTraceWindow{0, 0, 20}, encoder: json.NewEncoder(&output)}
	for step := 0; step < 4; step++ {
		// 控制邊界在第二步改另一個byte，不能冒充第二步mov的writer。
		if step == 1 {
			m.Mem[0x101] = 7
			plain.Mem[0x101] = 7
		}
		err := stepFD2WithMemoryObservation(m, o, step, 19)
		wantErr := plain.CPU.Step()
		if (err == nil) != (wantErr == nil) || !bytes.Equal(m.Mem, plain.Mem) ||
			m.CPU.R != plain.CPU.R || m.CPU.EIP != plain.CPU.EIP || m.CPU.EFlags != plain.CPU.EFlags {
			t.Fatalf("observer changed guest at step%d: %v %v", step, err, wantErr)
		}
	}
	records := decodeMemoryChangeRecords(t, output.Bytes())
	if len(records) != 3 || records[0]["kind"] != "baseline" || records[1]["kind"] != "change" || records[2]["kind"] != "change" {
		t.Fatalf("wrong records %s", output.Bytes())
	}
	if records[1]["instruction_step"] != float64(0) || records[1]["step"] != float64(1) || records[1]["eip_before"] != "0x0" || records[1]["eip_after"] != "0x7" {
		t.Fatal("instruction boundary wrong")
	}
	samples := records[2]["samples"].([]any)
	if len(samples) != 2 || records[2]["instruction_step"] != float64(2) || records[2]["control_seq"] != float64(19) {
		t.Fatal("same-step byte grouping wrong")
	}
	second := samples[1].(map[string]any)
	if second["before"] != float64(7) || second["after"] != float64(4) {
		t.Fatal("control mutation attributed to instruction")
	}
}

func TestFD2MemoryChangeWindowBudgetAndInvalidSource(t *testing.T) {
	for _, w := range []eipTraceWindow{{1, 1, 20}, {0, 0, 1}, {0, 0, 2}} {
		m := memoryChangeTestMachine()
		var output bytes.Buffer
		o := &fd2MemoryChangeObserver{addresses: []uint32{0x100}, window: w, encoder: json.NewEncoder(&output)}
		for step := 0; step < 3; step++ {
			if err := stepFD2WithMemoryObservation(m, o, step, 0); err != nil {
				t.Fatal(err)
			}
		}
		records := decodeMemoryChangeRecords(t, output.Bytes())
		want := 1
		if w.max == 2 {
			want = 2
		}
		if len(records) != want {
			t.Fatalf("window%+v records%s", w, output.Bytes())
		}
		if w.from == 1 && records[0]["instruction_step"] != float64(1) {
			t.Fatal("window start not inclusive")
		}
	}
	for _, addresses := range [][]uint32{{511}, {512}, {0xffffffff}} {
		m := memoryChangeTestMachine()
		var output bytes.Buffer
		o := &fd2MemoryChangeObserver{addresses: addresses, window: eipTraceWindow{0, 0, 3}, encoder: json.NewEncoder(&output)}
		before := append([]byte(nil), m.Mem...)
		o.begin(m, 0, 0)
		records := decodeMemoryChangeRecords(t, output.Bytes())
		want := addresses[0] == 511
		if records[0]["memory_valid"] != want || !bytes.Equal(before, m.Mem) {
			t.Fatal("source validation changed memory")
		}
		if !want && (len(records[0]["samples"].([]any)) != 0 || !o.invalid) {
			t.Fatal("invalid source published data")
		}
	}
	var output bytes.Buffer
	o := &fd2MemoryChangeObserver{addresses: []uint32{0}, window: eipTraceWindow{0, 0, 3}, encoder: json.NewEncoder(&output)}
	if o.begin(nil, 0, 0) {
		t.Fatal("nil source accepted")
	}
}

func TestFD2MemoryChangeRejectsOverwriteAndDisabledDoesNotCreateFile(t *testing.T) {
	dir := t.TempDir()
	f, err := openFD2MemoryChangeLog(dir, nil)
	if err != nil || f != nil {
		t.Fatal("disabled log")
	}
	if _, err := os.Stat(filepath.Join(dir, "memory-change.jsonl")); !os.IsNotExist(err) {
		t.Fatal("disabled created file")
	}
	if _, err := openFD2MemoryChangeLog("", []uint32{1}); err == nil {
		t.Fatal("missing run-dir accepted")
	}
	f, err = openFD2MemoryChangeLog(dir, []uint32{1})
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := openFD2MemoryChangeLog(dir, []uint32{1}); err == nil {
		t.Fatal("overwritten")
	}
	m := memoryChangeTestMachine()
	var output bytes.Buffer
	o := &fd2MemoryChangeObserver{encoder: json.NewEncoder(&output)}
	if err := stepFD2WithMemoryObservation(m, o, 0, 0); err != nil || output.Len() != 0 {
		t.Fatal("disabled observation")
	}
}
