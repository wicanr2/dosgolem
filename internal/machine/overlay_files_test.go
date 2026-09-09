package machine

import (
	"github.com/wicanr2/dosgolem/internal/cpu386"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestOverlayWritesPreserveSource(t *testing.T) {
	base, state := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(base, "FD2.TMP"), []byte("abcdef"), 0600)
	p, e := OpenDirectoryOverlayFiles(base, state)
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	f, e := p.OpenWrite("fd2.tmp", false)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.Read(make([]byte, 1)); e == nil {
		t.Fatal("唯寫可讀")
	}
	if _, e = f.(io.Writer).Write([]byte("XY")); e != nil {
		t.Fatal(e)
	}
	f.Close()
	f, e = p.OpenRead("FD2.TMP")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := io.ReadAll(f)
	if string(b) != "XYcdef" {
		t.Fatal("覆蓋讀取")
	}
	if _, e = f.(io.Writer).Write([]byte("Z")); e == nil {
		t.Fatal("唯讀可寫")
	}
	f.Close()
	b, _ = os.ReadFile(filepath.Join(base, "FD2.TMP"))
	if string(b) != "abcdef" {
		t.Fatal("原檔污染")
	}
	for _, name := range []string{"../FD2.TMP", "C:FD2.TMP", "a/b", "a\\b"} {
		if _, e = p.OpenWrite(name, false); e == nil {
			t.Fatal("路徑未拒絕")
		}
	}
	os.Symlink(filepath.Join(base, "FD2.TMP"), filepath.Join(state, "LINK.TMP"))
	if _, e = p.OpenWrite("LINK.TMP", false); e == nil {
		t.Fatal("symlink")
	}
	if q, e := OpenDirectoryOverlayFiles(base, base); e == nil {
		q.Close()
		t.Fatal("同根")
	}
}
func TestDOSOverlayOpenWriteSeekTruncate(t *testing.T) {
	base, state := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(base, "FD2.TMP"), []byte("abcdef"), 0600)
	p, e := OpenDirectoryOverlayFiles(base, state)
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	s := NewFD2StartupDOS(p)
	defer s.Close()
	m := &LEMachine{Mem: make([]byte, 512)}
	c := cpu386.New(m)
	c.Seg[cpu386.SegDS] = 0x160
	c.SetDescriptor(0x160, cpu386.Descriptor{Limit: 511, Writable: true})
	copy(m.Mem[32:], []byte("FD2.TMP\x00"))
	c.R[cpu386.EAX] = 0x3d02
	c.R[cpu386.EDX] = 32
	if !s.Handle(c, 0x21) || c.EFlags&cpu386.CF != 0 {
		t.Fatal("開讀寫")
	}
	h := c.R[cpu386.EAX] & 65535
	c.R[cpu386.EBX] = h
	c.R[cpu386.EAX] = 0x4200
	c.R[cpu386.ECX] = 0
	c.R[cpu386.EDX] = 2
	s.Handle(c, 0x21)
	copy(m.Mem[64:], []byte("XY"))
	c.R[cpu386.EAX] = 0x4000
	c.R[cpu386.ECX] = 2
	c.R[cpu386.EDX] = 64
	s.Handle(c, 0x21)
	if c.EFlags&cpu386.CF != 0 || c.R[cpu386.EAX]&65535 != 2 {
		t.Fatal("寫入")
	}
	c.R[cpu386.EAX] = 0x4000
	c.R[cpu386.ECX] = 0
	s.Handle(c, 0x21)
	if c.EFlags&cpu386.CF != 0 {
		t.Fatal("截斷")
	}
	s.Close()
	b, _ := os.ReadFile(filepath.Join(state, "FD2.TMP"))
	if string(b) != "abXY" {
		t.Fatal("寫入與截斷結果")
	}
	b, _ = os.ReadFile(filepath.Join(base, "FD2.TMP"))
	if string(b) != "abcdef" {
		t.Fatal("原檔污染")
	}
}

func TestDOSOverlayFullWidthWriteAndPreflight(t *testing.T) {
	base, state := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(base, "FD2.TMP"), []byte("original"), 0600)
	p, e := OpenDirectoryOverlayFiles(base, state)
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	s := NewFD2StartupDOS(p)
	defer s.Close()
	f, e := p.OpenWrite("FD2.TMP", true)
	if e != nil {
		t.Fatal(e)
	}
	h, code := s.handles().Add(f, "FD2.TMP")
	if code != 0 {
		t.Fatal(code)
	}
	m := &LEMachine{Mem: make([]byte, 207360+64)}
	c := cpu386.New(m)
	c.Seg[cpu386.SegDS] = 0x160
	c.SetDescriptor(0x160, cpu386.Descriptor{Limit: uint32(len(m.Mem) - 1), Writable: true})
	for i := range m.Mem {
		m.Mem[i] = byte(i*37 + 11)
	}
	for _, n := range []uint32{65536, 207360} {
		f.Seek(0, io.SeekStart)
		c.R[cpu386.EBX] = uint32(h)
		c.R[cpu386.EDX] = 64
		c.R[cpu386.ECX] = n
		c.R[cpu386.EAX] = 0xa5004000
		s.Handle(c, 0x21)
		if c.EFlags&cpu386.CF != 0 || c.R[cpu386.EAX] != n {
			t.Fatalf("完整寫入 %d 回 %d", n, c.R[cpu386.EAX])
		}
		f.Seek(0, io.SeekStart)
		got := make([]byte, n)
		if _, e := io.ReadFull(f, got); e != nil {
			t.Fatal(e)
		}
		for i, v := range got {
			if v != m.Mem[64+i] {
				t.Fatalf("資料差異 %d", i)
			}
		}
	}
	f.Seek(0, io.SeekStart)
	c.R[cpu386.ECX] = 207361
	c.R[cpu386.EAX] = 0x4000
	s.Handle(c, 0x21)
	if c.EFlags&cpu386.CF == 0 {
		t.Fatal("尾端越界仍寫入")
	}
	pos, _ := f.Seek(0, io.SeekCurrent)
	if pos != 0 {
		t.Fatal("拒絕前已更動檔案位置")
	}
	b, _ := os.ReadFile(filepath.Join(base, "FD2.TMP"))
	if string(b) != "original" {
		t.Fatal("原檔污染")
	}
}
