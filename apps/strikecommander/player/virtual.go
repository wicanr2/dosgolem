package player

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strconv"
	"strings"
)

type VirtualSource struct {
	FS          fs.FS
	Root, Label string
}
type streamAt struct {
	fsys     fs.FS
	name     string
	f        fs.File
	position int64
}

func (s *streamAt) Close() error { return s.f.Close() }
func (s *streamAt) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, fmt.Errorf("negative image offset")
	}
	if r, ok := s.f.(io.ReaderAt); ok {
		return r.ReadAt(p, off)
	}
	if r, ok := s.f.(io.Seeker); ok {
		if _, e := r.Seek(off, io.SeekStart); e != nil {
			return 0, e
		}
		n, e := io.ReadFull(s.f, p)
		return n, e
	}
	if off < s.position {
		if e := s.f.Close(); e != nil {
			return 0, e
		}
		f, e := s.fsys.Open(s.name)
		if e != nil {
			return 0, e
		}
		s.f = f
		s.position = 0
	}
	if off > s.position {
		n, e := io.CopyN(io.Discard, s.f, off-s.position)
		s.position += n
		if e != nil {
			return 0, e
		}
	}
	n, e := io.ReadFull(s.f, p)
	s.position += int64(n)
	return n, e
}
func virtualOpen(fsys fs.FS, n string) (readAtCloser, int64, error) {
	f, e := fsys.Open(n)
	if e != nil {
		return nil, 0, e
	}
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() {
		f.Close()
		return nil, 0, fmt.Errorf("不是一般來源檔：%s", n)
	}
	return &streamAt{fsys: fsys, name: n, f: f}, st.Size(), nil
}
func findFoldFS(fsys fs.FS, dir, name string) (string, error) {
	entries, e := fs.ReadDir(fsys, dir)
	if e != nil {
		return "", e
	}
	var names []string
	for _, x := range entries {
		if strings.EqualFold(x.Name(), name) {
			names = append(names, x.Name())
		}
	}
	if len(names) != 1 {
		return "", fmt.Errorf("%s內的%s不存在或大小寫同名不唯一", dir, name)
	}
	return path.Join(dir, names[0]), nil
}
func imageTargetFS(fsys fs.FS, n string) (string, int, error) {
	ext := strings.ToLower(path.Ext(n))
	if ext != ".cue" && ext != ".ccd" {
		return n, 0, nil
	}
	f, e := fsys.Open(n)
	if e != nil {
		return "", 0, e
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	if ext == ".ccd" {
		tracks := 0
		current, valid := false, false
		for scan.Scan() {
			s := strings.ToUpper(strings.TrimSpace(scan.Text()))
			if strings.HasPrefix(s, "[TRACK ") {
				tracks++
				current = s == "[TRACK 1]"
			}
			if current && strings.HasPrefix(s, "MODE=") {
				valid = strings.TrimSpace(strings.TrimPrefix(s, "MODE=")) == "1"
			}
		}
		if e = scan.Err(); e != nil {
			return "", 0, e
		}
		if tracks != 1 || !valid {
			return "", 0, fmt.Errorf("ccd必須只有TRACK1 MODE=1")
		}
		p, e := findFoldFS(fsys, path.Dir(n), strings.TrimSuffix(path.Base(n), path.Ext(n))+".img")
		return p, 2352, e
	}
	file := ""
	files, tracks, indexes, mode := 0, 0, 0, 0
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
		return "", 0, fmt.Errorf("映像格式不支援：%s", s)
	}
	if scan.Err() != nil {
		return "", 0, scan.Err()
	}
	if files != 1 || tracks != 1 || indexes != 1 {
		return "", 0, fmt.Errorf("cue必須只有FILE/TRACK1/INDEX1")
	}
	if !fs.ValidPath(file) || strings.ContainsAny(file, "\\:") {
		return "", 0, fmt.Errorf("cue FILE不是相對路徑")
	}
	dir := path.Dir(n)
	parts := strings.Split(file, "/")
	for i, part := range parts {
		p, e := findFoldFS(fsys, dir, part)
		if e != nil {
			return "", 0, e
		}
		if i == len(parts)-1 {
			return p, mode, nil
		}
		dir = p
	}
	return "", 0, fmt.Errorf("empty FILE")
}
func virtualImage(fsys fs.FS, n, label string) ([]SourceFile, error) {
	img, mode, e := imageTargetFS(fsys, n)
	if e != nil {
		return nil, e
	}
	open := func() (readAtCloser, int64, error) { return virtualOpen(fsys, img) }
	files, e := listISO(label+"/"+img, mode, open)
	if e != nil {
		return nil, e
	}
	var out []SourceFile
	for _, x := range files {
		x := x
		out = append(out, SourceFile{Source: label, Name: x.Path, Size: x.Size, Open: x.Open})
	}
	return out, nil
}
func virtualFiles(v VirtualSource) ([]SourceFile, error) {
	root := v.Root
	if root == "" {
		root = "."
	}
	st, e := fs.Stat(v.FS, root)
	if e != nil {
		return nil, e
	}
	if st.Mode().IsRegular() {
		if imageExt(root) {
			return virtualImage(v.FS, root, v.Label)
		}
		return []SourceFile{{Source: v.Label, Name: root, Size: st.Size(), Open: func() (io.ReadCloser, error) { return v.FS.Open(root) }}}, nil
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("來源不是一般檔或目錄")
	}
	dirs := []string{root}
	entries, e := fs.ReadDir(v.FS, root)
	if e != nil {
		return nil, e
	}
	for _, x := range entries {
		if x.IsDir() {
			dirs = append(dirs, path.Join(root, x.Name()))
		}
	}
	var out []SourceFile
	for _, d := range dirs {
		entries, e := fs.ReadDir(v.FS, d)
		if e != nil {
			return nil, e
		}
		for _, x := range entries {
			if x.IsDir() || x.Type()&fs.ModeSymlink != 0 {
				continue
			}
			n := path.Join(d, x.Name())
			if imageExt(n) {
				files, e := virtualImage(v.FS, n, v.Label)
				if e != nil {
					return nil, e
				}
				out = append(out, files...)
				continue
			}
			st, e := x.Info()
			if e != nil {
				return nil, e
			}
			if !st.Mode().IsRegular() {
				continue
			}
			n2 := n
			out = append(out, SourceFile{Source: v.Label, Name: n, Size: st.Size(), Open: func() (io.ReadCloser, error) { return v.FS.Open(n2) }})
		}
	}
	return out, nil
}
func LocateWithVirtual(ctx context.Context, native []string, virtual []VirtualSource, specs []FileSpec, progress func(string)) (Selection, error) {
	// Native results may be incomplete; merge all actual bytes by pinned hash.
	selection, _ := Locate(ctx, native, specs, progress)
	if selection.Files == nil {
		selection.Files = map[string]SourceFile{}
	}
	sizes := map[int64][]FileSpec{}
	for _, s := range specs {
		if s.Generated == "" {
			sizes[s.Size] = append(sizes[s.Size], s)
		}
	}
	for _, v := range virtual {
		selection.Sources = append(selection.Sources, v.Label)
		files, e := virtualFiles(v)
		if e != nil {
			return selection, e
		}
		for _, f := range files {
			if e = ctx.Err(); e != nil {
				return selection, e
			}
			candidates := sizes[f.Size]
			if len(candidates) == 0 {
				continue
			}
			r, e := f.Open()
			if e != nil {
				return selection, e
			}
			h, e := fileHash(r)
			r.Close()
			if e != nil {
				return selection, e
			}
			for _, s := range candidates {
				if s.SHA256 == h {
					if _, ok := selection.Files[s.Name]; !ok {
						selection.Files[s.Name] = f
					}
				}
			}
		}
	}
	var missing []string
	for _, s := range specs {
		if s.Generated == "" {
			if _, ok := selection.Files[s.Name]; !ok {
				missing = append(missing, s.Name)
			}
		}
	}
	if len(missing) > 0 {
		return selection, fmt.Errorf("缺必要檔：%s", strings.Join(missing, "、"))
	}
	return selection, nil
}
