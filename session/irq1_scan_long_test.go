package session

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// docs/spec/449 §3.2 的觸發案例：SCCD.EXE 冷開機到 MENU，經 ScanEvents 按住下方向鍵與上方向鍵，
// 與 cmd/probe `-hold 50@1010000000+20000000,48@1050000000+3000000` 的結果對拍。
//
// 長測試。與 448 的 TestLegacyPMProfileTrigger 相同，只在 DOSGOLEM_SC_ROOT 指向
// 《Strike Commander》的 SB16 設定樹時執行，否則 t.Skip。每個子測試把該樹實體複製到
// t.TempDir()，每次約 5 到 10 分鐘，逐一以
// `go test ./session -run 'TestIRQ1ScanTrigger/<子測試>' -timeout 30m -v` 執行。
//
// 子測試：
//   - Base：§3.2 的期望值全部要符合。
//   - NoRepeats：§3.2 的負對照，Advance 不呼叫 ScheduleHeldRepeats（測試私有開關
//     Owner.noHeldRepeats），只以 t.Log 記錄 KeyIRQs 總數與暫存器是否改變。

const (
	irq1TriggerFinal   = 1100000000
	irq1Indexed103     = "bc24e8ab4c8d6bc817911e9f8e71a3961125cf823aa74e8d96d5e5fb36d0e114"
	irq1Indexed109     = "02e210d55663e8ec7e3d9efc059d3cdd3741eed4191c55b573720f7e1fa45098"
	irq1Palette        = "ea48da3ee0628b0cf7609482ed5946e7ae78939b8c45d2fb0850a8bb566a5b60"
	irq1RegsWant       = "CS:IP 0018:581D AX BAB6 BX 13DC CX 2000 DX 0020 DS 0020 ES 00D8 SS:SP 0050:13BA"
	irq1KeyIRQsWant    = 18
	irq1KeyStallsWant  = 0
	irq1Digest103Steps = 1030000000
	irq1Digest109Steps = 1090000000
)

// irq1RegObserver 在 BeforeStep 看到 Steps == at 時記下暫存器。
type irq1RegObserver struct {
	at   uint64
	regs string
	seen bool
}

func (r *irq1RegObserver) BeforeStep(v StepView) error {
	if v.Steps() == r.at && !r.seen {
		r.seen = true
		r.regs = fmt.Sprintf("CS:IP %04X:%04X AX %04X BX %04X CX %04X DX %04X DS %04X ES %04X SS:SP %04X:%04X",
			v.CS(), v.IP(), v.AX(), v.BX(), v.CX(), v.DX(), v.DS(), v.ES(), v.SS(), v.SP())
	}
	return nil
}
func (r *irq1RegObserver) VideoWrite(machine.VideoWrite) {}
func (r *irq1RegObserver) Frame([]byte, [256][3]uint8)   {}

// irq1TriggerTurn 是一個回合：在目前的回合邊界送 ScanEvents，再跑 budget 道。
type irq1TriggerTurn struct {
	events []ScanEvent
	budget InstructionBudget
}

// irq1TriggerTurns 是 §3.2 的預算序列。各回合起點（m.Steps）依序是
// 0、1010000000、1030000000、1050000000、1053000000、1090000000、1100000000。
var irq1TriggerTurns = []irq1TriggerTurn{
	{nil, 1010000000},
	{[]ScanEvent{{Scan: 0x50}}, 20000000},
	{[]ScanEvent{{Scan: 0x50, Break: true}}, 20000000},
	{[]ScanEvent{{Scan: 0x48}}, 3000000},
	{[]ScanEvent{{Scan: 0x48, Break: true}}, 37000000},
	{nil, 10000000},
	{nil, 1},
}

type irq1TriggerResult struct {
	advanceErr          error
	steps               uint64
	indexed103, pal103  string
	indexed109, pal109  string
	regs                string
	keyIRQs, keyStalls  uint64
	scanEventsCommitted uint64
}

func runIRQ1Trigger(t *testing.T, noRepeats bool) irq1TriggerResult {
	t.Helper()
	src := os.Getenv(legacyTriggerEnv)
	root := t.TempDir()
	if err := copyLegacyTree(src, root); err != nil {
		t.Fatal(err)
	}
	if got := fileSHA256(t, filepath.Join(root, "SC.CFG")); got != legacySCCFGSHA256 {
		t.Fatalf("SC.CFG SHA-256 %s，要 %s", got, legacySCCFGSHA256)
	}
	exe, err := os.ReadFile(filepath.Join(root, "SCCD.EXE"))
	if err != nil {
		t.Fatal(err)
	}
	var want [32]byte
	if _, err := hex.Decode(want[:], []byte(legacySCCDSHA256)); err != nil {
		t.Fatal(err)
	}
	obs := &irq1RegObserver{at: irq1TriggerFinal}
	o, err := New(legacyTriggerConfig(obs, true))
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	o.noHeldRepeats = noRepeats
	if _, err := o.BootOriginal(BootInput{EXE: exe, ExpectedEXESHA256: want, SaveRoot: root}); err != nil {
		t.Fatal(err)
	}
	var res irq1TriggerResult
	for _, turn := range irq1TriggerTurns {
		// 回合邊界：先取 Digest 再 Deliver（Digest 在回合進行中被拒）。
		if at := o.machine.Steps; at == irq1Digest103Steps || at == irq1Digest109Steps {
			d, err := o.Digest()
			if err != nil {
				t.Fatal(err)
			}
			idx, pal := hex.EncodeToString(d.IndexedSHA256[:]), hex.EncodeToString(d.PaletteSHA256[:])
			if at == irq1Digest103Steps {
				res.indexed103, res.pal103 = idx, pal
			} else {
				res.indexed109, res.pal109 = idx, pal
			}
		}
		v, err := o.View()
		if err != nil {
			res.advanceErr = err
			break
		}
		in, err := o.Deliver(CapturedUpdate{StartedPanel: v.Panel, Layout: v.Layout, SourceGeneration: v.SourceGeneration, ScanEvents: turn.events})
		if err != nil {
			res.advanceErr = err
			break
		}
		res.scanEventsCommitted += in.ScanEventsCommitted
		r, err := o.Advance(turn.budget)
		res.steps = r.MachineStepsAfter
		res.keyIRQs += r.KeyIRQs
		res.keyStalls += r.KeyStalls
		t.Logf("回合 %d..%d：ScanPending %d→%d，KeyIRQs %d，KeyStalls %d",
			r.MachineStepsBefore, r.MachineStepsAfter, r.ScanPendingBefore, r.ScanPendingAfter, r.KeyIRQs, r.KeyStalls)
		if err != nil {
			res.advanceErr = err
			break
		}
	}
	res.regs = obs.regs
	return res
}

func TestIRQ1ScanTrigger(t *testing.T) {
	if os.Getenv(legacyTriggerEnv) == "" {
		t.Skip(legacyTriggerEnv + " 未設定：449 §3.2 觸發案例需要原版素材的複製樹")
	}
	t.Run("Base", func(t *testing.T) {
		res := runIRQ1Trigger(t, false)
		t.Logf("結果 %+v", res)
		if res.advanceErr != nil {
			t.Fatalf("Advance：%v", res.advanceErr)
		}
		if res.steps != irq1TriggerFinal+1 {
			t.Fatalf("MachineStepsAfter %d，要 %d", res.steps, irq1TriggerFinal+1)
		}
		if res.scanEventsCommitted != 4 {
			t.Errorf("ScanEventsCommitted 總數 %d，要 4", res.scanEventsCommitted)
		}
		for _, c := range []struct{ name, got, want string }{
			{"第 1030000000 步 IndexedSHA256", res.indexed103, irq1Indexed103},
			{"第 1030000000 步 PaletteSHA256", res.pal103, irq1Palette},
			{"第 1090000000 步 IndexedSHA256", res.indexed109, irq1Indexed109},
			{"第 1090000000 步 PaletteSHA256", res.pal109, irq1Palette},
			{"第 1100000000 步暫存器", res.regs, irq1RegsWant},
		} {
			if c.got != c.want {
				t.Errorf("%s %q，要 %q", c.name, c.got, c.want)
			}
		}
		if res.keyIRQs != irq1KeyIRQsWant {
			t.Errorf("KeyIRQs 總數 %d，要 %d", res.keyIRQs, irq1KeyIRQsWant)
		}
		if res.keyStalls != irq1KeyStallsWant {
			t.Errorf("KeyStalls 總數 %d，要 %d", res.keyStalls, irq1KeyStallsWant)
		}
	})
	t.Run("NoRepeats", func(t *testing.T) {
		res := runIRQ1Trigger(t, true)
		t.Logf("結果 %+v", res)
		changed := func(same bool) string {
			if res.advanceErr != nil || !same {
				return "有改變"
			}
			return "未改變"
		}
		t.Logf("負對照 NoRepeats：KeyIRQs %d（%s），暫存器 %q（%s），Indexed103 %s，Indexed109 %s",
			res.keyIRQs, changed(res.keyIRQs == irq1KeyIRQsWant), res.regs, changed(res.regs == irq1RegsWant),
			changed(res.indexed103 == irq1Indexed103), changed(res.indexed109 == irq1Indexed109))
	})
}
