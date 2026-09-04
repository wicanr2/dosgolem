package dos

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 檔案服務：`AH=3Dh` 開、`3Eh` 關、`3Fh` 讀、`42h` seek。

type handle struct {
	name string
	path string
	f    *os.File
	size int64
}

// resolve 把遊戲組出來的路徑對到實際檔案。
//
// ⚠ **只認 basename。** 遊戲會自己組出 `A:\<垃圾>\DATA.PAK` 這種路徑
// （多磁片版的殘留），目錄部分對不上硬碟安裝版
// （`rich2/docs/re/006` §5：檔名是執行期組出來的，所以靜態找不到引用）。
//
// **大小寫不分**：原版是 DOS，檔名全大寫；玩家的目錄可能是小寫。
func (d *DOS) resolve(name string) string {
	base := name
	if i := strings.LastIndexAny(base, `\/:`); i >= 0 {
		base = base[i+1:]
	}
	if base == "" {
		return ""
	}
	direct := filepath.Join(d.Root, base)
	if st, err := os.Stat(direct); err == nil && !st.IsDir() {
		return direct
	}
	entries, err := os.ReadDir(d.Root)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(e.Name(), base) {
			return filepath.Join(d.Root, e.Name())
		}
	}
	return ""
}

// readCString 讀一個 NUL 結尾的字串。
func (d *DOS) readCString(seg, off uint16, limit int) string {
	addr := cpu.Addr(seg, off)
	out := make([]byte, 0, 64)
	for i := 0; i < limit; i++ {
		ch := d.M.Read8(addr + uint32(i))
		if ch == 0 {
			break
		}
		out = append(out, ch)
	}
	return string(out)
}

func (d *DOS) open(c *cpu.CPU) {
	name := d.readCString(c.Seg[cpu.DS], c.R[cpu.DX], 128)
	path := d.resolve(name)
	if path == "" {
		d.Missing = append(d.Missing, name)
		c.R[cpu.AX] = 2 // File not found
		setCarry(c)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		d.Missing = append(d.Missing, name)
		c.R[cpu.AX] = 2
		setCarry(c)
		return
	}
	st, _ := f.Stat()
	h := d.nextHandle
	d.nextHandle++
	d.handles[h] = &handle{name: name, path: path, f: f, size: st.Size()}
	d.Opened = append(d.Opened, filepath.Base(path))
	c.R[cpu.AX] = h
	clearCarry(c)
}

func (d *DOS) close(c *cpu.CPU) {
	h, ok := d.handles[c.R[cpu.BX]]
	if !ok {
		c.R[cpu.AX] = 6 // Invalid handle
		setCarry(c)
		return
	}
	if h.f != nil {
		h.f.Close()
	}
	delete(d.handles, c.R[cpu.BX])
	clearCarry(c)
}

func (d *DOS) read(c *cpu.CPU) {
	bx, cx := c.R[cpu.BX], c.R[cpu.CX]
	if bx == 0 {
		d.readStdin(c, cx)
		return
	}
	h, ok := d.handles[bx]
	if !ok {
		c.R[cpu.AX] = 6
		setCarry(c)
		return
	}
	buf := make([]byte, cx)
	n, _ := h.f.Read(buf)
	if n < 0 {
		n = 0
	}
	d.M.WriteBytes(cpu.Addr(c.Seg[cpu.DS], c.R[cpu.DX]), buf[:n])
	c.R[cpu.AX] = uint16(n)
	clearCarry(c)
}

// readStdin 是 `AH=3Fh` 讀 handle 0，也就是 BASIC 的 `INKEY$`
// （`rich2/docs/re/005`「輸入路徑」：這是唯一的鍵盤輪詢路徑，不是 `int 16h`）。
//
// ⚠ **不能回「讀到 0 個」。** 那等同 EOF，主程式會當成輸入結束、
// 還原中斷向量然後 exit——`RUN.EXE` 之前就死在這裡。
func (d *DOS) readStdin(c *cpu.CPU, want uint16) {
	if want == 0 {
		c.R[cpu.AX] = 0
		clearCarry(c)
		return
	}
	ch := d.StdinFill
	if len(d.Stdin) > 0 {
		ch = d.Stdin[0]
		d.Stdin = d.Stdin[1:]
	}
	d.M.WriteBytes(cpu.Addr(c.Seg[cpu.DS], c.R[cpu.DX]), []byte{ch})
	c.R[cpu.AX] = 1
	clearCarry(c)
}

func (d *DOS) seek(c *cpu.CPU) {
	h, ok := d.handles[c.R[cpu.BX]]
	if !ok {
		c.R[cpu.AX] = 6
		setCarry(c)
		return
	}
	off := int64(c.R[cpu.CX])<<16 | int64(c.R[cpu.DX])
	// CX:DX 是**有號**的：從結尾往回 seek 用負數。
	if c.R[cpu.CX]&0x8000 != 0 {
		off -= 1 << 32
	}
	pos, err := h.f.Seek(off, int(al(c)))
	if err != nil {
		c.R[cpu.AX] = 1
		setCarry(c)
		return
	}
	c.R[cpu.AX] = uint16(pos)
	c.R[cpu.DX] = uint16(pos >> 16)
	clearCarry(c)
}
