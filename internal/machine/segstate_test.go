package machine

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// docs/spec/434-legacy-core-segmented-mode §9 第 3 項：PM 版本 3（分段模式旗標與六個快取）。

// segCachesForTest 是六個可辨識、且通過 §2.2 驗證的快取（CS、SS 的 D/B 為 0）。
// CS 的基底取 csBase，讓還原後的 Step 在階段 2 以快取基底取指令時仍指向同一段程式碼。
func segCachesForTest(csBase uint32) [6]SegCache {
	var c [6]SegCache
	c[cpu.ES] = SegCache{Base: 0x00110000, Limit: 0x000FFFFF, Attr: 0x0092}
	c[cpu.CS] = SegCache{Base: csBase, Limit: 0x000FFFFF, Attr: 0x809A}     // G=1
	c[cpu.SS] = SegCache{Base: 0x00020000, Limit: 0x00000000, Attr: 0x0096} // 向下展開，B=0
	c[cpu.DS] = SegCache{Base: 0x00000000, Limit: 0xFFFFFFFF, Attr: 0xC092} // G=1、B=1
	c[cpu.FS] = SegCache{}                                                  // 不可用
	c[cpu.GS] = SegCache{Base: 0x12345678, Limit: 0x0000ABCD, Attr: 0x1093} // AVL
	return c
}

// segmentedMachine 經 mov cr0 進分段模式，再把快取換成 segCachesForTest。
func segmentedMachine(t *testing.T) *Machine {
	t.Helper()
	m := handoffProg(t)
	mustNil(t, "mov cr0 設 PE", m.Step())
	if !m.CPU.Segmented() {
		t.Fatal("前提：應在分段模式")
	}
	for i, c := range segCachesForTest(csBaseOf(m)) {
		m.CPU.SetSegCache(i, c)
	}
	return m
}

func csBaseOf(m *Machine) uint32 { return uint32(m.CPU.Seg[cpu.CS]) << 4 }

func TestPMV3SegmentedLayout(t *testing.T) {
	m := segmentedMachine(t)
	b := m.encodePM()
	// 459 §5.1：encodePM 寫版本 4（佇列空時 148 bytes）；前 142 bytes 的版面照版本 3。
	if len(b) != 148 {
		t.Fatalf("PM 版本 4 共 148 bytes，實際 %d", len(b))
	}
	le := binary.LittleEndian
	if le.Uint16(b) != 4 || b[80] != 1 || b[81] != 0 {
		t.Errorf("版本號 %d、segmented（80）%d、保留（81）%d", le.Uint16(b), b[80], b[81])
	}
	names := [6]string{"ES", "CS", "SS", "DS", "FS", "GS"}
	for i, c := range segCachesForTest(csBaseOf(m)) {
		off := 82 + 10*i
		if got := le.Uint32(b[off:]); got != c.Base {
			t.Errorf("%s 基底（%d，4）= %08X，應為 %08X", names[i], off, got, c.Base)
		}
		if got := le.Uint32(b[off+4:]); got != c.Limit {
			t.Errorf("%s 界限（%d，4）= %08X，應為 %08X", names[i], off+4, got, c.Limit)
		}
		if got := le.Uint16(b[off+8:]); got != c.Attr {
			t.Errorf("%s 屬性（%d，2）= %04X，應為 %04X", names[i], off+8, got, c.Attr)
		}
	}
	f, err := decodePM(b)
	mustNil(t, "decodePM", err)
	if !f.segmented || f.cache != segCachesForTest(csBaseOf(m)) || f.cpl != 0 {
		t.Errorf("decodePM(encodePM) 不同：segmented=%v cpl=%d cache=%+v", f.segmented, f.cpl, f.cache)
	}
}

func TestLoadStatePMV3Validation(t *testing.T) {
	src := segmentedMachine(t)
	good := saveBytes(t, src)
	base := decodeStateForTest(t, good)
	mut := func(f func(pm []byte)) []byte {
		s := base
		s.PM = append([]byte(nil), base.PM...)
		f(s.PM)
		return encodeStateForTest(t, s)
	}
	le := binary.LittleEndian
	attr := func(i int, v uint16) func([]byte) { return func(p []byte) { le.PutUint16(p[82+10*i+8:], v) } }
	bad := []struct {
		name string
		f    func([]byte)
	}{
		{"segmented 為 2", func(p []byte) { p[80] = 2 }},
		{"segmented 為 FFh", func(p []byte) { p[80] = 0xFF }},
		{"保留 byte 不為 0", func(p []byte) { p[81] = 1 }},
		{"segmented=0 而快取不為 0", func(p []byte) { p[80] = 0 }},
		{"segmented=0 而只有一個快取的屬性不為 0", func(p []byte) {
			p[80] = 0
			for i := 82; i < 142; i++ {
				p[i] = 0
			}
			le.PutUint16(p[82+10*cpu.GS+8:], 0x0093)
		}},
		// 442 §2.8：segmented=1、PE=0 只在 PG=0 時可讀（殘留快取實模式，見下方對照組）。
		{"segmented=1 而 PE=0、PG=1", func(p []byte) { p[4] &^= 1; p[7] |= 0x80 }},
		{"segmented=1 而 VM=1", func(p []byte) { p[12] |= 2 }},
		{"segmented=1 而 pmCPL=1", func(p []byte) { p[3] = 1 }},
		{"segmented=1 而 pmCPL=3", func(p []byte) { p[3] = 3 }},
		{"快取屬性 bit 8", attr(cpu.DS, 0x0192)},
		{"快取屬性 bit 11", attr(cpu.ES, 0x0892)},
		{"快取屬性 bit 13", attr(cpu.GS, 0x2093)},
		{"SS 的 D/B 為 1", attr(cpu.SS, 0x4096)},
		// 443 §2.8：CS 的 D=1 只在 PE=1 時合法；PE=0（442 子狀態）時被拒。
		{"PE=0、PG=0 而 CS 的 D 為 1", func(p []byte) { p[4] &^= 1; attr(cpu.CS, 0x409A)(p) }},
	}
	for _, c := range bad {
		m := newPM(t)
		before := saveBytes(t, m)
		if err := m.LoadState(bytes.NewReader(mut(c.f))); err == nil {
			t.Errorf("%s：應回錯誤", c.name)
		}
		if !bytes.Equal(before, saveBytes(t, m)) {
			t.Errorf("%s：失敗的 LoadState 不得改變機器", c.name)
		}
	}
	// 對照組：原檔；DS、ES 的 D/B 與 CS 的 G、AVL 不驗；PE=1 時 CS 的 D 不驗（443）；segmented=0 且快取全 0（§2.1 的狀態）可讀。
	ok := []struct {
		name string
		f    func([]byte)
	}{
		{"原檔", func([]byte) {}},
		{"ES 的 D/B 為 1", attr(cpu.ES, 0x4092)},
		{"CS 的 G 與 AVL", attr(cpu.CS, 0x909A)},
		{"PE=1 而 CS 的 D 為 1（443 §2.8）", attr(cpu.CS, 0x409A)},
		{"segmented=1、PE=0、PG=0（442 的殘留快取實模式）", func(p []byte) { p[4] &^= 1 }},
		{"segmented=0 且快取全 0", func(p []byte) {
			p[80] = 0
			for i := 82; i < 142; i++ {
				p[i] = 0
			}
		}},
	}
	for _, c := range ok {
		m := newPM(t)
		mustNil(t, c.name, m.LoadState(bytes.NewReader(mut(c.f))))
	}
}

// TestLoadStateRejectsOldPMWithPE：版本 1、2 帶 PE=1、VM=0 經 LoadState 被拒（434 §2.1），
// 直接呼叫 decodePM 仍可讀；VM=1 或 PE=0 的舊版本可讀。
func TestLoadStateRejectsOldPMWithPE(t *testing.T) {
	src := newPM(t)
	mustNil(t, "LoadCOM", src.LoadCOM([]byte{0xB4, 0x4C, 0xCD, 0x21}))
	base := decodeStateForTest(t, saveBytes(t, src))
	preV4State(&base) // 459：版本 1、2 的狀態檔記憶體是舊版的 stub 區（§5.3 的升級前提）
	le := binary.LittleEndian
	make12 := func(ver uint16, cr0 uint32, hi uint16) []byte {
		cur := src.encodePM()
		size := pmV1Size
		if ver == 2 {
			size = pmV2Size
		}
		p := append([]byte(nil), cur[:size]...)
		le.PutUint16(p[0:], ver)
		le.PutUint32(p[4:], cr0)
		le.PutUint16(p[12:], hi)
		return p
	}
	for _, ver := range []uint16{1, 2} {
		for _, c := range []struct {
			name   string
			cr0    uint32
			hi     uint16
			reject bool
		}{
			{"PE=1、VM=0", 0x00000011, 0, true},
			{"PE=1、VM=0、PG=1", 0x80000011, eflagsHiRF, true},
			{"PE=1、VM=1", 0x00000011, eflagsHiVM, false},
			{"PE=0", 0x00000010, 0, false},
		} {
			pm := make12(ver, c.cr0, c.hi)
			if _, err := decodePM(pm); err != nil {
				t.Errorf("版本 %d %s：直接呼叫 decodePM 應可讀，實際 %v", ver, c.name, err)
			}
			s := base
			s.PM = pm
			m := newPM(t)
			before := saveBytes(t, m)
			err := m.LoadState(bytes.NewReader(encodeStateForTest(t, s)))
			if c.reject {
				if err == nil {
					t.Errorf("版本 %d %s：LoadState 應回錯誤", ver, c.name)
				}
				if !bytes.Equal(before, saveBytes(t, m)) {
					t.Errorf("版本 %d %s：失敗的 LoadState 不得改變機器", ver, c.name)
				}
			} else if err != nil {
				t.Errorf("版本 %d %s：LoadState 應可讀，實際 %v", ver, c.name, err)
			} else if m.CPU.Segmented() {
				t.Errorf("版本 %d %s：舊版本讀入後 segmented 應為 false", ver, c.name)
			}
		}
	}
}

// TestSegmentedRoundTrip：Snapshot／Restore 與 SaveState／LoadState 對 segmented 與六個快取往返；
// 還原 segmented=1 後 CPU.Code==nil，且 Step 以分段模式繼續：CS 快取基底指向 70000h，
// 那裡放一道可辨識的 mov al,5Ah；Seg[CS]×16＋IP 處是 nop。Step 之後 AL＝5Ah 證明自快取基底取指令。
func TestSegmentedRoundTrip(t *testing.T) {
	const csBase = 0x70000
	m := segmentedMachine(t)
	for i, c := range segCachesForTest(csBase) {
		m.CPU.SetSegCache(i, c)
	}
	ip := m.CPU.IP
	copy(m.Mem[csBase+uint32(ip):], []byte{0xB0, 0x5A}) // mov al,5Ah
	if m.Mem[csBaseOf(m)+uint32(ip)] != 0x90 {
		t.Fatalf("前提：Seg[CS]×16＋IP 處應是 nop")
	}
	want := segCachesForTest(csBase)
	snap := m.Snapshot()
	data := saveBytes(t, m)

	check := func(what string, d *Machine) {
		t.Helper()
		if !d.CPU.Segmented() {
			t.Errorf("%s：segmented 應為 true", what)
		}
		for i := range want {
			if got := d.CPU.SegCache(i); got != want[i] {
				t.Errorf("%s：快取 %d = %+v，應為 %+v", what, i, got, want[i])
			}
		}
		if d.CPU.Code != nil || !d.Translating() {
			t.Errorf("%s：還原 segmented=1 後 CPU.Code 應為 nil（xlat=%v）", what, d.Translating())
		}
	}

	// Snapshot／Restore：先把機器弄成未分段、快取清 0，再還原。
	other := handoffProg(t)
	other.Restore(snap)
	check("Restore", other)
	// 快照是複本：快照之後改機器不影響快照。
	m.CPU.SetSegCache(cpu.ES, SegCache{Base: 0xDEAD})
	other.Restore(snap)
	check("Restore（快照之後改過來源）", other)

	// Restore 到未分段的快照：segmented 回 false、快取回 0。
	rm := handoffProg(t)
	realSnap := rm.Snapshot()
	other.Restore(realSnap)
	if other.CPU.Segmented() || other.CPU.SegCache(cpu.CS) != (SegCache{}) {
		t.Error("Restore 到實模式快照：segmented 應為 false、快取清 0")
	}

	// SaveState／LoadState。
	dst := newPM(t)
	mustNil(t, "LoadState", dst.LoadState(bytes.NewReader(data)))
	check("LoadState", dst)
	if !bytes.Equal(saveBytes(t, dst), data) {
		t.Error("存檔→讀檔→存檔不一致")
	}
	// 以分段模式繼續：自 CS 快取基底取指令。
	dst.IRQ0Every = 0
	mustStep(t, dst, "還原後 Step")
	check("還原後 Step", dst)
	if uint8(dst.CPU.R[cpu.AX]) != 0x5A || dst.CPU.IP != ip+2 {
		t.Errorf("還原後應自 CS 快取基底取指令：AL=%02X IP+%d", uint8(dst.CPU.R[cpu.AX]), dst.CPU.IP-ip)
	}
	if dst.PMCPL() != 0 {
		t.Errorf("還原後 CPL = %d", dst.PMCPL())
	}
}
