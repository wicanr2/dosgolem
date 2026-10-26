package player

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type WallTime struct{ Year, Month, Day, Hour, Minute, Second int }

func (w WallTime) Time(loc *time.Location) time.Time {
	return time.Date(w.Year, time.Month(w.Month), w.Day, w.Hour, w.Minute, w.Second, 0, loc)
}

type FileSpec struct {
	Name      string
	Size      int64
	SHA256    string
	Wall      WallTime
	Generated string
}
type SourceFile struct {
	Source, Name string
	Size         int64
	Open         func() (io.ReadCloser, error)
}
type Selection struct {
	Files                map[string]SourceFile
	Sources, Diagnostics []string
}

func fileHash(r io.Reader) (string, error) {
	h := sha256.New()
	_, e := io.Copy(h, r)
	return hex.EncodeToString(h.Sum(nil)), e
}
func hashPath(p string) (string, error) {
	f, e := os.Open(p)
	if e != nil {
		return "", e
	}
	defer f.Close()
	return fileHash(f)
}
func regularSource(p string) (SourceFile, error) {
	st, e := os.Lstat(p)
	if e != nil {
		return SourceFile{}, e
	}
	if !st.Mode().IsRegular() {
		return SourceFile{}, fmt.Errorf("來源不是一般檔案：%s", p)
	}
	return SourceFile{Source: p, Name: filepath.Base(p), Size: st.Size(), Open: func() (io.ReadCloser, error) { return os.Open(p) }}, nil
}
func imageExt(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".cue", ".ccd", ".iso", ".img", ".bin":
		return true
	}
	return false
}
func sourceFiles(p string) ([]SourceFile, error) {
	st, e := os.Lstat(p)
	if e != nil {
		return nil, e
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("來源不可是symlink：%s", p)
	}
	if st.Mode().IsRegular() {
		if !imageExt(p) {
			f, e := regularSource(p)
			return []SourceFile{f}, e
		}
		return ImageFiles(p)
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("來源不是目錄或映像：%s", p)
	}
	var result []SourceFile
	var images []string
	dirs := []string{p}
	entries, e := os.ReadDir(p)
	if e != nil {
		return nil, e
	}
	for _, v := range entries {
		if v.IsDir() {
			dirs = append(dirs, filepath.Join(p, v.Name()))
		}
	}
	for _, dir := range dirs {
		entries, e := os.ReadDir(dir)
		if e != nil {
			return nil, e
		}
		for _, v := range entries {
			if v.Type()&os.ModeSymlink != 0 || v.IsDir() {
				continue
			}
			n := filepath.Join(dir, v.Name())
			st, e := v.Info()
			if e != nil {
				return nil, e
			}
			if !st.Mode().IsRegular() {
				continue
			}
			if imageExt(n) {
				images = append(images, n)
			} else {
				f, e := regularSource(n)
				if e != nil {
					return nil, e
				}
				result = append(result, f)
			}
		}
	}
	sort.Strings(images)
	for _, n := range images {
		f, e := ImageFiles(n)
		if e != nil {
			return nil, e
		}
		result = append(result, f...)
	}
	return result, nil
}

// Locate scans only explicit input locations. It never creates or modifies sources.
func Locate(ctx context.Context, sources []string, specs []FileSpec, progress func(string)) (Selection, error) {
	out := Selection{Files: map[string]SourceFile{}, Sources: nil}
	sizes := map[int64][]FileSpec{}
	for _, s := range specs {
		if s.Generated == "" {
			sizes[s.Size] = append(sizes[s.Size], s)
		}
	}
	var badStrike []string
	for _, p := range sources {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		files, e := sourceFiles(p)
		if e != nil {
			out.Diagnostics = append(out.Diagnostics, fmt.Sprintf("%s：%v", p, e))
			continue
		}
		used := false
		for _, f := range files {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			candidates := sizes[f.Size]
			if len(candidates) == 0 {
				continue
			}
			if progress != nil {
				progress(f.Source + " / " + f.Name)
			}
			r, e := f.Open()
			if e != nil {
				return out, e
			}
			h, e := fileHash(r)
			ce := r.Close()
			if e == nil {
				e = ce
			}
			if e != nil {
				return out, e
			}
			matched := false
			for _, s := range candidates {
				if h == s.SHA256 {
					matched = true
					if _, ok := out.Files[s.Name]; !ok {
						out.Files[s.Name] = f
						used = true
					}
				}
			}
			if !matched {
				for _, s := range candidates {
					if s.Name == "STRIKE.EXE" {
						badStrike = append(badStrike, fmt.Sprintf("%s/%s 實際SHA256 %s，期望 %s", f.Source, f.Name, h, s.SHA256))
					}
				}
			}
		}

		if used {
			out.Sources = append(out.Sources, p)
		}
	}

	var missing []string
	for _, s := range specs {
		if s.Generated == "" {
			if _, ok := out.Files[s.Name]; !ok {
				missing = append(missing, s.Name)
			}
		}
	}
	if len(missing) > 0 {
		reason := "缺必要檔：" + strings.Join(missing, "、") + "。安裝目錄沒有光碟上的資料檔，請一併提供光碟映像。"
		if _, ok := out.Files["STRIKE.EXE"]; !ok && len(badStrike) > 0 {
			reason += "版本不符；已知修補版STRIKE.EXE不受支援，請改提供光碟映像。" + strings.Join(badStrike, "；")
		}
		return out, fmt.Errorf("%s\n找過的位置：%s\n%s", reason, strings.Join(sources, "、"), strings.Join(out.Diagnostics, "\n"))
	}
	return out, nil
}

// DropPaths is deliberately filesystem-only; the window adapter passes the
// virtual root names before attempting os.File.Name() recovery.
func ValidateDropNames(names []string) error {
	seen := map[string]bool{}
	for _, n := range names {
		b := filepath.Base(n)
		if seen[b] {
			return fmt.Errorf("拖入項目有重複basename %s，請分次拖入", b)
		}
		seen[b] = true
	}
	return nil
}
