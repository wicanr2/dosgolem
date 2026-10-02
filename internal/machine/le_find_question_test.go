package machine

import (
	"bytes"
	"encoding/binary"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wicanr2/dosgolem/internal/cpu386"
)

func questionFindFixture(t *testing.T, files ReadOnlyFileProvider, pattern string) (*FD2StartupDOS, *cpu386.CPU, startupBus) {
	t.Helper()
	bus := startupBus(bytes.Repeat([]byte{0xa5}, 0x200))
	copy(bus[0x20:], pattern+"\x00")
	c := cpu386.New(bus)
	c.Seg[cpu386.SegDS] = 0x188
	c.SetDescriptor(0x188, cpu386.Descriptor{Limit: 0x1ff, Writable: true})
	s := NewMOO2StartupDOS(files)
	c.R[cpu386.EAX], c.R[cpu386.EDX] = 0x1a00, 0x80
	if !s.Handle(c, 0x21) {
		t.Fatal("DTA設定失敗")
	}
	c.R = [8]uint32{0x2b4e92, 0, 0x20, 0x80, 0x100, 0x110, 0x120, 0x130}
	c.EFlags = 0x246
	return s.FD2StartupDOS, c, bus
}

type questionNoEnumeration struct{ ReadOnlyFileProvider }

type questionEnumerationError struct{ questionNoEnumeration }

func (questionEnumerationError) ListReadOnlyNames() ([]string, error) { return nil, fs.ErrPermission }

func (questionNoEnumeration) OpenRead(string) (io.ReadSeekCloser, error) {
	return nil, fs.ErrNotExist
}

func TestMOO2FindFirstQuestionRejectsWithoutMutation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		files   ReadOnlyFileProvider
		pattern string
		change  func(*FD2StartupDOS, *cpu386.CPU, startupBus)
	}{
		{"無列舉", questionNoEnumeration{}, "save?.gam", nil},
		{"列舉錯誤", questionEnumerationError{}, "save?.gam", nil},
		{"其他遊戲", nil, "save?.gam", func(s *FD2StartupDOS, _ *cpu386.CPU, _ startupBus) { s.moo2Profile = false }},
		{"其他屬性", nil, "save?.gam", func(_ *FD2StartupDOS, c *cpu386.CPU, _ startupBus) { c.R[cpu386.ECX] = 1 }},
		{"DTA越界", nil, "save?.gam", func(s *FD2StartupDOS, _ *cpu386.CPU, _ startupBus) { s.dtaOffset = 0x1f0 }},
		{"父路徑", nil, "..\\save?.gam", nil},
		{"子路徑", nil, "dir\\save?.gam", nil},
		{"重複前綴", nil, ".\\.\\save?.gam", nil},
		{"磁碟機", nil, "C:save?.gam", nil},
		{"星號", nil, "save*.gam", nil},
		{"長檔名", nil, "ABCDEFGHI?.GAM", nil},
		{"無結束字元", nil, "save?.gam", func(_ *FD2StartupDOS, _ *cpu386.CPU, bus startupBus) {
			copy(bus[0x20:0x40], bytes.Repeat([]byte{'?'}, 32))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, c, bus := questionFindFixture(t, tc.files, tc.pattern)
			if tc.change != nil {
				tc.change(s, c, bus)
			}
			beforeR, beforeSeg, beforeFlags, before := c.R, c.Seg, c.EFlags, append([]byte(nil), bus...)
			if s.Handle(c, 0x21) || c.R != beforeR || c.Seg != beforeSeg || c.EFlags != beforeFlags || !bytes.Equal(bus, before) {
				t.Fatal("拒絕輸入時改動核心或記憶體")
			}
		})
	}
}

func TestMOO2FindFirstQuestionDirectoryFailures(t *testing.T) {
	for _, closed := range []bool{false, true} {
		t.Run(map[bool]string{false: "連結逃逸", true: "已關閉根目錄"}[closed], func(t *testing.T) {
			root, outside := t.TempDir(), filepath.Join(t.TempDir(), "outside")
			if err := os.WriteFile(outside, []byte{1}, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, filepath.Join(root, "SAVE1.GAM")); err != nil {
				t.Fatal(err)
			}
			files, err := OpenDirectoryReadOnlyFiles(root)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { files.Close() })
			if closed {
				if err := files.Close(); err != nil {
					t.Fatal(err)
				}
			}
			s, c, bus := questionFindFixture(t, files, "save?.gam")
			beforeR, beforeSeg, before := c.R, c.Seg, append([]byte(nil), bus...)
			if s.Handle(c, 0x21) || c.R != beforeR || c.Seg != beforeSeg || c.EFlags != 0x246 || !bytes.Equal(bus, before) {
				t.Fatal("根目錄保護失效或拒絕後改動")
			}
		})
	}
}

func TestMOO2FindFirstQuestionNilIsExplicitEmpty(t *testing.T) {
	s, c, bus := questionFindFixture(t, nil, "save?.gam")
	beforeR, beforeSeg, before := c.R, c.Seg, append([]byte(nil), bus...)
	beforeR[cpu386.EAX] = 0x12
	if !s.Handle(c, 0x21) || c.R != beforeR || c.Seg != beforeSeg || c.EFlags != 0x247 || !bytes.Equal(bus[0x8c:], before[0x8c:]) {
		t.Fatal("明示空目錄缺檔契約不符")
	}
}

func TestMOO2FindFirstQuestionActualFiles(t *testing.T) {
	for _, tc := range []struct {
		pattern string
		names   []string
		want    string
	}{
		{"save?.gam", []string{"SAVE10.GAM"}, ""},
		{"save?.gam", []string{"SAVE10.GAM", "save2.gam", "SAVE1.GAM"}, "SAVE1.GAM"},
		{"save?.gam", []string{"SAVE1.GAM", "SAVE.GAM"}, "SAVE.GAM"},
		{".\\save??.gam", []string{"SAVE10.GAM"}, "SAVE10.GAM"},
		{"save1.?am", []string{"SAVE1.GAM", "SAVE1.XAM"}, "SAVE1.GAM"},
		{"SAVE?.GAM", []string{"SAVE12.GAM", "LONGFILENAME.GAM"}, ""},
	} {
		t.Run(tc.pattern+tc.want, func(t *testing.T) {
			root := t.TempDir()
			for _, name := range tc.names {
				path := filepath.Join(root, name)
				if err := os.WriteFile(path, []byte{1, 2, 3}, 0o600); err != nil {
					t.Fatal(err)
				}
				stamp := time.Date(1996, 1, 1, 0, 0, 0, 0, time.UTC)
				if err := os.Chtimes(path, stamp, stamp); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Mkdir(filepath.Join(root, "SAVE0.GAM"), 0o700); err != nil {
				t.Fatal(err)
			}
			files, err := OpenDirectoryReadOnlyFiles(root)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { files.Close() })
			s, c, bus := questionFindFixture(t, files, tc.pattern)
			beforeR, beforeSeg, before := c.R, c.Seg, append([]byte(nil), bus...)
			if !s.Handle(c, 0x21) {
				t.Fatal("合法問號搜尋被拒絕")
			}
			flags := uint32(0x246)
			beforeR[cpu386.EAX] = 0x2b0000
			if tc.want == "" {
				beforeR[cpu386.EAX], flags = 0x12, 0x247
				if !bytes.Equal(bus[0x8c:0xab], before[0x8c:0xab]) {
					t.Fatal("缺檔改動舊結果區")
				}
			} else {
				end := bytes.IndexByte(bus[0x9e:0xab], 0)
				if end < 0 || string(bus[0x9e:0x9e+end]) != tc.want || bus[0x95] != 0x20 || binary.LittleEndian.Uint16(bus[0x96:]) != 0 || binary.LittleEndian.Uint16(bus[0x98:]) != 0x2021 || binary.LittleEndian.Uint32(bus[0x9a:]) != 3 {
					t.Fatalf("結果名稱／metadata不符：%X", bus[0x95:0xab])
				}
			}
			if c.R != beforeR || c.Seg != beforeSeg || c.EFlags != flags || !bytes.Equal(bus[:0x80], before[:0x80]) || !bytes.Equal(bus[0xab:], before[0xab:]) {
				t.Fatal("核心或DTA鄰接哨兵改變")
			}
			if bus[0x80] != 2 || !bytes.Contains(bus[0x81:0x8c], []byte{'?'}) {
				t.Fatal("搜尋header沒有保存問號pattern")
			}
		})
	}
}
