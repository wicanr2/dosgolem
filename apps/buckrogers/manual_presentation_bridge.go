package buckrogers

import "fmt"

// ManualPresentationBridge forwards the watcher's defensive lifecycle
// snapshot to the one consumer that owns presentation history and cursor.
type ManualPresentationBridge struct {
	watcher  *Watcher
	consumer *ManualPresentationConsumer
}

// NewManualPresentationBridge requires the existing output-only watcher and
// an already validated consumer. It does not create a presenter or a font.
func NewManualPresentationBridge(watcher *Watcher, consumer *ManualPresentationConsumer) (*ManualPresentationBridge, error) {
	if watcher == nil {
		return nil, fmt.Errorf("buckrogers: 手冊 presentation bridge 缺少 watcher")
	}
	if consumer == nil {
		return nil, fmt.Errorf("buckrogers: 手冊 presentation bridge 缺少 consumer")
	}
	return &ManualPresentationBridge{watcher: watcher, consumer: consumer}, nil
}

// Sync forwards exactly one current watcher snapshot. Consumer owns all
// prefix checks, failure behavior, and successful-event bookkeeping.
func (b *ManualPresentationBridge) Sync() (int, error) {
	if b == nil || b.watcher == nil || b.consumer == nil {
		return 0, fmt.Errorf("buckrogers: 手冊 presentation bridge 無效")
	}
	return b.consumer.Consume(b.watcher.PresentationEvents())
}
