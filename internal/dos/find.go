package dos

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 目錄搜尋（`AH=4Eh` Find First、`AH=4Fh` Find Next）。
//
// ⚠ **搜尋狀態存在 DTA 裡，不是存在服務層裡。** 程式會在兩次呼叫之間換 DTA
// （`AH=1Ah`）來同時跑兩個搜尋——安裝程式一邊掃來源目錄一邊掃目的目錄就是
// 這個形狀。狀態放服務層的話第二個搜尋會把第一個的進度洗掉，而症狀是
// 「檔案列表少了一半」或「同一個檔被處理兩次」，看起來像程式自己的迴圈寫錯。
// DOSBox-X 也是把搜尋編號放進 DTA（`DOS_DTA::SetDirID`／`GetDirID`）。
//
// DTA 的版面（`AH=4Eh` 成功之後）：
//
//	+00h..+14h  DOS 保留（21 bytes）——我們放搜尋編號與下一個索引
//	+15h        屬性
//	+16h        時間（DOS 打包格式）
//	+18h        日期
//	+1Ah        檔案長度（dword）
//	+1Eh        檔名（13 bytes，ASCIIZ）

// findState 是一次搜尋的結果清單與進度。
type findState struct {
	names []string // 已經排序的 basename（大寫）
	paths []string // 對應的實際路徑
}

// dtaFindMagic 標出 DTA 的保留區是我們寫的。
//
// 沒有這個標記的話，程式把 DTA 指到一段沒清過的記憶體再叫 `AH=4Fh`，
// 我們會拿垃圾當搜尋編號——查到的可能是**別的搜尋**的下一筆。
const dtaFindMagic = 0xD0

// findFirst 是 `AH=4Eh`：DS:DX ＝ 樣式（可含 `*`／`?`），CX ＝ 屬性遮罩。
func (d *DOS) findFirst(c *cpu.CPU) {
	pattern := d.readCString(c.Seg[cpu.DS], c.R[cpu.DX], 260)
	st := d.searchFor(pattern)
	if len(st.names) == 0 {
		d.Missing = append(d.Missing, pattern)
		d.fail(c, 18) // No more files
		return
	}
	id := d.nextFind
	d.nextFind++
	if d.finds == nil {
		d.finds = map[uint16]*findState{}
	}
	d.finds[id] = st
	d.emitFind(c, id, 0)
}

// findNext 是 `AH=4Fh`：接著上一次的位置。
func (d *DOS) findNext(c *cpu.CPU) {
	base := cpu.Addr(d.dtaSeg, d.dtaOff)
	if d.M.Read8(base) != dtaFindMagic {
		// DTA 裡沒有我們的搜尋狀態。**回「沒有更多檔案」而不是隨便挑一個**：
		// 挑一個的話程式會拿到一個它沒搜尋過的檔名。
		d.fail(c, 18)
		return
	}
	id := d.M.Read16(base + 1)
	idx := d.M.Read16(base + 3)
	if d.finds[id] == nil {
		d.fail(c, 18)
		return
	}
	d.emitFind(c, id, idx)
}

// emitFind 把第 idx 筆寫進 DTA，並把下一個索引記回去。
func (d *DOS) emitFind(c *cpu.CPU, id, idx uint16) {
	st := d.finds[id]
	if st == nil || int(idx) >= len(st.names) {
		d.fail(c, 18)
		return
	}
	info, err := os.Stat(st.paths[idx])
	if err != nil {
		d.fail(c, 18)
		return
	}

	base := cpu.Addr(d.dtaSeg, d.dtaOff)
	d.M.WriteBytes(base, make([]byte, 43))
	d.M.Write8(base, dtaFindMagic)
	d.M.Write16(base+1, id)
	d.M.Write16(base+3, idx+1)

	d.M.Write8(base+0x15, 0x20) // archive
	t, dt := dosDateTime(info)
	d.M.Write16(base+0x16, t)
	d.M.Write16(base+0x18, dt)
	size := uint32(info.Size())
	d.M.Write16(base+0x1A, uint16(size))
	d.M.Write16(base+0x1C, uint16(size>>16))
	name := st.names[idx]
	if len(name) > 12 {
		name = name[:12]
	}
	d.M.WriteBytes(base+0x1E, append([]byte(name), 0))

	c.R[cpu.AX] = 0
	clearCarry(c)
}

// searchFor 列出符合樣式的檔案。暫存層蓋過原版目錄（同名只留暫存層那一份）。
//
// **順序要固定**：`os.ReadDir` 已經照名字排序，我們再排一次大寫後的名字，
// 讓「同一份輸入每次得到同一個順序」在不同檔案系統上都成立——
// 順序不固定的話，同一支程式兩次跑會處理到不同的檔，而對拍會歸咎到別處。
func (d *DOS) searchFor(pattern string) *findState {
	pat := strings.ToUpper(baseName(pattern))
	seen := map[string]string{}
	for _, dir := range []string{d.Root, d.Scratch} {
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			up := strings.ToUpper(e.Name())
			if !matchDOS(pat, up) {
				continue
			}
			seen[up] = filepath.Join(dir, e.Name())
		}
	}
	st := &findState{}
	for name := range seen {
		st.names = append(st.names, name)
	}
	sort.Strings(st.names)
	for _, n := range st.names {
		st.paths = append(st.paths, seen[n])
	}
	return st
}

// matchDOS 是 FAT 的樣式比對：`*` 吃掉該欄剩下的字元，`?` 吃一個（可以是沒有），
// 而磁碟上的名字**先截成 8.3 再比**（主檔名 8、副檔名 3）。
//
// 截斷是必要的：FAT 上不存在四個字元的副檔名，`A.PAKX` 在真 DOS 眼裡就叫
// `A.PAK`。玩家的目錄是現代檔案系統，放得下長名字——不截的話同一個檔
// 用 `AH=3Dh` 開得起來（`resolve` 有截）卻在 `AH=4Eh` 的列表裡消失。
//
// ⚠ **主檔名與副檔名分開比。** `*` 只吃自己那一欄，`?` 也只佔一格；
// 照一般的 glob 寫成「吃到底」的話 `A?.DAT` 會把 `AB1.DAT` 收進來，
// 而多收的那些在程式眼裡是合法的檔名，它會照樣去開。
func matchDOS(pattern, name string) bool {
	pn, pe := splitDOSName(pattern)
	nn, ne := splitDOSName(name)
	return matchField(pn, nn, 8) && matchField(pe, ne, 3)
}

func splitDOSName(s string) (name, ext string) {
	if i := strings.LastIndex(s, "."); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

// matchField 比一欄（主檔名或副檔名）。
func matchField(pat, s string, width int) bool {
	// `*` 之後的字元在 FAT 上會被忽略：DOS 把樣式展開成固定寬度的
	// `?` 之後再比，所以 `A*B.TXT` 與 `A*.TXT` 等價。
	expanded := make([]byte, 0, width)
	for i := 0; i < len(pat) && len(expanded) < width; i++ {
		if pat[i] == '*' {
			for len(expanded) < width {
				expanded = append(expanded, '?')
			}
			break
		}
		expanded = append(expanded, pat[i])
	}
	if len(s) > width {
		s = s[:width]
	}
	for i := 0; i < len(expanded); i++ {
		if expanded[i] == '?' {
			continue // `?` 也吃「這裡沒有字元」
		}
		if i >= len(s) || expanded[i] != s[i] {
			return false
		}
	}
	// 樣式比名字短：多出來的字元不算相符（`A.TXT` 不該收 `AB.TXT`）。
	return len(s) <= len(expanded)
}

// dosDateTime 把檔案時間打包成 DOS 的兩個 word。
//
// 時間：`時<<11 | 分<<5 | 秒/2`；日期：`(年−1980)<<9 | 月<<5 | 日`。
// **要真的填**：程式拿它比新舊（存檔選單、增量安裝），全 0 會讓每一個檔
// 看起來都是 1980-01-01，於是「最新的存檔」永遠挑到同一個。
func dosDateTime(info os.FileInfo) (t, date uint16) {
	m := info.ModTime()
	t = uint16(m.Hour())<<11 | uint16(m.Minute())<<5 | uint16(m.Second()/2)
	year := m.Year() - 1980
	if year < 0 {
		year = 0
	}
	date = uint16(year)<<9 | uint16(int(m.Month()))<<5 | uint16(m.Day())
	return t, date
}
