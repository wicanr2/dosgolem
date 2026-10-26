package dos

import (
	"fmt"
	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/fsaccess"
	"os"
	"path/filepath"
)

// SetOriginalSaveRoots enables 463 after the session has validated the pair.
func (d *DOS) SetOriginalSaveRoots(original, save string) {
	d.Root = original
	d.Scratch = save
	d.protectedRoots = true
}

func protectedLookup(root string, segs []string, short bool) (string, error) {
	if len(segs) == 0 {
		return "", os.ErrNotExist
	}
	p, e := fsaccess.Resolve(root, filepath.Join(segs...), false)
	if os.IsNotExist(e) && short {
		last := segs[len(segs)-1]
		sn := dosName(last)
		if sn != last {
			ss := append([]string(nil), segs...)
			ss[len(ss)-1] = sn
			p, e = fsaccess.Resolve(root, filepath.Join(ss...), false)
		}
	}
	if e != nil {
		return "", e
	}
	st, e := os.Lstat(p)
	if e != nil {
		return "", e
	}
	if !st.Mode().IsRegular() {
		return "", fmt.Errorf("roots: read target not regular")
	}
	return p, nil
}
func (d *DOS) protectedResolve(name string) (string, error) {
	dirs, file := d.relComponents(name)
	candidates := [][]string{}
	if len(dirs) > 0 && file != "" {
		candidates = append(candidates, append(append([]string(nil), dirs...), file))
	}
	if segs := dosPathSegments(name); len(segs) > 1 {
		candidates = append(candidates, segs)
	}
	candidates = append(candidates, []string{baseName(name)})
	for _, ss := range candidates {
		for _, r := range []string{d.Scratch, d.Root} {
			p, e := protectedLookup(r, ss, true)
			if e == nil {
				return p, nil
			}
			if !os.IsNotExist(e) {
				return "", e
			}
		}
	}
	return "", os.ErrNotExist
}
func (d *DOS) protectedDirsExist(dirs []string) (bool, error) {
	if len(dirs) == 0 {
		return false, nil
	}
	for _, r := range []string{d.Scratch, d.Root} {
		p, e := fsaccess.Resolve(r, filepath.Join(dirs...), false)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return false, e
		}
		st, e := os.Stat(p)
		if e != nil {
			return false, e
		}
		if !st.IsDir() {
			return false, fmt.Errorf("roots: not directory")
		}
		return true, nil
	}
	return false, nil
}
func (d *DOS) protectedTarget(name string, directory, makeParents bool) (string, error) {
	dirs, file := d.relComponents(name)
	if directory {
		rel := d.underRoot(d.joinDir(name))
		if len(rel) == 0 {
			return "", fmt.Errorf("roots: mutation of root")
		}
		dirs, file = rel[:len(rel)-1], rel[len(rel)-1]
	}
	exists, e := d.protectedDirsExist(dirs)
	if e != nil {
		return "", e
	}
	if !exists {
		dirs = nil
		if !directory {
			file = baseName(name)
		}
	}
	rel := filepath.Join(append(append([]string(nil), dirs...), file)...)
	p, e := fsaccess.Resolve(d.Scratch, rel, true)
	if e != nil {
		return "", e
	}
	if e = fsaccess.Mutation(d.Scratch, p, directory); e != nil {
		return "", e
	}
	if makeParents {
		if e = os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			return "", e
		}
	}
	return p, nil
}
func (d *DOS) protectedRename(c *cpu.CPU, oldName, newName string) {
	src, e := d.protectedResolve(oldName)
	if e != nil {
		if os.IsNotExist(e) {
			d.fail(c, 2)
		} else {
			d.fail(c, 5)
		}
		return
	}
	inSave := fsaccess.Within(d.Scratch, src)
	if inSave {
		if e = fsaccess.Mutation(d.Scratch, src, false); e != nil {
			d.fail(c, 5)
			return
		}
	}
	dst, e := d.protectedTarget(newName, false, false)
	if e != nil {
		d.fail(c, 5)
		return
	}
	if e = os.MkdirAll(filepath.Dir(dst), 0755); e != nil {
		d.fail(c, 5)
		return
	}
	if inSave {
		e = os.Rename(src, dst)
	} else {
		var data []byte
		data, e = os.ReadFile(src)
		if e == nil {
			e = os.WriteFile(dst, data, 0644)
			if e == nil {
				d.stampScratch(dst)
			}
		}
	}
	if e != nil {
		d.fail(c, 5)
		return
	}
	d.trace(FileOp{Op: "rename", Fn: 0x56, Name: oldName})
	clearCarry(c)
}
