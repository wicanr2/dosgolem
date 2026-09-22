package dos

import (
	"bytes"
	"encoding/gob"
	"testing"
)

func digestDOSState(t *testing.T, s dosState) [32]byte {
	t.Helper()
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(&s); err != nil {
		t.Fatal(err)
	}
	d, err := StateDigest(&b)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestStateDigestIgnoresMapAndHandleOrder(t *testing.T) {
	left := dosState{Magic: dosStateMagic, Version: dosStateVersion, EMB: map[uint16][]byte{}, Handles: []handleState{{H: 7, Name: "b"}, {H: 2, Name: "a"}}, EMSHandles: []emsHandleState{{H: 9}, {H: 3}}}
	left.EMB[7], left.EMB[2] = []byte{7}, []byte{2}
	right := dosState{Magic: dosStateMagic, Version: dosStateVersion, EMB: map[uint16][]byte{}, Handles: []handleState{{H: 2, Name: "a"}, {H: 7, Name: "b"}}, EMSHandles: []emsHandleState{{H: 3}, {H: 9}}}
	right.EMB[2], right.EMB[7] = []byte{2}, []byte{7}
	if got, want := digestDOSState(t, left), digestDOSState(t, right); got != want {
		t.Fatalf("equivalent DOS state changed digest: %x != %x", got, want)
	}
}

func TestStateDigestDetectsDOSStateChange(t *testing.T) {
	left := dosState{Magic: dosStateMagic, Version: dosStateVersion, Drive: 2}
	right := left
	right.Drive = 3
	if got, want := digestDOSState(t, left), digestDOSState(t, right); got == want {
		t.Fatal("DOS change did not change digest")
	}
}
