package shadesource

import (
	"bytes"
	"encoding/json"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/model"
	"io"
	"testing"
)

type digestShortWriter struct{}

func (digestShortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }
func TestProtectedBoundedLegacyDigestAndReceipt(t *testing.T) {
	o, e := NewOwner(0)
	if e != nil {
		t.Fatal(e)
	}
	defer o.Close()
	pool, e := o.EnableProtectedProducer(0, 0)
	if e != nil {
		t.Fatal(e)
	}
	_ = pool
	s := model.Input{}
	old, e := o.BoundedLegacyDigest(s)
	if e != nil {
		t.Fatal(e)
	}
	o.worker.Lock()
	inside, e := o.profileLegacyDigest(s)
	o.worker.Unlock()
	if e != nil || inside != old {
		t.Fatal(e)
	}
	var wire bytes.Buffer
	if e = o.WriteBoundedLegacyJSON(s, &wire); e != nil {
		t.Fatal(e)
	}
	want, e := json.Marshal(s)
	if e != nil {
		t.Fatal(e)
	}
	want = append(want, '\n')
	if !bytes.Equal(want, wire.Bytes()) {
		t.Fatal("legacy receipt bytes changed")
	}
	p := InitialPacket(Identity{})
	want, e = MarshalPacket(p)
	if e != nil {
		t.Fatal(e)
	}
	want = append(want, '\n')
	wire.Reset()
	o.worker.Lock()
	e = o.profileWritePacket(&wire, p)
	o.worker.Unlock()
	if e != nil || !bytes.Equal(want, wire.Bytes()) {
		t.Fatal("packet receipt bytes changed", e)
	}
	if e = o.WriteBoundedLegacyJSON(s, digestShortWriter{}); e != io.ErrShortWrite {
		t.Fatal(e)
	}
	o.worker.Lock()
	e = o.profileWritePacket(digestShortWriter{}, p)
	o.worker.Unlock()
	if e != io.ErrShortWrite {
		t.Fatal(e)
	}
}
