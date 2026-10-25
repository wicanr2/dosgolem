package r5asset

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"unsafe"

	"github.com/wicanr2/dosgolem/apps/strikecommander/shadesource"
)

func loaderRig(t *testing.T, total uint64, root string, limits Limits) (*Loader, *shadesource.Owner, uint64) {
	t.Helper()
	o, e := shadesource.NewOwner(total)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = o.EnableProtectedProducer(0, 0); e != nil {
		t.Fatal(e)
	}
	base := o.OwnedBytes()
	l, e := NewForConfiguredHD(o, root, limits)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		l.Close()
		l.WaitDrained()
		if e := l.Dispose(); e != nil {
			t.Error(e)
		}
		if n := o.OwnedBytes(); n != base {
			t.Errorf("shared owner leaked %d want %d", n, base)
		}
		o.Close()
		if n := o.OwnedBytes(); n != 0 {
			t.Errorf("owner after Close %d", n)
		}
	})
	return l, o, base
}
func TestWholeBatchRealPixelsIdentityAndTransfer(t *testing.T) {
	sourceRoot, evidenceRoot := fixtureRoots(t, true, true)
	l, o, base := loaderRig(t, 0, sourceRoot, Limits{})
	if e := l.Load(); e != nil {
		t.Fatal(e)
	}
	t.Logf("sizes loader=%d asset=%d arena=%d workspace=%d allocation=%d page=%d stats=%+v", unsafe.Sizeof(Loader{}), unsafe.Sizeof(assetData{}), arenaBytes, textureWorkBytes, allocationOverhead, unsafe.Sizeof(page{}), l.Stats())
	if l.Stats().GlobalPeak > 268435456 || l.Stats().WorkerPeak != 3692800 {
		t.Fatal("controlled bound", l.Stats())
	}
	r, e := l.Acquire()
	if e != nil {
		t.Fatal(e)
	}
	defer l.Release(r)
	id, e := l.Identity(r)
	if e != nil || id.SHA != batchHash() {
		t.Fatal(id, e)
	}
	other, e := shadesource.NewOwner(0)
	if e != nil {
		t.Fatal(e)
	}
	defer other.Close()
	before := o.OwnedBytes()
	if e = l.Adopt(other, id.SHA); e == nil || o.OwnedBytes() != before {
		t.Fatal("wrong-owner adoption")
	}
	if e = l.Adopt(o, id.SHA); e != nil || o.OwnedBytes() != before {
		t.Fatal("same-owner adoption", e)
	}
	if e = l.Adopt(o, id.SHA); e == nil {
		t.Fatal("second publication right")
	}
	type row struct{ Model, Name, RawChannelSHA256 string }
	var oracle struct{ Rows []row }
	raw, e := os.ReadFile(filepath.Join(evidenceRoot, "texture-prototype-001/pixel-oracle-001.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(raw, &oracle); e != nil {
		t.Fatal(e)
	}
	var buf [4096 * 4]byte
	for m, p := range profiles {
		for slot, f := range p.Images {
			h := sha256.New()
			c := 3
			if f.Color == 6 {
				c = 4
			}
			for y := 0; y < 4096; y++ {
				for x := 0; x < 4096; x++ {
					px, n, e := l.Pixel(r, Model(m+1), slot, x, y)
					if e != nil || int(n) != c {
						t.Fatal(m, slot, x, y, n, e)
					}
					if c == 3 && px[3] != 255 {
						t.Fatal("RGB alpha")
					}
					copy(buf[x*c:(x+1)*c], px[:c])
				}
				h.Write(buf[:4096*c])
			}
			expected := oracle.Rows[m*3+slot]
			if expected.Model != p.Name || expected.Name != f.Name || hex.EncodeToString(h.Sum(nil)) != expected.RawChannelSHA256 {
				t.Fatal("independent Pillow raw-channel SHA", m, slot)
			}
			desc, e := l.Texture(r, Model(m+1), slot)
			if e != nil || desc.Channels != uint8(c) || desc.SHA != f.SHA {
				t.Fatal(desc, e)
			}
			if _, _, e = l.Pixel(r, Model(m+1), slot, 4096, 0); e == nil {
				t.Fatal("clamp silently accepted")
			}
		}
	}
	l.Close()
	if _, e = l.Acquire(); e == nil {
		t.Fatal("Acquire after Close")
	}
	if _, e = l.Vertex(r, F16, 0, 0); e != nil {
		t.Fatal("held reader lost data after Close", e)
	}
	if o.OwnedBytes() != before {
		t.Fatal("Close refunded reachable data")
	}
	l.Release(r)
	l.WaitDrained()
	if n := o.OwnedBytes(); n != base+l.limits.ControlBytes {
		t.Fatal("drain baseline", n)
	}
	if _, e = l.Vertex(r, F16, 0, 0); e == nil {
		t.Fatal("released reader accepted")
	}
}

// Synthetic Published fixture tests only lifecycle, never accepted source.
func tinyPublished(t *testing.T) (*Loader, *shadesource.Owner) {
	l, o, _ := loaderRig(t, 0, t.TempDir(), Limits{})
	h, e := l.allocate(12)
	if e != nil {
		t.Fatal(e)
	}
	d := new(assetData)
	d.bin[0] = make([]byte, 12)
	d.binHold[0] = h
	l.data = d
	return l, o
}
func TestReaderABAActiveCloseAndValueIsolation(t *testing.T) {
	l, o := tinyPublished(t)
	r, e := l.Acquire()
	if e != nil {
		t.Fatal(e)
	}
	r2, e := l.Acquire()
	if e != nil {
		t.Fatal(e)
	}
	if _, e = l.Acquire(); e == nil {
		t.Fatal("third reader")
	}
	l.Release(r)
	newR, e := l.Acquire()
	if e != nil || newR == r {
		t.Fatal("ABA", e)
	}
	if _, e = l.Identity(r); e == nil {
		t.Fatal("stale token")
	}
	l.Release(r)
	id, e := l.Identity(newR)
	if e != nil {
		t.Fatal(e)
	}
	id.Assets[0].Files[0].Bytes = 0
	still, e := l.Identity(newR)
	if e != nil || still.Assets[0].Files[0].Bytes != 9840 {
		t.Fatal("value alias")
	}
	_, e = l.begin(newR)
	if e != nil {
		t.Fatal(e)
	}
	before := o.OwnedBytes()
	l.Close()
	l.Release(newR)
	l.Release(r2)
	if l.Stats().Drained || o.OwnedBytes() != before {
		t.Fatal("active read refunded early")
	}
	l.end()
	l.WaitDrained()
	l.mu.Lock()
	l.slots[0].generation = ^uint64(0)
	l.slots[1].generation = ^uint64(0)
	l.mu.Unlock()
	// A separate still-open synthetic Published fixture exercises overflow.
	l2, _ := tinyPublished(t)
	l2.slots[0].generation = ^uint64(0)
	l2.slots[1].generation = ^uint64(0)
	if _, e = l2.Acquire(); e == nil {
		t.Fatal("generation wrap")
	}
}
func TestConcurrentReadersClose(t *testing.T) {
	l, _ := tinyPublished(t)
	r, _ := l.Acquire()
	s, _ := l.Acquire()
	var wg sync.WaitGroup
	wg.Add(2)
	for _, token := range []Reader{r, s} {
		go func(r Reader) {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				if _, e := l.Identity(r); e != nil {
					t.Error(e)
					return
				}
			}
			l.Release(r)
		}(token)
	}
	l.Close()
	wg.Wait()
	l.WaitDrained()
}
func TestLowCallerQuotaBeforeLargeBackingAndRollback(t *testing.T) {
	sourceRoot, _ := fixtureRoots(t, true, false)
	// Only 8KiB outside fixed control: even the first bin cannot be allocated.
	total := uint64(40<<20|64<<10) + 64<<20 + 64<<10 + 8192
	l, o, base := loaderRig(t, total, sourceRoot, Limits{})
	runtime.GC()
	var a, b runtime.MemStats
	runtime.ReadMemStats(&a)
	e := l.Load()
	runtime.ReadMemStats(&b)
	if e == nil {
		t.Fatal("quota succeeded")
	}
	v, ok := e.(Error)
	if !ok || v.Code != Capacity {
		t.Fatal(e)
	}
	if o.OwnedBytes() != base+l.limits.ControlBytes || l.Stats().Published {
		t.Fatal("transaction leaked", l.Stats())
	}
	if b.TotalAlloc-a.TotalAlloc > 1<<20 {
		t.Fatal("large backing before quota", b.TotalAlloc-a.TotalAlloc)
	}
	// Worker limit rejects before arena/ParseFloat/Pix allocation.
	l2, o2, base2 := loaderRig(t, 0, sourceRoot, Limits{WorkerBytes: JSONReservation - 1})
	if e = l2.Load(); e == nil || o2.OwnedBytes() != base2+l2.limits.ControlBytes {
		t.Fatal("worker low bound", e)
	}
}
func TestTransparentRawTileCollisionAndPalette(t *testing.T) {
	l, _, _ := loaderRig(t, 0, t.TempDir(), Limits{})
	w := new(workspace)
	t4, e := newTexture(4, l)
	if e != nil {
		t.Fatal(e)
	}
	defer t4.close()
	var a, b [1024]byte
	for i := 0; i < 256; i++ {
		copy(a[i*4:], []byte{71, 13, 255, 0})
		copy(b[i*4:], []byte{71, 14, 255, 0})
	}
	var hand [1024]byte
	copyTile(a[:], 16*4, 0, 0, 4, hand[:])
	if !bytes.Equal(hand[:], a[:]) {
		t.Fatal("tile capture changed straight hidden RGB")
	}
	i, e := intern(t4, w, a[:], 7, l)
	if e != nil {
		t.Fatal(e)
	}
	j, e := intern(t4, w, b[:], 7, l)
	if e != nil || i == j {
		t.Fatal("hash collision accepted", e)
	}
	k, e := intern(t4, w, a[:], 7, l)
	if e != nil || k != i {
		t.Fatal("identical tile not interned", e)
	}
	if e = t4.read(i, w.check[:]); e != nil || !bytes.Equal(w.check[:], a[:]) {
		t.Fatal("straight hidden RGB", e)
	}
	t4.Index[0] = i
	p, c, e := t4.pixel(0, 0)
	if e != nil || c != 4 || p != [4]byte{71, 13, 255, 0} {
		t.Fatal(p, c, e)
	}
	// Palette code corruption is rejected by the actual read path.
	rec := t4.Records[i]
	pg := t4.Pages[int(rec.Offset)/pageSize].data
	at := int(rec.Offset)%pageSize + int(rec.Palette)*4
	pg[at] = 255
	if e = t4.read(i, w.check[:]); e == nil {
		t.Fatal("bad palette code")
	}
}
