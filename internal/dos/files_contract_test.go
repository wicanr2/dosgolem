package dos

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// 檔案服務（`AH=3Ch`／`3Dh`／`3Eh`／`3Fh`／`40h`／`42h`／`4Eh`）的獨立契約測試。
//
// 期望值照 DOS 的介面自己列，不引用分支的既有測試。這一層的錯誤有一個共同
// 形狀：**呼叫端多半不檢查回傳值**，所以錯的資料會一路走到很後面才炸。

const fileScratchSeg = 0x3400

// putName 把一個 NUL 結尾的檔名擺進 DS:DX。
func putFileName(m *machine.Machine, name string) {
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = fileScratchSeg, 0
	m.WriteBytes(cpu.Addr(fileScratchSeg, 0), append([]byte(name), 0))
}

func openFile(t *testing.T, m *machine.Machine, d *DOS, name string, mode uint8) uint16 {
	t.Helper()
	putFileName(m, name)
	call(m, d, 0x21, 0x3D00|uint16(mode))
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("開 %s 失敗：AX=%04X", name, m.CPU.R[cpu.AX])
	}
	return m.CPU.R[cpu.AX]
}

func writeFixture(t *testing.T, d *DOS, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(d.Root, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// handle 號碼是 PSP 那張 job file table 的索引：
// 0–4 是標準 handle，開檔從 5 開始，**關掉的號碼要放回去**。
//
// 只增不重用的話，開開關關幾十次之後號碼爬過 20，而 C runtime 拿 handle 當
// 自己那張固定大小的表的索引——越界之後 `fopen` 會開成功再關掉並回 NULL，
// 症狀是「開得好好的檔突然開不起來」。
func TestHandleNumbersStartAtFiveAndAreReused(t *testing.T) {
	m, d := newTest(t)
	writeFixture(t, d, "A.DAT", "hello")

	first := openFile(t, m, d, "A.DAT", 0)
	if first != 5 {
		t.Fatalf("第一個 handle 是 %d，預期 5（0–4 是標準 handle）", first)
	}
	second := openFile(t, m, d, "A.DAT", 0)
	if second == first {
		t.Fatal("兩個同時開著的檔拿到同一個 handle")
	}

	m.CPU.R[cpu.BX] = first
	call(m, d, 0x21, 0x3E00)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatal("關檔失敗")
	}
	if again := openFile(t, m, d, "A.DAT", 0); again != first {
		t.Errorf("關掉 %d 之後再開拿到 %d——號碼沒放回去", first, again)
	}
}

// 表滿了要以 `AX=4`（開太多檔）失敗，**不是給一個越界的號碼**。
func TestOpenFailsWhenTableIsFull(t *testing.T) {
	m, d := newTest(t)
	writeFixture(t, d, "A.DAT", "x")
	for i := 0; ; i++ {
		putFileName(m, "A.DAT")
		call(m, d, 0x21, 0x3D00)
		if m.CPU.Flags&cpu.CF != 0 {
			if m.CPU.R[cpu.AX] != 4 {
				t.Errorf("表滿時 AX=%d，預期 4", m.CPU.R[cpu.AX])
			}
			return
		}
		if h := m.CPU.R[cpu.AX]; h >= d.MaxHandles {
			t.Fatalf("配出 handle %d，上限是 %d", h, d.MaxHandles)
		}
		if i > 64 {
			t.Fatal("開不完——上限沒有生效")
		}
	}
}

// 讀取要真的把位元組搬進 DS:DX，AX 回實際讀到的長度；
// 讀到檔尾回 0 而**不是**錯誤。
//
// 回錯誤的話，把「讀到 0」當結束條件的程式會走進錯誤處理路徑；
// 回一個沒填過的緩衝區則會讓它把上一次的資料當成新資料用。
func TestReadFillsBufferAndReportsEOFAsZero(t *testing.T) {
	m, d := newTest(t)
	writeFixture(t, d, "B.DAT", "ABCDE")
	h := openFile(t, m, d, "B.DAT", 0)

	const bufOff = 0x200
	m.CPU.R[cpu.BX], m.CPU.R[cpu.CX] = h, 3
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = fileScratchSeg, bufOff
	call(m, d, 0x21, 0x3F00)
	if m.CPU.R[cpu.AX] != 3 {
		t.Fatalf("讀 3 個位元組回 %d", m.CPU.R[cpu.AX])
	}
	for i, want := range []byte("ABC") {
		if got := m.Read8(cpu.Addr(fileScratchSeg, bufOff+uint16(i))); got != want {
			t.Fatalf("緩衝區第 %d 個位元組是 %02X，預期 %02X", i, got, want)
		}
	}

	// 再讀 10 個只剩 2 個。
	m.CPU.R[cpu.BX], m.CPU.R[cpu.CX] = h, 10
	call(m, d, 0x21, 0x3F00)
	if m.CPU.R[cpu.AX] != 2 {
		t.Fatalf("剩下的長度回 %d，預期 2", m.CPU.R[cpu.AX])
	}
	// 檔尾：0 個位元組、CF 不設。
	m.CPU.R[cpu.BX], m.CPU.R[cpu.CX] = h, 10
	call(m, d, 0x21, 0x3F00)
	if m.CPU.Flags&cpu.CF != 0 || m.CPU.R[cpu.AX] != 0 {
		t.Fatalf("檔尾回 CF=%t AX=%d，預期 CF=0 AX=0",
			m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.AX])
	}
}

// `AH=42h` 的三種基準都要成立，而且 DX:AX 回定位後的絕對位置。
//
// 從檔尾往回定位（AL=2、CX:DX 是負數）是讀表格檔的慣用法；
// 把 CX:DX 當無號數的話會定位到一個巨大的位置，之後每一次讀都回 0，
// 看起來像「檔案是空的」。
func TestSeekSupportsAllThreeOrigins(t *testing.T) {
	m, d := newTest(t)
	writeFixture(t, d, "C.DAT", "0123456789")
	h := openFile(t, m, d, "C.DAT", 0)

	seek := func(al uint8, off int32) uint32 {
		m.CPU.R[cpu.BX] = h
		m.CPU.R[cpu.CX] = uint16(uint32(off) >> 16)
		m.CPU.R[cpu.DX] = uint16(uint32(off))
		call(m, d, 0x21, 0x4200|uint16(al))
		if m.CPU.Flags&cpu.CF != 0 {
			t.Fatalf("AL=%d seek %d 失敗", al, off)
		}
		return uint32(m.CPU.R[cpu.DX])<<16 | uint32(m.CPU.R[cpu.AX])
	}
	if got := seek(0, 4); got != 4 {
		t.Errorf("從頭 seek 4 回 %d", got)
	}
	if got := seek(1, 2); got != 6 {
		t.Errorf("從目前位置 seek +2 回 %d，預期 6", got)
	}
	if got := seek(2, 0); got != 10 {
		t.Errorf("seek 到檔尾回 %d，預期 10", got)
	}
	if got := seek(2, -3); got != 7 {
		t.Errorf("從檔尾 seek −3 回 %d，預期 7", got)
	}
}

// 無效 handle 的三支（讀、寫以外的操作）要**失敗即關閉**，回 AX=6。
func TestInvalidHandleFailsClosed(t *testing.T) {
	m, d := newTest(t)
	for _, fn := range []uint16{0x3E00, 0x3F00, 0x4200} {
		m.CPU.R[cpu.BX] = 99
		m.CPU.R[cpu.CX] = 1
		call(m, d, 0x21, fn)
		if m.CPU.Flags&cpu.CF == 0 || m.CPU.R[cpu.AX] != 6 {
			t.Errorf("AH=%02X 對無效 handle 回 CF=%t AX=%d，預期 CF=1 AX=6",
				fn>>8, m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.AX])
		}
	}
}

// 預設**不寫任何檔**，但要回報成功並記一筆。
//
// 回失敗的話存檔功能一按就走進錯誤路徑（那與「沒做存檔」是兩種行為）；
// 不記的話「程式想寫什麼」這個問題沒有答案。
func TestWritesAreRecordedNotSilentlyDropped(t *testing.T) {
	m, d := newTest(t)
	writeFixture(t, d, "SAVE.DAT", "old")
	h := openFile(t, m, d, "SAVE.DAT", 2) // 讀寫模式

	m.WriteBytes(cpu.Addr(fileScratchSeg, 0x300), []byte("NEW"))
	m.CPU.R[cpu.BX], m.CPU.R[cpu.CX] = h, 3
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = fileScratchSeg, 0x300
	call(m, d, 0x21, 0x4000)
	if m.CPU.Flags&cpu.CF != 0 || m.CPU.R[cpu.AX] != 3 {
		t.Fatalf("寫檔回 CF=%t AX=%d，預期成功寫 3 個位元組",
			m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.AX])
	}
	if len(d.Wrote) != 1 || d.Wrote[0].N != 3 {
		t.Fatalf("Wrote=%+v，預期記一筆 3 個位元組", d.Wrote)
	}
	body, err := os.ReadFile(filepath.Join(d.Root, "SAVE.DAT"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "old" {
		t.Errorf("原版目錄被寫成 %q——預設不該落地", body)
	}
}

// 暫存層打開之後，寫入落在暫存層而**原版目錄仍然不動**，
// 而且下一次開同一個檔要讀到自己寫的那一份。
func TestScratchLayerShadowsTheOriginalDirectory(t *testing.T) {
	m, d := newTest(t)
	writeFixture(t, d, "SAVE.DAT", "old")
	d.Scratch = t.TempDir()

	h := openFile(t, m, d, "SAVE.DAT", 2)
	m.WriteBytes(cpu.Addr(fileScratchSeg, 0x300), []byte("NEW"))
	m.CPU.R[cpu.BX], m.CPU.R[cpu.CX] = h, 3
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = fileScratchSeg, 0x300
	call(m, d, 0x21, 0x4000)
	m.CPU.R[cpu.BX] = h
	call(m, d, 0x21, 0x3E00)

	if body, _ := os.ReadFile(filepath.Join(d.Root, "SAVE.DAT")); string(body) != "old" {
		t.Errorf("原版目錄被改成 %q", body)
	}
	if body, err := os.ReadFile(filepath.Join(d.Scratch, "SAVE.DAT")); err != nil {
		t.Errorf("暫存層沒有這個檔：%v", err)
	} else if string(body) != "NEW" {
		t.Errorf("暫存層的內容是 %q，預期 NEW", body)
	}

	// 再開一次要讀到暫存層那一份。
	h2 := openFile(t, m, d, "SAVE.DAT", 0)
	m.CPU.R[cpu.BX], m.CPU.R[cpu.CX] = h2, 3
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = fileScratchSeg, 0x300
	call(m, d, 0x21, 0x3F00)
	got := make([]byte, 3)
	for i := range got {
		got[i] = m.Read8(cpu.Addr(fileScratchSeg, 0x300+uint16(i)))
	}
	if string(got) != "NEW" {
		t.Errorf("再開讀到 %q，預期暫存層的 NEW", got)
	}
}

// 檔名解析只認 basename、不分大小寫，而且套 FAT 的 8.3 截斷。
//
// 三者都是「程式傳進來的字串與磁碟上的檔名對不上」的常見形狀，
// 而 open 失敗多半不會被檢查——程式會一路跑到沒有映射的記憶體才停。
func TestNameResolutionMatchesDOSRules(t *testing.T) {
	m, d := newTest(t)
	writeFixture(t, d, "DATA.PAK", "x")
	writeFixture(t, d, "STEEDPIC", "x")

	for _, name := range []string{
		`A:\WHATEVER\DATA.PAK`, // 目錄部分要丟掉
		"data.pak",             // 大小寫不分
		"steedpics",            // 8.3 截斷
	} {
		putFileName(m, name)
		call(m, d, 0x21, 0x3D00)
		if m.CPU.Flags&cpu.CF != 0 {
			t.Errorf("開 %q 失敗（AX=%d）", name, m.CPU.R[cpu.AX])
			continue
		}
		m.CPU.R[cpu.BX] = m.CPU.R[cpu.AX]
		call(m, d, 0x21, 0x3E00)
	}

	// 截斷不能把「真的不存在」變成開得起來。
	putFileName(m, "DATA.PAX")
	call(m, d, 0x21, 0x3D00)
	if m.CPU.Flags&cpu.CF == 0 {
		t.Error("DATA.PAX 竟然開到了 DATA.PAK")
	}
}

// Find First（`AH=4Eh`）把結果寫進 DTA，缺檔回 `AX=18` 而且**不動 DTA**。
//
// 污染 DTA 的話，呼叫端在錯誤路徑上讀到的是上一次的檔名與長度——
// 它會以為找到了一個不存在的檔。
func TestFindFirstWritesDTAAndLeavesItAloneOnFailure(t *testing.T) {
	m, d := newTest(t)
	writeFixture(t, d, "FOUND.DAT", "12345")

	// 先設一個自己的 DTA，順便驗 AH=1Ah／AH=2Fh 是同一份。
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = 0x2600, 0x40
	call(m, d, 0x21, 0x1A00)
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BX] = 0, 0
	call(m, d, 0x21, 0x2F00)
	if m.CPU.Seg[cpu.ES] != 0x2600 || m.CPU.R[cpu.BX] != 0x40 {
		t.Fatalf("AH=2Fh 回 %04X:%04X，預期 2600:0040",
			m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BX])
	}

	putFileName(m, "FOUND.DAT")
	m.CPU.R[cpu.CX] = 0
	call(m, d, 0x21, 0x4E00)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("Find First 失敗：AX=%d", m.CPU.R[cpu.AX])
	}
	dta := cpu.Addr(0x2600, 0x40)
	if got := m.Read16(dta + 0x1A); got != 5 {
		t.Errorf("DTA 的檔案長度是 %d，預期 5", got)
	}
	name := make([]byte, 9)
	for i := range name {
		name[i] = m.Read8(dta + 0x1E + uint32(i))
	}
	if string(name) != "FOUND.DAT" {
		t.Errorf("DTA 的檔名是 %q，預期 FOUND.DAT", name)
	}

	// 缺檔：AX=18，DTA 不動。
	m.Write8(dta+0x1E, 0x5A)
	putFileName(m, "MISSING.DAT")
	call(m, d, 0x21, 0x4E00)
	if m.CPU.Flags&cpu.CF == 0 || m.CPU.R[cpu.AX] != 18 {
		t.Errorf("缺檔回 CF=%t AX=%d，預期 CF=1 AX=18",
			m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.AX])
	}
	if got := m.Read8(dta + 0x1E); got != 0x5A {
		t.Errorf("缺檔卻動了 DTA（%02X）", got)
	}
}

// 資料真的放在子目錄的遊戲要走完整路徑（`docs/spec/195`）。
//
// 只認 basename 的失敗**看起來不像檔案問題**：遊戲不檢查開檔結果就往下跑，
// 最後停在某個等事件的迴圈，畫面照在、指令照跑。
func TestSubdirectoryPathsResolveCaseInsensitively(t *testing.T) {
	m, d := newTest(t)
	if err := os.MkdirAll(filepath.Join(d.Root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, d, filepath.Join("data", "GRAPHICS.DAT"), "gfx")

	for _, name := range []string{
		`data\GRAPHICS.DAT`,    // 原樣
		`DATA\graphics.dat`,    // 每一段都大小寫不分
		`C:\data\GRAPHICS.DAT`, // 磁碟機代號丟掉
		`\data\GRAPHICS.DAT`,   // 開頭的分隔符相對於 Root
		`.\data\GRAPHICS.DAT`,  // `.` 略過
	} {
		putFileName(m, name)
		call(m, d, 0x21, 0x3D00)
		if m.CPU.Flags&cpu.CF != 0 {
			t.Errorf("開 %q 失敗（AX=%d）", name, m.CPU.R[cpu.AX])
			continue
		}
		m.CPU.R[cpu.BX] = m.CPU.R[cpu.AX]
		call(m, d, 0x21, 0x3E00)
	}
}

// `..` 讓整條路徑不合格，退回 basename——不能讓遊戲組出來的路徑走出 Root。
func TestParentDirectoryEscapeFallsBackToBasename(t *testing.T) {
	m, d := newTest(t)
	writeFixture(t, d, "INSIDE.DAT", "x")

	// `..` 那一段被拒，退回 basename 就在 Root 裡找到了。
	putFileName(m, `..\INSIDE.DAT`)
	call(m, d, 0x21, 0x3D00)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("退回 basename 之後仍然開不到（AX=%d）", m.CPU.R[cpu.AX])
	}
	m.CPU.R[cpu.BX] = m.CPU.R[cpu.AX]
	call(m, d, 0x21, 0x3E00)

	// Root 外面真的存在的檔不得因為 `..` 而開得到。
	outside := filepath.Join(filepath.Dir(d.Root), "OUTSIDE.DAT")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	putFileName(m, `..\OUTSIDE.DAT`)
	call(m, d, 0x21, 0x3D00)
	if m.CPU.Flags&cpu.CF == 0 {
		t.Error("Root 外面的檔竟然開到了")
	}
}

// 目錄部分對不上時退回 basename——既有的多磁片版救援一個字都不能少。
func TestUnknownDirectoryStillFallsBackToBasename(t *testing.T) {
	m, d := newTest(t)
	writeFixture(t, d, "DATA.PAK", "x")

	putFileName(m, `A:\WHATEVER\DATA.PAK`)
	call(m, d, 0x21, 0x3D00)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("目錄對不上時沒退回 basename（AX=%d）", m.CPU.R[cpu.AX])
	}
	m.CPU.R[cpu.BX] = m.CPU.R[cpu.AX]
	call(m, d, 0x21, 0x3E00)
}

// `AH=0Eh` 選磁碟機：存進 `d.Drive` 讓 `AH=19h` 讀得到，AL 回磁碟機數量。
//
// **回 1 的話遊戲會判定沒有硬碟。**
func TestSelectDefaultDriveIsReadableBack(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.DX] = 0x0002 // C:
	call(m, d, 0x21, 0x0E00)
	if al := uint8(m.CPU.R[cpu.AX]); al != 3 {
		t.Errorf("AL=%d，預期 3（A、B、C）", al)
	}
	call(m, d, 0x21, 0x1900)
	if al := uint8(m.CPU.R[cpu.AX]); al != 2 {
		t.Errorf("AH=19h 回 AL=%d，預期 2（就是剛剛選的 C:）", al)
	}
}

// generic IOCTL 取裝置參數（`docs/spec/196`）：回報成固定硬碟，
// 而且 BPB 要與 `AH=1Bh` 說的同一組數字。
//
// **這一支是「要不要提示換片」的判斷來源。** 答不出來的程式會退而
// 用 `int 13h` 讀開機磁區，那條路再失敗就停在換片提示上。
func TestGetDeviceParametersReportsFixedDisk(t *testing.T) {
	m, d := newTest(t)
	const blk = 0x0200
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = 0x1000, blk
	base := uint32(0x1000)*16 + blk
	// 位移 0 是輸入，放一個看得出來的值確認我們不覆寫它。
	m.Write8(base, 0x04)

	m.CPU.R[cpu.BX] = 0x0003 // C:
	m.CPU.R[cpu.CX] = 0x0860 // CH=08 磁碟類別、CL=60 取裝置參數
	call(m, d, 0x21, 0x440D)

	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("回失敗（AX=%d）", m.CPU.R[cpu.AX])
	}
	if got := m.Read8(base); got != 0x04 {
		t.Errorf("位移 0（輸入）被覆寫成 %02X", got)
	}
	if got := m.Read8(base + 1); got != 0x05 {
		t.Errorf("裝置型別 %02X，預期 05（固定硬碟）", got)
	}
	if got := m.Read16(base + 2); got&1 == 0 {
		t.Errorf("屬性 %04X，預期 bit0 立起來（不可移除）", got)
	}
	if got := m.Read16(base + 4); got == 0 {
		t.Error("磁柱數是 0")
	}

	// BPB 要與 AH=1Bh 一致：同一支程式兩邊都問得到。
	bpb := base + 7
	call(m, d, 0x21, 0x1B00)
	wantSector, wantClust, wantMedia := m.CPU.R[cpu.CX], uint8(m.CPU.R[cpu.AX]),
		m.Read8(cpu.Addr(m.CPU.Seg[cpu.DS], m.CPU.R[cpu.BX]))
	if got := m.Read16(bpb + 0); got != wantSector {
		t.Errorf("BPB 每磁區位元組 %d，AH=1Bh 說 %d", got, wantSector)
	}
	if got := m.Read8(bpb + 2); got != wantClust {
		t.Errorf("BPB 每叢集磁區 %d，AH=1Bh 說 %d", got, wantClust)
	}
	if got := m.Read8(bpb + 10); got != wantMedia {
		t.Errorf("BPB 媒體描述子 %02X，AH=1Bh 說 %02X", got, wantMedia)
	}
}
