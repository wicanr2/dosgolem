package cpu386

import "testing"

// DB /0 = FILD m32int。FD2 第四關第 3 回合走到 0x15BBF 的 `DB 44` 就停了。
// 記憶體形式的 reg 欄位是指令延伸碼不是暫存器，所以只認 /0——其他延伸碼行為不同，
// 放行會安靜地算出錯的數字。
func TestFILDPushesSignedInteger(t *testing.T) {
	mem := testBus(make([]byte, 128))
	copy(mem, []byte{0xdb, 0x06}) // fild dword [esi]
	// -2 的補數：FILD 讀的是有號整數，當成無號會變成 42 億。
	mem[70], mem[71], mem[72], mem[73] = 0xfe, 0xff, 0xff, 0xff
	c := New(mem)
	c.Seg[SegDS] = 0x28
	c.SetDescriptor(0x28, Descriptor{Base: 64, Limit: 31})
	c.R[ESI] = 6
	if err := c.Step(); err != nil {
		t.Fatalf("FILD 失敗：%v", err)
	}
	if c.FPUDepth != 1 {
		t.Fatalf("FPUDepth=%d，應為 1", c.FPUDepth)
	}
	if c.FPUStack[0] != -2 {
		t.Fatalf("堆疊頂=%v，應為 -2（有號）", c.FPUStack[0])
	}
}

func TestFILDPushesOntoExistingStack(t *testing.T) {
	mem := testBus(make([]byte, 128))
	copy(mem, []byte{0xdb, 0x06})
	mem[70] = 7
	c := New(mem)
	c.Seg[SegDS] = 0x28
	c.SetDescriptor(0x28, Descriptor{Base: 64, Limit: 31})
	c.R[ESI] = 6
	c.FPUStack[0] = 3.5
	c.FPUDepth = 1
	if err := c.Step(); err != nil {
		t.Fatalf("FILD 失敗：%v", err)
	}
	if c.FPUDepth != 2 || c.FPUStack[0] != 7 || c.FPUStack[1] != 3.5 {
		t.Fatalf("推疊順序不對：depth=%d st0=%v st1=%v",
			c.FPUDepth, c.FPUStack[0], c.FPUStack[1])
	}
}

func TestFILDRefusesStackOverflow(t *testing.T) {
	mem := testBus(make([]byte, 128))
	copy(mem, []byte{0xdb, 0x06})
	c := New(mem)
	c.Seg[SegDS] = 0x28
	c.SetDescriptor(0x28, Descriptor{Base: 64, Limit: 31})
	c.R[ESI] = 6
	c.FPUDepth = 8
	if err := c.Step(); err == nil {
		t.Fatal("堆疊滿了還推得進去")
	}
}

// 反對照：其他延伸碼要明確拒絕，不能當成 FILD。
func TestFILDRefusesOtherExtensions(t *testing.T) {
	for _, modrm := range []byte{0x0e, 0x16, 0x1e, 0x2e, 0x3e} { // /1 /2 /3 /5 /7
		mem := testBus(make([]byte, 128))
		copy(mem, []byte{0xdb, modrm})
		c := New(mem)
		c.Seg[SegDS] = 0x28
		c.SetDescriptor(0x28, Descriptor{Base: 64, Limit: 31})
		c.R[ESI] = 6
		if err := c.Step(); err == nil {
			t.Fatalf("DB ModRM %02X 被當成 FILD 放行了", modrm)
		}
	}
}

// 反對照：越界讀取不能推一個零上去。
func TestFILDRefusesOutOfBounds(t *testing.T) {
	mem := testBus(make([]byte, 128))
	copy(mem, []byte{0xdb, 0x06})
	c := New(mem)
	c.Seg[SegDS] = 0x28
	c.SetDescriptor(0x28, Descriptor{Base: 64, Limit: 31})
	c.R[ESI] = 64
	if err := c.Step(); err == nil || c.FPUDepth != 0 {
		t.Fatalf("越界讀取沒有失敗即關閉：depth=%d", c.FPUDepth)
	}
}

// 正對照：FINIT（DB E3）那條既有路徑不能被記憶體形式的分支吃掉。
func TestFINITStillWorks(t *testing.T) {
	mem := testBus(make([]byte, 16))
	copy(mem, []byte{0xdb, 0xe3})
	c := New(mem)
	c.FPUDepth = 3
	c.FPUStatus = 0xffff
	if err := c.Step(); err != nil || c.FPUDepth != 0 || c.FPUStatus != 0 || c.FPUControl != 0x037f {
		t.Fatalf("FINIT 壞了：%v depth=%d status=%X control=%X",
			err, c.FPUDepth, c.FPUStatus, c.FPUControl)
	}
}
