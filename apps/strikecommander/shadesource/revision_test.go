package shadesource

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"testing"
	"time"
)

func TestAcceptVerifiedH4RevokesButBadDeclarationDoesNot(t *testing.T) {
	o, _ := NewOwner(0)
	defer o.Close()
	s := emptyInput()
	p := InitialPacket(emptyID())
	p.Proofs = []Proof{fullTestProof(p.Identity)}
	p, d, e := Finalize(s, p)
	if e != nil {
		t.Fatal(e)
	}
	held, e := o.Accept(s, p, d)
	if e != nil {
		t.Fatal(e)
	}
	defer held.Release()
	q := clonePacket(p)
	q.Proofs[0].Reads[0].Capture.Spans[0].Data[0] ^= 1
	q, qd, e := Finalize(s, q)
	if e != nil || q.Status != H4 {
		t.Fatal("H4 fixture")
	}
	if _, e = o.Accept(s, q, d); e == nil {
		t.Fatal("wrong external declaration accepted")
	}
	if e = held.Eligible(); e != nil {
		t.Fatal("invalid declaration poisoned good epoch")
	}
	if _, e = o.Accept(s, q, qd); e == nil {
		t.Fatal("verified H4 accepted")
	}
	if e = held.Eligible(); e == nil {
		t.Fatal("verified H4 did not revoke prior same-epoch lease")
	}
}
func TestJSONAllTwelveRecordsRequireExactFields(t *testing.T) {
	p, e := UnmarshalPacket([]byte(nonemptyOracleJSON))
	if e != nil {
		t.Fatal(e)
	}
	b, e := MarshalPacket(p)
	if e != nil {
		t.Fatal(e)
	}
	// Independent lists copied from the normative schema, not reflect-derived from
	// production struct definitions. Every field is tested missing and case-aliased.
	fields := map[string][]string{
		"Packet":   {"SchemaVersion", "RuleVersion", "LegacyInputDigest", "Identity", "Proofs", "Builds", "Calls", "Counters", "Status", "Reason"},
		"Identity": {"EXESHA", "SignatureSetSHA", "Base", "ResetGeneration", "ModelsGeneration", "ShadeEpoch", "GroupSerial", "CUKey"},
		"Far":      {"Seg", "Off"}, "FaceRef": {"Object", "Face"}, "Context": {"Steps", "Deliveries", "CS", "IP", "DS", "SS", "SP"},
		"Proof":     {"Steps", "Deliveries", "Base", "ModelsGeneration", "ShadeEpoch", "Reads", "Status", "Reason"},
		"ProofRead": {"SignatureIndex", "Capture"}, "Read": {"Start", "Requested", "State", "Spans"}, "Span": {"Off", "Requested", "OK", "Data"},
		"Build":    {"FaceRef", "Context", "ProofIndex", "Method24", "Raw5", "Status", "Reason"},
		"R4":       {"CallIndex", "FaceRef", "ElementIndex", "BuildIndex", "Context", "ProofIndex", "RowOrigin", "Row", "Table", "Status", "Reason"},
		"Counters": {"ReadCalls", "ReadBytes", "Candidates", "Confirmed", "Voided", "ShortReads", "ResourceRejected", "BindingRejected", "RowMismatch", "TableMismatch"}}
	marker := map[string]string{"SchemaVersion": "Packet", "EXESHA": "Identity", "Seg": "Far", "Object": "FaceRef", "CS": "Context", "Reads": "Proof", "SignatureIndex": "ProofRead", "Spans": "Read", "OK": "Span", "Method24": "Build", "CallIndex": "R4", "ReadCalls": "Counters"}
	type part struct {
		k     string
		i     int
		array bool
	}
	paths := map[string][]part{}
	var walk func(any, []part)
	walk = func(v any, path []part) {
		switch x := v.(type) {
		case map[string]any:
			for key, kind := range marker {
				if _, ok := x[key]; ok {
					if _, exists := paths[kind]; !exists {
						paths[kind] = append([]part{}, path...)
					}
				}
			}
			for k, y := range x {
				walk(y, append(append([]part{}, path...), part{k: k}))
			}
		case []any:
			for i, y := range x {
				walk(y, append(append([]part{}, path...), part{i: i, array: true}))
			}
		}
	}
	var root any
	if e = json.Unmarshal(b, &root); e != nil {
		t.Fatal(e)
	}
	walk(root, nil)
	if len(paths) != 12 {
		t.Fatal("independent vector lacks record types", len(paths))
	}
	for kind, names := range fields {
		for _, name := range names {
			for alias := 0; alias < 2; alias++ {
				var v any
				_ = json.Unmarshal(b, &v)
				cur := v
				for _, p := range paths[kind] {
					if p.array {
						cur = cur.([]any)[p.i]
					} else {
						cur = cur.(map[string]any)[p.k]
					}
				}
				m := cur.(map[string]any)
				value, ok := m[name]
				if !ok {
					t.Fatal("fixture missing", kind, name)
				}
				delete(m, name)
				if alias == 1 {
					key := string(bytes.ToLower([]byte(name[:1]))) + name[1:]
					m[key] = value
				}
				bad, _ := json.Marshal(v)
				if _, e = UnmarshalPacket(bad); e == nil {
					t.Fatalf("missing or case-aliased field accepted: %s.%s alias=%d", kind, name, alias)
				}
			}
		}
	}
	for _, name := range []string{"LegacyInputDigest"} {
		var v map[string]any
		_ = json.Unmarshal(b, &v)
		v[name] = v[name].([]any)[:31]
		bad, _ := json.Marshal(v)
		if _, e = UnmarshalPacket(bad); e == nil {
			t.Fatal("short fixed byte array accepted")
		}
	}
}

type errorWriter struct {
	err   error
	short bool
}

func (w errorWriter) Write(p []byte) (int, error) {
	if w.short {
		return len(p) - 1, nil
	}
	return 0, w.err
}

type blockedWriter struct{ entered, release chan struct{} }

func (w blockedWriter) Write(p []byte) (int, error) {
	close(w.entered)
	<-w.release
	return len(p), nil
}
func TestControlledStreamDirectAcceptAndWriterFailures(t *testing.T) {
	o, _ := NewOwner(0)
	defer o.Close()
	s := emptyInput()
	g, _ := o.NewGraph(emptyID())
	_, e := g.AddProof(fullTestProof(emptyID()))
	if e != nil {
		t.Fatal(e)
	}
	g.Seal()
	raw, _ := g.Acquire()
	defer raw.Release()
	before := o.OwnedBytes()
	want := errors.New("writer failure")
	if _, e = raw.Export(s, errorWriter{err: want}); !errors.Is(e, want) {
		t.Fatal("Writer error swallowed", e)
	}
	if _, e = raw.Export(s, errorWriter{short: true}); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal("short write swallowed", e)
	}
	if o.OwnedBytes() != before {
		t.Fatal("failed stream leaked source capacity")
	}
	var buf bytes.Buffer
	d, e := raw.Export(s, &buf)
	if e != nil {
		t.Fatal(e)
	}
	p, e := UnmarshalPacket(buf.Bytes())
	if e != nil || p.Status != Ready {
		t.Fatal(e)
	}
	h, _ := SourceDigest(p)
	if h != d.DeclaredSourceDigest {
		t.Fatal("stream altered canonical identity")
	}
	l, e := raw.Accept(s)
	if e != nil {
		t.Fatal("direct bounded raw accept", e)
	}
	defer l.Release()
	if _, e = raw.Accept(s); e == nil {
		t.Fatal("third external slot via direct accept")
	}
	if e = l.WriteJSON(errorWriter{short: true}); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal("accepted short write swallowed")
	}
	if l.Declaration() != d {
		t.Fatal("direct/stream declarations differ")
	}
}
func TestLeaseRuntimeSurfaceHasNoRawReturns(t *testing.T) {
	for _, typ := range []reflect.Type{reflect.TypeFor[*RawLease](), reflect.TypeFor[*Lease]()} {
		for i := 0; i < typ.NumMethod(); i++ {
			m := typ.Method(i)
			for j := 0; j < m.Type.NumOut(); j++ {
				r := m.Type.Out(j)
				if r == reflect.TypeFor[Packet]() || r == reflect.TypeFor[*Packet]() || r == reflect.TypeFor[[]byte]() {
					t.Fatalf("runtime lease exposes untracked source value: %s", m.Name)
				}
			}
		}
	}
}
func TestStreamSerializesReleaseAndClose(t *testing.T) {
	o, _ := NewOwner(0)
	g, _ := o.NewGraph(emptyID())
	g.Seal()
	raw, _ := g.Acquire()
	w := blockedWriter{make(chan struct{}), make(chan struct{})}
	exported := make(chan error, 1)
	go func() { _, e := raw.Export(emptyInput(), w); exported <- e }()
	<-w.entered
	released, closed := make(chan struct{}), make(chan struct{})
	go func() { raw.Release(); close(released) }()
	go func() { o.Close(); close(closed) }()
	select {
	case <-released:
		t.Fatal("Release escaped live writer")
	case <-closed:
		t.Fatal("Close escaped live writer")
	default:
	}
	if o.OwnedBytes() < fixedReservation {
		t.Fatal("writer workspace prematurely released")
	}
	close(w.release)
	if e := <-exported; e != nil {
		t.Fatal(e)
	}
	for _, c := range []chan struct{}{released, closed} {
		select {
		case <-c:
		case <-time.After(3 * time.Second):
			t.Fatal("serialized release/close deadlock")
		}
	}
	if o.OwnedBytes() != 0 {
		t.Fatal("writer/release leaked")
	}
}
func TestClonePlanCountsOutputCapacities(t *testing.T) {
	s, p := materialFixture(0x338d)
	_ = s
	data := make([]byte, 4, 100000)
	p.Builds = []Build{{Method24: &Read{Spans: []Span{{Data: data}}}}}
	planned, e := clonePlan(p)
	if e != nil {
		t.Fatal(e)
	}
	q := clonePacket(p)
	actual := footprint(q) + 256
	if actual > planned {
		t.Fatalf("unreserved clone capacity actual=%d planned=%d", actual, planned)
	}
	if cap(q.Builds[0].Method24.Spans[0].Data) != 4 || cap(q.Proofs) != len(q.Proofs) {
		t.Fatal("clone cap still depends on input backing capacity")
	}
	shared := InitialPacket(emptyID())
	data = make([]byte, 4096)
	for i := 0; i < 4200; i++ {
		r := testRead(Far{1, 0xff80}, data)
		shared.Calls = append(shared.Calls, R4{CallIndex: uint32(i), Table: r})
	}
	planned, e = clonePlan(shared)
	if e != nil {
		t.Fatal("shared output rejected using uninterned sum", e)
	}
	cloned := clonePacket(shared)
	actual = footprint(cloned) + 256
	if actual > planned || actual > MaxGraphBytes {
		t.Fatal("shared output exceeded reservation", actual, planned)
	}

	// Real low-total acceptance of a shared 4KiB table across 4200 witnesses.
	// The zero table is an explicit H2 diagnostic profile, never a fake Ready.
	scene, packet := materialFixture(0x338d)
	baseCall, baseW := scene.Raster.Calls[0], packet.Calls[0]
	baseCall.Params[7] = 0x1f04
	baseCall.Params[8] = 0xc89e
	baseW.RowOrigin = ExtraRead
	baseW.Table = testRead(Far{0xc89e, 0x1004}, make([]byte, 4096))
	baseW.Row = testRead(Far{0xc89e, 0x1f04}, make([]byte, 256))
	scene.Raster.Calls = nil
	packet.Calls = []R4{}
	for i := 0; i < 4200; i++ {
		c, w := baseCall, baseW
		c.Steps += uint64(i)
		c.Seq = uint16(i)
		w.Context.Steps = c.Steps
		w.CallIndex = uint32(i)
		scene.Raster.Calls = append(scene.Raster.Calls, c)
		packet.Calls = append(packet.Calls, w)
	}
	packet, decl, e := Finalize(scene, packet)
	if e != nil || packet.Status != H2 {
		t.Fatal("shared diagnostic fixture", e, packet.Status)
	}
	planned, e = clonePlan(packet)
	if e != nil {
		t.Fatal(e)
	}
	low, e := NewOwner(fixedReservation + planned)
	if e != nil {
		t.Fatal(e)
	}
	accepted, e := low.Accept(scene, packet, decl)
	if e != nil {
		t.Fatal("shared graph rejected under exact output reservation", e)
	}
	if accepted.bytes > planned || low.OwnedBytes() > low.TotalBytes() {
		t.Fatal("shared actual exceeded reserved total")
	}
	accepted.Release()
	low.Close()
	if low.OwnedBytes() != 0 {
		t.Fatal("shared low-budget acceptance leaked")
	}

}
