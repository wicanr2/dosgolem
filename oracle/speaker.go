package oracle

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// PC 喇叭的波形（`docs/spec/016`）。
//
// 智冠《三國演義》的語音是一個位元的取樣：程式把 8253 通道 0 重設成
// 幾千赫，在 IRQ0 的處理常式裡每次把一個位元送到埠 `0x61` 的 bit1。
// 對拍要驗的是**那串位元**——它是決定性的，波形怎麼合成則不是。

// SpeakerSample 是喇叭資料線的一次變化。
type SpeakerSample = machine.SpeakerSample

// Speaker 回喇叭資料線的變化序列。
//
// **只記變化**，所以長度是「切換次數」不是取樣數。一段從頭到尾沒動過
// 的訊號長度是 0——那是「沒出聲」，不是「沒接上」。
func (o *Oracle) Speaker() []SpeakerSample { return o.m.Speaker }

// PITDivisor 是 8253 通道 0 目前的分頻值（0 ＝ 還沒被設過，等同 65536）。
//
// 《三國演義》的「語音速度」（說明書寫 1–30000）就是寫進這裡的值。
func (o *Oracle) PITDivisor() uint32 { return o.m.PITDivisor() }

// PITHz 是通道 0 目前的中斷頻率。
func (o *Oracle) PITHz() float64 { return o.m.PITHz() }

// IRQ0Clamped 是「算出來的中斷間隔太小、被夾到下限」發生幾次。
//
// **非零表示波形的時間軸不可信**：原版要的取樣率比這台虛擬機跑得動的
// 還快，被夾住之後聲音會變慢。它不是錯誤，是一個要講出來的限制。
func (o *Oracle) IRQ0Clamped() int { return o.m.IRQ0Clamped }

// SpeakerWAV 把喇叭的波形寫成 8 位元單聲道 WAV。
//
// rate 是輸出取樣率（Hz），0 用 11025。時間軸的換算是
// `machine.StepsPerSecond()`——**那是模型不是量測**，它定義「這台虛擬機
// 一秒跑幾道指令」（`docs/spec/016` §3.2）。
//
// 兩個位準寫成 0x20／0xE0 而不是 0x00／0xFF：一位元的訊號滿幅輸出
// 在多數播放器上會削波，聽起來像壞掉。
func (o *Oracle) SpeakerWAV(w io.Writer, rate int) error {
	if rate <= 0 {
		rate = 11025
	}
	s := o.m.Speaker
	if len(s) == 0 {
		return fmt.Errorf("喇叭一次都沒動過——這一段沒有聲音")
	}
	sps := machine.StepsPerSecond()
	first, last := s[0].Step, s[len(s)-1].Step
	n := int(float64(last-first) / sps * float64(rate))
	if n <= 0 {
		return fmt.Errorf("波形只有 %d 道指令長，不足一個取樣", last-first)
	}
	pcm := make([]uint8, n)
	j := 0
	for i := 0; i < n; i++ {
		step := first + uint64(float64(i)/float64(rate)*sps)
		for j+1 < len(s) && s[j+1].Step <= step {
			j++
		}
		if s[j].Level != 0 {
			pcm[i] = 0xE0
		} else {
			pcm[i] = 0x20
		}
	}
	return writeWAV8(w, rate, pcm)
}

// writeWAV8 寫一份 8 位元單聲道、無壓縮的 WAV。
func writeWAV8(w io.Writer, rate int, pcm []uint8) error {
	var h []byte
	put32 := func(v uint32) { h = binary.LittleEndian.AppendUint32(h, v) }
	put16 := func(v uint16) { h = binary.LittleEndian.AppendUint16(h, v) }
	h = append(h, "RIFF"...)
	put32(uint32(36 + len(pcm)))
	h = append(h, "WAVEfmt "...)
	put32(16)              // fmt 區塊長度
	put16(1)               // PCM
	put16(1)               // 單聲道
	put32(uint32(rate))    // 取樣率
	put32(uint32(rate))    // 每秒位元組數 ＝ rate × 1 × 1
	put16(1)               // 區塊對齊
	put16(8)               // 位元深度
	h = append(h, "data"...)
	put32(uint32(len(pcm)))
	if _, err := w.Write(h); err != nil {
		return err
	}
	_, err := w.Write(pcm)
	return err
}
