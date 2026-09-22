package machine

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
)

// StateDigest 對一段 machine SaveState gob 計算與 map 寫入順序無關的摘要。
// 它只讀既有 machineState，不改變存態格式。
func StateDigest(r io.Reader) ([32]byte, error) {
	var s machineState
	if err := gob.NewDecoder(r).Decode(&s); err != nil {
		return [32]byte{}, fmt.Errorf("machine: 解碼狀態摘要輸入：%w", err)
	}
	if s.Magic != stateMagic || s.Version != stateVersion {
		return [32]byte{}, fmt.Errorf("machine: 狀態檔不認得（%q v%d）", s.Magic, s.Version)
	}
	b, err := json.Marshal(s)
	if err != nil {
		return [32]byte{}, fmt.Errorf("machine: 編碼狀態摘要：%w", err)
	}
	return sha256.Sum256(b), nil
}
