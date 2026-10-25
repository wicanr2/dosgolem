package r5asset

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"hash/crc32"
	"image"
	"unsafe"
)

const (
	pageSize               = 65536
	maxPages               = 2048
	maxTiles               = 65536
	dictSize               = maxTiles * 2
	decodedCapacity uint64 = 4096 * 4096 * 4
	decoderReserve  uint64 = 2 << 20
)

type workspace struct {
	hash                 [dictSize]uint64
	id                   [dictSize]uint32
	tile, check, palette [1024]byte
	code                 [256]byte
	localKey             [512]uint32
	localID              [512]uint16
	row                  [4096 * 4]byte
}

type tileRecord struct {
	Offset          uint32
	Length, Palette uint16
}
type page struct {
	data []byte
	hold *allocation
}
type texture struct {
	C, N    int
	Index   []uint32
	Records []tileRecord
	Pages   []page
	Used    int
	Payload uint64
	hold    *allocation
}

const textureWorkBytes = uint64(unsafe.Sizeof(workspace{}))

func newTexture(c int, l *Loader) (*texture, error) {
	if c != 3 && c != 4 {
		return nil, reject(Structure, 0, 300)
	}
	n := uint64(unsafe.Sizeof(texture{})) + maxTiles*4 + maxTiles*uint64(unsafe.Sizeof(tileRecord{})) + maxPages*uint64(unsafe.Sizeof(page{}))
	h, e := l.allocate(n)
	if e != nil {
		return nil, e
	}
	return &texture{C: c, Index: make([]uint32, maxTiles), Records: make([]tileRecord, maxTiles), Pages: make([]page, 0, maxPages), hold: h}, nil
}
func (t *texture) close() {
	if t == nil {
		return
	}
	for i := range t.Pages {
		p := &t.Pages[i]
		p.data = nil
		p.hold.release()
		p.hold = nil
	}
	t.Index = nil
	t.Records = nil
	t.Pages = nil
	t.N = 0
	t.Used = 0
	t.Payload = 0
	t.hold.release()
	t.hold = nil
}
func (t *texture) put(data []byte, pal uint16, l *Loader) (tileRecord, error) {
	if len(data) == 0 || len(data) > 1024 {
		return tileRecord{}, reject(Structure, 0, 301)
	}
	if len(t.Pages) == 0 || t.Used+len(data) > pageSize {
		if len(t.Pages) == cap(t.Pages) {
			return tileRecord{}, reject(Capacity, 0, 302)
		}
		h, e := l.allocate(pageSize)
		if e != nil {
			return tileRecord{}, e
		}
		t.Pages = append(t.Pages, page{make([]byte, pageSize), h})
		t.Used = 0
	}
	off := (len(t.Pages)-1)*pageSize + t.Used
	copy(t.Pages[len(t.Pages)-1].data[t.Used:t.Used+len(data)], data)
	t.Used += len(data)
	t.Payload += uint64(len(data))
	return tileRecord{uint32(off), uint16(len(data)), pal}, nil
}
func (t *texture) read(id uint32, out []byte) error {
	if int(id) >= t.N || len(out) != 256*t.C {
		return reject(Structure, 0, 303)
	}
	r := t.Records[id]
	pi, off := int(r.Offset)/pageSize, int(r.Offset)%pageSize
	if pi >= len(t.Pages) || int(r.Length) > pageSize-off {
		return reject(Structure, 0, 303)
	}
	data := t.Pages[pi].data[off : off+int(r.Length)]
	if r.Palette == 0 {
		if len(data) != len(out) {
			return reject(Structure, 0, 303)
		}
		copy(out, data)
		return nil
	}
	n := int(r.Palette)
	if n > 256 || len(data) != n*t.C+256 {
		return reject(Structure, 0, 303)
	}
	for i, code := range data[n*t.C:] {
		if int(code) >= n {
			return reject(Structure, 0, 303)
		}
		copy(out[i*t.C:(i+1)*t.C], data[int(code)*t.C:(int(code)+1)*t.C])
	}
	return nil
}
func tileHash(raw []byte) uint64 {
	h := uint64(14695981039346656037)
	for _, x := range raw {
		h = (h ^ uint64(x)) * 1099511628211
	}
	return h
}
func encodeTile(w *workspace, raw []byte, c int) ([]byte, uint16) {
	clear(w.localID[:])
	n := 0
	for i := 0; i < 256; i++ {
		key := uint32(0)
		for k := 0; k < c; k++ {
			key |= uint32(raw[i*c+k]) << uint(8*k)
		}
		slot := int((key*2654435761)>>23) & 511
		for w.localID[slot] != 0 && w.localKey[slot] != key {
			slot = (slot + 1) & 511
		}
		id := w.localID[slot]
		if id == 0 {
			id = uint16(n + 1)
			w.localID[slot] = id
			w.localKey[slot] = key
			copy(w.palette[n*c:(n+1)*c], raw[i*c:(i+1)*c])
			n++
		}
		w.code[i] = byte(id - 1)
	}
	if n*c+256 >= len(raw) {
		return raw, 0
	}
	copy(w.check[:n*c], w.palette[:n*c])
	copy(w.check[n*c:n*c+256], w.code[:])
	return w.check[:n*c+256], uint16(n)
}
func intern(t *texture, w *workspace, raw []byte, h uint64, b *Loader) (uint32, error) {
	slot := int(h & uint64(dictSize-1))
	for w.id[slot] != 0 {
		id := w.id[slot] - 1
		if w.hash[slot] == h {
			if e := t.read(id, w.check[:len(raw)]); e != nil {
				return 0, e
			}
			if bytes.Equal(raw, w.check[:len(raw)]) {
				return id, nil
			}
		}
		slot = (slot + 1) & (dictSize - 1)
	}
	if t.N >= maxTiles {
		return 0, reject(Structure, 0, 303)
	}
	data, pal := encodeTile(w, raw, t.C)
	r, e := t.put(data, pal, b)
	if e != nil {
		return 0, e
	}
	id := uint32(t.N)
	t.Records[id] = r
	t.N++
	w.hash[slot] = h
	w.id[slot] = id + 1
	return id, nil
}
func rawPixels(img image.Image, c int) ([]byte, int, error) {
	switch p := img.(type) {
	case *image.NRGBA:
		if c != 4 {
			return nil, 0, reject(Decode, 0, 304)
		}
		if p.Rect != image.Rect(0, 0, 4096, 4096) || p.Stride != 4096*4 || cap(p.Pix) != int(decodedCapacity) || len(p.Pix) != int(decodedCapacity) {
			return nil, 0, reject(Structure, 0, 303)
		}
		return p.Pix, p.Stride, nil
	case *image.RGBA:
		if c != 3 {
			return nil, 0, reject(Decode, 0, 305)
		}
		if p.Rect != image.Rect(0, 0, 4096, 4096) || p.Stride != 4096*4 || cap(p.Pix) != int(decodedCapacity) || len(p.Pix) != int(decodedCapacity) {
			return nil, 0, reject(Structure, 0, 303)
		}
		return p.Pix, p.Stride, nil
	}
	return nil, 0, reject(Structure, 0, 303)
}
func copyTile(pix []byte, stride, x, y, c int, out []byte) {
	for j := 0; j < 16; j++ {
		for i := 0; i < 16; i++ {
			from := (y+j)*stride + (x+i)*4
			to := (j*16 + i) * c
			copy(out[to:to+c], pix[from:from+c])
		}
	}
}
func build(pix []byte, stride int, c int, w *workspace, b *Loader) (t *texture, err error) {
	t, err = newTexture(c, b)
	if err != nil {
		return nil, err
	}
	candidate := t
	defer func() {
		if err != nil {
			candidate.close()
			t = nil
		}
	}()
	clear(w.id[:])
	for y := 0; y < 4096; y += 16 {
		for x := 0; x < 4096; x += 16 {
			raw := w.tile[:256*c]
			copyTile(pix, stride, x, y, c, raw)
			id, e := intern(t, w, raw, tileHash(raw), b)
			if e != nil {
				return nil, e
			}
			t.Index[(y/16)*256+x/16] = id
		}
	}
	return t, nil
}

func checkPNG(raw []byte, c int) error {
	if len(raw) < 33 || !bytes.Equal(raw[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
		return reject(Structure, 0, 306)
	}
	off := 8
	chunks := 0
	seenIDAT := false
	for off < len(raw) {
		if len(raw)-off < 12 {
			return reject(Structure, 0, 306)
		}
		n := uint64(binary.BigEndian.Uint32(raw[off : off+4]))
		if n > uint64(len(raw)-off-12) {
			return reject(Structure, 0, 306)
		}
		typ := string(raw[off+4 : off+8])
		end := off + 8 + int(n)
		if crc32.ChecksumIEEE(raw[off+4:end]) != binary.BigEndian.Uint32(raw[end:end+4]) {
			return reject(Structure, 0, 306)
		}
		chunks++
		switch typ {
		case "IHDR":
			want := byte(2)
			if c == 4 {
				want = 6
			}
			if off != 8 || n != 13 || binary.BigEndian.Uint32(raw[off+8:off+12]) != 4096 || binary.BigEndian.Uint32(raw[off+12:off+16]) != 4096 || raw[off+16] != 8 || raw[off+17] != want || raw[off+18] != 0 || raw[off+19] != 0 || raw[off+20] != 0 {
				return reject(Structure, 0, 306)
			}
		case "IDAT":
			if chunks == 1 {
				return reject(Structure, 0, 306)
			}
			seenIDAT = true
		case "IEND":
			if !seenIDAT || n != 0 || end+4 != len(raw) {
				return reject(Structure, 0, 306)
			}
			return nil
		default:
			return reject(Structure, 0, 306)
		}
		off = end + 4
	}
	return reject(Structure, 0, 306)
}

// Source/cache verification does not create a full reconstructed image.
func verify(t *texture, w *workspace, pix []byte, stride int) ([32]byte, error) {
	h := sha256.New()
	for y := 0; y < 4096; y++ {
		if t.C == 4 {
			h.Write(pix[y*stride : y*stride+4096*4])
		} else {
			for x := 0; x < 4096; x++ {
				at := y*stride + x*4
				if pix[at+3] != 255 {
					return [32]byte{}, reject(Decode, 0, 307)
				}
				copy(w.row[x*3:x*3+3], pix[at:at+3])
			}
			h.Write(w.row[:4096*3])
		}
	}
	for y := 0; y < 4096; y += 16 {
		for x := 0; x < 4096; x += 16 {
			copyTile(pix, stride, x, y, t.C, w.tile[:256*t.C])
			if e := t.read(t.Index[(y/16)*256+x/16], w.check[:256*t.C]); e != nil {
				return [32]byte{}, e
			}
			if !bytes.Equal(w.tile[:256*t.C], w.check[:256*t.C]) {
				return [32]byte{}, reject(Decode, 0, 308)
			}
		}
	}
	var out [32]byte
	copy(out[:], h.Sum(out[:0]))
	return out, nil
}
func (t *texture) pixel(x, y int) ([4]byte, uint8, error) {
	var out [4]byte
	if t == nil || x < 0 || x >= 4096 || y < 0 || y >= 4096 {
		return out, 0, reject(Structure, 0, 309)
	}
	id := t.Index[(y/16)*256+x/16]
	if int(id) >= t.N {
		return out, 0, reject(Structure, 0, 310)
	}
	r := t.Records[id]
	pi, off := int(r.Offset)/pageSize, int(r.Offset)%pageSize
	if pi >= len(t.Pages) || int(r.Length) > pageSize-off {
		return out, 0, reject(Structure, 0, 311)
	}
	raw := t.Pages[pi].data[off : off+int(r.Length)]
	at := (y%16)*16 + x%16
	if r.Palette == 0 {
		if len(raw) != 256*t.C {
			return out, 0, reject(Structure, 0, 312)
		}
		at *= t.C
	} else {
		n := int(r.Palette)
		if n > 256 || len(raw) != n*t.C+256 {
			return out, 0, reject(Structure, 0, 313)
		}
		code := int(raw[n*t.C+at])
		if code >= n {
			return out, 0, reject(Structure, 0, 314)
		}
		at = code * t.C
	}
	copy(out[:t.C], raw[at:at+t.C])
	if t.C == 3 {
		out[3] = 255
	}
	return out, uint8(t.C), nil
}
