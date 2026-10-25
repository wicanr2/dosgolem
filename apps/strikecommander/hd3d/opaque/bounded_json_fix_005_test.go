package opaque

import (
	"bytes"
	"encoding/json"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/model"
	"io"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
)

// The optional StringWriter deliberately violates its short-write contract.
// The byte Writer is independent: a correct wrapper may intentionally use only it.
type fix005Writer struct {
	out             bytes.Buffer
	strings, writes int
	short           bool
}

func (w *fix005Writer) Write(b []byte) (int, error) {
	w.writes++
	if w.short {
		return 0, nil
	}
	return w.out.Write(b)
}
func (w *fix005Writer) WriteString(s string) (int, error) {
	w.strings++
	n := len(s) / 2
	_, err := w.out.WriteString(s[:n])
	return n, err
}

func TestDigestFix005StringWriterCannotHideShortWrite(t *testing.T) {
	s := model.Input{Original: &hd3d.Result{Diagnostics: hd3d.Diagnostics{Notes: []string{strings.Repeat("x", 65536)}}}}
	want, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	want = append(want, '\n')
	for _, sequence := range []bool{false, true} {
		w := &fix005Writer{}
		if sequence {
			seq := newBoundedJSONSequence(w)
			err = seq.add(&s)
			if err == nil {
				err = seq.finish()
			}
		} else {
			err = WriteBoundedJSON(w, s, 32<<20, 32<<20)
		}
		if err != nil || w.strings != 0 || w.writes == 0 || !bytes.Equal(w.out.Bytes(), want) {
			t.Fatalf("strict byte path sequence=%v err=%v optionalStringCalls=%d writes=%d bytes=%d want=%d", sequence, err, w.strings, w.writes, w.out.Len(), len(want))
		}
		broken := &fix005Writer{short: true}
		if sequence {
			seq := newBoundedJSONSequence(broken)
			err = seq.add(&s)
			if err == nil {
				err = seq.finish()
			}
		} else {
			err = WriteBoundedJSON(broken, s, 32<<20, 32<<20)
		}
		if err != io.ErrShortWrite || broken.strings != 0 || broken.writes != 1 {
			t.Fatalf("short Write was swallowed/retried: sequence=%v err=%v strings=%d writes=%d", sequence, err, broken.strings, broken.writes)
		}
	}
}

func TestDigestFix005CallerWorkspaceBeforeMapReflection(t *testing.T) {
	s := model.Input{Original: &hd3d.Result{CallElements: make(map[uint16]hd3d.FarPtr, 65536)}}
	for i := 0; i < 65536; i++ {
		s.Original.CallElements[uint16(i)] = hd3d.FarPtr{Off: uint16(i)}
	}
	// Match C's independent probe. Input allocation precedes the interval.
	if err := WriteBoundedJSON(io.Discard, model.Input{}, 32<<20, 32<<20); err != nil {
		t.Fatal(err)
	}
	old := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(old)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	err := WriteBoundedJSON(io.Discard, s, 32<<20, 256<<10)
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(s)
	delta := after.TotalAlloc - before.TotalAlloc
	if err == nil {
		t.Fatal("lower caller workspace was accepted")
	}
	if delta > 256<<10 {
		t.Fatalf("map reflection happened before caller admission: allocated=%d workspace=%d", delta, 256<<10)
	}
	// Allocation tripwire only. TotalAlloc is neither live peak nor RSS.
	t.Logf("allocationBeforeReject=%d callerWorkspace=%d", delta, 256<<10)
	// The same legal map remains admitted under the unchanged default ceiling.
	got, err := BoundedInputDigest(s, 32<<20)
	if err != nil {
		t.Fatal(err)
	}
	want, err := InputDigest(s)
	if err != nil || got != want {
		t.Fatal("default workspace/hash changed", err, got, want)
	}
}
