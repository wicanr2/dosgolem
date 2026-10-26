package player

import (
	"bytes"
	"encoding/binary"
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func fixtureISO() []byte {
	b := make([]byte, 24*2048)
	p := b[16*2048 : 17*2048]
	copy(p, []byte{1, 'C', 'D', '0', '0', '1', 1})
	binary.LittleEndian.PutUint16(p[128:], 2048)
	binary.BigEndian.PutUint16(p[130:], 2048)
	rec := func(name []byte, lba, size uint32, flags byte) []byte {
		n := 33 + len(name)
		if n%2 != 0 {
			n++
		}
		r := make([]byte, n)
		r[0] = byte(n)
		binary.LittleEndian.PutUint32(r[2:], lba)
		binary.BigEndian.PutUint32(r[6:], lba)
		binary.LittleEndian.PutUint32(r[10:], size)
		binary.BigEndian.PutUint32(r[14:], size)
		r[25] = flags
		r[32] = byte(len(name))
		copy(r[33:], name)
		return r
	}
	root := rec([]byte{0}, 20, 2048, 2)
	copy(p[156:], root)
	at := 20 * 2048
	for _, r := range [][]byte{root, rec([]byte{1}, 20, 2048, 2), rec([]byte("MiXeD.DAT;1"), 21, 5, 0)} {
		copy(b[at:], r)
		at += len(r)
	}
	copy(b[21*2048:], "hello")
	return b
}
func TestISOFormats(t *testing.T) {
	dir := t.TempDir()
	iso := fixtureISO()
	raw := make([]byte, len(iso)/2048*2352)
	for i := 0; i < len(iso)/2048; i++ {
		s := raw[i*2352:]
		copy(s, []byte{0, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 0})
		s[15] = 1
		copy(s[16:], iso[i*2048:(i+1)*2048])
	}
	for _, tc := range []struct {
		name string
		b    []byte
	}{{"a.iso", iso}, {"b.bin", raw}} {
		p := filepath.Join(dir, tc.name)
		os.WriteFile(p, tc.b, 0600)
		files, e := ListISO(p)
		if e != nil || len(files) != 1 || files[0].Path != "MiXeD.DAT" || files[0].Size != 5 {
			t.Fatal(files, e)
		}
		r, e := files[0].Open()
		if e != nil {
			t.Fatal(e)
		}
		b, e := io.ReadAll(r)
		r.Close()
		if e != nil || string(b) != "hello" {
			t.Fatal(string(b), e)
		}
	}
	cue := filepath.Join(dir, "disk.cue")
	for _, tc := range []struct {
		text string
		pass bool
	}{{"file \"B.BIN\" binary\ntrack 01 mode1/2352\nindex 1 00:00:00\n", true}, {"FILE \"b.bin\" BINARY\nTRACK 1 MODE1/2048\nINDEX 01 00:00:00\n", false}, {"FILE \"b.bin\" BINARY\nTRACK 1 MODE1/2352\nPREGAP 00:02:00\nINDEX 1 00:00:00\n", false}, {"FILE \"b.bin\" BINARY\nTRACK 1 MODE1/2352\nINDEX 0 00:00:00\n", false}} {
		os.WriteFile(cue, []byte(tc.text), 0600)
		_, e := ListISO(cue)
		if (e == nil) != tc.pass {
			t.Fatal(tc.text, e)
		}
	}
	bad := append([]byte(nil), raw...)
	bad[16*2352] = 1
	os.WriteFile(filepath.Join(dir, "bad.bin"), bad, 0600)
	if _, e := ListISO(filepath.Join(dir, "bad.bin")); e == nil {
		t.Fatal("bad sync accepted")
	}
	bad = append([]byte(nil), iso...)
	bad[20*2048+34+34+25] = 128
	os.WriteFile(filepath.Join(dir, "multi.iso"), bad, 0600)
	if _, e := ListISO(filepath.Join(dir, "multi.iso")); e == nil {
		t.Fatal("multi extent accepted")
	}
}
func TestCDImageIndependentManifest(t *testing.T) {
	p := os.Getenv("SC_PLAYER_IMAGE")
	manifest := os.Getenv("SC_PLAYER_CD_MANIFEST")
	if p == "" || manifest == "" {
		t.Skip("original RO image not selected")
	}
	files, e := ListISO(p)
	if e != nil {
		t.Fatal(e)
	}
	expected := map[string][]string{}
	b, e := os.ReadFile(manifest)
	if e != nil {
		t.Fatal(e)
	}
	r := csv.NewReader(bytes.NewReader(b))
	r.Comma = '\t'
	r.Comment = '#'
	r.FieldsPerRecord = 4
	for {
		row, e := r.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		expected[row[0]] = row
	}
	if len(files) != len(expected) {
		t.Fatalf("files%d expected%d", len(files), len(expected))
	}
	for _, f := range files {
		row, ok := expected[f.Path]
		if !ok {
			t.Fatal(f.Path)
		}
		size, _ := strconv.ParseInt(row[1], 10, 64)
		lba, _ := strconv.ParseUint(row[2], 10, 32)
		if f.Size != size || f.LBA != uint32(lba) {
			t.Fatal(f.Path, "size/lba")
		}
		r, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		h, e := fileHash(r)
		r.Close()
		if e != nil || h != row[3] {
			t.Fatal(f.Path, h, e)
		}
	}
}
func TestFixedWallDates(t *testing.T) {
	for _, zone := range []string{"UTC", "Asia/Taipei", "America/Sao_Paulo"} {
		loc, e := time.LoadLocation(zone)
		if e != nil {
			t.Fatal(e)
		}
		for _, s := range RequiredFiles {
			v := s.Wall.Time(loc).In(loc)
			if v.Year() != 1995 || int(v.Month()) != 2 || v.Day() != 4 || v.Hour() != s.Wall.Hour || v.Minute() != s.Wall.Minute || v.Second() != s.Wall.Second {
				t.Fatal(zone, s.Name, v)
			}
		}
	}
}
func TestCueAmbiguousName(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "a.bin"), fixtureISO(), 0600)
	os.WriteFile(filepath.Join(d, "A.BIN"), fixtureISO(), 0600)
	cue := filepath.Join(d, "a.cue")
	os.WriteFile(cue, []byte("FILE \"a.bin\" BINARY\nTRACK 1 MODE1/2048\nINDEX 1 00:00:00\n"), 0600)
	if _, e := ListISO(cue); e == nil || !strings.Contains(e.Error(), "不唯一") {
		t.Fatal(e)
	}
}
