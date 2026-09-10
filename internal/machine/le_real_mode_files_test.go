package machine

import (
	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/cpu386"
	"os"
	"path/filepath"
	"testing"
)

func TestRealModeFilesShareProtectedModeHandles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "X.DAT"), []byte{1, 2, 3}, 0600); err != nil {
		t.Fatal(err)
	}
	files, err := OpenDirectoryReadOnlyFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	defer files.Close()
	s := NewFD2StartupDOS(files)
	defer s.Close()
	m, _ := newDPMITest(t)
	s.AttachMachine(m)
	copy(m.Mem[0x210:], []byte("X.DAT\x00"))
	r := cpu.New(&dpmiRealBus{m: m})
	r.Model = cpu.Model80386
	r.Seg[cpu.DS] = 0x20
	r.R[cpu.DX] = 0x10
	r.R[cpu.AX] = 0x3d00
	r.SetFlags(cpu.IF | cpu.CF)
	if !s.HandleRealMode(r, 0x21) || r.Flags&cpu.CF != 0 {
		t.Fatal("實模式open失敗")
	}
	handle := r.R[cpu.AX]
	r.R[cpu.AX] = 0x3f00
	r.R[cpu.BX] = handle
	r.R[cpu.CX] = 3
	r.R[cpu.DX] = 0x40
	if !s.HandleRealMode(r, 0x21) || r.R[cpu.AX] != 3 || m.Mem[0x240] != 1 || m.Mem[0x242] != 3 || r.Flags&cpu.IF == 0 {
		t.Fatal("DS:DX讀取錯誤")
	}
	c := m.CPU
	c.R[cpu386.EAX] = 0x3e00
	c.R[cpu386.EBX] = uint32(handle)
	if !s.Handle(c, 0x21) || c.EFlags&cpu386.CF != 0 || s.HasHandle(handle) {
		t.Fatal("兩路handle未共用")
	}
	r.R[cpu.AX] = 0x2500
	if s.HandleRealMode(r, 0x21) {
		t.Fatal("未知功能放行")
	}
}
