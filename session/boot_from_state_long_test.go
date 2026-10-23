package session

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// docs/spec/450 §3.4 的觸發案例：從 s449.state（第 4490000000 步，基地畫面）啟動，
// 跑 400000 步後與 cmd/probe 的 probe543 對拍。
//
// 長測試。只在 DOSGOLEM_SC_ROOT 指向 448 的 SB16 設定樹、DOSGOLEM_SC_STATE 指向狀態檔時執行，
// 否則 t.Skip。每個子測試把設定樹實體複製到 t.TempDir() 當 SaveRoot。逐一以
// `go test ./session -run 'TestBootFromStateTrigger/<子測試>' -timeout 30m -v` 執行。
//
// 子測試：
//   - Base：§3.4 的步驟與三項斷言。
//   - NoKbdRewrite：§3.4 的負對照，不做 §2.3 第 6 步，只記錄斷言是否改變（t.Log），不判失敗。

const (
	bfsTriggerStateEnv = "DOSGOLEM_SC_STATE"
	// 規格 459 重錄：狀態檔是 strike_commander workplace/rerec459/scratch/p485/s449.state；Indexed、Palette、暫存器取自
	// 同狀態上 probe543 配方的重跑（rerec459/logs/ref-p543.log、rerec459/scratch/p530/g.png、g.pal），畫面與舊錨點相同。
	bfsStateSHA256 = "0e436dd863672caf125af083dde337bfe8fa0373fb8b3ad0cb45976c215623bb"
	bfsStateSteps  = 4490000000
	bfsAdvance     = 400000
	bfsTargetSteps = bfsStateSteps + bfsAdvance
	bfsIndexed     = "b26228b4a0ade66ba4e2d9ccd368d0c2add297653396bf3341c5ea3fcbe13883"
	bfsPalette     = "f7816e01a78377ade14d76015e51cb1d081a12158d69fd979fc55cc7ba3a2db0"
	bfsRegsWant    = "CS:IP 0CF2:06E5 AX 45EB BX 0000 CX 0000 DX 0000 DS 4E66 ES 4E66 SS:SP 4E66:FC46"
)

// bfsHandleFiles 是 s449.state 的 4 個檔案代號（都在 /orig 下，§3.4）。
var bfsHandleFiles = []string{"OPTTEST.EXE", "LILTRE.TRE", "BIGTRE.TRE", "VOCLIST.TRE"}

// bfsRegObserver 只記錄：BeforeStep 看到 Steps == bfsTargetSteps 時記下暫存器。
type bfsRegObserver struct {
	regs string
	seen bool
}

func (r *bfsRegObserver) BeforeStep(v StepView) error {
	if v.Steps() == bfsTargetSteps && !r.seen {
		r.seen = true
		r.regs = fmt.Sprintf("CS:IP %04X:%04X AX %04X BX %04X CX %04X DX %04X DS %04X ES %04X SS:SP %04X:%04X",
			v.CS(), v.IP(), v.AX(), v.BX(), v.CX(), v.DX(), v.DS(), v.ES(), v.SS(), v.SP())
	}
	return nil
}
func (r *bfsRegObserver) VideoWrite(machine.VideoWrite) {}
func (r *bfsRegObserver) Frame([]byte, [256][3]uint8)   {}

type bfsTriggerResult struct {
	err          error
	receiptSteps uint64
	steps        uint64
	indexed      string
	palette      string
	regs         string
}

func runBootFromStateTrigger(t *testing.T, noKbdRewrite bool) bfsTriggerResult {
	t.Helper()
	root := t.TempDir()
	if err := copyLegacyTree(os.Getenv(legacyTriggerEnv), root); err != nil {
		t.Fatal(err)
	}
	if got := fileSHA256(t, filepath.Join(root, "SC.CFG")); got != legacySCCFGSHA256 {
		t.Fatalf("SC.CFG SHA-256 %s，要 %s", got, legacySCCFGSHA256)
	}
	for _, name := range bfsHandleFiles {
		t.Logf("SaveRoot/%s SHA-256 %s", name, fileSHA256(t, filepath.Join(root, name)))
	}
	raw, err := os.ReadFile(os.Getenv(bfsTriggerStateEnv))
	if err != nil {
		t.Fatal(err)
	}
	var want [32]byte
	if _, err := hex.Decode(want[:], []byte(bfsStateSHA256)); err != nil {
		t.Fatal(err)
	}
	obs := &bfsRegObserver{}
	o, err := New(legacyTriggerConfig(obs, true))
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	o.noKbdRewrite = noKbdRewrite
	// 狀態檔的 Root 是 /orig（自動包含）；/scratch 在這份資料上用不到（§3.4）。
	receipt, err := o.BootFromState(StateInput{State: raw, ExpectedStateSHA256: want, SaveRoot: root, RebaseFrom: []string{"/scratch"}})
	if err != nil {
		t.Fatal(err)
	}
	res := bfsTriggerResult{receiptSteps: receipt.Steps}
	t.Logf("BootReceipt Steps=%d LegacyState=%t StateSHA256=%x", receipt.Steps, receipt.LegacyState, receipt.StateSHA256)
	turn := func(budget InstructionBudget) (TickReceipt, error) {
		v, err := o.View()
		if err != nil {
			return TickReceipt{}, err
		}
		if _, err := o.Deliver(CapturedUpdate{StartedPanel: v.Panel, Layout: v.Layout, SourceGeneration: v.SourceGeneration}); err != nil {
			return TickReceipt{}, err
		}
		return o.Advance(budget)
	}
	r, err := turn(bfsAdvance)
	res.steps = r.MachineStepsAfter
	if err != nil {
		res.err = err
		return res
	}
	d, err := o.Digest()
	if err != nil {
		t.Fatal(err)
	}
	res.indexed = hex.EncodeToString(d.IndexedSHA256[:])
	res.palette = hex.EncodeToString(d.PaletteSHA256[:])
	if _, err := turn(1); err != nil {
		res.err = err
	}
	res.regs = obs.regs
	t.Logf("KeyIRQs=%d KbdIntercepts=%d", o.machine.KeyIRQs, o.dos.KbdIntercepts)
	return res
}

func TestBootFromStateTrigger(t *testing.T) {
	if os.Getenv(legacyTriggerEnv) == "" || os.Getenv(bfsTriggerStateEnv) == "" {
		t.Skip(legacyTriggerEnv + "、" + bfsTriggerStateEnv + " 未設定：§3.4 觸發案例需要設定樹與狀態檔")
	}
	t.Run("Base", func(t *testing.T) {
		res := runBootFromStateTrigger(t, false)
		t.Logf("err=%v receiptSteps=%d steps=%d indexed=%s palette=%s regs=%s",
			res.err, res.receiptSteps, res.steps, res.indexed, res.palette, res.regs)
		if res.receiptSteps != bfsStateSteps {
			t.Fatalf("BootReceipt.Steps %d，要 %d", res.receiptSteps, bfsStateSteps)
		}
		if res.err != nil {
			t.Fatalf("Advance：%v", res.err)
		}
		if res.steps != bfsTargetSteps {
			t.Fatalf("MachineStepsAfter %d，要 %d", res.steps, bfsTargetSteps)
		}
		if res.indexed != bfsIndexed {
			t.Errorf("IndexedSHA256 %s，要 %s", res.indexed, bfsIndexed)
		}
		if res.palette != bfsPalette {
			t.Errorf("PaletteSHA256 %s，要 %s", res.palette, bfsPalette)
		}
		if res.regs != bfsRegsWant {
			t.Errorf("暫存器 %q，要 %q", res.regs, bfsRegsWant)
		}
	})
	t.Run("NoKbdRewrite", func(t *testing.T) {
		res := runBootFromStateTrigger(t, true)
		t.Logf("err=%v receiptSteps=%d steps=%d indexed=%s palette=%s regs=%s",
			res.err, res.receiptSteps, res.steps, res.indexed, res.palette, res.regs)
		changed := func(ok bool) string {
			if res.err != nil || !ok {
				return "有改變"
			}
			return "未改變"
		}
		t.Logf("負對照（不做第 6 步）：步數 %s，Indexed %s，Palette %s，暫存器 %s",
			changed(res.steps == bfsTargetSteps), changed(res.indexed == bfsIndexed),
			changed(res.palette == bfsPalette), changed(res.regs == bfsRegsWant))
	})
}
