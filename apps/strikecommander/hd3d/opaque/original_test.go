package opaque

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/model"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// This only consumes existing captures. The parent owns the independent 4x20
// A'/B'/C' gate; this is an implementation smoke test, not that gate.
func TestOpaqueOriginalFirstCapturedGroups(t *testing.T) {
	root, caproot := os.Getenv("SC_MODEL_ROOT"), os.Getenv("SC_OPAQUE_CAPTURE_ROOT")
	if root == "" || caproot == "" {
		t.Skip("original root and existing metadata002 captures required")
	}
	lib, e := realspace.LoadLibrary(root)
	if e != nil {
		t.Fatal(e)
	}
	mat, e := LoadMaterialLibrary(root)
	if e != nil {
		t.Fatal(e)
	}
	exe, e := os.ReadFile(filepath.Join(root, "STRIKE.EXE"))
	if e != nil {
		t.Fatal(e)
	}
	tables, e := hd3d.TablesFromEXE(exe)
	if e != nil {
		t.Fatal(e)
	}
	var plan struct {
		Cases []struct{ Name, StateSHA256 string }
	}
	b, e := os.ReadFile(filepath.Join(caproot, "plan-001.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &plan); e != nil {
		t.Fatal(e)
	}
	for _, c := range plan.Cases {
		t.Run(c.Name, func(t *testing.T) {
			var meta struct {
				Groups []struct {
					Key, FrameSteps, SourceGeneration uint64
					SignatureValid                    bool
					Palette                           [256][3]uint8
					JSONSHA256                        string
				}
			}
			b, e := os.ReadFile(filepath.Join(caproot, c.Name+"-001", "B.json"))
			if e != nil {
				t.Fatal(e)
			}
			if e = json.Unmarshal(b, &meta); e != nil || len(meta.Groups) == 0 {
				t.Fatal("group metadata", e)
			}
			m := meta.Groups[0]
			f, e := os.Open(filepath.Join(caproot, c.Name+"-001", "B-group-001.json.gz"))
			if e != nil {
				t.Fatal(e)
			}
			gz, e := gzip.NewReader(f)
			if e != nil {
				t.Fatal(e)
			}
			raw, e := io.ReadAll(gz)
			gz.Close()
			f.Close()
			if e != nil {
				t.Fatal(e)
			}
			sum := sha256.Sum256(raw)
			if hex.EncodeToString(sum[:]) != m.JSONSHA256 {
				t.Fatal("captured group hash differs")
			}
			var g struct {
				Key, EndSteps uint64
				Cameras       []realspace.Camera
				Objects       []realspace.ObjObs
				Calls         []hd3d.Call
				List          []uint16
				Buf3D         []byte
				Raster        *hd3d.Result
				Elements      []model.Element
				ReadFailed    bool
			}
			if e = json.Unmarshal(raw, &g); e != nil {
				t.Fatal(e)
			}
			scene := model.Input{Key: g.Key, Cameras: g.Cameras, Objects: g.Objects, Raster: hd3d.Group{Key: g.Key, Calls: g.Calls, List: g.List, Buf3D: g.Buf3D}, Original: g.Raster, Elements: g.Elements}
			id := SceneIdentity{EXE: sha256.Sum256(exe), Assets: mat.Key(), AppGeneration: 1, SignatureGeneration: m.SourceGeneration, CUKey: g.Key}
			state, e := hex.DecodeString(c.StateSHA256)
			if e != nil || len(state) != 32 {
				t.Fatal("state identity")
			}
			copy(id.State[:], state)
			id.Group, e = InputDigest(scene)
			if e != nil {
				t.Fatal(e)
			}
			// One isolated caller lifetime is generation1. Recorder generation and the
			// state hash above come from the actual fixed capture, never process atomics.
			capture := CaptureIdentity{Scene: id, Kind: CaptureBuf3D, Steps: g.EndSteps, IndexedSHA: sha256.Sum256(g.Buf3D), OriginalPaletteSHA: PaletteHash(m.Palette)}
			r, e := NewRenderer(lib, mat, tables)
			if e != nil {
				t.Fatal(e)
			}
			snap, e := r.Accept(Input{Identity: id, Capture: &capture, Scene: scene, SignatureValid: m.SignatureValid, ReadFailed: g.ReadFailed})
			if e != nil {
				t.Fatal(e)
			}
			defer snap.Release()
			for _, n := range []int{2, 3} {
				res, e := snap.Render(n)
				if e != nil {
					t.Fatal(e)
				}
				rep := res.Report()
				t.Logf("N%d success=%d H1=%d H2=%d written=%d Needs3b=%t problems=%v", n, rep.Diagnostics.SuccessfulObjects, rep.Diagnostics.H1Objects, rep.Diagnostics.H2Objects, rep.Diagnostics.WrittenSamples, rep.Needs3b, rep.Diagnostics.Problems)
				if rep.Diagnostics.SuccessfulObjects == 0 || rep.Diagnostics.WrittenSamples == 0 || rep.Diagnostics.H2Objects != 0 {
					t.Fatal("original material nonvacuity or H2 failed")
				}
				pal, e := NewPaletteSnapshot(id, capture, m.Palette)
				if e != nil {
					t.Fatal(e)
				}
				bg, e := NewBackgroundComponents(id, capture, n, g.Buf3D, pal)
				if e != nil {
					t.Fatal(e)
				}
				_, e = ResolveRGB(res, bg, pal)
				if rep.Needs3b && e == nil || !rep.Needs3b && e != nil {
					t.Fatal("RGB/deferred contract", e)
				}
			}
		})
	}
}
