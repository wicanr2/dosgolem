package dos

import (
	"fmt"
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
	// writable 為真時 `AH=40h` 真的寫下去（`docs/spec/009`）。
	writable bool
}

// resolve 把遊戲組出來的路徑對到實際檔案。
//
// ⚠ **只認 basename。** 遊戲會自己組出 `A:\<垃圾>\DATA.PAK` 這種路徑
// （多磁片版的殘留），目錄部分對不上硬碟安裝版
// （`rich2/docs/re/006` §5：檔名是執行期組出來的，所以靜態找不到引用）。
//
// **大小寫不分**：原版是 DOS，檔名全大寫；玩家的目錄可能是小寫。
func (d *DOS) resolve(name string) string {
	base := baseName(name)
	if base == "" {
		return ""
	}
	// 暫存層蓋過原版目錄（`docs/spec/009` §2.2.1）：程式存過的東西
	// 下一次要讀得到自己寫的那一份，不是原版那一份。
	if d.Scratch != "" {
		if path := lookup(d.Scratch, base); path != "" {
			return path
		}
	}
	return lookup(d.Root, base)
}

// lookup 在一個目錄裡找 basename，大小寫不分。
func lookup(dir, base string) string {
	direct := filepath.Join(dir, base)
	if st, err := os.Stat(direct); err == nil && !st.IsDir() {
		return direct
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(e.Name(), base) {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

// baseName 取出 DOS 路徑的最後一段。遊戲會組出 `A:\<垃圾>\X.CHA`
// 這種路徑，目錄部分對不上實際安裝（`resolve` 的註解）。
func baseName(name string) string {
	base := name
	if i := strings.LastIndexAny(base, `\/:`); i >= 0 {
		base = base[i+1:]
	}
	return base
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

// open 是 `AH=3Dh`。存取模式在 `AL` 的低兩位：0 唯讀、1 唯寫、2 讀寫。
//
// 要寫而檔案只在唯讀的原版目錄裡時，**先整份複製到暫存層**再開
// （`docs/spec/009` §2.2.3）。沒有暫存層就退回唯讀——寫入照舊只記帳。
func (d *DOS) open(c *cpu.CPU) {
	name := d.readCString(c.Seg[cpu.DS], c.R[cpu.DX], 128)
	wantsWrite := d.Scratch != "" && al(c)&0x03 != 0
	path := d.resolve(name)
	if path == "" {
		d.Missing = append(d.Missing, name)
		c.R[cpu.AX] = 2 // File not found
		setCarry(c)
		return
	}
	var (
		f   *os.File
		err error
	)
	if wantsWrite {
		path, err = d.scratchCopy(name, path)
		if err == nil {
			f, err = os.OpenFile(path, os.O_RDWR, 0o644)
		}
	} else {
		f, err = os.Open(path)
	}
	if err != nil {
		d.Missing = append(d.Missing, name)
		c.R[cpu.AX] = 2
		setCarry(c)
		return
	}
	st, _ := f.Stat()
	h := d.nextHandle
	d.nextHandle++
	d.handles[h] = &handle{name: name, path: path, f: f, size: st.Size(), writable: wantsWrite}
	d.Opened = append(d.Opened, filepath.Base(path))
	c.R[cpu.AX] = h
	clearCarry(c)
}

// scratchCopy 保證暫存層裡有這個檔的一份可寫副本，回傳它的路徑。
// 已經在暫存層裡的就原樣用。
func (d *DOS) scratchCopy(name, path string) (string, error) {
	base := baseName(name)
	target := filepath.Join(d.Scratch, base)
	if _, err := os.Stat(target); err == nil {
		return target, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(d.Scratch, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return "", err
	}
	return target, nil
}

// create 是 `AH=3Ch`：在暫存層建立／截斷一個檔，回可讀寫的 handle。
//
// 沒有暫存層時**照舊不落地**，但仍要回一個合法 handle——回失敗的話
// 呼叫端會走錯誤路徑，而那與「存檔功能沒做」是兩種不同的行為。
func (d *DOS) create(c *cpu.CPU) {
	name := d.readCString(c.Seg[cpu.DS], c.R[cpu.DX], 128)
	base := baseName(name)
	if d.Scratch == "" || base == "" {
		h := d.nextHandle
		d.nextHandle++
		d.handles[h] = &handle{name: name}
		c.R[cpu.AX] = h
		clearCarry(c)
		return
	}
	if err := os.MkdirAll(d.Scratch, 0o755); err != nil {
		c.R[cpu.AX] = 3 // Path not found
		setCarry(c)
		return
	}
	path := filepath.Join(d.Scratch, base)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		c.R[cpu.AX] = 5 // Access denied
		setCarry(c)
		return
	}
	h := d.nextHandle
	d.nextHandle++
	d.handles[h] = &handle{name: name, path: path, f: f, writable: true}
	c.R[cpu.AX] = h
	clearCarry(c)
}

// unlink 是 `AH=41h`：只刪暫存層底下的，原版目錄永遠不動。
func (d *DOS) unlink(c *cpu.CPU) {
	name := d.readCString(c.Seg[cpu.DS], c.R[cpu.DX], 128)
	base := baseName(name)
	if d.Scratch == "" || base == "" {
		clearCarry(c)
		return
	}
	os.Remove(filepath.Join(d.Scratch, base))
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

// write 是 `AH=40h`。
//
// ⚠ **這是程式對我們說話的主要管道**，不是 `AH=09h`。BASIC runtime 的
// `PRINT` 與錯誤訊息都走這裡（handle 1／2），一次一小段。
// 沒接的話主控台是空的——看起來像「程式什麼都沒說」，
// 而實際上它正在印錯誤訊息（第一次跑通 CPU 之後就是這個症狀）。
//
// **原版目錄永遠不寫。** 有 `Scratch` 時寫進暫存層（`docs/spec/009`），
// 沒有時只記一筆再回報成功——安靜地成功會讓「存檔壞掉」完全查不出來。
func (d *DOS) write(c *cpu.CPU) {
	bx, cx := c.R[cpu.BX], c.R[cpu.CX]
	buf := make([]byte, cx)
	addr := cpu.Addr(c.Seg[cpu.DS], c.R[cpu.DX])
	for i := range buf {
		buf[i] = d.M.Read8(addr + uint32(i))
	}
	switch bx {
	case 1, 2: // stdout／stderr
		d.Console = append(d.Console, buf...)
	default:
		h, ok := d.handles[bx]
		if !ok {
			d.Wrote = append(d.Wrote, Write{Name: fmt.Sprintf("handle %d", bx), N: int(cx)})
			break
		}
		// `Wrote` 記的是「程式想存什麼」，寫成功也要記
		//（`docs/spec/009` §2.4），不是失敗清單。
		d.Wrote = append(d.Wrote, Write{Name: h.name, N: int(cx)})
		if h.writable && h.f != nil {
			n, err := h.f.Write(buf)
			if err != nil {
				c.R[cpu.AX] = 5 // Access denied
				setCarry(c)
				return
			}
			c.R[cpu.AX] = uint16(n)
			clearCarry(c)
			return
		}
	}
	c.R[cpu.AX] = cx
	clearCarry(c)
}
