package mixer

import (
	"encoding/binary"
	"io"
	"sort"

	"github.com/wicanr2/dosgolem/internal/dos"
)

// CD 音軌來源（`docs/spec/440-mixer-cd-audio` §2.1）：MSCDEX 的播放紀錄對上 raw 光碟映像，
// 44.1 kHz 線性內插成輸出取樣率。位置一律從絕對樣本序號算，不依賴 Render 被切成幾段。

const (
	cdRate          = 44100
	cdSectorBytes   = 2352
	cdSectorSamples = cdSectorBytes / 4 // 每扇區 588 個立體聲樣本
)

// cdSource 是掛上的光碟與一個扇區的讀取快取。
type cdSource struct {
	cd     *dos.CDROM
	image  io.ReaderAt
	off    int64 // 快取的扇區位移；−1 ＝ 沒有快取
	buf    [cdSectorBytes]byte
	errors int
}

// AttachCD 讓混音器播放 cd 的播放紀錄，音軌資料讀自 image（每扇區 2352 bytes 的 raw 映像）。
func (m *Mixer) AttachCD(cd *dos.CDROM, image io.ReaderAt) {
	m.cd = &cdSource{cd: cd, image: image, off: -1}
}

// CDErrors 回讀映像失敗的扇區數（失敗的扇區當靜音）。
func (m *Mixer) CDErrors() int {
	if m.cd == nil {
		return 0
	}
	return m.cd.errors
}

// stereo 回這段播放第 k 個 CD 樣本；落在資料軌或沒有映像位移的軌時 ok 為 false。
func (s *cdSource) stereo(p dos.CDPlay, k uint64) (l, r int64, ok bool) {
	off := s.cd.SectorOffset(p.Start + uint32(k/cdSectorSamples))
	if off < 0 {
		return 0, 0, false
	}
	if off != s.off {
		s.off = off
		if _, err := s.image.ReadAt(s.buf[:], off); err != nil {
			s.errors++
			clear(s.buf[:])
		}
	}
	i := (k % cdSectorSamples) * 4
	l = int64(int16(binary.LittleEndian.Uint16(s.buf[i:])))
	r = int64(int16(binary.LittleEndian.Uint16(s.buf[i+2:])))
	return l, r, true
}

// cdSample 回輸出樣本 n 的 CD 值（16 位元刻度）。
func (m *Mixer) cdSample(n uint64) (l, r int64) {
	s := m.cd
	plays := s.cd.Plays
	// 起點不晚於 n 的最後一段：紀錄依時間排序，重疊時取起點較晚的（§2.1）。
	i := sort.Search(len(plays), func(i int) bool { return m.sampleOf(plays[i].FromStep) > n }) - 1
	if i < 0 {
		return 0, 0
	}
	p := plays[i]
	if p.ToStep != 0 && n >= m.sampleOf(p.ToStep) {
		return 0, 0
	}
	num := (n - m.sampleOf(p.FromStep)) * cdRate
	k, f := num/SampleRate, int64(num%SampleRate)
	total := uint64(p.Length) * cdSectorSamples
	if k >= total {
		return 0, 0 // 播滿長度
	}
	l0, r0, ok := s.stereo(p, k)
	if !ok {
		return 0, 0
	}
	l1, r1 := l0, r0
	if k+1 < total {
		if a, b, ok := s.stereo(p, k+1); ok {
			l1, r1 = a, b
		}
	}
	return l0 + (l1-l0)*f/SampleRate, r0 + (r1-r0)*f/SampleRate
}
