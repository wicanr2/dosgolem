package dos

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// P2 那批 `int 21h` 的契約測試（`docs/knowledge-base/010-dos-int21-coverage.md`）。
// 每一條釘的都是「做錯了不會報錯」的那一面。

// `AH=0Ch` 要清掉模擬的鍵盤，但**不能清掉回放用的 `Stdin`**。
//
// 清掉的話，程式呼叫一次就把後面預備好的按鍵全部吃光，
// 而回放停在一個看起來像「程式自己不動了」的地方。
func TestFlushKeepsReplayStdin(t *testing.T) {
	m, d := newTest(t)
	m.PushBIOSKey(0x1E, 'A')
	d.Keys = []uint16{0x1E41}
	d.Stdin = []byte("XY")

	call(m, d, 0x21, 0x0C00)
	if n := m.BIOSKeyCount(); n != 0 {
		t.Errorf("BDA 環還有 %d 個鍵沒清掉", n)
	}
	if len(d.Keys) != 0 {
		t.Errorf("int 16h 佇列還有 %d 個鍵沒清掉", len(d.Keys))
	}
	if string(d.Stdin) != "XY" {
		t.Fatalf("回放佇列變成 %q，預期原封不動的 XY", d.Stdin)
	}

	// 帶 AL=01h 要真的做一次輸入，而且取的是 Stdin 的第一個字元。
	call(m, d, 0x21, 0x0C01)
	if got := uint8(m.CPU.R[cpu.AX]); got != 'X' {
		t.Errorf("AL=01h 讀到 %q，預期 X", got)
	}
	if string(d.Stdin) != "Y" {
		t.Errorf("讀完剩 %q，預期 Y", d.Stdin)
	}
}

// `AH=32h` 要回「無效磁碟機」，不能編一份磁碟參數區塊出來。
//
// 編出來的每個欄位都是假的，而程式會沿著裡面的 far 指標走進「驅動程式」，
// 或照那些數字自己算磁區——**編得越像，它走得越遠才炸**。
func TestDriveParamsRefusesToFabricate(t *testing.T) {
	m, d := newTest(t)
	call(m, d, 0x21, 0x3200)
	if got := uint8(m.CPU.R[cpu.AX]); got != 0xFF {
		t.Fatalf("AL 回 %02X，預期 FF（無效磁碟機）", got)
	}
	if d.Unimplemented[Call{Int: 0x21, AH: 0x32}] == 0 {
		t.Error("沒有記一筆——踩到的人看不到我們沒有 DPB")
	}
}

// `AH=34h` 的 InDOS 旗標要指到**真的記憶體**，而且不能落在向量或 stub 上。
//
// 回 0000:0000 的話，會自己加減那個旗標的 TSR 就把 `int 00h` 的向量改掉了。
func TestInDOSFlagPointsAtSafeMemory(t *testing.T) {
	m, d := newTest(t)
	call(m, d, 0x21, 0x3400)
	seg, off := m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BX]
	if seg == 0 && off == 0 {
		t.Fatal("InDOS 旗標指到 0000:0000——那是中斷向量表")
	}
	addr := cpu.Addr(seg, off)
	if addr < 0x600 {
		t.Errorf("旗標在 %05X，落在向量表／EMS 標頭裡", addr)
	}
	if v := m.Read8(addr); v != 0 {
		t.Errorf("旗標是 %02X，預期 0（我們的 int 21h 沒有可被打斷的中間狀態）", v)
	}
	// 程式寫回去不能弄壞任何向量的處理常式。
	m.Write8(addr, 1)
	for v := 0; v < 256; v++ {
		if b := m.Read8(cpu.Addr(m.Read16(uint32(v)*4+2), m.Read16(uint32(v)*4))); b == 0 {
			t.Fatalf("寫 InDOS 旗標之後向量 %02X 的第一個位元組變成 0", v)
		}
	}
}

// `AH=37h` 的選項字元要是 `/`，而且設了之後讀得回來。
//
// 回垃圾的話程式可能拿到 `\`，於是把自己的 `/S` 當成目錄名去開。
func TestSwitchCharRoundTrip(t *testing.T) {
	m, d := newTest(t)
	call(m, d, 0x21, 0x3700)
	if got := uint8(m.CPU.R[cpu.DX]); got != '/' {
		t.Fatalf("選項字元是 %q，預期 /", got)
	}
	m.CPU.R[cpu.DX] = '-'
	call(m, d, 0x21, 0x3701)
	call(m, d, 0x21, 0x3700)
	if got := uint8(m.CPU.R[cpu.DX]); got != '-' {
		t.Errorf("設成 - 之後讀回 %q", got)
	}
}

// `AH=58h` 設的策略要**真的改變 `AH=48h` 給出來的段**。
//
// 只記不用的話，程式依策略算出來的預期位址與實際拿到的不同，
// 而它多半是拿那個位址去比大小、算距離的——差異不會當場顯現。
func TestAllocStrategyChangesWhichBlockIsReturned(t *testing.T) {
	// 先造出兩個大小不同的空洞：低位址一個大的、稍高一個剛好的。
	setup := func(t *testing.T) (*machine.Machine, *DOS, uint16, uint16) {
		t.Helper()
		m, d := newTest(t)
		alloc := func(n uint16) uint16 {
			m.CPU.R[cpu.BX] = n
			call(m, d, 0x21, 0x4800)
			if m.CPU.Flags&cpu.CF != 0 {
				t.Fatalf("配 %d 段失敗", n)
			}
			return m.CPU.R[cpu.AX]
		}
		free := func(seg uint16) {
			m.CPU.Seg[cpu.ES] = seg
			call(m, d, 0x21, 0x4900)
			if m.CPU.Flags&cpu.CF != 0 {
				t.Fatalf("釋放 %04X 失敗", seg)
			}
		}
		big := alloc(0x100)  // 之後放掉 → 大空洞（低位址）
		_ = alloc(0x10)      // 卡住，不讓兩個空洞併起來
		small := alloc(0x10) // 之後放掉 → 剛好的空洞（高位址）
		_ = alloc(0x10)      // 同上，隔開後面的大片自由區
		free(big)
		free(small)
		return m, d, big, small
	}

	t.Run("first fit 拿低位址那一塊", func(t *testing.T) {
		m, d, big, small := setup(t)
		m.CPU.R[cpu.BX] = 0x10
		call(m, d, 0x21, 0x4800)
		if got := m.CPU.R[cpu.AX]; got != big {
			t.Errorf("first fit 拿到 %04X，預期最前面的空洞 %04X（另一個是 %04X）", got, big, small)
		}
	})

	t.Run("best fit 拿剛好那一塊", func(t *testing.T) {
		m, d, big, small := setup(t)
		m.CPU.R[cpu.BX] = 1 // best fit
		call(m, d, 0x21, 0x5801)
		if m.CPU.Flags&cpu.CF != 0 {
			t.Fatal("設 best fit 失敗")
		}
		m.CPU.R[cpu.BX] = 0x10
		call(m, d, 0x21, 0x4800)
		if got := m.CPU.R[cpu.AX]; got != small {
			t.Errorf("best fit 拿到 %04X，預期剛好的空洞 %04X（大的是 %04X）", got, small, big)
		}
	})

	t.Run("last fit 拿最高位址那一塊", func(t *testing.T) {
		m, d, big, small := setup(t)
		m.CPU.R[cpu.BX] = 2 // last fit
		call(m, d, 0x21, 0x5801)
		m.CPU.R[cpu.BX] = 0x10
		call(m, d, 0x21, 0x4800)
		got := m.CPU.R[cpu.AX]
		if got == big || got == small {
			t.Errorf("last fit 拿到 %04X，那是前面的空洞——預期最尾端的自由區", got)
		}
	})

	// 讀回來的值要與設進去的一致。
	m, d := newTest(t)
	m.CPU.R[cpu.BX] = 2
	call(m, d, 0x21, 0x5801)
	call(m, d, 0x21, 0x5800)
	if got := m.CPU.R[cpu.AX]; got != 2 {
		t.Errorf("讀回策略 %d，預期 2", got)
	}
}

// 不合法的策略要失敗，不能默默收下。
//
// 收下的話 `AH=58h AL=00h` 會回一個 DOS 不可能回的值，
// 而檢查回傳值的程式會判定「這不是 DOS」。
func TestAllocStrategyRejectsInvalidValues(t *testing.T) {
	m, d := newTest(t)
	for _, bad := range []uint16{3, 0x0F, 0x20, 0x1234} {
		m.CPU.R[cpu.BX] = bad
		m.CPU.SetFlags(m.CPU.Flags &^ cpu.CF)
		call(m, d, 0x21, 0x5801)
		if m.CPU.Flags&cpu.CF == 0 {
			t.Errorf("策略 %04X 竟然被接受", bad)
		}
	}
	// 合法的高位組合要收（40h ＝ 高位優先、80h ＝ 只用高位）。
	for _, ok := range []uint16{0x00, 0x01, 0x02, 0x40, 0x41, 0x80, 0x82} {
		m.CPU.R[cpu.BX] = ok
		call(m, d, 0x21, 0x5801)
		if m.CPU.Flags&cpu.CF != 0 {
			t.Errorf("合法策略 %04X 被拒絕", ok)
		}
	}
}

// UMB 連結狀態要讀得回來，而且**連起來時要留下痕跡**——
// 我們的配置池只有傳統記憶體，程式要求高位記憶體卻拿到傳統記憶體
// 這件事不能沒有紀錄。
func TestUMBLinkRoundTripIsRecorded(t *testing.T) {
	m, d := newTest(t)
	call(m, d, 0x21, 0x5802)
	if got := uint8(m.CPU.R[cpu.AX]); got != 0 {
		t.Errorf("預設 UMB 連結是 %d，預期 0（與 DOS 相同）", got)
	}
	m.CPU.R[cpu.BX] = 1
	call(m, d, 0x21, 0x5803)
	call(m, d, 0x21, 0x5802)
	if got := uint8(m.CPU.R[cpu.AX]); got != 1 {
		t.Fatalf("連起來之後讀回 %d", got)
	}
	if d.Unimplemented[Call{Int: 0x21, AH: 0x58, AL: 0x83}] == 0 {
		t.Error("UMB 連起來卻沒有記一筆")
	}
}

// `AH=5Ah` 要把產生的檔名**接回呼叫端的緩衝區**。
//
// 不寫回的話，呼叫端接著拿同一個緩衝區去 `AH=41h` 刪檔，刪到的是目錄名。
func TestCreateTempWritesNameBack(t *testing.T) {
	m, d := newTest(t)
	d.Scratch = t.TempDir()
	const buf = 0x3000
	m.WriteBytes(cpu.Addr(buf, 0), append([]byte(`C:\TMP\`), 0))
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = buf, 0

	call(m, d, 0x21, 0x5A00)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("建暫存檔失敗：AX=%04X", m.CPU.R[cpu.AX])
	}
	h := m.CPU.R[cpu.AX]
	got := d.readCString(buf, 0, 128)
	if !strings.HasPrefix(got, `C:\TMP\`) || len(got) <= len(`C:\TMP\`) {
		t.Fatalf("緩衝區是 %q，預期原路徑後面接上檔名", got)
	}
	name := got[len(`C:\TMP\`):]
	if _, err := os.Stat(filepath.Join(d.Scratch, name)); err != nil {
		t.Errorf("檔案 %s 沒有真的建出來：%v", name, err)
	}
	if _, ok := d.handles[h]; !ok {
		t.Errorf("handle %d 不存在", h)
	}

	// 第二次要換一個名字，否則兩個暫存檔會互相覆蓋。
	m.WriteBytes(cpu.Addr(buf, 0), append([]byte(`C:\TMP\`), 0))
	m.CPU.R[cpu.DX] = 0
	call(m, d, 0x21, 0x5A00)
	if second := d.readCString(buf, 0, 128); second == got {
		t.Errorf("第二次拿到同一個名字 %q", second)
	}

	// **同一份輸入要得到同一個名字**：換成時間或亂數的話，
	// 兩次執行的檔案清單對不起來，對拍會把差異歸到別處。
	m2, d2 := newTest(t)
	d2.Scratch = t.TempDir()
	m2.WriteBytes(cpu.Addr(buf, 0), append([]byte(`C:\TMP\`), 0))
	m2.CPU.Seg[cpu.DS], m2.CPU.R[cpu.DX] = buf, 0
	call(m2, d2, 0x21, 0x5A00)
	if again := d2.readCString(buf, 0, 128); again != got {
		t.Errorf("另一次執行拿到 %q，預期與 %q 相同", again, got)
	}
}

// `AH=5Ch` 對不存在的 handle 要失敗。
//
// 回成功的話，程式以為自己鎖住了一段其實不存在的檔案，接著放心地寫下去。
func TestLockRegionRejectsBadHandle(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.BX] = 99
	call(m, d, 0x21, 0x5C00)
	if m.CPU.Flags&cpu.CF == 0 {
		t.Fatal("鎖一個不存在的 handle 竟然成功")
	}
	if got := m.CPU.R[cpu.AX]; got != 6 {
		t.Errorf("錯誤碼 %d，預期 6（無效 handle）", got)
	}
}

// `AH=60h` 要吐出完整路徑。
//
// 回相對路徑的話，安裝程式把它存進設定檔，下一次從別的目錄啟動就找不到
// ——症狀出現在**下一次執行**，離這裡很遠。
func TestTrueNameProducesAbsolutePath(t *testing.T) {
	m, d := newTest(t)
	d.Dir = `GAME\DATA`
	const in, out = 0x3000, 0x3100
	cases := []struct{ arg, want string }{
		{`SAVE.DAT`, `C:\GAME\DATA\SAVE.DAT`},
		{`..\SAVE.DAT`, `C:\GAME\SAVE.DAT`},
		{`\ROOT\A.TXT`, `C:\ROOT\A.TXT`},
		{`a:\x\.\y`, `A:\X\Y`},
		{`..\..\..\Z`, `C:\Z`}, // 退過根目錄要停在根目錄
	}
	for _, tc := range cases {
		m.WriteBytes(cpu.Addr(in, 0), append([]byte(tc.arg), 0))
		m.CPU.Seg[cpu.DS], m.CPU.R[cpu.SI] = in, 0
		m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DI] = out, 0
		call(m, d, 0x21, 0x6000)
		if m.CPU.Flags&cpu.CF != 0 {
			t.Fatalf("%q 失敗：AX=%04X", tc.arg, m.CPU.R[cpu.AX])
		}
		if got := d.readCString(out, 0, 128); got != tc.want {
			t.Errorf("%q → %q，預期 %q", tc.arg, got, tc.want)
		}
	}
}

// `AH=6Ch` 的 CX 要說出**實際做了哪一件事**。
//
// 不填的話呼叫端讀到殘值，可能把一個已經有內容的存檔當成剛建的空白檔。
func TestExtendedOpenReportsActionTaken(t *testing.T) {
	newOpen := func(t *testing.T) (*machine.Machine, *DOS) {
		t.Helper()
		m, d := newTest(t)
		d.Scratch = t.TempDir()
		if err := os.WriteFile(filepath.Join(d.Root, "OLD.DAT"), []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}
		return m, d
	}
	const buf = 0x3000
	setName := func(m *machine.Machine, name string) {
		m.WriteBytes(cpu.Addr(buf, 0), append([]byte(name), 0))
		m.CPU.Seg[cpu.DS], m.CPU.R[cpu.SI] = buf, 0
	}

	t.Run("開既有的回 CX=1", func(t *testing.T) {
		m, d := newOpen(t)
		setName(m, "OLD.DAT")
		m.CPU.R[cpu.BX], m.CPU.R[cpu.CX], m.CPU.R[cpu.DX] = 0, 0, 0x0001
		call(m, d, 0x21, 0x6C00)
		if m.CPU.Flags&cpu.CF != 0 {
			t.Fatalf("開既有的檔失敗：AX=%04X", m.CPU.R[cpu.AX])
		}
		if got := m.CPU.R[cpu.CX]; got != 1 {
			t.Errorf("CX=%d，預期 1（開啟既有）", got)
		}
	})

	t.Run("建新的回 CX=2", func(t *testing.T) {
		m, d := newOpen(t)
		setName(m, "NEW.DAT")
		m.CPU.R[cpu.BX], m.CPU.R[cpu.CX], m.CPU.R[cpu.DX] = 2, 0, 0x0010
		call(m, d, 0x21, 0x6C00)
		if m.CPU.Flags&cpu.CF != 0 {
			t.Fatalf("建新檔失敗：AX=%04X", m.CPU.R[cpu.AX])
		}
		if got := m.CPU.R[cpu.CX]; got != 2 {
			t.Errorf("CX=%d，預期 2（建立）", got)
		}
	})

	t.Run("覆寫既有的回 CX=3", func(t *testing.T) {
		m, d := newOpen(t)
		setName(m, "OLD.DAT")
		m.CPU.R[cpu.BX], m.CPU.R[cpu.CX], m.CPU.R[cpu.DX] = 2, 0, 0x0002
		call(m, d, 0x21, 0x6C00)
		if m.CPU.Flags&cpu.CF != 0 {
			t.Fatalf("覆寫失敗：AX=%04X", m.CPU.R[cpu.AX])
		}
		if got := m.CPU.R[cpu.CX]; got != 3 {
			t.Errorf("CX=%d，預期 3（覆寫）", got)
		}
	})

	t.Run("該失敗的要失敗", func(t *testing.T) {
		m, d := newOpen(t)
		// 檔案存在但「存在就失敗」。
		setName(m, "OLD.DAT")
		m.CPU.R[cpu.DX] = 0x0010
		call(m, d, 0x21, 0x6C00)
		if m.CPU.Flags&cpu.CF == 0 {
			t.Error("既有檔配「存在就失敗」竟然成功")
		} else if got := m.CPU.R[cpu.AX]; got != 80 {
			t.Errorf("錯誤碼 %d，預期 80（檔案已存在）", got)
		}
		// 檔案不存在而且「不存在就失敗」。
		setName(m, "NOPE.DAT")
		m.CPU.R[cpu.DX] = 0x0001
		call(m, d, 0x21, 0x6C00)
		if m.CPU.Flags&cpu.CF == 0 {
			t.Error("開不存在的檔竟然成功")
		} else if got := m.CPU.R[cpu.AX]; got != 2 {
			t.Errorf("錯誤碼 %d，預期 2（找不到檔）", got)
		}
		// 失敗的原因要留給 AH=59h。
		call(m, d, 0x21, 0x5900)
		if got := m.CPU.R[cpu.AX]; got != 2 {
			t.Errorf("AH=59h 回 %d，預期 2", got)
		}
	})
}

// `AH=38h AL=00h` 把國別表寫進**呼叫端**的 DS:DX，DS 與 DX 不動
// （`docs/spec/196-country-info-caller-buffer`）。
//
// 反面：DS 被換掉之後，呼叫端所有以 DS 為基底的存取都落到別的段——
// TASM 就是這樣在第 848 條指令安靜地以回傳碼 7 結束。
func TestCountryInfoFillsCallerBuffer(t *testing.T) {
	m, d := newTest(t)
	const seg, off = 0x1706, 0x4E1A
	for i := uint32(0); i < 0x22; i++ {
		m.Write8(cpu.Addr(seg, off)+i, 0xCC)
	}
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = seg, off
	call(m, d, 0x21, 0x3800)
	if m.CPU.Seg[cpu.DS] != seg || m.CPU.R[cpu.DX] != off {
		t.Fatalf("DS:DX 被改成 %04X:%04X，預期維持 %04X:%04X",
			m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX], seg, off)
	}
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatal("CF 沒有清除")
	}
	if m.CPU.R[cpu.BX] != 81 || m.CPU.R[cpu.AX]&0xFF != 81 || m.CPU.R[cpu.AX]>>8 != 0x38 {
		t.Fatalf("AX=%04X BX=%04X，預期 AL＝BX＝81、AH 不變", m.CPU.R[cpu.AX], m.CPU.R[cpu.BX])
	}
	if v := m.Read8(cpu.Addr(seg, off)); v != 2 {
		t.Fatalf("緩衝區第 0 byte（日期格式）是 %02X，預期 02——表沒寫進呼叫端緩衝區", v)
	}
	if v := m.Read8(cpu.Addr(seg, off) + 0x18); v != 0xCC {
		t.Fatalf("緩衝區 +18h 被改成 %02X，預期只寫 18h bytes", v)
	}
}
