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
	// psp 是開這個檔的行程。**子行程結束時要關掉它名下的檔**——
	// 真 DOS 的 AH=4Ch 就是這樣做的，漏關的症狀是「跑久了開不了檔」，
	// 而錯誤碼是 04h（handle 用盡），完全不指向這裡。
	//
	// ⚠ 不能用「號碼 >= 進子行程時的下一個號碼」來判：號碼是
	// **最小的空號**，關掉再開會拿回舊號碼，區間判準因此不成立。
	psp uint16
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
	// 字元裝置：開 EMMXXXX0 成功 ＝ EMS 驅動存在（`docs/spec/007` §5）。
	// launcher 開完就關，不讀不寫；讀寫語意沒有證據，讀回 EOF、寫丟棄。
	if isEMMDevice(name) {
		h, ok := d.allocHandle()
		if !ok {
			c.R[cpu.AX] = 4 // Too many open files
			setCarry(c)
			return
		}
		d.handles[h] = &handle{name: name, psp: d.curPSP}
		d.Opened = append(d.Opened, name)
		if d.OnOpen != nil {
			d.OnOpen(name)
		}
		c.R[cpu.AX] = h
		clearCarry(c)
		return
	}
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
	h, ok := d.allocHandle()
	if !ok {
		f.Close()
		c.R[cpu.AX] = 4 // Too many open files
		setCarry(c)
		return
	}
	d.handles[h] = &handle{name: name, path: path, f: f, size: st.Size(), psp: d.curPSP}
	base := filepath.Base(path)
	d.Opened = append(d.Opened, base)
	d.trace(FileOp{Op: "open", Fn: 0x3D, Handle: h, Name: name,
		Arg: st.Size(), Len: int(h)})
	if d.OnOpen != nil {
		d.OnOpen(base)
	}
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
	if h.f == nil { // 字元裝置（EMMXXXX0）：讀回 EOF
		c.R[cpu.AX] = 0
		clearCarry(c)
		return
	}
	pos, _ := h.f.Seek(0, 1)
	buf := make([]byte, cx)
	n, _ := h.f.Read(buf)
	if n < 0 {
		n = 0
	}
	d.FileOps = append(d.FileOps, FileOp{Fn: 0x3F, Handle: bx, Name: h.name,
		Pos: pos, Len: n, Step: d.M.Steps})
	d.M.WriteBytes(cpu.Addr(c.Seg[cpu.DS], c.R[cpu.DX]), buf[:n])
	d.Reads = append(d.Reads, ReadOp{
		Step: d.M.Steps, Name: h.name, Handle: bx,
		Seg: c.Seg[cpu.DS], Off: c.R[cpu.DX], Want: cx, Got: n,
	})
	d.trace(FileOp{Op: "read", Fn: 0x3F, Handle: bx, Name: h.name,
		Arg: int64(cx), Pos: pos, Len: n})
	c.R[cpu.AX] = uint16(n)
	clearCarry(c)
}

// fileAttr 是 `AH=43h`。AL=00 取屬性：找到回 CX=0x20（archive 普通檔）；
// AL=01 設屬性**不做**（素材唯讀）——記一筆再清 CF，
// 與 Wrote 清單同一原則：看得見的假，不是安靜的假。
func (d *DOS) fileAttr(c *cpu.CPU) {
	if al(c) != 0x00 {
		d.note(0x21, 0x43, al(c))
		clearCarry(c)
		return
	}
	name := d.readCString(c.Seg[cpu.DS], c.R[cpu.DX], 128)
	if d.resolve(name) == "" && !isEMMDevice(name) {
		d.Missing = append(d.Missing, name)
		c.R[cpu.AX] = 2
		setCarry(c)
		return
	}
	c.R[cpu.CX] = 0x20
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
	// **有資料就照呼叫端要的個數給。**
	//
	// ⚠ 舊版不管 `want` 一律只回 1 個位元組，於是**擴充鍵永遠送不進去**：
	// 方向鍵是 `00` ＋ 掃描碼兩個位元組，呼叫端一次要兩個的時候
	// 只拿到那個 `00`——而 `00` 也正是佇列空的時候餵的值（＝「沒按鍵」），
	// 所以它被當成「什麼都沒按」丟掉。症狀是**方向鍵安靜地沒有反應**，
	// 看起來像編碼寫錯，而不是像讀取實作漏了一個參數。
	n := int(want)
	if n > len(d.Stdin) {
		n = len(d.Stdin)
	}
	if n > 0 {
		d.M.WriteBytes(cpu.Addr(c.Seg[cpu.DS], c.R[cpu.DX]), d.Stdin[:n])
		for _, ch := range d.Stdin[:n] {
			d.noteKey("int21-3F", ch)
		}
		d.Stdin = d.Stdin[n:]
		c.R[cpu.AX] = uint16(n)
		clearCarry(c)
		return
	}
	// 佇列空了。**預設還是要回「讀到 1 個」**——回 0 等同 EOF，
	// 主程式會還原中斷向量然後 exit。
	//
	// 例外是 StdinEmptyReadsZero：BASIC 的 `INKEY$` 空轉要看到**空字串**
	// 才會繼續等，餵 `00` 會讓它拿到 `CHR$(0)` 而立刻結束
	// （見該欄位的註解）。要讓「按任意鍵繼續」的畫面停住就打開它。
	if d.StdinEmptyReadsZero {
		c.R[cpu.AX] = 0
		clearCarry(c)
		return
	}
	d.M.WriteBytes(cpu.Addr(c.Seg[cpu.DS], c.R[cpu.DX]), []byte{d.StdinFill})
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
	if h.f == nil { // 字元裝置不能 seek
		c.R[cpu.AX] = 1
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
	d.trace(FileOp{Op: "seek", Fn: 0x42, Handle: c.R[cpu.BX],
		Name: h.name, Arg: off, Pos: pos})
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
// **寫檔一律不做。** 原版素材唯讀（`CLAUDE.md`），而 MVP-B 不需要存檔；
// 真的有人寫檔要看得到，所以記一筆而不是安靜地報成功。
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
		if h, ok := d.handles[bx]; ok {
			d.Wrote = append(d.Wrote, Write{Name: h.name, N: int(cx)})
		} else {
			d.Wrote = append(d.Wrote, Write{Name: fmt.Sprintf("handle %d", bx), N: int(cx)})
		}
	}
	c.R[cpu.AX] = cx
	clearCarry(c)
}

// trace 記一次檔案操作；FileTrace 是 nil 就不記。
func (d *DOS) trace(op FileOp) {
	op.Step = d.M.Steps
	d.FileOps = append(d.FileOps, op)
}

// allocHandle 給出**最小的**空號碼。
//
// ⚠ **關掉的號碼要放回去給下一次用。** DOS 的 handle 是 PSP 裡那張
// job file table 的索引，關檔就把那一格標成空，下一次開檔拿的是
// 最小的空格。只增不重用的話，一支開開關關幾十次的程式會拿到
// 越來越大的號碼，而 MSC 的低階 I/O 拿 handle 當自己表的索引
// （表和 JFT 一樣大）——號碼一超出範圍，`fopen` 就**開成功之後
// 立刻把它關掉並回 NULL**。症狀是「開得好好的檔突然開不起來」，
// 而且發生在與號碼配置毫無關聯的地方。
func (d *DOS) allocHandle() (uint16, bool) {
	for h := uint16(5); h < d.MaxHandles; h++ { // 0–4 是標準 handle
		if _, taken := d.handles[h]; !taken {
			return h, true
		}
	}
	return 0, false
}
