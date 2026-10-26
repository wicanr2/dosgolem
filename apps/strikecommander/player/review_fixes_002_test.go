package player

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/session"
)

func TestReviewSaveShadowsEqualFold(t *testing.T) {
	const alias = "ſTRIKE.EXE"
	if !strings.EqualFold(alias, "STRIKE.EXE") || strings.ToLower(alias) == strings.ToLower("STRIKE.EXE") {
		t.Fatal("fixture must distinguish Go EqualFold from ToLower")
	}
	spec := synthSpec("STRIKE.EXE", "oracle")
	for _, tc := range []struct {
		name, data string
		exact      bool
		wantErr    bool
		warnings   int
	}{
		{"different_unicode_alias", "changed", false, true, 0},
		{"identical_unicode_alias", "oracle", false, false, 1},
		{"exact_does_not_hide_bad_alias", "changed", true, true, 0},
		{"all_identical_aliases_warn", "oracle", true, false, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			save := t.TempDir()
			if tc.exact {
				if e := os.WriteFile(filepath.Join(save, spec.Name), []byte("oracle"), 0600); e != nil {
					t.Fatal(e)
				}
			}
			badPath := filepath.Join(save, alias)
			if e := os.WriteFile(badPath, []byte(tc.data), 0600); e != nil {
				t.Fatal(e)
			}
			warnings, e := CheckSaveShadows(save, "", []FileSpec{spec})
			if tc.wantErr {
				if e == nil || !strings.Contains(e.Error(), badPath) {
					t.Fatalf("REVIEW B1: mismatched EqualFold shadow accepted or not identified: %v", e)
				}
			} else if e != nil || len(warnings) != tc.warnings {
				t.Fatalf("REVIEW B1: identical aliases must each warn: %v %v", warnings, e)
			}
			if got, e := os.ReadFile(badPath); e != nil || string(got) != tc.data {
				t.Fatal("shadow preflight modified input", e)
			}
		})
	}
}

func TestReviewFocusLostSuppressesHeldMouse(t *testing.T) {
	for _, point := range [][2]int{{160, 40}, {160, 83}, {10, 190}} {
		for _, buttons := range [][2]bool{{true, false}, {false, true}, {true, true}} {
			for _, alreadySent := range []bool{false, true} {
				var s InputState
				in := HostInput{MouseX: point[0], MouseY: point[1], Left: buttons[0], Right: buttons[1]}
				if alreadySent {
					s.Observe(in, InputContext{})
					s.Commit(s.Build(in, InputContext{}))
				}
				c := InputContext{MenuPeriod: true, MenuVisible: true, ForceFocusLost: true}
				s.Observe(in, c) // n=0: first visibility must already suppress held buttons.
				s.Observe(in, c)
				r := s.Build(in, c)
				if !r.Update.FocusLost || r.Update.PointerDown != nil {
					t.Fatal("first advancing menu turn must only clear the held button")
				}
				s.Commit(r)
				c.ForceFocusLost = false
				// Move into k0 while still held, including switching physical sides.
				in.MouseX, in.MouseY = 160, 40
				for _, held := range [][2]bool{buttons, {true, true}, {false, true}, {true, false}} {
					in.Left, in.Right = held[0], held[1]
					s.Observe(in, c)
					r = s.Build(in, c)
					if r.Update.PointerDown != nil || r.Update.PointerUp != nil {
						t.Fatalf("REVIEW S1: held mouse reissued after FocusLost at %v, buttons=%v sent=%t", point, buttons, alreadySent)
					}
					s.Commit(r)
				}
				in.Left, in.Right = false, false
				s.Observe(in, c)
				s.Commit(s.Build(in, c))
				in.Right = true
				s.Observe(in, c)
				r = s.Build(in, c)
				if r.Update.PointerDown == nil {
					t.Fatal("real release and fresh k0 press were suppressed")
				}
			}
		}
	}
}

func reviewCapsTurn(s *InputState, held, toggle bool, c InputContext, advance bool) session.CapturedUpdate {
	c.Mac = true
	in := HostInput{Held: map[string]bool{"CapsLock": held}, CapsToggle: toggle}
	s.Observe(in, c)
	if !advance {
		return session.CapturedUpdate{}
	}
	r := s.Build(in, c)
	s.Commit(r)
	return r.Update
}

func reviewCapsScans(t *testing.T, got session.CapturedUpdate, want ...session.ScanEvent) {
	t.Helper()
	if !reflect.DeepEqual(got.ScanEvents, want) {
		t.Fatalf("REVIEW S2: Caps make/break mismatch: got %v want %v", got.ScanEvents, want)
	}
}

func TestReviewMacCapsSuppression(t *testing.T) {
	for _, blocked := range []InputContext{{Suppress: true}, {Suppress: true, ForceFocusLost: true}, {MenuVisible: true, ForceFocusLost: true}} {
		for _, makeAlreadySent := range []bool{false, true} {
			var s InputState
			if makeAlreadySent {
				reviewCapsScans(t, reviewCapsTurn(&s, true, true, InputContext{}, true), session.ScanEvent{Scan: 58})
			} else {
				reviewCapsTurn(&s, true, true, InputContext{}, false) // n=0 make pending.
			}
			reviewCapsTurn(&s, true, false, blocked, false) // Suppression with no new toggle.
			resume := InputContext{ForceFocusLost: blocked.ForceFocusLost}
			got := reviewCapsTurn(&s, true, false, resume, true)
			if makeAlreadySent {
				reviewCapsScans(t, got, session.ScanEvent{Scan: 58, Break: true})
			} else {
				reviewCapsScans(t, got)
			}
			if len(s.HeldCodes()) != 0 {
				t.Fatal("guest Caps key remained held after suppression")
			}
			reviewCapsScans(t, reviewCapsTurn(&s, true, false, InputContext{}, true))
			// Even a switch edge on release cannot resurrect the suppressed press.
			reviewCapsScans(t, reviewCapsTurn(&s, false, true, InputContext{}, true))
			reviewCapsScans(t, reviewCapsTurn(&s, true, true, InputContext{}, true), session.ScanEvent{Scan: 58})
			reviewCapsScans(t, reviewCapsTurn(&s, true, false, InputContext{}, true), session.ScanEvent{Scan: 58, Break: true})
			reviewCapsScans(t, reviewCapsTurn(&s, true, false, InputContext{}, true))
		}
	}
}

func TestReviewMacCapsMakeBreakAndCommit(t *testing.T) {
	var s InputState
	in := HostInput{Held: map[string]bool{"CapsLock": true}, CapsToggle: true}
	c := InputContext{Mac: true}
	s.Observe(in, c)
	// Build alone, including an n=0 caller declining to commit, cannot consume it.
	first, again := s.Build(in, c), s.Build(in, c)
	reviewCapsScans(t, first.Update, session.ScanEvent{Scan: 58})
	reviewCapsScans(t, again.Update, session.ScanEvent{Scan: 58})
	s.Commit(first)
	// A new physical switch while break is pending must release first.
	reviewCapsScans(t, reviewCapsTurn(&s, false, true, InputContext{}, true), session.ScanEvent{Scan: 58, Break: true})
	reviewCapsScans(t, reviewCapsTurn(&s, false, false, InputContext{}, true), session.ScanEvent{Scan: 58})
	reviewCapsScans(t, reviewCapsTurn(&s, false, false, InputContext{}, true), session.ScanEvent{Scan: 58, Break: true})
	reviewCapsScans(t, reviewCapsTurn(&s, false, false, InputContext{}, true))
	// If suppression intervenes after that second edge, keep only the owed break.
	reviewCapsScans(t, reviewCapsTurn(&s, true, true, InputContext{}, true), session.ScanEvent{Scan: 58})
	reviewCapsTurn(&s, false, true, InputContext{}, false)
	reviewCapsTurn(&s, true, false, InputContext{Suppress: true}, false)
	reviewCapsScans(t, reviewCapsTurn(&s, true, false, InputContext{}, true), session.ScanEvent{Scan: 58, Break: true})
	reviewCapsScans(t, reviewCapsTurn(&s, true, false, InputContext{}, true))
}

func TestReviewMacCapsBreakDuringSuppression(t *testing.T) {
	var s InputState
	reviewCapsScans(t, reviewCapsTurn(&s, true, true, InputContext{}, true), session.ScanEvent{Scan: 58})
	reviewCapsScans(t, reviewCapsTurn(&s, true, false, InputContext{Suppress: true}, true), session.ScanEvent{Scan: 58, Break: true})
	reviewCapsScans(t, reviewCapsTurn(&s, true, false, InputContext{}, true))
	if len(s.HeldCodes()) != 0 {
		t.Fatal("guest Caps state remained set")
	}
}

func TestReviewMacCapsToggleWithoutHeldState(t *testing.T) {
	// macOS may report only the switch edge, without a held-key interval.
	for _, blocked := range []InputContext{{Suppress: true}, {Suppress: true, ForceFocusLost: true}, {MenuVisible: true, ForceFocusLost: true}} {
		var s InputState
		reviewCapsTurn(&s, false, true, InputContext{}, false)
		reviewCapsTurn(&s, false, false, blocked, false)
		reviewCapsScans(t, reviewCapsTurn(&s, false, false, InputContext{}, true))
		reviewCapsScans(t, reviewCapsTurn(&s, false, true, InputContext{}, true), session.ScanEvent{Scan: 58})
		reviewCapsTurn(&s, false, false, blocked, false)
		reviewCapsScans(t, reviewCapsTurn(&s, false, false, InputContext{}, true), session.ScanEvent{Scan: 58, Break: true})
		reviewCapsScans(t, reviewCapsTurn(&s, false, false, InputContext{}, true))
	}
}

func TestReviewPhysicalAncestorAlias(t *testing.T) {
	base := t.TempDir()
	source, alias := filepath.Join(base, "original"), filepath.Join(base, "ORIGINAL")
	if e := os.Mkdir(source, 0755); e != nil {
		t.Fatal(e)
	}
	cache, save := filepath.Join(alias, "cache"), filepath.Join(base, "save")
	if _, e := os.Stat(cache); !os.IsNotExist(e) {
		t.Fatal("fixture requires missing cache", e)
	}
	aliasHits := 0
	stat := func(p string) (os.FileInfo, error) {
		if p == alias || strings.HasPrefix(p, alias+string(filepath.Separator)) {
			if p == alias {
				aliasHits++
			}
			p = source + strings.TrimPrefix(p, alias)
		}
		return os.Stat(p) // Real FileInfo with real device/inode, no synthetic Sys.
	}
	if e := checkIndependentPathsWithStat(cache, save, []string{source}, stat); e == nil {
		t.Fatal("REVIEW S3: source exists/cache missing physical alias nesting accepted")
	}
	if aliasHits == 0 {
		t.Fatal("physical ancestor injection was not exercised")
	}
	entries, e := os.ReadDir(source)
	if e != nil || len(entries) != 0 {
		t.Fatal("preflight wrote into source", entries, e)
	}
	if _, e := os.Stat(save); !os.IsNotExist(e) {
		t.Fatal("preflight created output", e)
	}
}

func TestReviewLinuxCaseSensitivePaths(t *testing.T) {
	base := t.TempDir()
	source, distinct := filepath.Join(base, "original"), filepath.Join(base, "ORIGINAL")
	for _, p := range []string{source, distinct} {
		if e := os.Mkdir(p, 0755); e != nil {
			t.Fatal(e)
		}
	}
	a, e := os.Stat(source)
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.Stat(distinct)
	if e != nil || os.SameFile(a, b) {
		t.Fatal("fixture must be a real case-sensitive filesystem", e)
	}
	if e := CheckIndependentPaths(filepath.Join(distinct, "cache"), filepath.Join(base, "save"), []string{source}); e != nil {
		t.Fatal("distinct case-sensitive directories incorrectly folded", e)
	}
}

func TestReviewCacheRenameRollback(t *testing.T) {
	for _, failAt := range []int{2, 3} {
		t.Run(string(rune('0'+failAt)), func(t *testing.T) {
			base := t.TempDir()
			sources := []string{filepath.Join(base, "source-a"), filepath.Join(base, "source-b")}
			for _, source := range sources {
				if e := os.Mkdir(source, 0755); e != nil {
					t.Fatal(e)
				}
				if e := os.WriteFile(filepath.Join(source, "renamed"), []byte("oracle"), 0444); e != nil {
					t.Fatal(e)
				}
			}
			cache := Cache{Root: filepath.Join(base, "cache"), Specs: []FileSpec{synthSpec("A.DAT", "oracle")}}
			save := filepath.Join(base, "save")
			if _, _, e := cache.Ensure(context.Background(), sources[:1], save, false); e != nil {
				t.Fatal(e)
			}
			oldTree, e := os.Stat(cache.tree())
			if e != nil {
				t.Fatal(e)
			}
			manifestPath := filepath.Join(cache.Root, "tree-v1.manifest")
			oldManifest, e := os.ReadFile(manifestPath)
			if e != nil {
				t.Fatal(e)
			}
			calls := 0
			injected := errors.New("planned rename failure")
			cache.Rename = func(a, b string) error {
				calls++
				if calls == failAt {
					return injected
				}
				return os.Rename(a, b)
			}
			if _, _, e := cache.Ensure(context.Background(), sources[1:], save, true); !errors.Is(e, injected) {
				t.Fatal("planned rename failure was not reached", e)
			}
			if calls <= failAt {
				t.Fatal("rollback rename was not exercised", calls)
			}
			cache.Rename = nil
			if e := cache.Recover(); e != nil {
				t.Fatal(e)
			}
			if e := cache.Verify(); e != nil {
				t.Fatal("old tree not valid after recovery", e)
			}
			gotTree, e := os.Stat(cache.tree())
			if e != nil || !os.SameFile(oldTree, gotTree) {
				t.Fatal("rollback did not restore original tree identity", e)
			}
			gotManifest, e := os.ReadFile(manifestPath)
			if e != nil || string(gotManifest) != string(oldManifest) {
				t.Fatal("rollback changed original manifest", e)
			}
			entries, e := os.ReadDir(cache.Root)
			if e != nil {
				t.Fatal(e)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), "tree-v1.old-") || strings.HasPrefix(entry.Name(), "tree-v1.importing-") {
					t.Fatal("recovery left a temporary tree", entry.Name())
				}
			}
		})
	}
}
