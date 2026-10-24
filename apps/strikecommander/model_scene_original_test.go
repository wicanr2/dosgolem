package strikecommander

// Optional, local-only 013 phase-2 acceptance against frozen original captures.
// No game is executed. Original data, trace, and captured groups remain read-only.
import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/geometry"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/model"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
)

type modelSceneFiles struct {
	Root, Reports, Evidence, Near string
	SHA256                        map[string]string
}

func modelSceneRead(t testing.TB, f *modelSceneFiles, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	f.SHA256[name] = fmt.Sprintf("%x", sha256.Sum256(b))
	return b
}

func modelSceneSetup(t testing.TB) (*modelSceneFiles, *realspace.Library, hd3d.Tables) {
	t.Helper()
	root := os.Getenv("SC_MODEL_ROOT")
	if root == "" {
		t.Skip("optional original scene acceptance: set SC_MODEL_ROOT")
	}
	env := func(k, fallback string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return fallback
	}
	f := &modelSceneFiles{Root: root, Reports: env("SC_MODEL_SCENE_REPORTS", "/project/workplace/m6model/rec"), Evidence: env("SC_MODEL_SCENE_EVIDENCE", "/project/workplace/m6model/p13-3"), Near: env("SC_MODEL_NEAR_TRACE", "/project/workplace/m6model/p13-4/n2-dive"), SHA256: map[string]string{}}
	for _, name := range []string{"STRIKE.EXE", "BIGTRE.TRE", "LILTRE.TRE", "TOBIGTRE.TRE"} {
		if _, err := os.Stat(filepath.Join(root, name)); os.IsNotExist(err) {
			t.Skipf("optional fixture missing: %s", name)
		}
	}
	exe := modelSceneRead(t, f, filepath.Join(root, "STRIKE.EXE"))
	if fmt.Sprintf("%x", sha256.Sum256(exe)) != "883270cd0038fe571376202233b8abf5efb77889e27f4f0caf97dfab7e529cb5" {
		t.Fatal("original EXE SHA mismatch")
	}
	for _, name := range []string{"BIGTRE.TRE", "LILTRE.TRE", "TOBIGTRE.TRE"} {
		modelSceneRead(t, f, filepath.Join(root, name))
	}
	lib, err := realspace.LoadLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	tables, err := hd3d.TablesFromEXE(exe)
	if err != nil {
		t.Fatal(err)
	}
	return f, lib, tables
}

func modelSceneJSON(t testing.TB, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func modelSceneWrite(t testing.TB, name string, b []byte) {
	t.Helper()
	f, e := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if e != nil {
		t.Fatal(e)
	}
	_, e = f.Write(b)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
}
func modelSceneOut(t testing.TB, env, prefix string) string {
	t.Helper()
	p := os.Getenv(env)
	if p == "" {
		t.Skip("set " + env + " to a new local receipt directory")
	}
	if !strings.HasPrefix(filepath.Base(p), prefix) {
		t.Fatal("receipt basename must start with " + prefix)
	}
	if e := os.Mkdir(p, 0755); e != nil {
		t.Fatal("receipt must not exist: ", e)
	}
	return p
}

func modelSceneLoad(t testing.TB, f *modelSceneFiles, report, group int) ModelGroup {
	t.Helper()
	name := filepath.Join(f.Reports, fmt.Sprintf("report-%03d/B/group-%03d.json.gz", report, group))
	raw := modelSceneRead(t, f, name)
	z, e := gzip.NewReader(bytes.NewReader(raw))
	if e != nil {
		t.Fatal(e)
	}
	var g ModelGroup
	e = json.NewDecoder(z).Decode(&g)
	if e != nil {
		t.Fatal(e)
	}
	if e = z.Close(); e != nil {
		t.Fatal(e)
	}
	return g
}

type modelSceneHashes struct{ Owner, Depth, Wire, Representative string }

func modelSceneHashFrame(t testing.TB, f geometry.Frame) modelSceneHashes {
	t.Helper()
	if f.Width != 320*f.Scale || f.Height != 200*f.Scale || len(f.Owner) != f.Width*f.Height || len(f.DepthQ48) != len(f.Owner) || f.Wire == nil {
		t.Fatal("invalid frame dimensions")
	}
	o, d := sha256.New(), sha256.New()
	var b [10]byte
	var q [8]byte
	for i, r := range f.Owner {
		if f.DepthQ48[i] == 0 {
			r = geometry.FaceRef{Object: math.MaxUint32, Face: math.MaxUint32}
		}
		binary.LittleEndian.PutUint32(b[:4], r.Object)
		binary.LittleEndian.PutUint32(b[4:8], r.Face)
		b[8] = 0
		b[9] = 0
		if r.Clipped {
			b[8] = 1
		}
		if r.Clamped {
			b[9] = 1
		}
		o.Write(b[:])
		binary.LittleEndian.PutUint64(q[:], f.DepthQ48[i])
		d.Write(q[:])
	}
	rep := sha256.New()
	var pair [4]byte
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			p := (y*f.Scale+(f.Scale-1)/2)*f.Width + x*f.Scale + (f.Scale-1)/2
			r := f.Owner[p]
			if f.DepthQ48[p] == 0 {
				r.Object = 65535
				r.Face = 65535
			}
			if r.Object > 65535 || r.Face > 65535 {
				t.Fatal("representative owner exceeds oracle uint16 format")
			}
			binary.LittleEndian.PutUint16(pair[:2], uint16(r.Object))
			binary.LittleEndian.PutUint16(pair[2:], uint16(r.Face))
			rep.Write(pair[:])
		}
	}
	return modelSceneHashes{fmt.Sprintf("%x", o.Sum(nil)), fmt.Sprintf("%x", d.Sum(nil)), fmt.Sprintf("%x", sha256.Sum256(f.Wire.Pix)), fmt.Sprintf("%x", rep.Sum(nil))}
}

type modelScenePalette struct {
	Group                                                                int
	Key, GroupSteps, PromotionFrameStep, SnapshotSteps, PaletteFrameStep uint64
	Palette                                                              [256][3]uint8
}
type modelSceneRow struct {
	Case        string
	Group, N    int
	Key, Steps  uint64
	Passed      bool
	B           model.BMetrics
	C           model.CMetrics
	Objects     []model.ObjectReport
	Diagnostics model.Diagnostics
	Stats       geometry.Stats
	Hashes      modelSceneHashes
	Palette     *modelScenePalette `json:",omitempty"`
	PNG         string             `json:",omitempty"`
}

func modelScenePNG(t testing.TB, name string, frame geometry.Frame, background []byte) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, frame.Width, frame.Height))
	if background != nil {
		if len(background) != len(img.Pix) {
			t.Fatal("background shape")
		}
		copy(img.Pix, background)
	} else {
		// Missing original background is an opaque black diagnostic canvas.
		// A transparent zero RGBA image would depend on the viewer backdrop.
		for i := 3; i < len(img.Pix); i += 4 {
			img.Pix[i] = 255
		}
	}
	draw.Draw(img, img.Bounds(), frame.Wire, frame.Wire.Bounds().Min, draw.Over)
	var b bytes.Buffer
	if e := png.Encode(&b, img); e != nil {
		t.Fatal(e)
	}
	modelSceneWrite(t, name, b.Bytes())
}

func TestModelSceneOriginal(t *testing.T) {
	f, lib, tables := modelSceneSetup(t)
	out := modelSceneOut(t, "SC_MODEL_SCENE_OUT", "original-")
	// Decode explicit JSON names; this is the independent original-TRE Python statistic.
	var baselines map[string]struct {
		Counts map[string]uint64 `json:"counts"`
		X      map[int64]uint64  `json:"ex_distribution"`
		Y      map[int64]uint64  `json:"ey_distribution"`
	}
	if e := json.Unmarshal(modelSceneRead(t, f, filepath.Join(f.Evidence, "b-summary-001.json")), &baselines); e != nil {
		t.Fatal(e)
	}
	var oracle []struct {
		Case, Group  string
		N            int
		Q48MapSHA256 string
	}
	if e := json.Unmarshal(modelSceneRead(t, f, filepath.Join(f.Evidence, "q48-groups-005.json")), &oracle); e != nil {
		t.Fatal(e)
	}
	expectedMap := map[string]string{}
	for _, r := range oracle {
		expectedMap[fmt.Sprintf("%s/%s/%d", r.Case, r.Group, r.N)] = r.Q48MapSHA256
	}
	rows := []modelSceneRow{}
	comparedMaps := 0
	for ri, caseName := range []string{"s760", "s800f2", "n1", "n2"} {
		report := 14 + ri
		var palettes []modelScenePalette
		if e := json.Unmarshal(modelSceneRead(t, f, filepath.Join(f.Reports, fmt.Sprintf("report-%03d/B/palette-groups.json", report))), &palettes); e != nil {
			t.Fatal(e)
		}
		if len(palettes) != 30 {
			t.Fatal("expected 30 current palettes")
		}
		totals := map[int]*model.BMetrics{2: {ErrorsX: map[int64]uint64{}, ErrorsY: map[int64]uint64{}}, 3: {ErrorsX: map[int64]uint64{}, ErrorsY: map[int64]uint64{}}}
		for group := 1; group <= 30; group++ {
			g := modelSceneLoad(t, f, report, group)
			before := modelSceneJSON(t, &g)
			for _, n := range []int{2, 3} {
				r, e := g.RenderModelDiagnostics(lib, tables, n)
				if e != nil {
					t.Fatalf("%s/%d/N%d: %v", caseName, group, n, e)
				}
				// The 99% B criterion applies to the complete state sample, not
				// each group. Preserve per-group Passed, including legitimate N1
				// groups with ey=1 at more than 1% of their 150 vertices.
				if !r.Passed || !r.B.Applicable || r.B.HardFailureX != 0 || r.B.HardFailureY != 0 || !r.C.Available || !r.C.Passed {
					t.Fatalf("%s/%d/N%d failed: B=%+v C=%+v problems=%+v", caseName, group, n, r.B, r.C, r.Diagnostics)
				}
				if r.C.C2OutsideBand != 0 || r.C.Unlinked != 0 || r.C.LinkedOutsideSet != 0 {
					t.Fatal("C hard gates")
				}
				for _, o := range r.Objects {
					if o.Status != realspace.Passed {
						t.Fatalf("fresh original-file object check: %+v", o)
					}
				}
				h := modelSceneHashFrame(t, r.Frame)
				if group == 1 && n == 2 {
					var tainted ModelGroup
					if e := json.Unmarshal(before, &tainted); e != nil {
						t.Fatal(e)
					}
					for i := range tainted.Objects {
						tainted.Objects[i].Verdict = realspace.ObjVerdict{Status: realspace.Passed, Selected: &realspace.ModelEntry{TRE: "fabricated capture metadata", Verts: [][3]int32{{math.MaxInt32, math.MinInt32, 0}}}}
					}
					fresh, e := tainted.RenderModelDiagnostics(lib, tables, n)
					if e != nil || !fresh.Passed || h != modelSceneHashFrame(t, fresh.Frame) || !reflect.DeepEqual(r.Objects, fresh.Objects) {
						t.Fatal("renderer trusted captured Selected/status metadata instead of original-file self-check: ", e)
					}
				}
				again, e := g.RenderModelDiagnostics(lib, tables, n)
				if e != nil {
					t.Fatal(e)
				}
				if h != modelSceneHashFrame(t, again.Frame) || h != modelSceneHashFrame(t, r.Frame) || !reflect.DeepEqual(r.B, again.B) || !reflect.DeepEqual(r.C, again.C) || !reflect.DeepEqual(r.Objects, again.Objects) || !reflect.DeepEqual(r.Diagnostics, again.Diagnostics) {
					t.Fatal("nondeterministic output or later Render mutates retained frame")
				}
				if !bytes.Equal(before, modelSceneJSON(t, &g)) {
					t.Fatal("Render modified captured input")
				}
				tot := totals[n]
				tot.Eligible += r.B.Eligible
				tot.ZeroX += r.B.ZeroX
				tot.ZeroY += r.B.ZeroY
				for k, v := range r.B.ErrorsX {
					tot.ErrorsX[k] += v
				}
				for k, v := range r.B.ErrorsY {
					tot.ErrorsY[k] += v
				}
				key := fmt.Sprintf("%s/group-%03d.json.gz/%d", caseName, group, n)
				if want, ok := expectedMap[key]; ok {
					if h.Representative != want {
						t.Fatalf("independent Q48 map %s: got %s want %s", key, h.Representative, want)
					}
					comparedMaps++
				}
				row := modelSceneRow{Case: caseName, Group: group, N: n, Key: g.Key, Steps: g.Steps, Passed: r.Passed, B: r.B, C: r.C, Objects: r.Objects, Diagnostics: r.Diagnostics, Stats: r.Frame.Stats, Hashes: h}
				if group == 1 || group == 28 {
					p := palettes[group-1]
					if p.Group != group || p.Key != g.Key || p.GroupSteps != g.Steps || p.PaletteFrameStep != p.PromotionFrameStep || p.SnapshotSteps < p.PaletteFrameStep {
						t.Fatal("current palette provenance mismatch")
					}
					row.PNG = fmt.Sprintf("%s-group-%03d-%dx-wire.png", caseName, group, n)
					row.Palette = &p
					modelScenePNG(t, filepath.Join(out, row.PNG), r.Frame, ScaleIndexedRGBA(g.Buf3D, p.Palette, n))
				}
				rows = append(rows, row)
			}
		}
		want := baselines[caseName]
		for n, tot := range totals {
			if tot.Eligible != want.Counts["eligible_unique"] || tot.ZeroX != want.Counts["zero_x"] || tot.ZeroY != want.Counts["zero_y"] || !reflect.DeepEqual(tot.ErrorsX, want.X) || !reflect.DeepEqual(tot.ErrorsY, want.Y) {
				t.Fatalf("independent B %s/N%d got %+v want %+v", caseName, n, tot, want)
			}
		}
		t.Logf("%s: 30 groups x N2/N3, original-file objects, B/C, two renders and immutable input PASS", caseName)
	}
	if comparedMaps != 80 {
		t.Fatalf("independent map coverage %d want 80", comparedMaps)
	}
	modelSceneWrite(t, filepath.Join(out, "groups.json"), modelSceneJSON(t, rows))
	modelSceneWrite(t, filepath.Join(out, "receipt.json"), modelSceneJSON(t, map[string]any{"Status": "PASS", "Groups": 120, "Renders": 484, "PoisonedCaptureMetadataChecks": 4, "IndependentQ48Maps": comparedMaps, "Go": runtime.Version(), "GOARCH": runtime.GOARCH, "InputsSHA256": f.SHA256, "OwnerCanonical": "raster order; uint32le object, uint32le face, byte clipped, byte clamped; unowned ffffffff/ffffffff/0/0", "DepthCanonical": "raster order uint64le Q48; zero means unowned", "WireCanonical": "image.RGBA Pix row order", "RepresentativeCanonical": "64000 pairs uint16le object/face; ffff/ffff means unowned, Python q48-005 format", "Scope": "offline frozen original captures, no game execution or formal HD composition"}))
}

func modelSceneHex(t testing.TB, s string, n int) []byte {
	t.Helper()
	b, e := hex.DecodeString(s)
	if e != nil || len(b) != n {
		t.Fatalf("hex len=%d want=%d err=%v", len(b), n, e)
	}
	return b
}
func modelSceneInt(t testing.TB, s string, base int) int64 {
	t.Helper()
	v, e := strconv.ParseInt(s, base, 64)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func modelScenePtr(t testing.TB, s string) realspace.FarPtr {
	t.Helper()
	p := strings.Split(s, ":")
	if len(p) != 2 {
		t.Fatal("invalid far pointer ", s)
	}
	return realspace.FarPtr{Seg: uint16(modelSceneInt(t, p[0], 16)), Off: uint16(modelSceneInt(t, p[1], 16))}
}

// Legacy C1 records do not save CamObj. Only frames with one O1 are admitted;
// camera identity is explicitly supplemented from that O1, not independently proved.
func modelSceneNearGroups(t testing.TB, f *modelSceneFiles) map[int]*ModelGroup {
	t.Helper()
	groups := map[int]*ModelGroup{}
	objects := map[int64]*realspace.ObjObs{}
	for _, name := range []string{"cam.tsv", "objs.tsv", "faces.tsv"} {
		s := bufio.NewScanner(bytes.NewReader(modelSceneRead(t, f, filepath.Join(f.Near, name))))
		s.Buffer(make([]byte, 4096), 1<<20)
		for s.Scan() {
			v := strings.Split(s.Text(), "\t")
			fr := int(modelSceneInt(t, v[0], 10))
			if fr < 103 || fr > 107 {
				continue
			}
			g := groups[fr]
			if g == nil {
				g = &ModelGroup{}
				groups[fr] = g
			}
			switch name {
			case "cam.tsv":
				if v[2] != "C1" {
					continue
				}
				if len(g.Cameras) != 0 {
					t.Fatal("multiple legacy C1")
				}
				cam := realspace.Camera{Steps: uint64(modelSceneInt(t, v[1], 10)), DS: uint16(modelSceneInt(t, v[3], 16))}
				copy(cam.Raw[:], modelSceneHex(t, v[5], len(cam.Raw)))
				g.Cameras = append(g.Cameras, cam)
			case "objs.tsv":
				if len(g.Objects) != 0 {
					t.Fatal("multiple legacy O1; CamObj supplement would be ambiguous")
				}
				kv := map[string]string{}
				for _, s := range v[3:] {
					p := strings.SplitN(s, "=", 2)
					if len(p) == 2 {
						kv[p[0]] = p[1]
					}
				}
				o := realspace.ObjObs{Steps: uint64(modelSceneInt(t, v[1], 10)), CamObj: uint16(modelSceneInt(t, kv["cam"], 16)), CamIdx: 0, Rec: modelScenePtr(t, kv["rec"]), RecVT: uint16(modelSceneInt(t, kv["vt"], 16)), Model: modelScenePtr(t, kv["model"]), LOD: uint8(modelSceneInt(t, kv["lod"], 10)), NFaces: int16(modelSceneInt(t, kv["nf"], 10)), FaceBase: modelScenePtr(t, kv["base"]), Flag17BC: uint8(modelSceneInt(t, kv["f17"], 10)), Near: int32(modelSceneInt(t, kv["near"], 10)), Far: int32(modelSceneInt(t, kv["far"], 10))}
				if kv["flags"] != "-" {
					t.Fatal("legacy object has fault flags: ", kv["flags"])
				}
				copy(o.RecRaw[:], modelSceneHex(t, kv["recraw"], len(o.RecRaw)))
				copy(o.ModelHead[:], modelSceneHex(t, kv["modelhead"], len(o.ModelHead)))
				copy(o.VertSHA[:], modelSceneHex(t, kv["vsha"], 32))
				nv := int(modelSceneInt(t, kv["nv"], 10))
				o.VertXYZ = modelSceneHex(t, kv["vxyz"], nv*12)
				o.VertRT = make([][3]int32, nv)
				for i := range o.VertRT {
					for j := 0; j < 3; j++ {
						o.VertRT[i][j] = int32(binary.LittleEndian.Uint32(o.VertXYZ[i*12+j*4:]))
					}
				}
				list := modelSceneHex(t, kv["listraw"], int(o.NFaces)*2)
				for i := 0; i < len(list); i += 2 {
					o.FaceList = append(o.FaceList, binary.LittleEndian.Uint16(list[i:]))
				}
				g.Objects = append(g.Objects, o)
				g.Cameras[0].CamObj = o.CamObj
				g.Steps = o.Steps
				g.Key = o.Steps
				objects[modelSceneInt(t, v[2], 10)] = &g.Objects[0]
			case "faces.tsv":
				o := objects[modelSceneInt(t, v[1], 10)]
				if o == nil {
					t.Fatal("face without object")
				}
				if v[9] != "ok" {
					t.Fatal("legacy face did not return")
				}
				face := realspace.FaceObs{SI: uint16(modelSceneInt(t, v[2], 10)), Face: modelScenePtr(t, v[5]), Method: modelScenePtr(t, strings.TrimPrefix(v[7], "m18=")), FaceVT: uint16(modelSceneInt(t, strings.TrimPrefix(v[6], "vt="), 16)), AL: uint8(modelSceneInt(t, v[11], 10)), Returned: true}
				idx := strings.Split(v[8], ",")
				pr := strings.Split(v[13], ",")
				if len(idx) != 3 || len(pr) != 3 {
					t.Fatal("legacy triple shape")
				}
				for i := 0; i < 3; i++ {
					face.VIdx[i] = int16(modelSceneInt(t, idx[i], 10))
					copy(face.Proj[i][:], modelSceneHex(t, pr[i], 18))
				}
				if v[12] != "-" {
					face.Elem = modelScenePtr(t, v[12])
				}
				copy(face.Geom[:], modelSceneHex(t, v[14], len(face.Geom)))
				o.Faces = append(o.Faces, face)
			}
		}
		if e := s.Err(); e != nil {
			t.Fatal(e)
		}
	}
	if len(groups) != 5 {
		t.Fatal("missing near frames")
	}
	return groups
}

func TestModelSceneNear(t *testing.T) {
	f, lib, tables := modelSceneSetup(t)
	out := modelSceneOut(t, "SC_MODEL_NEAR_OUT", "near-")
	groups := modelSceneNearGroups(t, f)
	rows := []modelSceneRow{}
	for fr := 103; fr <= 107; fr++ {
		g := groups[fr]
		if g == nil || len(g.Objects) != 1 || len(g.Cameras) != 1 {
			t.Fatal("legacy group shape")
		}
		before := modelSceneJSON(t, g)
		v := realspace.CheckObject(lib, g.Cameras, &g.Objects[0])
		if v.Status != realspace.Passed || v.XYZCompared != 338 || v.ScreenCompared != 159 {
			t.Fatalf("frame %d original-file self-check: %+v", fr, v)
		}
		for _, n := range []int{2, 3} {
			r, e := g.RenderModelDiagnostics(lib, tables, n)
			if e != nil {
				t.Fatal(e)
			}
			if r.Passed || r.C.Available || len(r.Objects) != 1 || r.Objects[0].Status != realspace.Passed || r.Objects[0].ClippedFaces != 81 || r.Objects[0].Triangles != 54 {
				t.Fatalf("near frame %d: C must be unavailable, geometry valid: %+v %+v", fr, r.C, r.Objects)
			}
			modelSceneNearOracle(t, g, v.Selected, n, r.Frame)
			h := modelSceneHashFrame(t, r.Frame)
			again, e := g.RenderModelDiagnostics(lib, tables, n)
			if e != nil || h != modelSceneHashFrame(t, again.Frame) || h != modelSceneHashFrame(t, r.Frame) {
				t.Fatal("near nondeterminism/alias: ", e)
			}
			if !bytes.Equal(before, modelSceneJSON(t, g)) {
				t.Fatal("near input mutated")
			}
			name := fmt.Sprintf("frame-%03d-%dx-black-wire.png", fr, n)
			modelScenePNG(t, filepath.Join(out, name), r.Frame, nil)
			rows = append(rows, modelSceneRow{Case: "n2-dive", Group: fr, N: n, Key: g.Key, Steps: g.Steps, B: r.B, C: r.C, Objects: r.Objects, Diagnostics: r.Diagnostics, Stats: r.Frame.Stats, Hashes: h, PNG: name})
		}
		t.Logf("near frame %d: XYZ338/screens159, N2/N3 formal Pa-first BigInt oracle and deterministic black wire PASS", fr)
	}
	modelSceneWrite(t, filepath.Join(out, "groups.json"), modelSceneJSON(t, rows))
	modelSceneWrite(t, filepath.Join(out, "receipt.json"), modelSceneJSON(t, map[string]any{"Status": "PASS geometry; C unavailable by design", "Frames": 5, "Go": runtime.Version(), "InputsSHA256": f.SHA256, "LegacyLimit": "C1 CamObj supplemented from same frame's sole O1; not independent camera-identity evidence. Old trace has no Buf3D/Writer/Elements, no C or original-background parity. No old Python fan/pixel oracle used."}))
}

// Independent formal Pa-first clipping and exact original-Q8 three-edge oracle.
// It deliberately does not call geometry.ProjectFace/ProjectPoint/Target.
func modelSceneNearOracle(t testing.TB, g *ModelGroup, entry *realspace.ModelEntry, n int, got geometry.Frame) {
	t.Helper()
	o := g.Objects[0]
	cam := g.Cameras[0].Raw
	cx := int16(binary.LittleEndian.Uint16(cam[8:]))
	cy := int16(binary.LittleEndian.Uint16(cam[10:]))
	fx := int32(binary.LittleEndian.Uint32(cam[12:]))
	fy := int32(binary.LittleEndian.Uint32(cam[16:]))
	m := realspace.MatrixFromRecord(o.RecRaw)
	type pt struct{ x, z, y int32 }
	type qpt struct {
		x, y *big.Int
		d    uint64
	}
	project := func(p pt) qpt {
		px := float64(float64(p.x) * float64(fx))
		py := float64(float64(p.y) * float64(fy))
		den := float64(float64(256) * float64(p.z))
		ux := float64(px / den)
		uy := float64(py / den)
		qx := float64(float64(cx) + ux)
		qy := float64(float64(int64(cy)+1) - uy)
		nx := float64(float64(n) * qx)
		ny := float64(float64(n) * qy)
		round := func(x float64) *big.Int {
			r := new(big.Rat).SetFloat64(x)
			r.Mul(r, big.NewRat(256, 1))
			sgn := r.Sign()
			a := new(big.Int).Abs(r.Num())
			q, rem := new(big.Int), new(big.Int)
			q.QuoRem(a, r.Denom(), rem)
			twice := new(big.Int).Lsh(rem, 1)
			cmp := twice.Cmp(r.Denom())
			if cmp > 0 || cmp == 0 && q.Bit(0) != 0 {
				q.Add(q, big.NewInt(1))
			}
			if sgn < 0 {
				q.Neg(q)
			}
			return q
		}
		return qpt{round(nx), round(ny), (uint64(1) << 48) / uint64(p.z)}
	}
	edge := func(a, b, p qpt) *big.Int {
		dx := new(big.Int).Sub(b.x, a.x)
		dy := new(big.Int).Sub(b.y, a.y)
		x := new(big.Int).Sub(p.x, a.x)
		y := new(big.Int).Sub(p.y, a.y)
		return new(big.Int).Sub(new(big.Int).Mul(dx, y), new(big.Int).Mul(dy, x))
	}
	depth := make([]uint64, got.Width*got.Height)
	owner := make([]uint32, len(depth))
	for i := range owner {
		owner[i] = math.MaxUint32
	}
	for fi, face := range o.Faces {
		if face.AL == 0 {
			continue
		}
		vertices := []pt{}
		for _, idx := range face.VIdx {
			v := realspace.Transform(entry.Verts[idx], m)
			vertices = append(vertices, pt{v[0], v[1], v[2]})
		}
		poly := []pt{}
		for i, a := range vertices {
			b := vertices[(i+1)%3]
			ai, bi := a.z >= o.Near, b.z >= o.Near
			if ai {
				poly = append(poly, a)
			}
			if ai != bi {
				pi, po := a, b
				if !ai {
					pi, po = b, a
				}
				lerp := func(x, y int32) int32 {
					num := new(big.Int).Mul(big.NewInt(int64(x)-int64(y)), big.NewInt(int64(pi.z)-int64(o.Near)))
					num.Quo(num, big.NewInt(int64(pi.z)-int64(po.z)))
					return int32(int64(x) - num.Int64())
				}
				poly = append(poly, pt{lerp(pi.x, po.x), o.Near, lerp(pi.y, po.y)})
			}
		}
		if o.Flag17BC != 0 {
			poly = vertices
		}
		for fan := 1; fan+1 < len(poly); fan++ {
			q := []qpt{project(poly[0]), project(poly[fan]), project(poly[fan+1])}
			area := edge(q[0], q[1], q[2])
			if area.Sign() == 0 {
				continue
			}
			if area.Sign() < 0 {
				q[1], q[2] = q[2], q[1]
				area.Neg(area)
			}
			a := []qpt{q[1], q[2], q[0]}
			b := []qpt{q[2], q[0], q[1]}
			inclusive := [3]bool{}
			var step [3]*big.Int
			for j := 0; j < 3; j++ {
				dy := new(big.Int).Sub(b[j].y, a[j].y)
				dx := new(big.Int).Sub(b[j].x, a[j].x)
				inclusive[j] = dy.Sign() < 0 || dy.Sign() == 0 && dx.Sign() > 0
				step[j] = new(big.Int).Mul(new(big.Int).Neg(dy), big.NewInt(256))
			}
			// All viewport samples are independently visited. Edge values advance
			// exactly by their affine differences; no renderer bbox/clipping reused.
			for y := 0; y < got.Height; y++ {
				p := qpt{big.NewInt(128), big.NewInt(int64(256*y + 128)), 0}
				var e [3]*big.Int
				for j := 0; j < 3; j++ {
					e[j] = edge(a[j], b[j], p)
				}
				num, delta := new(big.Int), new(big.Int)
				for j := 0; j < 3; j++ {
					d := new(big.Int).SetUint64(q[j].d)
					num.Add(num, new(big.Int).Mul(e[j], d))
					delta.Add(delta, new(big.Int).Mul(step[j], d))
				}
				for x := 0; x < got.Width; x++ {
					inside := true
					for j := 0; j < 3; j++ {
						if e[j].Sign() < 0 || e[j].Sign() == 0 && !inclusive[j] {
							inside = false
							break
						}
					}
					if inside {
						z := new(big.Int).Quo(num, area).Uint64()
						k := y*got.Width + x
						if z > depth[k] {
							depth[k] = z
							owner[k] = uint32(fi)
						}
					}
					for j := 0; j < 3; j++ {
						e[j].Add(e[j], step[j])
					}
					num.Add(num, delta)
				}
			}
		}
	}
	for i, z := range depth {
		if got.DepthQ48[i] != z || z != 0 && (got.Owner[i].Object != 0 || got.Owner[i].Face != owner[i]) {
			t.Fatalf("near BigInt oracle N%d pixel(%d,%d): got depth=%d owner=%+v want depth=%d face=%d", n, i%got.Width, i/got.Width, got.DepthQ48[i], got.Owner[i], z, owner[i])
		}
	}
}
