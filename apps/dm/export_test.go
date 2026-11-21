package dm

// 給 dm_test 的 Live 測試用（`docs/spec/198` §4 第 6 條）：反對照要能把位址讀錯，
// 核對要能讀怪物群記錄。

// 三個 ACTIVE_GROUP 全域的 DS 偏移（§3.8）。
const (
	DSActiveGroups     = dsActiveGroups
	DSActiveGroupMax   = dsActiveGroupMax
	DSActiveGroupCount = dsActiveGroupCount
)

// ActiveGroupsAt 照指定的三個位址讀表（`ActiveGroups` 用的是 §3.8 那三個）。
func (b *Bridge) ActiveGroupsAt(table, max, count uint16) ([]byte, error) {
	return b.activeGroupsAt(table, max, count)
}

// GroupRecord 讀型別 4（怪物群）第 i 筆記錄（16 位元組），走 §3.7 的 thing 資料指標。
func (b *Bridge) GroupRecord(i int) []byte {
	const groupType, groupBytes = 4, 16
	return b.O.Bytes(b.farPointer(dsThingData+4*groupType), (i+1)*groupBytes)[i*groupBytes:]
}
