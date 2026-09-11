package dm

import (
	"fmt"
	"strings"
)

// DS 偏移（dungeon_master 專案 `docs/re/73`，推論等級**已確認**）。
//
// 換算：IDA 的資料標籤是線性位址，DS 基底的 IDA 位址是 `0x34ED0`，
// 所以 `ds:偏移 ＝ IDA 位址 − 0x34ED0`。
const (
	dsChampions = 0x3592 // G407_s_Party.Champions[]
	dsGameTime  = 0x3C84 // G313_ul_GameTime，4 位元組
	dsMapIndex  = 0x3C8A // G309_i_PartyMapIndex
	dsPartyDir  = 0x3C92 // G308_i_PartyDirection
	dsPartyX    = 0x3C94 // G306_i_PartyMapX
	dsPartyY    = 0x3CE0 // G307_i_PartyMapY ⚠ 與 X 不相鄰
)

// CHAMPION 的大小與欄位偏移（同一份筆記）。
//
// ⚠ **`Statistics[7][3]` 在 x86 沒有補齊位元組**，所以 `Skills` 從**奇數**
// 位址 `+0x5B` 起算。照 68000 的對齊習慣去推，之後每一個欄位都會偏一。
const (
	ChampionSize = 0x13F // ＝ 319，與 `imul 13Fh` 吻合
	MaxChampions = 4

	offName             = 0x00 // char[8]
	offTitle            = 0x08 // char[20]
	offPoisonEventCount = 0x2A // 1 位元組
	offCurrentHealth    = 0x34
	offMaximumHealth    = 0x36
	offFood             = 0x42
	offWater            = 0x44
	offSlots            = 0xD3 // THING[30]，一格 2 位元組
	offLoad             = 0x10F

	nameLen  = 8
	titleLen = 20
	// SlotCount 是物品欄的格數。⚠ **編號不是連續的**：`13` 之後跳到
	// `22`–`29` 才是背包第一排的其餘格（`DEFS.H:693-722`）。
	// 兩側要照同一個順序輸出，不要照畫面排列。
	SlotCount = 30
)

// FoodWaterMaximum 是食物與水的滿值。
//
// 來源是 ReDMCSB（remake 的 `internal/game/champion.go`），**在 DOS 版驗證過**
// ——推論等級**已確認**：把食物壓到 300、跑 60 刻確認會掉到 298，
// 再開巡檢跑 60 刻，值回到 2048 而且沒有被別處鉗掉
// （`TestLiveSweepHoldsFoodAndWater`）。
//
// 已在 DOS 版直接讀到的是**下限**那兩個：`0FE00h` ＝ −512（開始挨餓）
// 與 `0FC00h` ＝ −1024（鉗位），與 remake 的 `FoodWaterStarving` 對得上。
const FoodWaterMaximum = 2048

// Champion 是一位勇士的巡檢相關狀態。
type Champion struct {
	Index            int
	Name, Title      string
	CurrentHealth    int
	MaxHealth        int
	Food, Water      int
	PoisonEventCount int
	Load             int
	Slots            [SlotCount]uint16
}

// Party 是一次取樣。
type Party struct {
	MapIndex  int
	GameTime  int
	X, Y      int
	Facing    int // 0＝北 1＝東 2＝南 3＝西
	Champions []Champion
}

// ReadParty 讀出隊伍與勇士的狀態（`docs/spec/198` §3.1）。
//
// 只回**有勇士的那幾格**（`MaximumHealth > 0`）。判準用最大生命力而不是
// 當前生命力：死掉的勇士當前值是 0，但那一格仍然有人，巡檢要把他救回來。
func (b *Bridge) ReadParty() (Party, error) {
	p := Party{
		MapIndex: int(b.O.Word(b.DS(dsMapIndex))),
		GameTime: int(b.gameTime()),
		X:        int(b.O.Word(b.DS(dsPartyX))),
		Y:        int(b.O.Word(b.DS(dsPartyY))),
		Facing:   int(b.O.Word(b.DS(dsPartyDir))),
	}
	if p.Facing < 0 || p.Facing > 3 {
		return p, fmt.Errorf("面向 ＝ %d，不在 0–3——位址或基底不對", p.Facing)
	}
	for i := 0; i < MaxChampions; i++ {
		base := uint16(dsChampions + i*ChampionSize)
		c, ok := decodeChampion(i, b.O.Bytes(b.DS(base), ChampionSize))
		if !ok {
			continue // 這一格沒有勇士
		}
		p.Champions = append(p.Champions, c)
	}
	return p, nil
}

// gameTime 讀 32 位元的遊戲刻（低位在 `ds:0x3C84`，高位在 `+2`）。
func (b *Bridge) gameTime() uint32 {
	lo := uint32(b.O.Word(b.DS(dsGameTime)))
	hi := uint32(b.O.Word(b.DS(dsGameTime + 2)))
	return hi<<16 | lo
}

// GameTime 是目前的遊戲刻。
func (b *Bridge) GameTime() uint32 { return b.gameTime() }

func indexByte(b []byte, c byte) int {
	for i, v := range b {
		if v == c {
			return i
		}
	}
	return -1
}

// decodeChampion 從一段 `ChampionSize` 位元組解出一位勇士。
//
// 回 `false` 表示**這一格沒有勇士**——判準是 `MaximumHealth <= 0`，
// 不是當前生命力：死掉的勇士當前值是 0，但那一格仍然有人，巡檢要把他救回來。
//
// 抽成吃 `[]byte` 的純函式，是為了讓偏移表有一支**不必跑原版**的測試守著。
func decodeChampion(index int, raw []byte) (Champion, bool) {
	if len(raw) < ChampionSize {
		return Champion{}, false
	}
	maxHP := int(int16(le16(raw, offMaximumHealth)))
	if maxHP <= 0 {
		return Champion{}, false
	}
	c := Champion{
		Index:            index,
		Name:             fixedStr(raw[offName : offName+nameLen]),
		Title:            fixedStr(raw[offTitle : offTitle+titleLen]),
		CurrentHealth:    int(int16(le16(raw, offCurrentHealth))),
		MaxHealth:        maxHP,
		Food:             int(int16(le16(raw, offFood))),
		Water:            int(int16(le16(raw, offWater))),
		PoisonEventCount: int(raw[offPoisonEventCount]),
		Load:             int(int16(le16(raw, offLoad))),
	}
	for n := 0; n < SlotCount; n++ {
		c.Slots[n] = le16(raw, offSlots+n*2)
	}
	return c, true
}

func le16(b []byte, off int) uint16 { return uint16(b[off]) | uint16(b[off+1])<<8 }

// fixedStr 讀一段定長字串欄位：砍掉 NUL 之後的東西，再去掉尾巴的空白。
func fixedStr(raw []byte) string {
	if i := indexByte(raw, 0); i >= 0 {
		raw = raw[:i]
	}
	return strings.TrimRight(string(raw), " ")
}
