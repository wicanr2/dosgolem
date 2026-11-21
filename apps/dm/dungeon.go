package dm

import (
	"fmt"

	"github.com/wicanr2/dosgolem/oracle"
)

// 地城映像傾印（`docs/spec/198` §3.7）。這一層**不解析**地城結構，
// 只照指標把會變的那幾段 bytes 讀出來，解碼交給 remake 那一側。
const (
	dsDungeonHeader     = 0x343C // far 指標 → DUNGEON_HEADER（IDA dword_3830C）
	dsDungeonMaps       = 0x3480 // far 指標 → MAP[MapCount]，一筆 16 位元組（IDA dword_38350）
	dsColumnsCumulative = 0x33D2 // far 指標 → 欄累計表（IDA dword_382A2）
	dsThingData         = 0x33F0 // 16 個 far 指標（IDA dword_382C0）
	dsSquareFirstThings = 0x3474 // far 指標（IDA dword_38344）
	dsRawMapData        = 0x347C // far 指標，偏移與段兩個 word（IDA word_3834C／word_3834E）

	dungeonHeaderSize = 44
	thingTypeCount    = 16
)

// thingDataByteCount 是每個型別一筆資料的位元組數（GRAPHICS.DAT 第 559 項，
// ReDMCSB `G235_auc_Graphic559_ThingDataByteCount`；dungeon_master 專案
// `internal/assets` 的同名表）。
var thingDataByteCount = [thingTypeCount]int{4, 6, 4, 8, 16, 4, 4, 4, 4, 8, 4, 0, 0, 0, 8, 4}

// farPointer 讀一個存成「偏移、段」兩個 word 的 far 指標。
func (b *Bridge) farPointer(off uint16) oracle.Addr {
	return oracle.Far(b.O.Word(b.DS(off+2)), b.O.Word(b.DS(off)))
}

// DungeonImage 傾印欄累計表、`SquareFirstThings`、型別 0–15 的 thing 資料、`RawMapData`，
// 照 `DUNGEON.DAT` 的順序接起來（`docs/spec/198` §3.7）。長度由記憶體裡的標頭與各層描述算。
//
// ⚠ 標頭不合理就回錯誤，不要拿錯的長度去讀別的記憶體：`MapCount` 要在 1–64，
// 型別 11–13 的數量要是 0（DM 沒有那三種）。
//
// ⚠ 欄累計表**會變**（東西走上空格時原版插一筆、後面每一欄加一），一定要跟
// `SquareFirstThings` 同一刻讀。拿存檔那一份去配，每一格的串列會錯位，
// 解出來的怪物一步跳過半張地圖，看起來仍然像合法的位置。
func (b *Bridge) DungeonImage() ([]byte, error) {
	header := b.O.Bytes(b.farPointer(dsDungeonHeader), dungeonHeaderSize)
	word := func(off int) int { return int(le16(header, off)) }
	maps := int(header[4])
	if maps < 1 || maps > 64 {
		return nil, fmt.Errorf("地城標頭的 MapCount ＝ %d，不在 1–64：指標或基底不對", maps)
	}
	for t := 11; t <= 13; t++ {
		if n := word(0x0C + 2*t); n != 0 {
			return nil, fmt.Errorf("地城標頭的型別 %d 有 %d 筆，DM 沒有這一型：指標或基底不對", t, n)
		}
	}
	// 欄數 ＝ 各層 `寬 + 1` 的和，寬在描述 +8 那個 word 的第 6–10 位元
	// （dungeon_master 專案 `internal/assets` 的 `Map.Columns`）。
	desc := b.O.Bytes(b.farPointer(dsDungeonMaps), 16*maps)
	columns := 0
	for i := 0; i < maps; i++ {
		columns += int(le16(desc, 16*i+8)>>6&0x1F) + 1
	}
	out := b.O.Bytes(b.farPointer(dsColumnsCumulative), columns*2)
	out = append(out, b.O.Bytes(b.farPointer(dsSquareFirstThings), word(0x0A)*2)...)
	for t := 0; t < thingTypeCount; t++ {
		n := word(0x0C+2*t) * thingDataByteCount[t]
		if n == 0 {
			continue
		}
		out = append(out, b.O.Bytes(b.farPointer(dsThingData+uint16(4*t)), n)...)
	}
	return append(out, b.O.Bytes(b.farPointer(dsRawMapData), word(0x02))...), nil
}
