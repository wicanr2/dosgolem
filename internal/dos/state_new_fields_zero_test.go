package dos

import (
	"bytes"
	"encoding/gob"
	"testing"
)

// 430 的「只加欄位」commit：SaveState 不寫入 XMSBlocks，解碼後必須為空。
func TestSaveStateLeavesXMSBlocksEmpty(t *testing.T) {
	_, d := newTest(t)
	var b bytes.Buffer
	if err := d.SaveState(&b); err != nil {
		t.Fatal(err)
	}
	var s dosState
	if err := gob.NewDecoder(&b).Decode(&s); err != nil {
		t.Fatal(err)
	}
	if s.XMSBlocks != nil {
		t.Errorf("XMSBlocks = %v，應為 nil", s.XMSBlocks)
	}
}
