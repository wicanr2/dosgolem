package strikecommander

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"
)

// Fixed workspace: no archive-sized read, dictionary strings, shape allocations
// or unbounded append. Both this entire struct and the inline retained owner are
// checked before their allocation. These limits describe R8-owned data, not the
// runtime's catalogs/fonts or Go process RSS.
type frontendMenuWorkspace struct {
	tomenu    [12291]byte
	mouse     [610]byte
	buttons   [10314]byte
	prefix    [4096]uint16
	suffix    [4096]byte
	stack     [4096]byte
	k1        [menuBlankPoints]byte
	mask      [menuButtonPoints]bool
	cursor    [121]byte
	cursorOn  [121]bool
	treHeader [8]byte
	treRecord [74]byte
}

func newFrontendMenu(root string, cfg FrontendMenuConfig) (*frontendMenuPolicy, error) {
	// 016a's already-verified cache profile is absolute. Relative user source
	// paths are still accepted by the importer and become absolute cache roots.
	// Refuse an unverified relative cache before allocation or filesystem IO.
	pathBytes := uint64(len(root)) + 1 + uint64(len("BIGTRE.TRE"))
	retained, workspace, err := menuAllocationBytes(runtime.GOOS, filepath.IsAbs(root), pathBytes, menuWindowsExtended(root), cfg)
	if err != nil {
		return nil, err
	}
	// This validation precedes both allocation sites and every filesystem read.
	m := new(frontendMenuPolicy)
	w := new(frontendMenuWorkspace)
	if err := w.load(root, m); err != nil {
		m.close()
		return nil, err
	}
	m.retained, m.workspacePeak = retained, workspace
	return m, nil
}

func menuAllocationBytes(goos string, absolute bool, pathBytes uint64, extended bool, cfg FrontendMenuConfig) (uint64, uint64, error) {
	retained := uint64(unsafe.Sizeof(frontendMenuPolicy{})) + frontendMenuHeaderBytes()
	pathWorkspace, err := menuPathWorkspace(goos, absolute, pathBytes, extended)
	if err != nil {
		return 0, 0, err
	}
	fixedWorkspace := uint64(unsafe.Sizeof(frontendMenuWorkspace{}))
	if pathWorkspace > ^uint64(0)-fixedWorkspace {
		return 0, 0, fmt.Errorf("016a: path workspace overflow")
	}
	workspace := fixedWorkspace + pathWorkspace
	rl, wl := cfg.RetainedBytes, cfg.WorkspaceBytes
	if rl == 0 {
		rl = frontendMenuRetainedLimit
	}
	if wl == 0 {
		wl = frontendMenuWorkspaceLimit
	}
	if rl > frontendMenuRetainedLimit || wl > frontendMenuWorkspaceLimit || retained > rl || workspace > wl {
		return 0, 0, fmt.Errorf("016a: R8 allocation budget (retained=%d workspace=%d)", retained, workspace)
	}
	return retained, workspace, nil
}

// menuPathWorkspace bounds the path payload allocations in Go 1.24.13's
// os.Open call chain. It is pure so Windows bounds can be checked on Linux.
// The caller must classify absolute with native filepath.IsAbs. The Windows
// absolute/extended precondition excludes Getwd and cwd-dependent retry sizes.
// Upgrade the pinned Go toolchain only after rechecking os/path_windows.go,
// os/file_windows.go and syscall/syscall_windows.go against the receipt.
func menuPathWorkspace(goos string, absolute bool, length uint64, extended bool) (uint64, error) {
	if !absolute {
		return 0, fmt.Errorf("016a: verified cache root must be absolute")
	}
	multiplier, extra := uint64(2), uint64(1) // joined string + byte NUL buffer
	if goos == "windows" {
		multiplier, extra = 3, 2 // joined string + cap(length+1) uint16 buffer
		if length >= 248 && !extended {
			// addExtendedPrefix: retain joined L, prefix[8]uint16, input
			// UTF16 cap(L+1), fullpath cap(L+1+8), returned UTF8 cap
			// 3*(L+1+8), then Open's UTF16 cap(3*(L+1+8)+1).
			// Sum every backing conservatively, even across function return:
			// L +16 +2*(L+1)+2*(L+9)+3*(L+9)+2*(3*(L+9)+1).
			multiplier, extra = 14, 119
		}
	}
	if length > (^uint64(0)-extra)/multiplier {
		return 0, fmt.Errorf("016a: path workspace overflow")
	}
	return multiplier*length + extra, nil
}

func menuWindowsExtended(path string) bool {
	sep := func(c byte) bool { return c == '\\' || c == '/' }
	return len(path) >= 4 && (path[:4] == `\??\` || sep(path[0]) && sep(path[1]) && path[2] == '?' && sep(path[3]))
}

func (w *frontendMenuWorkspace) load(root string, m *frontendMenuPolicy) error {
	file, err := os.Open(root + string(os.PathSeparator) + "BIGTRE.TRE")
	if err != nil {
		return err
	}
	defer file.Close()
	st, err := file.Stat()
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() || st.Size() < 8 {
		return fmt.Errorf("016a: BIGTRE shape")
	}
	if _, err = file.ReadAt(w.treHeader[:], 0); err != nil {
		return err
	}
	n := uint64(binary.LittleEndian.Uint32(w.treHeader[:]))
	directoryEnd := uint64(8) + n*74
	if n == 0 || n > 65536 || directoryEnd > uint64(st.Size()) {
		return fmt.Errorf("016a: TRE directory bounds")
	}
	var mouseOffset, tomenuOffset int64
	mouseHits, tomenuHits := 0, 0
	for i := uint64(0); i < n; i++ {
		if _, err = file.ReadAt(w.treRecord[:], int64(8+i*74)); err != nil {
			return err
		}
		name := w.treRecord[1:66]
		end := bytes.IndexByte(name, 0)
		if end < 0 {
			return fmt.Errorf("016a: TRE member name is not terminated")
		}
		name = name[:end]
		offset, size := uint64(binary.LittleEndian.Uint32(w.treRecord[66:])), uint64(binary.LittleEndian.Uint32(w.treRecord[70:]))
		if offset < directoryEnd || offset > uint64(st.Size()) || size > uint64(st.Size())-offset {
			return fmt.Errorf("016a: TRE member bounds")
		}
		switch {
		case bytes.EqualFold(name, []byte(`..\..\DATA\MOUSE.SHP`)):
			mouseHits++
			if size != uint64(len(w.mouse)) {
				return fmt.Errorf("016a: MOUSE size")
			}
			mouseOffset = int64(offset)
		case bytes.EqualFold(name, []byte(`..\..\DATA\GAMEFLOW\TOMENU.PAK`)):
			tomenuHits++
			if size != uint64(len(w.tomenu)) {
				return fmt.Errorf("016a: TOMENU size")
			}
			tomenuOffset = int64(offset)
		}
	}
	if mouseHits != 1 || tomenuHits != 1 {
		return fmt.Errorf("016a: TRE full member names must be unique")
	}
	if _, err = file.ReadAt(w.mouse[:], mouseOffset); err != nil {
		return err
	}
	if _, err = file.ReadAt(w.tomenu[:], tomenuOffset); err != nil {
		return err
	}
	return w.parse(m)
}

func menuAssetSHA(raw []byte, expected [32]byte) bool {
	return sha256.Sum256(raw) == expected
}

func (w *frontendMenuWorkspace) parse(m *frontendMenuPolicy) error {
	if !menuAssetSHA(w.mouse[:], [32]byte{0x8e, 0x7e, 0x07, 0xed, 0xa9, 0x88, 0x99, 0x42, 0xaa, 0x4a, 0x04, 0x23, 0x2c, 0x95, 0x97, 0xb7, 0x43, 0x04, 0x65, 0xe9, 0xac, 0xbd, 0x26, 0xfd, 0x27, 0xb3, 0xc7, 0xfb, 0xe2, 0xa5, 0xf8, 0x52}) || !menuAssetSHA(w.tomenu[:], [32]byte{0xe4, 0x5f, 0x17, 0xd8, 0x34, 0xa6, 0xea, 0xf9, 0x52, 0x59, 0x63, 0x5e, 0xf8, 0xf9, 0x8f, 0x6f, 0xa3, 0x2c, 0xe1, 0x9f, 0x93, 0xc8, 0xcc, 0xbf, 0x68, 0xc3, 0x16, 0xf0, 0x1f, 0xba, 0x6e, 0xe5}) {
		return fmt.Errorf("016a: MOUSE/TOMENU identity")
	}
	cursor, err := menuPAKEntry(w.mouse[:], 4, 0, 0)
	if err != nil {
		return err
	}
	if len(cursor) != 168 || !menuAssetSHA(cursor, [32]byte{0x1c, 0x47, 0xf1, 0xa4, 0xbd, 0x07, 0xae, 0xb2, 0x80, 0x4b, 0x31, 0xbb, 0x99, 0xe7, 0xb7, 0x9c, 0xe4, 0x05, 0x7d, 0x1b, 0x1b, 0x46, 0xa7, 0x10, 0xed, 0xf0, 0x6e, 0x68, 0xf6, 0x02, 0xbe, 0x82}) {
		return fmt.Errorf("016a: cursor entry identity")
	}
	for i := 0; i < 8; i += 2 {
		if binary.LittleEndian.Uint16(cursor[i:]) != 5 {
			return fmt.Errorf("016a: cursor origin")
		}
	}
	if err = menuDecodeShape(cursor, 11, 11, w.cursor[:], w.cursorOn[:]); err != nil {
		return err
	}
	points, colors := 0, uint8(0)
	for i, on := range w.cursorOn {
		if !on {
			continue
		}
		c := w.cursor[i]
		if c != 0 && c != 15 {
			return fmt.Errorf("016a: cursor color")
		}
		if points >= len(m.cursor) {
			return fmt.Errorf("016a: too many cursor writes")
		}
		m.cursor[points] = menuCursorPoint{int8(i%11 - 5), int8(i/11 - 5), c}
		points++
		if c == 0 {
			colors |= 1
		} else {
			colors |= 2
		}
	}
	if points != 56 || colors != 3 {
		return fmt.Errorf("016a: cursor write mask")
	}
	compressed, err := menuPAKEntry(w.tomenu[:], 3, 1, 0x20)
	if err != nil {
		return err
	}
	if len(compressed) < 4 || binary.LittleEndian.Uint32(compressed) != uint32(len(w.buttons)) {
		return fmt.Errorf("016a: button unpacked size")
	}
	if err = w.lzw(compressed[4:], false); err != nil {
		if err = w.lzw(compressed[4:], true); err != nil {
			return err
		}
	}
	if !menuAssetSHA(w.buttons[:], [32]byte{0x13, 0xdf, 0xa8, 0xec, 0xdd, 0x98, 0x6c, 0xf8, 0xb2, 0x98, 0xae, 0xa6, 0xd6, 0x4f, 0xcc, 0xf6, 0x2e, 0xd6, 0x1a, 0x95, 0x6f, 0x4f, 0xba, 0x47, 0x6e, 0x68, 0x95, 0x97, 0xb4, 0xf0, 0xe3, 0x7a}) {
		return fmt.Errorf("016a: button sub-PAK identity")
	}
	for index, k := range [2]int{0, 2} {
		entry, e := menuPAKEntry(w.buttons[:], 4, k, 0)
		if e != nil {
			return e
		}
		if e = menuDecodeShape(entry, menuButtonW, menuButtonH, m.reference[index][:], w.mask[:]); e != nil {
			return e
		}
		for _, on := range w.mask {
			if !on {
				return fmt.Errorf("016a: button reference is not opaque")
			}
		}
	}
	entry, err := menuPAKEntry(w.buttons[:], 4, 1, 0)
	if err != nil {
		return err
	}
	if err = menuDecodeShape(entry, menuBlankW, menuButtonH, w.k1[:], w.mask[:menuBlankPoints]); err != nil {
		return err
	}
	for _, on := range w.mask[:menuBlankPoints] {
		if !on {
			return fmt.Errorf("016a: k1 is not opaque")
		}
	}
	// Reproduce 008's tomenuBackground/inpaint contract with inline bounded
	// storage. Tests compare every pixel against that existing independent helper.
	copy(m.blank[:], w.k1[:])
	sp, _ := artSpecFor(ArtEvent{File: ArtTomenuButtons, K: 1})
	for i, c := range w.k1 {
		w.mask[i] = sp.center.in(i%menuBlankW, i/menuBlankW) && !bTomenu[c]
	}
	for y := sp.band.y0; y <= sp.band.y1; y++ {
		for x := sp.band.x0; x <= sp.band.x1; x++ {
			at := y*menuBlankW + x
			if !w.mask[at] {
				continue
			}
			found := false
			for pass := 0; pass < 2 && !found; pass++ {
				for d := 1; d < sp.band.x1-sp.band.x0+1 && !found; d++ {
					for _, nx := range [2]int{x - d, x + d} {
						if nx < sp.band.x0 || nx > sp.band.x1 {
							continue
						}
						p := y*menuBlankW + nx
						if !w.mask[p] && (pass == 1 || bTomenu[w.k1[p]]) {
							m.blank[at], found = w.k1[p], true
							break
						}
					}
				}
			}
		}
	}
	return nil
}

// PAK range validation is allocation-free. Count/type are fixed by each known
// resource; all offsets are checked before returning a borrowed workspace view.
func menuPAKEntry(data []byte, count, index int, kind byte) ([]byte, error) {
	if count < 1 || index < 0 || index >= count || len(data) < 4+4*count || int(binary.LittleEndian.Uint32(data)) != len(data) {
		return nil, fmt.Errorf("016a: PAK header")
	}
	first := 4 + 4*count
	var lo, hi int
	for i := 0; i < count; i++ {
		raw := binary.LittleEndian.Uint32(data[4+4*i:])
		start := int(raw & 0xffffff)
		end := len(data)
		if i+1 < count {
			end = int(binary.LittleEndian.Uint32(data[8+4*i:]) & 0xffffff)
		}
		if byte(raw>>24) != kind || start < first || end < start || end > len(data) || (i == 0 && start != first) {
			return nil, fmt.Errorf("016a: PAK entry bounds/type")
		}
		if i == index {
			lo, hi = start, end
		}
	}
	return data[lo:hi], nil
}

func (w *frontendMenuWorkspace) lzw(comp []byte, early bool) error {
	width, next, previous := 9, 258, -1
	bit, output := 0, 0
	for bit+width <= len(comp)*8 {
		code := 0
		for k := 0; k < width; k++ {
			code |= int((comp[(bit+k)/8]>>uint((bit+k)%8))&1) << uint(k)
		}
		bit += width
		if code == 256 {
			width, next, previous = 9, 258, -1
			continue
		}
		if code == 257 {
			if output != len(w.buttons) {
				return fmt.Errorf("016a: LZW output size")
			}
			return nil
		}
		if code > next || code >= 4096 || (previous < 0 && code >= 256) {
			return fmt.Errorf("016a: LZW code")
		}
		cur, count := code, 0
		extra := code == next
		if extra {
			cur = previous
		}
		for cur >= 256 {
			if cur < 258 || cur >= next || count >= len(w.stack)-1 {
				return fmt.Errorf("016a: LZW dictionary chain")
			}
			w.stack[count] = w.suffix[cur]
			count++
			cur = int(w.prefix[cur])
		}
		w.stack[count] = byte(cur)
		count++
		needed := count
		if extra {
			needed++
		}
		if needed > len(w.buttons)-output {
			return fmt.Errorf("016a: LZW output capacity")
		}
		first := byte(cur)
		for i := count - 1; i >= 0; i-- {
			w.buttons[output] = w.stack[i]
			output++
		}
		if extra {
			w.buttons[output] = first
			output++
		}
		if previous >= 0 && next < 4096 {
			w.prefix[next], w.suffix[next] = uint16(previous), first
			next++
		}
		previous = code
		limit := 1 << uint(width)
		if early {
			limit--
		}
		if next >= limit && width < 12 {
			width++
		}
	}
	return fmt.Errorf("016a: LZW missing end")
}

func menuDecodeShape(data []byte, width, height int, dst []byte, mask []bool) error {
	if len(data) < 10 || width <= 0 || height <= 0 || len(dst) != width*height || len(mask) != len(dst) {
		return fmt.Errorf("016a: shape buffers")
	}
	u16 := func(at int) int { return int(int16(binary.LittleEndian.Uint16(data[at:]))) }
	left, top := u16(2), u16(4)
	if left+u16(0)+1 != width || top+u16(6)+1 != height {
		return fmt.Errorf("016a: shape extent")
	}
	clear(dst)
	clear(mask)
	pos := 8
	put := func(x, y int, c byte) error {
		if x < 0 || x >= width || y < 0 || y >= height {
			return fmt.Errorf("016a: shape point bounds")
		}
		dst[y*width+x], mask[y*width+x] = c, true
		return nil
	}
	for {
		if len(data)-pos < 2 {
			return io.ErrUnexpectedEOF
		}
		code := int(binary.LittleEndian.Uint16(data[pos:]))
		pos += 2
		if code == 0 {
			if pos != len(data) {
				return fmt.Errorf("016a: shape trailing data")
			}
			return nil
		}
		if len(data)-pos < 4 {
			return io.ErrUnexpectedEOF
		}
		x, y, n := left+u16(pos), top+u16(pos+2), code>>1
		pos += 4
		if n == 0 || x < 0 || x+n > width || y < 0 || y >= height {
			return fmt.Errorf("016a: shape run bounds")
		}
		if code&1 == 0 {
			if n > len(data)-pos {
				return io.ErrUnexpectedEOF
			}
			for i := 0; i < n; i++ {
				if err := put(x+i, y, data[pos+i]); err != nil {
					return err
				}
			}
			pos += n
			continue
		}
		for done := 0; done < n; {
			if pos >= len(data) {
				return io.ErrUnexpectedEOF
			}
			sub := int(data[pos])
			pos++
			length := sub >> 1
			if length == 0 || length > n-done {
				return fmt.Errorf("016a: shape subrun bounds")
			}
			if sub&1 == 0 {
				if length > len(data)-pos {
					return io.ErrUnexpectedEOF
				}
				for i := 0; i < length; i++ {
					if err := put(x+done+i, y, data[pos+i]); err != nil {
						return err
					}
				}
				pos += length
			} else {
				if pos >= len(data) {
					return io.ErrUnexpectedEOF
				}
				for i := 0; i < length; i++ {
					if err := put(x+done+i, y, data[pos]); err != nil {
						return err
					}
				}
				pos++
			}
			done += length
		}
	}
}
