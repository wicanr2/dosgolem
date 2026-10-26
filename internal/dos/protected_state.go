package dos

import (
	"fmt"
	"github.com/wicanr2/dosgolem/internal/fsaccess"
	"io"
	"os"
	"path/filepath"
)

func (d *DOS) stageSplitHandles(s dosState, original, save string, from []string) (staged map[uint16]*handle, err error) {
	staged = map[uint16]*handle{}
	defer func() {
		if err != nil {
			for _, h := range staged {
				h.f.Close()
			}
		}
	}()
	prefixes := append([]string{s.Root}, from...)
	for _, hs := range s.Handles {
		mapped, e := RebasePath(hs.Path, prefixes, save)
		if e != nil {
			return staged, e
		}
		rel, e := filepath.Rel(save, mapped)
		if e != nil {
			return staged, e
		}
		p, e := fsaccess.Resolve(save, rel, false)
		if os.IsNotExist(e) {
			p, e = fsaccess.Resolve(original, rel, false)
		}
		if e != nil {
			return staged, e
		}
		st, e := os.Lstat(p)
		if e != nil {
			return staged, e
		}
		if !st.Mode().IsRegular() || (hs.Size != 0 && st.Size() != hs.Size) {
			return staged, fmt.Errorf("roots: state file type/size mismatch")
		}
		f, e := os.Open(p)
		if e != nil {
			return staged, e
		}
		if _, e = f.Seek(hs.Off, io.SeekStart); e != nil {
			f.Close()
			return staged, e
		}
		if _, duplicate := staged[hs.H]; duplicate {
			f.Close()
			return staged, fmt.Errorf("roots: duplicate state handle")
		}
		staged[hs.H] = &handle{name: hs.Name, path: p, f: f, size: hs.Size, psp: hs.PSP, refs: 1}
	}
	return staged, nil
}
