package fsaccess

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RootPair validates without writes. Writable must be checked only after this succeeds.
func RootPair(original, save string) (string, string, error) {
	roots := []string{original, save}
	infos := make([]os.FileInfo, 2)
	for i, r := range roots {
		if r == "" || strings.IndexByte(r, 0) >= 0 {
			return "", "", fmt.Errorf("roots: empty/NUL root")
		}
		r = filepath.Clean(r)
		st, err := os.Lstat(r)
		if err != nil {
			return "", "", err
		}
		if st.Mode()&os.ModeSymlink != 0 || !st.IsDir() {
			return "", "", fmt.Errorf("roots: root must be a non-symlink directory")
		}
		p, err := filepath.EvalSymlinks(r)
		if err != nil {
			return "", "", err
		}
		p, err = filepath.Abs(p)
		if err != nil {
			return "", "", err
		}
		roots[i] = p
		infos[i] = st
	}
	if os.SameFile(infos[0], infos[1]) || Within(roots[0], roots[1]) || Within(roots[1], roots[0]) {
		return "", "", fmt.Errorf("roots: same/nested/aliased roots")
	}
	return roots[0], roots[1], nil
}

// Within tests strict path-component containment, never a string prefix.
func Within(root, path string) bool {
	r, e := filepath.Rel(root, path)
	return e == nil && r != "." && r != ".." && !filepath.IsAbs(r) && !strings.HasPrefix(r, ".."+string(filepath.Separator))
}

// Resolve matches exact spelling first, otherwise a unique casefold entry.
// Only missing components return IsNotExist. Existing invalid components are hard errors.
// A missing suffix is returned only when allowMissing is true; no directory is created.
func Resolve(root, rel string, allowMissing bool) (string, error) {
	p := filepath.Join(root, rel)
	if !Within(root, p) {
		return "", fmt.Errorf("roots: path outside root")
	}
	parts := strings.Split(rel, string(filepath.Separator))
	cur := root
	for i, part := range parts {
		if part == "" || part == "." || part == ".." || strings.IndexByte(part, 0) >= 0 || strings.ContainsAny(part, `/\:`) || filepath.VolumeName(part) != "" {
			return "", fmt.Errorf("roots: invalid component")
		}
		next := filepath.Join(cur, part)
		st, err := os.Lstat(next)
		if os.IsNotExist(err) {
			es, e := os.ReadDir(cur)
			if e != nil {
				return "", e
			}
			match := ""
			for _, v := range es {
				if strings.EqualFold(v.Name(), part) {
					if match != "" {
						return "", fmt.Errorf("roots: ambiguous component %s", part)
					}
					match = v.Name()
				}
			}
			if match == "" {
				if allowMissing {
					return filepath.Join(append([]string{cur}, parts[i:]...)...), nil
				}
				return "", err
			}
			next = filepath.Join(cur, match)
			st, err = os.Lstat(next)
		}
		if err != nil {
			return "", err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("roots: symlink component")
		}
		if i < len(parts)-1 && !st.IsDir() {
			return "", fmt.Errorf("roots: middle component not directory")
		}
		cur = next
	}
	return cur, nil
}

// Mutation validates a complete SaveRoot descendant before any host change.
func Mutation(root, path string, directory bool) error {
	if !Within(root, path) {
		return fmt.Errorf("roots: mutation outside save")
	}
	rel, e := filepath.Rel(root, path)
	if e != nil {
		return e
	}
	p, e := Resolve(root, rel, true)
	if e != nil {
		return e
	}
	if p != path {
		return fmt.Errorf("roots: unresolved mutation spelling")
	}
	st, e := os.Lstat(p)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	if directory {
		if !st.IsDir() {
			return fmt.Errorf("roots: not directory")
		}
		return nil
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("roots: not regular")
	}
	return SingleLink(p, st)
}
