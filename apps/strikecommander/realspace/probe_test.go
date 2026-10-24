package realspace

// Legacy probe receipts are optional offline regression inputs. They do not
// replace phase-1 recordings on the current runner. Old C1 receipts lack SI;
// this test fills camera identity from O1 and therefore does not validate binding.

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func probeLines(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		t.Skip("missing original probe " + path)
	} else if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out [][]string
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 4096), 1024*1024)
	for s.Scan() {
		out = append(out, strings.Split(s.Text(), "\t"))
	}
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}
func probeHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func probeInt(t *testing.T, s string, base int) int64 {
	t.Helper()
	v, err := strconv.ParseInt(s, base, 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func probePtr(t *testing.T, s string) FarPtr {
	t.Helper()
	ss := strings.Split(s, ":")
	if len(ss) != 2 {
		t.Fatal("bad far pointer")
	}
	return FarPtr{Off: uint16(probeInt(t, ss[1], 16)), Seg: uint16(probeInt(t, ss[0], 16))}
}

func TestLegacyProbeOfflineChecks(t *testing.T) {
	root, research := os.Getenv("SC_REALSPACE_ROOT"), os.Getenv("SC_REALSPACE_PROBES")
	if root == "" || research == "" {
		t.Skip("requires original TRE plus existing probe receipts; no replacement data")
	}
	lib, err := LoadLibrary(root)
	if os.IsNotExist(err) {
		t.Skip("missing original TRE")
	} else if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name        string
		xyz, screen uint64
		objects     int
	}{{"s760", 6480, 6270, 150}, {"s800f2", 7147, 7147, 30}, {"n1", 16578, 16578, 210}, {"n2", 15090, 15090, 120}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(research, "p13-1", tc.name)
			cams := map[int]Camera{}
			for _, f := range probeLines(t, filepath.Join(dir, "cam.tsv")) {
				if f[2] != "C1" {
					continue
				}
				var c Camera
				copy(c.Raw[:], probeHex(t, f[5]))
				cams[int(probeInt(t, f[0], 10))] = c
			}
			faces := map[int][]FaceObs{}
			for _, f := range probeLines(t, filepath.Join(dir, "faces.tsv")) {
				id := int(probeInt(t, f[1], 10))
				face := FaceObs{SI: uint16(probeInt(t, f[2], 10)), Face: probePtr(t, f[5]), FaceVT: uint16(probeInt(t, strings.TrimPrefix(f[6], "vt="), 16)), Method: probePtr(t, strings.TrimPrefix(f[7], "m18=")), Returned: f[9] == "ok", AL: uint8(probeInt(t, f[11], 10))}
				for i, s := range strings.Split(f[8], ",") {
					face.VIdx[i] = int16(probeInt(t, s, 10))
				}
				for i, s := range strings.Split(f[13], ",") {
					if s != "-" {
						copy(face.Proj[i][:], probeHex(t, s))
					}
				}
				if f[12] != "-" {
					face.Elem = probePtr(t, f[12])
				}
				copy(face.Geom[:], probeHex(t, f[14]))
				faces[id] = append(faces[id], face)
			}
			var xyz, screen, near, appr, objects uint64
			for _, f := range probeLines(t, filepath.Join(dir, "objs.tsv")) {
				kv := map[string]string{}
				for _, s := range f[3:] {
					if k, v, ok := strings.Cut(s, "="); ok {
						kv[k] = v
					}
				}
				frame, id := int(probeInt(t, f[0], 10)), int(probeInt(t, f[2], 10))
				o := ObjObs{Steps: uint64(probeInt(t, f[1], 10)), Rec: probePtr(t, kv["rec"]), RecVT: uint16(probeInt(t, kv["vt"], 16)), Model: probePtr(t, kv["model"]), CamObj: uint16(probeInt(t, kv["cam"], 16)), CamIdx: 0, LOD: uint8(probeInt(t, kv["lod"], 10)), NFaces: int16(probeInt(t, kv["nf"], 10)), FaceBase: probePtr(t, kv["base"]), Flag17BC: uint8(probeInt(t, kv["f17"], 10)), Near: int32(probeInt(t, kv["near"], 10)), Far: int32(probeInt(t, kv["far"], 10)), Faces: faces[id]}
				copy(o.RecRaw[:], probeHex(t, kv["recraw"]))
				copy(o.ModelHead[:], probeHex(t, kv["modelhead"]))
				copy(o.VertSHA[:], probeHex(t, kv["vsha"]))
				o.VertXYZ = probeHex(t, kv["vxyz"])
				for i := 0; i < len(o.VertXYZ); i += 12 {
					o.VertRT = append(o.VertRT, [3]int32{readI32(o.VertXYZ[i:]), readI32(o.VertXYZ[i+4:]), readI32(o.VertXYZ[i+8:])})
				}
				lst := probeHex(t, kv["listraw"])
				for i := 0; i < len(lst); i += 2 {
					o.FaceList = append(o.FaceList, binary.LittleEndian.Uint16(lst[i:]))
				}
				cam, ok := cams[frame]
				if !ok {
					t.Fatal("no legacy C1")
				}
				cam.CamObj = o.CamObj
				v := CheckObject(lib, []Camera{cam}, &o)
				if v.Status != Passed {
					t.Fatalf("frame=%d obj=%d status=%s reason=%s xyz=%d/%d screen=%d/%d first=%+v", frame, id, v.Status, v.Reason, v.XYZCompared, v.XYZMismatch, v.ScreenCompared, v.ScreenMismatch, v.First)
				}
				objects++
				xyz += v.XYZCompared
				screen += v.ScreenCompared
				near += v.NearNoDivide
				if o.RecVT == 0x314a {
					appr++
				}
			}
			t.Logf("legacy offline only: objects=%d appr=%d xyz=%d screen=%d noDivide=%d; C1 identity filled from O1, not a camera binding receipt", objects, appr, xyz, screen, near)
			if int(objects) != tc.objects || xyz != tc.xyz || screen != tc.screen {
				t.Fatal("comparison counts differ from RE028")
			}
		})
	}
}
