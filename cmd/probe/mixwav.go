package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"

	"github.com/wicanr2/dosgolem/audio/mixer"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// mixWAV 是 `-dump-mix-wav`（`docs/spec/440-mixer-cd-audio` §2.2）：OPL、喇叭與 CD 音軌經
// audio/mixer 混成 48 kHz 立體聲，邊跑邊寫進檔案。

// mixEvery 是兩次 Render 之間的指令數。
const mixEvery = 1_000_000

type mixWAV struct {
	f      *os.File
	img    *os.File // nil：沒有 CD 映像
	mix    *mixer.Mixer
	path   string
	first  uint64 // 時間軸的起點（第一道指令的步數）
	last   uint64 // 已經 Render 到的步數
	frames int64
	buf    []float32
	out    []byte
}

// newMixWAV 開輸出檔並掛上混音器。觀測器照舊把寫入追加到 m.OPL、m.Speaker，其他倒檔旗標不受影響。
func newMixWAV(path string, m *machine.Machine, d *dos.DOS, image string) (*mixWAV, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	w := &mixWAV{f: f, path: path, mix: mixer.New(uint64(machine.StepsPerSecond())), first: m.Steps, last: m.Steps}
	if _, err := f.Write(make([]byte, 44)); err != nil { // 檔頭最後補
		f.Close()
		return nil, err
	}
	if d.CD != nil && image != "" {
		img, err := os.Open(image)
		if err != nil {
			f.Close()
			return nil, err
		}
		w.img = img
		w.mix.AttachCD(d.CD, img)
	}
	m.ObserveOPLWrites(func(x machine.OPLWrite) {
		m.OPL = append(m.OPL, x)
		w.mix.OPLWrite(x)
	})
	m.ObserveSpeaker(func(x machine.SpeakerSample) {
		m.Speaker = append(m.Speaker, x)
		w.mix.SpeakerSample(x)
	})
	m.ObservePITChannel2(w.mix.PITChannel2)
	return w, nil
}

// step 在主迴圈每一步之後呼叫；每 mixEvery 道指令合成一段。
func (w *mixWAV) step(m *machine.Machine) error {
	if m.Steps-w.last < mixEvery {
		return nil
	}
	return w.render(m.Steps)
}

func (w *mixWAV) render(to uint64) error {
	w.buf = w.mix.Render(w.buf[:0], w.last, to)
	w.last = to
	w.out = w.out[:0]
	for _, v := range w.buf {
		// v 是 [−1, 1]；×32768 四捨五入再夾，單獨的 CD 樣本可以原值還原。
		s := math.Round(float64(v) * 32768)
		s = math.Max(-32768, math.Min(32767, s))
		w.out = binary.LittleEndian.AppendUint16(w.out, uint16(int16(s)))
	}
	w.frames += int64(len(w.buf) / 2)
	_, err := w.f.Write(w.out)
	return err
}

// finish 補最後一段、寫檔頭並印出摘要。
func (w *mixWAV) finish(m *machine.Machine, d *dos.DOS) error {
	defer func() {
		if w.img != nil {
			w.img.Close()
		}
	}()
	if err := w.render(m.Steps); err != nil {
		w.f.Close()
		return err
	}
	data := uint32(w.frames * 4)
	hdr := make([]byte, 0, 44)
	hdr = append(hdr, "RIFF"...)
	hdr = binary.LittleEndian.AppendUint32(hdr, 36+data)
	hdr = append(hdr, "WAVEfmt "...)
	hdr = binary.LittleEndian.AppendUint32(hdr, 16)
	hdr = binary.LittleEndian.AppendUint16(hdr, 1) // PCM
	hdr = binary.LittleEndian.AppendUint16(hdr, 2)
	hdr = binary.LittleEndian.AppendUint32(hdr, mixer.SampleRate)
	hdr = binary.LittleEndian.AppendUint32(hdr, mixer.SampleRate*4)
	hdr = binary.LittleEndian.AppendUint16(hdr, 4)
	hdr = binary.LittleEndian.AppendUint16(hdr, 16)
	hdr = append(hdr, "data"...)
	hdr = binary.LittleEndian.AppendUint32(hdr, data)
	if _, err := w.f.WriteAt(hdr, 0); err != nil {
		w.f.Close()
		return err
	}
	if err := w.f.Close(); err != nil {
		return err
	}
	plays := 0
	if d.CD != nil && w.img != nil {
		plays = len(d.CD.Plays)
	}
	fmt.Printf("混音 → %s（48 kHz 立體聲，%.1f 秒，起點第 %d 道指令；OPL 寫入 %d 筆、CD 播放 %d 段、讀映像錯誤 %d、夾值樣本 %d）\n",
		w.path, float64(w.frames)/mixer.SampleRate, w.first, len(m.OPL), plays, w.mix.CDErrors(), w.mix.Clipped())
	return nil
}
