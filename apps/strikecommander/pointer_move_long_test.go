package strikecommander

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/session"
)

// dosgolem 規格 451 §3.2 的觸發案例：session 在第 4495000000 步交付一個移到 DOS (160, 75) 的
// 移動事件，對照 probe 以 -mouse-x 160 -mouse-y 75 -mouse-at 4495000000 連續執行的結果。
//
// 長測試。只在 DOSGOLEM_SC_ROOT 與 DOSGOLEM_SC_STATE_DIR 都設定時執行，否則 t.Skip
// （環境變數的意義同 trigger_long_test.go；這裡不需要字型）。
//
//   - 甲：從 p485/s449.state 起跑，停在第 4495000000 步交付移動，再跑到第 4496200000 步。
//   - 乙：從 p530/s4495b.state（probe 在同一步移動後、第 4495600000 步存的）跑到第 4496200000 步。
//   - 主斷言：甲的 Indexed 雜湊等於 probe 連續執行的 h.png；移動回合 DOSCallsCommitted 為 1；
//     甲在第 4496160311 步提交 "Transport to Strike Base"。
//   - 預期：甲、乙的 Digest 與 DOSMouse 全同。不符只記錄差異欄位，不判失敗。
//   - 負對照：甲不交付移動，Indexed 必須不同。

const (
	moveStartState  = "p485/s449.state"
	moveStartSHA    = "7dff6dd268ef38d04086aeaa3ba22d5e2b23c5dc953de97dfe73fc351b1e47a0"
	moveProbeState  = "p530/s4495b.state"
	moveProbeSHA    = "34c5b2e5a50131cda4537428ad0c1e4d999a3302a6bc6e60c34ba177fc3b48cd"
	moveAt          = 4495000000
	moveStop        = 4496200000
	moveDOSX        = 160
	moveDOSY        = 75
	moveIndexedWant = "133eaa9464a191e0f71cedf63acce990da37a19eadaa332e425b510f541ad779"
)

type moveRun struct {
	digest    session.StateDigest
	mouse     session.DOSMouseState
	moveCalls uint64
	commits   []Commit
}

// bootSC 複製設定樹、核對 SC.CFG，以 SessionConfig 建 owner 並從狀態檔啟動。
func bootSC(t *testing.T, state, sha string, obs session.StepObserver) (*session.Owner, session.BootReceipt) {
	t.Helper()
	root := t.TempDir()
	if err := CopyTree(os.Getenv(scRootEnv), root); err != nil {
		t.Fatal(err)
	}
	cfg, err := os.ReadFile(filepath.Join(root, "SC.CFG"))
	if err != nil {
		t.Fatal(err)
	}
	if s := sha256.Sum256(cfg); hex.EncodeToString(s[:]) != scCFGSHA256 {
		t.Fatalf("SC.CFG SHA-256 %x，要 %s", s, scCFGSHA256)
	}
	raw, err := os.ReadFile(filepath.Join(os.Getenv(scStateDirEnv), state))
	if err != nil {
		t.Fatal(err)
	}
	var want [32]byte
	if _, err := hex.Decode(want[:], []byte(sha)); err != nil {
		t.Fatal(err)
	}
	o, err := session.New(SessionConfig(obs))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { o.Close() })
	boot, err := o.BootFromState(session.StateInput{State: raw, ExpectedStateSHA256: want, SaveRoot: root, RebaseFrom: []string{"/scratch"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s：BootReceipt Steps=%d", state, boot.Steps)
	return o, boot
}

// advanceTo 交付一批（每次 Advance 前都要先 Deliver），再從第 from 步跑到絕對步數 stop。
func advanceTo(t *testing.T, o *session.Owner, moves []host.MouseEvent, from, stop uint64) session.InputReceipt {
	t.Helper()
	if stop <= from {
		t.Fatalf("目標第 %d 步不大於目前第 %d 步", stop, from)
	}
	v, err := o.View()
	if err != nil {
		t.Fatal(err)
	}
	in, err := o.Deliver(session.CapturedUpdate{StartedPanel: v.Panel, Layout: v.Layout, SourceGeneration: v.SourceGeneration, PointerMoves: moves})
	if err != nil {
		t.Fatal(err)
	}
	r, err := o.Advance(session.InstructionBudget(stop - from))
	if err != nil {
		t.Fatalf("Advance（停在第 %d 步）：%v", r.MachineStepsAfter, err)
	}
	if r.MachineStepsBefore != from || r.MachineStepsAfter != stop {
		t.Fatalf("從第 %d 步跑到第 %d 步，要 %d 到 %d", r.MachineStepsBefore, r.MachineStepsAfter, from, stop)
	}
	return in
}

func runMoveCase(t *testing.T, deliverMove bool) moveRun {
	t.Helper()
	var run moveRun
	lo := NewLabelObserver()
	lo.CursorY = moveDOSY
	lo.OnCommit = func(c Commit) { run.commits = append(run.commits, c) }
	o, boot := bootSC(t, moveStartState, moveStartSHA, &SessionObserver{Labels: lo})
	advanceTo(t, o, nil, boot.Steps, moveAt)
	var moves []host.MouseEvent
	if deliverMove {
		v, err := o.View()
		if err != nil {
			t.Fatal(err)
		}
		s := int(v.Layout.Scale)
		moves = []host.MouseEvent{{Kind: host.MouseEventMove, X: moveDOSX * s, Y: v.Layout.ChromeHeight + moveDOSY*s, Target: host.MouseTargetCanvas}}
	}
	in := advanceTo(t, o, moves, moveAt, moveStop)
	run.moveCalls = in.DOSCallsCommitted
	var err error
	if run.digest, err = o.Digest(); err != nil {
		t.Fatal(err)
	}
	if run.mouse, err = o.DOSMouse(); err != nil {
		t.Fatal(err)
	}
	for i, c := range run.commits {
		for _, l := range c.Labels {
			t.Logf("提交 %d：blit 入口第 %d 步 %q X0 %d X1 %d Y %d", i, c.BlitEntry, l.Key, l.X0, l.X1, l.Y)
		}
	}
	t.Logf("停止 %d：Indexed %x，DOSMouse %+v，移動回合 DOSCallsCommitted %d", run.digest.Steps, run.digest.IndexedSHA256, run.mouse, run.moveCalls)
	return run
}

func TestPointerMoveTrigger(t *testing.T) {
	for _, e := range []string{scRootEnv, scStateDirEnv} {
		if os.Getenv(e) == "" {
			t.Skip(e + " 未設定：451 §3.2 觸發案例需要設定樹與狀態檔")
		}
	}

	// 甲與主斷言。
	a := runMoveCase(t, true)
	if got := hex.EncodeToString(a.digest.IndexedSHA256[:]); got != moveIndexedWant {
		t.Errorf("甲的 Indexed SHA-256 %s，要 %s", got, moveIndexedWant)
	}
	if a.moveCalls != 1 {
		t.Errorf("甲的移動回合 DOSCallsCommitted %d，要 1", a.moveCalls)
	}
	found := false
	for _, c := range a.commits {
		if c.BlitEntry != 4496160311 {
			continue
		}
		found = true
		if len(c.Labels) != 1 {
			t.Errorf("第 4496160311 步的提交有 %d 筆標籤，要 1", len(c.Labels))
			break
		}
		l := c.Labels[0]
		if l.Key != "Transport to Strike Base" || l.X0 != 68 || l.X1 != 251 || l.Y != 175 {
			t.Errorf("第 4496160311 步提交 %q X0 %d X1 %d Y %d，要 \"Transport to Strike Base\" 68 251 175", l.Key, l.X0, l.X1, l.Y)
		}
	}
	if !found {
		t.Error("甲沒有第 4496160311 步的提交")
	}

	// 乙與預期：不符只記錄。
	o, boot := bootSC(t, moveProbeState, moveProbeSHA, nil)
	advanceTo(t, o, nil, boot.Steps, moveStop)
	b, err := o.Digest()
	if err != nil {
		t.Fatal(err)
	}
	bm, err := o.DOSMouse()
	if err != nil {
		t.Fatal(err)
	}
	var diffs []string
	ad := a.digest
	if ad.Steps != b.Steps {
		diffs = append(diffs, "Steps")
	}
	if ad.MemorySHA256 != b.MemorySHA256 {
		diffs = append(diffs, "Memory")
	}
	if ad.CPUSHA256 != b.CPUSHA256 {
		diffs = append(diffs, "CPU")
	}
	if ad.IndexedSHA256 != b.IndexedSHA256 {
		diffs = append(diffs, "Indexed")
	}
	if ad.PaletteSHA256 != b.PaletteSHA256 {
		diffs = append(diffs, "Palette")
	}
	if ad.ExtSHA256 != b.ExtSHA256 {
		diffs = append(diffs, "Ext")
	}
	if a.mouse != bm {
		diffs = append(diffs, "DOSMouse")
	}
	t.Logf("乙：Memory %x CPU %x Indexed %x Palette %x Ext %s DOSMouse %+v", b.MemorySHA256, b.CPUSHA256, b.IndexedSHA256, b.PaletteSHA256, b.ExtSHA256, bm)
	t.Logf("甲：Memory %x CPU %x Indexed %x Palette %x Ext %s DOSMouse %+v", ad.MemorySHA256, ad.CPUSHA256, ad.IndexedSHA256, ad.PaletteSHA256, ad.ExtSHA256, a.mouse)
	if len(diffs) == 0 {
		t.Log("預期：甲、乙的 Digest 與 DOSMouse 全同")
	} else {
		t.Logf("預期不符（不判甲錯）：差異欄位 %v", diffs)
	}

	// 負對照：不交付移動，Indexed 必須不同。
	n := runMoveCase(t, false)
	if n.digest.IndexedSHA256 == a.digest.IndexedSHA256 {
		t.Error("負對照：不交付移動時 Indexed 與甲相同")
	}
}
