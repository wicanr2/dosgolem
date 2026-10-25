package r5asset

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"
)

// Fixtures follow the app's existing explicit opt-in convention. Validate any
// configured roots before deciding to skip an absent companion root.
func fixtureRoots(t *testing.T, assets, evidence bool) (string, string) {
	t.Helper()
	assetRoot, evidenceRoot := os.Getenv("SC_R5_ASSET_ROOT"), os.Getenv("SC_R5_EVIDENCE_ROOT")
	roots := []struct {
		name, path string
		needed     bool
		files      []string
	}{
		{"SC_R5_ASSET_ROOT", assetRoot, assets, []string{
			"f16/model/model.gltf", "f16/model/model.bin", "f16/model/albedo.png", "f16/model/normal.png", "f16/model/metal_rough.png",
			"mig29/model/model.gltf", "mig29/model/model.bin", "mig29/model/albedo.png", "mig29/model/normal.png", "mig29/model/metal_rough.png"}},
		{"SC_R5_EVIDENCE_ROOT", evidenceRoot, evidence, []string{"identity-vector-002.json", "prototype-001/oracle-001.json", "texture-prototype-001/pixel-oracle-001.json"}},
	}
	var absent []string
	for _, r := range roots {
		if !r.needed {
			continue
		}
		if r.path == "" {
			absent = append(absent, r.name)
			continue
		}
		st, e := os.Stat(r.path)
		if e != nil || !st.IsDir() {
			t.Fatalf("configured %s=%q is not a readable fixture directory: %v", r.name, r.path, e)
		}
		for _, file := range r.files {
			name := filepath.Join(r.path, file)
			st, e := os.Stat(name)
			if e != nil || !st.Mode().IsRegular() {
				t.Fatalf("configured %s fixture file %q is missing or not regular: %v", r.name, name, e)
			}
		}
	}
	if len(absent) != 0 {
		t.Skipf("explicit read-only fixtures not configured: %s", strings.Join(absent, ", "))
	}
	return assetRoot, evidenceRoot
}

func TestFixedJSONBodyAndGrammar(t *testing.T) {
	if unsafe.Sizeof(token{}) != 16 || unsafe.Sizeof(frame{}) != 32 || arenaBytes > JSONBodyLimit || arenaBytes+numberReservation > JSONReservation {
		t.Fatal("fixed body sizing", arenaBytes)
	}
	for _, s := range []string{`{}`, `{"a":[true,false,null,-1.25e+2]}`, `{"a":"\ud83d\ude00"}`, `"` + strings.Repeat("a", 256) + `"`, strings.Repeat("[", 31) + "0" + strings.Repeat("]", 31)} {
		a := new(jsonArena)
		if _, e := a.parse([]byte(s)); e != nil {
			t.Fatal(s, e)
		}
	}
	for _, s := range []string{"", `{} {}`, `{"a":}`, `{"a":0,}`, `[0,]`, `01`, `+1`, `NaN`, `0x10`, `1e`, `"\uD800"`, `"\uDC00"`, `"\x20"`, `"` + strings.Repeat("a", 257) + `"`, strings.Repeat("[", 32) + "0" + strings.Repeat("]", 32), `[` + strings.Repeat("0,", 64) + `0]`, `[` + strings.Repeat("0,", 2048) + `0]`, strings.Repeat(" ", 9841)} {
		a := new(jsonArena)
		if _, e := a.parse([]byte(s)); e == nil {
			t.Fatal("accepted malformed/over-budget", s)
		}
	}
	a := new(jsonArena)
	if _, e := a.parse([]byte{'"', 0xff, '"'}); e == nil {
		t.Fatal("invalid UTF8")
	}
}

func TestFixedJSONDecodedDuplicateAndNumbers(t *testing.T) {
	for _, s := range []string{`{"a":0,"a":1}`, `{"a":0,"\u0061":1}`, `{"a":null}`, `{"a":1,"other":2}`} {
		a := new(jsonArena)
		root, e := a.parse([]byte(s))
		if e != nil {
			t.Fatal(e)
		}
		v := validator{}
		v.obj(root, "a", "")
		if v.err == nil {
			t.Fatal("profile guard did not run", s)
		}
	}
	a := new(jsonArena)
	root, e := a.parse([]byte(`{"a":18446744073709551616}`))
	if e != nil {
		t.Fatal(e)
	}
	v := validator{}
	v.u(root.get("a"))
	if v.err == nil {
		t.Fatal("uint64 overflow")
	}
	root, e = a.parse([]byte(`{"a":1e999}`))
	if e != nil {
		t.Fatal(e)
	}
	v = validator{}
	v.f(root.get("a"))
	if v.err == nil {
		t.Fatal("nonfinite decimal")
	}
}

func TestFixedIdentityLiteralVector(t *testing.T) {
	_, evidenceRoot := fixtureRoots(t, false, true)
	b, e := os.ReadFile(filepath.Join(evidenceRoot, "identity-vector-002.json"))
	if e != nil {
		t.Fatal(e)
	}
	var vectors struct {
		Rows  []struct{ Model, CanonicalHex, AssetSHA256 string }
		Batch struct{ CanonicalHex, BatchSHA256 string }
	}
	if e = json.Unmarshal(b, &vectors); e != nil {
		t.Fatal(e)
	}
	for i, p := range profiles {
		got := assetCanonical(p)
		want, e := hex.DecodeString(vectors.Rows[i].CanonicalHex)
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(got[:], want) || hex.EncodeToString(hashBytes(assetHash(p))) != vectors.Rows[i].AssetSHA256 {
			t.Fatal("literal identity differs", p.Name)
		}
	}
	if hex.EncodeToString(hashBytes(batchHash())) != vectors.Batch.BatchSHA256 {
		t.Fatal("literal batch differs")
	}
}
func hashBytes(x [32]byte) []byte { return x[:] }

func TestMeshRawAndInverseIndependentOracle(t *testing.T) {
	sourceRoot, evidenceRoot := fixtureRoots(t, true, true)
	b, e := os.ReadFile(filepath.Join(evidenceRoot, "prototype-001/oracle-001.json"))
	if e != nil {
		t.Fatal(e)
	}
	var oracle struct {
		Rows []struct {
			Model                                 string
			AccessorSHA256, InversePositionSHA256 []string
		}
	}
	if e = json.Unmarshal(b, &oracle); e != nil {
		t.Fatal(e)
	}
	for model, p := range profiles {
		t.Run(p.Name, func(t *testing.T) {
			j, e := os.ReadFile(filepath.Join(sourceRoot, p.Name, "model/model.gltf"))
			if e != nil {
				t.Fatal(e)
			}
			bin, e := os.ReadFile(filepath.Join(sourceRoot, p.Name, "model/model.bin"))
			if e != nil {
				t.Fatal(e)
			}
			if sha256.Sum256(j) != p.Files[0].SHA || sha256.Sum256(bin) != p.Files[1].SHA {
				t.Fatal("original input identity")
			}
			a := new(jsonArena)
			copy(a.raw[:], j)
			a.rawN = uint32(len(j))
			m, e := validateMesh(a, bin, p)
			if e != nil {
				t.Fatal(e)
			}
			for pi := 0; pi < m.Count; pi++ {
				for role := 0; role < 5; role++ {
					h := sha256.New()
					var raw [4]byte
					n := m.Prim[pi].Vertices
					if role == 4 {
						n = m.Prim[pi].Indices
					}
					for i := uint64(0); i < n; i++ {
						if role == 4 {
							x, e := indexAt(bin, m, pi, i)
							if e != nil {
								t.Fatal(e)
							}
							binary.LittleEndian.PutUint32(raw[:], x)
							h.Write(raw[:])
							continue
						}
						v, e := vertexAt(bin, m, pi, i)
						if e != nil {
							t.Fatal(e)
						}
						var values []float32
						switch role {
						case 0:
							values = v.Position[:]
						case 1:
							values = v.Normal[:]
						case 2:
							values = v.Tangent[:]
						case 3:
							values = v.UV[:]
						}
						for _, x := range values {
							binary.LittleEndian.PutUint32(raw[:], math.Float32bits(x))
							h.Write(raw[:])
						}
					}
					if hex.EncodeToString(h.Sum(nil)) != oracle.Rows[model].AccessorSHA256[pi*5+role] {
						t.Fatal("independent raw accessor SHA", pi, role)
					}
				}
				h := sha256.New()
				var raw [8]byte
				for i := uint64(0); i < m.Prim[pi].Vertices; i++ {
					v, e := vertexAt(bin, m, pi, i)
					if e != nil {
						t.Fatal(e)
					}
					for _, x := range inverse(v.Position) {
						binary.LittleEndian.PutUint64(raw[:], math.Float64bits(x))
						h.Write(raw[:])
					}
				}
				if hex.EncodeToString(h.Sum(nil)) != oracle.Rows[model].InversePositionSHA256[pi] {
					t.Fatal("independent Fraction inverse SHA", pi)
				}
			}
			// Value getters cannot alias or change the original captured bin.
			before := sha256.Sum256(bin)
			v, e := vertexAt(bin, m, 0, 0)
			if e != nil {
				t.Fatal(e)
			}
			v.Position[0] = 999
			if sha256.Sum256(bin) != before {
				t.Fatal("getter alias")
			}
			// These are controlled inner failures, beyond original file SHA.
			changed := append([]byte(nil), bin...)
			// UV has no POSITION min/max check which could mask a missing finite guard.
			binary.LittleEndian.PutUint32(changed[int(m.Prim[0].Offset[3]):], 0x7fc00000)
			copy(a.raw[:], j)
			a.rawN = uint32(len(j))
			if _, e = validateMesh(a, changed, p); e == nil {
				t.Fatal("binary finite guard")
			}
			copy(changed, bin)
			binary.LittleEndian.PutUint32(changed[int(m.Prim[0].Offset[4]):], ^uint32(0))
			copy(a.raw[:], j)
			a.rawN = uint32(len(j))
			if _, e = validateMesh(a, changed, p); e == nil {
				t.Fatal("binary index guard")
			}
		})
	}
}
