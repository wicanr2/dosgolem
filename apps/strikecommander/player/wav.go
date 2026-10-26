package player

import (
	"encoding/binary"
	"errors"
	"math"
	"os"
)

// WAVWriter streams the existing 48kHz stereo mixer format as PCM16, using
// Buck's truncation(v*32767). It never opens a playback device or keeps a song.
type WAVWriter struct {
	f      *os.File
	bytes  uint64
	closed bool
}

func NewWAV(p string) (*WAVWriter, error) {
	f, e := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if _, e = f.Write(make([]byte, 44)); e != nil {
		f.Close()
		return nil, e
	}
	return &WAVWriter{f: f}, nil
}
func (w *WAVWriter) Append(samples []float32) error {
	if w == nil || w.closed {
		return errors.New("WAV已關閉")
	}
	if len(samples)%2 != 0 {
		return errors.New("WAV需完整立體聲frame")
	}
	if w.bytes+uint64(len(samples))*2 > math.MaxUint32-36 {
		return errors.New("WAV超RIFF32上限")
	}
	var buf [8192]byte
	for len(samples) > 0 {
		n := min(len(samples), len(buf)/2)
		for i, v := range samples[:n] {
			if v > 1 {
				v = 1
			}
			if v < -1 {
				v = -1
			}
			binary.LittleEndian.PutUint16(buf[i*2:], uint16(int16(v*32767)))
		}
		got, e := w.f.Write(buf[:n*2])
		w.bytes += uint64(got)
		if e != nil {
			return e
		}
		if got != n*2 {
			return errors.New("WAV短寫")
		}
		samples = samples[n:]
	}
	return nil
}
func (w *WAVWriter) Close() error {
	if w == nil || w.closed {
		return nil
	}
	w.closed = true
	var b [44]byte
	copy(b[:], "RIFF")
	binary.LittleEndian.PutUint32(b[4:], uint32(w.bytes+36))
	copy(b[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(b[16:], 16)
	binary.LittleEndian.PutUint16(b[20:], 1)
	binary.LittleEndian.PutUint16(b[22:], 2)
	binary.LittleEndian.PutUint32(b[24:], 48000)
	binary.LittleEndian.PutUint32(b[28:], 48000*4)
	binary.LittleEndian.PutUint16(b[32:], 4)
	binary.LittleEndian.PutUint16(b[34:], 16)
	copy(b[36:], "data")
	binary.LittleEndian.PutUint32(b[40:], uint32(w.bytes))
	_, e := w.f.WriteAt(b[:], 0)
	if e == nil {
		e = w.f.Sync()
	}
	if ce := w.f.Close(); e == nil {
		e = ce
	}
	return e
}
