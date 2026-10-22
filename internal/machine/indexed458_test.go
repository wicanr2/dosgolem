package machine

import (
	"bytes"
	"reflect"
	"testing"
	"unsafe"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// docs/spec/458-session-indexed-into-and-videowrite-ctx §3.1 的 machine 側（第 1–5、7、8、9、10–13 項）。
// 期望值一律獨立於被測本體：不拿 Indexed() 當 IndexedInto 的期望值。固定內容與金樣在 indexed458_golden_test.go。

// hand458Unchained 依 201 的公式手算非鏈結 13h 的畫面（不呼叫 Unchained256）。單位照 vga.go 寫死：
// 起點 = DisplayStart()；列距 = crtc[13h]×2（0 時退回 320/4）；分割列 = LineCompare()/(crtc[09h]&1Fh+1)。
func hand458Unchained(m *Machine) []byte {
	const w, h = 320, 200
	start := int(m.VGA.DisplayStart())
	pitch := int(m.VGA.crtc[0x13]) * 2
	if pitch == 0 {
		pitch = w / 4
	}
	split := m.VGA.LineCompare() / (int(m.VGA.crtc[0x09]&0x1F) + 1)
	out := make([]byte, w*h)
	for y := 0; y < h; y++ {
		row := start + y*pitch
		if y > split {
			row = (y - split - 1) * pitch
		}
		for x := 0; x < w; x++ {
			out[y*w+x] = m.VGA.Planes[x&3][(row+x>>2)&(PlaneSize-1)]
		}
	}
	return out
}

// hand458Planar 手算 16 色平面：像素 (x,y) = 四個平面在 (row + x/8) & (PlaneSize−1) 的第 7−x%8 位元依序組成 bit0–bit3。
// 起點 = DisplayStart()；列距 = Pitch(w)；分割列直接用 LineCompare()（不除掃描數）。
func hand458Planar(m *Machine, w, h int) []byte {
	start := int(m.VGA.DisplayStart())
	pitch := m.VGA.Pitch(w)
	split := m.VGA.LineCompare()
	out := make([]byte, w*h)
	for y := 0; y < h; y++ {
		row := start + y*pitch
		if y > split {
			row = (y - split - 1) * pitch
		}
		for x := 0; x < w; x++ {
			off := (row + x/8) & (PlaneSize - 1)
			bit := uint(7 - x%8)
			var px byte
			for p := 0; p < 4; p++ {
				px |= (m.VGA.Planes[p][off] >> bit & 1) << p
			}
			out[y*w+x] = px
		}
	}
	return out
}

func wantLayout(t *testing.T, what string, l IndexedLayout, kind IndexedKind, mode uint8, w, h int) {
	t.Helper()
	if l.Kind != kind || l.Mode != mode || l.W != w || l.H != h {
		t.Errorf("%s：layout %+v，應為 Kind=%d Mode=%02X %d×%d", what, l, kind, mode, w, h)
	}
}

// §3.1 第 1 項：線性（鏈結 13h），期望值是 m.Mem 的直接切片複本。
func TestIndexedInto458Linear(t *testing.T) {
	m := New()
	m.SetVideoMode(0x13)
	r := lcg458(0x4581)
	for i := 0; i < 64000; i++ {
		m.Mem[0xA0000+i] = r.next()
	}
	want := append([]byte(nil), m.Mem[0xA0000:0xA0000+64000]...)
	dst := make([]byte, 64000)
	l, ok := m.IndexedInto(dst)
	if !ok || !bytes.Equal(dst, want) {
		t.Errorf("線性：ok=%v 內容相同=%v", ok, bytes.Equal(dst, want))
	}
	wantLayout(t, "線性", l, IndexedLinear, 0x13, 320, 200)
}

// §3.1 第 2、3 項：非鏈結與平面（12h、0Dh），手算與金樣兩個期望值；Indexed() 也對金樣（同一本體）。
func TestIndexedInto458HandAndGolden(t *testing.T) {
	for _, c := range cases458() {
		m := New()
		c.setup(m)
		l := m.IndexedLayout()
		var hand []byte
		switch l.Kind {
		case IndexedUnchained:
			hand = hand458Unchained(m)
		case IndexedPlanar:
			hand = hand458Planar(m, l.W, l.H)
		default:
			t.Fatalf("%s：Kind %d", c.name, l.Kind)
		}
		dst := make([]byte, l.W*l.H)
		if _, ok := m.IndexedInto(dst); !ok {
			t.Fatalf("%s：ok=false", c.name)
		}
		if !bytes.Equal(dst, hand) {
			t.Errorf("%s：IndexedInto 與手算不同", c.name)
		}
		if s := sha458(dst); s != c.golden {
			t.Errorf("%s：IndexedInto SHA-256 %s，2436db8 金樣 %s", c.name, s, c.golden)
		}
		if s := sha458(m.Indexed()); s != c.golden {
			t.Errorf("%s：Indexed() SHA-256 %s，2436db8 金樣 %s", c.name, s, c.golden)
		}
	}
}

// §3.1 第 3 項另驗緩衝重用（三種來源）：哨兵 A5h，呼叫一次；換內容再呼叫一次；兩次都等於各自的期望值；
// len(dst) > W*H 時尾端仍是哨兵。
func TestIndexedInto458BufferReuse(t *testing.T) {
	type src struct {
		name   string
		setup  func(m *Machine)
		refill func(m *Machine)
		want   func(m *Machine) []byte
	}
	linear := func(m *Machine) []byte { return append([]byte(nil), m.Mem[0xA0000:0xA0000+64000]...) }
	srcs := []src{
		{"線性", func(m *Machine) {
			m.SetVideoMode(0x13)
			r := lcg458(1)
			for i := 0; i < 64000; i++ {
				m.Mem[0xA0000+i] = r.next()
			}
		}, func(m *Machine) {
			r := lcg458(2)
			for i := 0; i < 64000; i++ {
				m.Mem[0xA0000+i] = r.next()
			}
		}, linear},
		{"非鏈結", setup458Unchained, func(m *Machine) { fill458Planes(m, 77) }, hand458Unchained},
		{"mode 12h", func(m *Machine) { setup458Planar(m, 0x12, seed458Planar12, 300) },
			func(m *Machine) { fill458Planes(m, 78) }, func(m *Machine) []byte { return hand458Planar(m, 640, 480) }},
		{"mode 0Dh", func(m *Machine) { setup458Planar(m, 0x0D, seed458Planar0D, 150) },
			func(m *Machine) { fill458Planes(m, 79) }, func(m *Machine) []byte { return hand458Planar(m, 320, 200) }},
	}
	for _, s := range srcs {
		m := New()
		s.setup(m)
		l := m.IndexedLayout()
		n := l.W * l.H
		dst := bytes.Repeat([]byte{0xA5}, n+37)
		for round := 0; round < 2; round++ {
			if round == 1 {
				s.refill(m)
			}
			want := s.want(m)
			if _, ok := m.IndexedInto(dst); !ok || !bytes.Equal(dst[:n], want) {
				t.Errorf("%s 第 %d 次：ok=%v 內容與期望值不同（殘留哨兵或上一次的內容）", s.name, round+1, ok)
			}
			if !bytes.Equal(dst[n:], bytes.Repeat([]byte{0xA5}, 37)) {
				t.Errorf("%s 第 %d 次：W*H 之後的哨兵被改", s.name, round+1)
			}
		}
	}
}

// §3.1 第 7 項：布局。
func TestIndexedLayout458(t *testing.T) {
	m := New()
	m.SetVideoMode(0x13)
	wantLayout(t, "鏈結 13h", m.IndexedLayout(), IndexedLinear, 0x13, 320, 200)
	m = New()
	setup458Unchained(m)
	wantLayout(t, "非鏈結 13h", m.IndexedLayout(), IndexedUnchained, 0x13, 320, 200)
	m = New()
	m.SetVideoMode(0x12)
	wantLayout(t, "mode 12h", m.IndexedLayout(), IndexedPlanar, 0x12, 640, 480)
	m = New()
	m.SetVideoMode(0x03)
	wantLayout(t, "mode 03h", m.IndexedLayout(), IndexedLinear, 0x03, 320, 200)
	m = New()
	m.SetVideoMode(0x12)
	m.planarOn = false // 模式編號是平面模式但 planarOn 為假（白箱）
	wantLayout(t, "12h 但 planarOn 假", m.IndexedLayout(), IndexedLinear, 0x12, 320, 200)
	// IndexedInto 回傳的 layout 與 IndexedLayout() 相同。
	m = New()
	setup458Unchained(m)
	if l, _ := m.IndexedInto(make([]byte, 64000)); l != m.IndexedLayout() {
		t.Error("IndexedInto 回傳的 layout 應與 IndexedLayout() 相同")
	}
}

// §3.1 第 8 項：緩衝不足時不寫、ok=false，layout 仍有效。
func TestIndexedInto458Short(t *testing.T) {
	for _, c := range cases458() {
		m := New()
		c.setup(m)
		l := m.IndexedLayout()
		dst := bytes.Repeat([]byte{0xA5}, l.W*l.H-1)
		got, ok := m.IndexedInto(dst)
		if ok || got != l || !bytes.Equal(dst, bytes.Repeat([]byte{0xA5}, len(dst))) {
			t.Errorf("%s：緩衝不足應 ok=false、不寫入、layout 有效：ok=%v layout=%+v", c.name, ok, got)
		}
	}
	m := New()
	m.SetVideoMode(0x13)
	if _, ok := m.IndexedInto(nil); ok {
		t.Error("nil 緩衝應 ok=false")
	}
}

// §3.1 第 9 項（machine 部分）：三種來源每次 0 配置。
func TestIndexedInto458NoAlloc(t *testing.T) {
	for _, c := range append(cases458(), case458{name: "線性", setup: func(m *Machine) { m.SetVideoMode(0x13) }}) {
		m := New()
		c.setup(m)
		dst := make([]byte, 640*480)
		if n := testing.AllocsPerRun(20, func() { m.IndexedInto(dst) }); n != 0 {
			t.Errorf("%s：IndexedInto 每次配置 %v 次", c.name, n)
		}
	}
}

// ---- §3.1 第 4 項：非恆等分頁（machine 層，RunUntilObserved ＋ SetOnFrame）--------------------------

// V86 程式：ES=A000 寫 64 個 P1，ES=8000 寫 64 個 P2，迴圈。
//
//	B8 00 A0  mov ax,A000   8E C0 mov es,ax   31 FF xor di,di   B9 40 00 mov cx,64   B0 11 mov al,P1   F3 AA rep stosb
//	B8 00 80  mov ax,8000   8E C0 mov es,ax   31 FF xor di,di   B9 40 00 mov cx,64   B0 22 mov al,P2   F3 AA rep stosb
//	EB DC     jmp 0100
var paged458COM = []byte{
	0xB8, 0x00, 0xA0, 0x8E, 0xC0, 0x31, 0xFF, 0xB9, 0x40, 0x00, 0xB0, 0x11, 0xF3, 0xAA,
	0xB8, 0x00, 0x80, 0x8E, 0xC0, 0x31, 0xFF, 0xB9, 0x40, 0x00, 0xB0, 0x22, 0xF3, 0xAA,
	0xEB, 0xE2,
}

const (
	p458P1    = 0x11
	p458P2    = 0x22
	p458InitA = 0x33     // 實體 A0000 的初值
	p458InitE = 0x44     // 實體頁 E 的初值
	p458E     = 0x300000 // 線性 A0000–AFFFF 對到的實體位址（Ext 內）
	p458L     = 0x80000  // 對到實體 A0000 的線性位址
)

func paged458(t *testing.T) *Machine {
	t.Helper()
	m, tb := newPagedProg(t, paged458COM, fPWU)
	m.SetVideoMode(0x13)
	for i := uint32(0); i < 16; i++ {
		tb.mapPage(0xA0000+i*0x1000, p458E+i*0x1000)
		tb.mapPage(p458L+i*0x1000, 0xA0000+i*0x1000)
	}
	tb.enable(t) // 重設 CR3：作廢 TLB
	for i := 0; i < 0x10000; i++ {
		m.Mem[0xA0000+i] = p458InitA
		m.Ext[p458E-ExtBase+uint32(i)] = p458InitE
	}
	mustNil(t, "SetVM", m.SetVM(true))
	m.VGAFrameEvery, m.nextFrame = 97, 97
	return m
}

func TestIndexedInto458NonIdentityPaging(t *testing.T) {
	m := paged458(t)
	var writes []VideoWrite
	m.ObserveVideoWrites(func(w VideoWrite) { writes = append(writes, w) })
	obs := make([]byte, 64000)
	var obsStep uint64
	frames, bad := 0, 0
	m.SetOnFrame(func() {
		frames++
		phys := m.Mem[0xA0000 : 0xA0000+64000]
		if obsStep != m.Steps || !bytes.Equal(obs, phys) {
			bad++
		}
	})
	if _, err := m.RunUntilObserved(nil, 5000, func(m *Machine) error {
		if _, ok := m.IndexedInto(obs); !ok {
			t.Fatal("IndexedInto ok=false")
		}
		obsStep = m.Steps
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if frames < 10 || bad != 0 {
		t.Errorf("Frame 那一步的觀察點應等於實體 Mem[A0000…]：%d 次回掃、%d 次不符", frames, bad)
	}
	if bytes.IndexByte(obs, p458P1) >= 0 || obs[0] != p458P2 || obs[63] != p458P2 || obs[64] != p458InitA {
		t.Errorf("畫面應含 P2、不含 P1：前 4 bytes % X、第 64 byte %02X", obs[:4], obs[64])
	}
	if m.Ext[p458E-ExtBase] != p458P1 {
		t.Error("前提：經線性 A0000 的寫入應落在實體頁 E")
	}
	if len(writes) == 0 {
		t.Fatal("經 L 的寫入應產生 VideoWrite")
	}
	for _, w := range writes {
		if w.Value != p458P2 || w.Offset >= 64 || w.Ctx != VWV86 {
			t.Fatalf("VideoWrite 應只來自經 L 的 P2 寫入、Offset 為實體位移、Ctx=VWV86：%+v", w)
		}
	}
}

// ---- §3.1 第 5 項：零副作用（白箱）-------------------------------------------------------

// sideState458 是 IndexedInto 前後要相同的狀態。
type sideState458 struct {
	gen          uint32
	tlb          [][1024]tlbEntry
	utlbR, utlbW [utlbSize]utlbEntry
	cpu          cpu.CPU
	pt           []byte
	ptw          uint64
	latch        [4]uint8
	steps        uint64
	frames       uint64
	deliveries   uint64
	digest       [32]byte
}

func takeSide458(t *testing.T, m *Machine) sideState458 {
	t.Helper()
	s := sideState458{gen: m.tlbGen, utlbR: m.utlbR, utlbW: m.utlbW, cpu: *m.CPU, ptw: m.PageTableWrites,
		latch: m.VGA.latch, steps: m.Steps, frames: m.Frames, deliveries: m.Deliveries()}
	if m.tlb != nil {
		for _, b := range m.tlb {
			if b != nil {
				s.tlb = append(s.tlb, *b)
			}
		}
	}
	s.pt = append([]byte(nil), m.Mem[0x70000:0x80000]...)
	var buf bytes.Buffer
	mustNil(t, "SaveState", m.SaveState(&buf))
	d, err := StateDigest(&buf)
	mustNil(t, "StateDigest", err)
	s.digest = d
	return s
}

func TestIndexedInto458NoSideEffects(t *testing.T) {
	for _, mode := range []uint8{0x13, 0x12} {
		// 前置：分頁開、V86；跑幾步讓 TLB、utlb、取指令頁快取有條目；線性 A0000 那一頁不在 TLB／utlbR，
		// 該頁 PTE accessed=0；平面模式 latch 非零；讀取監看涵蓋實體 A0000–AFFFF。
		m, tb := newPagedProg(t, stepLoopCOM, fPWU)
		m.SetVideoMode(mode)
		for i := uint32(0); i < 16; i++ {
			tb.mapPage(0xA0000+i*0x1000, 0xA0000+i*0x1000) // accessed=0
		}
		tb.enable(t)
		mustNil(t, "SetVM", m.SetVM(true))
		for i := 0; i < 200; i++ {
			mustStep(t, m, "暖機")
		}
		m.VGA.latch = [4]uint8{1, 2, 3, 4}
		e := &m.utlbR[(0xA0000>>12)&(utlbSize-1)]
		if e.page == 0xA0 && e.gen == m.tlbGen {
			t.Fatal("前提：線性 A0000 不在 utlbR")
		}
		if m.tlb[0] != nil && m.tlb[0][0xA0].gen == m.tlbGen {
			t.Fatal("前提：線性 A0000 不在 TLB")
		}
		if tb.pte(0xA0000)&fA != 0 || m.tlb == nil {
			t.Fatal("前提：A0000 的 PTE accessed=0、TLB 有條目")
		}
		reads := 0
		m.WatchReads(0xA0000, 0xAFFFF, func(uint32, uint8) { reads++ })
		before := takeSide458(t, m)
		dst := make([]byte, 640*480)
		m.IndexedInto(dst)
		m.IndexedLayout()
		after := takeSide458(t, m)
		if !reflect.DeepEqual(before, after) || reads != 0 || tb.pte(0xA0000)&fA != 0 {
			t.Errorf("mode %02X：IndexedInto 改了狀態（相同=%v），讀取監看 %d 次", mode, reflect.DeepEqual(before, after), reads)
		}
	}
}

// 兩台相同的機器，一台每步呼叫 IndexedInto、一台不呼叫，跑 1000 步逐步比對 CPU 暫存器、IP、Flags（平面模式另比
// VGA.latch），結束比 machine.StateDigest 與 Deliveries。程式在迴圈中讀寫 A0000 視窗（平面模式讀取會載 latch）。
//
//	B8 00 A0  mov ax,A000 ; 8E C0 mov es,ax ; 31 FF xor di,di
//	26 8A 05  mov al,es:[di] ; FE C0 inc al ; 26 88 05 mov es:[di],al ; 47 inc di ; EB F5 jmp 0107
var rw458COM = []byte{0xB8, 0x00, 0xA0, 0x8E, 0xC0, 0x31, 0xFF, 0x26, 0x8A, 0x05, 0xFE, 0xC0, 0x26, 0x88, 0x05, 0x47, 0xEB, 0xF5}

func TestIndexedInto458TwinMachines(t *testing.T) {
	for _, mode := range []uint8{0x13, 0x12} {
		var ms [2]*Machine
		for k := range ms {
			m, _ := newPagedProg(t, rw458COM, fPWU)
			m.SetVideoMode(mode)
			fill458Planes(m, 0x4587) // 平面非零：讀取載入的 latch 才看得出差異
			mustNil(t, "SetVM", m.SetVM(true))
			m.VGAFrameEvery, m.nextFrame = 101, 101
			ms[k] = m
		}
		dst := make([]byte, 640*480)
		for i := 0; i < 1000; i++ {
			ms[1].IndexedInto(dst)
			for _, m := range ms {
				mustStep(t, m, "雙機")
			}
			a, b := ms[0].CPU, ms[1].CPU
			if a.R != b.R || a.Hi != b.Hi || a.Seg != b.Seg || a.IP != b.IP || a.Flags != b.Flags {
				t.Fatalf("mode %02X 第 %d 步：暫存器不同", mode, i)
			}
			if mode == 0x12 && ms[0].VGA.latch != ms[1].VGA.latch {
				t.Fatalf("mode %02X 第 %d 步：latch 不同", mode, i)
			}
		}
		var d [2][32]byte
		for k, m := range ms {
			var buf bytes.Buffer
			mustNil(t, "SaveState", m.SaveState(&buf))
			s, err := StateDigest(&buf)
			mustNil(t, "StateDigest", err)
			d[k] = s
		}
		if d[0] != d[1] || ms[0].Deliveries() != ms[1].Deliveries() || ms[0].Frames == 0 {
			t.Errorf("mode %02X：1000 步後 StateDigest 或 Deliveries 不同（Frames %d）", mode, ms[0].Frames)
		}
		if mode == 0x12 && ms[0].VGA.latch == ([4]uint8{}) {
			t.Errorf("前提：平面模式的程式應載入非零 latch")
		}
	}
}

// ---- §3.1 第 10–13 項：VideoWrite.Ctx ------------------------------------------------------

// captureVW 執行一步並回收到的 VideoWrite（應恰好一筆）與該步起點的 CS:IP。
func captureVW(t *testing.T, m *Machine, what string) (VideoWrite, uint16, uint16) {
	t.Helper()
	var got []VideoWrite
	m.ObserveVideoWrites(func(w VideoWrite) { got = append(got, w) })
	cs, ip := m.CPU.Seg[cpu.CS], m.CPU.IP
	mustStep(t, m, what)
	m.ObserveVideoWrites(nil)
	if len(got) != 1 {
		t.Fatalf("%s：應有一筆 VideoWrite，實際 %d", what, len(got))
	}
	return got[0], cs, ip
}

func wantVW(t *testing.T, what string, w VideoWrite, cs, ip uint16, ctx VideoWriteCtx) {
	t.Helper()
	if w.Ctx != ctx || w.CS != cs || w.IP != ip || w.Offset != 0x10 || w.Value != 0x5C {
		t.Errorf("%s：%+v，應為 Ctx=%03b CS:IP=%04X:%04X Offset=10h Value=5Ch", what, w, ctx, cs, ip)
	}
}

// §3.1 第 10 項：實模式與 V86。程式 `mov es:[di],al`（26 88 05），ES=A000、DI=10h、AL=5Ch。
func TestVideoWriteCtx458RealAndV86(t *testing.T) {
	code := []byte{0x26, 0x88, 0x05}
	prep := func(m *Machine) {
		m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DI], m.CPU.R[cpu.AX] = 0xA000, 0x10, 0x5C
	}
	m := New()
	m.IRQ0Every = 0
	mustNil(t, "LoadCOM", m.LoadCOM(code))
	m.SetVideoMode(0x13)
	prep(m)
	w, cs, ip := captureVW(t, m, "實模式")
	wantVW(t, "實模式", w, cs, ip, 0)

	m = deliverProg(t, code)
	m.SetVideoMode(0x13)
	prep(m)
	w, cs, ip = captureVW(t, m, "V86")
	wantVW(t, "V86", w, cs, ip, VWV86)
	if !w.Ctx.V86() || w.Ctx.Segmented() || w.Ctx.Code32() {
		t.Error("V86：存取器")
	}
}

// §3.1 第 11 項：分段模式 16 位元碼段（18h）、D=1 碼段（28h）、442 殘留快取實模式。ES 快取基底 A0000h（測試注入）。
func TestVideoWriteCtx458Segmented(t *testing.T) {
	setES := func(m *Machine) {
		c := m.CPU.SegCache(cpu.DS)
		c.Base, c.Limit = 0xA0000, 0xFFFF
		m.CPU.Seg[cpu.ES] = 0x40
		m.CPU.SetSegCache(cpu.ES, c)
		m.CPU.R[cpu.DI], m.CPU.Hi[cpu.DI], m.CPU.R[cpu.AX] = 0x10, 0, 0x5C
	}
	m := d0Prog(t, []byte{0x26, 0x88, 0x05}) // mov es:[di],al
	m.SetVideoMode(0x13)
	setES(m)
	w, cs, ip := captureVW(t, m, "16 位元碼段")
	wantVW(t, "16 位元碼段", w, cs, ip, VWSegmented)
	if cs != 0x18 {
		t.Errorf("16 位元碼段：CS 應為選擇子 18h，實際 %04X", cs)
	}

	m = d1Prog(t, []byte{0x26, 0x88, 0x07}) // mov es:[edi],al（D=1：modrm 07 = [edi]）
	m.SetVideoMode(0x13)
	setES(m)
	w, cs, ip = captureVW(t, m, "D=1 碼段")
	wantVW(t, "D=1 碼段", w, cs, ip, VWSegmented|VWCode32)
	if cs != sel437D1 || !w.Ctx.Code32() {
		t.Errorf("D=1 碼段：CS 應為選擇子 %04X，實際 %04X", sel437D1, cs)
	}

	m = subProg(t, []byte{0x26, 0x88, 0x05})
	m.SetVideoMode(0x13)
	setES(m)
	w, cs, ip = captureVW(t, m, "442 殘留快取實模式")
	wantVW(t, "442 殘留快取實模式", w, cs, ip, VWSegmented)
}

// §3.1 第 12 項：違反 434 不變式的狀態（非分段模式但 CS 快取 D=1），結果不含 VWCode32。
func TestVideoWriteCtx458Whitebox(t *testing.T) {
	m := New()
	m.CPU.SetSegmented(false)
	m.CPU.SetSegCache(cpu.CS, SegCache{Base: 0, Limit: 0xFFFF, Attr: 0x9A | attrDB})
	if c := m.videoWriteCtx(); c != 0 {
		t.Errorf("非分段模式、CS D=1：Ctx %03b，應為 0", c)
	}
}

// §3.1 第 13 項：大小與寫入路徑配置。
func TestVideoWrite458SizeAndAlloc(t *testing.T) {
	if s := unsafe.Sizeof(VideoWrite{}); s != 24 {
		t.Errorf("VideoWrite 大小 %d，應為 24", s)
	}
	m := New()
	m.SetVideoMode(0x13)
	m.ObserveVideoWrites(func(VideoWrite) {})
	if n := testing.AllocsPerRun(100, func() { m.Write8(0xA0010, 1) }); n != 0 {
		t.Errorf("A0000 寫入每次配置 %v 次", n)
	}
}

// 微基準（§3.3，只報數字）。
func BenchmarkIndexedIntoLinear(b *testing.B) {
	m := New()
	m.SetVideoMode(0x13)
	dst := make([]byte, 64000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.IndexedInto(dst)
	}
}

func BenchmarkIndexedIntoUnchained(b *testing.B) {
	m := New()
	setup458Unchained(m)
	dst := make([]byte, 64000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.IndexedInto(dst)
	}
}

func BenchmarkIndexedIntoPlanar640x480(b *testing.B) {
	m := New()
	setup458Planar(m, 0x12, seed458Planar12, 300)
	dst := make([]byte, 640*480)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.IndexedInto(dst)
	}
}
