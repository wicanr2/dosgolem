package player

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/wicanr2/dosgolem/session"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestPaths(t *testing.T) {
	for _, tc := range []struct {
		os                string
		env               map[string]string
		home, data, cache string
	}{{"linux", map[string]string{"XDG_DATA_HOME": "relative", "XDG_CACHE_HOME": "relative"}, "/h", "/h/.local/share/strikecommander-cht", "/h/.cache/strikecommander-cht"}, {"linux", map[string]string{"XDG_DATA_HOME": "/d", "XDG_CACHE_HOME": "/c"}, "/h", "/d/strikecommander-cht", "/c/strikecommander-cht"}, {"darwin", nil, "/h", "/h/Library/Application Support/StrikeCommanderCHT", "/h/Library/Application Support/StrikeCommanderCHT/cache"}, {"windows", nil, `C:\Users\p`, `C:\Users\p\AppData\Roaming\StrikeCommanderCHT`, `C:\Users\p\AppData\Local\StrikeCommanderCHT\cache`}} {
		p, e := PathsFor(tc.os, tc.env, tc.home)
		if e != nil || p.Data != tc.data || p.Cache != tc.cache {
			t.Fatalf("%s %+v %v", tc.os, p, e)
		}
	}
}
func TestSetUnionSuppression(t *testing.T) {
	var s InputState
	apply := func(h map[string]bool, c InputContext) []session.ScanEvent {
		in := HostInput{Held: h}
		s.Observe(in, c)
		r := s.Build(in, c)
		s.Commit(r)
		return r.Update.ScanEvents
	}
	if got := apply(map[string]bool{"ArrowUp": true, "Numpad8": true}, InputContext{}); !reflect.DeepEqual(got, []session.ScanEvent{{Scan: 0x48}}) {
		t.Fatal(got)
	}
	if got := apply(map[string]bool{"Numpad8": true}, InputContext{}); len(got) != 0 {
		t.Fatal(got)
	}
	if got := apply(nil, InputContext{}); !reflect.DeepEqual(got, []session.ScanEvent{{Scan: 0x48, Break: true}}) {
		t.Fatal(got)
	}
	for _, key := range []string{"Escape", "Enter", "Slash"} {
		s = InputState{}
		h := map[string]bool{key: true}
		apply(h, InputContext{Suppress: true})
		if got := apply(h, InputContext{}); len(got) != 0 {
			t.Fatal(key, got)
		}
		apply(nil, InputContext{})
		if got := apply(h, InputContext{}); len(got) != 1 {
			t.Fatal(key, got)
		}
	}
}
func TestMENUClampAndMouse(t *testing.T) {
	x, y := ClampMENU(160, 83)
	if x != 160 || y != 70 {
		t.Fatal(x, y)
	}
	for y := 71; y <= 96; y++ {
		for x := 90; x <= 254; x++ {
			a, b := ClampMENU(x, y)
			if a >= 90 && a <= 254 && b >= 71 && b <= 96 {
				t.Fatal(a, b)
			}
		}
	}
	if x, y := DOSPoint(-1, -1, 3); x != -1 || y != -1 {
		t.Fatal(x, y)
	}
	var s InputState
	in := HostInput{Left: true, MouseX: 160, MouseY: 83}
	c := InputContext{MenuPeriod: true, MenuVisible: true}
	s.Observe(in, c)
	r := s.Build(in, c)
	if r.Update.PointerDown != nil {
		t.Fatal("k1 clicked")
	}
	s.Commit(r)
	in.MouseY = 40
	s.Observe(in, c)
	r = s.Build(in, c)
	if r.Update.PointerDown != nil {
		t.Fatal("held forbidden click turned into k0 click")
	}
	s.Commit(r)
	in.Left = false
	s.Observe(in, c)
	r = s.Build(in, c)
	s.Commit(r)
	in.Right = true
	s.Observe(in, c)
	r = s.Build(in, c)
	if r.Update.PointerDown == nil {
		t.Fatal("right maps to left")
	}
	s.Commit(r)
	in.Left = true
	in.Right = false
	s.Observe(in, c)
	r = s.Build(in, c)
	if r.Update.PointerDown != nil || r.Update.PointerUp != nil {
		t.Fatal("union edge")
	}
}
func TestPaceHandValues(t *testing.T) {
	var p Pace
	r := p.Step(0, true)
	if r.N != 192500 || r.Cap != 385000 || r.Acc != 0 {
		t.Fatal(r)
	}
	r = p.Step(0, false)
	if r.N != 0 || r.Acc != 0 {
		t.Fatal(r)
	}
	// Start with 60Hz history, then a one-second stall. Floor/cap, not fake wall time.
	p = Pace{}
	for i := 0; i < 60; i++ {
		p.Step(time.Second/60, i == 0)
	}
	r = p.Step(time.Second, false)
	if r.N != 385000 || r.Cap != 385000 || !r.Saturated || r.Acc != 385000 {
		t.Fatal(r)
	}
	for _, dt := range []time.Duration{time.Second / 50, time.Duration(16683350), time.Second / 60, time.Second / 144} {
		p = Pace{}
		for i := 0; i < 240; i++ {
			r = p.Step(dt, i == 0)
			if r.Slow {
				t.Fatalf("stable %v slow at %d", dt, i)
			}
		}
	}
	// §7.1's adaptive base also makes stable20Hz non-saturated. A1d's contrary
	// wording is a reported spec issue; this test does not invent a new trigger.
	p = Pace{}
	for i := 0; i < 240; i++ {
		r = p.Step(time.Second/20, i == 0)
	}
	if r.Slow {
		t.Fatal("changed normative §7.1")
	}
}
func synthSpec(name, data string) FileSpec {
	h := sha256.Sum256([]byte(data))
	return FileSpec{Name: name, Size: int64(len(data)), SHA256: hex.EncodeToString(h[:]), Wall: WallTime{1995, 2, 4, 16, 16, 28}}
}
func TestCacheAndSave(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "input")
	if e := os.Mkdir(source, 0755); e != nil {
		t.Fatal(e)
	}
	spec := synthSpec("A.DAT", "abcdef")
	if e := os.WriteFile(filepath.Join(source, "renamed.lower"), []byte("abcdef"), 0444); e != nil {
		t.Fatal(e)
	}
	cache := Cache{Root: filepath.Join(base, "cache"), Specs: []FileSpec{spec}}
	save := filepath.Join(base, "save")
	lock, e := Lock(cache.Root)
	if e != nil {
		t.Fatal(e)
	}
	defer lock.Close()
	if second, e := Lock(cache.Root); e == nil {
		second.Close()
		t.Fatal("second instance locked")
	}
	tree, _, e := cache.Ensure(context.Background(), []string{source}, save, false)
	if e != nil {
		t.Fatal(e)
	}
	if e = cache.Verify(); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(tree, ".DS_Store"), nil, 0600); e != nil {
		t.Fatal(e)
	}
	if e = cache.Verify(); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(tree, "unexpected"), nil, 0600); e != nil {
		t.Fatal(e)
	}
	if e = cache.Verify(); e == nil {
		t.Fatal("extra file accepted")
	}
	os.Remove(filepath.Join(tree, "unexpected"))
	// Corruption with missing source must leave the old tree intact.
	os.Chmod(filepath.Join(tree, "A.DAT"), 0644)
	os.WriteFile(filepath.Join(tree, "A.DAT"), []byte("abcdef!"), 0644)
	if _, _, e = cache.Ensure(context.Background(), []string{filepath.Join(base, "missing")}, save, false); e == nil {
		t.Fatal("missing source accepted")
	}
	if b, _ := os.ReadFile(filepath.Join(tree, "A.DAT")); string(b) != "abcdef!" {
		t.Fatal("old tree changed")
	}
	if _, _, e = cache.Ensure(context.Background(), []string{source}, save, false); e != nil {
		t.Fatal(e)
	}
	os.MkdirAll(save, 0755)
	os.WriteFile(filepath.Join(save, "a.dat"), []byte("abcdef"), 0600)
	if w, e := CheckSaveShadows(save, tree, cache.Specs); e != nil || len(w) != 1 {
		t.Fatal(w, e)
	}
	os.WriteFile(filepath.Join(save, "a.dat"), []byte("different"), 0600)
	if _, e = CheckSaveShadows(save, tree, cache.Specs); e == nil {
		t.Fatal("shadow accepted")
	}
}
func TestNestingSymlink(t *testing.T) {
	r := t.TempDir()
	source := filepath.Join(r, "source")
	os.Mkdir(source, 0755)
	alias := filepath.Join(r, "alias")
	if e := os.Symlink(source, alias); e != nil {
		t.Skip(e)
	}
	if e := CheckIndependentPaths(filepath.Join(alias, "cache"), filepath.Join(r, "save"), []string{source}); e == nil {
		t.Fatal("symlink bypass")
	}
	if e := CheckIndependentPaths(filepath.Join(r, "data/cache"), filepath.Join(r, "data/save"), []string{filepath.Join(r, "data/original")}); e != nil {
		t.Fatal(e)
	}
}
func TestSettings(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	s := DefaultSettings(true)
	s.Scale = 2
	s.Muted = true
	if e := SaveSettings(p, s); e != nil {
		t.Fatal(e)
	}
	got, scale, e := LoadSettings(p, true)
	if e != nil || !scale || !reflect.DeepEqual(got, s) {
		t.Fatal(got, scale, e)
	}
	os.WriteFile(p, []byte("bad"), 0600)
	got, _, e = LoadSettings(p, false)
	if e == nil || !reflect.DeepEqual(got, Settings{Version: 1, Scale: 3, Lang: "zh-TW", Subtitles: true, PauseOnBlur: true}) {
		t.Fatal(got, e)
	}
}
