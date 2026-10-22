package session

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// reusingObserver 照 apps/strikecommander 的 SessionObserver（docs/spec/456-sc-observer-view-alloc）
// 的寫法：每次 BeforeStep 把 view 存進同一個欄位，再以指標傳給下游。違約時在回呼外（VideoWrite）
// 經這個指標讀 view。
type reusingObserver struct {
	cur  StepView
	p    *StepView
	read uint16
}

func (r *reusingObserver) BeforeStep(v StepView) error {
	r.cur = v
	r.p = &r.cur
	_ = r.p.IP()
	return nil
}

func (r *reusingObserver) VideoWrite(machine.VideoWrite) {
	r.read = r.p.IP() // 回呼外使用
}

func (r *reusingObserver) Frame([]byte, [256][3]uint8) {}

// 456 §3：欄位重用的 view 在回呼外使用仍判誤用。第 5 步 stosb 的 VideoWrite 讀 view（讀到 0），
// 第 6 步前 ObserverFault。
func TestReusedViewFieldOutsideCallbackIsDetected(t *testing.T) {
	obs := &reusingObserver{read: 0xEEEE}
	o := observedOwner(t, obs)
	r, err := runTurn(t, o, 20)
	if obs.read != 0 {
		t.Fatalf("回呼外經指標讀 IP 得 %#x，要 0", obs.read)
	}
	assertObserverFault(t, o, r, err, 5)
}
