package machine

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"testing"
)

// 規格 461：讀入狀態檔後，通道 0 讀數一律取 IRQ0 實際使用的分頻（m.PITDiv）。

func pit461Program(m *Machine, div uint16) {
	m.Out8(0x43, 0x36)
	m.Out8(0x40, uint8(div))
	m.Out8(0x40, uint8(div>>8))
}

func pit461Save(t *testing.T, m *Machine) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := m.SaveState(&b); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func pit461Load(t *testing.T, m *Machine, b []byte) {
	t.Helper()
	if err := m.LoadState(bytes.NewReader(b)); err != nil {
		t.Fatal(err)
	}
}

// §5 第 1 條：存讀檔後 PITDivisor 為 9943、StepsPerSecondNow 逐位元相同、PITHz 與常數相同、PITProgrammed 為假。
func TestPIT461SaveLoad(t *testing.T) {
	m := New()
	pit461Program(m, 9943)
	if m.PITDiv != 9943 || m.PITDivisor() != 9943 || !m.PITProgrammed() {
		t.Fatalf("存檔前：PITDiv=%d PITDivisor=%d programmed=%t", m.PITDiv, m.PITDivisor(), m.PITProgrammed())
	}
	sps := m.StepsPerSecondNow()
	n := New()
	pit461Load(t, n, pit461Save(t, m))
	if got := n.PITDivisor(); got != 9943 {
		t.Errorf("讀檔後 PITDivisor=%d，應為 9943", got)
	}
	if got := n.StepsPerSecondNow(); math.Float64bits(got) != math.Float64bits(sps) {
		t.Errorf("讀檔後 StepsPerSecondNow=%v（%016x），存檔前 %v（%016x）", got, math.Float64bits(got), sps, math.Float64bits(sps))
	}
	// 規格寫「與常數 315e6/264/9943 相同」。Go 常數運算是精確有理數、只捨入一次；PITHz 是執行期兩次捨入。
	// 兩種寫法都記下，斷言以執行期次序（PITBaseHz ÷ float64(9943)）為準；常數寫法另斷言並在不同時報告。
	runtimeOrder := PITBaseHz / float64(uint32(9943))
	const exact = 315e6 / 264 / 9943
	hz := n.PITHz()
	t.Logf("PIT461 PITHz=%016x runtime=%016x const=%016x", math.Float64bits(hz), math.Float64bits(runtimeOrder), math.Float64bits(exact))
	if math.Float64bits(hz) != math.Float64bits(runtimeOrder) {
		t.Errorf("讀檔後 PITHz=%v，PITBaseHz/9943=%v", hz, runtimeOrder)
	}
	if math.Float64bits(hz) != math.Float64bits(exact) {
		t.Errorf("讀檔後 PITHz=%v，常數 315e6/264/9943=%v（差在捨入次數，見規格 461 §5 第 1 條）", hz, float64(exact))
	}
	if n.PITProgrammed() {
		t.Error("讀檔後 PITProgrammed 應為假（狀態檔不存寫入計數）")
	}
}

// §5 第 2 條：先寫過 PIT 的機器載入一份本次載入後未寫過 PIT 的狀態。
func TestPIT461LoadIntoProgrammedMachine(t *testing.T) {
	cases := []struct {
		name string
		mk   func() []byte
		want uint32
	}{
		{"開機未寫過", func() []byte { return pit461Save(t, New()) }, PITDefaultDivisor},
		// 狀態檔的 PITDiv 不是 65,536：先在另一台寫 9943 存檔，再由讀檔後未再寫的機器存一次。
		{"讀檔後未再寫、PITDiv 9943", func() []byte {
			a := New()
			pit461Program(a, 9943)
			b := New()
			pit461Load(t, b, pit461Save(t, a))
			if b.PITProgrammed() {
				t.Fatal("中繼機器讀檔後 PITProgrammed 應為假")
			}
			return pit461Save(t, b)
		}, 9943},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st := c.mk()
			old := New()
			pit461Program(old, 4096)
			if !old.PITProgrammed() || old.PITDivisor() != 4096 {
				t.Fatalf("舊機器：programmed=%t PITDivisor=%d", old.PITProgrammed(), old.PITDivisor())
			}
			pit461Load(t, old, st)
			if old.PITProgrammed() {
				t.Error("載入後 PITProgrammed 應為假")
			}
			if old.PITDiv != c.want {
				t.Fatalf("狀態檔的 PITDiv 讀回 %d，預期 %d", old.PITDiv, c.want)
			}
			if got := old.PITDivisor(); got != old.PITDiv {
				t.Errorf("載入後 PITDivisor=%d，狀態檔 PITDiv=%d", got, old.PITDiv)
			}
		})
	}
}

// §5 第 3 條：初值。
func TestPIT461Initial(t *testing.T) {
	m := New()
	if m.PITDiv != PITDefaultDivisor {
		t.Fatalf("New() 的 PITDiv=%d", m.PITDiv)
	}
	if got := m.PITDivisor(); got != PITDefaultDivisor {
		t.Errorf("New() 未寫 PIT：PITDivisor=%d", got)
	}
	if m.PITProgrammed() {
		t.Error("New() 未寫 PIT：PITProgrammed 應為假")
	}
	if got := (&Machine{}).PITDivisor(); got != PITDefaultDivisor {
		t.Errorf("零值 Machine：PITDivisor=%d", got)
	}
}

// 快照還原寫入計數（461 §2 第 3 條；speaker_test 的「還原之後回到未設定」另由該檔守）。
func TestPIT461SnapshotKeepsWrites(t *testing.T) {
	m := New()
	pit461Program(m, 9943)
	s := m.Snapshot()
	n := New()
	n.Restore(s)
	if !n.PITProgrammed() || n.PITDivisor() != 9943 {
		t.Errorf("Restore 後 programmed=%t PITDivisor=%d", n.PITProgrammed(), n.PITDivisor())
	}
}

// ---- §5 第 6 條：分歧抽樣 ----------------------------------------------------

// pit461Diff 是一處「舊解讀者（pit 型別）與 pitWrite → setPITDiv」不同之處。
type pit461Diff struct {
	Index      int    // 在 40h／43h 寫入序列中的序號
	Step       uint64 // 寫入當下的 m.Steps（LE 為 Sequence）
	Port       uint16
	Val        uint8
	OldDivisor uint32 // pit.divisor，0 映射 65,536
	PITDiv     uint32 // 同一筆寫入之後的 m.PITDiv
}

func (d pit461Diff) String() string {
	return fmt.Sprintf("#%d step=%d out %02Xh,%02Xh old=%d PITDiv=%d", d.Index, d.Step, d.Port, d.Val, d.OldDivisor, d.PITDiv)
}

// pit461ReplayResult 是一次重播的摘要。
type pit461ReplayResult struct {
	Writes     int // 40h、43h 寫入數
	Compared   int // haveLo 為假、做過比較的次數
	Diffs      []pit461Diff
	OldWrites  uint64 // 舊解讀者完整寫入次數（pit.writes）
	NewWrites  uint64 // setPITDiv 呼叫次數（pitWrites）
	FinalDiv   uint32 // 重播完的 m.PITDiv
	ProgDiffer bool   // 舊 PITProgrammed（writes>0）與新的不同
}

// pit461Replay 把 40h、43h 的寫入依序重播給一個獨立的 pit 值，同時重播給一台新機器的 pitWrite
// （與原機器同一個解讀者，setPITDiv 每次呼叫之後的 PITDiv 即（步數、分頻）序列）。每次重播後，
// pit.haveLo 為假就比較 pit.divisor（0 映射 65,536）與 PITDiv。
func pit461Replay(ws []PortWrite) pit461ReplayResult {
	var p pit
	m := New()
	var r pit461ReplayResult
	for _, w := range ws {
		if w.Port != 0x40 && w.Port != 0x43 {
			continue
		}
		p.out(w.Port, w.Val)
		m.pitWrite(w.Port, w.Val)
		r.Writes++
		if p.haveLo {
			continue
		}
		r.Compared++
		old := p.divisor
		if old == 0 {
			old = PITDefaultDivisor
		}
		if old != m.PITDiv {
			r.Diffs = append(r.Diffs, pit461Diff{Index: r.Writes - 1, Step: w.Step, Port: w.Port, Val: w.Val, OldDivisor: old, PITDiv: m.PITDiv})
		}
	}
	r.OldWrites, r.NewWrites, r.FinalDiv = p.writes, m.pitWrites, m.PITDiv
	r.ProgDiffer = (p.writes > 0) != (m.pitWrites > 0)
	return r
}

func pit461Report(t *testing.T, name string, r pit461ReplayResult) {
	t.Helper()
	t.Logf("PIT461SUMMARY name=%s writes=%d compared=%d diffs=%d oldWrites=%d newWrites=%d finalPITDiv=%d programmedDiffer=%t",
		name, r.Writes, r.Compared, len(r.Diffs), r.OldWrites, r.NewWrites, r.FinalDiv, r.ProgDiffer)
	for _, d := range r.Diffs {
		t.Logf("PIT461DIFF name=%s %s", name, d)
	}
}

// 重播器自檢：§2.1 的兩種分歧與 §2.1 末的鎖存例外，重播器都要報出來；一般序列報 0。
func TestPIT461ReplayDetectsDivergence(t *testing.T) {
	w := func(p uint16, v uint8) PortWrite { return PortWrite{Port: p, Val: v} }
	cases := []struct {
		name  string
		ws    []PortWrite
		diffs int
	}{
		{"一般序列", []PortWrite{w(0x43, 0x36), w(0x40, 0xD7), w(0x40, 0x26)}, 0},
		{"未送 43h 就寫 40h", []PortWrite{w(0x40, 0xD7), w(0x40, 0x26)}, 1},
		{"43h=B6h 之後寫 40h", []PortWrite{w(0x43, 0x36), w(0x40, 0x00), w(0x40, 0x10), w(0x43, 0xB6), w(0x42, 1), w(0x40, 0xD7), w(0x40, 0x26)}, 1},
		{"通道 2 後單數 40h 再鎖存", []PortWrite{w(0x43, 0x36), w(0x40, 0x00), w(0x40, 0x10), w(0x43, 0xB6), w(0x40, 0x11), w(0x43, 0x00), w(0x40, 0x22), w(0x40, 0x33)}, -1},
	}
	for _, c := range cases {
		r := pit461Replay(c.ws)
		pit461Report(t, c.name, r)
		if c.diffs >= 0 && len(r.Diffs) != c.diffs {
			t.Errorf("%s：不同 %d 處，預期 %d", c.name, len(r.Diffs), c.diffs)
		}
		if c.diffs < 0 && len(r.Diffs) == 0 && !r.ProgDiffer {
			t.Errorf("%s：相位錯開應被報出", c.name)
		}
	}
}

// FD2（LE 路徑）：固定原版 LE entry 自然執行到 AIL 寫完分頻（同 TestFD2ProgramsPITDivisor），
// 把 LEPortWrite 序列（Sequence 當步數）重播給兩個實模式解讀者。LE 自身的 pit 不在 461 範圍。
func TestPIT461SampleFD2(t *testing.T) {
	if os.Getenv("DOSGOLEM_FD2_ROOT") == "" {
		t.Skip("DOSGOLEM_FD2_ROOT 未設定")
	}
	m, _ := fixedFD2Machine(t)
	for steps := 0; steps < 5000 && m.CPU.EIP != 0x3e882; steps++ {
		if err := m.CPU.Step(); err != nil {
			t.Fatalf("FD2 step=%d: %v", steps, err)
		}
	}
	ws := make([]PortWrite, 0, len(m.PortLog))
	for _, w := range m.PortLog {
		ws = append(ws, PortWrite{Port: w.Port, Val: w.Value, Step: w.Sequence})
	}
	r := pit461Replay(ws)
	pit461Report(t, "FD2", r)
	if r.Writes == 0 {
		t.Error("FD2 沒有 40h／43h 寫入，抽樣無效")
	}
}
