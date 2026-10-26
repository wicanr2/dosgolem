package player

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
	"time"
)

// hideAt forces the reader-only fs.File path, with no Name, ReaderAt or Seeker.
type readerOnlyFS struct{ fs.FS }
type readerOnlyFile struct{ fs.File }

func (f readerOnlyFS) ReadDir(n string) ([]fs.DirEntry, error) { return fs.ReadDir(f.FS, n) }
func (f readerOnlyFS) Open(n string) (fs.File, error) {
	v, e := f.FS.Open(n)
	if e != nil {
		return nil, e
	}
	return readerOnlyFile{v}, nil
}
func TestVirtualSourceNoNativeName(t *testing.T) {
	fsys := readerOnlyFS{fstest.MapFS{"original/disk.iso": &fstest.MapFile{Data: fixtureISO()}, "original/disk.cue": &fstest.MapFile{Data: []byte("FILE \"disk.iso\" BINARY\nTRACK 1 MODE1/2048\nINDEX 1 00:00:00\n")}}}
	files, e := virtualImage(fsys, "original/disk.cue", "opaque-drop")
	if e != nil || len(files) != 1 {
		t.Fatal(files, e)
	}
	r, e := files[0].Open()
	if e != nil {
		t.Fatal(e)
	}
	b, e := io.ReadAll(r)
	r.Close()
	if e != nil || string(b) != "hello" {
		t.Fatal(string(b), e)
	}
	spec := synthSpec("A.DAT", "hello")
	v := []VirtualSource{{FS: fsys, Root: "original", Label: "opaque-drop"}}
	selection, e := LocateWithVirtual(context.Background(), nil, v, []FileSpec{spec}, nil)
	if e != nil || len(selection.Files) != 1 {
		t.Fatal(selection, e)
	}
	d := t.TempDir()
	cache := Cache{Root: filepath.Join(d, "cache"), Specs: []FileSpec{spec}, Resolve: func(ctx context.Context) (Selection, error) {
		return LocateWithVirtual(ctx, nil, v, []FileSpec{spec}, nil)
	}}
	tree, _, e := cache.Ensure(context.Background(), nil, filepath.Join(d, "save"), false)
	if e != nil {
		t.Fatal(e)
	}
	if b, e := os.ReadFile(filepath.Join(tree, "A.DAT")); e != nil || string(b) != "hello" {
		t.Fatal(string(b), e)
	}
	if e = ValidateDropNames([]string{"x/disk.iso", "y/disk.iso"}); e == nil {
		t.Fatal("duplicate basename accepted")
	}
}
func TestOriginalCacheReadOnly(t *testing.T) {
	image := os.Getenv("SC_PLAYER_IMAGE")
	if image == "" {
		t.Skip("original RO image not selected")
	}
	before, e := os.Stat(image)
	if e != nil {
		t.Fatal(e)
	}
	h, e := hashPath(image)
	if e != nil {
		t.Fatal(e)
	}
	d := t.TempDir()
	cache := Cache{Root: filepath.Join(d, "cache")}
	start := time.Now()
	tree, _, e := cache.Ensure(context.Background(), []string{image}, filepath.Join(d, "save"), false)
	if e != nil {
		t.Fatal(e)
	}
	elapsed := time.Since(start)
	start = time.Now()
	if e = cache.Verify(); e != nil {
		t.Fatal(e)
	}
	verify := time.Since(start)
	after, e := os.Stat(image)
	if e != nil {
		t.Fatal(e)
	}
	h2, e := hashPath(image)
	if e != nil || h2 != h || !before.ModTime().Equal(after.ModTime()) || before.Size() != after.Size() {
		t.Fatal("source changed", e)
	}
	entries, e := os.ReadDir(tree)
	if e != nil || len(entries) != 33 {
		t.Fatal(len(entries), e)
	}
	used, e := cache.UsedSources()
	if e != nil || !reflect.DeepEqual(used, []string{image}) {
		t.Fatal(used, e)
	}
	if w, e := CheckSaveShadows(filepath.Join(d, "save"), tree, RequiredFiles); e != nil || len(w) != 0 {
		t.Fatal(w, e)
	}
	t.Logf("source SHA256=%s,33 files,import=%s,fullVerify=%s; includes I/O, not VM or Prepare performance", h, elapsed, verify)
}
