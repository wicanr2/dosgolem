package hd3d

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

func testList() []uint16 {
	a := make([]uint16, 201)
	for i := range a {
		a[i] = uint16(16 + i*320)
	}
	return a
}
func testTables() Tables {
	b := bytes.Repeat([]byte{9}, 200)
	for i, o := range []uint16{8, 56, 104, 152} {
		binary.LittleEndian.PutUint16(b[0xc0+i*2:], o)
	}
	return Tables{b}
}
func testR6() Call {
	return Call{Kind: R6, Desc: [6]uint16{0xffff, 0xe654, 0, 0, 319, 199}, Params: [13]uint16{0, 43, 32, 160, 32}, DS: 0x1000, SS: 0x1000, Aux: AuxRef{Color0: 9, Color1: 9, HasFlags: true}}
}
func testRow(kind CallKind, xs [3]uint16, color uint16) Call {
	k := Call{Kind: kind, Seq: 7, Desc: [6]uint16{0xffff, 0xe654, 0, 0, 319, 199}, Params: [13]uint16{0, 3, 0x100}, Verts: make([]byte, 66)}
	for i, x := range xs {
		binary.LittleEndian.PutUint16(k.Verts[i*22:], x)
		binary.LittleEndian.PutUint16(k.Verts[i*22+2:], 20)
		binary.LittleEndian.PutUint16(k.Verts[i*22+4:], color)
	}
	return k
}
func single(k Call, seed byte) *context {
	c := newContext(testList(), testTables())
	for i := range c.buf {
		c.buf[i] = seed
		c.written[i] = true
	}
	runCalls(c, []Call{k}, false)
	return c
}

func TestFlatAndLookupHandValues(t *testing.T) {
	k := testRow(R1, [3]uint16{10, 14, 12}, 7)
	c := single(k, 5)
	for x := 8; x < 17; x++ {
		want := byte(5)
		if x >= 10 && x <= 14 {
			want = 7
		}
		if c.buf[20*320+x] != want {
			t.Fatalf("R1 x=%d got=%d want=%d", x, c.buf[20*320+x], want)
		}
	}
	k.Kind = R2
	k.Table = make([]byte, 256)
	for i := range k.Table {
		k.Table[i] = byte(i + 1)
	}
	c = single(k, 5)
	for x := 8; x < 17; x++ {
		want := byte(5)
		if x >= 10 && x < 14 {
			want = 6
		}
		if c.buf[20*320+x] != want {
			t.Fatalf("R2 x=%d got=%d want=%d", x, c.buf[20*320+x], want)
		}
	}
	k = testRow(R2, [3]uint16{10, 11, 10}, 7)
	k.Table = make([]byte, 256)
	for i := range k.Table {
		k.Table[i] = 99
	}
	c = single(k, 5)
	if c.buf[20*320+10] != 5 || c.buf[20*320+11] != 5 {
		t.Fatal("R2 R-L=1 must write nothing")
	}
}

func TestCaptureOffsetsWrapAndMissing(t *testing.T) {
	c := newContext(testList(), Tables{})
	k := Call{Kind: R4}
	c.call = &k
	s, err := c.capture(Capture{[]Span{{0xfffe, []byte{1, 2, 3, 4}}}}, "wrap")
	if err != nil {
		t.Fatal(err)
	}
	for off, want := range map[uint16]byte{0xfffe: 1, 0xffff: 2, 0: 3, 1: 4} {
		if got := c.readCapture(s, off); got != want {
			t.Fatalf("off=%04x: %d want %d", off, got, want)
		}
	}
	c.readCapture(s, 2)
	if c.capShort != 1 || len(c.faults) == 0 {
		t.Fatal("missing span must fail")
	}
}

func TestR4SinglePointExceptionShadeAndTransparency(t *testing.T) {
	k := testRow(R4, [3]uint16{10, 10, 10}, 0)
	k.Params[4], k.Params[6] = 0xfff0, 1
	k.Tex.Pixels = Capture{[]Span{{0xfff0, []byte{7}}}}
	c := single(k, 9)
	if c.buf[20*320+10] != 7 || len(c.uninit) != 0 || len(c.faults) != 0 {
		t.Fatalf("R4 single point exception: %v %v", c.uninit, c.faults)
	}
	k.Params[7], k.Params[8] = 0xfffe, 0x1234
	shade := make([]byte, 256)
	for i := range shade {
		shade[i] = byte(100 + i)
	}
	k.Tex.Shade = Capture{[]Span{{0xfffe, shade}}}
	c = single(k, 9)
	if c.buf[20*320+10] != 107 || len(c.faults) != 0 {
		t.Fatalf("shade offset/wrap: %d %v", c.buf[20*320+10], c.faults)
	}
	k.Tex.Pixels = Capture{[]Span{{0xfff0, []byte{255}}}}
	c = single(k, 9)
	if c.buf[20*320+10] != 9 || c.writer[20*320+10] != 0xffff {
		t.Fatal("FF texture must remain transparent")
	}
}

func TestBlackBoxPatchNestedAndMetadata(t *testing.T) {
	before := bytes.Repeat([]byte{9}, PixelCount)
	after := append([]byte(nil), before...)
	known := testRow(R1, [3]uint16{10, 14, 12}, 7)
	known.Seq = 1
	known.Object = true
	known.Element = FarPtr{0x20, 0x1234}
	for x := 10; x <= 14; x++ {
		after[20*320+x] = 7
	}
	after[20*320+10] = 123
	patch, d := MakePatch(before, after, []Call{known}, testList(), testTables())
	if len(d.Faults) != 0 || len(patch) != 1 || patch[0] != (PixelSet{20*320 + 10, 123}) {
		t.Fatalf("patch includes known raster writes: %v %v", patch, d.Faults)
	}
	black := Call{Seq: 2, Kind: BlackBox, Patch: patch, Element: FarPtr{0x30, 0x4321}}
	r := Replay(Group{List: testList(), Calls: []Call{testR6(), known, black}, Buf3D: after}, testTables())
	i := 20*320 + 10
	if !r.Repaintable || r.Writer[i] != 2 || !r.Written[i] || !r.ObjectWritten[i] || r.CallElements[2] != black.Element {
		t.Fatalf("patch metadata: valid=%t diag=%+v", r.Valid, r.Diagnostics)
	}
	if before[i] != 9 || after[i] != 123 {
		t.Fatal("input snapshots modified")
	}
	black.Patch = []PixelSet{{PixelCount, 1}}
	r = Replay(Group{List: testList(), Calls: []Call{testR6(), black}}, testTables())
	if r.Valid || r.Diagnostics.OutOfBounds != 1 {
		t.Fatal("out of bounds patch must fail")
	}
}

func TestUninitializedAfterLastRule(t *testing.T) {
	for _, writeAfter := range []bool{false, true} {
		c := newContext(testList(), Tables{})
		k := Call{Kind: R5}
		c.call = &k
		c.beginCall()
		f := newFrame(c, "synthetic frame", 32)
		f.r8(-1)
		if writeAfter {
			c.w(16, 7)
		}
		c.endCall()
		if writeAfter && len(c.uninit) == 0 {
			t.Fatal("read before later write must fail")
		}
		if !writeAfter && (len(c.uninit) != 0 || len(c.uninitAfterLast) == 0) {
			t.Fatal("read after last write should be classified separately")
		}
	}
}

func TestArithmeticFaultsAndSentinels(t *testing.T) {
	c := newContext(testList(), Tables{})
	k := Call{Kind: R1}
	c.call = &k
	q, r := c.idiv64(-7, 3, "test")
	if q != -2 || r != -1 {
		t.Fatal(q, r)
	}
	c.idiv64(math.MinInt64, -1, "overflow")
	c.idiv32(math.MinInt32, -1, "overflow16")
	if len(c.faults) != 2 {
		t.Fatal("idiv overflows must fault")
	}
	if shrd16(-1) != math.MaxUint32 || sx16(0xffff) != math.MaxUint32 || cnt8(0) != 256 {
		t.Fatal("instruction widths changed")
	}
	if shrd12(1<<32) != 1<<20 || shrd16(1<<32) != 1<<16 {
		t.Fatal("R5 12-bit and edge 16-bit extraction must stay distinct")
	}
	rp := Replay(Group{}, Tables{})
	if rp.Valid || rp.Writer[0] != 0xffff || rp.Written[0] {
		t.Fatal("unwritten sentinel")
	}
	r6 := testR6()
	r6.Seq = 0xffff
	rp = Replay(Group{List: testList(), Calls: []Call{r6}}, testTables())
	if rp.Valid {
		t.Fatal("reserved sequence must fail")
	}
	r6 = testR6()
	r6.Aux.Flags = 0x400
	rp = Replay(Group{List: testList(), Calls: []Call{r6}}, testTables())
	if rp.Valid {
		t.Fatal("DF=1 must fail")
	}
	r6 = testR6()
	r6.SS++
	rp = Replay(Group{List: testList(), Calls: []Call{r6}}, testTables())
	if rp.Valid {
		t.Fatal("DS!=SS must fail")
	}
	if _, err := TablesFromEXE(nil); err == nil {
		t.Fatal("real tables cannot be synthesized as fallback")
	}
}

// Ported from tools/m6replay/r3_test.go: these establish mutation sensitivity,
// not original parity for the still-unsampled R3 variants.
func TestM6Detectable(t *testing.T) {
	k := testRow(R3, [3]uint16{10, 40, 20}, 0x7f00)
	binary.LittleEndian.PutUint16(k.Verts[22+4:], 0x8100)
	a, b := newContext(testList(), Tables{}), newContext(testList(), Tables{})
	runCalls(a, []Call{k}, false)
	b.mut = map[string]bool{"r3movzx": true}
	runCalls(b, []Call{k}, false)
	diff := 0
	for i := range a.buf {
		if a.written[i] != b.written[i] || a.buf[i] != b.buf[i] {
			diff++
		}
	}
	if diff == 0 || len(a.faults) != 0 || len(b.faults) != 0 {
		t.Fatal("M6 synthetic sign-bit mutation not detected")
	}
	t.Logf("single-row M6 differing pixels %d", diff)
}

func TestMovsxGeneralDetectable(t *testing.T) {
	k := testRow(R3, [3]uint16{10, 60, 5}, 0)
	for i, p := range [][2]uint16{{10, 0x7f00}, {40, 0x8100}, {70, 0x2000}} {
		binary.LittleEndian.PutUint16(k.Verts[i*22+2:], p[0])
		binary.LittleEndian.PutUint16(k.Verts[i*22+4:], p[1])
	}
	a, b := newContext(testList(), Tables{}), newContext(testList(), Tables{})
	runCalls(a, []Call{k}, false)
	b.mut = map[string]bool{"r3movsxgen": true}
	runCalls(b, []Call{k}, false)
	diff := 0
	for i := range a.buf {
		if a.written[i] != b.written[i] || a.buf[i] != b.buf[i] {
			diff++
		}
	}
	if diff == 0 {
		t.Fatal("general movzx/movsx mutation not detected")
	}
	t.Logf("general sign-extension differing pixels %d", diff)
}
