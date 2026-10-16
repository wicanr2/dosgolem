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
	dec := gob.NewDecoder(r)
	if err := dec.Decode(&s); err != nil {
		return [32]byte{}, fmt.Errorf("machine: 解碼狀態摘要輸入：%w", err)
	}
	if s.Magic != stateMagic || (s.Version != stateVersion && s.Version != stateVersionSB8) {
		return [32]byte{}, fmt.Errorf("machine: 狀態檔不認得（%q v%d）", s.Magic, s.Version)
	}
	// 444 §2.9：版本 4 的第二個值（sbDMA8State）以 JSON 的 SHA-256 接在最後；版本 3 的摘要不變。
	var sb8Sum []byte
	if s.Version == stateVersionSB8 {
		var st sbDMA8State
		if err := dec.Decode(&st); err != nil {
			return [32]byte{}, fmt.Errorf("machine: 解碼狀態摘要輸入的 sbDMA8State：%w", err)
		}
		j, err := json.Marshal(st)
		if err != nil {
			return [32]byte{}, fmt.Errorf("machine: 編碼 sbDMA8State 摘要：%w", err)
		}
		sum := sha256.Sum256(j)
		sb8Sum = sum[:]
	}
	// 430：Ext 先算 SHA-256，不把整個 Ext 做 JSON base64。kb == 0 時 Ext 為空，
	// 摘要與加欄位之前完全相同。
	ext := s.Ext
	s.Ext = nil
	b, err := json.Marshal(s)
	if err != nil {
		return [32]byte{}, fmt.Errorf("machine: 編碼狀態摘要：%w", err)
	}
	if len(ext) == 0 && sb8Sum == nil {
		return sha256.Sum256(b), nil
	}
	if len(ext) != 0 {
		extSum := sha256.Sum256(ext)
		b = append(b, extSum[:]...)
	}
	return sha256.Sum256(append(b, sb8Sum...)), nil
}
