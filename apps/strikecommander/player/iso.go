package player

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var cueFile = regexp.MustCompile(`(?i)^FILE\s+"([^"]+)"\s+BINARY$`)
var cueTrack = regexp.MustCompile(`(?i)^TRACK\s+0?1\s+MODE1/(2352|2048)$`)
var cueIndex = regexp.MustCompile(`(?i)^INDEX\s+0?1\s+00:00:00$`)

func findFold(dir, name string) (string, error) {
	entries, e := os.ReadDir(dir)
	if e != nil {
		return "", e
	}
	var found []string
	for _, v := range entries {
		if strings.EqualFold(v.Name(), name) {
			found = append(found, v.Name())
		}
	}
	if len(found) != 1 {
		return "", fmt.Errorf("%s內的%s不存在或大小寫同名不唯一", dir, name)
	}
	p := filepath.Join(dir, found[0])
	s, e := os.Lstat(p)
	if e != nil || !s.Mode().IsRegular() {
		return "", fmt.Errorf("%s不是一般檔案", p)
	}
	return p, nil
}
func imageTarget(p string) (string, int, error) {
	ext := strings.ToLower(filepath.Ext(p))
	if ext != ".cue" && ext != ".ccd" {
		return p, 0, nil
	}
	f, e := os.Open(p)
	if e != nil {
		return "", 0, e
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	if ext == ".cue" {
		var file string
		mode, tracks, indexes, files := 0, 0, 0, 0
		for scan.Scan() {
			s := strings.TrimSpace(scan.Text())
			if s == "" {
				continue
			}
			if m := cueFile.FindStringSubmatch(s); m != nil {
				file = m[1]
				files++
				continue
			}
			if m := cueTrack.FindStringSubmatch(s); m != nil {
				mode, _ = strconv.Atoi(m[1])
				tracks++
				continue
			}
			if cueIndex.MatchString(s) {
				indexes++
				continue
			}
			return "", 0, fmt.Errorf("映像格式不支援：%s 的 %s", p, s)
		}
		if e := scan.Err(); e != nil {
			return "", 0, e
		}
		if files != 1 || tracks != 1 || indexes != 1 {
			return "", 0, errors.New("映像格式不支援：cue必須只有一個FILE、TRACK1及INDEX1")
		}
		if filepath.IsAbs(file) || filepath.Base(file) != file {
			return "", 0, errors.New("cue FILE必須是同目錄相對檔名")
		}
		q, e := findFold(filepath.Dir(p), file)
		return q, mode, e
	}
	tracks := 0
	inTrack := false
	modeOK := false
	for scan.Scan() {
		s := strings.TrimSpace(scan.Text())
		upper := strings.ToUpper(s)
		if strings.HasPrefix(upper, "[TRACK ") {
			tracks++
			inTrack = upper == "[TRACK 1]"
		}
		if inTrack && strings.HasPrefix(upper, "MODE=") {
			modeOK = strings.TrimSpace(strings.TrimPrefix(upper, "MODE=")) == "1"
		}
	}
	if scan.Err() != nil {
		return "", 0, scan.Err()
	}
	if tracks != 1 || !modeOK {
		return "", 0, errors.New("映像格式不支援：ccd必須只有TRACK1 MODE=1")
	}
	q, e := findFold(filepath.Dir(p), strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))+".img")
	return q, 2352, e
}

type readAtCloser interface {
	io.ReaderAt
	io.Closer
}
type ISOFile struct {
	Path           string
	Size           int64
	LBA            uint32
	image          string
	sector, offset int
	openImage      func() (readAtCloser, int64, error)
}
type isoReader struct {
	f    readAtCloser
	file ISOFile
	pos  int64
}

func (r *isoReader) Close() error { return r.f.Close() }
func (r *isoReader) Read(p []byte) (int, error) {
	if r.pos >= r.file.Size {
		return 0, io.EOF
	}
	n := 0
	for len(p) > 0 && r.pos < r.file.Size {
		within := r.pos % 2048
		k := min(int64(len(p)), 2048-within, r.file.Size-r.pos)
		off := (int64(r.file.LBA)+r.pos/2048)*int64(r.file.sector) + int64(r.file.offset) + within
		got, e := r.f.ReadAt(p[:k], off)
		n += got
		r.pos += int64(got)
		p = p[got:]
		if e != nil {
			return n, e
		}
		if got != int(k) {
			return n, io.ErrUnexpectedEOF
		}
	}
	return n, nil
}
func (x ISOFile) Open() (io.ReadCloser, error) {
	var f readAtCloser
	var e error
	if x.openImage != nil {
		f, _, e = x.openImage()
	} else {
		f, e = os.Open(x.image)
	}
	if e != nil {
		return nil, e
	}
	return &isoReader{f: f, file: x}, nil
}

func both32(b []byte) (uint32, error) {
	if len(b) < 8 {
		return 0, io.ErrUnexpectedEOF
	}
	n := binary.LittleEndian.Uint32(b)
	if n != binary.BigEndian.Uint32(b[4:]) {
		return 0, errors.New("ISO雙端序欄位不符")
	}
	return n, nil
}
func ListISO(p string) ([]ISOFile, error) {
	img, declared, e := imageTarget(p)
	if e != nil {
		return nil, e
	}
	return listISO(img, declared, func() (readAtCloser, int64, error) {
		f, e := os.Open(img)
		if e != nil {
			return nil, 0, e
		}
		st, e := f.Stat()
		if e != nil {
			f.Close()
			return nil, 0, e
		}
		return f, st.Size(), nil
	})
}
func listISO(img string, declared int, open func() (readAtCloser, int64, error)) ([]ISOFile, error) {
	f, size, e := open()
	if e != nil {
		return nil, e
	}
	defer f.Close()
	sector, offset := 0, 0
	for _, mode := range []int{2352, 2048} {
		b := make([]byte, 2048)
		off := 0
		if mode == 2352 {
			off = 16
			head := make([]byte, 16)
			if _, e = f.ReadAt(head, 16*int64(mode)); e != nil {
				continue
			}
			sync := []byte{0, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 0}
			if string(head[:12]) != string(sync) || head[15] != 1 {
				continue
			}
		}
		if _, e = f.ReadAt(b, 16*int64(mode)+int64(off)); e == nil && b[0] == 1 && string(b[1:6]) == "CD001" && b[6] == 1 {
			sector, offset = mode, off
			break
		}
	}
	if sector == 0 || size%int64(sector) != 0 {
		return nil, errors.New("映像損毀：sector sync、模式或大小不符")
	}
	if declared != 0 && declared != sector {
		return nil, errors.New("映像格式不支援：宣告MODE與實際sector不符")
	}
	pvd := make([]byte, 2048)
	if _, e = f.ReadAt(pvd, 16*int64(sector)+int64(offset)); e != nil {
		return nil, e
	}
	if binary.LittleEndian.Uint16(pvd[128:130]) != 2048 || binary.BigEndian.Uint16(pvd[130:132]) != 2048 {
		return nil, errors.New("ISO邏輯sector非2048")
	}
	type dir struct {
		prefix    string
		lba, size uint32
	}
	parse := func(b []byte) (uint32, uint32, error) {
		l, e := both32(b[2:10])
		if e != nil {
			return 0, 0, e
		}
		s, e := both32(b[10:18])
		if e != nil {
			return 0, 0, e
		}
		if (uint64(l)*2048+uint64(s)+2047)/2048 > uint64(size/int64(sector)) {
			return 0, 0, errors.New("映像損毀：extent超出檔案")
		}
		return l, s, nil
	}
	rootLen := int(pvd[156])
	if rootLen < 34 || 156+rootLen > len(pvd) {
		return nil, errors.New("ISO根目錄記錄損毀")
	}
	rl, rs, e := parse(pvd[156 : 156+rootLen])
	if e != nil {
		return nil, e
	}
	todo := []dir{{"", rl, rs}}
	seen := map[uint32]bool{}
	var out []ISOFile
	for len(todo) > 0 {
		d := todo[0]
		todo = todo[1:]
		if seen[d.lba] {
			return nil, errors.New("映像損毀：目錄extent循環或重複")
		}
		seen[d.lba] = true
		reader := &isoReader{f: f, file: ISOFile{Size: int64(d.size), LBA: d.lba, sector: sector, offset: offset}}
		buf := make([]byte, 2048)
		for position := uint32(0); position < d.size; {
			want := min(uint32(2048), d.size-position)
			if _, e = io.ReadFull(reader, buf[:want]); e != nil {
				return nil, e
			}
			for i := 0; i < int(want); {
				n := int(buf[i])
				if n == 0 {
					break
				}
				if n < 34 || i+n > int(want) {
					return nil, errors.New("映像損毀：目錄記錄長度")
				}
				b := buf[i : i+n]
				i += n
				if b[25]&128 != 0 {
					return nil, errors.New("映像格式不支援：多段extent")
				}
				ln := int(b[32])
				if 33+ln > len(b) {
					return nil, errors.New("映像損毀：檔名長度")
				}
				name := string(b[33 : 33+ln])
				if name == "\x00" || name == "\x01" {
					continue
				}
				if strings.ContainsAny(name, "/\\") || name == ".." {
					return nil, errors.New("映像損毀：目錄檔名")
				}
				name = strings.TrimSuffix(name, ";1")
				l, s, e := parse(b)
				if e != nil {
					return nil, e
				}
				full := name
				if d.prefix != "" {
					full = d.prefix + "/" + name
				}
				if b[25]&2 != 0 {
					todo = append(todo, dir{full, l, s})
				} else {
					out = append(out, ISOFile{full, int64(s), l, img, sector, offset, open})
				}
			}
			position += want
		}
	}
	return out, nil
}
func ImageFiles(p string) ([]SourceFile, error) {
	files, e := ListISO(p)
	if e != nil {
		return nil, e
	}
	out := make([]SourceFile, 0, len(files))
	for _, x := range files {
		x := x
		out = append(out, SourceFile{Source: p, Name: x.Path, Size: x.Size, Open: x.Open})
	}
	return out, nil
}
