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
}

// resolve 把遊戲組出來的路徑對到實際檔案。
//
// ⚠ **只認 basename。** 遊戲會自己組出 `A:\<垃圾>\DATA.PAK` 這種路徑
// （多磁片版的殘留），目錄部分對不上硬碟安裝版
// （`rich2/docs/re/006` §5：檔名是執行期組出來的，所以靜態找不到引用）。
//
// **大小寫不分**：原版是 DOS，檔名全大寫；玩家的目錄可能是小寫。
//
// **超過 8.3 的名字會截斷**，因為 FAT 就是這樣：程式傳 `steedpics` 進來，
// 真 DOS 開到的是 `STEEDPIC`。不截的話這裡回「找不到檔」，而程式多半不檢查
// 開檔結果——KOL 就是一路跑進沒有映射的記憶體才停，**看起來像模擬器的 bug，
// 其實是檔名沒對上**。
func (d *DOS) resolve(name string) string {
	base := name
	if i := strings.LastIndexAny(base, `\/:`); i >= 0 {
		base = base[i+1:]
	}
	if base == "" {
		return ""
	}
	if p := d.lookup(base); p != "" {
		return p
	}
	if short := dosName(base); short != base {
		return d.lookup(short)
	}
	return ""
}

// lookup 在遊戲目錄裡找一個檔，先直接查再大小寫不分地掃一遍。
func (d *DOS) lookup(base string) string {
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

// dosName 把檔名截成 FAT 的 8.3：主檔名留前 8 個字元，副檔名留前 3 個。
//
// 開頭的 `.` 不當成副檔名分隔（DOS 沒有隱藏檔那套，但切出空的主檔名只會更糟）。
func dosName(base string) string {
	name, ext := base, ""
	if i := strings.LastIndex(base, "."); i > 0 {
		name, ext = base[:i], base[i+1:]
	}
	if len(name) > 8 {
		name = name[:8]
	}
	if len(ext) > 3 {
		ext = ext[:3]
	}
	if ext == "" {
		return name
	}
	return name + "." + ext
}

// maxHandles 是一個程式同時開得起的檔案數。真 DOS 預設 20（`FILES=` 再多也
// 一樣，那管的是系統表；程式自己的 JFT 是 20 項，要更多得用 `AH=67h`）。
const maxHandles = 20

// allocHandle 找第一個沒人用的 handle 號碼。
//
// ⚠ **號碼一定要重用。** 真 DOS 從 JFT 挑第一個空位，所以「開→關→開」拿到的
// 是同一個小號碼。只增不減的話，開關幾十次之後 handle 就爬到 20 以上——
// 而遊戲常拿 handle 當自己那張表的索引，越界之後讀到別人的欄位。
//
// 症狀完全不像 handle 的問題：《武士傳說》的表存著「這個 handle 上次 seek 到
// 哪」，越界之後新開的檔繼承了別人的位置，於是**該發的 `AH=42h` 整個沒發**，
// 解壓器從檔案開頭讀到目錄本身，最後停在走不完的字典鏈裡。
func (d *DOS) allocHandle() uint16 {
	for h := uint16(5); h < maxHandles; h++ {
		if _, ok := d.handles[h]; !ok {
			return h
		}
	}
	return 0
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
		h := d.allocHandle()
		if h == 0 {
			c.R[cpu.AX] = 4 // Too many open files
			setCarry(c)
			return
		}
		d.handles[h] = &handle{name: name}
		d.Opened = append(d.Opened, name)
		c.R[cpu.AX] = h
		clearCarry(c)
		return
	}
	path := d.resolve(name)
	if path == "" {
		d.Missing = append(d.Missing, name)
		d.Reads = append(d.Reads, FileOp{Kind: "open", Name: name,
			Failed: true, Step: d.M.Steps})
		c.R[cpu.AX] = 2 // File not found
		setCarry(c)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		d.Missing = append(d.Missing, name)
		d.Reads = append(d.Reads, FileOp{Kind: "open", Name: name,
			Failed: true, Step: d.M.Steps})
		c.R[cpu.AX] = 2
		setCarry(c)
		return
	}
	st, _ := f.Stat()
	h := d.allocHandle()
	if h == 0 {
		f.Close()
		d.Reads = append(d.Reads, FileOp{Kind: "open", Name: name,
			Failed: true, Step: d.M.Steps})
		c.R[cpu.AX] = 4 // Too many open files
		setCarry(c)
		return
	}
	d.handles[h] = &handle{name: name, path: path, f: f, size: st.Size()}
	d.Opened = append(d.Opened, filepath.Base(path))
	d.Reads = append(d.Reads, FileOp{Kind: "open", Name: name, Handle: h,
		Got: int(st.Size()), Step: d.M.Steps})
	c.R[cpu.AX] = h
	clearCarry(c)
}

func (d *DOS) close(c *cpu.CPU) {
	h, ok := d.handles[c.R[cpu.BX]]
	if !ok {
		d.Reads = append(d.Reads, FileOp{Kind: "close", Handle: c.R[cpu.BX],
			Failed: true, Step: d.M.Steps})
		c.R[cpu.AX] = 6 // Invalid handle
		setCarry(c)
		return
	}
	d.Reads = append(d.Reads, FileOp{Kind: "close", Name: h.name,
		Handle: c.R[cpu.BX], Step: d.M.Steps})
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
		d.Reads = append(d.Reads, FileOp{Kind: "read", Handle: bx,
			Want: int(cx), Failed: true, Step: d.M.Steps})
		c.R[cpu.AX] = 6
		setCarry(c)
		return
	}
	if h.f == nil { // 字元裝置（EMMXXXX0）：讀回 EOF
		d.Reads = append(d.Reads, FileOp{Kind: "read", Name: h.name, Handle: bx,
			Want: int(cx), Step: d.M.Steps})
		c.R[cpu.AX] = 0
		clearCarry(c)
		return
	}
	at, _ := h.f.Seek(0, 1) // 讀之前的位置，記進診斷
	buf := make([]byte, cx)
	n, _ := h.f.Read(buf)
	if n < 0 {
		n = 0
	}
	into := cpu.Addr(c.Seg[cpu.DS], c.R[cpu.DX])
	d.M.WriteBytes(into, buf[:n])
	d.Reads = append(d.Reads, FileOp{Kind: "read", Name: h.name, Handle: bx,
		Offset: at, Want: int(cx), Got: n, Into: into, Step: d.M.Steps})
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
		d.Reads = append(d.Reads, FileOp{Kind: "seek", Handle: c.R[cpu.BX],
			Whence: int(al(c)), Failed: true, Step: d.M.Steps})
		c.R[cpu.AX] = 6
		setCarry(c)
		return
	}
	if h.f == nil { // 字元裝置不能 seek
		d.Reads = append(d.Reads, FileOp{Kind: "seek", Name: h.name,
			Handle: c.R[cpu.BX], Whence: int(al(c)), Failed: true, Step: d.M.Steps})
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
	d.Reads = append(d.Reads, FileOp{Kind: "seek", Name: h.name,
		Handle: c.R[cpu.BX], Whence: int(al(c)), Offset: off,
		Got: int(pos), Step: d.M.Steps})
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

// FileOp 是一次 seek 或 read 的紀錄（診斷用，`DOS.Reads`）。
//
// Kind 是 "open"、"close"、"seek" 或 "read"；Name 是那個 handle 開的檔名。
// seek 記 Whence 與要求的位移，read 記要求的長度、實際讀到的位元組數與落點。
//
// ⚠ **失敗的呼叫也要記**（`Failed`）。原本 handle 無效就直接返回不留紀錄，
// 於是「程式對錯的 handle seek，然後拿沒 seek 過的檔案指標去讀」這個形狀
// 在軌跡裡看起來像**根本沒呼叫 seek**——差一步就會歸咎到程式邏輯上。
type FileOp struct {
	Kind   string
	Name   string
	Handle uint16
	Whence int
	Offset int64
	Want   int
	Got    int
	Into   uint32 // read 的落點（線性位址）
	Failed bool
	Step   uint64
}
