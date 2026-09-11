// Package vgm 把 OPL（AdLib／Sound Blaster FM）的暫存器寫入序列編成
// VGM 檔（`docs/spec/190-opl-vgm-dump`）。
//
// **這一層是純的**：不認識 machine、不認識 DOS、不認識任何一支程式。
// 進來的是一串 `{Bank, Reg, Val, Step}`，出去的是一份合法的 VGM。
// 判準是 `docs/spec/006-layering` 那一句——換一支 binary 之後這段還成立嗎
// ——任何有 AdLib 的 DOS 程式都會把音符寫進 `0x388`／`0x389`，所以成立。
//
// **不合成波形。** VGM 是交給外部播放器的中介格式；「暫存器串是樂譜、
// 波形是演奏」那條線不動（`docs/spec/004-machine` §6）。
package vgm

import (
	"encoding/binary"
	"fmt"
	"io"
)

// SampleRate 是 VGM 延遲的單位：1/44100 秒。
//
// ⚠ **它與輸出音檔的取樣率無關。** 這是格式定死的刻度，播放器用什麼
// 取樣率合成是播放器的事。把它改成 22050「因為輸出是 22 kHz」會讓
// 整首曲子快一倍，而檔案照樣合法、播放器照樣不出聲抗議。
const SampleRate = 44100

// 標準時脈。AdLib／Sound Blaster 的 OPL2 吃 3.579545 MHz（NTSC 副載波），
// OPL3 吃它的四倍。
const (
	ym3812Clock = 3_579_545  // OPL2
	ymf262Clock = 14_318_180 // OPL3
)

// 資料起點。**不能用「資料接在 0x40」那種精簡版面**：OPL 的時脈欄位在
// `0x50`／`0x5C`，資料從 `0x40` 起就會把它蓋掉，而症狀是播放器認不出
// 晶片、安靜地播出無聲（`docs/spec/190-opl-vgm-dump` §5）。
const dataStart = 0x100

// Write 是一次 OPL 暫存器寫入。
//
// Bank 0 ＝ `0x388/0x389`（OPL2 相容），1 ＝ `0x38A/0x38B`（OPL3 第二組）。
// Step 是「第幾道指令」，零點由 Encode 自己取第一筆。
type Write struct {
	Bank uint8
	Reg  uint8
	Val  uint8
	Step uint64
}

// Stats 是編出來的東西的摘要。**呼叫端要把它印出來。**
//
// 「錄到 0 筆」與「錄到 3 萬筆」看檔案大小就分得出來；
// 「曲速差四倍」只有 Seconds 看得出來。
type Stats struct {
	Writes  int     // 暫存器寫入筆數
	Samples uint64  // 總延遲取樣數（44100 Hz）
	Seconds float64 // ＝ Samples / SampleRate
	Chip    string  // 判到的晶片："YM3812"（OPL2）或 "YMF262"（OPL3）
}

// Encode 把 writes 編成 VGM 寫進 w。
//
// stepsPerSecond 是「這台機器一秒跑幾道指令」，用來把 Step 換算成時間；
// 它必須 > 0，這裡不猜——猜錯的長相是「曲子播得出來，只是速度不對」，
// 沒有任何一個環節會報錯（`docs/spec/190-opl-vgm-dump` §3）。
//
// writes 要依 Step 遞增。**順序不重排**：OPL 的寫入是有序的副作用，
// 排序會改變意思。萬一遇到倒退的 Step，時間停住、寫入照發。
func Encode(w io.Writer, writes []Write, stepsPerSecond float64) (Stats, error) {
	var st Stats
	if len(writes) == 0 {
		// **不要寫出一份 0 筆的合法 VGM。** 它會一路通過後面每一個步驟，
		// 最後在喇叭上變成「沒聲音」，而那時候已經離這裡很遠了。
		return st, fmt.Errorf("OPL 一次都沒被寫過，沒有東西可以倒——" +
			"檢查順序見 docs/spec/190-opl-vgm-dump §7（AdLib 開了嗎、步數夠不夠、這段是不是走 MIDI）")
	}
	if stepsPerSecond <= 0 {
		return st, fmt.Errorf("時間基準要大於 0，拿到 %g", stepsPerSecond)
	}

	// 晶片判準是「有沒有寫過第二組」，不是「有沒有開 NEW 位元」。
	// 把 OPL3 判成 OPL2 會丟掉一半的寫入（少掉聲部）；反過來只是檔頭上
	// 的晶片名不同，聲音一樣。兩種誤判的代價不對等，所以往 OPL3 偏。
	opl3 := false
	for _, e := range writes {
		if e.Bank != 0 {
			opl3 = true
			break
		}
	}

	base := writes[0].Step
	body := make([]byte, 0, len(writes)*4+16)
	var now uint64 // 已經發出去的延遲取樣數
	for _, e := range writes {
		var want uint64
		if e.Step > base {
			want = uint64(float64(e.Step-base)*SampleRate/stepsPerSecond + 0.5)
		}
		if want > now {
			body = appendWait(body, want-now)
			now = want
		}
		switch {
		case !opl3:
			body = append(body, 0x5A, e.Reg, e.Val) // YM3812
		case e.Bank == 0:
			body = append(body, 0x5E, e.Reg, e.Val) // YMF262 port 0
		default:
			body = append(body, 0x5F, e.Reg, e.Val) // YMF262 port 1
		}
	}
	// ⚠ **沒有 0x66 的 VGM 不是短了一點，是整份廢掉**：播放器讀到檔尾
	// 才發現沒有結束標記，多數的反應是把後面的垃圾當成命令。所以收尾
	// 寫在這裡，不交給呼叫端記得。
	body = append(body, 0x66)

	h := make([]byte, dataStart)
	copy(h, "Vgm ")
	put := func(off int, v uint32) { binary.LittleEndian.PutUint32(h[off:], v) }
	put(0x04, uint32(dataStart-4+len(body))) // EOF 位移（相對 0x04）
	put(0x08, 0x00000151)                    // 版本 1.51
	put(0x18, uint32(now))                   // 總取樣數
	// 0x1C 循環位移與 0x20 循環取樣數留 0：**不猜循環點**。
	// 猜錯的循環聽起來只是「接得有點怪」，沒有任何東西會開口。
	put(0x34, dataStart-0x34) // 資料位移（相對 0x34）
	st.Chip = "YM3812"
	if opl3 {
		st.Chip = "YMF262"
		put(0x5C, ymf262Clock)
	} else {
		put(0x50, ym3812Clock)
	}

	if _, err := w.Write(h); err != nil {
		return st, err
	}
	if _, err := w.Write(body); err != nil {
		return st, err
	}
	st.Writes = len(writes)
	st.Samples = now
	st.Seconds = float64(now) / SampleRate
	return st, nil
}

// appendWait 發 n 個取樣的延遲，挑最短的命令。
//
//	0x70+k    等 k+1 個取樣（1–16），一個位元組
//	0x61 nnnn 等 nnnn 個取樣（16 位元小端），三個位元組
func appendWait(b []byte, n uint64) []byte {
	for n > 0 {
		switch {
		case n <= 16:
			b = append(b, byte(0x70+(n-1)))
			n = 0
		case n > 0xFFFF:
			b = append(b, 0x61, 0xFF, 0xFF)
			n -= 0xFFFF
		default:
			b = append(b, 0x61, byte(n), byte(n>>8))
			n = 0
		}
	}
	return b
}
