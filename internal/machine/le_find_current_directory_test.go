package machine

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wicanr2/dosgolem/internal/cpu386"
)

type findCurrentDirectoryProvider struct {
	ReadOnlyFileProvider
	names []string
}

func (p *findCurrentDirectoryProvider) OpenRead(name string) (io.ReadSeekCloser, error) {
	p.names = append(p.names, name)
	return p.ReadOnlyFileProvider.OpenRead(name)
}

func TestMOO2FindFirstCurrentDirectoryMatchesPlain(t *testing.T) {
	for _, name := range []string{"simtex.lbx", "ABCDEFGH.XYZ"} {
		for _, present := range []bool{false, true} {
			t.Run(name+map[bool]string{false: "/缺檔", true: "/存在"}[present], func(t *testing.T) {
				root := t.TempDir()
				if present {
					path := filepath.Join(root, name)
					if err := os.WriteFile(path, []byte{1, 2, 3}, 0o600); err != nil {
						t.Fatal(err)
					}
					stamp := time.Date(1996, 1, 1, 0, 0, 0, 0, time.UTC)
					if err := os.Chtimes(path, stamp, stamp); err != nil {
						t.Fatal(err)
					}
				}
				files, err := OpenDirectoryReadOnlyFiles(root)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { files.Close() })
				provider := &findCurrentDirectoryProvider{ReadOnlyFileProvider: files}
				var plainR [8]uint32
				var plainFlags uint32
				var plainDTA []byte
				for _, prefix := range []string{"", ".\\"} {
					bus := startupBus(bytes.Repeat([]byte{0xa5}, 0x200))
					copy(bus[0x20:], prefix+name+"\x00")
					c := cpu386.New(bus)
					c.Seg[cpu386.SegDS] = 0x188
					c.SetDescriptor(0x188, cpu386.Descriptor{Limit: 0x1ff, Writable: true})
					s := NewMOO2StartupDOS(provider)
					c.R[cpu386.EAX], c.R[cpu386.EDX] = 0x1a00, 0x80
					if !s.Handle(c, 0x21) {
						t.Fatal("DTA 設定失敗")
					}
					c.R = [8]uint32{0x2b4e38, 0, 0x20, 0x80, 0x100, 0x110, 0x120, 0x130}
					c.EFlags = 0x246
					beforeR, beforeSeg := c.R, c.Seg
					beforeData := append([]byte(nil), bus...)
					if !s.Handle(c, 0x21) {
						t.Fatal("已審查的目前目錄查詢遭拒絕")
					}
					wantAX, wantFlags := uint32(0x12), uint32(0x247)
					if present {
						wantAX, wantFlags = 0x2b0000, 0x246
					}
					beforeR[cpu386.EAX] = wantAX
					if c.R != beforeR || c.Seg != beforeSeg || c.EFlags != wantFlags ||
						!bytes.Equal(bus[:0x80], beforeData[:0x80]) || !bytes.Equal(bus[0x80+43:], beforeData[0x80+43:]) {
						t.Fatalf("返回或鄰接資料不符：R=%X flags=%X", c.R, c.EFlags)
					}
					if !present && !bytes.Equal(bus[0x8c:0x80+43], beforeData[0x8c:0x80+43]) {
						t.Fatal("缺檔覆寫了既有保留區／結果區")
					}
					if prefix == "" {
						plainR, plainFlags, plainDTA = c.R, c.EFlags, append([]byte(nil), bus[0x80:0x80+43]...)
					} else if c.R != plainR || c.EFlags != plainFlags || !bytes.Equal(bus[0x80:0x80+43], plainDTA) {
						t.Fatal("加前綴後未得到相同的執行器結果")
					}
				}
				if len(provider.names) != 2 || provider.names[0] != name || provider.names[1] != name {
					t.Fatalf("提供者收到未正規化的路徑：%q", provider.names)
				}
			})
		}
	}
}

func TestMOO2FindFirstCurrentDirectoryRejectsOutsideContract(t *testing.T) {
	for _, tc := range []struct {
		name string
		fd2  bool
		cx   uint32
		dta  uint32
	}{
		{".\\", false, 0, 0x80}, {".\\.\\simtex.lbx", false, 0, 0x80},
		{"..\\simtex.lbx", false, 0, 0x80}, {".\\dir\\x.lbx", false, 0, 0x80},
		{"C:simtex.lbx", false, 0, 0x80}, {"./simtex.lbx", false, 0, 0x80},
		{".\\s*.lbx", false, 0, 0x80}, {".\\ABCDEFGHI.XYZ", false, 0, 0x80},
		{".\\simtex.lbx", true, 0, 0x80}, {".\\simtex.lbx", false, 1, 0x80},
		{".\\simtex.lbx", false, 0, 0x1f0},
	} {
		bus := startupBus(bytes.Repeat([]byte{0xa5}, 0x200))
		copy(bus[0x20:], tc.name+"\x00")
		c := cpu386.New(bus)
		c.Seg[cpu386.SegDS] = 0x188
		c.SetDescriptor(0x188, cpu386.Descriptor{Limit: 0x1ff, Writable: true})
		s := NewFD2StartupDOS(nil)
		s.moo2Profile = !tc.fd2
		c.R[cpu386.EAX], c.R[cpu386.EDX] = 0x1a00, tc.dta
		s.Handle(c, 0x21)
		c.R[cpu386.EAX], c.R[cpu386.ECX], c.R[cpu386.EDX], c.EFlags = 0x2b4e38, tc.cx, 0x20, 0x246
		beforeR, beforeSeg, beforeData := c.R, c.Seg, append([]byte(nil), bus...)
		if s.Handle(c, 0x21) || c.R != beforeR || c.Seg != beforeSeg || c.EFlags != 0x246 || !bytes.Equal(bus, beforeData) {
			t.Fatalf("未核准輸入沒有完整拒絕：%q fd2=%t cx=%X dta=%X", tc.name, tc.fd2, tc.cx, tc.dta)
		}
	}
}
