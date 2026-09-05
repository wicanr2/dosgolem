package machine

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/binary"
	"os"
	"testing"
)

func leFixture() []byte {
	b := make([]byte, 0x200)
	b[0], b[1] = 'M', 'Z'
	binary.LittleEndian.PutUint32(b[0x3c:], 0x80)
	h := b[0x80:]
	h[0], h[1] = 'L', 'E'
	binary.LittleEndian.PutUint16(h[8:], 2)
	binary.LittleEndian.PutUint32(h[0x18:], 1)
	binary.LittleEndian.PutUint32(h[0x1c:], 0x1234)
	binary.LittleEndian.PutUint32(h[0x28:], 0x1000)
	binary.LittleEndian.PutUint32(h[0x40:], 0xb0)
	binary.LittleEndian.PutUint32(h[0x44:], 1)
	o := h[0xb0:]
	for i, v := range []uint32{0x2000, 0x10000, 0x2045, 1, 2, 0} {
		binary.LittleEndian.PutUint32(o[i*4:], v)
	}
	return b
}

func TestInspectLE(t *testing.T) {
	h, err := InspectLE(leFixture())
	if err != nil {
		t.Fatal(err)
	}
	if h.Offset != 0x80 || h.CPUType != 2 || h.PageSize != 0x1000 || len(h.Objects) != 1 {
		t.Fatalf("unexpected header: %+v", h)
	}
	if h.Objects[0].VirtualSize != 0x2000 || h.Objects[0].RelocationBase != 0x10000 {
		t.Fatalf("unexpected object: %+v", h.Objects[0])
	}
}

func TestInspectLERejectsInvalidInputs(t *testing.T) {
	for name, b := range map[string][]byte{
		"not-mz":          make([]byte, 0x100),
		"truncated":       append([]byte{'M', 'Z'}, make([]byte, 10)...),
		"not-le":          func() []byte { b := leFixture(); b[0x80] = 'L'; b[0x81] = 'X'; return b }(),
		"object-overflow": func() []byte { b := leFixture(); binary.LittleEndian.PutUint32(b[0x80+0x44:], 0xffffffff); return b }(),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := InspectLE(b); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestInspectFD2WhenProvided(t *testing.T) {
	path := os.Getenv("DOSGOLEM_FD2_EXE")
	if path == "" {
		t.Skip("DOSGOLEM_FD2_EXE 未設定")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 357074 || md5.Sum(b) != [16]byte{0xb9, 0x7c, 0xaf, 0x22, 0x39, 0xa2, 0x7a, 0x89, 0x60, 0x69, 0xd0, 0x35, 0x49, 0xd9, 0x6e, 0x1e} || sha256.Sum256(b) != [32]byte{0x22, 0x2b, 0x7d, 0x06, 0x7a, 0xd4, 0x45, 0x0e, 0xb9, 0xc5, 0xf6, 0xe6, 0xbc, 0xe1, 0x79, 0x7d, 0x54, 0xbb, 0x05, 0x04, 0x17, 0xba, 0x39, 0xce, 0xd6, 0x06, 0x7f, 0x80, 0x39, 0xf2, 0x8c, 0x4f} {
		t.Fatal("FD2.EXE 雜湊或大小不符")
	}
	h, err := InspectLE(b)
	if err != nil {
		t.Fatal(err)
	}
	if h.Offset != 0x28b8 || h.ObjectCount != 3 || h.PageSize != 0x1000 || h.EIPObject != 1 || h.EIP != 0x2c964 || h.ESPObject != 2 || h.ESP != 0x56b0 {
		t.Fatalf("unexpected FD2 header: %+v", h)
	}
	want := []LEObject{{0x3ebd9, 0x10000, 0x2045, 1, 0x3f, 0}, {0x56b0, 0x50000, 0x2043, 0x40, 4, 0}, {0x34d2, 0x60000, 0x2043, 0x44, 4, 0}}
	for i := range want {
		if h.Objects[i] != want[i] {
			t.Fatalf("object %d: got %+v want %+v", i+1, h.Objects[i], want[i])
		}
	}
}
