package dm

import (
	"fmt"

	"github.com/wicanr2/dosgolem/oracle"
)

// codeSeg 是 `fires` 的一個程式碼段（IDA 線性位址）。
type codeSeg struct {
	Name       string
	Start, End uint32 // IDA 線性範圍，左閉右開
	Base       uint32 // 段基底的 IDA 線性位址（＝ selector × 16）
}

// codeSegs 是 `fires` 的完整程式碼段表，由 IDA 匯出
// （dungeon_master 專案 `tools/dm_segs.py`，`workplace/ida/segs.txt`）。
//
// **為什麼需要它**：`fires` 是多程式碼段的 C 程式，`sub_1E063`（`F309`）
// 在 `seg010`、主迴圈 `sub_10C94` 在 `seg001`。要**跳進去執行**一支常式
// 就得給對的 `CS`——常式裡的 `call near ptr`、`jmp cs:jpt_…[bx]` 全部是
// 相對段基底算的。
//
// ⚠ **`oracle.IDA()` 回的是正規化的段**（線性 >> 4，偏移 0–15）。
// 拿它當 `CS` 去 `Call`，機器碼的位置是對的，但第一個 near call 就跳到
// 別的地方——**而且不會報錯**，只會在別處跑滿預算。實測過一次：
// 停在 `seg028` 的繪圖常式裡。
//
// 讀寫記憶體、`OnCall`、`Stub` 都只看**線性位址**，不受這一層影響。
var codeSegs = [...]codeSeg{
	{"seg000", 0x10000, 0x10C5B, 0x10000},
	{"seg001", 0x10C5B, 0x10F65, 0x10C50},
	{"seg002", 0x10F65, 0x12CF8, 0x10F60},
	{"seg003", 0x12CF8, 0x13620, 0x12CF0},
	{"seg004", 0x13620, 0x14FE0, 0x13620},
	{"seg005", 0x14FE0, 0x155A9, 0x14FE0},
	{"seg006", 0x155A9, 0x185D7, 0x155A0},
	{"seg007", 0x185D7, 0x1B3B3, 0x185D0},
	{"seg008", 0x1B3B3, 0x1B8B5, 0x1B3B0},
	{"seg009", 0x1B8B5, 0x1D06D, 0x1B8B0},
	{"seg010", 0x1D06D, 0x1F6A6, 0x1D060},
	{"seg011", 0x1F6A6, 0x204F1, 0x1F6A0},
	{"seg012", 0x204F1, 0x21262, 0x204F0},
	{"seg013", 0x21262, 0x22F90, 0x21260},
	{"seg014", 0x22F90, 0x23BE2, 0x22F90},
	{"seg015", 0x23BE2, 0x23E2D, 0x23BE0},
	{"seg016", 0x23E2D, 0x24265, 0x23E20},
	{"seg017", 0x24265, 0x24C00, 0x24260},
	{"seg018", 0x24C00, 0x269D0, 0x24C00},
	{"seg019", 0x269D0, 0x27117, 0x269D0},
	{"seg020", 0x27117, 0x27719, 0x27110},
	{"seg021", 0x27719, 0x27D14, 0x27710},
	{"seg022", 0x27D14, 0x2963C, 0x27D10},
	{"seg023", 0x2963C, 0x2B128, 0x29630},
	{"seg024", 0x2B128, 0x2B9B2, 0x2B120},
	{"seg025", 0x2B9B2, 0x2BC1D, 0x2B9B0},
	{"seg026", 0x2BC1D, 0x2C7A6, 0x2BC10},
	{"seg027", 0x2C7A6, 0x30D68, 0x2C7A0},
	{"seg028", 0x30D68, 0x32DD7, 0x30D60},
	{"seg029", 0x32DD7, 0x336EC, 0x32DD0},
	{"seg030", 0x336EC, 0x3390D, 0x336E0},
	{"seg031", 0x3390D, 0x33B19, 0x33900},
	{"seg032", 0x33B19, 0x33C42, 0x33B10},
	{"seg033", 0x33C42, 0x33CC6, 0x33C40},
	{"seg034", 0x33CC6, 0x34EC0, 0x33CC0},
}

// DSegBase 是資料段的 IDA 線性基底，同一張段表裡的 `dseg`。
//
// 這是 [DGroupParaOffset] 的來源：`(0x34ED0 − 0x10000) / 16 ＝ 0x24ED`，
// 而 `0x10000` 是 IDA 放 MZ 映像的地方。兩個數字由同一張表推出來，
// 不是兩個實測值相減。
const DSegBase = 0x34ED0

// Code 把 IDA 線性位址換成**可以跳進去執行**的位址：段用它自己所屬的
// 程式碼段，偏移相對那個段基底。
//
// 用在 `oracle.Call`；讀寫與掛勾請用 [Bridge.IDA]。
func (b *Bridge) Code(linear uint32) (oracle.Addr, error) {
	for _, s := range codeSegs {
		if linear < s.Start || linear >= s.End {
			continue
		}
		off := linear - s.Base
		if off > 0xFFFF {
			return oracle.Addr{}, fmt.Errorf(
				"IDA %#X 在 %s 裡的偏移 %#X 超過 64 KB——段表壞了", linear, s.Name, off)
		}
		seg := (s.Base - b.idaOffset) / 16
		return oracle.Far(uint16(seg), uint16(off)), nil
	}
	return oracle.Addr{}, fmt.Errorf("IDA %#X 不在任何程式碼段裡", linear)
}

// CodeSegName 回某個 IDA 線性位址所屬的段名（除錯訊息用）。
func CodeSegName(linear uint32) string {
	for _, s := range codeSegs {
		if linear >= s.Start && linear < s.End {
			return s.Name
		}
	}
	return "?"
}
