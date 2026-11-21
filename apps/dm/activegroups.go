package dm

import "fmt"

// ACTIVE_GROUP 傾印（`docs/spec/198` §3.8）。同 `DungeonImage`，這一層**不解析**欄位，
// 照指標把整張表讀出來，解碼交給 remake 那一側。
const (
	dsActiveGroups          = 0x34AC // far 指標 → ACTIVE_GROUP[上限]（IDA dword_3837C）
	dsActiveGroupMax        = 0x34B0 // word：上限 G376（IDA word_38380）
	dsActiveGroupCount      = 0x0D98 // word：現有筆數 G377（IDA word_35C68）
	activeGroupByteCount    = 16
	activeGroupMaxPlausible = 255
)

// ActiveGroups 傾印整張 ACTIVE_GROUP 表（`上限 × 16` 位元組，含空位）。
//
// ⚠ 核對不過就回錯誤，不要拿錯的長度去讀別的記憶體：上限要在 1–255、
// 現有筆數不超過上限、`+0 ≥ 0` 的筆數要等於現有筆數。
func (b *Bridge) ActiveGroups() ([]byte, error) {
	return b.activeGroupsAt(dsActiveGroups, dsActiveGroupMax, dsActiveGroupCount)
}

func (b *Bridge) activeGroupsAt(tableOff, maxOff, countOff uint16) ([]byte, error) {
	max := int(b.O.Word(b.DS(maxOff)))
	count := int(b.O.Word(b.DS(countOff)))
	if max < 1 || max > activeGroupMaxPlausible {
		return nil, fmt.Errorf("ACTIVE_GROUP 上限 ＝ %d，不在 1–%d：位址或基底不對", max, activeGroupMaxPlausible)
	}
	if count > max {
		return nil, fmt.Errorf("ACTIVE_GROUP 現有 %d 筆，超過上限 %d：位址或基底不對", count, max)
	}
	table := b.O.Bytes(b.farPointer(tableOff), max*activeGroupByteCount)
	used := 0
	for i := 0; i < max; i++ {
		if int16(le16(table, i*activeGroupByteCount)) >= 0 {
			used++
		}
	}
	if used != count {
		return nil, fmt.Errorf("ACTIVE_GROUP 表裡有 %d 筆在用，現有筆數是 %d：指標不對", used, count)
	}
	return table, nil
}
