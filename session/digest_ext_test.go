package session

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// docs/spec/430-physical-memory-and-xms-unification §3：Digest 的 ExtSHA256。

func TestDigestExtSHA256EmptyByDefault(t *testing.T) {
	o := startSyntheticOwner(t, observerLoopCOM)
	d, err := o.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if d.ExtSHA256 != "" {
		t.Errorf("kb == 0 時 ExtSHA256 ＝ %q，應為空字串（不影響既有收據）", d.ExtSHA256)
	}
}

func TestDigestExtSHA256TracksExt(t *testing.T) {
	o := newTestOwner(t, nil)
	if err := o.machine.SetExtendedMemoryKB(4096); err != nil {
		t.Fatal(err)
	}
	layout := sessionLayout(1, 2, false)
	if err := o.mouse.ApplyLayout(layout); err != nil {
		t.Fatal(err)
	}
	o.layout, o.layoutSet = layout, true
	if err := o.machine.LoadCOM(observerLoopCOM); err != nil {
		t.Fatal(err)
	}
	if err := o.startLoadedMachine(); err != nil {
		t.Fatal(err)
	}
	d1, err := o.Digest()
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(o.machine.Ext)
	if d1.ExtSHA256 != hex.EncodeToString(sum[:]) {
		t.Errorf("ExtSHA256 ＝ %q", d1.ExtSHA256)
	}
	o.machine.WritePhys8(0x200000, 1)
	d2, _ := o.Digest()
	if d2.ExtSHA256 == d1.ExtSHA256 || d2.ExtSHA256 == "" {
		t.Error("Ext 改變後 ExtSHA256 必須改變")
	}
}
