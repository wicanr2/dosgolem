package machine

import "testing"

func TestPITDivisorFromPortWrites(t *testing.T) {
	cases := []struct {
		name string
		cmd  uint8
		data []uint8
		want uint32
	}{
		{"rich2 模式3 先低後高 17000", 0x36, []uint8{17000 & 0xFF, 17000 >> 8}, 17000},
		{"臥龍傳 4096", 0x36, []uint8{0x00, 0x10}, 4096},
		{"模式2 也要吃", 0x34, []uint8{0xFF, 0x00}, 255},
		{"只寫低位元組", 0x14, []uint8{0x40}, 0x40},
		{"只寫高位元組", 0x24, []uint8{0x02}, 0x200},
		{"寫 0 代表 65536", 0x36, []uint8{0x00, 0x00}, PITDefaultDivisor},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var p pit
			if !p.out(0x43, c.cmd) {
				t.Fatalf("命令位元組 %#02x 沒被吃下", c.cmd)
			}
			for _, v := range c.data {
				if !p.out(0x40, v) {
					t.Fatalf("資料位元組 %#02x 沒被吃下", v)
				}
			}
			if p.divisor != c.want {
				t.Fatalf("除數 %d，預期 %d", p.divisor, c.want)
			}
		})
	}
}

func TestPITIgnoresOtherChannelsAndLatch(t *testing.T) {
	var p pit
	p.out(0x43, 0x36)
	p.out(0x40, 0x68)
	p.out(0x40, 0x42) // 17000
	// 通道 2（喇叭）不該動到通道 0。
	if p.out(0x43, 0xB6) {
		t.Fatal("通道 2 的命令被當成通道 0")
	}
	if p.out(0x40, 0x11) {
		t.Fatal("通道 2 選中之後仍吃通道 0 的資料埠")
	}
	if p.divisor != 17000 {
		t.Fatalf("除數被別的通道改成 %d", p.divisor)
	}
	// 鎖存命令（存取位元 00）只讀計數，不改設定。
	p.out(0x43, 0x00)
	if p.divisor != 17000 {
		t.Fatalf("鎖存命令改到了除數：%d", p.divisor)
	}
}

func TestPITHzDefaultsToBIOSRate(t *testing.T) {
	m := &Machine{}
	if m.PITProgrammed() {
		t.Fatal("還沒被設過就回報已設定")
	}
	if got := m.PITDivisor(); got != PITDefaultDivisor {
		t.Fatalf("預設除數 %d，預期 %d", got, PITDefaultDivisor)
	}
	if hz := m.PITHz(); hz < 18.2 || hz > 18.3 {
		t.Fatalf("預設頻率 %.4f Hz，預期 18.2", hz)
	}
	m.pit.out(0x43, 0x36)
	m.pit.out(0x40, 17000&0xFF)
	m.pit.out(0x40, 17000>>8)
	if !m.PITProgrammed() {
		t.Fatal("設過了卻回報沒設過")
	}
	if hz := m.PITHz(); hz < 70.18 || hz > 70.19 {
		t.Fatalf("除數 17000 應該是 70.187 Hz，得到 %.4f", hz)
	}
}

// 除數 17,000（rich2 實際寫進去的）要換回對拍釘住的 165,000 道指令——
// 這一條把 `DefaultIRQ0Every` 綁在量到的除數上，它才不是一個裸常數。
func TestPITStepsPerTick(t *testing.T) {
	for _, c := range []struct {
		name string
		d    uint32
		want uint64
	}{
		{"rich2 17000", 17000, DefaultIRQ0Every},
		{"wolong 4096", 4096, 39755},
		{"BIOS 預設", 65536, 636085},
		{"0 當 65536", 0, 636085},
		{"除數 1 也不能回 0", 1, 10},
	} {
		if got := PITStepsPerTick(c.d); got != c.want {
			t.Errorf("%s：除數 %d 得 %d 道指令，預期 %d", c.name, c.d, got, c.want)
		}
	}
}
