// Package session owns one sealed DOS session lifecycle.
//
// It intentionally exposes phase and fault values only.  The machine, DOS,
// panel, and bridges stay private so an open frontend configuration cannot
// rebind one half of a turn to another target.
package session

import (
	"errors"
	"fmt"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/presentation"
)

// Phase is the monotonic lifecycle of one sealed session.
type Phase uint8

const (
	PhaseBooting Phase = iota + 1
	PhaseRunning
	PhaseStopped
	PhaseFailed
	PhaseClosed
)

// InstructionBudget is an explicit, finite instruction-attempt limit for one
// accepted running turn.  The caller owns the concrete limit; zero is never a
// valid running-turn budget.
type InstructionBudget uint64

// StopReason classifies the session-level result without discarding the raw
// machine stop code kept by TickReceipt.
type StopReason uint8

const (
	StopReasonUnknown StopReason = iota
	StopReasonPanelPaused
	StopReasonBudgetExhausted
	StopReasonProgramStopped
	StopReasonPredicateStopped
	StopReasonBreakpointStopped
	StopReasonOriginalFault
	StopReasonObserverFault
	StopReasonFrontendFault
)

// TickReceipt records one Advance attempt.  Steps is a machine Step-attempt
// delta, not a count of successfully executed instructions.  RawStop and
// RawError are deliberately retained for audit: in particular, RunUntil may
// return StopBudget together with a non-nil error.
// KeysPendingBefore/After bracket DOS keyboard-queue depth around the turn:
// early rejections report the unchanged snapshot, the stepping path reports
// the post-step depth, so input-consumption evidence never needs a second
// machine read.
type TickReceipt struct {
	Epoch              uint64
	Budget             InstructionBudget
	MachineStepsBefore uint64
	MachineStepsAfter  uint64
	Steps              uint64
	KeysPendingBefore  int
	KeysPendingAfter   int
	// ScanPendingBefore/After bracket the machine's timed scan-event depth
	// (docs/spec/449 §2.6): Before is taken after ScheduleHeldRepeats, After
	// after the machine ran. Early returns report the entry depth for both.
	ScanPendingBefore int
	ScanPendingAfter  int
	// KeyIRQs and KeyStalls are this turn's deltas of machine.KeyIRQs and
	// machine.KeyStalls() (KeyStalls counts blocked steps, not events).
	KeyIRQs    uint64
	KeyStalls  uint64
	RawStop    machine.Stop
	HasRawStop bool
	RawError   error
	Phase      Phase
	Reason     StopReason
}

// CapturedUpdate is the value-only input captured for one future frontend
// update. This controlled slice accepts already-classified panel events,
// pointer events, and explicitly mapped BIOS keys. Panel transitions that
// require a new mouse layout fail before commit until the owner has a private
// layout-projection contract.
type CapturedUpdate struct {
	StartedPanel host.PanelState
	Layout       host.MouseLayout
	// SourceGeneration, StartedPanel, and Layout must come from one Owner.View
	// result. Zero is never a compatibility token: accepting it would permit a
	// same-value ABA source to bypass the sealed owner check.
	SourceGeneration uint64
	PanelEvents      []host.PanelEvent
	PointerDown      *host.MouseEvent
	PointerUp        *host.MouseEvent
	FocusLost        bool
	BIOSKeys         []dos.Key
	// PointerMoves 是不按鍵的游標移動，依序在 PointerDown 之前規劃與交付
	// （docs/spec/451 §2.2）。每個元素的 Kind 都必須是 host.MouseEventMove，
	// 最多 maxPointerMoves 個；違反時整批拒絕。
	PointerMoves []host.MouseEvent
	// ScanEvents 是硬體鍵盤事件（按下／放開），依序經 IRQ1 送出（docs/spec/449）。
	ScanEvents []ScanEvent
}

// ScanEvent is one hardware keyboard event (docs/spec/449 §2.1).  Scan is the
// scan code without bit 7; Break reports a release.
type ScanEvent struct {
	Scan  uint8
	Break bool
}

// maxPointerMoves 是一批 PointerMoves 的上限（docs/spec/451 §2.2）。前端要自行合併，
// 通常每畫格只送最後的位置。
const maxPointerMoves = 64

// InputReceipt records a successfully accepted update. Epoch is an accepted
// batch ordinal, not a DOS step count. A paused panel batch still has one.
type InputReceipt struct {
	Epoch       uint64
	Phase       Phase
	HostChanged bool
	// DOSCallsCommitted counts committed MouseOutput and BIOS key calls, not
	// DOS mouse callbacks/events. In particular, a committed MoveMouse call
	// whose coordinate is unchanged intentionally produces no DOS move event.
	DOSCallsCommitted uint64
	Paused            bool
	// ScanEventsCommitted counts HeldKeyDown/HeldKeyUp calls that returned
	// true; ScanReleasesForced is the ReleaseHeldKeys result; HeldScans is
	// HeldMask() after commit (docs/spec/449 §2.6).  None is a DOS call.
	ScanEventsCommitted uint64
	ScanReleasesForced  int
	HeldScans           [2]uint64
}

// Config contains value-only host construction choices. In particular it does
// not accept an existing machine, DOS, panel, or bridge.
type Config struct {
	InitialScale  host.OutputScale
	InitialLayout host.MouseLayout
	// Observer is installed privately before the first instruction and
	// cannot be replaced (docs/spec/238).  nil keeps the bare RunUntil path.
	Observer StepObserver
	// AdLib makes the OPL2 present before the first instruction
	// (docs/spec/240 §3.1).  Off and on are independent baselines.
	AdLib bool
	// ClockPercent 是機器時脈比例 10–100（docs/spec/242）；0 視為 100。
	ClockPercent int

	// 以下對應 cmd/probe 的同名選項（docs/spec/448 §2.1），零值等於不設。

	// ProgramPath 是環境區塊裡的程式全路徑（probe 設成 `C:\` 加執行檔名）；
	// 帶目錄時 dos.New 把目前目錄設成該目錄。空字串維持 machine 預設。
	ProgramPath string
	// DOSFiles 是 SFT 檔案數（-dos-files），0 或 6..255。
	DOSFiles int
	// ExtendedMemoryKB 是延伸記憶體 KB 數（-ext-mem-kb）。
	ExtendedMemoryKB int
	// NoEMS 撤銷內建 EMM（-no-ems）。
	NoEMS bool
	// ProtectedModeProfile 開保護模式 profile（-pm-profile），需要 ExtendedMemoryKB。
	ProtectedModeProfile bool
	// SoundBlaster 讓 SB16（220h）存在（-sound-blaster），一併讓 OPL 存在。
	SoundBlaster bool
	// SBDMA8 開 SB16 的 8 位元 DMA 與 IRQ7（-sb-dma8），需要 SoundBlaster。
	SBDMA8 bool
	// Mode13Phase 讓 mode 13h 的 3DAh 回掃相位依指令數推算（-mode13-phase）。
	Mode13Phase bool
	// ExecEnvCopy 讓 EXEC 替子行程建新環境（-exec-env，docs/spec/439）。
	ExecEnvCopy bool
	// ExecReclaim 讓巢狀子行程結束時併回 arena（-exec-reclaim，docs/spec/441）。
	ExecReclaim bool
	// KbdIntercept 讓 BIOS int 09h 經 int 15h AH=4Fh 攔截（-kbd-int15，docs/spec/447）。
	KbdIntercept bool
	// SparseObserve 讓實作 SparseObserver 的觀察器只在觸發 IP、步數期限與單次喚醒的步驟收到 BeforeStep
	// （docs/spec/460）。預設假：每步呼叫（238）。觀察器不實作 SparseObserver 時沒有作用。
	SparseObserve bool
}

// applyMachineConfig 依 docs/spec/448 §2.2 的順序把 Config 的 machine 選項套到剛建好的機器，
// 與 cmd/probe 冷開機路徑的先後相同。只在 New 內、dos.New 之前呼叫。
//
// SetAdLib 與 SetSoundBlaster 只以真值呼叫：SetSoundBlaster(false) 在 SBDMA8 開啟後會 panic，
// 無條件的 SetAdLib(false) 會把 SoundBlaster 開的 OPL 關掉（448 §2.1）。
func applyMachineConfig(m *machine.Machine, cfg Config) error {
	if err := m.SetExtendedMemoryKB(cfg.ExtendedMemoryKB); err != nil {
		return fmt.Errorf("session: ExtendedMemoryKB：%w", err)
	}
	if err := m.SetEMSPresent(!cfg.NoEMS); err != nil {
		return fmt.Errorf("session: NoEMS：%w", err)
	}
	if err := m.SetProtectedModeProfile(cfg.ProtectedModeProfile); err != nil {
		return fmt.Errorf("session: ProtectedModeProfile：%w", err)
	}
	if cfg.ProgramPath != "" {
		m.ProgramPath = cfg.ProgramPath
	}
	if cfg.AdLib {
		m.SetAdLib(true)
	}
	if cfg.SoundBlaster {
		m.SetSoundBlaster(true)
	}
	if err := m.SetSBDMA8(cfg.SBDMA8); err != nil {
		return fmt.Errorf("session: SBDMA8：%w", err)
	}
	if err := m.SetDOSFiles(cfg.DOSFiles); err != nil {
		return fmt.Errorf("session: DOSFiles：%w", err)
	}
	m.Mode13Phase = cfg.Mode13Phase
	if cfg.KbdIntercept {
		if err := m.EnableKbdIntercept(); err != nil {
			return fmt.Errorf("session: KbdIntercept：%w", err)
		}
	}
	return nil
}

// SetMouseIRQ12Direct 是 docs/spec/459-mouse-irq12-v86-delivery §3.8 的診斷開關（§6.4 N1 的負對照）：保護模式剖面下
// 讓滑鼠事件照實模式剖面直接遠呼叫，不經 IRQ12。只能在 BootOriginal／BootFromState 之前呼叫；不進 Config、不進狀態檔。
// 開啟時的結果是負對照，不是正式收據。
func (o *Owner) SetMouseIRQ12Direct(on bool) error {
	if o == nil {
		return errors.New("session: Owner 不得為 nil")
	}
	if o.phase != PhaseBooting {
		return fmt.Errorf("session: SetMouseIRQ12Direct 只能在開機前呼叫（phase %d）", o.phase)
	}
	return o.machine.SetMouseIRQ12Direct(on)
}

// Status is a value-only lifecycle receipt.  It deliberately exposes neither
// an owned resource nor a mutable routing capability.
type Status struct {
	Phase      Phase
	FirstFault error
	CloseError error
}

// View is the immutable routing-input snapshot of a sealed Running owner. It
// contains neither an owned target nor a mutable bridge.  It is deliberately
// not a presentation-pixel snapshot.
type View struct {
	Phase            Phase
	Panel            host.PanelState
	Layout           host.MouseLayout
	Epoch            uint64
	SourceGeneration uint64
}

// Owner privately owns the resources which must share a session lifetime.
// All methods are for the one frontend goroutine named by spec 019.
type Owner struct {
	machine  *machine.Machine
	dos      *dos.DOS
	panel    *host.PanelController
	mouse    *host.MouseBridge
	keyboard *presentation.KeyboardBridge
	closer   resourceCloser

	phase          Phase
	epoch          uint64
	turnPending    bool
	turnPaused     bool
	generation     uint64
	committing     bool
	layout         host.MouseLayout
	layoutSet      bool
	terminalReason StopReason
	firstFault     error
	closeErr       error
	closed         bool

	observer   StepObserver
	viewToken  uint64
	viewOpen   bool
	viewMisuse bool
	// sparse 是 Config.SparseObserve；filter 是稀疏模式的觀察條件（docs/spec/460 §3.1），跨回合保留計數。
	sparse bool
	filter machine.StepFilter

	// noHeldRepeats is a test-only switch for the docs/spec/449 §3.2
	// negative control; nothing outside this package's tests sets it.
	noHeldRepeats bool
	// noKbdRewrite is a test-only switch for the docs/spec/450 §3.4
	// negative control (BootFromState skips step 6).
	noKbdRewrite bool

	// clockPercent 是 Config.ClockPercent 的原值（docs/spec/450 §2.4）。
	// m.ClockPercent() 對 0 與 100 都回 100，不能拿來判斷是否明示給過。
	clockPercent int
}

// New creates a new sealed, synthetic-ready owner.  It neither loads an
// original executable nor accepts one; later boot work must remain within this
// owner rather than exposing these resources.
func New(cfg Config) (*Owner, error) {
	if cfg.InitialLayout.Epoch == 0 && cfg.InitialLayout != (host.MouseLayout{}) {
		return nil, errors.New("session: InitialLayout 非零時必須有 layout epoch")
	}
	// 448 §2.3：在呼叫任何 setter 之前檢查。
	if cfg.SBDMA8 && !cfg.SoundBlaster {
		return nil, errors.New("session: SBDMA8 需要 SoundBlaster（docs/spec/444 §2.1）")
	}
	panel, err := host.NewPanelController(cfg.InitialScale)
	if err != nil {
		return nil, err
	}
	m := machine.New()
	// 448 §2.2：machine 的選項要在 dos.New 之前套用（ProgramPath 決定目前目錄）。
	// 這時 DOS 還沒建立，錯誤直接回傳。
	if err := applyMachineConfig(m, cfg); err != nil {
		return nil, err
	}
	d := dos.New(m, "")
	// DOS.Install must follow LoadEXE.  This narrow owner has no original
	// boot yet, so Booting deliberately does not install DOS or permit steps.
	// A future private boot must LoadEXE, then Install, then enter Running.
	keepDOS := false
	defer func() {
		if !keepDOS {
			d.Close()
		}
	}()
	d.ExecEnvCopy = cfg.ExecEnvCopy
	d.ExecReclaim = cfg.ExecReclaim
	mouse, err := host.NewMouseBridge(d)
	if err != nil {
		return nil, err
	}
	if cfg.InitialLayout.Epoch != 0 {
		state, err := panel.Snapshot()
		if err != nil {
			return nil, err
		}
		if cfg.InitialLayout.Scale != state.Scales.ActiveScale || cfg.InitialLayout.PanelOpen != state.Open {
			return nil, errors.New("session: InitialLayout 與初始 panel 狀態不一致")
		}
		if _, changed, err := host.ProjectPresentationLayout(cfg.InitialLayout, state); err != nil || changed {
			return nil, errors.New("session: InitialLayout 與正式呈現幾何不一致")
		}
		if err := mouse.ApplyLayout(cfg.InitialLayout); err != nil {
			return nil, err
		}
	}
	keyboard, err := presentation.NewKeyboardBridgeWithBIOS(panel, m, d)
	if err != nil {
		return nil, err
	}
	o, err := newOwner(m, d, panel, mouse, keyboard, dosCloser{dos: d})
	if err != nil {
		return nil, err
	}
	if cfg.InitialLayout.Epoch != 0 {
		o.layout, o.layoutSet = cfg.InitialLayout, true
	}
	o.observer = cfg.Observer
	o.sparse = cfg.SparseObserve
	o.clockPercent = cfg.ClockPercent
	// 448 §2.4：SoundBlaster 也會讓 OPL 存在，沒有觀測器時 OPL 寫入會一直累積，
	// 所以只要 OPL 存在就掛空收集器（與 runObserved 的 defer 判準相同）。
	if m.AdLibPresent() {
		o.muteAudio()
	}
	if cfg.ClockPercent != 0 {
		if err := m.SetClockPercent(cfg.ClockPercent); err != nil {
			return nil, err
		}
	}
	keepDOS = true
	return o, nil
}

// ReportDrawFault synchronously latches the first inspectable Draw fault.
// It is intentionally independent of any later Update call.
func (o *Owner) ReportDrawFault(err error) error {
	if o == nil {
		return fmt.Errorf("session: Owner 不得為 nil")
	}
	if err == nil {
		err = errors.New("session: ReportDrawFault 需要非 nil error")
	}
	if o.phase != PhaseFailed && o.phase != PhaseClosed {
		o.terminalReason = StopReasonFrontendFault
	}
	return o.fail(err)
}

// Deliver accepts one value-only update through a pure prepare and exclusive
// commit.  It deliberately does not accept mutable bridges or a machine from
// its caller.  Pointer/focus transport uses the owner-private captured layout;
// prevalidated panel transitions then project and install their next private
// layout after the batch's pointer/focus portion has committed.
func (o *Owner) Deliver(update CapturedUpdate) (InputReceipt, error) {
	if o == nil {
		return InputReceipt{Phase: PhaseFailed}, errors.New("session: Owner 不得為 nil")
	}
	receipt := InputReceipt{Epoch: o.epoch, Phase: o.phase}
	if o.committing {
		return o.frontendFaultInputReceipt(receipt, errors.New("session: Commit 期間不得重入 Deliver"))
	}
	if o.phase != PhaseRunning {
		if o.phase == PhaseStopped || o.phase == PhaseClosed {
			return receipt, fmt.Errorf("session: phase %d 不可接納新回合", o.phase)
		}
		return o.frontendFaultInputReceipt(receipt, errors.New("session: 非 Running owner 不可 Deliver"))
	}
	if o.turnPending {
		return o.frontendFaultInputReceipt(receipt, errors.New("session: 前一回合尚未 Advance"))
	}
	plan, err := o.prepare(update)
	if err != nil {
		return o.frontendFaultInputReceipt(receipt, err)
	}
	return o.commit(plan)
}

type deliveredPlan struct {
	owner       *Owner
	generation  uint64
	base, final host.PanelState
	baseMouse   host.MouseBridgeSnapshot
	finalMouse  host.MouseBridgeSnapshot
	layout      host.MouseLayout
	panelEvents []preparedPanelEvent
	mouseEvents []preparedMouseEvent
	biosKeys    []dos.Key
	scanEvents  []ScanEvent
	focusLost   bool
	hostChanged bool
	paused      bool
}

type preparedPanelEvent struct {
	event         host.PanelEvent
	final         host.PanelState
	layout        host.MouseLayout
	layoutChanged bool
}

type preparedMouseEvent struct {
	event       host.MouseEvent
	route       host.MouseRoute
	next        host.MouseBridgeSnapshot
	actionCount uint8
}

// prepare performs no Route, Handle, ApplyLayout, DeliverBIOSKey, DOS write,
// or machine step.  It only consumes immutable snapshots and pure planners.
func (o *Owner) prepare(update CapturedUpdate) (deliveredPlan, error) {
	if o.epoch == ^uint64(0) || o.generation == ^uint64(0) {
		return deliveredPlan{}, errors.New("session: epoch 或 generation 溢位")
	}
	if update.SourceGeneration == 0 || update.SourceGeneration != o.generation {
		return deliveredPlan{}, errors.New("session: CapturedUpdate 的 SourceGeneration 與 owner 不一致")
	}
	// Every batch's three capture values must be one View result, even when it
	// contains no pointer. FocusLost retains MouseBridge's release exception
	// for its event routing, but cannot use a separately stale batch layout.
	if update.Layout != o.layout {
		return deliveredPlan{}, errors.New("session: CapturedUpdate 的 layout 與 owner 不一致")
	}
	if err := o.keyboard.ValidateBIOSForPanel(o.panel); err != nil {
		return deliveredPlan{}, err
	}
	base, err := o.panel.Snapshot()
	if err != nil {
		return deliveredPlan{}, err
	}
	if !o.layoutSet {
		return deliveredPlan{}, errors.New("session: Running owner 缺少私有 layout")
	}
	canonical, changed, err := host.ProjectPresentationLayout(o.layout, base)
	if err != nil || changed || canonical != o.layout {
		if err == nil {
			err = errors.New("session: owner 私有 layout 未按目前 panel 正規化")
		}
		return deliveredPlan{}, err
	}
	baseMouse := o.mouse.Snapshot()
	if !baseMouse.HasCurrent || baseMouse.Current != o.layout {
		return deliveredPlan{}, errors.New("session: owner mouse snapshot 與私有 layout 不一致")
	}
	if update.StartedPanel != base {
		return deliveredPlan{}, errors.New("session: CapturedUpdate 的起點 panel 與 owner 不一致")
	}
	plan := deliveredPlan{owner: o, generation: o.generation, base: base, final: base, baseMouse: baseMouse, layout: o.layout}
	plan.finalMouse = plan.baseMouse
	hasPointer := update.PointerDown != nil || update.PointerUp != nil || update.FocusLost || len(update.PointerMoves) != 0
	if hasPointer {
		// MouseBridge deliberately permits FocusLost to release an existing
		// press even after its internal press epoch has changed. The batch layout
		// itself was already required above to be the current View layout.
		if !plan.baseMouse.HasCurrent || plan.baseMouse.Current != o.layout {
			return deliveredPlan{}, errors.New("session: owner mouse snapshot 與私有 layout 不一致")
		}
		// 451 §2.2：移動先於 PointerDown 規劃；commit 依 plan.mouseEvents 的順序交付，
		// 所以「移動到 A、在 B 按下」會先送 MoveMouse(A)。Target 未分類由 planMouseEvent 拒絕。
		if len(update.PointerMoves) > maxPointerMoves {
			return deliveredPlan{}, fmt.Errorf("session: PointerMoves %d 個，超過上限 %d（docs/spec/451 §2.2）", len(update.PointerMoves), maxPointerMoves)
		}
		for _, move := range update.PointerMoves {
			if move.Kind != host.MouseEventMove {
				return deliveredPlan{}, errors.New("session: PointerMoves 必須是 MouseEventMove")
			}
			if err := planMouseEvent(&plan, update.Layout, move); err != nil {
				return deliveredPlan{}, err
			}
		}
		if update.PointerDown != nil {
			if update.PointerDown.Kind != host.MouseEventDown {
				return deliveredPlan{}, errors.New("session: PointerDown 必須是 MouseEventDown")
			}
			if err := planMouseEvent(&plan, update.Layout, *update.PointerDown); err != nil {
				return deliveredPlan{}, err
			}
		}
		if update.PointerUp != nil {
			if update.PointerUp.Kind != host.MouseEventUp {
				return deliveredPlan{}, errors.New("session: PointerUp 必須是 MouseEventUp")
			}
			if err := planMouseEvent(&plan, update.Layout, *update.PointerUp); err != nil {
				return deliveredPlan{}, err
			}
		}
		if update.FocusLost {
			if err := planMouseEvent(&plan, update.Layout, host.MouseEvent{Kind: host.MouseEventFocusLost}); err != nil {
				return deliveredPlan{}, err
			}
		}
	}
	if update.PointerDown != nil && update.PointerDown.Target != host.MouseTargetHost && len(update.PanelEvents) != 0 {
		return deliveredPlan{}, errors.New("session: 同批 panel route 與非 host Down 分類矛盾")
	}
	projectedLayout := plan.layout
	for _, event := range update.PanelEvents {
		next, route, err := host.PlanPanelRoute(plan.final, event)
		if err != nil {
			return deliveredPlan{}, err
		}
		if route.ForwardToDOS {
			return deliveredPlan{}, errors.New("session: PanelEventKeyboard 必須以 BIOSKeys 明示交付")
		}
		layout, layoutChanged := projectedLayout, false
		if o.layoutSet {
			layout, layoutChanged, err = host.ProjectPresentationLayout(projectedLayout, next)
			if err != nil {
				return deliveredPlan{}, err
			}
		}
		plan.final = next
		plan.panelEvents = append(plan.panelEvents, preparedPanelEvent{
			event: event, final: next, layout: layout, layoutChanged: layoutChanged,
		})
		if layoutChanged {
			projectedLayout = layout
		}
		plan.hostChanged = plan.hostChanged || route.ConsumedByHost
	}
	if o.layoutSet && projectedLayout != plan.layout {
		// A sealed owner always applies this value in commit after the matching
		// prevalidated panel route. It is intentionally private: callers only
		// bring the captured layout for their pointer event.
		plan.finalMouse.Current = projectedLayout
		plan.finalMouse.HasCurrent = true
	}
	// 449 §2.1：拒絕條件在 paused 批次也照查。
	if len(update.ScanEvents) != 0 && len(update.BIOSKeys) != 0 {
		return deliveredPlan{}, errors.New("session: 同一批不得同時有 ScanEvents 與 BIOSKeys（docs/spec/449 §2.1）")
	}
	for _, ev := range update.ScanEvents {
		if ev.Scan == 0 || ev.Scan >= 0x80 {
			return deliveredPlan{}, fmt.Errorf("session: ScanEvent 掃描碼 %02Xh 不在 01h..7Fh（docs/spec/449 §2.1）", ev.Scan)
		}
	}
	plan.paused = plan.base.Open || plan.final.Open || plan.hostChanged
	if !plan.paused {
		plan.biosKeys = append(plan.biosKeys, update.BIOSKeys...)
	}
	plan.scanEvents = append([]ScanEvent(nil), update.ScanEvents...)
	plan.focusLost = update.FocusLost
	return plan, nil
}

func planMouseEvent(plan *deliveredPlan, layout host.MouseLayout, event host.MouseEvent) error {
	if event.Target != host.MouseTargetCanvas && event.Target != host.MouseTargetHost && event.Target != host.MouseTargetOutside && event.Kind != host.MouseEventFocusLost {
		return errors.New("session: pointer target 未分類")
	}
	next, err := host.PlanMouseRoute(plan.finalMouse, layout, event)
	if err != nil {
		return err
	}
	switch next.Route.Reason {
	case "stale-layout-epoch-rejected", "unknown-event-rejected", "non-left-rejected", "nil-bridge-rejected":
		return fmt.Errorf("session: pointer route 在 commit 前拒絕：%s", next.Route.Reason)
	}
	plan.mouseEvents = append(plan.mouseEvents, preparedMouseEvent{event: event, route: next.Route, next: next.Next, actionCount: next.ActionCount})
	plan.finalMouse = next.Next
	return nil
}

// commit rechecks every source before its first output. The owner never leaks
// panel, bridge, DOS, or machine mutators; after this check it executes only
// prevalidated mouse actions and explicit BIOS key pushes.
func (o *Owner) commit(plan deliveredPlan) (InputReceipt, error) {
	receipt := InputReceipt{Epoch: o.epoch, Phase: o.phase}
	if plan.owner != o || plan.generation != o.generation || o.committing {
		return o.frontendFaultInputReceipt(receipt, errors.New("session: stale 或重入的 Deliver plan"))
	}
	current, err := o.panel.Snapshot()
	if err != nil || current != plan.base || o.mouse.Snapshot() != plan.baseMouse || (o.layoutSet && o.layout != plan.layout) {
		if err == nil {
			err = errors.New("session: Deliver plan 的 panel source 已漂移")
		}
		return o.frontendFaultInputReceipt(receipt, err)
	}
	if err := o.keyboard.ValidateBIOSForPanel(o.panel); err != nil {
		return o.frontendFaultInputReceipt(receipt, err)
	}
	o.committing = true
	defer func() { o.committing = false }()
	for _, prepared := range plan.mouseEvents {
		route := o.mouse.Handle(plan.layout, prepared.event)
		if route != prepared.route || o.mouse.Snapshot() != prepared.next {
			panic("session: prevalidated mouse commit 發生不可能的分歧")
		}
	}
	for _, prepared := range plan.panelEvents {
		state, route, err := o.panel.Route(prepared.event)
		if err != nil || route.ForwardToDOS || state != prepared.final {
			if err == nil {
				err = errors.New("session: 預檢的 panel commit 發生分歧")
			}
			return o.frontendFaultInputReceipt(receipt, err)
		}
		if prepared.layoutChanged {
			if err := o.mouse.ApplyLayout(prepared.layout); err != nil {
				panic("session: 預檢的 layout commit 發生不可能的分歧: " + err.Error())
			}
			o.layout = prepared.layout
			o.layoutSet = true
		}
	}
	final, err := o.panel.Snapshot()
	if err != nil || final != plan.final {
		if err == nil {
			err = errors.New("session: panel commit 結果與純 plan 不一致")
		}
		return o.frontendFaultInputReceipt(receipt, err)
	}
	if o.mouse.Snapshot() != plan.finalMouse || (o.layoutSet && o.layout != plan.finalMouse.Current) {
		panic("session: 預檢的 mouse/layout commit 發生不可能的分歧")
	}
	for _, key := range plan.biosKeys {
		o.dos.PushKey(key)
	}
	scanCommitted, scanForced := o.commitScanEvents(plan)
	o.epoch++
	o.generation++
	o.turnPending = true
	o.turnPaused = plan.paused
	keyCalls := uint64(len(plan.biosKeys))
	return InputReceipt{Epoch: o.epoch, Phase: o.phase, HostChanged: plan.hostChanged,
		DOSCallsCommitted: keyCalls + mouseActionCount(plan.mouseEvents), Paused: plan.paused,
		ScanEventsCommitted: scanCommitted, ScanReleasesForced: scanForced, HeldScans: o.machine.HeldMask()}, nil
}

// commitScanEvents applies docs/spec/449 §2.4 after mouse, panel, and BIOS
// keys.  It touches only the machine's timed keys and held state.
func (o *Owner) commitScanEvents(plan deliveredPlan) (committed uint64, forced int) {
	m := o.machine
	switch {
	case plan.paused:
		// 規則 3：全部不 commit，強制放開。
	case plan.focusLost:
		// 規則 2：只 commit 放開事件。
		for _, ev := range plan.scanEvents {
			if ev.Break && m.HeldKeyUp(ev.Scan) {
				committed++
			}
		}
	default:
		// 規則 1。
		for _, ev := range plan.scanEvents {
			var ok bool
			if ev.Break {
				ok = m.HeldKeyUp(ev.Scan)
			} else {
				ok = m.HeldKeyDown(ev.Scan)
			}
			if ok {
				committed++
			}
		}
		return committed, 0
	}
	return committed, m.ReleaseHeldKeys()
}

func mouseActionCount(events []preparedMouseEvent) uint64 {
	var count uint64
	for _, prepared := range events {
		count += uint64(prepared.actionCount)
	}
	return count
}

// Advance consumes the one privately accepted running turn and returns an
// auditable machine receipt.  This is deliberately the synthetic/no-observer
// slice: bare Machine.RunUntil does not dispatch oracle.OnCall hooks.  Public
// input delivery and an observer-aware runner are intentionally not invented
// here; a later sealed Deliver implementation must perform the READY
// captured-update/prepare/commit contract before calling acceptTurn.
func (o *Owner) Advance(budget InstructionBudget) (TickReceipt, error) {
	if o == nil {
		return TickReceipt{Reason: StopReasonFrontendFault}, errors.New("session: Owner 不得為 nil")
	}
	receipt := TickReceipt{Epoch: o.epoch, Budget: budget, Phase: o.phase, Reason: o.terminalReason}
	receipt.KeysPendingBefore = o.dos.KeysPending()
	receipt.KeysPendingAfter = receipt.KeysPendingBefore
	receipt.ScanPendingBefore = o.machine.TimedKeysPending()
	receipt.ScanPendingAfter = receipt.ScanPendingBefore
	switch o.phase {
	case PhaseStopped:
		receipt.Reason = StopReasonProgramStopped
		return receipt, nil
	case PhaseFailed:
		return receipt, errors.Join(o.firstFault, o.closeErr)
	case PhaseClosed:
		receipt.Reason = StopReasonFrontendFault
		return receipt, errors.Join(errors.New("session: 已關閉的 owner 不可 Advance"), o.closeErr)
	case PhaseBooting:
		return o.frontendFaultReceipt(receipt, errors.New("session: Booting owner 尚未進入 Running"))
	case PhaseRunning:
		// Continue below.
	default:
		return o.frontendFaultReceipt(receipt, errors.New("session: 未知 owner phase"))
	}
	if !o.turnPending {
		return o.frontendFaultReceipt(receipt, errors.New("session: Advance 前必須先由 Deliver 接納一個回合"))
	}
	if !o.turnPaused && budget == 0 {
		return o.frontendFaultReceipt(receipt, errors.New("session: InstructionBudget 必須為正數"))
	}
	if o.generation == ^uint64(0) {
		return o.frontendFaultReceipt(receipt, errors.New("session: Advance 前 SourceGeneration 溢位"))
	}
	o.turnPending = false
	if o.turnPaused {
		o.turnPaused = false
		o.generation++
		receipt.Phase = o.phase
		receipt.Reason = StopReasonPanelPaused
		return receipt, nil
	}
	if o.dos.Exited {
		o.phase = PhaseStopped
		o.terminalReason = StopReasonProgramStopped
		o.generation++
		receipt.Phase = o.phase
		receipt.Reason = o.terminalReason
		return receipt, nil
	}

	// 449 §2.5：跑機器之前排好這一回合的 typematic 重複碼。
	if !o.noHeldRepeats {
		o.machine.ScheduleHeldRepeats(uint64(budget))
	}
	receipt.ScanPendingBefore = o.machine.TimedKeysPending()
	keyIRQs, keyStalls := o.machine.KeyIRQs, o.machine.KeyStalls()
	receipt.MachineStepsBefore = o.machine.Steps
	var rawStop machine.Stop
	var rawErr, observerErr error
	if o.observer != nil {
		rawStop, rawErr, observerErr = o.runObserved(uint64(budget))
	} else {
		rawStop, rawErr = o.machine.RunUntil(nil, uint64(budget))
	}
	receipt.HasRawStop = true
	receipt.RawStop = rawStop
	receipt.RawError = rawErr
	receipt.MachineStepsAfter = o.machine.Steps
	receipt.KeysPendingAfter = o.dos.KeysPending()
	receipt.ScanPendingAfter = o.machine.TimedKeysPending()
	receipt.KeyIRQs = o.machine.KeyIRQs - keyIRQs
	receipt.KeyStalls = o.machine.KeyStalls() - keyStalls
	if receipt.MachineStepsAfter < receipt.MachineStepsBefore {
		return o.originalFaultReceipt(receipt, errors.New("session: Machine.Steps 不得倒退"))
	}
	receipt.Steps = receipt.MachineStepsAfter - receipt.MachineStepsBefore
	if receipt.Steps > uint64(budget) {
		return o.originalFaultReceipt(receipt, errors.New("session: Machine.Steps 差分超過 InstructionBudget"))
	}
	if observerErr != nil {
		return o.observerFaultReceipt(receipt, observerErr)
	}
	if rawErr != nil {
		return o.originalFaultReceipt(receipt, rawErr)
	}
	if o.dos.Exited {
		o.phase = PhaseStopped
		o.terminalReason = StopReasonProgramStopped
		o.generation++
		receipt.Phase = o.phase
		receipt.Reason = o.terminalReason
		return receipt, nil
	}
	switch rawStop {
	case machine.StopBudget:
		if receipt.Steps == uint64(budget) {
			o.generation++
			receipt.Phase = o.phase
			receipt.Reason = StopReasonBudgetExhausted
			return receipt, nil
		}
		return o.originalFaultReceipt(receipt, errors.New("session: StopBudget 未耗盡 InstructionBudget"))
	case machine.StopPredicate:
		// No observer/predicate contract is installed in this slice.  A future
		// owner may expose PredicateStopped only after that contract is READY.
		return o.originalFaultReceipt(receipt, errors.New("session: 未宣告的 machine predicate stop"))
	case machine.StopBreakpoint:
		return o.originalFaultReceipt(receipt, errors.New("session: 未宣告的 machine breakpoint stop"))
	default:
		return o.originalFaultReceipt(receipt, fmt.Errorf("session: 未知 machine stop %v", rawStop))
	}
}

// Close performs the owned resource close at most once.  A close after a
// fault preserves that first fault; a normal close moves directly to Closed.
func (o *Owner) Close() error {
	if o == nil {
		return fmt.Errorf("session: Owner 不得為 nil")
	}
	if !o.closed {
		o.closed = true
		if o.closer == nil {
			o.closeErr = errors.New("session: 缺少私有資源關閉器")
		} else {
			o.closeErr = o.closer.Close()
		}
		if o.phase != PhaseFailed {
			o.phase = PhaseClosed
		}
	}
	return o.closeErr
}

// Status returns immutable lifecycle state; it cannot be used to reach an
// owned machine, DOS, panel, or bridge.
func (o *Owner) Status() Status {
	if o == nil {
		return Status{}
	}
	return Status{Phase: o.phase, FirstFault: o.firstFault, CloseError: o.closeErr}
}

// View returns a value-only input-capture source. It is valid only while the
// owner is Running and its private layout is canonical for the current panel.
// A snapshot/layout failure is a synchronous owner fault; the returned View is
// always zero on error, so no stale partial source can be reused.
func (o *Owner) View() (View, error) {
	if o == nil {
		return View{}, errors.New("session: Owner 不得為 nil")
	}
	if o.phase != PhaseRunning {
		return View{}, fmt.Errorf("session: phase %d 不可取得 View，請讀取 Status", o.phase)
	}
	if !o.layoutSet {
		return View{}, o.fail(errors.New("session: Running owner 缺少私有 layout"))
	}
	panel, err := o.panel.Snapshot()
	if err != nil {
		return View{}, o.fail(fmt.Errorf("session: 讀取 owner panel snapshot: %w", err))
	}
	canonical, changed, err := host.ProjectPresentationLayout(o.layout, panel)
	if err != nil || changed || canonical != o.layout {
		if err == nil {
			err = errors.New("session: owner 私有 layout 未按目前 panel 正規化")
		}
		return View{}, o.fail(err)
	}
	return View{Phase: o.phase, Panel: panel, Layout: o.layout, Epoch: o.epoch, SourceGeneration: o.generation}, nil
}

// startLoadedMachine is the private half of a future boot path.  Its caller
// must already have loaded the executable into o.machine; keeping it private
// prevents a frontend from supplying or retaining a mutable machine target.
func (o *Owner) startLoadedMachine() error {
	if o == nil {
		return errors.New("session: Owner 不得為 nil")
	}
	if o.phase != PhaseBooting {
		return o.fail(errors.New("session: 只有 Booting owner 可啟動"))
	}
	if o.generation == ^uint64(0) {
		return o.fail(errors.New("session: 啟動前 SourceGeneration 溢位"))
	}
	o.dos.Install()
	o.phase = PhaseRunning
	o.generation++
	return nil
}

// acceptTurn is the minimal owner-only epoch gate.  It deliberately contains
// no made-up input routing: the future Deliver method must call it only after
// the complete captured-update prepare/commit operation succeeds.
func (o *Owner) acceptTurn() (uint64, error) {
	return o.acceptTurnWithPause(false)
}

func (o *Owner) acceptTurnWithPause(paused bool) (uint64, error) {
	if o == nil {
		return 0, errors.New("session: Owner 不得為 nil")
	}
	if o.phase != PhaseRunning {
		if o.phase == PhaseStopped || o.phase == PhaseClosed {
			return o.epoch, fmt.Errorf("session: phase %d 不可接納新回合", o.phase)
		}
		return o.epoch, o.fail(errors.New("session: 非 Running owner 不可接納新回合"))
	}
	if o.turnPending {
		return o.epoch, o.fail(errors.New("session: 前一回合尚未 Advance"))
	}
	if o.epoch == ^uint64(0) || o.generation == ^uint64(0) {
		return o.epoch, o.fail(errors.New("session: epoch 或 SourceGeneration 溢位"))
	}
	o.epoch++
	o.turnPending = true
	o.turnPaused = paused
	o.generation++
	return o.epoch, nil
}

func (o *Owner) frontendFaultInputReceipt(receipt InputReceipt, cause error) (InputReceipt, error) {
	o.terminalReason = StopReasonFrontendFault
	err := o.fail(cause)
	receipt.Phase = o.phase
	return receipt, err
}

func (o *Owner) frontendFaultReceipt(receipt TickReceipt, cause error) (TickReceipt, error) {
	o.terminalReason = StopReasonFrontendFault
	err := o.fail(cause)
	receipt.Phase = o.phase
	receipt.Reason = o.terminalReason
	return receipt, err
}

func (o *Owner) observerFaultReceipt(receipt TickReceipt, cause error) (TickReceipt, error) {
	o.terminalReason = StopReasonObserverFault
	err := o.fail(cause)
	receipt.Phase = o.phase
	receipt.Reason = o.terminalReason
	return receipt, err
}

func (o *Owner) originalFaultReceipt(receipt TickReceipt, cause error) (TickReceipt, error) {
	o.terminalReason = StopReasonOriginalFault
	err := o.fail(cause)
	receipt.Phase = o.phase
	receipt.Reason = o.terminalReason
	return receipt, err
}

func (o *Owner) fail(err error) error {
	if o.phase == PhaseFailed {
		return errors.Join(o.firstFault, o.closeErr)
	}
	if o.phase == PhaseClosed {
		return errors.Join(err, o.closeErr)
	}
	o.firstFault = err
	o.phase = PhaseFailed
	return errors.Join(err, o.Close())
}

type resourceCloser interface {
	Close() error
}

type dosCloser struct{ dos *dos.DOS }

func (c dosCloser) Close() error {
	if c.dos == nil {
		return errors.New("session: 私有 DOS 不得為 nil")
	}
	c.dos.Close()
	return nil
}

func newOwner(m *machine.Machine, d *dos.DOS, panel *host.PanelController, mouse *host.MouseBridge, keyboard *presentation.KeyboardBridge, closer resourceCloser) (*Owner, error) {
	if m == nil || d == nil || panel == nil || mouse == nil || keyboard == nil || closer == nil {
		return nil, errors.New("session: 封閉 owner 的私有資源不得為 nil")
	}
	return &Owner{machine: m, dos: d, panel: panel, mouse: mouse, keyboard: keyboard, closer: closer, phase: PhaseBooting}, nil
}
