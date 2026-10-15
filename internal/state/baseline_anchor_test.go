package state

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// 前置錨點測試（docs/spec/429-dos-sft-chain §5 第 2 項 (b)、
// docs/spec/430-physical-memory-and-xms-unification §5 第 1 項 (b)）。
//
// 這些常數由「尚未實作 SFT 與實體記憶體統一」的 loader 算出，內容是固定的最小
// 程式載入後的整機狀態。之後的實作 commit 在預設選項下（SetDOSFiles(0)、
// extended memory 0、未呼叫 SetEMSPresent）必須讓它們維持不變。
//
// 語意錨點（記憶體雜湊、兩個 StateDigest）永不變；gob 位元組可因 430 新增欄位的
// 型別描述改變一次（本 commit）。MachineState、DOSState 兩個常數已在該 commit 更新，
// 之後的行為 commit 不得再動。
//
// 改動任一個常數，等於宣告「預設行為改變」：既有收據記的是當時的記憶體內容，
// 預設值一變，舊收據重跑會得到另一個畫面而沒有任何測試報錯
// （CLAUDE.md「為了重跑既有收據而存在的機制」）。要改請先在規格裡寫明原因。
//
// 夾具刻意不讀原版檔；程式以 bytes 現場組出。MemPoison 是全域變數，
// 相關測試以 t.Cleanup 還原，不得使用 t.Parallel。

// anchorRoot 是 dos.SaveState 會記下的 Root；固定字串避免 t.TempDir() 的路徑進入狀態檔。
const anchorRoot = "/anchor"

// anchorCOM：mov ah,4Ch / int 21h。
var anchorCOM = []byte{0xB4, 0x4C, 0xCD, 0x21}

// anchorEXE 組出最小 MZ：檔頭 2 段（32 bytes）、映像 16 bytes，
// 進入點 0000:0000 放同樣的 mov ah,4Ch / int 21h，SS:SP = 0000:0100，無重定位。
func anchorEXE() []byte {
	const hdrPar, imgLen = 2, 16
	b := make([]byte, hdrPar*16+imgLen)
	copy(b, "MZ")
	le := binary.LittleEndian
	total := len(b)
	le.PutUint16(b[2:], uint16(total%512)) // e_cblp
	le.PutUint16(b[4:], uint16((total+511)/512))
	le.PutUint16(b[6:], 0)      // 重定位項數
	le.PutUint16(b[8:], hdrPar) // 檔頭段數
	le.PutUint16(b[10:], 0)     // min alloc
	le.PutUint16(b[12:], 0xFFFF)
	le.PutUint16(b[14:], 0)     // SS
	le.PutUint16(b[16:], 0x100) // SP
	le.PutUint16(b[20:], 0)     // IP
	le.PutUint16(b[22:], 0)     // CS
	le.PutUint16(b[24:], 0x1C)  // 重定位表位移
	copy(b[hdrPar*16:], anchorCOM)
	return b
}

type anchorSums struct {
	Mem, MachineState, MachineDigest, DOSState, DOSDigest string
}

func hex(b []byte) string { return fmt.Sprintf("%x", b) }

func anchorMeasure(t *testing.T, load func(*machine.Machine) error) anchorSums {
	t.Helper()
	m := machine.New()
	if err := load(m); err != nil {
		t.Fatal(err)
	}
	d := dos.New(m, anchorRoot)
	d.Install()
	d.Root = anchorRoot

	var s anchorSums
	mem := sha256.Sum256(m.Mem)
	s.Mem = hex(mem[:])

	var mb bytes.Buffer
	if err := m.SaveState(&mb); err != nil {
		t.Fatal(err)
	}
	ms := sha256.Sum256(mb.Bytes())
	s.MachineState = hex(ms[:])

	dg, err := machine.StateDigest(bytes.NewReader(mb.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	s.MachineDigest = hex(dg[:])

	var db bytes.Buffer
	if err := d.SaveState(&db); err != nil {
		t.Fatal(err)
	}
	ds := sha256.Sum256(db.Bytes())
	s.DOSState = hex(ds[:])

	dd, err := dos.StateDigest(bytes.NewReader(db.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	s.DOSDigest = hex(dd[:])
	return s
}

func anchorCheck(t *testing.T, name string, got, want anchorSums) {
	t.Helper()
	t.Logf("%s: %+v", name, got)
	if got != want {
		t.Errorf("%s 錨點不符\n got  %+v\n want %+v", name, got, want)
	}
}

var (
	anchorEXEWant = anchorSums{
		Mem:           "4f3a968fef00769408fc8a6e7c820b095ac78143c2b9e4069e66192f6666a4ad",
		MachineState:  "b26e47038326be86935e5d1fb9a6e39021ad77d521487d30da41b3b10997e7f0",
		MachineDigest: "f1fc9fcfacaa82f4df011708d6bd95999ac69c467e70093d402c062f81db3b55",
		DOSState:      "01227c1c9f97333e289e682321d67afcdcc7c7b67e809da6da88b6aab533a47b",
		DOSDigest:     "e24dd93659449209d6fa38f672a9aee05e7ce223ab3fbb105d16cc0874ffee02",
	}
	anchorCOMWant = anchorSums{
		Mem:           "396efb9c4be37c09147ab292bf12d9e4447b560ad3069511a7ebd539f2464b1d",
		MachineState:  "efd1997c178c19cfd65258c7dcfdc72f98303c1579d3c888eb824f62c75947cb",
		MachineDigest: "1927242326e91a008d480bd5ad35e1118e54dc554ac71b728193fe4df8fc7579",
		DOSState:      "8cda094fe2bef9eaa3c91d4af6eb0b0bcbfe6ebc7a739fc79d2e45eb3fba614e",
		DOSDigest:     "fcf10f07497ae2a4bff738e45a5354483518f359a8de02c368e734cdb8319278",
	}
)

const anchorEXEPoisonMem = "e2fe4ef95e27e7ad207d76a5681275618d80fce53e8cc1dee04a75fb3e00dd60"

func TestAnchorBaselineEXE(t *testing.T) {
	got := anchorMeasure(t, func(m *machine.Machine) error { return m.LoadEXE(anchorEXE()) })
	anchorCheck(t, "EXE", got, anchorEXEWant)
}

func TestAnchorBaselineCOM(t *testing.T) {
	got := anchorMeasure(t, func(m *machine.Machine) error { return m.LoadCOM(anchorCOM) })
	anchorCheck(t, "COM", got, anchorCOMWant)
}

func TestAnchorBaselineEXEPoison(t *testing.T) {
	old := machine.MemPoison
	machine.MemPoison = 0xCC
	t.Cleanup(func() { machine.MemPoison = old })

	m := machine.New()
	if err := m.LoadEXE(anchorEXE()); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(m.Mem)
	got := hex(sum[:])
	t.Logf("EXE poison 0xCC Mem: %s", got)
	if got != anchorEXEPoisonMem {
		t.Errorf("EXE MemPoison=0xCC 的 m.Mem 錨點不符\n got  %s\n want %s", got, anchorEXEPoisonMem)
	}
}
