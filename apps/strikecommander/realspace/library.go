package realspace

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Diagnostic struct{ TRE, Name, Reason string }
type TREStats struct {
	TRE                               string
	SHA256                            [32]byte
	HeaderCount, DataStart            uint32
	Entries, Objects, IFF, Poly, Appr int
}
type LibraryStats struct {
	Archives                                  []TREStats
	Poly, Appr, Rejected, NoGeometry, NotFORM int
}

// Library owns decoded geometry read from the player's local archives.
// Entries and identification results are read-only views; callers must not mutate them.
type Library struct {
	entries     []*ModelEntry
	diagnostics []Diagnostic
	stats       LibraryStats
}

func (l *Library) Entries() []*ModelEntry    { return append([]*ModelEntry(nil), l.entries...) }
func (l *Library) Diagnostics() []Diagnostic { return append([]Diagnostic(nil), l.diagnostics...) }
func (l *Library) Stats() LibraryStats {
	s := l.stats
	s.Archives = append([]TREStats(nil), s.Archives...)
	return s
}

func objectName(name string) bool {
	parts := strings.Split(strings.ToUpper(name), `\`)
	return len(parts) >= 2 && parts[len(parts)-2] == "OBJECTS" && strings.HasSuffix(parts[len(parts)-1], ".IFF")
}

func LoadLibrary(root string) (*Library, error) {
	l := &Library{}
	for _, name := range []string{"BIGTRE.TRE", "LILTRE.TRE", "TOBIGTRE.TRE"} {
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			return nil, fmt.Errorf("model library %s: %w", name, err)
		}
		if err := l.addTRE(name, b); err != nil {
			return nil, err
		}
	}
	return l, nil
}

func (l *Library) reject(tre, name, reason string) {
	l.stats.Rejected++
	l.diagnostics = append(l.diagnostics, Diagnostic{tre, name, reason})
}
func (l *Library) addTRE(tre string, b []byte) error {
	if len(b) < 8 {
		return fmt.Errorf("%s: short TRE header", tre)
	}
	n, start := binary.LittleEndian.Uint32(b), binary.LittleEndian.Uint32(b[4:])
	// The count bounds the directory. DataStart may be much later than its end;
	// scanning until DataStart would misinterpret intervening asset bytes as names.
	if uint64(n)*74+8 > uint64(len(b)) || uint64(n)*74+8 > uint64(start) || uint64(start) > uint64(len(b)) {
		return fmt.Errorf("%s: TRE directory out of bounds", tre)
	}
	st := TREStats{TRE: tre, SHA256: sha256.Sum256(b), HeaderCount: n, DataStart: start, Entries: int(n)}
	for i := uint32(0); i < n; i++ {
		d := b[8+int(i)*74 : 8+int(i+1)*74]
		nb := d[1:66]
		if z := bytes.IndexByte(nb, 0); z >= 0 {
			nb = nb[:z]
		}
		name := string(nb)
		if !objectName(name) {
			continue
		}
		st.Objects++
		off, size := binary.LittleEndian.Uint32(d[66:]), binary.LittleEndian.Uint32(d[70:])
		if uint64(off)+uint64(size) > uint64(len(b)) {
			l.reject(tre, name, "TRE entry out of bounds")
			continue
		}
		data := b[int(off):int(uint64(off)+uint64(size))]
		if len(data) < 4 || string(data[:4]) != "FORM" {
			l.stats.NotFORM++
			l.diagnostics = append(l.diagnostics, Diagnostic{tre, name, "not FORM"})
			continue
		}
		st.IFF++
		ch := map[string][]byte{}
		if err := walkIFF(data, nil, ch, 0); err != nil {
			l.reject(tre, name, err.Error())
			continue
		}
		found := false
		for _, kind := range []ModelKind{POLY, APPR} {
			part := "POLY"
			if kind == APPR {
				part = "AFTB"
			}
			pre := "REAL/APPR/" + part + "/"
			if ch[pre+"VERT"] == nil || ch[pre+"TRIS/VTRI"] == nil {
				continue
			}
			found = true
			e, err := decodeModel(tre, name, kind, pre, ch)
			if err != nil {
				l.reject(tre, name, err.Error())
				continue
			}
			l.entries = append(l.entries, e)
			if kind == POLY {
				l.stats.Poly++
				st.Poly++
			} else {
				l.stats.Appr++
				st.Appr++
			}
		}
		if !found {
			l.stats.NoGeometry++
			l.diagnostics = append(l.diagnostics, Diagnostic{tre, name, "no POLY/AFTB geometry"})
		}
	}
	l.stats.Archives = append(l.stats.Archives, st)
	return nil
}

func walkIFF(b []byte, path []string, ch map[string][]byte, depth int) error {
	if depth > 64 {
		return fmt.Errorf("IFF nesting limit")
	}
	for off := 0; off < len(b); {
		if len(b)-off < 8 {
			return fmt.Errorf("IFF short chunk header")
		}
		tag := string(b[off : off+4])
		size := uint64(binary.BigEndian.Uint32(b[off+4:]))
		end := uint64(off) + 8 + size
		if end > uint64(len(b)) {
			return fmt.Errorf("IFF chunk out of bounds: %s", strings.Join(append(path, tag), "/"))
		}
		data := b[off+8 : int(end)]
		if tag == "FORM" || tag == "LIST" || tag == "CAT " {
			if len(data) < 4 {
				return fmt.Errorf("IFF short container")
			}
			next := append(append([]string(nil), path...), string(data[:4]))
			if err := walkIFF(data[4:], next, ch, depth+1); err != nil {
				return err
			}
		} else {
			key := strings.Join(append(path, tag), "/")
			if _, ok := ch[key]; !ok {
				ch[key] = data
			}
		}
		end += size & 1
		if end > uint64(len(b)) {
			return fmt.Errorf("IFF missing odd padding")
		}
		off = int(end)
	}
	return nil
}

func decodeModel(tre, name string, kind ModelKind, pre string, ch map[string][]byte) (*ModelEntry, error) {
	vb, vt := ch[pre+"VERT"], ch[pre+"TRIS/VTRI"]
	if len(vb) == 0 || len(vb)%12 != 0 || len(vt)%8 != 0 {
		return nil, fmt.Errorf("invalid VERT/VTRI stride")
	}
	e := &ModelEntry{TRE: tre, Name: name, Kind: kind, VertSHA: sha256.Sum256(vb), Verts: make([][3]int32, len(vb)/12), VTRI: make([][8]byte, len(vt)/8)}
	for i := range e.Verts {
		for j := 0; j < 3; j++ {
			e.Verts[i][j] = int32(binary.LittleEndian.Uint32(vb[i*12+j*4:]))
		}
	}
	for i := range e.VTRI {
		copy(e.VTRI[i][:], vt[i*8:i*8+8])
		for _, v := range e.VTRI[i][1:4] {
			if int(v) >= len(e.Verts) {
				return nil, fmt.Errorf("VTRI vertex index out of bounds")
			}
		}
	}
	for key, b := range ch {
		if !strings.HasPrefix(key, pre+"DETA/LVL") {
			continue
		}
		level, err := strconv.Atoi(strings.TrimPrefix(key, pre+"DETA/LVL"))
		if err != nil || level < 0 || level > 255 {
			return nil, fmt.Errorf("invalid LOD number")
		}
		if len(b) < 4 || (len(b)-4)%2 != 0 {
			return nil, fmt.Errorf("invalid LVL stride")
		}
		for len(e.LOD) <= level {
			e.LOD = append(e.LOD, nil)
		}
		e.LOD[level] = make([]uint16, (len(b)-4)/2)
		for i := range e.LOD[level] {
			idx := binary.LittleEndian.Uint16(b[4+i*2:])
			if int(idx) >= len(e.VTRI) {
				return nil, fmt.Errorf("LOD triangle index out of bounds")
			}
			e.LOD[level][i] = idx
		}
	}
	if len(e.LOD) == 0 {
		return nil, fmt.Errorf("no LOD table")
	}
	geometry := func(h hash.Hash, verts []byte) [32]byte {
		writeLP(h, verts)
		for _, lod := range e.LOD {
			if lod == nil {
				continue
			}
			b := make([]byte, 0, len(lod)*2)
			for _, idx := range lod {
				b = binary.LittleEndian.AppendUint16(b, idx)
			}
			writeLP(h, b)
		}
		b := make([]byte, 0, len(e.VTRI)*4)
		for _, r := range e.VTRI {
			b = append(b, r[:4]...)
		}
		writeLP(h, b)
		var key [32]byte
		copy(key[:], h.Sum(nil))
		return key
	}
	e.GeoKey = geometry(sha256.New(), vb)
	if kind == APPR {
		ymin := e.Verts[0][1]
		for _, v := range e.Verts {
			if v[1] < ymin {
				ymin = v[1]
			}
		}
		nonApex := make([]byte, 0, len(vb))
		var restMin int64 = 1<<63 - 1
		for i, v := range e.Verts {
			if v[1] == ymin {
				e.Apex = append(e.Apex, i)
			} else {
				nonApex = append(nonApex, vb[i*12:i*12+12]...)
				if int64(v[1]) < restMin {
					restMin = int64(v[1])
				}
			}
		}
		if len(e.Apex) == len(e.Verts) || restMin-int64(ymin) < 100 {
			return nil, fmt.Errorf("APPR apex separation below 100")
		}
		e.LNTH = append([]byte(nil), ch[pre+"LNTH"]...)
		if len(e.LNTH)%4 != 0 {
			return nil, fmt.Errorf("invalid LNTH stride")
		}
		e.AppKey = geometry(sha256.New(), nonApex)
	}
	return e, nil
}

func writeLP(h hash.Hash, b []byte) {
	var n [4]byte
	binary.LittleEndian.PutUint32(n[:], uint32(len(b)))
	h.Write(n[:])
	h.Write(b)
}

var faceClass = map[uint16]byte{0x346d: 1, 0x3495: 2, 0x3441: 4, 0x3415: 5, 0x33e9: 6, 0x33b9: 7, 0x338d: 8, 0x335d: 9}

func compatibleFaces(e *ModelEntry, lod int, triples [][3]int16, faceVT []uint16, filter bool) bool {
	if lod < 0 || lod >= len(e.LOD) || e.LOD[lod] == nil || len(e.LOD[lod]) != len(triples) || len(faceVT) != len(triples) {
		return false
	}
	for i, t := range triples {
		r := e.VTRI[e.LOD[lod][i]]
		if t != [3]int16{int16(r[1]), int16(r[2]), int16(r[3])} {
			return false
		}
		if want, ok := faceClass[faceVT[i]]; filter && ok && r[0] != want {
			return false
		}
	}
	return true
}
func sortEntries(es []*ModelEntry, kind ModelKind) {
	sort.SliceStable(es, func(i, j int) bool {
		a, b := es[i], es[j]
		ka, kb := a.GeoKey, b.GeoKey
		if kind == APPR {
			ka, kb = a.AppKey, b.AppKey
		}
		if c := bytes.Compare(ka[:], kb[:]); c != 0 {
			return c < 0
		}
		if a.TRE != b.TRE {
			return strings.ToUpper(a.TRE) < strings.ToUpper(b.TRE)
		}
		return strings.ToUpper(a.Name) < strings.ToUpper(b.Name)
	})
}
func (l *Library) Identify(sha [32]byte, lod int, triples [][3]int16, faceVT []uint16) []*ModelEntry {
	return l.identify(sha, nil, lod, triples, faceVT, POLY, true)
}
func (l *Library) IdentifyAppr(rt [][3]int32, lod int, triples [][3]int16, faceVT []uint16) []*ModelEntry {
	return l.identify([32]byte{}, rt, lod, triples, faceVT, APPR, true)
}
func (l *Library) identify(sha [32]byte, rt [][3]int32, lod int, triples [][3]int16, faceVT []uint16, kind ModelKind, filter bool) []*ModelEntry {
	var found []*ModelEntry
	if l == nil {
		return found
	}
	for _, e := range l.entries {
		if e.Kind != kind {
			continue
		}
		if kind == POLY {
			if e.VertSHA != sha {
				continue
			}
		} else {
			if len(rt) != len(e.Verts) {
				continue
			}
			apex := map[int]bool{}
			for _, i := range e.Apex {
				apex[i] = true
			}
			ok := true
			for i, v := range e.Verts {
				if apex[i] {
					if rt[i][0] != v[0] || rt[i][2] != v[2] {
						ok = false
						break
					}
				} else if rt[i] != v {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
		}
		if compatibleFaces(e, lod, triples, faceVT, filter) {
			found = append(found, e)
		}
	}
	sortEntries(found, kind)
	return found
}
