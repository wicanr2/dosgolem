package strikecommander

import (
	"testing"

	"github.com/wicanr2/dosgolem/session"
)

// docs/spec/456-sc-observer-view-alloc：BeforeStep 以 &s.cur 傳給三個觀察器，每步不配置。
// 改回以值傳 StepView（每次轉介面配置一份 16 bytes 複本）時這個測試失敗。
// StepView 零值的方法都回 0（沒有 owner），三個觀察器只走 IP 分派。
func TestSessionObserverBeforeStepAllocs(t *testing.T) {
	fo := NewFlightObserver(func(Kind, string) (string, int, bool) { return "", 0, false })
	so := &SessionObserver{Labels: NewLabelObserver(), Flight: fo, Subtitles: &Subtitles{T: SubtitleDefaultT}}
	var v session.StepView
	if n := testing.AllocsPerRun(1000, func() { _ = so.BeforeStep(v) }); n != 0 {
		t.Fatalf("BeforeStep 每次配置 %.1f 次，要 0", n)
	}
}
