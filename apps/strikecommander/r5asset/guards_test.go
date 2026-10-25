package r5asset

import (
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/wicanr2/dosgolem/apps/strikecommander/shadesource"
)

func TestLifeGuardsHaveSafeCleanup(t *testing.T) {
	l, o := tinyPublished(t)
	a, _ := l.Acquire()
	defer l.Release(a)
	l.Release(a)
	b, _ := l.Acquire()
	defer l.Release(b)
	_, stale := l.Identity(a)
	other, e := shadesource.NewOwner(0)
	if e != nil {
		t.Fatal(e)
	}
	defer other.Close()
	adopt := l.Adopt(other, l.identity.SHA)
	if stale == nil {
		t.Fatal("stale generation accepted")
	}
	if adopt == nil {
		t.Fatal("wrong owner accepted")
	}
	_, e = l.begin(b)
	if e != nil {
		t.Fatal(e)
	}
	before := o.OwnedBytes()
	l.Close()
	l.Release(b)
	early := l.Stats().Drained || o.OwnedBytes() != before
	l.end() // Always end the started read before asserting, including mutants.
	if early {
		t.Fatal("active read backing refunded early")
	}
}
func TestCloseWhileLoadWaitsForWorker(t *testing.T) {
	sourceRoot, _ := fixtureRoots(t, true, false)
	l, o, base := loaderRig(t, 0, sourceRoot, Limits{})
	locked, release, workerDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(workerDone)
		_ = o.CompositionWorkspace(func() error { close(locked); <-release; return nil })
	}()
	<-locked
	done := make(chan error, 1)
	go func() { done <- l.Load() }()
	for {
		l.mu.Lock()
		started := l.loading
		l.mu.Unlock()
		if started {
			break
		}
		runtime.Gosched()
	}
	l.Close()
	close(release)
	e := <-done
	<-workerDone
	if e == nil {
		t.Fatal("Close failed to cancel Load")
	}
	l.WaitDrained()
	if l.Stats().Published || o.OwnedBytes() != base+l.limits.ControlBytes {
		t.Fatal("Close/Load leaked", l.Stats())
	}
}
func TestPNGInnerGuardBeforeDecoder(t *testing.T) {
	sourceRoot, _ := fixtureRoots(t, true, false)
	p := filepath.Join(sourceRoot, "f16/model/albedo.png")
	raw, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	if e = checkPNG(raw, 4); e != nil {
		t.Fatal(e)
	}
	for _, offset := range []int{16, 24, 28} { // width, bit depth, interlace
		b := append([]byte(nil), raw...)
		b[offset] ^= 1
		binary.BigEndian.PutUint32(b[29:33], crc32.ChecksumIEEE(b[12:29]))
		if e = checkPNG(b, 4); e == nil {
			t.Fatal("IHDR guard", offset)
		}
	}
	if e = checkPNG(raw, 3); e == nil {
		t.Fatal("slot channel mismatch")
	}
	raw[40] ^= 1
	if e = checkPNG(raw, 4); e == nil {
		t.Fatal("CRC guard")
	}
}
