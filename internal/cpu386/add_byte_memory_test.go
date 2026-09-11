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

// 目的是 r/m8 的那一半：00 ADD、08 OR、20 AND、28 SUB、30 XOR。FD2 第四關第 1 回合
// 走到 0x1C2CB 的 00 就停了，而它的反方向 02 早就支援——兩邊只差結果寫回哪裡。
func TestByteOpsWritingBackToMemory(t *testing.T) {
	for _, tc := range []struct {
		name   string
		op     byte
		mem    uint8
		reg    uint8
		want   uint8
		wantCF bool
	}{
		{"ADD", 0x00, 0x04, 0x03, 0x07, false},
		{"ADD 進位", 0x00, 0xff, 0x02, 0x01, true},
		{"OR", 0x08, 0xf0, 0x0f, 0xff, false},
		{"AND", 0x20, 0xf0, 0x3c, 0x30, false},
		{"SUB", 0x28, 0x05, 0x03, 0x02, false},
		{"SUB 借位", 0x28, 0x01, 0x02, 0xff, true},
		{"XOR", 0x30, 0xf0, 0xff, 0x0f, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mem := testBus(make([]byte, 128))
			copy(mem, []byte{tc.op, 0x0e}) // op [esi], cl
			mem[70] = tc.mem
			c := New(mem)
			c.Seg[SegDS] = 0x28
			// 寫回記憶體的形式要可寫段；segmentLinear 對 write 會檢查 Writable。
			c.SetDescriptor(0x28, Descriptor{Base: 64, Limit: 31, Writable: true})
			c.R[ESI] = 6
			c.R[ECX] = 0x12340000 | uint32(tc.reg)
			c.EFlags = IF
			if err := c.Step(); err != nil {
				t.Fatalf("%02X 記憶體形式失敗：%v", tc.op, err)
			}
			if mem[70] != tc.want {
				t.Fatalf("記憶體=%02X，應為 %02X", mem[70], tc.want)
			}
			if c.R[ECX] != 0x12340000|uint32(tc.reg) {
				t.Fatalf("來源暫存器被改到了：ECX=%08X", c.R[ECX])
			}
			if got := c.EFlags&CF != 0; got != tc.wantCF {
				t.Fatalf("CF=%v，應為 %v（flags=%X）", got, tc.wantCF, c.EFlags)
			}
		})
	}
}

func TestByteOpsWritingBackToRegister(t *testing.T) {
	mem := testBus(make([]byte, 16))
	copy(mem, []byte{0x00, 0xc8}) // add al, cl
	c := New(mem)
	c.R[EAX] = 0x00000004
	c.R[ECX] = 0x00000003
	c.EFlags = IF
	if err := c.Step(); err != nil || c.R[EAX] != 0x00000007 {
		t.Fatalf("暫存器形式：%v EAX=%08X", err, c.R[EAX])
	}
}

// 越界寫入要失敗即關閉，不能只寫一半或靜靜跳過。
func TestByteOpsRefuseOutOfBoundsWrite(t *testing.T) {
	mem := testBus(make([]byte, 128))
	copy(mem, []byte{0x00, 0x0e})
	c := New(mem)
	c.Seg[SegDS] = 0x28
	c.SetDescriptor(0x28, Descriptor{Base: 64, Limit: 31, Writable: true})
	c.R[ESI] = 64
	c.R[ECX] = 0x00000003
	if err := c.Step(); err == nil {
		t.Fatal("越界寫入沒有失敗")
	}
}
