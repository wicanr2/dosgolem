package shadesource

import (
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/model"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/opaque"
	"io"
)

// profileLegacyDigest requires Owner.worker already held. No reentrant lock.
func (o *Owner) profileLegacyDigest(s model.Input) ([32]byte, error) {
	if o.producer == nil {
		return opaque.InputDigest(s)
	}
	return opaque.BoundedInputDigest(s, 32<<20)
}

// BoundedLegacyDigest is callback-external and uses the existing fixed worker.
func (o *Owner) BoundedLegacyDigest(s model.Input) (out [32]byte, err error) {
	err = o.CompositionWorkspace(func() error { var e error; out, e = opaque.BoundedInputDigest(s, 32<<20); return e })
	return
}
func (o *Owner) WriteBoundedLegacyJSON(s model.Input, w io.Writer) error {
	return o.CompositionWorkspace(func() error { return opaque.WriteBoundedInputJSON(w, s) })
}
func (o *Owner) profileWritePacket(w io.Writer, p Packet) error {
	if o.producer == nil {
		return writePacket(w, p)
	}
	if e := packetShape(&p); e != nil {
		return e
	}
	// Match original receipt limit; default Go JSON with newline, no whole value.
	return opaque.WriteBoundedJSON(w, p, 8<<20, 16<<20)
}
