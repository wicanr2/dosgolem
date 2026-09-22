package dos

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

// StateDigest 對一段 DOS SaveState gob 計算與 map 走訪順序無關的摘要。
// Handles 與 EMSHandles 是由 map 產生的持久化 slice，故先依 handle 排序。
func StateDigest(r io.Reader) ([32]byte, error) {
	var s dosState
	if err := gob.NewDecoder(r).Decode(&s); err != nil {
		return [32]byte{}, fmt.Errorf("dos: 解碼狀態摘要輸入：%w", err)
	}
	if s.Magic != dosStateMagic || s.Version != dosStateVersion {
		return [32]byte{}, fmt.Errorf("dos: 狀態檔不認得（%q v%d）", s.Magic, s.Version)
	}
	sort.Slice(s.Handles, func(i, j int) bool { return s.Handles[i].H < s.Handles[j].H })
	sort.Slice(s.EMSHandles, func(i, j int) bool { return s.EMSHandles[i].H < s.EMSHandles[j].H })
	b, err := json.Marshal(s)
	if err != nil {
		return [32]byte{}, fmt.Errorf("dos: 編碼狀態摘要：%w", err)
	}
	return sha256.Sum256(b), nil
}
