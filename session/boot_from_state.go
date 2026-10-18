// Sealed owner boot from a cmd/probe state file (docs/spec/450).

package session

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/wicanr2/dosgolem/internal/state"
)

// maxBootStateSize is a corrupt-input guard on the gzip-compressed state
// file (s449.state measures 3.6 MB).
const maxBootStateSize = 64 << 20

// StateInput carries one probe -save-state file and the writable save root.
type StateInput struct {
	State               []byte // probe -save-state 的整個檔案內容
	ExpectedStateSHA256 [32]byte
	SaveRoot            string // 同 BootInput.SaveRoot
	// RebaseFrom 是狀態檔裡開檔路徑的舊前綴（probe 的 -scratch 等），讀檔時改寫到 SaveRoot 底下。
	// 狀態檔內記的 Root 一律包含在內，不必重複給。
	RebaseFrom []string
}

// BootFromState installs DOS, loads the state file and enters Running
// (docs/spec/450 §2.3).  Step 1 rejects without touching phase; steps 2-6
// fail the owner.  The caller's State slice is never retained.  No EXE is
// loaded: the program image comes from the state file.  On success m.Steps is
// the state file's step count, recorded in BootReceipt.Steps.
func (o *Owner) BootFromState(input StateInput) (BootReceipt, error) {
	// 第 1 步：前置檢查，不改 phase。
	if o == nil {
		return BootReceipt{}, errors.New("session: Owner 不得為 nil")
	}
	if o.phase != PhaseBooting {
		return BootReceipt{}, fmt.Errorf("session: phase %d 不可開機", o.phase)
	}
	if o.generation == ^uint64(0) {
		return BootReceipt{}, errors.New("session: 啟動前 SourceGeneration 溢位")
	}
	if len(input.State) == 0 || len(input.State) > maxBootStateSize {
		return BootReceipt{}, fmt.Errorf("session: 狀態檔長度 %d 不合法", len(input.State))
	}
	var zero [32]byte
	if input.ExpectedStateSHA256 == zero {
		return BootReceipt{}, errors.New("session: 缺少狀態檔 SHA-256 期望值")
	}
	st := append([]byte(nil), input.State...)
	sum := sha256.Sum256(st)
	if sum != input.ExpectedStateSHA256 {
		return BootReceipt{}, errors.New("session: 狀態檔 SHA-256 不符")
	}
	if err := checkSaveRoot(input.SaveRoot); err != nil {
		return BootReceipt{}, err
	}
	if o.clockPercent != 0 {
		return BootReceipt{}, fmt.Errorf("session: 從狀態檔啟動不接受 ClockPercent（%d）：讀檔會依存檔改寫時脈（docs/spec/450 §2.4）", o.clockPercent)
	}

	// 第 2 步。
	o.dos.Root = input.SaveRoot
	o.dos.Scratch = input.SaveRoot
	o.dos.RebaseTo = input.SaveRoot
	o.dos.RebaseFrom = append([]string(nil), input.RebaseFrom...)
	// 第 3 步：state.Load 的前提是 Install 已跑過；讀檔後不能再跑（會重設 freeSeg、arena、curPSP）。
	o.dos.Install()
	// 第 4 步。Machine.LoadState 先失敗時 DOS.LoadState 不會被呼叫，這裡也清一次。
	err := state.LoadFrom(bytes.NewReader(st), o.machine, o.dos)
	o.dos.RebaseTo, o.dos.RebaseFrom = "", nil
	if err != nil {
		return BootReceipt{}, o.fail(fmt.Errorf("session: 讀狀態檔：%w", err))
	}
	// 第 5 步：存檔時有鍵按著的話，MouseBridge 不知道，放開永遠不會送出。
	if o.dos.Mouse.Buttons != 0 {
		return BootReceipt{}, o.fail(fmt.Errorf("session: 狀態檔存於滑鼠鍵按住時（Buttons=%#x）", o.dos.Mouse.Buttons))
	}
	// 第 6 步：狀態檔覆蓋了 stub 區與客體碼（447 §2.1）。
	if !o.noKbdRewrite {
		if o.machine.KbdIntercept {
			if err := o.machine.EnableKbdIntercept(); err != nil {
				return BootReceipt{}, o.fail(fmt.Errorf("session: KbdIntercept：%w", err))
			}
		} else if err := o.machine.CheckKbdInterceptAbsent(); err != nil {
			return BootReceipt{}, o.fail(fmt.Errorf("session: %w", err))
		}
	}
	// 第 7 步。
	o.phase = PhaseRunning
	o.generation++
	return BootReceipt{Phase: PhaseRunning, StateSHA256: sum, Steps: o.machine.Steps, LegacyState: o.machine.LegacyState}, nil
}
