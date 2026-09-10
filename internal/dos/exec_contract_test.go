package dos

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// EXEC（`AH=4Bh`）與行程模型的獨立契約測試。
//
// 期望值照 DOS 的 EXEC 約定自己列，不引用分支的既有測試。這一組釘的是
// 殼鏈：每一條的反面都不報錯，只會讓鏈**安靜地斷在某一跳**——
// 外面看到的是「跑滿指令上限，程式還活著」。

// 一支子程式跑完之後，父程式要在**它自己的**上下文裡接著跑。
//
// 三件事一起驗：控制權回到 int 21h 的下一道、暫存器沒被子程式弄髒、
// 子程式佔的記憶體收回去。少了最後一條，連續 EXEC 十次就會把 640 KB 用光，
// 而失敗發生在第十一次——與這裡毫無關聯的地方。
func TestExecReturnsToParentWithMemoryReclaimed(t *testing.T) {
	m, d := newTest(t)
	writeChild(t, d, "A.COM", []byte{0xB8, 0x01, 0x4C, 0xCD, 0x21}) // exit 1

	freeBefore := d.freeSeg
	pspBefore := d.curPSP
	m.CPU.R[cpu.SI], m.CPU.R[cpu.DI] = 0xAAAA, 0xBBBB

	for i := 0; i < 4; i++ {
		execChild(m, d, "A.COM")
		if m.CPU.Flags&cpu.CF != 0 {
			t.Fatalf("第 %d 次 EXEC 失敗：AX=%04X Missing=%v", i, m.CPU.R[cpu.AX], d.Missing)
		}
		for n := 0; n < 200 && len(d.procStack) > 0; n++ {
			if err := m.Step(); err != nil {
				t.Fatalf("第 %d 次跑子程式：%v", i, err)
			}
		}
		if len(d.procStack) != 0 {
			t.Fatalf("第 %d 次的子程式沒有結束", i)
		}
		if d.freeSeg != freeBefore {
			t.Fatalf("第 %d 次之後配置游標是 %04X，預期回到 %04X——記憶體沒收回來",
				i, d.freeSeg, freeBefore)
		}
		if d.curPSP != pspBefore {
			t.Fatalf("第 %d 次之後 curPSP 是 %04X，預期回到 %04X", i, d.curPSP, pspBefore)
		}
		if m.CPU.R[cpu.SI] != 0xAAAA || m.CPU.R[cpu.DI] != 0xBBBB {
			t.Fatalf("第 %d 次之後父程式的 SI/DI 被弄髒：%04X/%04X",
				i, m.CPU.R[cpu.SI], m.CPU.R[cpu.DI])
		}
	}
}

// 子程式的離開碼由 `AH=4Dh` 交出去，而且**只交一次**。
//
// DOS 的語意是「取回上一支子程式的回傳碼」，讀過就清；不清的話殼會
// 對同一個碼反應兩次——那看起來像它自己邏輯有問題。
func TestExitCodeIsDeliveredOnce(t *testing.T) {
	m, d := newTest(t)
	writeChild(t, d, "B.COM", []byte{0xB8, 0x07, 0x4C, 0xCD, 0x21}) // exit 7
	execChild(m, d, "B.COM")
	for n := 0; n < 200 && len(d.procStack) > 0; n++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	call(m, d, 0x21, 0x4D00)
	if got := m.CPU.R[cpu.AX]; got != 0x0007 {
		t.Fatalf("AH=4Dh 回 %04X，預期 0007（AH ＝ 結束方式、AL ＝ 離開碼）", got)
	}
	call(m, d, 0x21, 0x4D00)
	if got := m.CPU.R[cpu.AX]; got == 0x0007 {
		t.Error("第二次還回同一個離開碼——讀過要清掉")
	}
}

// 子程式開的檔在它結束時要關掉。
//
// 漏關的症狀離現場很遠：跑久了 handle 用盡，`AH=3Dh` 回錯誤碼 4，
// 而錯誤發生在一支與洩漏者無關的程式裡。
func TestChildHandlesAreClosedOnExit(t *testing.T) {
	m, d := newTest(t)
	if err := os.WriteFile(filepath.Join(d.Root, "DATA.BIN"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 子程式：mov dx,offset name; mov ax,3D00h; int 21h; mov ax,4C00h; int 21h
	// 檔名擺在 0100h 之後的映像尾巴，位移由載入位置決定，所以直接用 DS:DX
	// 指到 PSP 的命令列尾那一段——這裡只要「有一個開著的檔」。
	child := []byte{
		0xBA, 0x20, 0x01, // mov dx,0120h
		0xB8, 0x00, 0x3D, // mov ax,3D00h
		0xCD, 0x21, // int 21h
		0xB8, 0x00, 0x4C, // mov ax,4C00h
		0xCD, 0x21, // int 21h
	}
	// 把檔名塞到映像的 0120h（相對 .COM 的 0100h 起點是第 0x20 個位元組）。
	img := make([]byte, 0x20)
	copy(img, child)
	img = append(img, []byte("DATA.BIN\x00")...)
	writeChild(t, d, "C.COM", img)

	execChild(m, d, "C.COM")
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("EXEC 失敗：Missing=%v", d.Missing)
	}
	for n := 0; n < 500 && len(d.procStack) > 0; n++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if len(d.procStack) != 0 {
		t.Fatal("子程式沒有結束")
	}
	if len(d.handles) != 0 {
		t.Errorf("子程式結束後還有 %d 個 handle 開著", len(d.handles))
	}
}

// 覆疊載入（`AL=03h`）**不建立 PSP、不改 CS:IP**：它只是把映像搬進
// 呼叫端指定的段。跳進去是呼叫端自己的事。
//
// 替呼叫端跳進去的話，主程式的下一道指令永遠不會執行——
// 而畫面停在覆疊自己的迴圈裡，看起來像覆疊掛了。
func TestOverlayLoadsWithoutTakingControl(t *testing.T) {
	m, d := newTest(t)
	// 一支只有一個重定位項的 MZ：重定位項指到映像的 0010h。
	img := overlayMZ(t)
	if err := os.WriteFile(filepath.Join(d.Root, "OVL.EXE"), img, 0o644); err != nil {
		t.Fatal(err)
	}

	const loadSeg, fixup = 0x4000, 0x4000
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = 0x3000, 0
	m.WriteBytes(cpu.Addr(0x3000, 0), append([]byte("OVL.EXE"), 0))
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BX] = 0x3000, 0x80
	m.Write16(cpu.Addr(0x3000, 0x80), loadSeg)
	m.Write16(cpu.Addr(0x3000, 0x82), fixup)
	m.CPU.Seg[cpu.CS], m.CPU.IP = 0x1234, 0x5678

	call(m, d, 0x21, 0x4B03)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("覆疊載入失敗：AX=%04X", m.CPU.R[cpu.AX])
	}
	if m.CPU.R[cpu.AX] != 0 {
		t.Errorf("成功時 AX ＝ %04X，預期 0", m.CPU.R[cpu.AX])
	}
	if m.CPU.Seg[cpu.CS] != 0x1234 || m.CPU.IP != 0x5678 {
		t.Fatalf("覆疊載入把控制權拿走了：CS:IP ＝ %04X:%04X",
			m.CPU.Seg[cpu.CS], m.CPU.IP)
	}
	if got := m.Read16(uint32(loadSeg)*16 + 0x10); got != 0x1111+fixup {
		t.Errorf("重定位項是 %04X，預期 %04X（0x1111 ＋ 重定位因子）", got, 0x1111+fixup)
	}
	if len(d.Overlays) != 1 {
		t.Errorf("Overlays 記了 %d 筆——沒有這份紀錄就看不出程式載過哪些模組",
			len(d.Overlays))
	}
}

// 缺檔要**失敗即關閉**：CF=1、AX=2，而且把名字記進 Missing。
//
// 回成功的話呼叫端會 far call 進一片空白，然後死在一個與這裡毫無關聯的位址。
func TestExecMissingFileFailsClosed(t *testing.T) {
	m, d := newTest(t)
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = 0x3000, 0
	m.WriteBytes(cpu.Addr(0x3000, 0), append([]byte("NOPE.EXE"), 0))
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BX] = 0x3000, 0x80

	for _, al := range []uint16{0x00, 0x03} {
		before := len(d.Missing)
		call(m, d, 0x21, 0x4B00|al)
		if m.CPU.Flags&cpu.CF == 0 || m.CPU.R[cpu.AX] != 2 {
			t.Errorf("AL=%02X 缺檔回 CF=%t AX=%04X，預期 CF=1 AX=2",
				al, m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.AX])
		}
		if len(d.Missing) == before {
			t.Errorf("AL=%02X 缺檔沒有記進 Missing", al)
		}
	}
}

// overlayMZ 造一支最小的覆疊用 MZ：0010h 放一個待重定位的段值。
func overlayMZ(t *testing.T) []byte {
	t.Helper()
	code := make([]byte, 0x20)
	code[0x10] = 0x11
	code[0x11] = 0x11 // 0x1111
	return buildMZForOverlay(code, 0x10)
}

func buildMZForOverlay(code []byte, relocAt uint16) []byte {
	const hdrParas = 2
	hdr := make([]byte, hdrParas*16)
	total := len(hdr) + len(code)
	put := func(off int, v uint16) {
		hdr[off] = uint8(v)
		hdr[off+1] = uint8(v >> 8)
	}
	copy(hdr, "MZ")
	put(0x02, uint16(total%512))
	put(0x04, uint16((total+511)/512))
	put(0x06, 1)
	put(0x08, hdrParas)
	put(0x0A, 0)
	put(0x0C, 0xFFFF)
	put(0x10, 0x0100)
	put(0x18, 0x1C)
	put(0x1C, relocAt)
	put(0x1E, 0)
	return append(hdr, code...)
}

// 監督佇列：疊底的程式結束之後，排隊的下一支要接著跑。
//
// 這是「殼鏈」的模型——沒有它，第一支程式結束就等於整台結束，
// 而那與「鏈跑完了」看起來一樣。
func TestSupervisorQueueRunsTheNextProgram(t *testing.T) {
	m, d := newTest(t)
	writeChild(t, d, "NEXT.COM", []byte{0xB8, 0x05, 0x4C, 0xCD, 0x21})
	d.Enqueue("NEXT.COM", "")

	// 疊底程式直接結束。
	call(m, d, 0x21, 0x4C00)
	if d.Exited {
		t.Fatal("佇列裡還有程式就宣告整台結束了")
	}
	if d.curPSP == 0 {
		t.Fatal("下一支程式沒有拿到 PSP")
	}
	for n := 0; n < 500 && !d.Exited; n++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if !d.Exited || d.ExitCode != 5 {
		t.Fatalf("佇列裡那一支的離開碼是 %d（Exited=%t），預期 5", d.ExitCode, d.Exited)
	}
}

// 子程式用 `AH=48h` 拿走的區塊，在它結束時要**連同假 MCB 標頭**一起消失。
//
// 反面不會報錯：arena 留著那幾塊，`syncMCB` 就繼續把 `'M'`＋擁有者＋大小
// ＋8 個空白寫進客體記憶體。接手的程式把自己的區塊撐大蓋過那一段之後，
// 那 16 個位元組落在它的程式碼裡——某道指令的立即數被換掉，堆疊從此歪掉，
// 幾十萬道指令之後 `retf` 進垃圾。症狀與這裡毫無關聯。
// （logh3 `docs/re/270`：`add sp,24h` 變成 `add sp,4Dh`。）
func TestChildBlocksAndTheirMCBsVanishOnExit(t *testing.T) {
	m, d := newTest(t)
	// 子程式：AH=48h 要 0x100 段，然後 AH=4Ch 結束。
	writeChild(t, d, "C.COM", []byte{
		0xB4, 0x48, 0xBB, 0x00, 0x01, 0xCD, 0x21, // mov ah,48; mov bx,100h; int 21h
		0xB8, 0x00, 0x4C, 0xCD, 0x21, // mov ax,4C00h; int 21h
	})
	execChild(m, d, "C.COM")
	for n := 0; n < 500 && len(d.procStack) > 0; n++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if len(d.procStack) != 0 {
		t.Fatal("子程式沒有結束")
	}
	for _, b := range d.arena {
		if b.free {
			continue
		}
		t.Fatalf("子程式結束後 arena 還留著已配置的 %04X（%d 段，擁有者 %04X）"+
			"——它的假 MCB 標頭會留在客體記憶體裡", b.seg, b.size, b.owner)
	}
	// 標頭本身也要不見：那一段記憶體不該再有 MCB 簽章。
	for _, b := range d.arena {
		if sig := m.Read8(uint32(b.seg)*16) & 0xFF; b.free && sig != 'M' && sig != 'Z' {
			t.Fatalf("自由區塊 %04X 的 MCB 簽章是 %02X，鏈斷了", b.seg, sig)
		}
	}
}
