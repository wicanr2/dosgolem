package machine

import (
	"encoding/binary"
	"testing"
)

// TestMZLastPageIsNineBits 釘住 `e_cblp` 只取低九位。
//
// 那一格是「最後一頁用了幾個位元組」，一頁 512，所以合法值是 0..511。
// **高位的位元是打包工具留下來的垃圾**；不遮的話映像長度會算成天文
// 數字，而載入器只會說「檔案太短」——指向完全錯的方向。
//
// 量到的案例：智冠《三國演義》加強版的 `DATA0.GRP`（87,696 bytes）
// 寫著 `e_cblp = 0xAA90`；遮成九位得到 144，`(172−1)×512 + 144`
// 正好是檔案長度。
func TestMZLastPageIsNineBits(t *testing.T) {
	hdr := make([]byte, 32)
	copy(hdr, "MZ")
	put := func(off int, v uint16) { binary.LittleEndian.PutUint16(hdr[off:], v) }
	put(2, 0xAA90) // e_cblp，越界
	put(4, 172)    // e_cp
	put(8, 192)    // e_cparhdr ＝ 3072 bytes

	h, err := parseMZ(hdr)
	if err != nil {
		t.Fatal(err)
	}
	if h.LastPage != 144 {
		t.Errorf("e_cblp 讀成 %d，遮成九位應該是 144", h.LastPage)
	}
	total := (int(h.Pages)-1)*512 + int(h.LastPage)
	if total != 87696 {
		t.Errorf("映像長度算成 %d，應該是 87696（＝ 那個檔案的長度）", total)
	}

	// 合法值不受影響——15 個真檔裡只有一個越界，其餘遮不遮都一樣。
	for _, v := range []uint16{0, 1, 48, 144, 358, 462, 482, 504, 511} {
		put(2, v)
		h, err := parseMZ(hdr)
		if err != nil {
			t.Fatal(err)
		}
		if h.LastPage != v {
			t.Errorf("合法的 e_cblp %d 被改成 %d", v, h.LastPage)
		}
	}
}
