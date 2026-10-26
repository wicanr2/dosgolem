package player

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ManifestFile struct {
	Name   string
	Size   int64
	SHA256 string
	Wall   WallTime
}
type Manifest struct {
	Version int
	Files   []ManifestFile
	Sources []string
}
type Cache struct {
	Resolve  func(context.Context) (Selection, error)
	Root     string
	Specs    []FileSpec
	Progress func(string)
	Warn     func(string)
	Rename   func(string, string) error
	Sleep    func(time.Duration)
}

func (c Cache) specs() []FileSpec {
	if c.Specs != nil {
		return c.Specs
	}
	return RequiredFiles
}
func systemFile(n string) bool {
	return n == ".DS_Store" || strings.EqualFold(n, "Thumbs.db") || strings.EqualFold(n, "desktop.ini") || strings.HasPrefix(n, "._")
}
func (c Cache) tree() string { return filepath.Join(c.Root, "tree-v1") }
func (c Cache) rename(a, b string) error {
	rename := c.Rename
	if rename == nil {
		rename = os.Rename
	}
	sleep := c.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	e := rename(a, b)
	for _, d := range []time.Duration{100, 200, 400, 800, 1600} {
		if e == nil || !os.IsPermission(e) {
			break
		}
		sleep(d * time.Millisecond)
		e = rename(a, b)
	}
	return e
}
func (c Cache) Verify() error { return c.verifyAt(c.tree(), filepath.Join(c.Root, "tree-v1.manifest")) }
func (c Cache) verifyAt(tree, manifest string) error {
	b, e := os.ReadFile(manifest)
	if e != nil {
		return e
	}
	var m Manifest
	if e = json.Unmarshal(b, &m); e != nil {
		return e
	}
	specs := c.specs()
	if m.Version != 1 || len(m.Files) != len(specs) {
		return fmt.Errorf("快取manifest格式不符")
	}
	expected := map[string]FileSpec{}
	for _, s := range specs {
		expected[s.Name] = s
	}
	seen := map[string]bool{}
	for _, f := range m.Files {
		s, ok := expected[f.Name]
		if !ok || seen[f.Name] || f.Size != s.Size || f.SHA256 != s.SHA256 || f.Wall != s.Wall {
			return fmt.Errorf("快取manifest固定來源不符：%s", f.Name)
		}
		seen[f.Name] = true
		p := filepath.Join(tree, f.Name)
		st, e := os.Lstat(p)
		if e != nil {
			return e
		}
		if !st.Mode().IsRegular() || st.Size() != s.Size || !st.ModTime().Equal(s.Wall.Time(time.Local)) {
			return fmt.Errorf("快取大小／mtime不符：%s", p)
		}
		h, e := hashPath(p)
		if e != nil {
			return e
		}
		if h != s.SHA256 {
			return fmt.Errorf("快取SHA256不符：%s", p)
		}
	}
	entries, e := os.ReadDir(tree)
	if e != nil {
		return e
	}
	for _, v := range entries {
		if !seen[v.Name()] && !systemFile(v.Name()) {
			return fmt.Errorf("快取多出項目：%s", v.Name())
		}
	}
	return nil
}

// Recover runs while the caller holds the process lock. Old trees are restored
// before any residue is removed; an invalid active tree never destroys a valid old one.
func (c Cache) Recover() error {
	if e := os.MkdirAll(c.Root, 0755); e != nil {
		return e
	}
	entries, e := os.ReadDir(c.Root)
	if e != nil {
		return e
	}
	var old []string
	for _, x := range entries {
		if strings.HasPrefix(x.Name(), "tree-v1.old-") {
			old = append(old, filepath.Join(c.Root, x.Name()))
		}
	}
	sort.Strings(old)
	_, te := os.Lstat(c.tree())
	if os.IsNotExist(te) && len(old) > 0 {
		if e = c.rename(old[len(old)-1], c.tree()); e != nil {
			return e
		}
	}
	// If a crash left new tree plus old manifest, rollback only to an old tree
	// that is demonstrably valid under that fixed manifest.
	if c.Verify() != nil {
		for i := len(old) - 1; i >= 0; i-- {
			if c.verifyAt(old[i], filepath.Join(c.Root, "tree-v1.manifest")) == nil {
				bad := filepath.Join(c.Root, fmt.Sprintf("tree-v1.importing-recovery-%d", os.Getpid()))
				if _, e = os.Lstat(c.tree()); e == nil {
					if e = c.rename(c.tree(), bad); e != nil {
						return e
					}
				}
				if e = c.rename(old[i], c.tree()); e != nil {
					return e
				}
				break
			}
		}
	}
	entries, e = os.ReadDir(c.Root)
	if e != nil {
		return e
	}
	for _, x := range entries {
		if strings.HasPrefix(x.Name(), "tree-v1.importing-") || strings.HasPrefix(x.Name(), "tree-v1.old-") {
			if e = os.RemoveAll(filepath.Join(c.Root, x.Name())); e != nil {
				return e
			}
		}
	}
	return nil
}
func (c Cache) Ensure(ctx context.Context, sources []string, save string, reimport bool) (string, []string, error) {
	if e := CheckIndependentPaths(c.Root, save, sources); e != nil {
		return "", nil, e
	}
	if e := c.Recover(); e != nil {
		return "", nil, e
	}
	if !reimport && c.Verify() == nil {
		return c.tree(), nil, nil
	}
	var selection Selection
	var e error
	if c.Resolve != nil {
		selection, e = c.Resolve(ctx)
	} else {
		selection, e = Locate(ctx, sources, c.specs(), c.Progress)
	}
	if e != nil {
		return "", selection.Diagnostics, e
	}
	temp, e := os.MkdirTemp(c.Root, "tree-v1.importing-")
	if e != nil {
		return "", nil, e
	}
	defer os.RemoveAll(temp)
	manifest := Manifest{Version: 1, Sources: append([]string(nil), selection.Sources...)}
	for _, s := range c.specs() {
		if e = ctx.Err(); e != nil {
			return "", nil, e
		}
		p := filepath.Join(temp, s.Name)
		if c.Progress != nil {
			c.Progress("匯入 " + s.Name)
		}
		var r io.ReadCloser
		if s.Generated != "" {
			r = io.NopCloser(strings.NewReader(s.Generated))
		} else {
			r, e = selection.Files[s.Name].Open()
			if e != nil {
				return "", nil, e
			}
		}
		f, e := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			r.Close()
			return "", nil, e
		}
		_, e = io.Copy(f, r)
		rc := r.Close()
		if e == nil {
			e = rc
		}
		if e == nil {
			e = f.Sync()
		}
		fc := f.Close()
		if e == nil {
			e = fc
		}
		if e != nil {
			return "", nil, e
		}
		h, e := hashPath(p)
		if e != nil {
			return "", nil, e
		}
		st, e := os.Stat(p)
		if e != nil {
			return "", nil, e
		}
		if h != s.SHA256 || st.Size() != s.Size {
			return "", nil, fmt.Errorf("匯入後來源變動或雜湊不符：%s", s.Name)
		}
		mt := s.Wall.Time(time.Local)
		if e = os.Chtimes(p, mt, mt); e != nil {
			return "", nil, e
		}
		if e = os.Chmod(p, 0444); e != nil {
			return "", nil, e
		}
		if back := mt.In(time.Local); back.Year() != s.Wall.Year || int(back.Month()) != s.Wall.Month || back.Day() != s.Wall.Day || back.Hour() != s.Wall.Hour || back.Minute() != s.Wall.Minute || back.Second() != s.Wall.Second {
			if c.Warn != nil {
				c.Warn("夏令時缺口已正規化：" + s.Name)
			}
		}
		manifest.Files = append(manifest.Files, ManifestFile{s.Name, s.Size, s.SHA256, s.Wall})
	}
	mb, e := json.MarshalIndent(manifest, "", "  ")
	if e != nil {
		return "", nil, e
	}
	mf := temp + ".manifest"
	defer os.Remove(mf)
	if e = os.WriteFile(mf, append(mb, '\n'), 0600); e != nil {
		return "", nil, e
	}
	if e = c.verifyAt(temp, mf); e != nil {
		return "", nil, e
	}
	old := filepath.Join(c.Root, fmt.Sprintf("tree-v1.old-%d", os.Getpid()))
	hadOld := false
	if _, e = os.Lstat(c.tree()); e == nil {
		if e = c.rename(c.tree(), old); e != nil {
			return "", nil, e
		}
		hadOld = true
	} else if !os.IsNotExist(e) {
		return "", nil, e
	}
	if e = c.rename(temp, c.tree()); e != nil {
		if hadOld {
			_ = c.rename(old, c.tree())
		}
		return "", nil, e
	}
	if e = c.rename(mf, filepath.Join(c.Root, "tree-v1.manifest")); e != nil {
		_ = c.rename(c.tree(), temp)
		if hadOld {
			_ = c.rename(old, c.tree())
		}
		return "", nil, e
	}
	if hadOld {
		if e = os.RemoveAll(old); e != nil && c.Warn != nil {
			c.Warn(e.Error())
		}
	}
	return c.tree(), selection.Diagnostics, nil
}

// CheckSaveShadows inspects names and hashes only. It never changes saves.
func CheckSaveShadows(save, tree string, specs []FileSpec) ([]string, error) {
	if e := os.MkdirAll(save, 0755); e != nil {
		return nil, e
	}
	st, e := os.Lstat(save)
	if e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("存檔層不可用或為symlink：%s", save)
	}
	entries, e := os.ReadDir(save)
	if e != nil {
		return nil, e
	}
	var warnings []string
	for _, x := range entries {
		var matches []FileSpec
		for _, s := range specs {
			if strings.EqualFold(x.Name(), s.Name) {
				matches = append(matches, s)
			}
		}
		if len(matches) != 0 {
			p := filepath.Join(save, x.Name())
			if !x.Type().IsRegular() {
				return nil, fmt.Errorf("存檔層原版同名項目不是一般檔：%s", p)
			}
			h, e := hashPath(p)
			if e != nil {
				return nil, e
			}
			for _, s := range matches {
				if h != s.SHA256 {
					return nil, fmt.Errorf("存檔層的%s會蓋掉原版資料，請把此檔移出存檔層", p)
				}
			}
			warnings = append(warnings, "存檔層同名檔與原版相同："+p)
		}
	}
	return warnings, nil
}

func (c Cache) UsedSources() ([]string, error) {
	b, e := os.ReadFile(filepath.Join(c.Root, "tree-v1.manifest"))
	if e != nil {
		return nil, e
	}
	var m Manifest
	if e = json.Unmarshal(b, &m); e != nil {
		return nil, e
	}
	return append([]string(nil), m.Sources...), nil
}
