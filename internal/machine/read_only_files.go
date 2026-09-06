package machine

import (
	"io"
	"io/fs"
	"os"
	"strings"
)

type ReadOnlyFileProvider interface {
	OpenRead(name string) (io.ReadSeekCloser, error)
}

type DirectoryReadOnlyFiles struct {
	root *os.Root
}

func OpenDirectoryReadOnlyFiles(path string) (*DirectoryReadOnlyFiles, error) {
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	return &DirectoryReadOnlyFiles{root: root}, nil
}

func (p *DirectoryReadOnlyFiles) Close() error {
	if p == nil || p.root == nil {
		return nil
	}
	return p.root.Close()
}

func (p *DirectoryReadOnlyFiles) OpenRead(name string) (io.ReadSeekCloser, error) {
	if p == nil || p.root == nil || name == "" || name == "." || name == ".." ||
		strings.ContainsAny(name, "/\\:") {
		return nil, fs.ErrPermission
	}
	entries, err := fs.ReadDir(p.root.FS(), ".")
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !strings.EqualFold(entry.Name(), name) {
			continue
		}
		file, err := p.root.Open(entry.Name())
		if err != nil {
			return nil, err
		}
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			file.Close()
			if err != nil {
				return nil, err
			}
			return nil, fs.ErrPermission
		}
		return file, nil
	}
	return nil, fs.ErrNotExist
}
