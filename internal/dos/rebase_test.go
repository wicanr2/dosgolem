package dos

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// docs/spec/450 §3.1：開檔路徑改寫。

// §3.1 第 1 項：RebasePath 表驅動。
func TestRebasePath(t *testing.T) {
	for _, tc := range []struct {
		name     string
		path     string
		prefixes []string
		want     string // 空字串＝預期回錯
	}{
		{"正常改寫", "/orig/X", []string{"/orig"}, "/to/X"},
		{"子目錄", "/orig/A/B.DAT", []string{"/orig"}, "/to/A/B.DAT"},
		{"最長前綴", "/a/save/X", []string{"/a", "/a/save"}, "/to/X"},
		{"最長前綴（順序相反）", "/a/save/X", []string{"/a/save", "/a"}, "/to/X"},
		{"最長前綴只在前綴底下", "/a/other/X", []string{"/a", "/a/save"}, "/to/other/X"},
		{"兄弟目錄不符合", "/orig2/X", []string{"/orig"}, ""},
		{"等於前綴回錯", "/orig", []string{"/orig"}, ""},
		{"等於前綴（尾斜線）回錯", "/orig/", []string{"/orig"}, ""},
		{"含 .. 先 Clean 再比對", "/orig/../etc/x", []string{"/orig"}, ""},
		{"..x 檔名照常接受", "/orig/..x", []string{"/orig"}, "/to/..x"},
		{"路徑重複分隔字元", "/orig//sub///X", []string{"/orig"}, "/to/sub/X"},
		{"前綴重複分隔字元", "/orig/X", []string{"/orig//"}, "/to/X"},
		{"前綴重複分隔字元仍取最長", "/a/save/X", []string{"/a////////", "/a/save"}, "/to/X"},
		{"前綴尾斜線", "/orig/X", []string{"/orig/"}, "/to/X"},
		{"前綴為 /", "/orig/X", []string{"/"}, "/to/orig/X"},
		{"空字串與 . 前綴略過", "/orig/X", []string{"", ".", "/orig"}, "/to/X"},
		{"只有空字串與 . 前綴", "X", []string{"", "."}, ""},
		{"沒有前綴符合", "/other/X", []string{"/orig", "/scratch"}, ""},
		{"沒有前綴", "/orig/X", nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RebasePath(tc.path, tc.prefixes, "/to")
			if tc.want == "" {
				if err == nil {
					t.Fatalf("RebasePath(%q, %q) = %q，預期回錯", tc.path, tc.prefixes, got)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("RebasePath(%q, %q) = %q, %v，預期 %q", tc.path, tc.prefixes, got, err, tc.want)
			}
		})
	}
}

// rebaseFixture 是 §3.1 第 2、3 項的存檔端：Root 是 A、Scratch 是 B，A 下開 X、B 下開 Z，
// 各讀到中段後 SaveState。
type rebaseFixture struct {
	a, b   string
	hx, hz uint16
	state  []byte
}

const (
	rebaseX = "XXXXXXXXxxxxxxxx" // A/X 原檔，16 bytes
	rebaseZ = "ZZZZZZzzzzzz"     // B/Z 原檔，12 bytes
	// C 內的複本：大小相同、內容不同。
	rebaseCX = "0123456789abcdef"
	rebaseCZ = "ghijklmnopqr"
)

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// readHandle 以 int 21h AH=3Fh 從 h 讀 n bytes，回讀到的內容。
func readHandle(t *testing.T, m *machine.Machine, d *DOS, h uint16, n uint16) string {
	t.Helper()
	const bufOff = 0x200
	m.CPU.R[cpu.BX], m.CPU.R[cpu.CX] = h, n
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = fileScratchSeg, bufOff
	call(m, d, 0x21, 0x3F00)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("讀 handle %d 失敗：AX=%04X", h, m.CPU.R[cpu.AX])
	}
	got := make([]byte, m.CPU.R[cpu.AX])
	for i := range got {
		got[i] = m.Read8(cpu.Addr(fileScratchSeg, bufOff+uint16(i)))
	}
	return string(got)
}

func newRebaseFixture(t *testing.T) rebaseFixture {
	t.Helper()
	f := rebaseFixture{a: t.TempDir(), b: t.TempDir()}
	writeFile(t, f.a, "X", rebaseX)
	writeFile(t, f.b, "Z", rebaseZ)
	m := machine.New()
	d := New(m, f.a)
	d.Scratch = f.b
	d.Install()
	d.freeSeg = 0x2000
	f.hx = openFile(t, m, d, "X", 0)
	f.hz = openFile(t, m, d, "Z", 0)
	if got := d.handles[f.hx].path; got != filepath.Join(f.a, "X") {
		t.Fatalf("存檔端 X 開到 %s", got)
	}
	if got := d.handles[f.hz].path; got != filepath.Join(f.b, "Z") {
		t.Fatalf("存檔端 Z 開到 %s", got)
	}
	if got := readHandle(t, m, d, f.hx, 8); got != rebaseX[:8] {
		t.Fatalf("存檔端讀 X 得 %q", got)
	}
	if got := readHandle(t, m, d, f.hz, 6); got != rebaseZ[:6] {
		t.Fatalf("存檔端讀 Z 得 %q", got)
	}
	var buf bytes.Buffer
	if err := d.SaveState(&buf); err != nil {
		t.Fatal(err)
	}
	d.Close()
	f.state = buf.Bytes()
	return f
}

// §3.1 第 2 項：改寫成功。
func TestLoadStateRebasesHandles(t *testing.T) {
	f := newRebaseFixture(t)
	c := t.TempDir()
	writeFile(t, c, "X", rebaseCX)
	writeFile(t, c, "Z", rebaseCZ)

	m := machine.New()
	d := New(m, t.TempDir())
	d.Install()
	d.freeSeg = 0x2000
	d.RebaseTo, d.RebaseFrom = c, []string{f.b}
	if err := d.LoadState(bytes.NewReader(f.state)); err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if d.RebaseTo != "" || d.RebaseFrom != nil {
		t.Errorf("讀檔後 RebaseTo=%q RebaseFrom=%q，應清空", d.RebaseTo, d.RebaseFrom)
	}
	if d.Root != c {
		t.Errorf("讀檔後 Root=%q，要 %q", d.Root, c)
	}
	for _, tc := range []struct {
		h    uint16
		name string
		want string
	}{{f.hx, "X", rebaseCX[8:]}, {f.hz, "Z", rebaseCZ[6:]}} {
		hd := d.handles[tc.h]
		if hd == nil {
			t.Fatalf("handle %d（%s）沒有還原", tc.h, tc.name)
		}
		if want := filepath.Join(c, tc.name); hd.path != want {
			t.Errorf("handle %d 的 path=%q，要 %q", tc.h, hd.path, want)
		}
		if got := readHandle(t, m, d, tc.h, 32); got != tc.want {
			t.Errorf("handle %d（%s）讀到 %q，要 C 內複本同位置之後的 %q", tc.h, tc.name, got, tc.want)
		}
	}
}

func dosDigest(t *testing.T, d *DOS) [32]byte {
	t.Helper()
	var buf bytes.Buffer
	if err := d.SaveState(&buf); err != nil {
		t.Fatal(err)
	}
	sum, err := StateDigest(&buf)
	if err != nil {
		t.Fatal(err)
	}
	return sum
}

// §3.1 第 3 項：改寫失敗時維持原狀。
func TestLoadStateRebaseFailureKeepsDOS(t *testing.T) {
	f := newRebaseFixture(t)
	full := t.TempDir()
	writeFile(t, full, "X", rebaseCX)
	writeFile(t, full, "Z", rebaseCZ)
	badSize := t.TempDir()
	writeFile(t, badSize, "X", rebaseCX+"!")
	writeFile(t, badSize, "Z", rebaseCZ)
	noZ := t.TempDir()
	writeFile(t, noZ, "X", rebaseCX)

	for _, tc := range []struct {
		name string
		to   string
		from []string
	}{
		{"RebaseFrom 漏給 B", full, nil},
		{"C 內 X 的大小不同", badSize, []string{f.b}},
		{"C 內沒有 Z", noZ, []string{f.b}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := machine.New()
			root := t.TempDir()
			writeFile(t, root, "Q", "QUIET-OLD-FILE")
			d := New(m, root)
			d.Install()
			d.freeSeg = 0x2000
			defer d.Close()
			d.Drive, d.Dir = 3, "ELSEWHERE"
			hq := openFile(t, m, d, "Q", 0)
			if got := readHandle(t, m, d, hq, 6); got != "QUIET-" {
				t.Fatalf("讀檔前讀 Q 得 %q", got)
			}
			before := dosDigest(t, d)

			d.RebaseTo, d.RebaseFrom = tc.to, tc.from
			if err := d.LoadState(bytes.NewReader(f.state)); err == nil {
				t.Fatal("預期 LoadState 回錯")
			}
			if d.RebaseTo != "" || d.RebaseFrom != nil {
				t.Errorf("失敗後 RebaseTo=%q RebaseFrom=%q，應清空", d.RebaseTo, d.RebaseFrom)
			}
			if after := dosDigest(t, d); after != before {
				t.Error("失敗後 DOS 的 StateDigest 改變了")
			}
			if d.Root != root || d.Drive != 3 || d.Dir != "ELSEWHERE" {
				t.Errorf("失敗後 Root=%q Drive=%d Dir=%q", d.Root, d.Drive, d.Dir)
			}
			// 舊檔案代號仍可 seek 與讀。
			m.CPU.R[cpu.BX], m.CPU.R[cpu.CX], m.CPU.R[cpu.DX] = hq, 0, 2
			call(m, d, 0x21, 0x4200)
			if m.CPU.Flags&cpu.CF != 0 {
				t.Fatalf("失敗後舊檔案代號 seek 不了：AX=%04X", m.CPU.R[cpu.AX])
			}
			if got := readHandle(t, m, d, hq, 32); !strings.HasPrefix(got, "IET-OLD") {
				t.Errorf("失敗後舊檔案代號讀到 %q", got)
			}
		})
	}
}

// §3.1 第 4 項的補充：RebaseTo 為空時開原路徑、Root 取狀態檔的值（既有行為）。
func TestLoadStateWithoutRebaseKeepsPaths(t *testing.T) {
	f := newRebaseFixture(t)
	m := machine.New()
	d := New(m, t.TempDir())
	d.Install()
	d.freeSeg = 0x2000
	if err := d.LoadState(bytes.NewReader(f.state)); err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if d.Root != f.a {
		t.Errorf("Root=%q，要狀態檔的 %q", d.Root, f.a)
	}
	if got := d.handles[f.hx].path; got != filepath.Join(f.a, "X") {
		t.Errorf("X 的 path=%q", got)
	}
	if got := readHandle(t, m, d, f.hz, 32); got != rebaseZ[6:] {
		t.Errorf("Z 讀到 %q", got)
	}
}
