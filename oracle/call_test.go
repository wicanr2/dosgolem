package oracle_test

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
)

// 合成映像裡各段程式碼的位移。IDA 線性位址 ＝ 0x10000 ＋ 位移
//（`Load` 就是這樣定 `idaOffset` 的）。
const (
	synthEntry = 0x00 // `jmp $`：進入點，這份測試不跑它
	synthAdd   = 0x10 // far：回 arg0 ＋ arg1，DX ＝ 1234h
	synthConst = 0x20 // far：回 4321h:BEEFh，不吃參數
	synthLoop  = 0x30 // `jmp $`：永遠不回來
)

// synthCode 是那四段的機器碼。
//
// **合成的，不是原版。** 這一份測試因此在任何機器上都跑得起來，
// 不必玩家自備素材——`Call` 是新的契約，沒有回歸保護不行。
func synthCode() []byte {
	code := make([]byte, 0x400) // 1 KB：程式碼在最前面，其餘當堆疊
	copy(code[synthEntry:], []byte{0xEB, 0xFE})
	copy(code[synthAdd:], []byte{
		0x55,             // push bp
		0x8B, 0xEC, //       mov  bp, sp
		0x8B, 0x46, 0x06, // mov  ax, [bp+6]   ; arg0
		0x03, 0x46, 0x08, // add  ax, [bp+8]   ; arg1
		0xBA, 0x34, 0x12, // mov  dx, 1234h
		0x5D,             // pop  bp
		0xCB, //             retf
	})
	copy(code[synthConst:], []byte{
		0xBA, 0x21, 0x43, // mov dx, 4321h
		0xB8, 0xEF, 0xBE, // mov ax, 0BEEFh
		0xCB, //             retf
	})
	copy(code[synthLoop:], []byte{0xEB, 0xFE})
	return code
}

// synthEXE 把那段程式碼包成一支最小的 MZ 執行檔，寫進暫存目錄。
//
// SS 與 CS 同段、SP ＝ 0x400：堆疊從映像頂端往下長，離程式碼（0…0x40）
// 遠得很，`Call` 推的那幾個 word 不會蓋到它。
func synthEXE(t *testing.T) string {
	t.Helper()
	code := synthCode()
	const hdrPar = 2
	hdr := make([]byte, hdrPar*16)
	total := len(hdr) + len(code)

	put := func(off int, v uint16) { binary.LittleEndian.PutUint16(hdr[off:], v) }
	copy(hdr, "MZ")
	put(0x02, uint16(total%512)) // 最後一頁的 bytes
	put(0x04, uint16((total+511)/512))
	put(0x06, 0)      // 沒有重定位
	put(0x08, hdrPar) // 檔頭段數
	put(0x0A, 0x10)   // MinAlloc
	put(0x0C, 0xFFFF) // MaxAlloc
	put(0x0E, 0)      // SS（相對載入段）
	put(0x10, 0x0400) // SP
	put(0x14, synthEntry)
	put(0x16, 0)      // CS（相對載入段）
	put(0x18, 0x001C) // 重定位表位移
	put(0x1A, 0)      // overlay

	path := filepath.Join(t.TempDir(), "SYNTH.EXE")
	if err := os.WriteFile(path, append(hdr, code...), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func synth(t *testing.T) *oracle.Oracle {
	t.Helper()
	o, err := oracle.Load(synthEXE(t), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(o.Close)
	return o
}

// 回傳值是 DX:AX，參數照 C 的 far 慣例由右到左推。
func TestCallReturnsDXAX(t *testing.T) {
	o := synth(t)
	got, err := o.Call(o.IDA(0x10000 + synthConst))
	if err != nil {
		t.Fatal(err)
	}
	if want := uint32(0x4321BEEF); got != want {
		t.Fatalf("回傳 %#08x，期望 %#08x", got, want)
	}
}

func TestCallPassesArgs(t *testing.T) {
	o := synth(t)
	got, err := o.Call(o.IDA(0x10000+synthAdd), 3, 4)
	if err != nil {
		t.Fatal(err)
	}
	if want := uint32(0x12340007); got != want {
		t.Fatalf("3＋4 回 %#08x，期望 %#08x（AX 該是 7、DX 該是 1234h）", got, want)
	}
}

// 呼叫完暫存器要回到原樣——不然一連串呼叫會把機器帶到別的地方去。
func TestCallRestoresRegisters(t *testing.T) {
	o := synth(t)
	before := o.IP()
	if _, err := o.Call(o.IDA(0x10000+synthAdd), 1, 2); err != nil {
		t.Fatal(err)
	}
	if after := o.IP(); after != before {
		t.Fatalf("呼叫之後 CS:IP 變成 %s，呼叫前是 %s", after, before)
	}
	// 再呼叫一次要拿到一樣的結果：狀態真的乾淨。
	got, err := o.Call(o.IDA(0x10000+synthAdd), 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if want := uint32(0x12340003); got != want {
		t.Fatalf("第二次呼叫回 %#08x，期望 %#08x", got, want)
	}
}

// 回不來要**報錯**，不是靜靜地回一個看起來合理的值。
func TestCallGivesUpOnRunaway(t *testing.T) {
	o := synth(t)
	if _, err := o.CallBudget(10_000, o.IDA(0x10000+synthLoop)); err == nil {
		t.Fatal("永遠不回來的常式竟然回來了")
	}
}

// hook 在 Call 期間照常觸發——這是「誰呼叫了誰」那條線的前提。
func TestCallFiresHooks(t *testing.T) {
	o := synth(t)
	var hits int
	o.OnCall(o.IDA(0x10000+synthAdd), func(*oracle.Oracle) { hits++ })
	for i := 0; i < 3; i++ {
		if _, err := o.Call(o.IDA(0x10000+synthAdd), 1, 1); err != nil {
			t.Fatal(err)
		}
	}
	if hits != 3 {
		t.Fatalf("hook 觸發 %d 次，期望 3", hits)
	}
}

// Stub 讓常式不執行，直接回指定的值。
//
// 判準是**副作用沒有發生**：`synthConst` 自己會把 DX 設成 4321h，stub 掉之後
// 回傳的高位必須是 stub 給的，不是常式給的。只比 AX 的話兩者分不出來。
func TestStubReplacesRoutine(t *testing.T) {
	o := synth(t)
	o.StubValue(o.IDA(0x10000+synthConst), 0x00FF0042)
	got, err := o.Call(o.IDA(0x10000 + synthConst))
	if err != nil {
		t.Fatal(err)
	}
	if want := uint32(0x00FF0042); got != want {
		t.Fatalf("回 %#08x，期望 %#08x（常式跑掉了？）", got, want)
	}

	// 取消之後回到常式自己的值。
	o.Stub(o.IDA(0x10000+synthConst), nil)
	got, err = o.Call(o.IDA(0x10000 + synthConst))
	if err != nil {
		t.Fatal(err)
	}
	if want := uint32(0x4321BEEF); got != want {
		t.Fatalf("取消 stub 之後回 %#08x，期望 %#08x", got, want)
	}
}

// stub 裡讀得到參數——SP 版面要與常式進入的那一刻相同。
func TestStubSeesArgs(t *testing.T) {
	o := synth(t)
	var seen [2]uint16
	o.Stub(o.IDA(0x10000+synthAdd), func(p *oracle.Oracle) uint32 {
		seen[0], seen[1] = p.Arg(0), p.Arg(1)
		return 0
	})
	if _, err := o.Call(o.IDA(0x10000+synthAdd), 11, 22); err != nil {
		t.Fatal(err)
	}
	if seen[0] != 11 || seen[1] != 22 {
		t.Fatalf("stub 讀到 (%d, %d)，期望 (11, 22)", seen[0], seen[1])
	}
}

// 被 stub 的常式在迴圈裡也不會把預算耗光——一次 stub 算一道指令。
func TestStubCountsAsOneStep(t *testing.T) {
	o := synth(t)
	o.StubValue(o.IDA(0x10000+synthLoop), 1)
	before := o.Steps()
	if _, err := o.Call(o.IDA(0x10000 + synthLoop)); err != nil {
		t.Fatalf("stub 掉的無窮迴圈應該立刻回來：%v", err)
	}
	if n := o.Steps() - before; n != 1 {
		t.Fatalf("走了 %d 道指令，期望 1", n)
	}
}
