package cpu386

import "testing"

// 02 = ADD r8, r/m8、22 = AND r8, r/m8。兩個的記憶體形式原本直接 fail；FD2 第三關
// 第 2 回合結束時走到 0x1E1C1 的 `02` 就停在那裡，整場對拍收不了尾。
func TestADDByteMemoryFormUpdatesRegisterAndFlags(t *testing.T) {
	mem := testBus(make([]byte, 128))
	copy(mem, []byte{0x02, 0x0e}) // add cl, [esi]
	mem[70] = 0x03
	c := New(mem)
	c.Seg[SegDS] = 0x28
	c.SetDescriptor(0x28, Descriptor{Base: 64, Limit: 31})
	c.R[ESI] = 6
	c.R[ECX] = 0x12340004
	c.EFlags = IF
	if err := c.Step(); err != nil {
		t.Fatalf("ADD byte 記憶體形式失敗：%v", err)
	}
	if c.R[ECX] != 0x12340007 {
		t.Fatalf("ECX=%08X，應為 12340007（只改低位元組）", c.R[ECX])
	}
	if mem[70] != 3 {
		t.Fatal("來源記憶體被寫壞了；02 的目的是暫存器")
	}
	if c.EFlags&CF != 0 || c.EFlags&ZF != 0 || c.EFlags&SF != 0 {
		t.Fatalf("flags=%X：4+3 不該有進位、零或負號", c.EFlags)
	}
}

// 進位與零旗標要真的來自 byte 運算，不是 32 位元結果。
func TestADDByteMemoryFormWrapsWithinTheByte(t *testing.T) {
	mem := testBus(make([]byte, 128))
	copy(mem, []byte{0x02, 0x0e})
	mem[70] = 0x02
	c := New(mem)
	c.Seg[SegDS] = 0x28
	c.SetDescriptor(0x28, Descriptor{Base: 64, Limit: 31})
	c.R[ESI] = 6
	c.R[ECX] = 0x123400fe
	c.EFlags = IF
	if err := c.Step(); err != nil {
		t.Fatalf("ADD byte 記憶體形式失敗：%v", err)
	}
	if c.R[ECX] != 0x12340000 {
		t.Fatalf("ECX=%08X，應為 12340000：0xFE+2 在 byte 內繞回，高位元組不動", c.R[ECX])
	}
	if c.EFlags&CF == 0 || c.EFlags&ZF == 0 {
		t.Fatalf("flags=%X：0xFE+2 要同時設進位與零旗標", c.EFlags)
	}
}

func TestANDByteMemoryFormUpdatesRegisterAndFlags(t *testing.T) {
	mem := testBus(make([]byte, 128))
	copy(mem, []byte{0x22, 0x0e}) // and cl, [esi]
	mem[70] = 0x0f
	c := New(mem)
	c.Seg[SegDS] = 0x28
	c.SetDescriptor(0x28, Descriptor{Base: 64, Limit: 31})
	c.R[ESI] = 6
	c.R[ECX] = 0x123400f0
	c.EFlags = IF | CF
	if err := c.Step(); err != nil {
		t.Fatalf("AND byte 記憶體形式失敗：%v", err)
	}
	if c.R[ECX] != 0x12340000 {
		t.Fatalf("ECX=%08X，應為 12340000", c.R[ECX])
	}
	if c.EFlags&ZF == 0 || c.EFlags&CF != 0 {
		t.Fatalf("flags=%X：AND 結果為零要設 ZF 並清掉 CF", c.EFlags)
	}
}

// 暫存器形式原本就支援，補一條正對照：沒有它，上面那些「記憶體形式會過」也可能
// 只是因為整條路徑被改壞成永遠成功。
func TestADDByteRegisterFormStillWorks(t *testing.T) {
	mem := testBus(make([]byte, 16))
	copy(mem, []byte{0x02, 0xc8}) // add cl, al
	c := New(mem)
	c.R[EAX] = 0x00000003
	c.R[ECX] = 0x12340004
	c.EFlags = IF
	if err := c.Step(); err != nil || c.R[ECX] != 0x12340007 {
		t.Fatalf("暫存器形式壞了：%v ECX=%08X", err, c.R[ECX])
	}
}

// 越界讀取要失敗即關閉，不能讀到零當成合法值。
func TestADDByteMemoryFormRefusesOutOfBounds(t *testing.T) {
	mem := testBus(make([]byte, 128))
	copy(mem, []byte{0x02, 0x0e})
	c := New(mem)
	c.Seg[SegDS] = 0x28
	c.SetDescriptor(0x28, Descriptor{Base: 64, Limit: 31})
	c.R[ESI] = 64
	c.R[ECX] = 0x12340004
	c.EFlags = IF
	flags := c.EFlags
	if err := c.Step(); err == nil || c.R[ECX] != 0x12340004 || c.EFlags != flags {
		t.Fatal("越界讀取沒有保持目的暫存器與旗標")
	}
}
