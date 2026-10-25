package shadesource

import (
	"bytes"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/model"
	"sync"
	"testing"
)

func emptyInput() model.Input { return model.Input{Key: 1, Raster: hd3d.Group{Key: 1}} }
func emptyID() Identity {
	return Identity{EXESHA: HashLiteral(EXESHAHex), SignatureSetSHA: HashLiteral(SignatureSetSHAHex), Base: 577, ModelsGeneration: 1, ResetGeneration: 1, ShadeEpoch: 1, GroupSerial: 1, CUKey: 1}
}
func TestOwnerDeclaredImmutableAndMixedLimit(t *testing.T) {
	o, e := NewOwner(0)
	if e != nil {
		t.Fatal(e)
	}
	s := emptyInput()
	p, d, e := Finalize(s, InitialPacket(emptyID()))
	if e != nil {
		t.Fatal(e)
	}
	a, e := o.Accept(s, p, d)
	if e != nil {
		t.Fatal(e)
	}
	first, e := leaseJSON(a)
	if e != nil {
		t.Fatal(e)
	}
	bad := p
	bad.Counters.Candidates++
	if _, e = o.Accept(s, bad, d); e == nil {
		t.Fatal("modified extra accepted under fixed external digest")
	}
	_, d2, e := Finalize(s, bad)
	if e != nil {
		t.Fatal(e)
	}
	b, e := o.Accept(s, bad, d2)
	if e != nil {
		t.Fatal(e)
	}
	if a == b || a.Declaration() == b.Declaration() {
		t.Fatal("same legacy different extra aliased")
	}
	if _, e = o.Accept(s, p, d); e == nil {
		t.Fatal("third external lease")
	}
	after, _ := leaseJSON(a)
	if !bytes.Equal(first, after) {
		t.Fatal("first immutable receipt changed")
	}
	for i := 0; i < 3; i++ {
		if _, e = o.NewGraph(emptyID()); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = o.NewGraph(emptyID()); e == nil {
		t.Fatal("fourth recorder graph")
	}
	o.Close()
	if o.OwnedBytes() < fixedReservation {
		t.Fatal("Close released consumer workspace")
	}
	a.Release()
	a.Release()
	if o.OwnedBytes() == 0 {
		t.Fatal("Close dropped live external")
	}
	b.Release()
	if o.OwnedBytes() != 0 {
		t.Fatalf("leak %d", o.OwnedBytes())
	}
}
func TestOwnerPendingRetireAndRawLease(t *testing.T) {
	o, _ := NewOwner(0)
	r, e := o.ReservePending()
	if e != nil {
		t.Fatal(e)
	}
	if _, e = o.ReservePending(); e == nil {
		t.Fatal("second pending")
	}
	r.Release()
	g, e := o.NewGraph(emptyID())
	if e != nil {
		t.Fatal(e)
	}
	g.Seal()
	l, e := g.Acquire()
	if e != nil {
		t.Fatal(e)
	}
	g.Retire()
	if _, e = g.Acquire(); e == nil {
		t.Fatal("retired handle reacquired")
	}
	o.Close()
	p, _, e := exportPacket(l, emptyInput())
	if e != nil || p.Status != H2 {
		t.Fatal("live external invalidated by Close", e)
	}
	if o.OwnedBytes() == 0 {
		t.Fatal("live bytes uncharged")
	}
	l.Release()
	if o.OwnedBytes() != 0 {
		t.Fatal("last lease leak")
	}
}
func TestOwnerConcurrentCapacity(t *testing.T) {
	o, _ := NewOwner(0)
	p, d, e := Finalize(emptyInput(), InitialPacket(emptyID()))
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	got := make(chan *Lease, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if l, e := o.Accept(emptyInput(), p, d); e == nil {
				got <- l
			}
		}()
	}
	wg.Wait()
	close(got)
	n := 0
	for l := range got {
		n++
		l.Release()
	}
	if n != 2 {
		t.Fatalf("concurrent result count %d", n)
	}
	o.Close()
	if o.OwnedBytes() != 0 {
		t.Fatal("concurrent capacity leaked")
	}
}
func TestOwnerLowBudgetAndNoRawAlias(t *testing.T) {
	o, _ := NewOwner(fixedReservation + 4096)
	g, e := o.NewGraph(emptyID())
	if e != nil {
		t.Fatal(e)
	}
	if _, e = o.ReservePending(); e == nil {
		t.Fatal("8KiB reserve exceeded lower total")
	}
	g.Retire()
	o.Close()
	if o.OwnedBytes() != 0 {
		t.Fatal("capacity rollback")
	}
	o, _ = NewOwner(0)
	g, _ = o.NewGraph(emptyID())
	r, _ := o.ReservePending()
	b := Build{Raw5: CaptureRead(func(_ uint32, p []byte) bool { p[4] = 9; return true }, Far{1, 2}, 5)}
	if e = g.AddBuild(b, r); e != nil {
		t.Fatal(e)
	}
	x := g.BuildValue(Index(0))
	x.Raw5.Spans[0].Data[4] = 99
	y := g.BuildValue(Index(0))
	if y.Raw5.Spans[0].Data[4] != 9 {
		t.Fatal("BuildValue exposed raw backing")
	}
	g.Retire()
	o.Close()
	if o.OwnedBytes() != 0 {
		t.Fatal("raw copy leak")
	}
}

func TestClonePreservesSharedWrappedTable(t *testing.T) {
	p := InitialPacket(emptyID())
	data := make([]byte, 4096)
	for i := range data {
		data[i] = byte(i)
	}
	for i := 0; i < 20; i++ {
		r := testRead(Far{1, 0xff80}, data)
		p.Calls = append(p.Calls, R4{CallIndex: uint32(i), Table: r})
	}
	q := clonePacket(p)
	first := &q.Calls[0].Table.Spans[0].Data[0]
	for _, c := range q.Calls {
		if &c.Table.Spans[0].Data[0] != first {
			t.Fatal("shared 4096 content multiplied by witness count")
		}
		if !bytes.Equal(readBytes(c.Table), data) {
			t.Fatal("shared wrapped content changed")
		}
	}
	q.Calls[0].Table.Spans[0].Data[0] ^= 1
	if readBytes(p.Calls[0].Table)[0] != 0 {
		t.Fatal("clone aliases original caller bytes")
	}
}

func leaseJSON(l *Lease) ([]byte, error) {
	var b bytes.Buffer
	e := l.WriteJSON(&b)
	return b.Bytes(), e
}
func exportPacket(l *RawLease, s model.Input) (Packet, Declaration, error) {
	var b bytes.Buffer
	d, e := l.Export(s, &b)
	p, decode := UnmarshalPacket(b.Bytes())
	if decode != nil {
		return p, d, decode
	}
	return p, d, e
}
