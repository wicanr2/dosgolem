package machine

import (
	"bytes"
	"encoding/gob"
	"testing"
)

// 430 的「只加欄位」commit：SaveState 不寫入新欄位，解碼後 Ext、EMSAbsent、
// KbdAwaitOutport 必須是零值。行為 commit 之後 kb == 0 的存檔仍須如此。
func TestSaveStateLeavesNewFieldsZero(t *testing.T) {
	m := New()
	if err := m.LoadCOM([]byte{0xB4, 0x4C, 0xCD, 0x21}); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := m.SaveState(&b); err != nil {
		t.Fatal(err)
	}
	var s machineState
	if err := gob.NewDecoder(&b).Decode(&s); err != nil {
		t.Fatal(err)
	}
	if s.Ext != nil {
		t.Errorf("Ext = %d bytes，應為 nil", len(s.Ext))
	}
	if s.EMSAbsent {
		t.Error("EMSAbsent 應為 false")
	}
	if s.KbdAwaitOutport {
		t.Error("KbdAwaitOutport 應為 false")
	}
}
