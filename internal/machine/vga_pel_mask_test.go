package machine

import (
	"bytes"
	"encoding/gob"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu386"
)

func TestVGAPELMaskPaletteAndRGBPhase(t *testing.T) {
	m := New()
	if m.In8(0x3c6) != 0xff {
		t.Fatal("開機遮罩")
	}
	for i := 0; i < 256; i++ {
		m.Out8(0x3c8, byte(i))
		for _, v := range []byte{byte(i), byte(i >> 2), byte(255 - i)} {
			m.Out8(0x3c9, v)
		}
	}
	raw := m.DAC
	for _, mask := range []byte{0, 0xff, 0x0f, 0x55, 0xa2} {
		m.Out8(0x3c6, mask)
		if m.In8(0x3c6) != mask {
			t.Fatal("遮罩讀回")
		}
		pal := m.Palette()
		for i := 0; i < 256; i++ {
			for ch := 0; ch < 3; ch++ {
				v := raw[int(byte(i)&mask)*3+ch]
				if pal[i][ch] != v<<2|v>>4 {
					t.Fatalf("色號 %02x 遮罩 %02x 通道 %d", i, mask, ch)
				}
			}
		}
		if m.DAC != raw {
			t.Fatal("遮罩改到原始調色盤")
		}
	}
	m.Out8(0x3c8, 255)
	m.Out8(0x3c9, 0x7f)
	m.Out8(0x3c6, 0x55)
	m.In8(0x3c6)
	m.Out8(0x3c9, 0x82)
	m.Out8(0x3c9, 3)
	m.Out8(0x3c9, 4)
	if m.DAC[765] != 63 || m.DAC[766] != 2 || m.DAC[767] != 3 || m.DAC[0] != 4 || m.dacIndex != 0 || m.dacPhase != 1 {
		t.Fatal("中途讀寫遮罩打斷 RGB 或索引環繞")
	}
	m.SetVideoMode(0x12)
	if m.In8(0x3c6) != 0xff {
		t.Fatal("模式設定未恢復遮罩")
	}
}

func TestVGAPELMaskPlanarConsumer(t *testing.T) {
	m := New()
	m.SetVideoMode(0x12)
	m.In8(0x3da)
	m.Out8(0x3c0, 7)
	m.Out8(0x3c0, 0x1e)
	m.Out8(0x3c8, 0x0e)
	for _, v := range []byte{63, 32, 1} {
		m.Out8(0x3c9, v)
	}
	m.Out8(0x3c8, 0x1e)
	for _, v := range []byte{1, 2, 3} {
		m.Out8(0x3c9, v)
	}
	m.PlanarPutPixel(0, 0x80, 7)
	m.Out8(0x3c6, 0x0f)
	w, h, rgb := m.PlanarRGB()
	if w != 640 || h != 480 || !bytes.Equal(rgb[:3], []byte{255, 130, 4}) || m.VGA.DACIndex(7) != 0x1e {
		t.Fatal("平面屬性索引後未套遮罩")
	}
	m.Out8(0x3c6, 0xff)
	_, _, rgb = m.PlanarRGB()
	if !bytes.Equal(rgb[:3], []byte{4, 8, 12}) {
		t.Fatal("恢復遮罩未恢復原色")
	}
}

func TestVGAPELMaskBothStatePaths(t *testing.T) {
	for _, mask := range []byte{0, 0x5a, 0xff} {
		m := New()
		m.Out8(0x3c8, 12)
		m.Out8(0x3c9, 1)
		m.Out8(0x3c6, mask)
		snap := m.Snapshot()
		var saved bytes.Buffer
		if err := m.SaveState(&saved); err != nil {
			t.Fatal(err)
		}
		m.Out8(0x3c6, ^mask)
		m.Out8(0x3c8, 42)
		m.Restore(snap)
		loaded := New()
		if err := loaded.LoadState(&saved); err != nil {
			t.Fatal(err)
		}
		for _, restored := range []*Machine{m, loaded} {
			if restored.In8(0x3c6) != mask || restored.dacIndex != 12 || restored.dacPhase != 1 {
				t.Fatal("還原漏掉遮罩或 RGB 中途狀態")
			}
			restored.Out8(0x3c9, 2)
			restored.Out8(0x3c9, 3)
			if restored.DAC[36] != 1 || restored.DAC[37] != 2 || restored.DAC[38] != 3 || restored.dacIndex != 13 || restored.dacPhase != 0 {
				t.Fatal("還原後 RGB 沒接著寫")
			}
		}
	}
}

func TestVGAPELMaskLegacyV2State(t *testing.T) {
	// 真正省略新欄位，避免把新結構的零值當成舊線上格式。
	legacy := struct {
		Magic            string
		Version          int
		Mem, DAC, Planes []byte
		Ports            map[uint16]byte
	}{Magic: stateMagic, Version: 2, Mem: make([]byte, MemSize), DAC: make([]byte, 768), Planes: New().VGA.Raw(), Ports: map[uint16]byte{0x3c6: 0}}
	legacy.DAC[3] = 63
	var saved bytes.Buffer
	if err := gob.NewEncoder(&saved).Encode(legacy); err != nil {
		t.Fatal(err)
	}
	m := New()
	m.Out8(0x3c6, 0)
	if err := m.LoadState(&saved); err != nil {
		t.Fatal(err)
	}
	if m.In8(0x3c6) != 0xff || m.Palette()[1][0] != 255 {
		t.Fatal("舊 v2 未保留無遮罩查色，不得從 Ports 猜值")
	}
}

func TestLEVGAPELMaskConsumerAndIsolation(t *testing.T) {
	p := NewLEOPLPorts()
	before := p.State()
	if v, ok := p.In8(0x3c6); !ok || v != 0xff {
		t.Fatal("LE 遮罩讀取未接")
	}
	for _, ev := range []struct {
		port  uint16
		value byte
	}{{0x3c8, 3}, {0x3c9, 63}, {0x3c6, 0x0f}, {0x3c9, 32}, {0x3c9, 0}} {
		if !p.Out8(ev.port, ev.value) {
			t.Fatal("LE DAC 寫入未接")
		}
	}
	m := &LEMachine{Mem: make([]byte, 0x110000)}
	m.CPU = cpu386.New(m)
	if !InstallLEVideo(m, p) {
		t.Fatal("LE 顯示安裝")
	}
	if m.Video.Palette()[0x13] != [3]byte{255, 130, 0} || p.State() != before {
		t.Fatal("LE 查色未遮罩或影響音訊狀態")
	}
	if v, ok := p.In8(0x3c6); !ok || v != 0x0f || p.Reads[0x3c6] != 2 || p.Writes[0x3c6] != 1 {
		t.Fatal("LE 遮罩讀回或診斷")
	}
	if p.Out8(0x3c7, 0) {
		t.Fatal("未定義 DAC 讀取週期不得默認接受")
	}
	if _, ok := p.In8(0x3c7); ok {
		t.Fatal("未定義 DAC 狀態不得猜值")
	}
	m.CPU.R[cpu386.EAX] = 0x13
	if !m.Video.Handle(m.CPU) {
		t.Fatal("LE 模式設定")
	}
	if v, _ := p.In8(0x3c6); v != 0xff {
		t.Fatal("LE mode13 未初始化遮罩")
	}
}
