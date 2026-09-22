package machine

import (
	"bytes"
	"encoding/gob"
	"testing"
)

func digestMachineState(t *testing.T, s machineState) [32]byte {
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

func TestStateDigestIgnoresMapInsertionOrder(t *testing.T) {
	left := machineState{Magic: stateMagic, Version: stateVersion, Ports: map[uint16]uint8{}, PortsIn: map[uint16]uint64{}}
	left.Ports[3], left.Ports[1] = 9, 7
	left.PortsIn[3], left.PortsIn[1] = 99, 77
	right := machineState{Magic: stateMagic, Version: stateVersion, Ports: map[uint16]uint8{}, PortsIn: map[uint16]uint64{}}
	right.Ports[1], right.Ports[3] = 7, 9
	right.PortsIn[1], right.PortsIn[3] = 77, 99
	if got, want := digestMachineState(t, left), digestMachineState(t, right); got != want {
		t.Fatalf("map insertion order changed digest: %x != %x", got, want)
	}
}

func TestStateDigestDetectsCPUStateChange(t *testing.T) {
	left := machineState{Magic: stateMagic, Version: stateVersion, Steps: 7}
	right := left
	right.Steps++
	if got, want := digestMachineState(t, left), digestMachineState(t, right); got == want {
		t.Fatal("CPU step change did not change digest")
	}
}
