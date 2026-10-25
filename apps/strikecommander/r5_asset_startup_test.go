package strikecommander

import (
	"github.com/wicanr2/dosgolem/apps/strikecommander/r5asset"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"unsafe"
)

func r5StartupFixtureRoots(t *testing.T) (string, string) {
	t.Helper()
	modelRoot, assetRoot := os.Getenv("SC_MODEL_ROOT"), os.Getenv("SC_R5_ASSET_ROOT")
	// Same SC_MODEL_ROOT name and unset/empty opt-in convention as existing
	// TestFrontendConfigureHDRealAssetsQuotaAndClose. No alias or default path.
	for _, r := range []struct {
		name, path string
		files      []string
	}{
		{"SC_MODEL_ROOT", modelRoot, []string{"STRIKE.EXE"}},
		{"SC_R5_ASSET_ROOT", assetRoot, []string{"f16/model/model.gltf", "f16/model/model.bin", "f16/model/albedo.png", "f16/model/normal.png", "f16/model/metal_rough.png", "mig29/model/model.gltf", "mig29/model/model.bin", "mig29/model/albedo.png", "mig29/model/normal.png", "mig29/model/metal_rough.png"}},
	} {
		if r.path == "" {
			continue
		}
		st, e := os.Stat(r.path)
		if e != nil || !st.IsDir() {
			t.Fatalf("configured %s=%q is not a readable fixture directory: %v", r.name, r.path, e)
		}
		for _, f := range r.files {
			name := filepath.Join(r.path, f)
			st, e := os.Stat(name)
			if e != nil || !st.Mode().IsRegular() {
				t.Fatalf("configured %s fixture file %q is missing or not regular: %v", r.name, name, e)
			}
		}
	}
	if modelRoot == "" || assetRoot == "" {
		t.Skip("explicit read-only SC_MODEL_ROOT and SC_R5_ASSET_ROOT fixtures required")
	}
	return modelRoot, assetRoot
}
func TestR5FactoryConfiguredOwnerStartupAndClose(t *testing.T) {
	modelRoot, assetRoot := r5StartupFixtureRoots(t)
	cfg := runtimeTestConfig(t)
	cfg.HD = &FrontendHDConfig{GameRoot: modelRoot, StateSHA: [32]byte{1}, AppGeneration: 1}
	s, e := newR5Startup(cfg, assetRoot, r5asset.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	o := s.runtime.hd.owner // Keep actual owner for final accounting, never nil it.
	t.Logf("actual factory owned=%d asset=%+v wrapper=%d", o.OwnedBytes(), s.assets.Stats(), unsafe.Sizeof(r5Startup{}))
	if s.runtime.so.Models.ShadeSourceOwner() != o || s.runtime.hd.pool != s.runtime.so.Models.bounded.pool || s.runtime.hd.renderer == nil || s.runtime.hd.prepared == nil || s.runtime.so.steps != 0 {
		t.Fatal("factory premise")
	}
	if unsafe.Sizeof(r5Startup{}) > 8192 {
		t.Fatal("unbudgeted wrapper")
	}
	reader, e := s.assets.Acquire()
	if e != nil {
		t.Fatal(e)
	}
	defer s.assets.Release(reader)
	id, e := s.assets.Identity(reader)
	if e != nil {
		t.Fatal(e)
	}
	before := o.OwnedBytes()
	if e = s.assets.Adopt(o, id.SHA); e == nil || before != o.OwnedBytes() {
		t.Fatal("unique publication or second accounting")
	}
	done := make(chan struct{})
	go func() { s.Close(); close(done) }()
	for !s.assets.Stats().Closing {
		runtime.Gosched()
	}
	if o.OwnedBytes() != before {
		t.Fatal("factory closed shared owner before final asset reader")
	}
	if _, e = s.assets.Vertex(reader, r5asset.F16, 0, 0); e != nil {
		t.Fatal("held factory reader", e)
	}
	s.assets.Release(reader)
	<-done
	s.Close()
	if o.OwnedBytes() != 0 {
		t.Fatal("factory Close leaked original owner", o.OwnedBytes())
	}
}
