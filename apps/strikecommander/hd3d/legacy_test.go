package hd3d

// Old evidence receipts are test-only input adapters. They are not current-runner
// captures and cannot establish the product recorder's completion or neutrality.
import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type legacyStore struct {
	files map[string]string
	cache map[string][]byte
}

func openLegacyStore(path string) (*legacyStore, error) {
	es, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	s := &legacyStore{map[string]string{}, map[string][]byte{}}
	for _, e := range es {
		if len(e.Name()) >= 16 {
			k := e.Name()[:16]
			if _, ok := s.files[k]; ok {
				return nil, fmt.Errorf("duplicate capture hash prefix")
			}
			s.files[k] = filepath.Join(path, e.Name())
		}
	}
	return s, nil
}
func (s *legacyStore) get(key string) ([]byte, error) {
	if b, ok := s.cache[key]; ok {
		return b, nil
	}
	p, ok := s.files[key]
	if !ok {
		return nil, fmt.Errorf("capture %s missing", key)
	}
	b, err := os.ReadFile(p)
	if err == nil {
		s.cache[key] = b
	}
	return b, err
}
func legacyWords(s string) []uint16 {
	var out []uint16
	for _, f := range strings.Fields(s) {
		x, _ := strconv.ParseUint(f, 16, 16)
		out = append(out, uint16(x))
	}
	return out
}
func legacyHex(s string) uint16 { x, _ := strconv.ParseUint(s, 16, 16); return uint16(x) }

var legacyHexField = regexp.MustCompile(`(\w+)=([0-9A-Fa-f\-]+)`)
var legacySHAField = regexp.MustCompile(`sha=([0-9a-f\-]+)`)
var legacyTblOffset = regexp.MustCompile(`tbl13DA=[0-9A-F]{4}:([0-9A-F]{4})`)

type legacyGroup struct {
	group  Group
	writer [PixelCount]uint16
	dir    string
}

func loadLegacyGroup(dir string, store *legacyStore, tables Tables) (legacyGroup, error) {
	g := legacyGroup{dir: dir}
	g.group.Key, _ = strconv.ParseUint(filepath.Base(dir), 10, 64)
	read := func(name string) ([]byte, error) { return os.ReadFile(filepath.Join(dir, name)) }
	vb, err := read("verts.bin")
	if err != nil {
		return g, err
	}
	tb, err := read("tables.bin")
	if err != nil {
		return g, err
	}
	g.group.Buf3D, err = read("buf3d.bin")
	if err != nil {
		return g, err
	}
	wb, err := read("writer.bin")
	if err != nil {
		return g, err
	}
	if len(g.group.Buf3D) != PixelCount || len(wb) != 2*PixelCount {
		return g, fmt.Errorf("legacy oracle length")
	}
	for i := range g.writer {
		g.writer[i] = binary.LittleEndian.Uint16(wb[i*2:])
	}
	text, err := read("calls.tsv")
	if err != nil {
		return g, err
	}
	for li, line := range strings.Split(strings.TrimRight(string(text), "\n"), "\n") {
		if li == 0 {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < 15 {
			return g, fmt.Errorf("short legacy call")
		}
		seq, _ := strconv.ParseUint(f[0], 10, 16)
		step, _ := strconv.ParseUint(f[2], 10, 64)
		c := Call{Seq: uint16(seq), Kind: CallKind(f[1]), Steps: step, DS: legacyHex(f[4]), SS: legacyHex(f[5]), SP: legacyHex(f[6])}
		copy(c.Params[:], legacyWords(f[8]))
		copy(c.Desc[:], legacyWords(f[9]))
		vo, _ := strconv.Atoi(f[10])
		vl, _ := strconv.Atoi(f[11])
		to, _ := strconv.Atoi(f[12])
		if vo < 0 || vl < 0 || vo+vl > len(vb) {
			return g, fmt.Errorf("legacy vertex bounds")
		}
		c.Verts = vb[vo : vo+vl]
		if c.Kind == R2 {
			if to < 0 || to+256 > len(tb) {
				return g, fmt.Errorf("legacy table bounds")
			}
			c.Table = tb[to : to+256]
		}
		var tabH string
		for _, m := range legacyHexField.FindAllStringSubmatch(f[14], -1) {
			switch m[1] {
			case "c38BE":
				c.Aux.Color0 = byte(legacyHex(m[2]))
			case "c38C0":
				c.Aux.Color1 = byte(legacyHex(m[2]))
			case "flags":
				c.Aux.HasFlags = true
				c.Aux.Flags = legacyHex(m[2])
			case "tabh":
				tabH = m[2]
			case "resA5":
				c.Aux.HasResid = true
				c.Aux.ResA5 = legacyHex(m[2])
			case "res9F":
				c.Aux.Res9F = legacyHex(m[2])
			}
		}
		if tabH != "" {
			b, err := store.get(tabH)
			if err != nil {
				return g, err
			}
			list := make([]uint16, len(b)/2)
			for i := range list {
				list[i] = binary.LittleEndian.Uint16(b[i*2:])
			}
			if g.group.List == nil {
				g.group.List = list
			} else if !equalWords(g.group.List, list) {
				return g, fmt.Errorf("legacy line list changes")
			}
		}
		shas := legacySHAField.FindAllStringSubmatch(f[14], -1)
		if c.Kind == R4 || c.Kind == R5 {
			if len(shas) < 2 {
				return g, fmt.Errorf("legacy texture references missing")
			}
			pixels, err := store.get(shas[0][1])
			if err != nil {
				return g, err
			}
			c.Tex.Pixels = Capture{[]Span{{0, pixels}}}
			if shas[1][1] != "-" {
				shade, err := store.get(shas[1][1])
				if err != nil {
					return g, err
				}
				c.Tex.Shade = Capture{[]Span{{0, shade}}}
			}
			if c.Kind == R5 {
				if len(shas) < 3 {
					return g, fmt.Errorf("legacy R5 table missing")
				}
				c.Aux.Table, err = store.get(shas[2][1])
				if err != nil {
					return g, err
				}
				if m := legacyTblOffset.FindStringSubmatch(f[14]); m != nil {
					c.Aux.TableOff = legacyHex(m[1])
				}
			}
		}
		if c.Kind == "R10" {
			c.Kind = BlackBox
			pre, err := read(fmt.Sprintf("r10-%d.pre.bin", c.Seq))
			if err != nil {
				return g, err
			}
			post, err := read(fmt.Sprintf("r10-%d.post.bin", c.Seq))
			if err != nil {
				return g, err
			}
			var d Diagnostics
			c.Patch, d = MakePatch(pre, post, nil, g.group.List, tables)
			if len(d.Faults) > 0 {
				return g, fmt.Errorf("legacy blackbox: %v", d.Faults)
			}
		}
		g.group.Calls = append(g.group.Calls, c)
	}
	return g, nil
}
func equalWords(a, b []uint16) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func legacyInputs(t *testing.T) (string, Tables) {
	t.Helper()
	root, exe := os.Getenv("SC_RASTER_REC"), os.Getenv("SC_RASTER_EXE")
	if root == "" || exe == "" {
		t.Skip("requires original rec fixtures and STRIKE.EXE; no substitute")
	}
	b, err := os.ReadFile(exe)
	if os.IsNotExist(err) {
		t.Skip("original executable absent")
	} else if err != nil {
		t.Fatal(err)
	}
	tables, err := TablesFromEXE(b)
	if err != nil {
		t.Fatal(err)
	}
	return root, tables
}
func legacyDirs(t *testing.T, root, state string) []string {
	t.Helper()
	es, err := os.ReadDir(filepath.Join(root, state))
	if os.IsNotExist(err) {
		t.Skip("original receipt absent: " + state)
	} else if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	for _, e := range es {
		if _, err := strconv.ParseUint(e.Name(), 10, 64); err == nil && e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Strings(dirs)
	return dirs
}

func cropCapture(full Capture, off uint16, roww uint16, u0, u1, v0, v1 int) Capture {
	var out Capture
	read := func(off uint16) byte {
		for _, s := range full.Spans {
			d := int(uint16(off - s.Off))
			if d < len(s.Data) {
				return s.Data[d]
			}
		}
		panic("test adapter full capture absent")
	}
	for v := v0; v <= v1; v++ {
		start := uint16(int(off) + v*int(roww) + u0)
		b := make([]byte, u1-u0+1)
		for i := range b {
			b[i] = read(start + uint16(i))
		}
		out.Spans = append(out.Spans, Span{start, b})
	}
	return out
}
func cropCalls(calls []Call, shrink bool) []Call {
	out := append([]Call(nil), calls...)
	for i := range out {
		c := &out[i]
		if c.Kind != R4 && c.Kind != R5 {
			continue
		}
		n := int(c.Params[1])
		off, row := c.Params[4], c.Params[6]
		pad := 0
		if c.Kind == R5 {
			n = 3
			off, row = c.Params[7], c.Params[9]
			pad = 2
		}
		minu, maxu, minv, maxv := 256, -1, 256, -1
		for k := 0; k < n; k++ {
			u := int(binary.LittleEndian.Uint16(c.Verts[k*22+6:]) >> 8)
			v := int(binary.LittleEndian.Uint16(c.Verts[k*22+8:]) >> 8)
			minu = min(minu, u)
			maxu = max(maxu, u)
			minv = min(minv, v)
			maxv = max(maxv, v)
		}
		if shrink && c.Kind == R4 {
			maxv--
		}
		if shrink && c.Kind == R5 {
			pad = 0
		}
		c.Tex.Pixels = cropCapture(c.Tex.Pixels, off, row, minu-pad, maxu+pad, minv-pad, maxv+pad)
		if c.Kind == R4 && len(c.Tex.Shade.Spans) > 0 {
			c.Tex.Shade = cropCapture(c.Tex.Shade, c.Params[7], 256, 0, 255, 0, 0)
		}
	}
	return out
}

func TestLegacy60Groups(t *testing.T) {
	root, tables := legacyInputs(t)
	for _, state := range []string{"s800-p2b", "s760-p2b", "s800f2-p2b"} {
		t.Run(state, func(t *testing.T) {
			store, err := openLegacyStore(filepath.Join(root, state, "tex"))
			if err != nil {
				t.Fatal(err)
			}
			dirs := legacyDirs(t, root, state)
			if len(dirs) != 20 {
				t.Fatalf("groups %d want 20", len(dirs))
			}
			for _, dir := range dirs {
				g, err := loadLegacyGroup(filepath.Join(root, state, dir), store, tables)
				if err != nil {
					t.Fatal(err)
				}
				for _, rect := range []bool{false, true} {
					input := g.group
					if rect {
						input.Calls = cropCalls(input.Calls, false)
					}
					r := Replay(input, tables)
					badWriter := 0
					for i, w := range r.Writer {
						if w != g.writer[i] {
							badWriter++
						}
					}
					if !r.Repaintable || r.Diagnostics.PixelMismatch != 0 || badWriter != 0 {
						t.Fatalf("%s rect=%t valid=%t mismatch=%d writer=%d faults=%v cap=%d uninit=%d after=%d", dir, rect, r.Valid, r.Diagnostics.PixelMismatch, badWriter, r.Diagnostics.Faults, r.Diagnostics.CaptureShort, len(r.Diagnostics.Uninitialized), len(r.Diagnostics.UninitializedAfterLast))
					}
				}
			}
			t.Logf("20/20 pixel+writer exact, both full legacy captures and typed rectangles; old evidence only")
		})
	}
}

func TestRectShrinkDetected(t *testing.T) {
	root, tables := legacyInputs(t)
	state := "s760-p2b"
	store, err := openLegacyStore(filepath.Join(root, state, "tex"))
	if err != nil {
		t.Fatal(err)
	}
	caught := 0
	for _, dir := range legacyDirs(t, root, state) {
		g, err := loadLegacyGroup(filepath.Join(root, state, dir), store, tables)
		if err != nil {
			t.Fatal(err)
		}
		g.group.Calls = cropCalls(g.group.Calls, true)
		r := Replay(g.group, tables)
		if r.Diagnostics.CaptureShort > 0 && !r.Repaintable {
			caught++
		}
	}
	if caught == 0 {
		t.Fatal("M15 shrinking captures remained undetected")
	}
	t.Logf("M15 capture-short groups %d/20", caught)
}

func TestLegacyR2ValueSnapshots(t *testing.T) {
	root, tables := legacyInputs(t)
	state := "s800f2-p4b"
	dirs := legacyDirs(t, root, state)
	store, err := openLegacyStore(filepath.Join(root, state, "tex"))
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, dir := range dirs {
		path := filepath.Join(root, state, dir)
		g, err := loadLegacyGroup(path, store, tables)
		if err != nil {
			t.Fatal(err)
		}
		for _, call := range g.group.Calls {
			if call.Kind != R2 {
				continue
			}
			pre, e1 := os.ReadFile(filepath.Join(path, fmt.Sprintf("r2-%d.pre.bin", call.Seq)))
			post, e2 := os.ReadFile(filepath.Join(path, fmt.Sprintf("r2-%d.post.bin", call.Seq)))
			if os.IsNotExist(e1) || os.IsNotExist(e2) {
				continue
			}
			if e1 != nil || e2 != nil || len(pre) != PixelCount || len(post) != PixelCount {
				t.Fatal("bad R2 snapshots")
			}
			c := newContext(g.group.List, tables)
			copy(c.buf[:], pre)
			for i := range c.written {
				c.written[i] = true
			}
			runCalls(c, []Call{call}, false)
			if len(c.faults) > 0 || !bytes.Equal(c.buf[:], post) {
				t.Fatalf("R2 value snapshot seq=%d faults=%v", call.Seq, c.faults)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Skip("no original per-R2 snapshots available")
	}
	t.Logf("R2 pre/post snapshots exact: %d calls", checked)
}

func TestR5UnknownVariantAndResidualStop(t *testing.T) {
	root, tables := legacyInputs(t)
	state := "s760-p2b"
	store, err := openLegacyStore(filepath.Join(root, state, "tex"))
	if err != nil {
		t.Fatal(err)
	}
	dirs := legacyDirs(t, root, state)
	if len(dirs) == 0 {
		t.Skip("no original group")
	}
	g, err := loadLegacyGroup(filepath.Join(root, state, dirs[0]), store, tables)
	if err != nil {
		t.Fatal(err)
	}
	unknown := g.group
	unknown.Calls = append([]Call(nil), g.group.Calls...)
	found := false
	for i := range unknown.Calls {
		if unknown.Calls[i].Kind == R5 {
			unknown.Calls[i].Params[11] = 1
			found = true
		}
	}
	if !found {
		t.Skip("no original R5")
	}
	r := Replay(unknown, tables)
	if r.Valid || len(r.Diagnostics.Faults) == 0 {
		t.Fatal("unknown R5 variant must stop")
	}
	resid := g.group
	resid.Calls = append([]Call(nil), g.group.Calls...)
	for i := range resid.Calls {
		if resid.Calls[i].Kind == R5 {
			resid.Calls[i].Aux.HasResid = false
		}
	}
	r = Replay(resid, tables)
	if r.Valid || len(r.Diagnostics.Faults) == 0 {
		t.Fatal("flat R5 without captured residual must stop")
	}
}
