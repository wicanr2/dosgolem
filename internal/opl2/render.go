package opl2

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// Event 是「在第幾道指令寫了哪個暫存器」。時間換算由呼叫端給的每秒指令數決定。
type Event struct {
	Step     uint64
	Reg, Val uint8
}

// Render 依時間順序套用事件並合成（`docs/spec/196` §3.3）：從第一次 Key-On 起，到最後一筆事件後 2 秒。
// 回 16 位元取樣與不支援功能的計數；沒有 Key-On 時回錯誤。
func Render(events []Event, stepsPerSecond float64, rate int) ([]int16, map[string]int, error) {
	if rate <= 0 {
		rate = 22050
	}
	first := -1
	for i, e := range events {
		if e.Reg >= 0xB0 && e.Reg <= 0xB8 && e.Val&0x20 != 0 {
			first = i
			break
		}
	}
	if first < 0 {
		return nil, nil, fmt.Errorf("OPL2 一次 Key-On 都沒有——這一段沒有音樂")
	}
	s := New(rate)
	j := 0
	for ; j < first; j++ { // Key-On 之前的設定（音色）先套上
		s.Write(events[j].Reg, events[j].Val)
	}
	start := events[first].Step
	end := events[len(events)-1].Step
	n := int(float64(end-start)/stepsPerSecond*float64(rate)) + 2*rate
	pcm := make([]int16, n)
	for i := range pcm {
		step := start + uint64(float64(i)/float64(rate)*stepsPerSecond)
		for j < len(events) && events[j].Step <= step {
			s.Write(events[j].Reg, events[j].Val)
			j++
		}
		pcm[i] = int16(math.Round(s.Sample() * 32767))
	}
	return pcm, s.Unsupported, nil
}

// WriteWAV16 寫 16 位元單聲道 PCM WAV。
func WriteWAV16(w io.Writer, rate int, pcm []int16) error {
	var h []byte
	put32 := func(v uint32) { h = binary.LittleEndian.AppendUint32(h, v) }
	put16 := func(v uint16) { h = binary.LittleEndian.AppendUint16(h, v) }
	h = append(h, "RIFF"...)
	put32(uint32(36 + 2*len(pcm)))
	h = append(h, "WAVEfmt "...)
	put32(16)
	put16(1)
	put16(1)
	put32(uint32(rate))
	put32(uint32(rate * 2))
	put16(2)
	put16(16)
	h = append(h, "data"...)
	put32(uint32(2 * len(pcm)))
	if _, err := w.Write(h); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, pcm)
}
