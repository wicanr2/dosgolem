package session

import (
	"testing"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/dos"
)

// docs/spec/451 §3.1 第 3 到 9 項：PointerMoves 的交付與 DOSMouse。
//
// 除特別註明外 owner 都用 sessionLayout(1, OutputScale2, false)：DOS 座標 (x, y)
// 對 host (2x, 36+2y)，初始游標 (0, 0)，沒設範圍時 DOS 層不夾限。

func dosMove(x, y int) host.MouseEvent {
	return host.MouseEvent{Kind: host.MouseEventMove, X: 2 * x, Y: 36 + 2*y, Target: host.MouseTargetCanvas}
}

func dosPointer(kind host.MouseEventKind, x, y int) *host.MouseEvent {
	e := dosMove(x, y)
	e.Kind = kind
	return &e
}

func pointerUpdate(t *testing.T, o *Owner, moves []host.MouseEvent) CapturedUpdate {
	t.Helper()
	view := ownerView(t, o)
	return CapturedUpdate{StartedPanel: view.Panel, Layout: view.Layout, SourceGeneration: view.SourceGeneration, PointerMoves: moves}
}

func setMouseHandler(o *Owner) {
	o.dos.Mouse.Handler.Set = true
	o.dos.Mouse.Handler.Mask = dos.EvMove | dos.EvLeftDown | dos.EvLeftUp
}

// 第 3 項：整批拒絕時 owner 進入 PhaseFailed，epoch、generation、mouse snapshot、DOS 座標都不變。
func TestOwnerDeliverPointerMovesRejectsWholeBatch(t *testing.T) {
	many := func(n int) []host.MouseEvent {
		moves := make([]host.MouseEvent, n)
		for i := range moves {
			moves[i] = dosMove(10, 20)
		}
		return moves
	}
	down := dosMove(10, 20)
	down.Kind = host.MouseEventDown
	zeroTarget := dosMove(10, 20)
	zeroTarget.Target = 0
	for _, tc := range []struct {
		name  string
		moves []host.MouseEvent
	}{
		{"kind-down", []host.MouseEvent{down}},
		{"zero-target", []host.MouseEvent{zeroTarget}},
		{"65-moves", many(65)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := startSyntheticOwnerWithLayout(t, []byte{0x90, 0x90}, sessionLayout(1, host.OutputScale2, false))
			update := pointerUpdate(t, o, tc.moves)
			epoch, generation, snapshot := o.epoch, o.generation, o.mouse.Snapshot()
			x, y := o.dos.Mouse.X, o.dos.Mouse.Y
			receipt, err := o.Deliver(update)
			if err == nil || receipt.Phase != PhaseFailed || o.Status().Phase != PhaseFailed {
				t.Fatalf("Deliver = (%+v, %v)，要整批拒絕並進入 PhaseFailed", receipt, err)
			}
			if o.epoch != epoch || o.generation != generation || o.mouse.Snapshot() != snapshot ||
				o.dos.Mouse.X != x || o.dos.Mouse.Y != y {
				t.Fatalf("拒絕後狀態變了：epoch %d→%d generation %d→%d snapshot %+v→%+v DOS (%d,%d)→(%d,%d)",
					epoch, o.epoch, generation, o.generation, snapshot, o.mouse.Snapshot(), x, y, o.dos.Mouse.X, o.dos.Mouse.Y)
			}
		})
	}
	t.Run("64-moves-accepted", func(t *testing.T) {
		o := startSyntheticOwnerWithLayout(t, []byte{0x90, 0x90}, sessionLayout(1, host.OutputScale2, false))
		receipt, err := o.Deliver(pointerUpdate(t, o, many(64)))
		if err != nil || receipt.Phase != PhaseRunning || receipt.DOSCallsCommitted != 64 || o.dos.Mouse.X != 10 || o.dos.Mouse.Y != 20 {
			t.Fatalf("64 個移動 = (%+v, %v)，mouse=%+v", receipt, err, o.dos.Mouse)
		}
	})
}

// 第 4 項：同批「移動到 A、在 B 按下、在 B 放開」的 DOS 事件順序與呼叫數。
func TestOwnerDeliverPointerMovesBeforeDownAndUp(t *testing.T) {
	o := startSyntheticOwnerWithLayout(t, []byte{0x90, 0x90}, sessionLayout(1, host.OutputScale2, false))
	setMouseHandler(o)
	const ax, ay, bx, by = 10, 20, 30, 40
	update := pointerUpdate(t, o, []host.MouseEvent{dosMove(ax, ay)})
	update.PointerDown = dosPointer(host.MouseEventDown, bx, by)
	update.PointerUp = dosPointer(host.MouseEventUp, bx, by)
	receipt, err := o.Deliver(update)
	if err != nil || receipt.DOSCallsCommitted != 5 {
		t.Fatalf("Deliver = (%+v, %v)，要 DOSCallsCommitted 5", receipt, err)
	}
	want := []dos.Poll{
		{X: ax, Y: ay, Buttons: dos.EvMove},
		{X: bx, Y: by, Buttons: dos.EvMove},
		{X: bx, Y: by, Buttons: dos.EvLeftDown},
		{X: bx, Y: by, Buttons: dos.EvLeftUp},
	}
	events := o.dos.Mouse.Events
	if len(events) != len(want) {
		t.Fatalf("Mouse.Events=%+v，要 %d 筆", events, len(want))
	}
	for i, w := range want {
		if events[i].X != w.X || events[i].Y != w.Y || events[i].Buttons != w.Buttons {
			t.Fatalf("Mouse.Events[%d]=%+v，要 X %d Y %d 遮罩 %#x；全部 %+v", i, events[i], w.X, w.Y, w.Buttons, events)
		}
	}
}

// 第 5 項：移到目前座標仍算一次 MoveMouse 呼叫，但 DOS 層不排事件。
func TestOwnerDeliverPointerMoveCountsUnchangedPosition(t *testing.T) {
	t.Run("unchanged", func(t *testing.T) {
		o := startSyntheticOwnerWithLayout(t, []byte{0x90, 0x90}, sessionLayout(1, host.OutputScale2, false))
		setMouseHandler(o)
		before := len(o.dos.Mouse.Events)
		receipt, err := o.Deliver(pointerUpdate(t, o, []host.MouseEvent{dosMove(int(o.dos.Mouse.X), int(o.dos.Mouse.Y))}))
		if err != nil || receipt.DOSCallsCommitted != 1 || len(o.dos.Mouse.Events) != before {
			t.Fatalf("Deliver = (%+v, %v)，events %d→%d；要 1 次呼叫、不增事件", receipt, err, before, len(o.dos.Mouse.Events))
		}
	})
	t.Run("changed", func(t *testing.T) {
		o := startSyntheticOwnerWithLayout(t, []byte{0x90, 0x90}, sessionLayout(1, host.OutputScale2, false))
		setMouseHandler(o)
		before := len(o.dos.Mouse.Events)
		receipt, err := o.Deliver(pointerUpdate(t, o, []host.MouseEvent{dosMove(5, 7)}))
		if err != nil || receipt.DOSCallsCommitted != 1 || len(o.dos.Mouse.Events) != before+1 {
			t.Fatalf("Deliver = (%+v, %v)，events %d→%d；要 1 次呼叫、增 1 筆事件", receipt, err, before, len(o.dos.Mouse.Events))
		}
	})
}

// 第 6 項：面板開著時只含移動的批次不送 DOS。
func TestOwnerDeliverPointerMoveWithPanelOpenIsConsumed(t *testing.T) {
	o := startSyntheticOwnerWithLayout(t, []byte{0x90, 0x90}, sessionLayout(1, host.OutputScale2, false))
	openOwnerPanelForTest(t, o)
	x, y := o.dos.Mouse.X, o.dos.Mouse.Y
	move := host.MouseEvent{Kind: host.MouseEventMove, X: 40, Y: o.layout.ChromeHeight + 40, Target: host.MouseTargetCanvas}
	receipt, err := o.Deliver(pointerUpdate(t, o, []host.MouseEvent{move}))
	if err != nil || receipt.DOSCallsCommitted != 0 || receipt.HostChanged || !receipt.Paused ||
		o.dos.Mouse.X != x || o.dos.Mouse.Y != y {
		t.Fatalf("Deliver = (%+v, %v)，mouse=%+v；要不送 DOS、paused", receipt, err, o.dos.Mouse)
	}
	consumePausedTurn(t, o, 1)
}

// 第 7 項：批次開頭面板關著、同批才打開時，移動照樣寫進 DOS，批次 paused。
func TestOwnerDeliverPointerMoveCommitsBeforeSameBatchPanelOpen(t *testing.T) {
	o := startSyntheticOwnerWithLayout(t, []byte{0x90, 0x90}, sessionLayout(1, host.OutputScale2, false))
	update := pointerUpdate(t, o, []host.MouseEvent{dosMove(12, 34)})
	update.PanelEvents = []host.PanelEvent{{Kind: host.PanelEventOpen}}
	receipt, err := o.Deliver(update)
	if err != nil || !receipt.Paused || receipt.DOSCallsCommitted != 1 || o.dos.Mouse.X != 12 || o.dos.Mouse.Y != 34 || !ownerPanel(t, o).Open {
		t.Fatalf("Deliver = (%+v, %v)，mouse=%+v；要移動寫進 DOS、批次 paused", receipt, err, o.dos.Mouse)
	}
}

// 第 8 項：DOSMouse 的值、mickey 與 phase 規則。
func TestOwnerDOSMouseReportsValuesDuringPendingTurn(t *testing.T) {
	o := startSyntheticOwnerWithLayout(t, []byte{0x90, 0x90}, sessionLayout(1, host.OutputScale2, false))
	o.dos.Mouse.X, o.dos.Mouse.Y = 50, 20
	before, err := o.DOSMouse()
	if err != nil {
		t.Fatal(err)
	}
	const dx = 7
	if _, err := o.Deliver(pointerUpdate(t, o, []host.MouseEvent{dosMove(50+dx, 20)})); err != nil {
		t.Fatal(err)
	}
	if !o.turnPending {
		t.Fatal("Deliver 後應在回合進行中")
	}
	got, err := o.DOSMouse()
	m := o.dos.Mouse
	want := DOSMouseState{X: m.X, Y: m.Y, MickeyX: m.MickeyX, MickeyY: m.MickeyY, Buttons: m.Buttons}
	if err != nil || got != want {
		t.Fatalf("DOSMouse = (%+v, %v)，要 %+v", got, err, want)
	}
	if got.X != 50+dx || got.Y != 20 || got.MickeyX != before.MickeyX+dx || got.MickeyY != before.MickeyY {
		t.Fatalf("DOSMouse = %+v，前 %+v；要 MickeyX 加 %d、MickeyY 不變", got, before, dx)
	}

	booting, err := New(Config{InitialScale: host.OutputScale2})
	if err != nil {
		t.Fatal(err)
	}
	defer booting.Close()
	if _, err := booting.DOSMouse(); err == nil || booting.Status().Phase != PhaseBooting {
		t.Fatalf("Booting 時 DOSMouse err=%v phase=%d，要錯誤且 phase 不變", err, booting.Status().Phase)
	}
	o.Close()
	if _, err := o.DOSMouse(); err == nil || o.Status().Phase != PhaseClosed {
		t.Fatalf("Closed 時 DOSMouse err=%v phase=%d，要錯誤且 phase 不變", err, o.Status().Phase)
	}
}

// 第 9 項：拖曳。回呼在 Advance 時會真的遠呼叫 Handler，所以 Handler 指向合成程式裡的
// RETF 樁（回呼以 FarCallWithReturn 進入，只推返回的 CS:IP，結束用 RETF）。
func TestOwnerDeliverPointerMoveWhilePressedKeepsPress(t *testing.T) {
	// 0x100..0x103 是 NOP，0x104 是 RETF 樁；每次 Advance 只跑一步，碰不到樁。
	o := startSyntheticOwnerWithLayout(t, []byte{0x90, 0x90, 0x90, 0x90, 0xCB}, sessionLayout(1, host.OutputScale2, false))
	setMouseHandler(o)
	o.dos.Mouse.Handler.Seg, o.dos.Mouse.Handler.Off = o.machine.CPU.Seg[cpu.CS], 0x104

	first := pointerUpdate(t, o, nil)
	first.PointerDown = dosPointer(host.MouseEventDown, 10, 10)
	if _, err := o.Deliver(first); err != nil {
		t.Fatal(err)
	}
	consumeRunningTurn(t, o, 1)

	before := len(o.dos.Mouse.Events)
	if _, err := o.Deliver(pointerUpdate(t, o, []host.MouseEvent{dosMove(20, 10)})); err != nil {
		t.Fatal(err)
	}
	state, err := o.DOSMouse()
	events := o.dos.Mouse.Events
	if err != nil || len(events) != before+1 || events[len(events)-1].Buttons != dos.EvMove || state.Buttons&1 == 0 {
		t.Fatalf("拖曳移動：DOSMouse=(%+v, %v) events=%+v；要多一筆移動、左鍵仍按著", state, err, events)
	}
	consumeRunningTurn(t, o, 2)

	third := pointerUpdate(t, o, nil)
	third.PointerUp = dosPointer(host.MouseEventUp, 20, 10)
	plan, err := o.prepare(third)
	if err != nil || len(plan.mouseEvents) != 1 || plan.mouseEvents[0].route.Reason != "up-release" {
		t.Fatalf("第三批 prepare = (%+v, %v)，要 up-release", plan.mouseEvents, err)
	}
	receipt, err := o.commit(plan)
	if err != nil || receipt.DOSCallsCommitted != 2 || o.dos.Mouse.Buttons != 0 || o.mouse.Pressed() {
		t.Fatalf("第三批 commit = (%+v, %v)，mouse=%+v", receipt, err, o.dos.Mouse)
	}
}
