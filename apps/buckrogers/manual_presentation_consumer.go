package buckrogers

import "fmt"

// ManualPresentationConsumer applies append-only watcher snapshots to one
// output-only manual overlay. It never reconstructs lifecycle values from
// observations or touches the original machine.
type ManualPresentationConsumer struct {
	overlay  *RuntimeManualOverlay
	consumed []ManualPresentationEvent
}

// NewManualPresentationConsumer accepts only an already validated presenter.
func NewManualPresentationConsumer(overlay *RuntimeManualOverlay) (*ManualPresentationConsumer, error) {
	if overlay == nil {
		return nil, fmt.Errorf("buckrogers: 手冊 presentation consumer 缺少 presenter")
	}
	return &ManualPresentationConsumer{overlay: overlay}, nil
}

// Consume accepts only an unchanged consumed prefix followed by new events.
// Each event becomes consumed only after the presenter has accepted it.
func (c *ManualPresentationConsumer) Consume(events []ManualPresentationEvent) (int, error) {
	if c == nil || c.overlay == nil {
		return 0, fmt.Errorf("buckrogers: 手冊 presentation consumer 無效")
	}
	if len(events) < len(c.consumed) {
		return 0, fmt.Errorf("buckrogers: 手冊 presentation snapshot 不得縮短")
	}
	for i := range c.consumed {
		if events[i] != c.consumed[i] {
			return 0, fmt.Errorf("buckrogers: 手冊 presentation snapshot 歷史於 %d 漂移", i)
		}
	}
	start := len(c.consumed)
	for i := start; i < len(events); i++ {
		if err := c.overlay.Apply(events[i]); err != nil {
			return i - start, fmt.Errorf("buckrogers: 手冊 presentation event %d 拒絕：%w", i, err)
		}
		c.consumed = append(c.consumed, events[i])
	}
	return len(events) - start, nil
}

// Consumed returns a defensive value copy of the successfully applied prefix.
func (c *ManualPresentationConsumer) Consumed() ([]ManualPresentationEvent, error) {
	if c == nil || c.overlay == nil {
		return nil, fmt.Errorf("buckrogers: 手冊 presentation consumer 無效")
	}
	return append([]ManualPresentationEvent(nil), c.consumed...), nil
}
