package machine

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"reflect"
	"testing"
)

func pcmDMA(m *Machine, page byte, addr uint16, n uint32) {
	for _, w := range [][2]uint16{{0x0a, 5}, {0x83, uint16(page)}, {0x0c, 0}, {2, addr & 255}, {2, addr >> 8}, {3, uint16((n - 1) & 255)}, {3, uint16((n - 1) >> 8)}, {0x0b, 0x49}, {0x0a, 1}} {
		m.Out8(w[0], byte(w[1]))
	}
	for _, b := range []byte{0x40, 166, 0x14, byte(n - 1), byte((n - 1) >> 8)} {
		m.Out8(0x22c, b)
	}
}

func TestSBPCMPhysicalAndWrap(t *testing.T) {
	for _, wrap := range []bool{false, true} {
		t.Run(fmt.Sprint(wrap), func(t *testing.T) {
			m := newSB8(t, []byte{0xeb, 0xfe}, func(m *Machine) {
				if err := m.SetExtendedMemoryKB(4096); err != nil {
					t.Fatal(err)
				}
				if err := m.SetProtectedModeProfile(true); err != nil {
					t.Fatal(err)
				}
			})
			tb := newTables(m, 0x40000)
			tb.identity(0, 0x100000, fPWU)
			addr := uint16(0x2000)
			n := uint32(256)
			if wrap {
				addr, n = 0xfff0, 32
			}
			for _, a := range []uint32{0x12000, 0x1f000, 0x10000} {
				tb.mapPage(a, a+0x20000)
			}
			want := make([]byte, n)
			for i := range want {
				want[i] = byte(i)
				phys := uint32(0x10000) | uint32(addr+uint16(i))
				m.Mem[phys] = want[i]
				m.Mem[phys+0x20000] = 255 - want[i]
			}
			tb.enable(t)
			var got SBPCMEvent
			m.ObserveSBPCM(func(e SBPCMEvent) { got = e })
			pcmDMA(m, 1, addr, n)
			if got.Kind != "start" || !bytes.Equal(got.Data, want) {
				t.Fatalf("physical wrap=%t: %+v", wrap, got)
			}
			if got.Data[0] == m.Peek8(0x10000|uint32(addr)) {
				t.Fatal("夾具未分開實體與線性")
			}
			if got.SHA256 != fmt.Sprintf("%x", sha256.Sum256(want)) {
				t.Fatal("hash")
			}
			// 回呼切片不與客體記憶體共用。
			got.Data[0] = 123
			if m.Mem[0x10000|uint32(addr)] != 0 {
				t.Fatal("Data 與 Mem 共用")
			}
		})
	}
}

func TestSBPCMEventsSpeakerOffAndLogShape(t *testing.T) {
	typ := reflect.TypeOf(SBDMAEvent{})
	if typ.NumField() != len(SBDMAEventFields) {
		t.Fatal("SBDMA 欄位數改變")
	}
	for i := 0; i < typ.NumField(); i++ {
		if typ.Field(i).Type.Kind() == reflect.Slice {
			t.Fatal("SBDMA 不可持有切片")
		}
	}
	m := newSB8(t, []byte{0xeb, 0xfe}, nil)
	for i := 0; i < 256; i++ {
		m.Mem[0x12000+i] = byte(i)
	}
	var evs []SBPCMEvent
	var dma []SBDMAEvent
	m.OnSBDMA = func(e SBDMAEvent) { dma = append(dma, e) }
	m.ObserveSBPCM(func(e SBPCMEvent) {
		if len(dma) == 0 || dma[len(dma)-1].Kind != e.Kind {
			t.Fatal("SBPCM 必須在 SBDMA 後")
		}
		evs = append(evs, e)
	})
	// D1h 前與 D3h 後的兩段都必須取樣。
	for _, off := range []bool{false, true} {
		if off {
			m.Out8(0x22c, 0xd1)
			m.Out8(0x22c, 0xd3)
		}
		pcmDMA(m, 1, 0x2000, 256)
		if dma[len(dma)-1].SpeakerOn {
			t.Fatal("夾具喇叭應關閉")
		}
		m.Steps += 1000
		m.Out8(0x22c, 0xd0)
		m.Steps += 1000
		m.Out8(0x22c, 0xd4)
		m.Steps = m.sb8.due
		m.sb8Poll()
	}
	// 靜音區塊不帶 Data，重設取消須照常轉發。
	for _, b := range []byte{0x80, 3, 0} {
		m.Out8(0x22c, b)
	}
	m.Steps = m.sb8.due
	m.sb8Poll()
	pcmDMA(m, 1, 0x2000, 256)
	m.Out8(0x226, 1)
	var filtered []SBDMAEvent
	for _, e := range dma {
		switch e.Kind {
		case "start", "pause", "resume", "cancel", "complete":
			filtered = append(filtered, e)
		}
	}
	if len(filtered) != len(evs) {
		t.Fatalf("events %d != %d", len(evs), len(filtered))
	}
	var start, due uint64
	for i, e := range evs {
		d := filtered[i]
		if e.Kind == "start" {
			start, due = e.Step, e.Due
		}
		if e.Kind == "resume" {
			start += e.Due - due
			due = e.Due
		}
		if e.Kind != d.Kind || e.Step != d.Step || e.Due != d.Due || e.Start != start || e.Transferred != d.Transferred {
			t.Fatalf("event %d: %+v DMA %+v", i, e, d)
		}
		if e.Kind == "start" && e.Command != 0x80 {
			if len(e.Data) != 256 {
				t.Fatalf("SpeakerOff start 無 Data: %+v", e)
			}
			for j, b := range e.Data {
				if b != byte(j) {
					t.Fatal("圖樣")
				}
			}
			if e.SHA256 != d.SHA256 || e.SHA256 != fmt.Sprintf("%x", sha256.Sum256(e.Data)) {
				t.Fatal("start SHA")
			}
		} else if e.Data != nil || e.SHA256 != "" {
			t.Fatal("只有非靜音 start 可帶 Data/SHA")
		}
	}
}

func TestSBPCMMutatedAndTransientReset(t *testing.T) {
	m := newSB8(t, []byte{0xeb, 0xfe}, nil)
	m.ObserveSBPCM(func(SBPCMEvent) {})
	pcmDMA(m, 1, 0x2000, 32)
	var saved bytes.Buffer
	if err := m.SaveState(&saved); err != nil {
		t.Fatal(err)
	}
	snapshot := m.Snapshot()
	m.Mem[0x12000]++
	m.Steps = m.sb8.due
	m.sb8Poll()
	if m.SBPCMMutated() != 1 {
		t.Fatalf("mutated %d", m.SBPCMMutated())
	}
	m.Restore(snapshot)
	if m.SBPCMMutated() != 0 || m.sb8pcmStartSHA != "" || m.sb8pcmData != nil {
		t.Fatal("Restore 暫態未歸零")
	}
	// 從播放中的狀態接續，沒有 start 就 complete 不計改寫。
	m.Mem[0x12000]++
	m.Steps = m.sb8.due
	m.sb8Poll()
	if m.SBPCMMutated() != 0 {
		t.Fatal("無 start 被誤算")
	}
	m.sb8pcmMutated = 7
	m.sb8pcmStartSHA = "stale"
	m.sb8pcmData = []byte{1}
	if err := m.LoadState(bytes.NewReader(saved.Bytes())); err != nil {
		t.Fatal(err)
	}
	if m.SBPCMMutated() != 0 || m.sb8pcmStartSHA != "" || m.sb8pcmData != nil {
		t.Fatal("LoadState 暫態未歸零")
	}
}

func TestSBPCMDoesNotChangeState(t *testing.T) {
	a, b := newSB8(t, []byte{0xeb, 0xfe}, nil), newSB8(t, []byte{0xeb, 0xfe}, nil)
	b.ObserveSBPCM(func(e SBPCMEvent) {
		if len(e.Data) > 0 {
			e.Data[0] ^= 255
		}
	})
	for _, m := range []*Machine{a, b} {
		pcmDMA(m, 1, 0x2000, 32)
		m.Steps += 100
		m.Out8(0x22c, 0xd0)
		m.Steps += 100
		m.Out8(0x22c, 0xd4)
		m.Steps = m.sb8.due
		m.sb8Poll()
	}
	var x, y bytes.Buffer
	if err := a.SaveState(&x); err != nil {
		t.Fatal(err)
	}
	if err := b.SaveState(&y); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(x.Bytes(), y.Bytes()) || !reflect.DeepEqual(a.SBDMALog, b.SBDMALog) {
		t.Fatal("SBPCM 改變狀態或 SBDMA 收據")
	}
}
