package machine

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"strings"
)

type WriteFileProvider interface {
	OpenWrite(name string, readWrite bool) (io.ReadSeekCloser, error)
}
type DirectoryOverlayFiles struct {
	base  *DirectoryReadOnlyFiles
	state *os.Root
}

func OpenDirectoryOverlayFiles(basePath, statePath string) (*DirectoryOverlayFiles, error) {
	base, err := OpenDirectoryReadOnlyFiles(basePath)
	if err != nil {
		return nil, err
	}
	state, err := os.OpenRoot(statePath)
	if err != nil {
		base.Close()
		return nil, err
	}
	a, errA := base.root.Stat(".")
	b, errB := state.Stat(".")
	if errA != nil || errB != nil || os.SameFile(a, b) {
		base.Close()
		state.Close()
		return nil, fs.ErrPermission
	}
	return &DirectoryOverlayFiles{base: base, state: state}, nil
}
func (p *DirectoryOverlayFiles) Close() error {
	a := p.base.Close()
	b := p.state.Close()
	return errors.Join(a, b)
}
func (p *DirectoryOverlayFiles) stateName(name string) (string, error) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\:") {
		return "", fs.ErrPermission
	}
	entries, err := fs.ReadDir(p.state.FS(), ".")
	if err != nil {
		return "", err
	}
	found := ""
	for _, e := range entries {
		if strings.EqualFold(e.Name(), name) {
			if !e.Type().IsRegular() || found != "" {
				return "", fs.ErrPermission
			}
			found = e.Name()
		}
	}
	if found == "" {
		return "", fs.ErrNotExist
	}
	return found, nil
}
func (p *DirectoryOverlayFiles) OpenRead(name string) (io.ReadSeekCloser, error) {
	actual, err := p.stateName(name)
	if err == nil {
		return p.state.Open(actual)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return p.base.OpenRead(name)
}
func (p *DirectoryOverlayFiles) OpenWrite(name string, readWrite bool) (io.ReadSeekCloser, error) {
	actual, err := p.stateName(name)
	if errors.Is(err, fs.ErrNotExist) {
		input, e := p.base.OpenRead(name)
		if e != nil {
			return nil, e
		}
		defer input.Close()
		f, e := p.state.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return nil, e
		}
		_, copyErr := io.Copy(f, input)
		closeErr := f.Close()
		if copyErr != nil || closeErr != nil {
			p.state.Remove(name)
			return nil, errors.Join(copyErr, closeErr)
		}
		actual = name
	} else if err != nil {
		return nil, err
	}
	mode := os.O_WRONLY
	if readWrite {
		mode = os.O_RDWR
	}
	return p.state.OpenFile(actual, mode, 0600)
}
