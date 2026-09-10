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
	// writable 為真時 `AH=40h` 真的寫下去（暫存層或 AllowFileWrites）。
	writable bool
	// refs 是有幾個 handle 號碼指著這一份。
	//
	// `AH=45h`／`46h` 複製出來的號碼**共用同一個檔案指標**（真 DOS 的 JFT
	// 兩項指向同一個 SFT），所以關掉其中一個不能關檔——關了的話，
	// 程式對另一個號碼讀會拿到「無效 handle」，而它剛剛才成功複製過。
	refs int
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
	// 先試完整路徑（`docs/spec/195`）：資料真的放在子目錄的遊戲
	// （《Dungeon Master》的 `data\GRAPHICS.DAT`）只有這條路走得通。
	// **順序不能顛倒**——先試 basename 的話，頂層剛好有同名檔時
	// 會開到錯的那一份。
	if segs := dosPathSegments(name); len(segs) > 1 {
		if d.Scratch != "" {
			if p := lookupPath(d.Scratch, segs); p != "" {
				return p
			}
		}
		if p := lookupPath(d.Root, segs); p != "" {
			return p
		}
	}
	base := baseName(name)
	if base == "" {
		return ""
	}
	// 暫存層蓋過原版目錄（`docs/spec/009` §2.2.1）：程式存過的東西
	// 下一次要讀得到自己寫的那一份，不是原版那一份。
	if d.Scratch != "" {
		if p := lookupDOS(d.Scratch, base); p != "" {
			return p
		}
	}
	return lookupDOS(d.Root, base)
}

// dosPathSegments 把 DOS 路徑拆成一段一段，回 nil 表示這條路徑不能走
// 完整解析（`docs/spec/195` §2）。
//
// ⚠ **`..` 一律讓整條失敗。** 讓遊戲組出來的路徑走出 Root 等於把模擬器
// 當檔案總管；回 nil 之後呼叫端退回 basename，那條路只在 Root 裡找。
func dosPathSegments(name string) []string {
	// 磁碟機代號丟掉：我們只有一個 Root，`C:` 與 `A:` 都對到它。
	if i := strings.Index(name, ":"); i >= 0 {
		name = name[i+1:]
	}
	name = strings.ReplaceAll(name, `\`, "/")
	var segs []string
	for _, s := range strings.Split(name, "/") {
		switch s {
		case "", ".":
			// 開頭的分隔符與空的一段都略過：真 DOS 的絕對路徑
			// 相對於磁碟機根目錄，而 Root 就是那個根目錄。
		case "..":
			return nil
		default:
			segs = append(segs, s)
		}
	}
	return segs
}

// lookupPath 沿著 segs 逐段大小寫不分地解析。中間段必須是目錄、
// 最後一段必須是檔案。
//
// **中間段不做 8.3 截斷**：截斷是 FAT 對檔名的限制，對目錄名多猜一次
// 只會讓「找到了但是別的東西」變得可能。
func lookupPath(dir string, segs []string) string {
	for i, s := range segs {
		last := i == len(segs)-1
		next := lookupEntry(dir, s, !last)
		if next == "" {
			if !last {
				return ""
			}
			// 最後一段照既有規則再試 8.3 截斷過的名字。
			return lookupDOS(dir, s)
		}
		dir = next
	}
	return dir
}

// lookupEntry 在 dir 裡找一個名字，wantDir 決定要目錄還是要檔案。
func lookupEntry(dir, name string, wantDir bool) string {
	direct := filepath.Join(dir, name)
	if st, err := os.Stat(direct); err == nil && st.IsDir() == wantDir {
		return direct
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() != wantDir {
			continue
		}
		if strings.EqualFold(e.Name(), name) {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

// lookupDOS 在一個目錄裡找 basename，找不到再用 8.3 截斷過的名字找一次。
func lookupDOS(dir, base string) string {
	if p := lookup(dir, base); p != "" {
		return p
	}
	if short := dosName(base); short != base {
		return lookup(dir, short)
	}
	return ""
}

// lookup 在一個目錄裡找 basename，先直接查再大小寫不分地掃一遍。
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
// 這種路徑，目錄部分對不上實際安裝（見 resolve 的註解）。
func baseName(name string) string {
	base := name
	if i := strings.LastIndexAny(base, `\/:`); i >= 0 {
		base = base[i+1:]
	}
	return base
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
	// 字元裝置：開 EMMXXXX0 成功 ＝ EMS 驅動存在（`docs/spec/007` §5）。
	// launcher 開完就關，不讀不寫；讀寫語意沒有證據，讀回 EOF、寫丟棄。
	if isEMMDevice(name) {
		h, ok := d.allocHandle()
		if !ok {
			c.R[cpu.AX] = 4 // Too many open files
			setCarry(c)
			return
		}
		d.handles[h] = &handle{name: name, psp: d.curPSP, refs: 1}
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
		d.noteMissingAccess(c, name)
		c.R[cpu.AX] = 2 // File not found
		setCarry(c)
		return
	}
	// 兩條可寫的路：**有暫存層就寫時複製到暫存層**（原版目錄永遠不動），
	// 沒有暫存層才回頭看 AllowFileWrites 的逐檔白名單（就地寫）。
	// 順序不能反——白名單優先的話，設了暫存層的人還是會寫到原版目錄裡。
	writeAccess := al(c)&3 == 1 || al(c)&3 == 2
	allowed := writeAccess && (d.Scratch != "" ||
		d.writableFiles[strings.ToUpper(filepath.Base(path))])
	var f *os.File
	var err error
	if allowed && d.Scratch != "" {
		path, err = d.scratchCopy(name, path)
		if err == nil {
			f, err = os.OpenFile(path, os.O_RDWR, 0o644)
		}
	} else if allowed {
		f, err = os.OpenFile(path, os.O_RDWR, 0)
	} else {
		f, err = os.Open(path)
	}
	if err != nil {
		d.Missing = append(d.Missing, name)
		d.noteMissingAccess(c, name)
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
	d.handles[h] = &handle{name: name, path: path, f: f, size: st.Size(),
		psp: d.curPSP, writable: allowed, refs: 1}
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

func (d *DOS) noteMissingAccess(c *cpu.CPU, name string) {
	access := FileAccess{Name: name, CS: c.Seg[cpu.CS], IP: c.IP, DS: c.Seg[cpu.DS], DX: c.R[cpu.DX], SS: c.Seg[cpu.SS], BP: c.R[cpu.BP]}
	bp := access.BP
	for i := range access.Callers {
		if bp == 0 {
			break
		}
		frame := StackFrame{BP: bp, IP: d.M.Read16(cpu.Addr(access.SS, bp+2)), CS: d.M.Read16(cpu.Addr(access.SS, bp+4))}
		for j := range frame.Code {
			frame.Code[j] = d.M.Read8(cpu.Addr(frame.CS, frame.IP-8+uint16(j)))
		}
		for j := range frame.Args {
			frame.Args[j] = d.M.Read16(cpu.Addr(access.SS, bp+6+uint16(j*2)))
		}
		access.Callers[i] = frame
		next := d.M.Read16(cpu.Addr(access.SS, bp))
		if next <= bp {
			break
		}
		bp = next
	}
	d.MissingAccess = append(d.MissingAccess, access)
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
		h, ok := d.allocHandle()
		if !ok {
			c.R[cpu.AX] = 4 // Too many open files
			setCarry(c)
			return
		}
		d.handles[h] = &handle{name: name, psp: d.curPSP, refs: 1}
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
	h, ok := d.allocHandle()
	if !ok {
		f.Close()
		c.R[cpu.AX] = 4 // Too many open files
		setCarry(c)
		return
	}
	d.handles[h] = &handle{name: name, path: path, f: f, psp: d.curPSP, writable: true, refs: 1}
	d.trace(FileOp{Op: "create", Fn: 0x3C, Handle: h, Name: name})
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
		d.trace(FileOp{Op: "close", Fn: 0x3E, Handle: c.R[cpu.BX], Failed: true})
		c.R[cpu.AX] = 6 // Invalid handle
		setCarry(c)
		return
	}
	d.trace(FileOp{Op: "close", Fn: 0x3E, Handle: c.R[cpu.BX], Name: h.name})
	d.releaseHandle(c.R[cpu.BX], h)
	clearCarry(c)
}

// releaseHandle 放掉一個號碼；最後一個放掉的才真的關檔。
func (d *DOS) releaseHandle(num uint16, h *handle) {
	delete(d.handles, num)
	h.refs--
	if h.refs <= 0 && h.f != nil {
		h.f.Close()
		h.f = nil
	}
}

func (d *DOS) read(c *cpu.CPU) {
	bx, cx := c.R[cpu.BX], c.R[cpu.CX]
	if bx == 0 {
		d.readStdin(c, cx)
		return
	}
	h, ok := d.handles[bx]
	if !ok {
		d.trace(FileOp{Op: "read", Fn: 0x3F, Handle: bx, Arg: int64(cx), Failed: true})
		c.R[cpu.AX] = 6
		setCarry(c)
		return
	}
	if h.f == nil { // 字元裝置（EMMXXXX0）：讀回 EOF
		d.trace(FileOp{Op: "read", Fn: 0x3F, Handle: bx, Name: h.name, Arg: int64(cx)})
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
		d.trace(FileOp{Op: "seek", Fn: 0x42, Handle: c.R[cpu.BX],
			Whence: al(c), Failed: true})
		c.R[cpu.AX] = 6
		setCarry(c)
		return
	}
	if h.f == nil { // 字元裝置不能 seek
		d.trace(FileOp{Op: "seek", Fn: 0x42, Handle: c.R[cpu.BX], Name: h.name,
			Whence: al(c), Failed: true})
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
		d.trace(FileOp{Op: "seek", Fn: 0x42, Handle: c.R[cpu.BX], Name: h.name,
			Whence: al(c), Arg: off, Failed: true})
		c.R[cpu.AX] = 1
		setCarry(c)
		return
	}
	d.trace(FileOp{Op: "seek", Fn: 0x42, Handle: c.R[cpu.BX],
		Name: h.name, Whence: al(c), Arg: off, Pos: pos})
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
// **原版目錄永遠不寫。** 有 `Scratch` 時寫進暫存層（`docs/spec/009`）；
// 沒有暫存層時只有 AllowFileWrites 逐檔允許的 handle 會落地，其餘只記一筆
// 再回報成功——安靜地失敗會讓「存檔壞掉」完全查不出來。
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
