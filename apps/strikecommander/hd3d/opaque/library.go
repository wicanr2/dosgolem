package opaque

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type Asset struct {
	TRE, Name string
	IFF       []byte
}
type materialEntry struct {
	key      [32]byte
	vert     [32]byte
	vtri     [][8]byte
	lod      [][]uint16
	uv       [][8]byte
	textures [][]byte
}
type decodedTexture struct {
	w, h   uint16
	pixels []byte
	key    [32]byte
}

// MaterialLibrary owns original IFF contents, not serialized Selected metadata.
// Its content-addressed decoded cache is bounded and never exposed to callers.
type MaterialLibrary struct {
	entries      map[string]*materialEntry
	bands        []Band
	key          [32]byte
	owned        uint64
	mu           sync.Mutex
	decoded      map[[32]byte]decodedTexture
	decodedBytes uint64
}

func assetKey(tre, name string, kind realspace.ModelKind) string {
	return strings.ToUpper(tre) + "\x00" + strings.ToUpper(name) + "\x00" + string(kind)
}
func (l *MaterialLibrary) Key() [32]byte { return l.key }
func (l *MaterialLibrary) OwnedBytes() uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.owned + l.decodedBytes
}
func (l *MaterialLibrary) entry(tre, name string, k realspace.ModelKind) (*materialEntry, error) {
	e := l.entries[assetKey(tre, name, k)]
	if e == nil {
		return nil, fmt.Errorf("no original material %s/%s/%s", tre, name, k)
	}
	return e, nil
}
func walkChunks(b []byte, path string, out map[string][][]byte, depth int) error {
	if depth > 64 {
		return fmt.Errorf("IFF nesting limit")
	}
	for off := 0; off < len(b); {
		if len(b)-off < 8 {
			return fmt.Errorf("short IFF header")
		}
		tag := string(b[off : off+4])
		n := uint64(binary.BigEndian.Uint32(b[off+4:]))
		end := uint64(off) + 8 + n
		if end > uint64(len(b)) {
			return fmt.Errorf("IFF chunk bounds")
		}
		v := b[off+8 : int(end)]
		if tag == "FORM" || tag == "LIST" || tag == "CAT " {
			if len(v) < 4 {
				return fmt.Errorf("short container")
			}
			p := path + string(v[:4]) + "/"
			if string(v[:4]) == "TXMS" {
				out[strings.TrimSuffix(p, "/")] = append(out[strings.TrimSuffix(p, "/")], v)
			}
			if err := walkChunks(v[4:], p, out, depth+1); err != nil {
				return err
			}
		} else {
			out[path+tag] = append(out[path+tag], v)
		}
		end += n & 1
		if end > uint64(len(b)) {
			return fmt.Errorf("missing IFF pad")
		}
		off = int(end)
	}
	return nil
}
func NewMaterialLibrary(assets []Asset, paletteIFF []byte) (*MaterialLibrary, error) {
	l := &MaterialLibrary{entries: map[string]*materialEntry{}, decoded: map[[32]byte]decodedTexture{}}
	pch := map[string][][]byte{}
	if err := walkChunks(paletteIFF, "", pch, 0); err != nil {
		return nil, err
	}
	var rang []byte
	for k, vs := range pch {
		if strings.HasSuffix(k, "/RANG") {
			if rang != nil || len(vs) != 1 {
				return nil, fmt.Errorf("RANG not unique")
			}
			rang = vs[0]
		}
	}
	if len(rang) < 2 || len(rang) != 2+13*int(binary.LittleEndian.Uint16(rang)) {
		return nil, fmt.Errorf("RANG size")
	}
	for i := 2; i < len(rang); i += 13 {
		b := Band{binary.LittleEndian.Uint16(rang[i:]), binary.LittleEndian.Uint16(rang[i+2:]), binary.LittleEndian.Uint16(rang[i+4:])}
		if !b.valid() {
			return nil, fmt.Errorf("RANG range")
		}
		l.bands = append(l.bands, b)
	}
	ordered := append([]Asset(nil), assets...)
	sort.Slice(ordered, func(i, j int) bool {
		return assetKey(ordered[i].TRE, ordered[i].Name, "") < assetKey(ordered[j].TRE, ordered[j].Name, "")
	})
	h := sha256.New()
	h.Write(paletteIFF)
	l.owned = uint64(len(paletteIFF))
	for _, a := range ordered {
		raw := append([]byte(nil), a.IFF...)
		l.owned += uint64(len(raw))
		if l.owned > 256<<20 {
			return nil, fmt.Errorf("original material storage exceeds total budget")
		}
		h.Write([]byte(assetKey(a.TRE, a.Name, "")))
		sum := sha256.Sum256(raw)
		h.Write(sum[:])
		ch := map[string][][]byte{}
		if err := walkChunks(raw, "", ch, 0); err != nil {
			return nil, fmt.Errorf("%s: %w", a.Name, err)
		}
		for _, kind := range []realspace.ModelKind{realspace.POLY, realspace.APPR} {
			part := "POLY"
			if kind == realspace.APPR {
				part = "AFTB"
			}
			pre := "REAL/APPR/" + part + "/"
			vb, vt := ch[pre+"VERT"], ch[pre+"TRIS/VTRI"]
			if len(vb) == 0 && len(vt) == 0 {
				continue
			}
			if len(vb) != 1 || len(vt) != 1 || len(vb[0])%12 != 0 || len(vt[0])%8 != 0 {
				return nil, fmt.Errorf("material geometry chunks not unique")
			}
			e := &materialEntry{key: sum, vert: sha256.Sum256(vb[0])}
			for j := 0; j < len(vt[0]); j += 8 {
				var v [8]byte
				copy(v[:], vt[0][j:j+8])
				e.vtri = append(e.vtri, v)
			}
			for k, vs := range ch {
				if strings.HasPrefix(k, pre+"DETA/LVL") {
					n, err := strconv.Atoi(strings.TrimPrefix(k, pre+"DETA/LVL"))
					if err != nil || n < 0 || n > 255 || len(vs) != 1 || len(vs[0]) < 4 || len(vs[0])%2 != 0 {
						return nil, fmt.Errorf("material LOD chunks")
					}
					for len(e.lod) <= n {
						e.lod = append(e.lod, nil)
					}
					b := vs[0][4:]
					e.lod[n] = make([]uint16, 0, len(b)/2)
					for i := 0; i < len(b); i += 2 {
						e.lod[n] = append(e.lod[n], binary.LittleEndian.Uint16(b[i:]))
					}
				}
			}
			for _, b := range ch[pre+"TRIS/UVXY"] {
				if len(b)%8 != 0 {
					return nil, fmt.Errorf("UVXY stride")
				}
				for j := 0; j < len(b); j += 8 {
					var u [8]byte
					copy(u[:], b[j:j+8])
					e.uv = append(e.uv, u)
				}
			}
			e.textures = ch[pre+"TRIS/TXMS/TXMP"]
			key := assetKey(a.TRE, a.Name, kind)
			if l.entries[key] != nil {
				return nil, fmt.Errorf("duplicate original material asset")
			}
			l.entries[key] = e
		}
	}
	copy(l.key[:], h.Sum(nil))
	return l, nil
}
func (l *MaterialLibrary) texture(raw []byte) (decodedTexture, error) {
	k := sha256.Sum256(raw)
	l.mu.Lock()
	defer l.mu.Unlock()
	if v, ok := l.decoded[k]; ok {
		return v, nil
	}
	v, e := decodeTexture(raw)
	if e != nil {
		return v, e
	}
	if l.decodedBytes+uint64(len(v.pixels)) > 32<<20 {
		return decodedTexture{}, Failure{Gate: H3, Object: -1, Face: -1, Pass: -1, Reason: "decoded cache capacity"}
	}
	l.decoded[k] = v
	l.decodedBytes += uint64(len(v.pixels))
	return v, nil
}

// Same bounded late-width LZW contract as the original-source tools/sc decoder.
// The W*H prefix is the texture; any trailing decoded byte has unknown purpose.
func decodeTexture(raw []byte) (decodedTexture, error) {
	if len(raw) < 14 || string(raw[12:14]) != "LZ" {
		return decodedTexture{}, fmt.Errorf("TXMP header")
	}
	w, h := binary.LittleEndian.Uint16(raw[8:]), binary.LittleEndian.Uint16(raw[10:])
	want := uint64(w) * uint64(h)
	if want == 0 || want > 65536 {
		return decodedTexture{}, fmt.Errorf("TXMP range")
	}
	src := raw[14:]
	var dic [4096][]byte
	reset := func() {
		for i := range dic {
			dic[i] = nil
		}
		for i := 0; i < 256; i++ {
			dic[i] = []byte{byte(i)}
		}
	}
	reset()
	next, width, pos := 258, 9, 0
	var prev, out []byte
	for pos+width <= len(src)*8 {
		var code int
		for i := 0; i < width; i++ {
			code |= int((src[(pos+i)/8]>>uint((pos+i)%8))&1) << uint(i)
		}
		pos += width
		if code == 256 {
			reset()
			next, width, prev = 258, 9, nil
			continue
		}
		if code == 257 {
			break
		}
		var entry []byte
		if code < next && dic[code] != nil {
			entry = dic[code]
		} else if code == next && prev != nil {
			entry = append(append([]byte(nil), prev...), prev[0])
		} else {
			return decodedTexture{}, fmt.Errorf("LZW invalid code")
		}
		out = append(out, entry...)
		if prev != nil && next < 4096 {
			dic[next] = append(append([]byte(nil), prev...), entry[0])
			next++
		}
		prev = entry
		if next >= (1<<uint(width)) && width < 12 {
			width++
		}
		if uint64(len(out)) >= want {
			break
		}
	}
	if uint64(len(out)) != want {
		return decodedTexture{}, fmt.Errorf("LZW bounded size differs")
	}
	return decodedTexture{w: w, h: h, pixels: out, key: sha256.Sum256(out)}, nil
}

// LoadMaterialLibrary reads original TRE/IFF assets outside all machine callbacks.
// Only object IFFs and PALETTE.IFF are retained; no captured bytes are synthesized.
func LoadMaterialLibrary(root string) (*MaterialLibrary, error) {
	// Scope follows the independently parsed original geometry library. An
	// unrelated malformed object that it rejects must not prevent valid sources
	// loading; a missing referenced source still fails H2 at binding.
	geo, err := realspace.LoadLibrary(root)
	if err != nil {
		return nil, err
	}
	needed := map[string]bool{}
	for _, e := range geo.Entries() {
		needed[strings.ToUpper(e.TRE)+"\x00"+strings.ToUpper(e.Name)] = true
	}
	var assets []Asset
	var palette []byte
	for _, tre := range []string{"BIGTRE.TRE", "LILTRE.TRE", "TOBIGTRE.TRE"} {
		b, e := os.ReadFile(filepath.Join(root, tre))
		if e != nil {
			return nil, e
		}
		if len(b) < 8 {
			return nil, fmt.Errorf("short TRE")
		}
		n, start := binary.LittleEndian.Uint32(b), binary.LittleEndian.Uint32(b[4:])
		if uint64(n)*74+8 > uint64(start) || uint64(start) > uint64(len(b)) {
			return nil, fmt.Errorf("TRE directory bounds")
		}
		for i := uint32(0); i < n; i++ {
			d := b[8+int(i)*74 : 8+int(i+1)*74]
			name := string(bytes.TrimRight(bytes.SplitN(d[1:66], []byte{0}, 2)[0], "\x00"))
			off, size := binary.LittleEndian.Uint32(d[66:]), binary.LittleEndian.Uint32(d[70:])
			if uint64(off)+uint64(size) > uint64(len(b)) {
				return nil, fmt.Errorf("TRE asset bounds")
			}
			up := strings.ToUpper(name)
			raw := b[off : uint64(off)+uint64(size)]
			if tre == "BIGTRE.TRE" && up == `..\..\DATA\PALETTE\PALETTE.IFF` { // 013a explicit RANG source, not other archives palettes.
				if palette != nil {
					return nil, fmt.Errorf("palette source not unique")
				}
				palette = raw
			}
			parts := strings.Split(up, `\`)
			if needed[strings.ToUpper(tre)+"\x00"+up] && len(parts) >= 2 && parts[len(parts)-2] == "OBJECTS" && strings.HasSuffix(up, ".IFF") && len(raw) >= 4 && string(raw[:4]) == "FORM" {
				assets = append(assets, Asset{tre, name, raw})
			}
		}
	}
	if palette == nil {
		return nil, fmt.Errorf("missing original palette IFF")
	}
	return NewMaterialLibrary(assets, palette)
}
