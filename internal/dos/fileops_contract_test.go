package dos

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// 第一批補洞的契約測試（`docs/spec/184-mvp-scope-review` 批次 1）。
//
// 期望值照 DOS 的介面自己列。每一支挑進來的理由都是「對應到 C 標準函式庫的
// 常用函式」，所以測的是那個對應關係成不成立，不是「有沒有回 CF=0」。

// Find First／Next 要走完整個目錄，而且順序固定。
//
// 只有 First 沒有 Next 的話，列目錄的迴圈拿到第一個之後就結束——
// 程式會以為目錄裡只有一個檔，而那看起來像**素材沒安裝完整**。
func TestFindNextWalksTheWholeDirectory(t *testing.T) {
	m, d := newTest(t)
	for _, n := range []string{"A.PAK", "B.PAK", "C.PAK", "OTHER.DAT"} {
		writeFixture(t, d, n, "x")
	}

	putFileName(m, "*.PAK")
	call(m, d, 0x21, 0x4E00)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("Find First 失敗：AX=%d", m.CPU.R[cpu.AX])
	}
	dta := cpu.Addr(machine.PSPSeg, 0x80)
	readName := func() string {
		out := make([]byte, 0, 13)
		for i := uint32(0); i < 13; i++ {
			ch := m.Read8(dta + 0x1E + i)
			if ch == 0 {
				break
			}
			out = append(out, ch)
		}
		return string(out)
	}

	var got []string
	got = append(got, readName())
	for i := 0; i < 10; i++ {
		call(m, d, 0x21, 0x4F00)
		if m.CPU.Flags&cpu.CF != 0 {
			break
		}
		got = append(got, readName())
	}
	want := []string{"A.PAK", "B.PAK", "C.PAK"}
	if len(got) != len(want) {
		t.Fatalf("列出 %v，預期 %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 個是 %q，預期 %q（順序要固定）", i, got[i], want[i])
		}
	}
	// 走完之後要回「沒有更多檔案」（AX=18），不是繞回第一個。
	if m.CPU.R[cpu.AX] != 18 {
		t.Errorf("走完之後 AX=%d，預期 18", m.CPU.R[cpu.AX])
	}
}

// 樣式比對照 FAT 的規則：主檔名與副檔名分開比，`*` 只吃它那一欄，
// 而**磁碟上的名字先截成 8.3 再比**。
//
// 兩件事都不能照一般 glob 想：
//
//   - `*` 吃到底的話，`A?.DAT` 這種樣式的邊界會跑掉（`AB1.DAT` 會被收進來）。
//   - 截斷是必要的：FAT 上根本不存在四個字元的副檔名，`A.PAKX` 這個檔在
//     真 DOS 眼裡就叫 `A.PAK`。玩家的目錄是現代檔案系統，放得下長名字——
//     不截的話，同一個檔用 `AH=3Dh` 開得起來（`resolve` 有截）卻在
//     `AH=4Eh` 的列表裡消失，而那個不一致查起來很貴。
func TestWildcardMatchesFATRules(t *testing.T) {
	for _, c := range []struct {
		pat, name string
		want      bool
	}{
		{"*.PAK", "A.PAK", true},
		{"*.PAK", "DATA.PAK", true},
		{"*.PAK", "A.PAKX", true}, // 磁碟上的長名字截成 8.3 之後就是 A.PAK
		{"*.*", "A.PAK", true},
		{"*", "README", true},
		{"A?.DAT", "A1.DAT", true},
		{"A?.DAT", "AB1.DAT", false},
		{"A?.DAT", "A.DAT", true}, // `?` 也吃「這裡沒有字元」
		{"DATA.PAK", "DATA.PAK", true},
		{"DATA.PAK", "DATA.PAX", false},
	} {
		if got := matchDOS(c.pat, c.name); got != c.want {
			t.Errorf("matchDOS(%q, %q) = %t，預期 %t", c.pat, c.name, got, c.want)
		}
	}
}

// 兩個搜尋可以同時進行：狀態存在各自的 DTA 裡。
//
// 狀態放服務層的話，第二個搜尋會把第一個的進度洗掉——症狀是
// 「檔案列表少了一半」或「同一個檔被處理兩次」。
func TestTwoSearchesDoNotClobberEachOther(t *testing.T) {
	m, d := newTest(t)
	for _, n := range []string{"A.PAK", "B.PAK", "X.DAT", "Y.DAT"} {
		writeFixture(t, d, n, "x")
	}
	nameAt := func(seg, off uint16) string {
		out := make([]byte, 0, 13)
		for i := uint32(0); i < 13; i++ {
			ch := m.Read8(cpu.Addr(seg, off) + 0x1E + i)
			if ch == 0 {
				break
			}
			out = append(out, ch)
		}
		return string(out)
	}

	// 搜尋一：DTA 在 2000:0000
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = 0x2000, 0
	call(m, d, 0x21, 0x1A00)
	putFileName(m, "*.PAK")
	call(m, d, 0x21, 0x4E00)
	if got := nameAt(0x2000, 0); got != "A.PAK" {
		t.Fatalf("搜尋一的第一筆是 %q", got)
	}

	// 搜尋二：DTA 換到 2100:0000
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = 0x2100, 0
	call(m, d, 0x21, 0x1A00)
	putFileName(m, "*.DAT")
	call(m, d, 0x21, 0x4E00)
	if got := nameAt(0x2100, 0); got != "X.DAT" {
		t.Fatalf("搜尋二的第一筆是 %q", got)
	}

	// 回到搜尋一：下一筆該是 B.PAK，不是被搜尋二洗掉。
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = 0x2000, 0
	call(m, d, 0x21, 0x1A00)
	call(m, d, 0x21, 0x4F00)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatal("搜尋一的 Find Next 失敗——進度被第二個搜尋洗掉了")
	}
	if got := nameAt(0x2000, 0); got != "B.PAK" {
		t.Errorf("搜尋一的第二筆是 %q，預期 B.PAK", got)
	}
}

// `AH=45h` 複製出來的 handle **與本尊共用檔案指標**。
func TestDuplicatedHandleSharesTheFilePointer(t *testing.T) {
	m, d := newTest(t)
	writeFixture(t, d, "D.DAT", "0123456789")
	h := openFile(t, m, d, "D.DAT", 0)

	m.CPU.R[cpu.BX] = h
	call(m, d, 0x21, 0x4500)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("dup 失敗：AX=%d", m.CPU.R[cpu.AX])
	}
	dup := m.CPU.R[cpu.AX]
	if dup == h {
		t.Fatal("dup 回了同一個號碼")
	}

	// 對複本 seek，本尊要跟著動。
	m.CPU.R[cpu.BX], m.CPU.R[cpu.CX], m.CPU.R[cpu.DX] = dup, 0, 5
	call(m, d, 0x21, 0x4200)
	m.CPU.R[cpu.BX], m.CPU.R[cpu.CX] = h, 2
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = fileScratchSeg, 0x100
	call(m, d, 0x21, 0x3F00)
	got := []byte{
		m.Read8(cpu.Addr(fileScratchSeg, 0x100)),
		m.Read8(cpu.Addr(fileScratchSeg, 0x101)),
	}
	if string(got) != "56" {
		t.Errorf("從本尊讀到 %q，預期 56——兩個號碼沒有共用檔案指標", got)
	}

	// 關掉複本，本尊還要能用。
	m.CPU.R[cpu.BX] = dup
	call(m, d, 0x21, 0x3E00)
	m.CPU.R[cpu.BX], m.CPU.R[cpu.CX] = h, 1
	call(m, d, 0x21, 0x3F00)
	if m.CPU.Flags&cpu.CF != 0 || m.CPU.R[cpu.AX] != 1 {
		t.Errorf("關掉複本之後本尊讀不了（CF=%t AX=%d）——關檔沒有走參考計數",
			m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.AX])
	}
}

// `AH=46h`（dup2）把指定的號碼指到另一份，原本開著的那一份要先關掉。
func TestForceDuplicateRedirectsTheNumber(t *testing.T) {
	m, d := newTest(t)
	writeFixture(t, d, "E.DAT", "abcdef")
	writeFixture(t, d, "F.DAT", "ABCDEF")
	src := openFile(t, m, d, "E.DAT", 0)
	dst := openFile(t, m, d, "F.DAT", 0)

	m.CPU.R[cpu.BX], m.CPU.R[cpu.CX] = src, dst
	call(m, d, 0x21, 0x4600)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("dup2 失敗：AX=%d", m.CPU.R[cpu.AX])
	}
	// 現在從 dst 讀到的要是 E.DAT 的內容。
	m.CPU.R[cpu.BX], m.CPU.R[cpu.CX] = dst, 3
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = fileScratchSeg, 0x100
	call(m, d, 0x21, 0x3F00)
	got := make([]byte, 3)
	for i := range got {
		got[i] = m.Read8(cpu.Addr(fileScratchSeg, 0x100+uint16(i)))
	}
	if string(got) != "abc" {
		t.Errorf("重導向之後讀到 %q，預期 abc", got)
	}
}

// `AH=57h AL=00h` 要回真的檔案時間，不是全 0。
//
// 全 0 的話每個檔看起來都是 1980-01-01，「挑最新的存檔」永遠挑到同一個。
func TestFileTimeReportsRealTimestamp(t *testing.T) {
	m, d := newTest(t)
	writeFixture(t, d, "G.DAT", "x")
	path := filepath.Join(d.Root, "G.DAT")
	when := time.Date(1994, 3, 15, 13, 45, 20, 0, time.Local)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
	h := openFile(t, m, d, "G.DAT", 0)

	m.CPU.R[cpu.BX] = h
	call(m, d, 0x21, 0x5700)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("取時間失敗：AX=%d", m.CPU.R[cpu.AX])
	}
	wantTime := uint16(13)<<11 | uint16(45)<<5 | uint16(10)
	wantDate := uint16(1994-1980)<<9 | uint16(3)<<5 | uint16(15)
	if m.CPU.R[cpu.CX] != wantTime {
		t.Errorf("時間 %04X，預期 %04X（13:45:20）", m.CPU.R[cpu.CX], wantTime)
	}
	if m.CPU.R[cpu.DX] != wantDate {
		t.Errorf("日期 %04X，預期 %04X（1994-03-15）", m.CPU.R[cpu.DX], wantDate)
	}
}

// `AH=59h` 回的錯誤碼要與**剛才那一次失敗**一致。
func TestExtendedErrorMatchesTheLastFailure(t *testing.T) {
	m, d := newTest(t)

	putFileName(m, "NOPE.DAT")
	call(m, d, 0x21, 0x3D00)
	if m.CPU.Flags&cpu.CF == 0 {
		t.Fatal("開不存在的檔竟然成功")
	}
	call(m, d, 0x21, 0x5900)
	if m.CPU.R[cpu.AX] != 2 {
		t.Fatalf("AH=59h 回 %d，預期 2（找不到檔）", m.CPU.R[cpu.AX])
	}
	if m.CPU.R[cpu.BX]>>8 == 0 {
		t.Error("錯誤類別（BH）是 0——會被讀成「沒有錯誤」")
	}

	// 換一種失敗：無效 handle。
	m.CPU.R[cpu.BX] = 99
	m.CPU.R[cpu.CX] = 1
	call(m, d, 0x21, 0x3F00)
	call(m, d, 0x21, 0x5900)
	if m.CPU.R[cpu.AX] != 6 {
		t.Errorf("AH=59h 回 %d，預期 6（無效 handle）", m.CPU.R[cpu.AX])
	}
}

// `AH=0Ah` 的第二個位元組是**實際字元數**，內容以 CR 結尾。
func TestBufferedInputFillsLengthAndTerminator(t *testing.T) {
	m, d := newTest(t)
	d.Stdin = []byte("HELLO\rNEXT")

	const off = 0x400
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = fileScratchSeg, off
	m.Write8(cpu.Addr(fileScratchSeg, off), 20) // 容量
	m.Write8(cpu.Addr(fileScratchSeg, off+1), 0xAA)
	call(m, d, 0x21, 0x0A00)

	if got := m.Read8(cpu.Addr(fileScratchSeg, off+1)); got != 5 {
		t.Fatalf("實際長度是 %d，預期 5", got)
	}
	line := make([]byte, 5)
	for i := range line {
		line[i] = m.Read8(cpu.Addr(fileScratchSeg, off+2+uint16(i)))
	}
	if string(line) != "HELLO" {
		t.Errorf("內容是 %q，預期 HELLO", line)
	}
	if got := m.Read8(cpu.Addr(fileScratchSeg, off+7)); got != '\r' {
		t.Errorf("結尾是 %02X，預期 0D", got)
	}
	// CR 要被吃掉，下一次讀從 NEXT 開始。
	if string(d.Stdin) != "NEXT" {
		t.Errorf("剩下的輸入是 %q，預期 NEXT", d.Stdin)
	}
}

// `AH=5Bh` 在檔案已存在時要失敗（錯誤碼 80），不能當成 `AH=3Ch` 覆蓋掉。
func TestCreateNewRefusesExistingFile(t *testing.T) {
	m, d := newTest(t)
	writeFixture(t, d, "SAVE1.DAT", "old")
	d.Scratch = t.TempDir()

	putFileName(m, "SAVE1.DAT")
	call(m, d, 0x21, 0x5B00)
	if m.CPU.Flags&cpu.CF == 0 || m.CPU.R[cpu.AX] != 80 {
		t.Fatalf("已存在卻回 CF=%t AX=%d，預期 CF=1 AX=80",
			m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.AX])
	}
	if body, _ := os.ReadFile(filepath.Join(d.Root, "SAVE1.DAT")); string(body) != "old" {
		t.Errorf("原檔被動過：%q", body)
	}

	// 不存在的名字要建得起來。
	putFileName(m, "SAVE2.DAT")
	call(m, d, 0x21, 0x5B00)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("建新檔失敗：AX=%d", m.CPU.R[cpu.AX])
	}
}

// `AH=56h` 更名只動暫存層；沒有暫存層時回「拒絕存取」而不是假裝成功。
func TestRenameOnlyTouchesScratch(t *testing.T) {
	m, d := newTest(t)
	writeFixture(t, d, "OLD.DAT", "body")

	putName2 := func(oldName, newName string) {
		m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = fileScratchSeg, 0
		m.WriteBytes(cpu.Addr(fileScratchSeg, 0), append([]byte(oldName), 0))
		m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DI] = fileScratchSeg, 0x40
		m.WriteBytes(cpu.Addr(fileScratchSeg, 0x40), append([]byte(newName), 0))
	}

	putName2("OLD.DAT", "NEW.DAT")
	call(m, d, 0x21, 0x5600)
	if m.CPU.Flags&cpu.CF == 0 || m.CPU.R[cpu.AX] != 5 {
		t.Fatalf("沒有暫存層卻回 CF=%t AX=%d，預期 CF=1 AX=5",
			m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.AX])
	}

	d.Scratch = t.TempDir()
	putName2("OLD.DAT", "NEW.DAT")
	call(m, d, 0x21, 0x5600)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("有暫存層還是失敗：AX=%d", m.CPU.R[cpu.AX])
	}
	if _, err := os.Stat(filepath.Join(d.Root, "OLD.DAT")); err != nil {
		t.Error("原版目錄的檔被搬走了")
	}
	if body, err := os.ReadFile(filepath.Join(d.Scratch, "NEW.DAT")); err != nil {
		t.Errorf("暫存層沒有新名字：%v", err)
	} else if string(body) != "body" {
		t.Errorf("新名字的內容是 %q，預期 body", body)
	}
}

// `int 33h AX=0Bh` 回相對位移，**讀過歸零**。
//
// 不歸零的話，用相對位移轉視角的程式會一直收到同一個位移，畫面自己轉不停。
func TestMouseMotionCountersResetOnRead(t *testing.T) {
	m, d := newTest(t)
	m.SetVideoMode(0x13)
	d.MoveMouse(10, 20)
	d.MoveMouse(30, 25)

	call(m, d, 0x33, 0x000B)
	if int16(m.CPU.R[cpu.CX]) != 30 || int16(m.CPU.R[cpu.DX]) != 25 {
		t.Fatalf("相對位移 (%d,%d)，預期 (30,25)",
			int16(m.CPU.R[cpu.CX]), int16(m.CPU.R[cpu.DX]))
	}
	call(m, d, 0x33, 0x000B)
	if m.CPU.R[cpu.CX] != 0 || m.CPU.R[cpu.DX] != 0 {
		t.Errorf("第二次回 (%d,%d)，預期 (0,0)", m.CPU.R[cpu.CX], m.CPU.R[cpu.DX])
	}

	// 往回移動要給負數。
	d.MoveMouse(20, 25)
	call(m, d, 0x33, 0x000B)
	if int16(m.CPU.R[cpu.CX]) != -10 {
		t.Errorf("往回移動回 %d，預期 −10", int16(m.CPU.R[cpu.CX]))
	}
}
