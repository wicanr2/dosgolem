package machine

import (
	"bytes"
	"github.com/wicanr2/dosgolem/internal/cpu386"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestFD2ProtectedReadUsesFullECXAndEAX(t *testing.T) {
	for _, n := range []int{96768, 143872} {
		root := t.TempDir()
		data := make([]byte, n+3)
		for i := range data {
			data[i] = byte(i*17 + 3)
		}
		if err := os.WriteFile(filepath.Join(root, "FDSHAP.DAT"), data, 0600); err != nil {
			t.Fatal(err)
		}
		provider, err := OpenDirectoryReadOnlyFiles(root)
		if err != nil {
			t.Fatal(err)
		}
		defer provider.Close()
		bus := startupBus(make([]byte, n+256))
		copy(bus[16:], []byte("FDSHAP.DAT\x00"))
		c := cpu386.New(bus)
		c.Seg[cpu386.SegDS] = 0x160
		c.SetDescriptor(0x160, cpu386.Descriptor{Limit: uint32(len(bus) - 1), Writable: true})
		s := NewFD2StartupDOS(provider)
		defer s.Close()
		c.R[cpu386.EAX] = 0x3d00
		c.R[cpu386.EDX] = 16
		s.Handle(c, 0x21)
		handle := c.R[cpu386.EAX]
		c.R[cpu386.EAX] = 0xabcd3f00
		c.R[cpu386.EBX] = handle
		c.R[cpu386.ECX] = uint32(n)
		c.R[cpu386.EDX] = 128
		if !s.Handle(c, 0x21) || c.EFlags&cpu386.CF != 0 || c.R[cpu386.EAX] != uint32(n) || !bytes.Equal(bus[128:128+n], data[:n]) {
			t.Fatalf("完整讀取%d卻返回%d flags=%X", n, c.R[cpu386.EAX], c.EFlags)
		}
		c.R[cpu386.EAX] = 0x3f00
		c.R[cpu386.ECX] = 16
		c.R[cpu386.EDX] = 128
		s.Handle(c, 0x21)
		if c.R[cpu386.EAX] != 3 || !bytes.Equal(bus[128:131], data[n:]) {
			t.Fatal("短讀未回實際長度")
		}
		c.R[cpu386.EAX] = 0x3f00
		s.Handle(c, 0x21)
		if c.R[cpu386.EAX] != 0 || c.EFlags&cpu386.CF != 0 {
			t.Fatal("EOF錯誤")
		}
		file, _ := s.handles().Get(uint16(handle))
		file.Seek(0, io.SeekStart)
		c.R[cpu386.EAX] = 0x3f00
		c.R[cpu386.ECX] = 0x10001
		c.R[cpu386.EDX] = uint32(len(bus) - 1)
		s.Handle(c, 0x21)
		pos, _ := file.Seek(0, io.SeekCurrent)
		if c.EFlags&cpu386.CF == 0 || pos != 0 {
			t.Fatal("越界不應消耗檔案位置")
		}
	}
}
