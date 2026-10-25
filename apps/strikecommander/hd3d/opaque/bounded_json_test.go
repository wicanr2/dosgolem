package opaque

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/model"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type jsonCountWriter struct {
	calls int
	n     int
}

func (w *jsonCountWriter) Write(p []byte) (int, error) { w.calls++; w.n += len(p); return len(p), nil }

type jsonShortWriter struct{}

func (jsonShortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }
func TestBoundedJSONExactStandardBytesAndCanonicalCorners(t *testing.T) {
	values := []any{nil, string([]byte{0, 8, 9, 10, 12, 13, 31, 0xff}) + string(rune(0x2028)) + string(rune(0x2029)), struct {
		B []byte
		A [3]byte
		S string
		P *int `json:",omitempty"`
		M map[uint16]hd3d.FarPtr
	}{B: []byte{0, 1, 255}, A: [3]byte{0, 1, 255}, S: "<>&\"\\\b\f\n\r\t\x00\x1f\u2028\u2029\ufffd\xff", M: map[uint16]hd3d.FarPtr{2: {Off: 2}, 10: {Off: 10}, 65535: {Seg: 65535}}}, []byte(nil), []byte{}, []uint16(nil), []uint16{}, model.Input{}, model.Input{Objects: []realspace.ObjObs{}}, model.Input{Objects: []realspace.ObjObs{{Verdict: realspace.ObjVerdict{Reason: "ignored", Selected: &realspace.ModelEntry{LNTH: bytes.Repeat([]byte{7}, 100)}}}}}}
	for i, v := range values {
		var got bytes.Buffer
		if e := WriteBoundedJSON(&got, v, 32<<20, 32<<20); e != nil {
			t.Fatal(i, e)
		}
		want, e := json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
		want = append(want, '\n')
		if !bytes.Equal(got.Bytes(), want) {
			t.Fatalf("case%d wire mismatch\ngot %q\nwant%q", i, got.Bytes(), want)
		}
	}
	for _, s := range []model.Input{{}, {Objects: []realspace.ObjObs{}}, {Objects: []realspace.ObjObs{{Verdict: realspace.ObjVerdict{Reason: "ignored"}}}}} {
		old, e := InputDigest(s)
		if e != nil {
			t.Fatal(e)
		}
		got, e := BoundedInputDigest(s, 32<<20)
		if e != nil || got != old {
			t.Fatal("canonical difference", e, got, old)
		}
	}
}
func TestBoundedJSONPreflightBeforeAnyWriteAndWorkerLimits(t *testing.T) {
	w := &jsonCountWriter{}
	if e := WriteBoundedJSON(w, [][]byte{bytes.Repeat([]byte{7}, 16000), bytes.Repeat([]byte{8}, 16000)}, 30000, 32<<20); e == nil || w.calls != 0 {
		t.Fatal("output preflight allocated/wrote", e, w)
	}
	if e := WriteBoundedJSON(w, model.Input{}, 32<<20, boundedJSONFixedWorkspace-1); e == nil || w.calls != 0 {
		t.Fatal("workspace preflight ignored", e, w)
	}
	if e := WriteBoundedJSON(jsonShortWriter{}, model.Input{}, 32<<20, 32<<20); e != io.ErrShortWrite {
		t.Fatal("short write swallowed", e)
	}
	m := map[uint16]hd3d.FarPtr{2: {}, 10: {}}
	p, e := planBoundedJSON(m, false, 32<<20, 32<<20)
	if e != nil || p.WorkspaceBytes != boundedJSONFixedWorkspace+2*256 {
		t.Fatal(p, e)
	}
	if e = WriteBoundedJSON(w, m, 32<<20, p.WorkspaceBytes-1); e == nil || w.calls != 0 {
		t.Fatal("map workspace not preflighted", e, w)
	}
}
func TestBoundedJSONSharedFiveThousandCallProbe(t *testing.T) {
	table := make([]byte, 3072)
	s := model.Input{Raster: hd3d.Group{Calls: make([]hd3d.Call, 5000)}}
	for i := range s.Raster.Calls {
		s.Raster.Calls[i].Aux.Table = table
	}
	want, e := InputDigest(s)
	if e != nil {
		t.Fatal(e)
	}
	got, e := BoundedInputDigest(s, 32<<20)
	if e != nil || got != want {
		t.Fatal(got, want, e)
	}
	p, e := planBoundedJSON(s, true, 32<<20, 32<<20)
	if e != nil || p.EncodedBytes != 22395125 || p.WorkspaceBytes != boundedJSONFixedWorkspace {
		t.Fatal("probe resource plan", p, e)
	}
	if 2*p.EncodedBytes <= 32<<20 {
		t.Fatal("fixture does not exceed old two-buffer worker")
	}
	if _, e = BoundedInputDigest(s, p.EncodedBytes-1); e == nil {
		t.Fatal("caller output/intermediate limit raised")
	}
}
func TestBoundedJSONOriginalEightySceneDigests(t *testing.T) {
	root := os.Getenv("SC_DIGEST_CORPUS")
	if root == "" {
		t.Skip("explicit original read-only corpus required")
	}
	total := 0
	var maxOutput, maxFull, maxWork uint64
	for _, state := range []string{"s760", "s800f2", "n1", "n2"} {
		for i := 1; i <= 20; i++ {
			p := filepath.Join(root, state+"-004", fmt.Sprintf("B-source-%03d.json", i))
			raw, e := os.ReadFile(p)
			if e != nil {
				t.Fatal(e)
			}
			var env struct {
				Legacy   model.Input
				Declared struct {
					LegacyInputDigest struct {
						Hex string `json:"hex"`
					}
				}
			}
			if e = json.Unmarshal(raw, &env); e != nil {
				t.Fatal(e)
			}
			literal, e := hex.DecodeString(env.Declared.LegacyInputDigest.Hex)
			if e != nil || len(literal) != 32 {
				t.Fatal("independent source004 declaration lacks digest", e)
			}
			old, e := InputDigest(env.Legacy)
			if e != nil {
				t.Fatal(e)
			}
			got, e := BoundedInputDigest(env.Legacy, 32<<20)
			if e == nil && !bytes.Equal(got[:], literal) {
				t.Fatal(p, "recorded source004 canonical digest differs")
			}
			if e != nil || got != old {
				t.Fatal(p, e, got, old)
			}
			var wire bytes.Buffer
			if e = WriteBoundedJSON(&wire, env.Legacy, 32<<20, 32<<20); e != nil {
				t.Fatal(p, e)
			}
			expected, e := json.Marshal(env.Legacy)
			if e != nil {
				t.Fatal(e)
			}
			expected = append(expected, '\n')
			if !bytes.Equal(expected, wire.Bytes()) {
				t.Fatal(p, "receipt wire mismatch")
			}
			q, e := planBoundedJSON(env.Legacy, true, 32<<20, 32<<20)
			if e != nil {
				t.Fatal(e)
			}
			maxOutput = max(maxOutput, q.EncodedBytes)
			maxFull = max(maxFull, uint64(len(expected)-1))
			maxWork = max(maxWork, q.WorkspaceBytes)
			total++
			t.Logf("%s group%d digest=%x JSON=%d workspace=%d legacywireSHA=%x", state, i, got, q.EncodedBytes, q.WorkspaceBytes, sha256.Sum256(wire.Bytes()))
		}
	}
	if total != 80 {
		t.Fatal(total)
	}
	t.Logf("80 actual scenes exact; max canonical=%d max full=%d max workspace=%d", maxOutput, maxFull, maxWork)
}

type badJSON struct{}

func (badJSON) MarshalJSON() ([]byte, error) { return []byte(`123`), nil }
func TestBoundedJSONRestrictedSchemaAndSequence(t *testing.T) {
	for _, v := range []any{float64(1), badJSON{}, struct{ int }{}, map[int][100]byte{1: {}}, func() any { type node struct{ Next *node }; n := &node{}; n.Next = n; return n }()} {
		w := &jsonCountWriter{}
		if err := WriteBoundedJSON(w, v, 32<<20, 32<<20); err == nil || w.calls != 0 {
			t.Fatalf("unsupported/cyclic schema wrote output: %T %v", v, err)
		}
	}
	for _, n := range []int{1, 2, 3, 3071, 3072, 3073, 6144, 6145} {
		b := bytes.Repeat([]byte{0xff}, n)
		var out bytes.Buffer
		if err := WriteBoundedJSON(&out, b, 32<<20, 32<<20); err != nil {
			t.Fatal(err)
		}
		want, _ := json.Marshal(b)
		if !bytes.Equal(out.Bytes(), append(want, '\n')) {
			t.Fatalf("base64 boundary %d", n)
		}
	}
	calls := []hd3d.Call{{Seq: 2}, {Seq: 10}}
	calls[0].Aux.Table = bytes.Repeat([]byte{9}, 3073)
	var got, want bytes.Buffer
	seq := newBoundedJSONSequence(&got)
	enc := json.NewEncoder(&want)
	for _, v := range []any{SceneIdentity{}, calls[0], calls[1]} {
		if err := seq.add(v); err != nil {
			t.Fatal(err)
		}
		if err := enc.Encode(v); err != nil {
			t.Fatal(err)
		}
	}
	if err := seq.finish(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), want.Bytes()) {
		t.Fatal("material sequence changed canonical bytes")
	}
	w := &jsonCountWriter{}
	s := model.Input{Raster: hd3d.Group{Calls: make([]hd3d.Call, DefaultLimits().Calls+1)}}
	if err := WriteBoundedInputJSON(w, s); err == nil || w.calls != 0 {
		t.Fatal("call ceiling not checked before receipt output", err)
	}
}

func TestBoundedJSONRejectBeforeOutputBackingAllocation(t *testing.T) {
	// Input exists before the interval. TotalAlloc is an allocation-tripwire only;
	// it is not reported as live memory, residency, or a worker-peak measurement.
	input := bytes.Repeat([]byte{0x7f}, 8<<20)
	w := &jsonCountWriter{}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	err := WriteBoundedJSON(w, input, 1024, 32<<20)
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(input)
	if err == nil || w.calls != 0 {
		t.Fatal("oversized encoded value not rejected", err, w)
	}
	if after.TotalAlloc-before.TotalAlloc > boundedJSONFixedWorkspace {
		t.Fatal("preflight allocated output-sized backing", after.TotalAlloc-before.TotalAlloc)
	}
}

func TestBoundedJSONSequenceReusesWorkspace(t *testing.T) {
	calls := make([]hd3d.Call, 5000)
	for i := range calls {
		calls[i].Seq = uint16(i)
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	seq := newBoundedJSONSequence(io.Discard)
	for i := range calls {
		if err := seq.add(&calls[i]); err != nil {
			t.Fatal(err)
		}
	}
	if err := seq.finish(); err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(calls)
	// Allocation regression detector, never a statement of peak resident memory.
	if after.TotalAlloc-before.TotalAlloc > boundedJSONFixedWorkspace {
		t.Fatal("sequence allocated per-call workspace", after.TotalAlloc-before.TotalAlloc)
	}
}

func TestBoundedJSONFullReceiptDoesNotInventOutputCeiling(t *testing.T) {
	s := model.Input{Objects: []realspace.ObjObs{{Verdict: realspace.ObjVerdict{Selected: &realspace.ModelEntry{LNTH: make([]byte, 25<<20)}}}}}
	p, err := planBoundedJSON(s, false, ^uint64(0), 32<<20)
	if err != nil || p.EncodedBytes <= 32<<20 {
		t.Fatal("fixture needs full receipt over32MiB", p, err)
	}
	if _, err = BoundedInputDigest(s, 32<<20); err != nil {
		t.Fatal("serialized Selected must not enter canonical digest", err)
	}
	w := &jsonCountWriter{}
	if err = WriteBoundedInputJSON(w, s); err != nil || uint64(w.n) != p.EncodedBytes+1 {
		t.Fatal("streamed full receipt got an invented output ceiling", err, w.n, p)
	}
}

func TestBoundedJSONTypedSmallKeysMatchLegacyBytes(t *testing.T) {
	id := SceneIdentity{State: [32]byte{1, 255}, AppGeneration: ^uint64(0), SignatureGeneration: 8}
	r := &Renderer{typedFreeze: true}
	if got := r.sourceEpochKey(id); got != epochKey(id) {
		t.Fatal("typed epoch key changed", got, epochKey(id))
	}
	v := struct {
		ID         SceneIdentity
		Cap        *CaptureIdentity
		ReadFailed bool
		Rule       string
	}{id, &CaptureIdentity{}, true, RuleVersion}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	got, err := boundedJSONDigest(v, 32<<20)
	if err != nil || got != sha256.Sum256(b) {
		t.Fatal("typed identity key changed", got, err)
	}
}

func TestBoundedJSONTypedLowerCallerLimit(t *testing.T) {
	r := &Renderer{typedFreeze: true, limits: DefaultLimits()}
	r.limits.IntermediateBytes = 1024
	if err := r.sourceStatusLocked(SceneIdentity{}); err == nil {
		t.Fatal("lowered typed workspace was ignored")
	}
	if err := r.blockSourceLocked(SceneIdentity{}); err == nil {
		t.Fatal("H4 ledger bypassed lower workspace admission")
	}
	if _, err := boundedJSONDigest(SceneIdentity{}, 1024); err == nil {
		t.Fatal("small-key helper raised lower caller workspace")
	}
}
