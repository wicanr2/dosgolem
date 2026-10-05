package machine

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/cpu386"
	"github.com/wicanr2/dosgolem/internal/dosfile"
)

func TestMOO2WindowsVersionAbsentPreservesState(t *testing.T) {
	mem := []byte{0xcd, 0x2f, 0x83, 0xf8, 0}
	c := cpu386.New(startupBus(mem))
	c.R = [8]uint32{cpu386.EAX: 0xabcd160a, cpu386.EDX: 0x1608a, cpu386.ESP: 0x3ebbac, cpu386.EBP: 0x3ebc06, cpu386.ESI: 0x3e0151, cpu386.EDI: 0x3a2090}
	c.Seg = [6]uint16{cpu386.SegCS: 0x180, cpu386.SegDS: 0x188, cpu386.SegES: 0x188, cpu386.SegGS: 0x20, cpu386.SegSS: 0x188}
	c.EIP, c.EFlags = 0x21788a, 0x246
	r, seg, ip, flags := c.R, c.Seg, c.EIP, c.EFlags
	before := append([]byte(nil), mem...)
	s := NewMOO2StartupDOS(nil)
	for i := 0; i < 2; i++ {
		if !s.Handle(c, 0x2f) || c.R != r || c.Seg != seg || c.EIP != ip || c.EFlags != flags || !bytes.Equal(mem, before) || s.Calls() != 0 {
			t.Fatal("未安裝 Windows 的限定查詢改變輸入狀態")
		}
	}
	if NewFD2StartupDOS(nil).Handle(c, 0x2f) {
		t.Fatal("Windows 查詢許可擴張到其他程式設定")
	}
	for _, ax := range []uint32{0x1600, 0x160b, 0x1680, 0xffff} {
		c.R[cpu386.EAX] = ax
		r = c.R
		if s.Handle(c, 0x2f) || c.R != r || c.Seg != seg || c.EIP != ip || c.EFlags != flags || !bytes.Equal(mem, before) {
			t.Fatalf("未知多工服務 %04X 未保持拒絕邊界", ax)
		}
	}
}

func TestMOO2ProtectedMouseQueryUsesControlledState(t *testing.T) {
	c := cpu386.New(startupBus(make([]byte, 8)))
	s := NewMOO2StartupDOS(nil)
	c.R[cpu386.EAX], c.R[cpu386.EBX], c.R[cpu386.ECX], c.R[cpu386.EDX], c.EFlags =
		0xabcd0003, 0x1111ffff, 0x2222ffff, 0x3333ffff, 0x216
	if !s.Handle(c, 0x33) || c.R[cpu386.EAX] != 0xabcd0003 || c.R[cpu386.EBX] != 0x11110000 ||
		c.R[cpu386.ECX] != 0x22220140 || c.R[cpu386.EDX] != 0x33330064 || c.EFlags != 0x216 {
		t.Fatalf("MOO2 滑鼠初態回傳：EAX=%X EBX=%X ECX=%X EDX=%X flags=%X",
			c.R[cpu386.EAX], c.R[cpu386.EBX], c.R[cpu386.ECX], c.R[cpu386.EDX], c.EFlags)
	}
	s.SetMouseState(417, 122, 2)
	if !s.Handle(c, 0x33) || c.R[cpu386.EBX] != 0x11110002 ||
		c.R[cpu386.ECX] != 0x222201a1 || c.R[cpu386.EDX] != 0x3333007a || c.EFlags != 0x216 {
		t.Fatalf("MOO2 受控滑鼠回傳：EBX=%X ECX=%X EDX=%X flags=%X",
			c.R[cpu386.EBX], c.R[cpu386.ECX], c.R[cpu386.EDX], c.EFlags)
	}
	c.R[cpu386.EAX] = 5
	before := c.R
	if s.Handle(c, 0x33) || c.R != before {
		t.Fatal("未列的滑鼠功能應失敗即關閉")
	}
	fd2 := NewFD2StartupDOS(nil)
	c.R[cpu386.EAX] = 3
	before = c.R
	if fd2.Handle(c, 0x33) || c.R != before {
		t.Fatal("通用 FD2 啟動設定不得啟用 MOO2 滑鼠預設")
	}
}

func TestMOO2ProtectedMouseSoftwareReset(t *testing.T) {
	c := cpu386.New(startupBus(make([]byte, 8)))
	s := NewMOO2StartupDOS(nil)
	s.SetMouseState(417, 122, 2)
	c.R[cpu386.EAX], c.R[cpu386.EBX], c.R[cpu386.ECX], c.R[cpu386.EDX], c.EFlags =
		0xabcd0021, 0x11110000, 0x22220005, 0x33330006, 0x216
	beforeSeg := c.Seg
	if !s.Handle(c, 0x33) || c.R[cpu386.EAX] != 0xabcdffff || c.R[cpu386.EBX] != 0x11110003 ||
		c.R[cpu386.ECX] != 0x22220005 || c.R[cpu386.EDX] != 0x33330006 || c.EFlags != 0x216 || c.Seg != beforeSeg {
		t.Fatalf("MOO2 滑鼠軟體重設返回：R=%X flags=%X Seg=%X", c.R, c.EFlags, c.Seg)
	}
	c.R[cpu386.EAX] = 0xabcd0003
	if !s.Handle(c, 0x33) || c.R[cpu386.EBX] != 0x11110000 ||
		c.R[cpu386.ECX] != 0x22220140 || c.R[cpu386.EDX] != 0x33330064 {
		t.Fatalf("MOO2 軟體重設後位置／按鍵：R=%X", c.R)
	}
	fd2 := NewFD2StartupDOS(nil)
	c.R[cpu386.EAX] = 0x21
	beforeR := c.R
	if fd2.Handle(c, 0x33) || c.R != beforeR {
		t.Fatal("一般 FD2 啟動設定不得接受 MOO2 專用重設")
	}
}

func TestMOO2MouseResetRespectsVideoModeAndPreservesRegisters(t *testing.T) {
	for _, function := range []uint16{0, 0x21} {
		for _, video := range []bool{false, true} {
			c := cpu386.New(startupBus(make([]byte, 8)))
			s := NewMOO2StartupDOS(nil)
			wantY := uint32(100)
			if video {
				c.R[cpu386.EAX], c.R[cpu386.EBX] = 0x4f02, 0x0101
				if !s.Handle(c, 0x10) {
					t.Fatal("VBE 模式設定失敗")
				}
				wantY = 240
			}
			s.SetMouseState(417, 122, 2)
			s.mouseSensitivityX, s.mouseSensitivityY, s.mouseDoubleSpeed = 0, 0, 0
			c.R = [8]uint32{0xabcd0000 | uint32(function), 0x12340000, 0x22330005, 0x33440006, 0x11000004, 0x55000005, 0x66000006, 0x77000007}
			c.EFlags = 0x16
			want, beforeSeg := c.R, c.Seg
			want[cpu386.EAX] = 0xabcdffff
			want[cpu386.EBX] = c.R[cpu386.EBX]&0xffff0000 | 3
			if !s.Handle(c, 0x33) || c.R != want || c.Seg != beforeSeg || c.EFlags != 0x16 || s.mouseSensitivityX != 0 || s.mouseSensitivityY != 0 || s.mouseDoubleSpeed != 0 {
				t.Fatalf("重設功能=%X video=%t R=%X flags=%X", function, video, c.R, c.EFlags)
			}
			c.R[cpu386.EAX] = 3
			if !s.Handle(c, 0x33) || uint16(c.R[cpu386.EBX]) != 0 || uint16(c.R[cpu386.ECX]) != 320 || uint16(c.R[cpu386.EDX]) != uint16(wantY) {
				t.Fatalf("重設後查詢：video=%t R=%X", video, c.R)
			}
		}
	}
	c := cpu386.New(startupBus(make([]byte, 8)))
	c.EFlags = 0x16
	before := c.R
	if NewFD2StartupDOS(nil).Handle(c, 0x33) || c.R != before || c.EFlags != 0x16 {
		t.Fatal("一般 FD2 不得接受重設")
	}
}

func TestMOO2MouseSensitivityQueryStateRoundTrip(t *testing.T) {
	for _, zero := range []bool{false, true} {
		for _, reset := range []uint16{0, 0x21} {
			c := cpu386.New(startupBus(make([]byte, 8)))
			s := NewMOO2StartupDOS(nil)
			value := uint32(50)
			if zero {
				c.R[cpu386.EAX] = 0x1a
				if !s.Handle(c, 0x33) {
					t.Fatal("零敏感度 setter")
				}
				value = 0
			}
			c.R[cpu386.EAX] = uint32(reset)
			if !s.Handle(c, 0x33) {
				t.Fatal("滑鼠重設")
			}
			c.R = [8]uint32{0xabcd001b, 0x11220007, 0x22330008, 0x33440009, 0x100004, 0x100005, 0x100006, 0x100007}
			c.EFlags = 0x12
			want, beforeSeg := c.R, c.Seg
			for _, reg := range []int{cpu386.EBX, cpu386.ECX, cpu386.EDX} {
				want[reg] = want[reg]&0xffff0000 | value
			}
			for i := 0; i < 2; i++ {
				if !s.Handle(c, 0x33) || c.R != want || c.Seg != beforeSeg || c.EFlags != 0x12 || uint32(s.mouseSensitivityX) != value || uint32(s.mouseSensitivityY) != value || uint32(s.mouseDoubleSpeed) != value {
					t.Fatalf("敏感度查詢 zero=%t reset=%X R=%X flags=%X", zero, reset, c.R, c.EFlags)
				}
			}
		}
	}
	c := cpu386.New(startupBus(make([]byte, 8)))
	c.R[cpu386.EAX], c.EFlags = 0x1b, 0x12
	before := c.R
	if NewFD2StartupDOS(nil).Handle(c, 0x33) || c.R != before || c.EFlags != 0x12 {
		t.Fatal("一般 FD2 不得接受敏感度查詢")
	}
}

func TestMOO2MouseCoordinateRanges(t *testing.T) {
	cases := []struct {
		name              string
		a, b, start, want int16
		inputs, outputs   []int16
	}{
		{"水平原版", 0, 1278, 1500, 1278, []int16{-1, 0, 1278, 1279}, []int16{0, 0, 1278, 1278}},
		{"垂直原版", 0, 479, -1, 0, []int16{-1, 0, 479, 480}, []int16{0, 0, 479, 479}},
		{"反序", 800, 100, 320, 320, []int16{99, 100, 799, 800, 801}, []int16{100, 100, 799, 800, 800}},
		{"負值", -10, -40, 10, -10, []int16{-41, -40, -25, -10, -9}, []int16{-40, -40, -25, -10, -10}},
		{"跨零", -20, 20, -30, -20, []int16{-21, 0, 21}, []int16{-20, 0, 20}},
		{"零寬", 15, 15, 320, 15, []int16{14, 15, 16}, []int16{15, 15, 15}},
		{"有號極值", 32767, -32768, -32768, -32768, []int16{-32768, 0, 32767}, []int16{-32768, 0, 32767}},
	}
	for _, function := range []uint16{7, 8} {
		for _, tc := range cases {
			t.Run(tc.name+string(rune('0'+function)), func(t *testing.T) {
				c := cpu386.New(startupBus(make([]byte, 8)))
				s := NewMOO2StartupDOS(nil)
				s.SetMouseState(uint16(tc.start), uint16(tc.start), 2)
				c.R = [8]uint32{0xabcd0000 | uint32(function), 0x11220000 | uint32(uint16(tc.a)), 0x22330000 | uint32(uint16(tc.b)), 0x33440055, 0x100004, 0x100005, 0x100006, 0x100007}
				c.EFlags = 0x16
				before, segments := c.R, c.Seg
				if !s.Handle(c, 0x33) || c.R != before || c.Seg != segments || c.EFlags != 0x16 || s.mouseButtons != 2 || s.mouseSensitivityX != 50 || s.mouseSensitivityY != 50 || s.mouseDoubleSpeed != 50 {
					t.Fatalf("範圍設定改動不應改的欄位：R=%X flags=%X", c.R, c.EFlags)
				}
				check := func(axisWant, otherWant int16) {
					t.Helper()
					c.R[cpu386.EAX] = 3
					if !s.Handle(c, 0x33) || uint16(c.R[cpu386.EBX]) != 2 {
						t.Fatal("位置查詢失敗或改按鍵")
					}
					axis, other := int16(c.R[cpu386.ECX]), int16(c.R[cpu386.EDX])
					if function == 8 {
						axis, other = other, axis
					}
					if axis != axisWant || other != otherWant {
						t.Fatalf("axis=%d other=%d，預期 %d/%d", axis, other, axisWant, otherWant)
					}
				}
				check(tc.want, tc.start)
				for i, input := range tc.inputs {
					s.SetMouseState(uint16(input), uint16(input), 2)
					check(tc.outputs[i], input)
				}
			})
		}
	}
	for _, function := range []uint16{7, 8} {
		c := cpu386.New(startupBus(make([]byte, 8)))
		c.R[cpu386.EAX], c.R[cpu386.ECX], c.R[cpu386.EDX], c.EFlags = uint32(function), 0, 1278, 0x16
		before := c.R
		if NewFD2StartupDOS(nil).Handle(c, 0x33) || c.R != before || c.EFlags != 0x16 {
			t.Fatal("一般 FD2 不得接受範圍設定")
		}
	}
}

func TestMOO2MouseCoordinateRangeResetAndAxisIsolation(t *testing.T) {
	for _, function := range []uint16{0, 0x21} {
		c := cpu386.New(startupBus(make([]byte, 8)))
		s := NewMOO2StartupDOS(nil)
		c.R[cpu386.EAX], c.R[cpu386.EBX] = 0x4f02, 0x101
		if !s.Handle(c, 0x10) {
			t.Fatal("VBE 模式設定")
		}
		c.R[cpu386.EAX], c.R[cpu386.ECX], c.R[cpu386.EDX] = 7, 0, 1278
		if !s.Handle(c, 0x33) {
			t.Fatal("水平範圍設定")
		}
		c.R[cpu386.EAX], c.R[cpu386.EDX] = 8, 479
		if !s.Handle(c, 0x33) {
			t.Fatal("垂直範圍設定")
		}
		s.SetMouseState(1300, 500, 2)
		if s.mouseX != 1278 || s.mouseY != 479 {
			t.Fatal("兩軸範圍未共同限制注入")
		}
		c.R[cpu386.EAX], c.R[cpu386.ECX], c.R[cpu386.EDX] = 7, 100, 200
		if !s.Handle(c, 0x33) || s.mouseX != 200 || s.mouseY != 479 {
			t.Fatal("修改水平範圍影響垂直狀態")
		}
		c.R[cpu386.EAX] = uint32(function)
		if !s.Handle(c, 0x33) || s.mouseRangeX.set || s.mouseRangeY.set || s.mouseX != 320 || s.mouseY != 240 || s.mouseButtons != 0 {
			t.Fatal("重設未清自訂範圍或未回模式中心")
		}
		s.SetMouseState(417, 122, 2)
		if s.mouseX != 417 || s.mouseY != 122 {
			t.Fatal("重設後仍套舊自訂範圍")
		}
	}
}

func TestMOO2ProtectedMouseZeroSensitivity(t *testing.T) {
	c := cpu386.New(startupBus(make([]byte, 8)))
	s := NewMOO2StartupDOS(nil)
	if s.mouseSensitivityX != 50 || s.mouseSensitivityY != 50 || s.mouseDoubleSpeed != 50 {
		t.Fatalf("MOO2 受控敏感度初態=%d,%d,%d", s.mouseSensitivityX, s.mouseSensitivityY, s.mouseDoubleSpeed)
	}
	s.SetMouseState(417, 122, 2)
	c.R[cpu386.EAX], c.R[cpu386.EBX], c.R[cpu386.ECX], c.R[cpu386.EDX], c.EFlags =
		0xabcd001a, 0x11110000, 0x22220000, 0x33330000, 0x212
	beforeR, beforeSeg, beforeFlags := c.R, c.Seg, c.EFlags
	if !s.Handle(c, 0x33) || c.R != beforeR || c.Seg != beforeSeg || c.EFlags != beforeFlags ||
		s.mouseSensitivityX != 0 || s.mouseSensitivityY != 0 || s.mouseDoubleSpeed != 0 {
		t.Fatalf("MOO2 零敏感度設定：R=%X flags=%X raw=%d,%d,%d", c.R, c.EFlags,
			s.mouseSensitivityX, s.mouseSensitivityY, s.mouseDoubleSpeed)
	}
	c.R[cpu386.EAX] = 3
	if !s.Handle(c, 0x33) || c.R[cpu386.EBX] != 0x11110002 ||
		c.R[cpu386.ECX] != 0x222201a1 || c.R[cpu386.EDX] != 0x3333007a {
		t.Fatalf("敏感度設定不應改受控位置與按鍵：R=%X", c.R)
	}
	c.R[cpu386.EAX], c.R[cpu386.EBX], c.R[cpu386.ECX], c.R[cpu386.EDX] = 0x1a, 0x11110001, 0x22220000, 0x33330000
	beforeR = c.R
	if !s.Handle(c, 0x33) || c.R != beforeR || s.mouseSensitivityX != 1 || s.mouseSensitivityY != 0 || s.mouseDoubleSpeed != 0 {
		t.Fatal("規格 254：非零敏感度設定須接受且保留暫存器")
	}
	fd2 := NewFD2StartupDOS(nil)
	c.R[cpu386.EBX] = 0
	beforeR = c.R
	if fd2.Handle(c, 0x33) || c.R != beforeR {
		t.Fatal("一般 FD2 啟動設定不得接受 MOO2 專用敏感度設定")
	}
}

func TestMOO2MouseSensitivitySettingLimitsAndState(t *testing.T) {
	cases := []struct{ input, want [3]uint16 }{
		{[3]uint16{100, 100, 479}, [3]uint16{100, 100, 100}},
		{[3]uint16{0, 0, 0}, [3]uint16{0, 0, 0}},
		{[3]uint16{0, 51, 99}, [3]uint16{0, 51, 99}},
		{[3]uint16{99, 100, 101}, [3]uint16{99, 100, 100}},
		{[3]uint16{101, 65535, 0}, [3]uint16{100, 100, 0}},
		{[3]uint16{50, 50, 50}, [3]uint16{50, 50, 50}},
	}
	for _, tc := range cases {
		c := cpu386.New(startupBus(make([]byte, 8)))
		s := NewMOO2StartupDOS(nil)
		for _, axis := range []uint32{7, 8} {
			c.R[cpu386.EAX], c.R[cpu386.ECX], c.R[cpu386.EDX] = axis, 100, 500
			if !s.Handle(c, 0x33) {
				t.Fatal("範圍設定")
			}
		}
		s.SetMouseState(417, 122, 2)
		rangeX, rangeY := s.mouseRangeX, s.mouseRangeY
		c.R = [8]uint32{0xabcd001a, 0x22330000 | uint32(tc.input[1]), 0x33440000 | uint32(tc.input[2]), 0x11220000 | uint32(tc.input[0]), 0x100004, 0x100005, 0x100006, 0x100007}
		c.EFlags = 0x16
		before, segments := c.R, c.Seg
		for repeat := 0; repeat < 2; repeat++ {
			if !s.Handle(c, 0x33) || c.R != before || c.Seg != segments || c.EFlags != 0x16 || s.mouseRangeX != rangeX || s.mouseRangeY != rangeY || s.mouseX != 417 || s.mouseY != 122 || s.mouseButtons != 2 {
				t.Fatalf("敏感度設定有不應發生的副作用：input=%v R=%X", tc.input, c.R)
			}
		}
		c.R[cpu386.EAX] = 0xabcd001b
		want := before
		want[cpu386.EAX] = 0xabcd001b
		for i, reg := range []int{cpu386.EBX, cpu386.ECX, cpu386.EDX} {
			want[reg] = want[reg]&0xffff0000 | uint32(tc.want[i])
		}
		if !s.Handle(c, 0x33) || c.R != want || c.Seg != segments || c.EFlags != 0x16 {
			t.Fatalf("敏感度讀回：input=%v R=%X，預期 %X", tc.input, c.R, want)
		}
		for _, reset := range []uint32{0, 0x21} {
			c.R[cpu386.EAX] = reset
			if !s.Handle(c, 0x33) {
				t.Fatal("重設")
			}
			c.R[cpu386.EAX] = 0xabcd001b
			if !s.Handle(c, 0x33) || c.R != want || c.Seg != segments || c.EFlags != 0x16 {
				t.Fatalf("重設不應改敏感度：input=%v reset=%X R=%X", tc.input, reset, c.R)
			}
		}
		c.R = before
		if NewFD2StartupDOS(nil).Handle(c, 0x33) || c.R != before || c.EFlags != 0x16 {
			t.Fatal("一般 FD2 不得接受設定")
		}
	}
}

func TestMOO2ProtectedVideoMode03(t *testing.T) {
	c := cpu386.New(startupBus(make([]byte, 8)))
	s := NewMOO2StartupDOS(nil)
	c.R[cpu386.EAX], c.R[cpu386.EBX], c.R[cpu386.ECX], c.R[cpu386.EDX], c.EFlags =
		3, 0x01800188, 0x01880188, 0x00200000, 0x216
	beforeR, beforeSeg, beforeFlags := c.R, c.Seg, c.EFlags
	if !s.Handle(c, 0x10) || c.R != beforeR || c.Seg != beforeSeg || c.EFlags != beforeFlags ||
		!s.videoModeSet || s.videoMode != 3 {
		t.Fatalf("MOO2 模式 03h 啟動返回：R=%X Seg=%X flags=%X mode=%d set=%v",
			c.R, c.Seg, c.EFlags, s.videoMode, s.videoModeSet)
	}
	c.R[cpu386.EAX] = 0x13
	beforeR = c.R
	if s.Handle(c, 0x10) || c.R != beforeR || s.videoMode != 3 {
		t.Fatal("未審查的視訊模式須拒絕且保留既有設定")
	}
	fd2 := NewFD2StartupDOS(nil)
	c.R[cpu386.EAX] = 3
	beforeR = c.R
	if fd2.Handle(c, 0x10) || c.R != beforeR {
		t.Fatal("一般 FD2 啟動設定不得接受 MOO2 視訊模式近似")
	}
}

func TestMOO2ProtectedVBEZeroDisplayStart(t *testing.T) {
	c := cpu386.New(startupBus(make([]byte, 8)))
	s := NewMOO2StartupDOS(nil)
	c.R[cpu386.EAX], c.R[cpu386.EBX], c.R[cpu386.ECX], c.R[cpu386.EDX], c.EFlags =
		0x4f07, 0, 0, 0, 0x216
	beforeR, beforeSeg, beforeFlags := c.R, c.Seg, c.EFlags
	if !s.Handle(c, 0x10) || c.R[cpu386.EAX] != 0x4f || c.Seg != beforeSeg || c.EFlags != beforeFlags ||
		!s.vbeStartSet || s.vbeStartX != 0 || s.vbeStartY != 0 {
		t.Fatalf("MOO2 4F07h 零座標返回錯誤：R=%X Seg=%X flags=%X start=%d,%d set=%t",
			c.R, c.Seg, c.EFlags, s.vbeStartX, s.vbeStartY, s.vbeStartSet)
	}
	beforeR[cpu386.EAX] = 0x4f
	if c.R != beforeR {
		t.Fatal("4F07h 改動非目的暫存器")
	}
	for _, regs := range [][4]uint32{{0x14f07, 0, 0, 0}, {0x4f07, 1, 0, 0}, {0x4f07, 0, 1, 0}, {0x4f07, 0, 0, 1}} {
		c.R[cpu386.EAX], c.R[cpu386.EBX], c.R[cpu386.ECX], c.R[cpu386.EDX] = regs[0], regs[1], regs[2], regs[3]
		beforeR = c.R
		if s.Handle(c, 0x10) || c.R != beforeR || !s.vbeStartSet || s.vbeStartX != 0 || s.vbeStartY != 0 {
			t.Fatalf("未知 4F07h 形狀須拒絕：R=%X", c.R)
		}
	}
	fd2 := NewFD2StartupDOS(nil)
	c.R[cpu386.EAX], c.R[cpu386.EBX], c.R[cpu386.ECX], c.R[cpu386.EDX] = 0x4f07, 0, 0, 0
	beforeR = c.R
	if fd2.Handle(c, 0x10) || c.R != beforeR {
		t.Fatal("一般 FD2 設定不得接受 MOO2 4F07h")
	}
}

func TestMOO2ProtectedVBEMode0101(t *testing.T) {
	c := cpu386.New(startupBus(make([]byte, 8)))
	s := NewMOO2StartupDOS(nil)
	c.R[cpu386.EAX], c.R[cpu386.EBX], c.R[cpu386.ECX], c.R[cpu386.EDX], c.EFlags =
		0x4f02, 0x0101, 0, 0, 0x216
	beforeR, beforeSeg, beforeFlags := c.R, c.Seg, c.EFlags
	if !s.Handle(c, 0x10) || c.R[cpu386.EAX] != 0x4f || c.Seg != beforeSeg || c.EFlags != beforeFlags ||
		!s.vbeModeSet || s.vbeMode != 0x0101 {
		t.Fatalf("MOO2 4F02h 模式 0101h 返回錯誤：R=%X Seg=%X flags=%X mode=%04X set=%t",
			c.R, c.Seg, c.EFlags, s.vbeMode, s.vbeModeSet)
	}
	beforeR[cpu386.EAX] = 0x4f
	if c.R != beforeR {
		t.Fatal("4F02h 改動非目的暫存器")
	}
	for _, regs := range [][4]uint32{{0x14f02, 0x101, 0, 0}, {0x4f02, 0x100, 0, 0}, {0x4f02, 0x10101, 0, 0}, {0x4f02, 0x101, 1, 0}, {0x4f02, 0x101, 0, 1}} {
		c.R[cpu386.EAX], c.R[cpu386.EBX], c.R[cpu386.ECX], c.R[cpu386.EDX] = regs[0], regs[1], regs[2], regs[3]
		beforeR = c.R
		if s.Handle(c, 0x10) || c.R != beforeR || !s.vbeModeSet || s.vbeMode != 0x0101 {
			t.Fatalf("未知 4F02h 形狀須拒絕：R=%X", c.R)
		}
	}
	fd2 := NewFD2StartupDOS(nil)
	c.R[cpu386.EAX], c.R[cpu386.EBX], c.R[cpu386.ECX], c.R[cpu386.EDX] = 0x4f02, 0x0101, 0, 0
	beforeR = c.R
	if fd2.Handle(c, 0x10) || c.R != beforeR {
		t.Fatal("一般 FD2 設定不得接受 MOO2 4F02h")
	}
}

func TestMOO2AttachMachineSharesProtectedAndRealModePorts(t *testing.T) {
	m := &LEMachine{Mem: make([]byte, 0x120000), Ports: map[uint16]uint8{}}
	m.CPU = cpu386.New(m)
	s := NewMOO2StartupDOS(nil)
	if err := s.AttachMachine(m); err != nil {
		t.Fatal(err)
	}
	if m.Mem[0x463] != 0xd4 || m.Mem[0x464] != 0x03 {
		t.Fatal("MOO2 啟動沒有初始化彩色 CRTC 基底埠")
	}
	if m.CPU.PortIn == nil || m.CPU.PortOut == nil || s.DPMI.RealModeIO == nil {
		t.Fatal("MOO2 兩種模式未連至平台埠")
	}
	if !m.CPU.PortOut(0x226, 1) || !s.DPMI.RealModeIO.Out8(0x226, 0) {
		t.Fatal("跨模式 DSP 重設寫入失敗")
	}
	if value, ok := m.CPU.PortIn(0x22a); !ok || value != 0xaa {
		t.Fatalf("保護模式讀不到實模式 DSP 重設回覆：%02X ok=%t", value, ok)
	}
	if !m.CPU.PortOut(0x0d4, 1) || !s.DPMI.RealModeIO.Out8(0x0d4, 5) {
		t.Fatal("跨模式第二 DMA 控制器遮罩寫入失敗")
	}
	if m.CPU.PortOut(0x0d2, 5) || s.DPMI.RealModeIO.Out8(0x0d2, 5) {
		t.Fatal("未知第二 DMA 控制器埠被放行")
	}
	fd2 := NewFD2StartupDOS(nil)
	other := &LEMachine{Mem: make([]byte, 8)}
	other.CPU = cpu386.New(other)
	fd2.AttachMachine(other)
	if other.CPU.PortIn != nil || other.CPU.PortOut != nil || fd2.DPMI.RealModeIO != nil {
		t.Fatal("一般 FD2 設定被 MOO2 專用接線改動")
	}
}

func TestMOO2AttachMachineAdvancesBIOSClockAndPreservesHook(t *testing.T) {
	m := &LEMachine{Mem: make([]byte, 0x120000)}
	m.CPU = cpu386.New(m)
	calls := 0
	m.CPU.StepHook = func(*cpu386.CPU) (bool, error) { calls++; return true, nil }
	s := NewMOO2StartupDOS(nil)
	if err := s.AttachMachine(m); err != nil {
		t.Fatal(err)
	}
	p, ok := s.DPMI.RealModeIO.(*LEOPLPorts)
	if !ok || p.BIOSClock == nil {
		t.Fatal("MOO2 未接既有 BIOS 時鐘")
	}
	m.CPU.EFlags = 0x202
	beforeR, beforeSeg := m.CPU.R, m.CPU.Seg
	for i := 0; i < 54925; i++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if binary.LittleEndian.Uint32(m.Mem[0x46c:]) != 0 {
		t.Fatal("tick 過早")
	}
	if err := m.CPU.Step(); err != nil || calls != 54926 || p.BIOSClock.Micros != 54926 ||
		binary.LittleEndian.Uint32(m.Mem[0x46c:]) != 1 || m.CPU.R != beforeR || m.CPU.Seg != beforeSeg || m.CPU.EFlags != 0x202 {
		t.Fatalf("MOO2 時鐘／hook：calls=%d micros=%d flags=%X err=%v", calls, p.BIOSClock.Micros, m.CPU.EFlags, err)
	}
	binary.LittleEndian.PutUint32(m.Mem[8*4:], 0x1234)
	p.BIOSClock.Pending = true
	if err := m.CPU.Step(); err == nil || calls != 54926 || m.CPU.R != beforeR || m.CPU.EFlags != 0x202 {
		t.Fatalf("客製 IRQ0 未拒絕：calls=%d err=%v", calls, err)
	}
}

func TestMOO2AttachMachineRejectsBIOSOverlap(t *testing.T) {
	m := &LEMachine{Mem: make([]byte, 0x120000)}
	m.CPU = cpu386.New(m)
	m.Mem[0x463] = 0xff
	s := NewMOO2StartupDOS(nil)
	if err := s.AttachMachine(m); err == nil {
		t.Fatal("既有 BIOS 資料被覆寫")
	}
	if m.Mem[0x463] != 0xff || m.CPU.PortIn != nil || s.DPMI.RealModeIO != nil {
		t.Fatal("安裝失敗仍修改了原有狀態")
	}
}

func TestProtectedDOSFindFirstExactMissingAndPresent(t *testing.T) {
	root := t.TempDir()
	provider, err := OpenDirectoryReadOnlyFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { provider.Close() })
	bus := startupBus(make([]byte, 0x200))
	copy(bus[0x20:], "MOX.SET\x00")
	for i := 0x80; i < 0x80+43; i++ {
		bus[i] = 0xa5
	}
	c := cpu386.New(bus)
	c.Seg[cpu386.SegDS] = 0x188
	c.SetDescriptor(0x188, cpu386.Descriptor{Limit: 0x1ff, Writable: true})
	s := NewMOO2StartupDOS(provider)
	c.R[cpu386.EAX], c.R[cpu386.EDX], c.EFlags = 0x381a99, 0x80, 0x246
	if !s.Handle(c, 0x21) {
		t.Fatal("Set DTA")
	}
	c.R[cpu386.EAX], c.R[cpu386.ECX], c.R[cpu386.EDX] = 0x384e99, 0, 0x20
	if !s.Handle(c, 0x21) || c.R[cpu386.EAX] != 0x12 || c.EFlags != 0x247 || c.R[cpu386.EDX] != 0x20 ||
		!bytes.Equal(bus[0x80:0x8c], []byte{2, 'M', 'O', 'X', 0, 0, 0, 0, 0, 'S', 'E', 'T'}) ||
		!bytes.Equal(bus[0x8c:0x80+43], bytes.Repeat([]byte{0xa5}, 31)) {
		t.Fatalf("缺檔收據：EAX=%X flags=%X DTA=% X", c.R[cpu386.EAX], c.EFlags, bus[0x80:0x80+43])
	}
	path := filepath.Join(root, "MOX.SET")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(1996, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	c.R[cpu386.EAX], c.EFlags = 0x384e99, 0x246
	if !s.Handle(c, 0x21) || c.R[cpu386.EAX] != 0x380000 || c.EFlags != 0x246 ||
		bus[0x80+0x15] != 0x20 || binary.LittleEndian.Uint16(bus[0x80+0x16:]) != 0 ||
		binary.LittleEndian.Uint16(bus[0x80+0x18:]) != 0x2021 ||
		binary.LittleEndian.Uint32(bus[0x80+0x1a:]) != 0 ||
		string(bus[0x80+0x1e:0x80+0x26]) != "MOX.SET\x00" {
		t.Fatalf("存在收據：EAX=%X flags=%X DTA=% X", c.R[cpu386.EAX], c.EFlags, bus[0x80:0x80+43])
	}
}

func TestProtectedDOSFindFirstRejectsUnsupportedAndInvalidDTA(t *testing.T) {
	bus := startupBus(make([]byte, 0x100))
	copy(bus[0x20:], "MO*.SET\x00")
	c := cpu386.New(bus)
	c.Seg[cpu386.SegDS] = 0x188
	c.SetDescriptor(0x188, cpu386.Descriptor{Limit: 0xff, Writable: true})
	s := NewMOO2StartupDOS(nil)
	c.R[cpu386.EAX], c.R[cpu386.EDX] = 0x4e00, 0x20
	if s.Handle(c, 0x21) {
		t.Fatal("未設定 DTA 竟接受 FindFirst")
	}
	c.R[cpu386.EAX], c.R[cpu386.EDX] = 0x1a00, 0xf0
	s.Handle(c, 0x21)
	c.R[cpu386.EAX], c.R[cpu386.EDX] = 0x4e00, 0x20
	if s.Handle(c, 0x21) {
		t.Fatal("萬用字元竟被接受")
	}
	copy(bus[0x20:], "MOX.SET\x00")
	if s.Handle(c, 0x21) {
		t.Fatal("DTA 越界竟被接受")
	}
	if c.R[cpu386.EAX] != 0x4e00 || c.EFlags&cpu386.CF != 0 {
		t.Fatal("失敗即關閉時改動暫存器")
	}
}

func TestProtectedDOSSetDTAKeepsFullPointerAndMachineState(t *testing.T) {
	bus := startupBus(make([]byte, 0x100))
	before := append([]byte(nil), bus...)
	c := cpu386.New(bus)
	c.Seg[cpu386.SegDS] = 0x188
	c.R[cpu386.EAX], c.R[cpu386.EBX], c.R[cpu386.EDX], c.EFlags =
		0x00381a99, 0x003c3828, 0x003c3828, 0x246
	s := NewMOO2StartupDOS(nil)
	if !s.Handle(c, 0x21) || s.dtaSelector != 0x188 || s.dtaOffset != 0x003c3828 ||
		c.R[cpu386.EAX] != 0x00381a99 || c.R[cpu386.EBX] != 0x003c3828 ||
		c.R[cpu386.EDX] != 0x003c3828 || c.EFlags != 0x246 || !bytes.Equal(bus, before) {
		t.Fatalf("AH=1Ah 指標或狀態：DS=%X offset=%X EAX=%X EDX=%X flags=%X",
			s.dtaSelector, s.dtaOffset, c.R[cpu386.EAX], c.R[cpu386.EDX], c.EFlags)
	}
	c.Seg[cpu386.SegDS] = 0x160
	c.R[cpu386.EDX] = 0x00123456
	if s.dtaSelector != 0x188 || s.dtaOffset != 0x003c3828 {
		t.Fatal("變更呼叫端 DS／EDX 後，既存 DTA 指標被改動")
	}
	c.R[cpu386.EAX] = 0x1a00
	if !s.Handle(c, 0x21) || s.dtaSelector != 0x160 || s.dtaOffset != 0x00123456 {
		t.Fatalf("第二次 AH=1Ah 未替換指標：DS=%X offset=%X", s.dtaSelector, s.dtaOffset)
	}
}

func TestFD2StartupDOSOpenReadOnly(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "MDI.INI"), []byte("driver\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := OpenDirectoryReadOnlyFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { provider.Close() })
	bus := startupBus(make([]byte, 0x100))
	copy(bus[0x20:], []byte("mdi.ini\x00"))
	c := cpu386.New(bus)
	c.Seg[cpu386.SegDS] = 0x160
	c.SetDescriptor(0x160, cpu386.Descriptor{Limit: 0xff, Writable: true})
	s := NewFD2StartupDOS(provider)
	t.Cleanup(func() { s.Close() })
	c.R[cpu386.EAX], c.R[cpu386.EDX], c.EFlags = 0x3d00, 0x20, cpu386.CF
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 5 || c.EFlags&cpu386.CF != 0 || !s.HasHandle(5) {
		t.Fatalf("open EAX=%X flags=%X handle=%t", c.R[cpu386.EAX], c.EFlags, s.HasHandle(5))
	}
	c.R[cpu386.EAX] = 0x3d01
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 5 || c.EFlags&cpu386.CF == 0 {
		t.Fatalf("write mode EAX=%X flags=%X", c.R[cpu386.EAX], c.EFlags)
	}
	copy(bus[0x20:], []byte("../MDI.INI\x00"))
	c.R[cpu386.EAX] = 0x3d00
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 5 || c.EFlags&cpu386.CF == 0 || s.HasHandle(6) {
		t.Fatalf("unsafe path EAX=%X flags=%X nextHandle=%t", c.R[cpu386.EAX], c.EFlags, s.HasHandle(6))
	}
	copy(bus[0x20:], []byte("MISSING.INI\x00"))
	c.R[cpu386.EAX] = 0x3d00
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 2 || c.EFlags&cpu386.CF == 0 || s.HasHandle(6) {
		t.Fatalf("missing path EAX=%X flags=%X nextHandle=%t", c.R[cpu386.EAX], c.EFlags, s.HasHandle(6))
	}
}

func TestFD2StartupDOSDeviceInformation(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "MDI.INI"), []byte("driver\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := OpenDirectoryReadOnlyFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { provider.Close() })
	bus := startupBus(make([]byte, 0x100))
	copy(bus[0x20:], []byte("MDI.INI\x00"))
	c := cpu386.New(bus)
	c.Seg[cpu386.SegDS] = 0x160
	c.SetDescriptor(0x160, cpu386.Descriptor{Limit: 0xff})
	s := NewFD2StartupDOS(provider)
	t.Cleanup(func() { s.Close() })
	c.R[cpu386.EAX], c.R[cpu386.EDX] = 0x3d00, 0x20
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 5 || c.EFlags&cpu386.CF != 0 {
		t.Fatalf("open EAX=%X flags=%X", c.R[cpu386.EAX], c.EFlags)
	}

	c.R[cpu386.EAX], c.R[cpu386.EBX], c.R[cpu386.EDX], c.EFlags = 0x4400, 5, 0xa5a5ffff, cpu386.CF
	if !s.Handle(c, 0x21) || c.R[cpu386.EDX] != 0xa5a50000 || c.EFlags&cpu386.CF != 0 {
		t.Fatalf("device info EAX=%X EDX=%X flags=%X", c.R[cpu386.EAX], c.R[cpu386.EDX], c.EFlags)
	}

	c.R[cpu386.EAX], c.R[cpu386.EBX], c.EFlags = 0x4400, 6, 0
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 6 || c.EFlags&cpu386.CF == 0 {
		t.Fatalf("invalid handle EAX=%X flags=%X", c.R[cpu386.EAX], c.EFlags)
	}

	c.R[cpu386.EAX], c.R[cpu386.EBX], c.EFlags = 0x4401, 5, 0
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 1 || c.EFlags&cpu386.CF == 0 {
		t.Fatalf("unsupported IOCTL EAX=%X flags=%X", c.R[cpu386.EAX], c.EFlags)
	}
}

func TestFD2StartupDOSReadFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "MDI.INI"), []byte("driver\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := OpenDirectoryReadOnlyFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { provider.Close() })
	bus := startupBus(make([]byte, 0x100))
	copy(bus[0x20:], []byte("MDI.INI\x00"))
	c := cpu386.New(bus)
	c.Seg[cpu386.SegDS] = 0x160
	c.SetDescriptor(0x160, cpu386.Descriptor{Limit: 0xff, Writable: true})
	s := NewFD2StartupDOS(provider)
	t.Cleanup(func() { s.Close() })
	c.R[cpu386.EAX], c.R[cpu386.EDX] = 0x3d00, 0x20
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 5 {
		t.Fatalf("open EAX=%X flags=%X", c.R[cpu386.EAX], c.EFlags)
	}

	c.R[cpu386.EAX], c.R[cpu386.EBX], c.R[cpu386.ECX], c.R[cpu386.EDX], c.EFlags = 0x3f00, 5, 4, 0x40, cpu386.CF
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 4 || c.EFlags&cpu386.CF != 0 || string(bus[0x40:0x44]) != "driv" {
		t.Fatalf("read EAX=%X flags=%X data=%q", c.R[cpu386.EAX], c.EFlags, bus[0x40:0x44])
	}
	c.R[cpu386.EAX], c.R[cpu386.ECX], c.R[cpu386.EDX] = 0x3f00, 8, 0x44
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 3 || string(bus[0x44:0x47]) != "er\n" {
		t.Fatalf("short read EAX=%X flags=%X data=%q", c.R[cpu386.EAX], c.EFlags, bus[0x44:0x47])
	}
	c.R[cpu386.EAX], c.R[cpu386.ECX], c.R[cpu386.EDX] = 0x3f00, 8, 0x48
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 0 || c.EFlags&cpu386.CF != 0 {
		t.Fatalf("EOF EAX=%X flags=%X", c.R[cpu386.EAX], c.EFlags)
	}
	c.R[cpu386.EAX], c.R[cpu386.ECX], c.R[cpu386.EDX] = 0x3f00, 0, 0xffffffff
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 0 || c.EFlags&cpu386.CF != 0 {
		t.Fatalf("zero read EAX=%X flags=%X", c.R[cpu386.EAX], c.EFlags)
	}
	c.R[cpu386.EAX], c.R[cpu386.EBX], c.R[cpu386.ECX], c.R[cpu386.EDX], c.EFlags = 0x3f00, 6, 1, 0x40, 0
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 6 || c.EFlags&cpu386.CF == 0 {
		t.Fatalf("invalid handle EAX=%X flags=%X", c.R[cpu386.EAX], c.EFlags)
	}
}

func TestFD2StartupDOSReadRejectsDestinationRange(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "MDI.INI"), []byte("driver\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := OpenDirectoryReadOnlyFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { provider.Close() })
	bus := startupBus(make([]byte, 0x40))
	copy(bus[0x10:], []byte("MDI.INI\x00"))
	c := cpu386.New(bus)
	c.Seg[cpu386.SegDS] = 0x160
	c.SetDescriptor(0x160, cpu386.Descriptor{Limit: 0x3f, Writable: true})
	s := NewFD2StartupDOS(provider)
	t.Cleanup(func() { s.Close() })
	c.R[cpu386.EAX], c.R[cpu386.EDX] = 0x3d00, 0x10
	s.Handle(c, 0x21)
	c.R[cpu386.EAX], c.R[cpu386.EBX], c.R[cpu386.ECX], c.R[cpu386.EDX] = 0x3f00, 5, 4, 0x3e
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 5 || c.EFlags&cpu386.CF == 0 {
		t.Fatalf("range EAX=%X flags=%X", c.R[cpu386.EAX], c.EFlags)
	}
	c.R[cpu386.EAX], c.R[cpu386.EBX], c.R[cpu386.ECX], c.R[cpu386.EDX] = 0x3f00, 5, 4, 0x20
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 4 || string(bus[0x20:0x24]) != "driv" {
		t.Fatalf("rewound read EAX=%X flags=%X data=%q", c.R[cpu386.EAX], c.EFlags, bus[0x20:0x24])
	}
}

func TestFD2StartupDOSSeekFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "MDI.INI"), []byte("driver\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := OpenDirectoryReadOnlyFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { provider.Close() })
	bus := startupBus(make([]byte, 0x100))
	copy(bus[0x20:], []byte("MDI.INI\x00"))
	c := cpu386.New(bus)
	c.Seg[cpu386.SegDS] = 0x160
	c.SetDescriptor(0x160, cpu386.Descriptor{Limit: 0xff})
	s := NewFD2StartupDOS(provider)
	t.Cleanup(func() { s.Close() })
	c.R[cpu386.EAX], c.R[cpu386.EDX] = 0x3d00, 0x20
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 5 {
		t.Fatalf("open EAX=%X flags=%X", c.R[cpu386.EAX], c.EFlags)
	}

	seek := func(mode uint8, offset int32) (uint32, bool) {
		c.R[cpu386.EAX] = 0xa5a50000 | 0x4200 | uint32(mode)
		c.R[cpu386.EBX] = 5
		c.R[cpu386.ECX] = 0xc7c70000 | uint32(uint16(uint32(offset)>>16))
		c.R[cpu386.EDX] = 0xb6b60000 | uint32(uint16(offset))
		c.EFlags |= cpu386.CF
		if !s.Handle(c, 0x21) {
			t.Fatal("seek was not handled")
		}
		return uint32(uint16(c.R[cpu386.EDX]))<<16 | uint32(uint16(c.R[cpu386.EAX])), c.EFlags&cpu386.CF == 0
	}
	if pos, ok := seek(0, 2); !ok || pos != 2 || c.R[cpu386.EAX]>>16 != 0xa5a5 || c.R[cpu386.EDX]>>16 != 0xb6b6 {
		t.Fatalf("start seek pos=%X ok=%t EAX=%X EDX=%X", pos, ok, c.R[cpu386.EAX], c.R[cpu386.EDX])
	}
	if pos, ok := seek(1, -1); !ok || pos != 1 {
		t.Fatalf("current seek pos=%X ok=%t", pos, ok)
	}
	if pos, ok := seek(2, -2); !ok || pos != 5 {
		t.Fatalf("end seek pos=%X ok=%t", pos, ok)
	}
	if _, ok := seek(3, 0); ok || uint16(c.R[cpu386.EAX]) != 1 {
		t.Fatalf("invalid mode EAX=%X flags=%X", c.R[cpu386.EAX], c.EFlags)
	}
	if pos, ok := seek(1, 0); !ok || pos != 5 {
		t.Fatalf("position changed after invalid mode pos=%X ok=%t", pos, ok)
	}
	if _, ok := seek(0, -1); ok || uint16(c.R[cpu386.EAX]) != 1 {
		t.Fatalf("negative position EAX=%X flags=%X", c.R[cpu386.EAX], c.EFlags)
	}
	if pos, ok := seek(1, 0); !ok || pos != 5 {
		t.Fatalf("position not restored pos=%X ok=%t", pos, ok)
	}
	c.R[cpu386.EAX], c.R[cpu386.EBX] = 0x4200, 6
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 6 || c.EFlags&cpu386.CF == 0 {
		t.Fatalf("invalid handle EAX=%X flags=%X", c.R[cpu386.EAX], c.EFlags)
	}
}

type startupBus []byte

func (b startupBus) Read8(addr uint32) (uint8, error)      { return b[addr], nil }
func (b startupBus) Write8(addr uint32, value uint8) error { b[addr] = value; return nil }

func TestFD2StartupDOS(t *testing.T) {
	c := cpu386.New(startupBus(make([]byte, 1)))
	s := &FD2StartupDOS{}
	c.R[cpu386.EAX] = 0x3000
	c.R[cpu386.EBX] = 0x50484152
	if !s.Handle(c, 0x21) || uint16(c.R[cpu386.EAX]) != 0x1606 {
		t.Fatalf("DOS version response EAX=%08X", c.R[cpu386.EAX])
	}
	if c.Seg[cpu386.SegDS] != 0x160 || c.Seg[cpu386.SegES] != 0x28 || c.Seg[cpu386.SegGS] != 0x20 || c.Seg[cpu386.SegSS] != 0x160 {
		t.Fatalf("startup selectors DS=%X ES=%X GS=%X SS=%X", c.Seg[cpu386.SegDS], c.Seg[cpu386.SegES], c.Seg[cpu386.SegGS], c.Seg[cpu386.SegSS])
	}
	if value, ok := c.SegmentRead16(0x28, 0x2c); !ok || value != 0x30 {
		t.Fatalf("ES environment cell value=%X ok=%v", value, ok)
	}
	if _, ok := c.SegmentRead16(0x28, 0x2e); ok {
		t.Fatal("unknown ES environment cell was accepted")
	}
	if !c.SegmentLoadOK(0x28, cpu386.SegDS) || !c.SegmentLoadOK(0x28, cpu386.SegES) || c.SegmentLoadOK(0x28, cpu386.SegSS) {
		t.Fatal("PSP selector load destinations mismatch")
	}
	if !c.SegmentLoadOK(0x30, cpu386.SegDS) || !c.SegmentLoadOK(0x30, cpu386.SegES) || !c.SegmentLoadOK(0x30, cpu386.SegFS) || c.SegmentLoadOK(0x30, cpu386.SegSS) {
		t.Fatal("environment selector load destinations mismatch")
	}
	if c.SegmentLoadOK(0x20, cpu386.SegGS) {
		t.Fatal("FD2 profile unexpectedly accepts MOO2 GS selector")
	}
	for offset, want := range minimalFD2Environment {
		if got, ok := c.SegmentRead8(0x30, uint32(offset)); !ok || got != want {
			t.Fatalf("environment[%d]=%X ok=%v want %X", offset, got, ok, want)
		}
	}
	if _, ok := c.SegmentRead8(0x30, uint32(len(minimalFD2Environment))); ok {
		t.Fatal("environment out-of-range read accepted")
	}
	c.R[cpu386.EAX] = 0xff00
	c.R[cpu386.EDX] = 0x78
	if !s.Handle(c, 0x21) || c.R[cpu386.EAX] != 0x4734ffff || c.Seg[cpu386.SegGS] != 0x20 {
		t.Fatalf("DOS/4G response EAX=%08X GS=%04X", c.R[cpu386.EAX], c.Seg[cpu386.SegGS])
	}
	c.R[cpu386.EAX] = 0x2c00
	for want := uint32(0); want < 3; want++ {
		if !s.Handle(c, 0x21) || uint8(c.R[cpu386.EDX]>>8) != uint8(want) {
			t.Fatalf("DOS time second=%d EDX=%08X", want, c.R[cpu386.EDX])
		}
		c.R[cpu386.EAX] = 0x2c00
	}
	c.R[cpu386.EAX] = 0x2900
	if s.Handle(c, 0x21) || s.Calls() != 5 {
		t.Fatal("unknown post-startup call was accepted")
	}
}

func TestMOO2ProvisionalStartupEnvironment(t *testing.T) {
	c := cpu386.New(startupBus(make([]byte, 1)))
	s := NewMOO2StartupDOS(nil)
	c.R[cpu386.EAX], c.R[cpu386.EBX] = 0x3000, 0x50484152
	c.EFlags = 0x246
	if !s.Handle(c, 0x21) || s.Calls() != 1 {
		t.Fatal("MOO2 DOS 版本查詢未通過暫定服務入口")
	}
	if c.R[cpu386.EAX] != 5 || c.R[cpu386.EBX] != 0x5048ff00 || c.Seg[cpu386.SegDS] != 0x188 || c.Seg[cpu386.SegSS] != 0x188 || c.Seg[cpu386.SegES] != 0x28 || c.Seg[cpu386.SegGS] != 0x20 || c.EFlags != 0x246 {
		t.Fatalf("MOO2 固定輔助基準返回不符：EAX=%X EBX=%X DS=%X ES=%X GS=%X SS=%X", c.R[cpu386.EAX], c.R[cpu386.EBX], c.Seg[cpu386.SegDS], c.Seg[cpu386.SegES], c.Seg[cpu386.SegGS], c.Seg[cpu386.SegSS])
	}
	if !c.SegmentLoadOK(0x20, cpu386.SegGS) || c.SegmentLoadOK(0x20, cpu386.SegDS) || c.SegmentLoadOK(0x20, cpu386.SegFS) {
		t.Fatal("MOO2 GS=0020h 載入許可超出原版觀測範圍")
	}
	for offset, want := range minimalMOO2Environment {
		got, ok := c.SegmentRead8(0x30, uint32(offset))
		if !ok || got != want {
			t.Fatalf("MOO2 environment[%d]=%X ok=%v want=%X", offset, got, ok, want)
		}
	}
	if _, ok := c.SegmentRead8(0x30, uint32(len(minimalMOO2Environment))); ok {
		t.Fatal("MOO2 environment 超出界限仍可讀")
	}
	c.R[cpu386.EAX], c.R[cpu386.EDX] = 0xff00, 0x78
	c.EFlags |= cpu386.CF
	if !s.Handle(c, 0x21) || s.Calls() != 2 || c.R[cpu386.EAX] != 0x4734ffff || c.Seg[cpu386.SegDS] != 0x188 || c.Seg[cpu386.SegGS] != 0x20 || c.EFlags&cpu386.CF != 0 {
		t.Fatal("MOO2 DOS/4G 私有查詢未通過暫定服務入口")
	}
}

func TestFD2StartupDOSRejectsWrongOrder(t *testing.T) {
	c := cpu386.New(startupBus(make([]byte, 1)))
	s := &FD2StartupDOS{}
	c.R[cpu386.EAX] = 0xff00
	c.R[cpu386.EDX] = 0x78
	if s.Handle(c, 0x21) || s.Calls() != 0 {
		t.Fatal("out-of-order DOS/4G call was accepted")
	}
}

func TestFD2StartupDPMIGetRealModeInterruptVector(t *testing.T) {
	c := cpu386.New(startupBus(make([]byte, 1)))
	s := &FD2StartupDOS{}
	s.SetRealModeVector(8, 0xf000, 0x1234)
	c.R[cpu386.EAX] = 0xaaaa0200
	c.R[cpu386.EBX] = 0xbbbb0008
	c.R[cpu386.ECX] = 0xcccc0000
	c.R[cpu386.EDX] = 0xdddd0000
	c.EFlags |= cpu386.CF
	if !s.Handle(c, 0x31) || c.R[cpu386.ECX] != 0xccccf000 || c.R[cpu386.EDX] != 0xdddd1234 || c.EFlags&cpu386.CF != 0 || s.Calls() != 0 {
		t.Fatalf("DPMI 0200 ECX=%X EDX=%X flags=%X calls=%d", c.R[cpu386.ECX], c.R[cpu386.EDX], c.EFlags, s.Calls())
	}

	// AX=0201h（設實模式向量）現在由通用的 DPMI 主機做（`dpmi.go`）；
	// 沒實作的那些仍然要被拒絕，改用 AX=0301h（尚未支援的實模式遠呼叫）驗。
	c.R[cpu386.EAX] = 0x0301
	if s.Handle(c, 0x31) {
		t.Fatal("unknown DPMI function was accepted")
	}
}

func TestFD2StartupDOSInterruptVectors(t *testing.T) {
	c := cpu386.New(startupBus(make([]byte, 1)))
	s := &FD2StartupDOS{}
	s.SetRealModeVector(8, 0xf000, 0x1234)
	c.Seg[cpu386.SegDS] = 0x160
	c.R[cpu386.EAX] = 0xaaaa2508
	c.R[cpu386.EDX] = 0x12345678
	c.EFlags |= cpu386.CF
	if !s.Handle(c, 0x21) || c.EFlags&cpu386.CF != 0 || s.Calls() != 0 {
		t.Fatalf("DOS 2508 flags=%X calls=%d", c.EFlags, s.Calls())
	}

	c.Seg[cpu386.SegES] = 0xffff
	c.R[cpu386.EAX] = 0xbbbb3508
	c.R[cpu386.EBX] = 0xdeadbeef
	c.EFlags |= cpu386.CF
	if !s.Handle(c, 0x21) || c.Seg[cpu386.SegES] != 0x160 || c.R[cpu386.EBX] != 0x12345678 || c.EFlags&cpu386.CF != 0 || s.Calls() != 0 {
		t.Fatalf("DOS 3508 ES=%X EBX=%X flags=%X calls=%d", c.Seg[cpu386.SegES], c.R[cpu386.EBX], c.EFlags, s.Calls())
	}

	c.R[cpu386.EAX] = 0x0200
	c.R[cpu386.EBX] = 8
	if !s.Handle(c, 0x31) || uint16(c.R[cpu386.ECX]) != 0xf000 || uint16(c.R[cpu386.EDX]) != 0x1234 {
		t.Fatalf("DPMI vector was contaminated ECX=%X EDX=%X", c.R[cpu386.ECX], c.R[cpu386.EDX])
	}
}

// 規格414：依原拒絕的高位EDX核對，只讀查詢不得配置handle或改來源。
func TestMOO2ProtectedFileAttributesReadOnly(t *testing.T) {
	root := t.TempDir()
	input := []byte("original-data")
	name := filepath.Join(root, "SAVE10.GAM")
	if err := os.WriteFile(name, input, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "DIRECTORY"), 0700); err != nil {
		t.Fatal(err)
	}
	provider, err := OpenDirectoryReadOnlyFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Close()
	for _, tc := range []struct {
		name, path                            string
		err                                   uint32
		invalid, overflow, noNUL, nilProvider bool
	}{
		{name: "原高位指標與普通檔", path: "save10.gam"},
		{name: "缺檔", path: "MISSING.GAM", err: 2},
		{name: "目錄拒絕", path: "DIRECTORY", err: 5},
		{name: "路徑拒絕", path: "../SAVE10.GAM", err: 5},
		{name: "無提供者", path: "SAVE10.GAM", err: 5, nilProvider: true},
		{name: "descriptor越界", path: "SAVE10.GAM", err: 3, invalid: true},
		{name: "32位指標溢位", path: "SAVE10.GAM", err: 3, overflow: true},
		{name: "未終止", err: 3, noNUL: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mem := make([]byte, 0x2bd904+280)
			copy(mem[0x2bd904:], append([]byte(tc.path), 0))
			if tc.noNUL {
				for i := 0; i < 260; i++ {
					mem[0x2bd904+i] = 'A'
				}
			}
			c := cpu386.New(startupBus(mem))
			c.Seg[cpu386.SegDS] = 0x188
			c.SetDescriptor(0x188, cpu386.Descriptor{Base: 0, Limit: uint32(len(mem) - 1), Writable: true})
			c.R = [8]uint32{cpu386.EAX: 0xabcd4300, cpu386.ECX: 0x98761234, cpu386.EDX: 0x1212eeee, cpu386.EBX: 0x1234abcd, cpu386.ESP: 0x88888888, cpu386.EBP: 0x99999999, cpu386.ESI: 0x13572468, cpu386.EDI: 0x24681357}
			c.R[cpu386.EDX] = 0x2bd904
			if tc.invalid {
				c.SetDescriptor(0x188, cpu386.Descriptor{Base: 0, Limit: 0x2bd903, Writable: true})
			}
			if tc.overflow {
				c.R[cpu386.EDX] = 0xffffffff
			}
			c.EIP, c.EFlags = 0x219e75, 0x247
			oldR, oldSeg, oldIP, oldFlags := c.R, c.Seg, c.EIP, c.EFlags
			oldMem := append([]byte(nil), mem...)
			s := NewMOO2StartupDOS(provider)
			if tc.nilProvider {
				s = NewMOO2StartupDOS(nil)
			}
			oldTable := s.table
			if !s.Handle(c, 0x21) {
				t.Fatal("已支援的AH43/AL0未路由")
			}
			wantR := oldR
			wantFlags := oldFlags
			if tc.err == 0 {
				wantR[cpu386.ECX] = 0x98760020
				wantFlags &^= cpu386.CF
			} else {
				wantR[cpu386.EAX] = 0xabcd0000 | tc.err
				wantFlags |= cpu386.CF
			}
			if c.R != wantR || c.Seg != oldSeg || c.EIP != oldIP || c.EFlags != wantFlags || !bytes.Equal(mem, oldMem) || s.table != oldTable || s.HasHandle(5) || s.Calls() != 0 {
				t.Fatalf("唯讀屬性或非輸出狀態錯誤：R=%X flags=%X", c.R, c.EFlags)
			}
			got, e := os.ReadFile(name)
			if e != nil || !bytes.Equal(got, input) {
				t.Fatal("查詢改變來源檔")
			}
		})
	}
}
func TestMOO2ProtectedFileAttributesUnsupportedPreservesState(t *testing.T) {
	for _, tc := range []struct {
		name      string
		ax        uint32
		fd2       bool
		interrupt uint8
	}{
		{"AL1保持拒絕", 0x4301, false, 0x21}, {"其他AL", 0x4302, false, 0x21},
		{"FD2保持拒絕", 0x4300, true, 0x21}, {"非DOS中斷", 0x4300, false, 0x22},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mem := make([]byte, 16)
			c := cpu386.New(startupBus(mem))
			c.R[cpu386.EAX] = tc.ax
			c.R[cpu386.EDX] = 0xffffffff
			c.EFlags = 0x247
			s := NewMOO2StartupDOS(nil).FD2StartupDOS
			if tc.fd2 {
				s = NewFD2StartupDOS(nil)
			}
			r, seg, ip, flags := c.R, c.Seg, c.EIP, c.EFlags
			oldTable := s.table
			if s.Handle(c, tc.interrupt) || c.R != r || c.Seg != seg || c.EIP != ip || c.EFlags != flags || s.Calls() != 0 || s.table != oldTable || s.HasHandle(5) {
				t.Fatal("未支援子功能改狀態或放行")
			}
		})
	}
}

type create423File struct {
	*bytes.Reader
	closed bool
}

func (f *create423File) Close() error { f.closed = true; return nil }

type create423Provider struct {
	file  io.ReadSeekCloser
	err   error
	calls int
}

func (p *create423Provider) OpenRead(string) (io.ReadSeekCloser, error) { return nil, os.ErrNotExist }
func (p *create423Provider) CreateFile(string) (io.ReadSeekCloser, error) {
	p.calls++
	return p.file, p.err
}

func create423CPU() (*cpu386.CPU, []byte) {
	mem := make([]byte, 0x2bd894+280)
	copy(mem[0x2bd894:], []byte("SAVE1.GAM\x00"))
	c := cpu386.New(startupBus(mem))
	c.Seg = [6]uint16{cpu386.SegCS: 0x180, cpu386.SegDS: 0x188, cpu386.SegES: 0x188, cpu386.SegGS: 0x20, cpu386.SegSS: 0x188}
	c.SetDescriptor(0x188, cpu386.Descriptor{Limit: uint32(len(mem) - 1), Writable: true})
	c.R = [8]uint32{cpu386.EAX: 0xabcd3c80, cpu386.ECX: 0x98760000, cpu386.EDX: 0x2bd894, cpu386.EBX: 0x12340042, cpu386.ESP: 0x2bd458, cpu386.EBP: 0x2bd88a, cpu386.ESI: 0x24681357, cpu386.EDI: 0xffffffff}
	c.EIP, c.EFlags = 0x237107, 0x247
	return c, mem
}
func assertCreate423State(t *testing.T, c *cpu386.CPU, oldR [8]uint32, oldSeg [6]uint16, oldIP, oldFlags uint32, mem, oldMem []byte, result uint16, success bool) {
	t.Helper()
	oldR[cpu386.EAX] = oldR[cpu386.EAX]&0xffff0000 | uint32(result)
	if success {
		oldFlags &^= cpu386.CF
	} else {
		oldFlags |= cpu386.CF
	}
	if c.R != oldR || c.Seg != oldSeg || c.EIP != oldIP || c.EFlags != oldFlags || !bytes.Equal(mem, oldMem) {
		t.Fatalf("非輸出狀態變更：R=%X flags=%X", c.R, c.EFlags)
	}
}

func TestMOO2ProtectedCreateFileAndWriteClose(t *testing.T) {
	base, state := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "SAVE1.GAM"), []byte("immutable"), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := OpenDirectoryOverlayFiles(base, state)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	s := NewMOO2StartupDOS(p)
	defer s.Close()
	for _, al := range []uint32{0x80, 0, 0xff} {
		c, mem := create423CPU()
		c.R[cpu386.EAX] = 0xabcd3c00 | al
		oldR, oldSeg, oldIP, oldFlags := c.R, c.Seg, c.EIP, c.EFlags
		oldMem := append([]byte(nil), mem...)
		if !s.Handle(c, 0x21) {
			t.Fatal("原AH3C未接通")
		}
		handle := uint16(c.R[cpu386.EAX])
		assertCreate423State(t, c, oldR, oldSeg, oldIP, oldFlags, mem, oldMem, handle, true)
		if handle != 5 || !s.HasHandle(handle) || s.Calls() != 0 {
			t.Fatal("handle重用或startup計數錯誤")
		}
		copy(mem[64:], []byte("saved"))
		c.R[cpu386.EAX], c.R[cpu386.EBX], c.R[cpu386.ECX], c.R[cpu386.EDX] = 0x4000, uint32(handle), 5, 64
		if !s.Handle(c, 0x21) || c.EFlags&cpu386.CF != 0 || c.R[cpu386.EAX] != 5 {
			t.Fatal("新handle寫入失敗")
		}
		c.R[cpu386.EAX] = 0x3e00
		if !s.Handle(c, 0x21) || c.EFlags&cpu386.CF != 0 || s.HasHandle(handle) {
			t.Fatal("原close未接入")
		}
		if b, err := os.ReadFile(filepath.Join(state, "SAVE1.GAM")); err != nil || string(b) != "saved" {
			t.Fatal("實際保存內容錯誤", b, err)
		}
		if b, err := os.ReadFile(filepath.Join(base, "SAVE1.GAM")); err != nil || string(b) != "immutable" {
			t.Fatal("base內容變更", b, err)
		}
	}
}

func TestMOO2ProtectedCreateFileFailures(t *testing.T) {
	for _, name := range []string{"無能力", "唯讀provider", "provider錯誤", "nil檔案", "錯誤附檔案", "越界", "溢位", "260未終止", "非法路徑", "裝置名稱", "handle滿"} {
		t.Run(name, func(t *testing.T) {
			c, mem := create423CPU()
			provider := &create423Provider{file: &create423File{Reader: bytes.NewReader(nil)}}
			s := NewMOO2StartupDOS(provider)
			defer s.Close()
			want := uint16(5)
			wantCalls := 0
			switch name {
			case "無能力":
				s = NewMOO2StartupDOS(nil)
			case "唯讀provider":
				p, err := OpenDirectoryReadOnlyFiles(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				defer p.Close()
				s = NewMOO2StartupDOS(p)
			case "provider錯誤":
				provider.file = nil
				provider.err = errors.New("denied")
				wantCalls = 1
			case "nil檔案":
				provider.file = nil
				wantCalls = 1
			case "錯誤附檔案":
				provider.err = errors.New("denied")
				wantCalls = 1
			case "越界":
				c.SetDescriptor(0x188, cpu386.Descriptor{Limit: 0x2bd893, Writable: true})
				want = 3
			case "溢位":
				c.R[cpu386.EDX] = 0xffffffff
				want = 3
			case "260未終止":
				for i := 0; i < 260; i++ {
					mem[0x2bd894+i] = 'A'
				}
				want = 3
			case "非法路徑":
				copy(mem[0x2bd894:], []byte("../SAVE1.GAM\x00"))
			case "裝置名稱":
				copy(mem[0x2bd894:], []byte("AUX.GAM\x00"))
			case "handle滿":
				want = 4
				f := &create423File{Reader: bytes.NewReader(nil)}
				// 使用不重用的表填入，避免fixture為每個handle做線性搜尋。
				s.table = dosfile.NewTable()
				for h := uint32(dosfile.FirstHandle); h < 0xffff; h++ {
					if handle, code := s.table.Add(f, "occupied"); code != 0 || uint32(handle) != h {
						t.Fatal("滿表fixture", handle, code)
					}
				}
			}
			oldR, oldSeg, oldIP, oldFlags := c.R, c.Seg, c.EIP, c.EFlags
			oldMem := append([]byte(nil), mem...)
			if !s.Handle(c, 0x21) {
				t.Fatal("支援的AH3C未回錯誤")
			}
			assertCreate423State(t, c, oldR, oldSeg, oldIP, oldFlags, mem, oldMem, want, false)
			if provider.calls != wantCalls || name != "handle滿" && s.HasHandle(5) {
				t.Fatal("失敗呼叫provider或配置handle", provider.calls)
			}
			if name == "錯誤附檔案" && !provider.file.(*create423File).closed {
				t.Fatal("provider錯誤漏關檔")
			}
		})
	}
}
func TestMOO2ProtectedCreateFileUnsupportedKeepsState(t *testing.T) {
	for _, name := range []string{"FD2", "非零屬性", "其他interrupt", "實模式"} {
		t.Run(name, func(t *testing.T) {
			c, mem := create423CPU()
			p := &create423Provider{file: &create423File{Reader: bytes.NewReader(nil)}}
			s := NewMOO2StartupDOS(p).FD2StartupDOS
			if name == "FD2" {
				s = NewFD2StartupDOS(p)
			}
			if name == "非零屬性" {
				c.R[cpu386.ECX] |= 1
			}
			oldR, oldSeg, oldIP, oldFlags := c.R, c.Seg, c.EIP, c.EFlags
			oldMem := append([]byte(nil), mem...)
			n := uint8(0x21)
			if name == "其他interrupt" {
				n = 0x22
			}
			if name == "實模式" {
				s.DPMI.m = &LEMachine{Mem: make([]byte, 1024)}
				r := &cpu.CPU{}
				r.R[cpu.AX] = 0x3c80
				old := r.R
				if s.HandleRealMode(r, 0x21) || r.R != old {
					t.Fatal("實模式不當接通")
				}
			} else if s.Handle(c, n) {
				t.Fatal("不支援契約被接通")
			}
			if c.R != oldR || c.Seg != oldSeg || c.EIP != oldIP || c.EFlags != oldFlags || !bytes.Equal(mem, oldMem) || p.calls != 0 || s.HasHandle(5) {
				t.Fatal("拒絕有副作用")
			}
			s.Close()
		})
	}
}
