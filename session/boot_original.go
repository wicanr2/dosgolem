// Sealed owner private boot: LoadEXE, then Install, then Running.
//
// The original tree (read-only input) stays with the composition root; the
// owner only verifies the executable bytes and the save root handed to it.
// Observer installation, multi-layer composition and save/load are out of
// scope (Buck #16/#18 DRAFT).

package session

import (
	"github.com/wicanr2/dosgolem/internal/fsaccess"

	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strings"
)

// maxBootEXESize is a corrupt-input guard, not a version assertion.
// START.EXE measures 67,619 bytes; loadability is decided by LoadEXE.
const maxBootEXESize = 4 << 20

// BootInput carries the executable bytes and the writable save root for one
// private boot.  Paths to the read-only original tree never enter the owner.
type BootInput struct {
	EXE               []byte
	ExpectedEXESHA256 [32]byte
	SaveRoot          string
}

// BootReceipt proves one completed boot without exposing machine state.
// BootOriginal fills EXESHA256 and leaves the three state fields zero;
// BootFromState fills the state fields and leaves EXESHA256 zero
// (docs/spec/450 §2.1).
type BootReceipt struct {
	Phase     Phase
	EXESHA256 [32]byte
	// StateSHA256 是狀態檔的 SHA-256。
	StateSHA256 [32]byte
	// Steps 是讀檔後的 m.Steps（docs/spec/450 §2.3 的零步例外）。
	Steps uint64
	// LegacyState 是讀檔後的 m.LegacyState。
	LegacyState bool
}

// BootOriginal loads the executable, installs DOS and enters Running.
// Steps 1-3 below reject without touching phase; only step 4 may fail the
// owner.  The caller's EXE slice is never retained.
func (o *Owner) BootOriginal(input BootInput) (BootReceipt, error) {
	if o == nil {
		return BootReceipt{}, errors.New("session: Owner 不得為 nil")
	}
	if o.phase != PhaseBooting {
		return BootReceipt{}, fmt.Errorf("session: phase %d 不可開機", o.phase)
	}
	if len(input.EXE) == 0 || len(input.EXE) > maxBootEXESize {
		return BootReceipt{}, fmt.Errorf("session: EXE 長度 %d 不合法", len(input.EXE))
	}
	var zero [32]byte
	if input.ExpectedEXESHA256 == zero {
		return BootReceipt{}, errors.New("session: 缺少 EXE SHA-256 期望值")
	}
	if err := checkSaveRoot(input.SaveRoot); err != nil {
		return BootReceipt{}, err
	}
	exe := append([]byte(nil), input.EXE...)
	sum := sha256.Sum256(exe)
	if sum != input.ExpectedEXESHA256 {
		return BootReceipt{}, errors.New("session: EXE SHA-256 不符")
	}
	o.dos.Root = input.SaveRoot
	// 存檔根本身就是已驗證可寫的複製樹；設成 Scratch，遊戲寫檔才會真的落地
	// （dosgolem 規格 009：Scratch 為空時寫入只記帳）。見規格 235 補充。
	o.dos.Scratch = input.SaveRoot
	if err := o.machine.LoadEXE(exe); err != nil {
		return BootReceipt{}, o.fail(fmt.Errorf("session: 載入 EXE: %w", err))
	}
	// dos.Install 無回錯值：簽名上沒有錯分支可標；它只在 LoadEXE 成功後跑。
	if err := o.startLoadedMachine(); err != nil {
		return BootReceipt{}, err
	}
	return BootReceipt{Phase: PhaseRunning, EXESHA256: sum}, nil
}

// checkSaveRoot 是 BootOriginal 與 BootFromState 共用的 save root 檢查
// （docs/spec/450 §2.1）：拒絕空字串與含 NUL，再 Lstat、symlink、目錄、可寫。
// 不做 filepath.Clean：檢查與使用的是同一個字串。
func checkSaveRoot(root string) error {
	if root == "" {
		return errors.New("session: 缺少 save root")
	}
	if strings.IndexByte(root, 0) >= 0 {
		return errors.New("session: save root 含 NUL")
	}
	info, err := os.Lstat(root)
	if err != nil {
		return fmt.Errorf("session: save root 不可用: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("session: save root 不可為 symlink")
	}
	if !info.IsDir() {
		return errors.New("session: save root 不是目錄")
	}
	if err := fsaccess.Writable(root); err != nil {
		return fmt.Errorf("session: save root 不可寫: %v", err)
	}
	return nil
}
