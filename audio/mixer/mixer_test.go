package mixer

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

const sps = 11550000

func TestSampleMappingNoDrift(t *testing.T) {
	m := New(sps)
	var out []float32
	var step uint64
	for i := 0; i < 600; i++ { // 10 秒，每格 192500 步
		out = m.Render(out[:0], step, step+192500)
		if len(out) != 1600 {
			t.Fatalf("第 %d 格 %d 個 float，要 1600", i, len(out))
		}
		step += 192500
	}
	if m.next != 480000 {
		t.Fatalf("樣本序號 %d", m.next)
	}
}

func TestPauseAndDiscontinuity(t *testing.T) {
	m := New(sps)
	if got := m.Render(nil, 1000000, 1000000); len(got) != 0 {
		t.Fatal("0 步卻有樣本")
	}
	// 第一次從非 0 步數開始：不補前面的樣本。
	got := m.Render(nil, 1000000, 1000000+192500)
	if len(got) != 1600 && len(got) != 1602 {
		t.Fatalf("首段 %d", len(got))
	}
}

func TestTimerRegistersNotSent(t *testing.T) {
	m := New(sps)
	for _, reg := range []uint8{0x02, 0x03, 0x04} {
		if _, ok := m.oplAddr(machine.OPLWrite{Reg: reg}); ok {
			t.Fatalf("%02X 送進合成器", reg)
		}
		// 第二組在非 newm 下對到第一組，同樣不送。
		if _, ok := m.oplAddr(machine.OPLWrite{Reg: reg, Bank: 1}); ok {
			t.Fatalf("第二組 %02X 送進合成器", reg)
		}
	}
}

func TestBank1Addressing(t *testing.T) {
	m := New(sps)
	if a, _ := m.oplAddr(machine.OPLWrite{Reg: 0xB0, Bank: 1}); a != 0xB0 {
		t.Fatalf("非 newm 第二組 %03X", a)
	}
	if a, _ := m.oplAddr(machine.OPLWrite{Reg: 0x05, Val: 1, Bank: 1}); a != 0x105 {
		t.Fatalf("05h %03X", a)
	}
	if a, _ := m.oplAddr(machine.OPLWrite{Reg: 0xB0, Bank: 1}); a != 0x1B0 {
		t.Fatalf("newm 第二組 %03X", a)
	}
	if a, ok := m.oplAddr(machine.OPLWrite{Reg: 0x04, Bank: 1}); a != 0x104 || !ok {
		t.Fatalf("newm 第二組 04h %03X %v", a, ok)
	}
	m.oplAddr(machine.OPLWrite{Reg: 0x05, Val: 0, Bank: 1})
	if a, _ := m.oplAddr(machine.OPLWrite{Reg: 0xB0, Bank: 1}); a != 0xB0 {
		t.Fatalf("關 newm 後 %03X", a)
	}
}

func rms(s []float32) float64 {
	var sum float64
	for _, v := range s {
		sum += float64(v) * float64(v)
	}
	return math.Sqrt(sum / float64(len(s)))
}

func TestSpeakerSquareWave(t *testing.T) {
	m := New(sps)
	m.PITChannel2(machine.PIT2Change{Step: 0, Divisor: 1193182 / 1000, Mode: 3})
	m.SpeakerSample(machine.SpeakerSample{Step: 0, Level: 1, Gate: 1})
	out := m.Render(nil, 0, sps/10) // 0.1 秒
	if rms(out[2000:]) < 0.05 {
		t.Fatalf("方波太小 %f", rms(out))
	}
	// 關閘門、資料線 0：靜音（隔直後趨近 0）。
	m.SpeakerSample(machine.SpeakerSample{Step: sps / 10, Level: 0, Gate: 0})
	out = m.Render(nil, sps/10, sps)
	if r := rms(out[len(out)-4000:]); r > 1e-3 {
		t.Fatalf("靜音仍有 %f", r)
	}
	// 超過奈奎斯特：靜音。
	m.PITChannel2(machine.PIT2Change{Step: sps, Divisor: 2})
	m.SpeakerSample(machine.SpeakerSample{Step: sps, Level: 1, Gate: 1})
	out = m.Render(nil, sps, sps+sps/10)
	if r := rms(out[2000:]); r > 1e-3 {
		t.Fatalf("超音波未靜音 %f", r)
	}
}

func TestOPLProducesSound(t *testing.T) {
	m := New(sps)
	// 一個最簡單的正弦音：通道 0，載波 op 3（偏移 03h）。
	w := func(step uint64, reg, val uint8) {
		m.OPLWrite(machine.OPLWrite{Step: step, Reg: reg, Val: val})
	}
	w(0, 0x20, 0x01)
	w(0, 0x23, 0x21) // EG 持續型
	w(0, 0x40, 0x3F) // 調變器靜音
	w(0, 0x43, 0x00)
	w(0, 0x60, 0xF0)
	w(0, 0x63, 0xF0)
	w(0, 0x80, 0x0F)
	w(0, 0x83, 0x0F)
	w(0, 0xC0, 0x31) // 左右都開（OPL2 下無作用）、加法合成
	w(0, 0xA0, 0x44)
	w(1, 0xB0, 0x32) // key-on
	out := m.Render(nil, 0, sps/5)
	if rms(out) < 0.01 {
		t.Fatalf("OPL 無聲 %f", rms(out))
	}
}

func TestRingBounds(t *testing.T) {
	r := NewRing(100) // 4800 框
	r.Push(make([]float32, 2*6000))
	if b := r.Buffered(); b != 4800 {
		t.Fatalf("上限 %d", b)
	}
	if _, d := r.Stats(); d != 1200 {
		t.Fatalf("丟棄 %d", d)
	}
	p := make([]byte, 8*5000+3)
	n, err := r.Read(p)
	if err != nil || n != 8*5000 {
		t.Fatalf("Read %d %v", n, err)
	}
	if u, _ := r.Stats(); u != 1 {
		t.Fatalf("不足次數 %d", u)
	}
	// 回到預填狀態：不足門檻（50 ms＝2400 框）時輸出靜音。
	r.Push(ones(1000))
	r.Read(p[:8*100])
	if f32(p, 0) != 0 {
		t.Fatal("未達預填門檻卻輸出樣本")
	}
	// 達門檻後開始播放，開頭淡入、之後為原值。
	r.Push(ones(2000))
	r.Read(p[:8*200])
	if f32(p, 0) != 0 || f32(p, 2*(fadeFrames+5)) != 1 {
		t.Fatalf("淡入 %v %v", f32(p, 0), f32(p, 2*(fadeFrames+5)))
	}
	// 斷流：手上的樣本淡出到接近 0，其後補 0，並記一次不足。
	u0, _ := r.Stats()
	left := r.Buffered()
	r.Read(p[:8*(left+50)])
	if u, _ := r.Stats(); u != u0+1 {
		t.Fatal("斷流未記錄")
	}
	if last := f32(p, 2*(left-1)); last > 0.05 {
		t.Fatalf("斷流前未淡出 %v", last)
	}
	if f32(p, 2*(left+10)) != 0 {
		t.Fatal("斷流後應為靜音")
	}
}

func ones(frames int) []float32 {
	s := make([]float32, 2*frames)
	for i := range s {
		s[i] = 1
	}
	return s
}

func f32(p []byte, i int) float32 { return math.Float32frombits(binary.LittleEndian.Uint32(p[4*i:])) }
