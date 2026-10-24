package session

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// 013 §3.12、012 M11/M12：只有測試包內的觀察器持有 Owner。
// 注入發生在合法 BeforeStep 內；不擴張 StepView 的唯讀 API。
type m6DigestInjector struct {
	owner                 *Owner
	kind                  string
	calls, injected       int
	step                  uint64
	legal                 bool
	beforeByte, afterByte byte
	beforeHi, afterHi     uint16
}

func (x *m6DigestInjector) BeforeStep(v StepView) error {
	x.calls++
	if x.kind == "" || x.calls != 17 {
		return nil
	}
	// Advance 會先消費 turnPending；回呼合法性由視圖 token 與 viewOpen 判定。
	if !v.live() || v.o != x.owner || !x.owner.viewOpen {
		return fmt.Errorf("M6 注入沒有位於合法 BeforeStep")
	}
	x.legal, x.step = true, v.Steps()
	switch x.kind {
	case "M11":
		// 合成程式只操作 0800h；0900h 不參與執行、畫面或 DOS 檔案。
		a := uint32(machine.PSPSeg)*16 + 0x900
		x.beforeByte = x.owner.machine.Mem[a]
		x.owner.machine.Mem[a] ^= 1
		x.afterByte = x.owner.machine.Mem[a]
	case "M12":
		x.beforeHi = x.owner.machine.CPU.Hi[cpu.AX]
		x.owner.machine.CPU.Hi[cpu.AX] ^= 1
		x.afterHi = x.owner.machine.CPU.Hi[cpu.AX]
	default:
		return fmt.Errorf("未知的 M6 注入 %q", x.kind)
	}
	x.injected++
	return nil
}

func (*m6DigestInjector) VideoWrite(machine.VideoWrite) {}
func (*m6DigestInjector) Frame([]byte, [256][3]uint8)   {}

type m6DigestSnapshot struct {
	state StateDigest
	full  FullDigest
}

func m6DigestRead(t *testing.T, o *Owner) m6DigestSnapshot {
	t.Helper()
	d, err := o.Digest()
	if err != nil {
		t.Fatal(err)
	}
	f, err := o.MachineDigest()
	if err != nil {
		t.Fatal(err)
	}
	return m6DigestSnapshot{d, f}
}

func m6DigestRun(t *testing.T, root, kind string) (m6DigestSnapshot, m6DigestSnapshot, *m6DigestInjector) {
	t.Helper()
	o := startSyntheticOwner(t, prog454Session())
	t.Cleanup(func() {
		if err := o.Close(); err != nil {
			t.Error(err)
		}
	})
	// 各組共用同一路徑與同一份未改動的 fixture，沒有摘要路徑正規化。
	o.dos.Root, o.dos.Scratch = root, root
	x := &m6DigestInjector{owner: o, kind: kind}
	o.observer = x
	before := m6DigestRead(t, o)
	r, err := runTurn(t, o, InstructionBudget(2000))
	if err != nil {
		t.Fatal(err)
	}
	if r.MachineStepsBefore != 0 || r.MachineStepsAfter != 2000 || x.calls != 2000 {
		t.Fatalf("合成程式步數／回呼數不同：%d..%d，calls=%d", r.MachineStepsBefore, r.MachineStepsAfter, x.calls)
	}
	return before, m6DigestRead(t, o), x
}

func TestM6BeforeStepDigestNegative(t *testing.T) {
	root := t.TempDir()
	fixture := filepath.Join(root, "SYNTH.COM")
	code := prog454Session()
	stamp := time.Unix(946684800, 0)
	if err := os.WriteFile(fixture, code, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(fixture, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	initial, baseline, a := m6DigestRun(t, root, "")
	initial2, baseline2, a2 := m6DigestRun(t, root, "")
	if initial != initial2 || baseline != baseline2 || a.injected != 0 || a2.injected != 0 {
		t.Fatal("A/A 前提失敗：相同 fixture、SaveRoot、mtime 與 budget 的完整摘要不同")
	}
	t.Logf("A/A：初始與停止點 Digest 全欄位、MachineDigest.Machine、MachineDigest.DOS 相同；budget=2000；fixture_sha256=%x；mtime=%d", sha256.Sum256(code), stamp.Unix())
	t.Logf("A：memory=%x machine=%x dos=%x", baseline.state.MemorySHA256, baseline.full.Machine, baseline.full.DOS)
	for _, kind := range []string{"M11", "M12"} {
		t.Run(kind, func(t *testing.T) {
			before, changed, x := m6DigestRun(t, root, kind)
			if before != initial {
				t.Fatal("突變組的初始完整摘要不等於 A/A 基準")
			}
			if !x.legal || x.injected != 1 || x.step != 16 {
				t.Fatalf("注入前提失敗：合法=%t 次數=%d 步數=%d", x.legal, x.injected, x.step)
			}
			if changed.full.Machine == baseline.full.Machine || changed.full.DOS != baseline.full.DOS {
				t.Fatalf("%s 偵測器未捕獲機器差異，或 DOS 狀態意外改變", kind)
			}
			switch kind {
			case "M11":
				if x.afterByte != x.beforeByte^1 || changed.state.MemorySHA256 == baseline.state.MemorySHA256 {
					t.Fatal("M11 的單次記憶體注入沒有被 MemorySHA256 捕獲")
				}
				// 除了記憶體摘要，其他全部 StateDigest 欄位必須相同。
				normalized := changed.state
				normalized.MemorySHA256 = baseline.state.MemorySHA256
				if normalized != baseline.state {
					t.Fatal("M11 意外改變其他 Digest 欄位")
				}
			case "M12":
				if x.afterHi != x.beforeHi^1 || changed.state != baseline.state {
					t.Fatal("M12 應只改 Hi[AX]；既有 Digest 全欄位必須相同")
				}
			}
			t.Logf("%s 已捕獲：BeforeStep step=%d，注入一次；memory=%x machine=%x dos=%x；DOS 與 A 相同", kind, x.step, changed.state.MemorySHA256, changed.full.Machine, changed.full.DOS)
		})
	}
	got, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(got) != sha256.Sum256(code) || !info.ModTime().Equal(stamp) {
		t.Fatal("合成 fixture 的 bytes 或 mtime 被改動")
	}
}
