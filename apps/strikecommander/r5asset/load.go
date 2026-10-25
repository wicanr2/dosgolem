package r5asset

import (
	"bytes"
	"crypto/sha256"
	"image/png"
	"io"
	"os"
	"path/filepath"
)

// A child budget of the already reserved, serialized 32MiB worker. It is not
// another global reservation. Callers may only reduce this limit.
type workerBudget struct{ limit, used, peak uint64 }

func (w *workerBudget) reserve(n uint64) error {
	if w.used > w.limit || n > w.limit-w.used {
		return reject(Capacity, 0, 400)
	}
	w.used += n
	if w.used > w.peak {
		w.peak = w.used
	}
	return nil
}
func (w *workerBudget) release(n uint64) {
	if n > w.used {
		panic("r5asset worker accounting")
	}
	w.used -= n
}

// The bounded profile path is checked without following symlinks, including
// each parent component. The caller must provide a stable, read-only AssetRoot.
func regularPath(name string) error {
	for i := 1; i <= len(name); i++ {
		if i < len(name) && !os.IsPathSeparator(name[i]) {
			continue
		}
		part := name[:i]
		st, e := os.Lstat(part)
		if e != nil {
			return reject(Identity, 0, 401)
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return reject(Identity, 0, 402)
		}
		if i < len(name) {
			if !st.IsDir() {
				return reject(Identity, 0, 403)
			}
		} else if !st.Mode().IsRegular() {
			return reject(Identity, 0, 404)
		}
	}
	return nil
}
func readPinnedInto(name string, s fileIdentity, dst []byte) error {
	if uint64(len(dst)) != s.Bytes {
		return reject(Capacity, 0, 405)
	}
	if e := regularPath(name); e != nil {
		return e
	}
	before, e := os.Lstat(name)
	if e != nil {
		return reject(Identity, 0, 406)
	}
	f, e := os.Open(name)
	if e != nil {
		return reject(Identity, 0, 407)
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() || !os.SameFile(before, st) || st.Size() < 0 || uint64(st.Size()) != s.Bytes {
		return reject(Identity, 0, 408)
	}
	if _, e = io.ReadFull(f, dst); e != nil {
		return reject(Identity, 0, 409)
	}
	var tail [1]byte
	n, e := f.Read(tail[:])
	if n != 0 || e != io.EOF {
		return reject(Identity, 0, 410)
	}
	after, e := os.Lstat(name)
	if e != nil || after.Mode()&os.ModeSymlink != 0 || !os.SameFile(st, after) || after.Size() != st.Size() {
		return reject(Identity, 0, 411)
	}
	if sha256.Sum256(dst) != s.SHA {
		return reject(Identity, 0, 412)
	}
	return nil
}
func (l *Loader) path(p profile, s fileIdentity) string {
	return filepath.Join(l.root, p.Name, "model", s.Name)
}
func contextual(e error, stage, model, slot uint8) error {
	if e == nil {
		return nil
	}
	v, ok := e.(Error)
	if !ok {
		v = reject(Capacity, 0, 413)
	}
	v.Stage = stage
	v.Model = model
	v.Slot = slot
	return v
}

func (l *Loader) loadMesh(d *assetData, m int) error {
	p := profiles[m]
	// The caller reserved the ENTIRE arena+strconv bound before this allocation.
	ar := new(jsonArena)
	if e := readPinnedInto(l.path(p, p.Files[0]), p.Files[0], ar.raw[:p.Files[0].Bytes]); e != nil {
		return contextual(e, 1, p.Model, 0)
	}
	ar.rawN = uint32(p.Files[0].Bytes)
	hold, e := l.allocate(p.BinBytes)
	if e != nil {
		return contextual(e, 1, p.Model, 1)
	}
	d.binHold[m] = hold
	d.bin[m] = make([]byte, int(p.BinBytes))
	if e = readPinnedInto(l.path(p, p.Files[1]), p.Files[1], d.bin[m]); e != nil {
		return contextual(e, 1, p.Model, 1)
	}
	mesh, e := validateMesh(ar, d.bin[m], p)
	if e != nil {
		return contextual(e, 1, p.Model, 0)
	}
	d.mesh[m] = mesh
	return nil
}

type imageTemporary struct{ compressed, pix *allocation }

func (t imageTemporary) release() { t.compressed.release(); t.pix.release() }
func (l *Loader) loadImage(p profile, s fileIdentity) (t *texture, temp imageTemporary, err error) {
	h, e := l.allocate(s.Bytes)
	if e != nil {
		return nil, temp, e
	}
	temp.compressed = h
	raw := make([]byte, int(s.Bytes))
	if e = readPinnedInto(l.path(p, s), s, raw); e != nil {
		return nil, temp, e
	}
	c := 3
	if s.Color == 6 {
		c = 4
	}
	if e = checkPNG(raw, c); e != nil {
		return nil, temp, e
	}
	h, e = l.allocate(decodedCapacity)
	if e != nil {
		return nil, temp, e
	}
	temp.pix = h
	img, e := png.Decode(bytes.NewReader(raw))
	if e != nil {
		return nil, temp, reject(Decode, 0, 414)
	}
	pix, stride, e := rawPixels(img, c)
	if e != nil {
		return nil, temp, e
	}
	w := new(workspace)
	t, e = build(pix, stride, c, w, l)
	if e != nil {
		return nil, temp, e
	}
	// No source/decoded/worker references escape this function. A failure closes
	// the just-created cache; the transaction owns all earlier images.
	candidate := t
	good := false
	defer func() {
		if !good {
			candidate.close()
			t = nil
		}
	}()
	if _, e = verify(t, w, pix, stride); e != nil {
		return nil, temp, e
	}
	good = true
	return t, temp, nil
}

// Load is called by the private startup factory before any guest input/steps.
// It publishes exactly one complete batch and can be retried after failure.
func (l *Loader) Load() (err error) {
	l.mu.Lock()
	if l.closing || l.disposed {
		l.mu.Unlock()
		return reject(Closed, 0, 415)
	}
	if l.loading || l.data != nil {
		l.mu.Unlock()
		return reject(Ownership, 0, 416)
	}
	l.loading = true
	l.mu.Unlock()
	d := new(assetData) // Included in the pre-reserved fixed controller budget.
	committed := false
	defer func() {
		if !committed {
			d.close()
		}
		l.mu.Lock()
		l.loading = false
		l.finishLocked()
		l.mu.Unlock()
	}()
	w := workerBudget{limit: l.limits.WorkerBytes}
	err = l.owner.CompositionWorkspace(func() error {
		if arenaBytes > JSONBodyLimit || arenaBytes+numberReservation > JSONReservation {
			return reject(Capacity, 0, 417)
		}
		for m := range profiles {
			if e := w.reserve(JSONReservation); e != nil {
				return e
			}
			e := l.loadMesh(d, m) // All parser aliases end before refunding this child.
			w.release(JSONReservation)
			if e != nil {
				return e
			}
		}
		for m, p := range profiles {
			for slot, s := range p.Images {
				if l.stopped() {
					return reject(Closed, 0, 418)
				}
				if e := w.reserve(textureWorkBytes + decoderReserve); e != nil {
					return contextual(e, 2, p.Model, uint8(slot+2))
				}
				t, temp, e := l.loadImage(p, s)
				// The inner frame has returned; neither raw, Pix nor dictionary is still
				// reachable through the transaction when these reservations are released.
				temp.release()
				w.release(textureWorkBytes + decoderReserve)
				if e != nil {
					return contextual(e, 2, p.Model, uint8(slot+2))
				}
				d.tex[m*3+slot] = t
			}
		}
		return nil
	})
	maxAtomic(&l.workerPeak, w.peak)
	if err != nil {
		if _, ok := err.(Error); ok {
			return err
		}
		return contextual(err, 3, 0, 0)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closing || l.disposed {
		return reject(Closed, 0, 419)
	}
	l.data = d
	committed = true
	return nil
}
