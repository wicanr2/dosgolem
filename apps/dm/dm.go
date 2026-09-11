// Package dm 是《Dungeon Master》（FTL Games 1987，DOS 版 1989）專屬的
// 巡檢 adapter（`docs/spec/198`）。
//
// 這一層放的是**這一支 binary 的知識**（`docs/spec/006` 的分層判準）：
// 位址表、結構偏移、開機流程。機器層與觀測層看不到它們。
//
//	o, _ := dm.Load(exe, root)
//	dm.Boot(o)
//	b, _ := dm.Bind(o)
//	p, _ := b.ReadParty()
//
// ⚠ **不含任何原版檔案**，素材由玩家自備。
//
// ⚠ **巡檢跑出來的不是 parity 證據**（`docs/spec/198` §1）：它證明的是
// 「這一層的畫面、物品與感應器在兩側對得上」，不是「正常玩家玩起來一樣」。
package dm

import (
	"fmt"

	"github.com/wicanr2/dosgolem/oracle"
)

// 啟動鏈。`dm.exe` 是殼，它 EXEC `selector`（問顯示卡與音效），
// `selector` 再 EXEC `fires`——**遊戲引擎與所有位址都在 `fires` 裡**。
const (
	// ShellName 是 Load 要給的最外層執行檔（basename）。
	ShellName = "dm.exe"
	// EngineName 是引擎的 basename，`Bind` 拿它在 EXEC 紀錄裡認人。
	EngineName = "fires"
	// SelectorAnswers 是 `selector` 三個問題的答案（EGA／無音效／…）。
	// 送錯的話畫面會停在問題上，而 `Boot` 只會跑滿預算。
	SelectorAnswers = "111"
)

// 畫面幾何。DM 的 DOS 版是 mode 13h。
const (
	ScreenW, ScreenH = 320, 200
)

// DGroupParaOffset 是 DGROUP 相對於 `fires` 映像起點的**節**（16 位元組）數。
//
// 由段表推出來：`(DSegBase − 0x10000) / 16`，其中 `0x10000` 是 IDA 放 MZ
// 映像的位置。這與 dungeon_master 專案兩個實測值相減的結果一致——
// DS 基底的執行期線性 `0x2B320` 減映像線性 `0x6450` 也是 `0x24ED0`
// （`docs/re/71`、`docs/re/73`）。
//
// ⚠ 這是 linker 決定的、跟著執行檔走的常數，**不是**跟著 DOS 記憶體配置走的。
// 配置變了的是映像段，`Bind` 每次從 EXEC 紀錄重算，不寫死。
const DGroupParaOffset = (DSegBase - 0x10000) / 16

// Bridge 是「IDA 位址 ↔ 這一次執行」的換算，以及建立在它上面的讀寫。
//
// ⚠ **不要用 `oracle.Oracle` 自己的 `IDA()` 與 `DS()`。** 那兩支的基底是
// **最外層**程式（`dm.exe`）的載入位置，而 DM 的位址全部屬於被 EXEC 兩層的
// `fires`。用錯基底讀到的是另一塊記憶體，**而且不會報錯**。
type Bridge struct {
	O *oracle.Oracle

	imageSeg  uint16 // fires 的映像段
	idaOffset uint32 // IDA 線性 ＝ 執行期線性 ＋ idaOffset
	dgroupSeg uint16
}

// Bind 從 EXEC 紀錄算出 `fires` 的位址基底，並用程式自己的 `DS` 驗證。
//
// ⚠ **設完不驗等於沒設**（`oracle.SetDGroup` 的說明）。這裡的驗證是
// 「算出來的 DGROUP 段要等於程式現在的 `DS`」——兩個獨立來源，
// 一個從 EXEC 紀錄推、一個是 CPU 的現況。
//
// 所以 **`Bind` 要在 `fires` 已經進到主迴圈之後呼叫**：C 的啟動碼跑完
// `DS` 才是 DGROUP。開機中途呼叫會拿到「算得出來但驗不過」的錯誤，
// 那正是要的行為——不要讓它安靜地回一個錯的基底。
func Bind(o *oracle.Oracle) (*Bridge, error) {
	var psp uint16
	found := false
	for _, r := range o.ExecLog() {
		if !equalFold(r.Base, EngineName) || r.Exit != 0xFF {
			continue
		}
		psp, found = r.PSP, true
	}
	if !found {
		return nil, fmt.Errorf("EXEC 紀錄裡沒有還在跑的 %q（走到哪了：%v）",
			EngineName, execNames(o))
	}
	image := psp + 0x10 // EXE 映像接在 PSP 的 16 節之後
	b := &Bridge{
		O:         o,
		imageSeg:  image,
		idaOffset: 0x10000 - uint32(image)*16,
		dgroupSeg: image + DGroupParaOffset,
	}
	if ds := o.DSReg(); ds != b.dgroupSeg {
		return nil, fmt.Errorf(
			"DGROUP 算出來是 %04X，程式現在的 DS 是 %04X——"+
				"要嘛還沒跑進 fires 的主迴圈，要嘛 DS 正被暫時改掉（遠指標、字串搬移、中斷）",
			b.dgroupSeg, ds)
	}
	return b, nil
}

// IDA 把 dungeon_master 專案筆記裡的五位 IDA 線性位址換成這一次執行的位址。
func (b *Bridge) IDA(linear uint32) oracle.Addr {
	run := linear - b.idaOffset
	return oracle.Far(uint16(run>>4), uint16(run&0xF))
}

// DS 把 `ds:XXXX` 換成這一次執行的位址。
func (b *Bridge) DS(off uint16) oracle.Addr { return oracle.Far(b.dgroupSeg, off) }

// IDAOffset 是這一次執行的換算常數。實測值是 `0x9BB0`——
// 對得上就表示 DOS 的記憶體配置與 dungeon_master 專案取位址時相同。
func (b *Bridge) IDAOffset() uint32 { return b.idaOffset }

// ImageSeg 是 `fires` 的映像段。
func (b *Bridge) ImageSeg() uint16 { return b.imageSeg }

// DGroupSeg 是 `fires` 的 DGROUP 段。
func (b *Bridge) DGroupSeg() uint16 { return b.dgroupSeg }

func execNames(o *oracle.Oracle) []string {
	var out []string
	for _, r := range o.ExecLog() {
		out = append(out, r.Base)
	}
	return out
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if 'A' <= x && x <= 'Z' {
			x += 'a' - 'A'
		}
		if 'A' <= y && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}
