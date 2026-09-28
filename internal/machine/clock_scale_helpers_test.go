package machine

import (
	"bytes"
	"encoding/gob"
	"testing"
)

func decodeStateForTest(t *testing.T, b []byte) machineState {
	t.Helper()
	var s machineState
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func encodeStateForTest(t *testing.T, s machineState) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(s); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
