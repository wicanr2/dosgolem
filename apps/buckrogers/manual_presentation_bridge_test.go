package buckrogers

import "testing"

func newManualPresentationBridgeFixture(t *testing.T) (*ManualPresentationBridge, *Watcher, *ManualPresentationConsumer, *RuntimeManualOverlay, *Catalog) {
	t.Helper()
	consumer, overlay, catalog := newManualPresentationConsumerFixture(t)
	watcher := NewWatcher(catalog)
	bridge, err := NewManualPresentationBridge(watcher, consumer)
	if err != nil {
		t.Fatal(err)
	}
	return bridge, watcher, consumer, overlay, catalog
}

func TestManualPresentationBridgeForwardsAppendOnlyWatcherSnapshotsOnce(t *testing.T) {
	bridge, watcher, consumer, overlay, catalog := newManualPresentationBridgeFixture(t)
	events := manualPresentationEvents(catalog)
	watcher.presentation = append(watcher.presentation, events[0])
	if got, err := bridge.Sync(); err != nil || got != 1 {
		t.Fatalf("begin sync=%d,%v", got, err)
	}
	watcher.presentation = append(watcher.presentation, events[1:]...)
	if got, err := bridge.Sync(); err != nil || got != 2 {
		t.Fatalf("append sync=%d,%v", got, err)
	}
	assertConsumed(t, consumer, events)
	if overlay.ActiveGeneration() != 1 || len(overlay.ActiveKeys()) != 14 || len(overlay.Actions()) != 1 {
		t.Fatalf("visible state generation=%d keys=%d actions=%d", overlay.ActiveGeneration(), len(overlay.ActiveKeys()), len(overlay.Actions()))
	}
	if got, err := bridge.Sync(); err != nil || got != 0 {
		t.Fatalf("replay sync=%d,%v", got, err)
	}
	if len(overlay.Actions()) != 1 {
		t.Fatalf("完整 watcher replay 不得重複 action：%d", len(overlay.Actions()))
	}
}

func TestManualPresentationBridgeFailsClosedForWatcherHistoryDrift(t *testing.T) {
	bridge, watcher, consumer, overlay, catalog := newManualPresentationBridgeFixture(t)
	events := manualPresentationEvents(catalog)
	watcher.presentation = append(watcher.presentation, events...)
	if got, err := bridge.Sync(); err != nil || got != len(events) {
		t.Fatalf("initial sync=%d,%v", got, err)
	}

	watcher.presentation[0].Step++
	if got, err := bridge.Sync(); err == nil || got != 0 {
		t.Fatalf("漂移 watcher history 必須拒絕，got=%d err=%v", got, err)
	}
	assertConsumed(t, consumer, events)
	if overlay.ActiveGeneration() != 1 || len(overlay.ActiveKeys()) != 14 || len(overlay.Actions()) != 1 {
		t.Fatalf("漂移失敗不得改 presenter：generation=%d keys=%d actions=%d", overlay.ActiveGeneration(), len(overlay.ActiveKeys()), len(overlay.Actions()))
	}
}

func TestManualPresentationBridgePropagatesConsumerFailureWithoutAdvancing(t *testing.T) {
	bridge, watcher, consumer, overlay, _ := newManualPresentationBridgeFixture(t)
	watcher.presentation = append(watcher.presentation,
		ManualPresentationEvent{Step: 10, Kind: ManualPresentationBegin, Generation: 1},
		ManualPresentationEvent{Step: 20, Kind: ManualPresentationClear, Generation: 2},
	)
	if got, err := bridge.Sync(); err == nil || got != 1 {
		t.Fatalf("部分失敗 sync=%d,%v", got, err)
	}
	assertConsumed(t, consumer, watcher.presentation[:1])
	if overlay.ActiveGeneration() != 1 || len(overlay.ActiveKeys()) != 0 || len(overlay.Actions()) != 0 {
		t.Fatalf("部分失敗 state generation=%d keys=%d actions=%d", overlay.ActiveGeneration(), len(overlay.ActiveKeys()), len(overlay.Actions()))
	}
	if got, err := bridge.Sync(); err == nil || got != 0 {
		t.Fatalf("失敗 event 重試不得跳過，got=%d err=%v", got, err)
	}
}

func TestManualPresentationBridgeRejectsNil(t *testing.T) {
	consumer, _, catalog := newManualPresentationConsumerFixture(t)
	watcher := NewWatcher(catalog)
	if got, err := NewManualPresentationBridge(nil, consumer); got != nil || err == nil {
		t.Fatalf("nil watcher constructor=%#v,%v", got, err)
	}
	if got, err := NewManualPresentationBridge(watcher, nil); got != nil || err == nil {
		t.Fatalf("nil consumer constructor=%#v,%v", got, err)
	}
	var bridge *ManualPresentationBridge
	if got, err := bridge.Sync(); err == nil || got != 0 {
		t.Fatalf("nil bridge sync=%d,%v", got, err)
	}
}
