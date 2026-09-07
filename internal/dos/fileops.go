package dos

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 補齊的檔案／目錄服務：`AH=39h`–`3Bh`、`45h`／`46h`、`56h`、`57h`、`59h`、
// `5Bh`、`68h`。挑這幾支的理由見
// `docs/knowledge-base/010-dos-int21-coverage.md` 的 P1 那一節：
// 它們一對一對應到 C 標準函式庫的常用函式，缺了會在很平常的操作上失敗。

// fail 設 CF、把錯誤碼放進 AX，並記成「最近一次錯誤」給 `AH=59h`。
//
// **每一條失敗路徑都要走它。** 只在部分路徑記的話，`AH=59h` 回的是
// 更早之前那一次的原因——而呼叫端會照那個原因決定下一步。
func (d *DOS) fail(c *cpu.CPU, code uint16) {
	d.lastErr = code
	c.R[cpu.AX] = code
	setCarry(c)
}

// extendedError 是 `AH=59h`：AX ＝ 錯誤碼、BH ＝ 類別、BL ＝ 建議動作、
// CH ＝ 發生位置。
//
// 類別與建議動作的值照 DOS 3.0 的定義；程式多半只看 AX，但**看 BH 的那些
// 會據此決定要不要重試**，回 0 會被讀成「沒有錯誤」。
func (d *DOS) extendedError(c *cpu.CPU) {
	code := d.lastErr
	c.R[cpu.AX] = code
	var class, action, locus uint8
	switch code {
	case 0:
		class, action, locus = 0, 0, 0
	case 2, 3, 18: // 找不到檔／路徑、沒有更多檔案
		class, action, locus = 0x08, 0x03, 0x02 // not found／使用者重輸入／區塊裝置
	case 4: // 開太多檔
		class, action, locus = 0x01, 0x04, 0x01 // 資源用盡／稍後重試
	case 5: // 拒絕存取
		class, action, locus = 0x03, 0x04, 0x02
	case 6: // 無效 handle
		class, action, locus = 0x07, 0x07, 0x01 // 程式錯誤／立刻放棄
	case 8: // 記憶體不足
		class, action, locus = 0x01, 0x04, 0x05 // 資源用盡／記憶體
	default:
		class, action, locus = 0x0D, 0x07, 0x01 // 不明／立刻放棄
	}
	c.R[cpu.BX] = uint16(class)<<8 | uint16(action)
	c.R[cpu.CX] = uint16(locus) << 8
	clearCarry(c)
}

// dupHandle 是 `AH=45h`：複製一個 handle 號碼，**共用同一個檔案指標**。
//
// 共用是重點：`dup` 出來的號碼與本尊 seek 到同一個位置。各自一份的話，
// 程式對複本 seek 之後從本尊讀，讀到的是舊位置——而它剛剛才成功複製過，
// 完全不會懷疑這裡。
func (d *DOS) dupHandle(c *cpu.CPU) {
	h, ok := d.handles[c.R[cpu.BX]]
	if !ok {
		d.trace(FileOp{Op: "dup", Fn: 0x45, Handle: c.R[cpu.BX], Failed: true})
		d.fail(c, 6)
		return
	}
	num, ok := d.allocHandle()
	if !ok {
		d.trace(FileOp{Op: "dup", Fn: 0x45, Handle: c.R[cpu.BX], Failed: true})
		d.fail(c, 4)
		return
	}
	h.refs++
	d.handles[num] = h
	d.trace(FileOp{Op: "dup", Fn: 0x45, Handle: num, Name: h.name, Arg: int64(c.R[cpu.BX])})
	c.R[cpu.AX] = num
	clearCarry(c)
}

// dup2Handle 是 `AH=46h`：把 CX 這個號碼指到 BX 那一份（原本開著的先關掉）。
//
// 這是 `dup2`／重導向的作法：程式把 stdout（號碼 1）指到一個檔上。
func (d *DOS) dup2Handle(c *cpu.CPU) {
	src, ok := d.handles[c.R[cpu.BX]]
	if !ok {
		d.fail(c, 6)
		return
	}
	dst := c.R[cpu.CX]
	if dst >= d.MaxHandles {
		d.fail(c, 6)
		return
	}
	if old, exists := d.handles[dst]; exists {
		d.releaseHandle(dst, old)
	}
	src.refs++
	d.handles[dst] = src
	d.trace(FileOp{Op: "dup2", Fn: 0x46, Handle: dst, Name: src.name, Arg: int64(c.R[cpu.BX])})
	clearCarry(c)
}

// fileTime 是 `AH=57h`：AL=00 取、AL=01 設檔案的日期時間。
//
// 取的那一支要真的填：程式拿它比新舊（存檔選單挑最新的、安裝程式跳過
// 較新的檔），全 0 會讓每一個檔看起來都是 1980-01-01。
// 設的那一支不落地（素材唯讀），記一筆再回成功——與 `AH=43h AL=01h` 同一原則。
func (d *DOS) fileTime(c *cpu.CPU) {
	h, ok := d.handles[c.R[cpu.BX]]
	if !ok {
		d.fail(c, 6)
		return
	}
	switch al(c) {
	case 0x00:
		if h.path == "" {
			// 字元裝置沒有時間。回 0 而不是失敗：程式問的是「這個 handle
			// 的時間」，對裝置而言那本來就沒有意義。
			c.R[cpu.CX], c.R[cpu.DX] = 0, 0
			clearCarry(c)
			return
		}
		info, err := os.Stat(h.path)
		if err != nil {
			d.fail(c, 5)
			return
		}
		t, date := dosDateTime(info)
		c.R[cpu.CX], c.R[cpu.DX] = t, date
		clearCarry(c)
	case 0x01:
		d.note(0x21, 0x57, 0x01)
		clearCarry(c)
	default:
		d.note(0x21, 0x57, al(c))
		d.fail(c, 1) // 無效功能
	}
}

// createNew 是 `AH=5Bh`：與 `AH=3Ch` 一樣建檔，但**檔案已存在就失敗**。
//
// 差別有意義：存檔流程用它避免覆蓋既有存檔。當成 `3Ch` 處理的話，
// 程式以為自己建了新檔，實際上把舊的截斷了。
func (d *DOS) createNew(c *cpu.CPU) {
	name := d.readCString(c.Seg[cpu.DS], c.R[cpu.DX], 128)
	if d.resolve(name) != "" {
		d.fail(c, 80) // 檔案已存在
		return
	}
	d.create(c)
}

// commitFile 是 `AH=68h`／`6Ah`：把緩衝寫回磁碟。
//
// 沒有暫存層時我們本來就不落地，回成功即可；有的話真的 Sync——
// 程式在 `fflush` 之後假設資料已經在磁碟上，接著可能就去讀它。
func (d *DOS) commitFile(c *cpu.CPU) {
	h, ok := d.handles[c.R[cpu.BX]]
	if !ok {
		d.fail(c, 6)
		return
	}
	if h.f != nil && h.writable {
		if err := h.f.Sync(); err != nil {
			d.fail(c, 5)
			return
		}
	}
	d.trace(FileOp{Op: "commit", Fn: 0x68, Handle: c.R[cpu.BX], Name: h.name})
	clearCarry(c)
}

// renameFile 是 `AH=56h`：DS:DX ＝ 舊名、ES:DI ＝ 新名。
//
// **只動暫存層。** 沒有暫存層時回「拒絕存取」而不是假裝成功：
// 假裝成功之後程式會去開新名字，而那個檔不存在——它會把「開不起來」
// 歸咎到別的地方。唯讀媒體上真 DOS 回的也是這個碼。
func (d *DOS) renameFile(c *cpu.CPU) {
	oldName := d.readCString(c.Seg[cpu.DS], c.R[cpu.DX], 128)
	newName := d.readCString(c.Seg[cpu.ES], c.R[cpu.DI], 128)
	if d.Scratch == "" {
		d.note(0x21, 0x56, 0)
		d.fail(c, 5) // 拒絕存取
		return
	}
	src := d.resolve(oldName)
	if src == "" {
		d.Missing = append(d.Missing, oldName)
		d.fail(c, 2)
		return
	}
	dst := filepath.Join(d.Scratch, strings.ToUpper(baseName(newName)))
	if !strings.HasPrefix(src, d.Scratch) {
		// 來源還在原版目錄：先複製進暫存層再改名，原版目錄不動。
		data, err := os.ReadFile(src)
		if err != nil {
			d.fail(c, 5)
			return
		}
		if err := os.MkdirAll(d.Scratch, 0o755); err != nil {
			d.fail(c, 3)
			return
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			d.fail(c, 5)
			return
		}
		d.trace(FileOp{Op: "rename", Fn: 0x56, Name: oldName, Arg: int64(len(data))})
		clearCarry(c)
		return
	}
	if err := os.Rename(src, dst); err != nil {
		d.fail(c, 5)
		return
	}
	d.trace(FileOp{Op: "rename", Fn: 0x56, Name: oldName})
	clearCarry(c)
}

// mkdir／rmdir 是 `AH=39h`／`3Ah`：**只動暫存層**，原版目錄永遠不碰。
//
// 沒有暫存層時記一筆再回成功：存檔目錄建不起來會讓程式在寫檔之前就放棄，
// 而寫檔本來就只記帳不落地——回失敗等於把整條路提早斬斷。
func (d *DOS) mkdir(c *cpu.CPU) {
	name := d.readCString(c.Seg[cpu.DS], c.R[cpu.DX], 128)
	if d.Scratch == "" {
		d.note(0x21, 0x39, 0)
		clearCarry(c)
		return
	}
	if err := os.MkdirAll(filepath.Join(d.Scratch, strings.ToUpper(baseName(name))), 0o755); err != nil {
		d.fail(c, 3) // 路徑找不到
		return
	}
	clearCarry(c)
}

func (d *DOS) rmdir(c *cpu.CPU) {
	name := d.readCString(c.Seg[cpu.DS], c.R[cpu.DX], 128)
	if d.Scratch == "" {
		d.note(0x21, 0x3A, 0)
		clearCarry(c)
		return
	}
	if err := os.Remove(filepath.Join(d.Scratch, strings.ToUpper(baseName(name)))); err != nil {
		d.fail(c, 3)
		return
	}
	clearCarry(c)
}

// chdir 是 `AH=3Bh`。我們只認 basename，所以目錄只是一個字串——
// 但 `AH=47h`（取目前目錄）要回得出來，兩支必須一致。
//
// **不驗證目錄存不存在**：我們的 Root 是攤平的，程式切到 `\SAVE` 之後
// 開的檔仍然照 basename 解析。回失敗會讓程式以為安裝不完整。
func (d *DOS) chdir(c *cpu.CPU) {
	path := d.readCString(c.Seg[cpu.DS], c.R[cpu.DX], 128)
	path = strings.TrimPrefix(strings.ToUpper(path), "C:")
	path = strings.TrimPrefix(path, `\`)
	d.Dir = path
	d.note(0x21, 0x3B, 0) // 記一筆：目錄是假的，踩到的人要看得到
	clearCarry(c)
}

// bufferedInput 是 `AH=0Ah`：讀一整行進 DS:DX 的緩衝區。
//
// 版面（DOS 定的）：
//
//	[0]  呼叫端填的容量（含結尾的 CR）
//	[1]  我們填的實際字元數（**不含** CR）
//	[2…] 內容，結尾補一個 CR（0Dh）
//
// ⚠ **位元組 1 一定要填。** 呼叫端讀它決定要處理幾個字元；不填的話它讀到的是
// 自己上一次留下的值——多半是 0（看起來像使用者直接按了 Enter），
// 或一個過大的數字（於是它把緩衝區後面的垃圾當成輸入）。
//
// 佇列空的時候回「空的一行」而不是阻塞：這一層沒有排程器，
// 而阻塞式的 `AH=01h` 已經有 `Blocked` 那條路（`docs/spec/008`）。
func (d *DOS) bufferedInput(c *cpu.CPU) {
	base := cpu.Addr(c.Seg[cpu.DS], c.R[cpu.DX])
	maxLen := int(d.M.Read8(base))
	if maxLen == 0 {
		clearCarry(c)
		return
	}
	line := make([]byte, 0, maxLen)
	for len(line) < maxLen-1 && len(d.Stdin) > 0 {
		ch := d.Stdin[0]
		d.Stdin = d.Stdin[1:]
		if ch == '\r' || ch == '\n' {
			break
		}
		line = append(line, ch)
		d.noteKey("int21-AH0A", ch)
	}
	d.M.Write8(base+1, uint8(len(line)))
	d.M.WriteBytes(base+2, append(line, '\r'))
	// 回顯：真 DOS 的 AH=0Ah 是有回顯的，而 Console 是我們看得到程式
	// 「收到什麼」的唯一管道。
	d.Console = append(d.Console, line...)
	d.Console = append(d.Console, '\r', '\n')
	clearCarry(c)
}
