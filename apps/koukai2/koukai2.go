// Package koukai2 是《大航海時代 II》DOS 版（光榮）專屬的導航與對拍支援。
//
// 這一層放的是**這一支 binary 的知識**（`docs/spec/006-layering`）：載入設定、
// `MAIN.EXE` 的 IDA 位址換算。規則語意與位址的證據在 koukai2 專案的
// `docs/spec/`，這裡不重抄。
//
// ⚠ 不含任何原版檔案，素材由玩家自備。
package koukai2

import (
	"fmt"
	"strings"

	"github.com/wicanr2/dosgolem/oracle"
)

// 畫面幾何：模式 12h（640×480），遊戲區 640×400，VRAM 起點第 40 列
// （koukai2 `docs/spec/023-display-model`）。
const (
	ScreenW, ScreenH        = 640, 480
	ContentTop, ContentHigh = 40, 400
)

// 主要程式的 IDA 資料段線性基底（koukai2 `docs/spec/000-common`）。
//
// IDA 把 MZ 映像載在線性 0x10000，所以 `DS:xxxx` 的線性位址 ＝ 這個值 ＋ xxxx。
const (
	MainDGroup uint32 = 0x48150
	OpenDGroup uint32 = 0x1A0B0
)

// Load 載入 `KOUKAI2.COM`。root 是原版素材目錄（玩家自備）。
//
// 兩個設定弄錯都不會報錯：
//
//   - Adlib：`FMDRV.COM` 走 OPL 埠 388h，不開時音樂路徑不同。
//   - MouseXScale ＝ 1：遊戲把滑鼠範圍設成 X 0..639，與像素 1:1。
func Load(root string) (*oracle.Oracle, error) {
	return oracle.LoadWith(root+"/KOUKAI2.COM", root, oracle.Options{
		Adlib:       true,
		MouseXScale: 1,
		COM:         true,
	})
}

// Bind 是某一支被 EXEC 的程式的位址換算。
//
// **位址屬於被 EXEC 的程式**，基底要從 EXEC 紀錄算（PSP＋10h 是映像段），
// 不能用 `o.IDA`：那一支是以最外層 `KOUKAI2.COM` 的載入段換算的，
// 拿來讀 `MAIN.EXE` 不會報錯，只會讀到另一塊記憶體。
type Bind struct {
	Name    string
	PSP     uint16
	Running bool
}

// Current 回最後一支載入、還沒結束的程式。TSR（FMDRV.COM）不算。
func Current(o *oracle.Oracle) (Bind, bool) {
	log := o.ExecLog()
	for i := len(log) - 1; i >= 0; i-- {
		r := log[i]
		if r.TSR || r.Ended {
			continue
		}
		return Bind{Name: strings.ToUpper(r.Base), PSP: r.PSP, Running: true}, true
	}
	return Bind{}, false
}

// IsMain 回目前是否跑到 `MAIN.EXE`。
func IsMain(o *oracle.Oracle) bool {
	b, ok := Current(o)
	return ok && b.Name == "MAIN.EXE"
}

// loadSeg 是 EXE 映像段，IDA 線性 0x10000 對應到這裡。
func (b Bind) loadSeg() uint16 { return b.PSP + 0x10 }

// Code 把 IDA 線性位址換成執行期位址（段取 IDA 段換算，偏移取低 4 位元以內）。
func (b Bind) Code(linear uint32) oracle.Addr {
	off := linear - 0x10000
	return oracle.Addr{Seg: b.loadSeg() + uint16(off>>4), Off: uint16(off & 0xF)}
}

// Seg 回 IDA 段（如 0x4815）換算後的執行期段值。
func (b Bind) Seg(idaSeg uint16) uint16 { return idaSeg - 0x1000 + b.loadSeg() }

// DS 回 `MAIN.EXE` 的 `DS:off`。
func (b Bind) DS(off uint16) oracle.Addr {
	return oracle.Addr{Seg: b.Seg(uint16(MainDGroup >> 4)), Off: off}
}

// ToIDA 把執行期線性位址換回 IDA 線性位址。
func (b Bind) ToIDA(a oracle.Addr) uint32 {
	return a.Linear() - uint32(b.loadSeg())*16 + 0x10000
}

// Describe 印一行目前狀態：哪一支程式、CS:IP 的 IDA 位址、呼叫端。
func Describe(o *oracle.Oracle) string {
	b, ok := Current(o)
	if !ok {
		return fmt.Sprintf("程式 無  CS:IP %04X:%04X", o.IP().Seg, o.IP().Off)
	}
	return fmt.Sprintf("程式 %s PSP=%04X  CS:IP %04X:%04X（IDA %05X）",
		b.Name, b.PSP, o.IP().Seg, o.IP().Off, b.ToIDA(o.IP()))
}
