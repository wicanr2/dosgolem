package r5asset

import (
	"crypto/sha256"
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestJSONEachFixedBoundary(t *testing.T) {
	array := func(n int) string { return "[" + strings.Repeat("0,", n-1) + "0]" }
	object := func(n int) string {
		var b strings.Builder
		b.WriteByte('{')
		for i := 0; i < n; i++ {
			if i != 0 {
				b.WriteByte(',')
			}
			b.WriteString(`"k` + strconv.Itoa(i) + `":0`)
		}
		b.WriteByte('}')
		return b.String()
	}
	valid := []string{array(64), object(32), `"` + strings.Repeat(`\n`, 128) + `"`, `"` + strings.Repeat("😀", 64) + `"`, "1." + strings.Repeat("0", 62), "0" + strings.Repeat(" ", 9839)}
	invalid := []string{array(65), object(33), `"` + strings.Repeat(`\n`, 129) + `"`, `"` + strings.Repeat("😀", 65) + `"`, "1." + strings.Repeat("0", 63), "0" + strings.Repeat(" ", 9840)}
	for i, s := range valid {
		if _, e := new(jsonArena).parse([]byte(s)); e != nil {
			t.Fatal("boundary positive", i, e)
		}
	}
	for i, s := range invalid {
		if _, e := new(jsonArena).parse([]byte(s)); e == nil {
			t.Fatal("boundary negative", i)
		}
	}
	// Largest node fixture is exactly 2048 values / 4095 tokens. The next value
	// also crosses token capacity; these are not two independent mutant claims.
	s := "[" + strings.Repeat(array(63)+",", 31) + array(62) + "]"
	a := new(jsonArena)
	if _, e := a.parse([]byte(s)); e != nil || a.nodes != 2048 || a.n != 4095 {
		t.Fatal("node/token positive", a.nodes, a.n, e)
	}
	if _, e := new(jsonArena).parse([]byte("[" + strings.Repeat(array(63)+",", 31) + array(63) + "]")); e == nil {
		t.Fatal("node/token upper bound")
	}
}
func TestDecoderFailureAfterInnerCRCAndIdentity(t *testing.T) {
	sourceRoot, _ := fixtureRoots(t, true, false)
	p := profiles[0]
	s := p.Images[0]
	raw, e := os.ReadFile(filepath.Join(sourceRoot, p.Name, "model", s.Name))
	if e != nil {
		t.Fatal(e)
	}
	// Change the first IDAT's zlib method, then make CRC AND the inner file SHA
	// self-consistent. This reaches png.Decode instead of outer identity/CRC.
	off := 33
	n := int(binary.BigEndian.Uint32(raw[off : off+4]))
	if string(raw[off+4:off+8]) != "IDAT" {
		t.Fatal("fixture profile")
	}
	raw[off+8] = 0
	binary.BigEndian.PutUint32(raw[off+8+n:off+12+n], crc32.ChecksumIEEE(raw[off+4:off+8+n]))
	s.SHA = sha256.Sum256(raw)
	root := t.TempDir()
	dir := filepath.Join(root, p.Name, "model")
	if e = os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, s.Name), raw, 0600); e != nil {
		t.Fatal(e)
	}
	l, o, base := loaderRig(t, 0, root, Limits{})
	// A synthetic inner fixture only; normal Load still pins the original SHA.
	var err error
	err = o.CompositionWorkspace(func() error {
		w := workerBudget{limit: 32 << 20}
		if e := w.reserve(textureWorkBytes + decoderReserve); e != nil {
			return e
		}
		_, temp, e := l.loadImage(p, s)
		temp.release()
		w.release(textureWorkBytes + decoderReserve)
		return e
	})
	v, ok := err.(Error)
	if !ok || v.Code != Decode {
		t.Fatal("did not reach decoder", err)
	}
	if o.OwnedBytes() != base+l.limits.ControlBytes {
		t.Fatal("decoder temporary leak", o.OwnedBytes())
	}
}
