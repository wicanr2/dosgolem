package machine

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wicanr2/dosgolem/internal/cpu386"
)

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
	c.R[cpu386.EAX] = 4
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
	c.R[cpu386.EAX], c.R[cpu386.EBX] = 0x1a, 0x11110001
	beforeR = c.R
	if s.Handle(c, 0x33) || c.R != beforeR || s.mouseSensitivityX != 0 {
		t.Fatal("非零敏感度輸入須拒絕且保留狀態")
	}
	fd2 := NewFD2StartupDOS(nil)
	c.R[cpu386.EBX] = 0
	beforeR = c.R
	if fd2.Handle(c, 0x33) || c.R != beforeR {
		t.Fatal("一般 FD2 啟動設定不得接受 MOO2 專用敏感度設定")
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
