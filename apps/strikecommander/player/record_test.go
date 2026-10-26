package player

import (
	"bytes"
	"encoding/binary"
	"github.com/wicanr2/dosgolem/session"
	"os"
	"path/filepath"
	"testing"
)

func TestRecordingStreamAndInput(t *testing.T) {
	var b bytes.Buffer
	h := RecordingHeader{Format: "strikecommander-player-turns", Version: 1, StepsPerSecond: 11550000}
	w, e := NewRecording(&b, h)
	if e != nil {
		t.Fatal(e)
	}
	turn := RecordedTurn{N: 7, Input: session.CapturedUpdate{ScanEvents: []session.ScanEvent{{Scan: 59}}}, Appearance: Appearance{Scale: 3, Language: "en"}}
	if e = w.Append(turn); e != nil {
		t.Fatal(e)
	}
	count := 0
	_, e = ReplayRecording(&b, func(r RecordedTurn) error {
		count++
		if r.N != 7 || r.Input.ScanEvents[0].Scan != 59 {
			t.Fatal(r)
		}
		return nil
	})
	if e != nil || count != 1 {
		t.Fatal(e, count)
	}
	if e = w.Append(RecordedTurn{}); e == nil {
		t.Fatal("zero turn accepted")
	}
}
func TestWAVLiteral(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.wav")
	w, e := NewWAV(p)
	if e != nil {
		t.Fatal(e)
	}
	if e = w.Append([]float32{1, -1, 0.5, -0.5}); e != nil {
		t.Fatal(e)
	}
	if e = w.Close(); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(p)
	if e != nil || len(b) != 52 || string(b[:4]) != "RIFF" || string(b[8:16]) != "WAVEfmt " || binary.LittleEndian.Uint32(b[24:]) != 48000 || binary.LittleEndian.Uint32(b[40:]) != 8 {
		t.Fatal(b, e)
	}
	want := []int16{32767, -32767, 16383, -16383}
	for i, x := range want {
		if int16(binary.LittleEndian.Uint16(b[44+i*2:])) != x {
			t.Fatal(i, b)
		}
	}
	if e = w.Close(); e != nil {
		t.Fatal(e)
	}
}
