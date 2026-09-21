package buckrogers

import "testing"

func newManualPresentationConsumerFixture(t *testing.T) (*ManualPresentationConsumer, *RuntimeManualOverlay, *Catalog) {
	t.Helper()
	catalog := manualOverlayCatalog("繁中段落")
	overlay, err := NewRuntimeManualOverlay(loadManualOverlayLayout(t), catalog, manualOverlayFont(catalog), 2)
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := NewManualPresentationConsumer(overlay)
	if err != nil {
		t.Fatal(err)
	}
	return consumer, overlay, catalog
}

func manualPresentationEvents(catalog *Catalog) []ManualPresentationEvent {
	return []ManualPresentationEvent{
		{Step: 10, Kind: ManualPresentationBegin, Generation: 1},
		{Step: 20, Kind: ManualPresentationClear, Generation: 1},
		{Step: 30, Kind: ManualPresentationRequest, Generation: 1, Request: manualOverlayRequest(catalog, 1)},
	}
}

func assertConsumed(t *testing.T, consumer *ManualPresentationConsumer, want []ManualPresentationEvent) {
	t.Helper()
	got, err := consumer.Consumed()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("consumed=%d，want=%d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("consumed[%d]=%#v，want=%#v", i, got[i], want[i])
		}
	}
}

func TestManualPresentationConsumerConsumesAppendOnlySnapshotsOnce(t *testing.T) {
	consumer, overlay, catalog := newManualPresentationConsumerFixture(t)
	events := manualPresentationEvents(catalog)

	if got, err := consumer.Consume(events[:1]); err != nil || got != 1 {
		t.Fatalf("begin consume=%d,%v", got, err)
	}
	if overlay.ActiveGeneration() != 1 || len(overlay.ActiveKeys()) != 0 || len(overlay.Actions()) != 0 {
		t.Fatalf("begin state generation=%d keys=%d actions=%d", overlay.ActiveGeneration(), len(overlay.ActiveKeys()), len(overlay.Actions()))
	}
	if got, err := consumer.Consume(events); err != nil || got != 2 {
		t.Fatalf("append consume=%d,%v", got, err)
	}
	assertConsumed(t, consumer, events)
	if len(overlay.ActiveKeys()) != 14 || len(overlay.Actions()) != 1 {
		t.Fatalf("request state keys=%d actions=%d", len(overlay.ActiveKeys()), len(overlay.Actions()))
	}
	if got, err := consumer.Consume(events); err != nil || got != 0 {
		t.Fatalf("replay consume=%d,%v", got, err)
	}
	if len(overlay.Actions()) != 1 {
		t.Fatalf("完整 replay 不得重複 action：%d", len(overlay.Actions()))
	}
}

func TestManualPresentationConsumerRejectsHistoryDriftAndShrink(t *testing.T) {
	consumer, overlay, catalog := newManualPresentationConsumerFixture(t)
	events := manualPresentationEvents(catalog)
	if got, err := consumer.Consume(events); err != nil || got != len(events) {
		t.Fatalf("initial consume=%d,%v", got, err)
	}

	mutated := append([]ManualPresentationEvent(nil), events...)
	mutated[0].Step++
	if got, err := consumer.Consume(mutated); err == nil || got != 0 {
		t.Fatalf("歷史 mutation 必須拒絕，got=%d err=%v", got, err)
	}
	if got, err := consumer.Consume(events[:2]); err == nil || got != 0 {
		t.Fatalf("snapshot shrink 必須拒絕，got=%d err=%v", got, err)
	}
	assertConsumed(t, consumer, events)
	if overlay.ActiveGeneration() != 1 || len(overlay.ActiveKeys()) != 14 || len(overlay.Actions()) != 1 {
		t.Fatalf("被拒 snapshot 不得改 presenter：generation=%d keys=%d actions=%d", overlay.ActiveGeneration(), len(overlay.ActiveKeys()), len(overlay.Actions()))
	}
}

func TestManualPresentationConsumerFailsClosedAndAdvancesOnlySuccessfulPrefix(t *testing.T) {
	consumer, overlay, catalog := newManualPresentationConsumerFixture(t)
	request := manualOverlayRequest(catalog, 1)
	invalid := []ManualPresentationEvent{{Step: 10, Kind: ManualPresentationRequest, Generation: 1, Request: request}}
	if got, err := consumer.Consume(invalid); err == nil || got != 0 {
		t.Fatalf("沒有 begin 的 request 必須拒絕，got=%d err=%v", got, err)
	}
	assertConsumed(t, consumer, nil)
	if overlay.ActiveGeneration() != 0 || len(overlay.ActiveKeys()) != 0 || len(overlay.Actions()) != 0 {
		t.Fatalf("無效第一筆不得改 presenter：generation=%d keys=%d actions=%d", overlay.ActiveGeneration(), len(overlay.ActiveKeys()), len(overlay.Actions()))
	}
	if got, err := consumer.Consume([]ManualPresentationEvent{{Step: 11, Kind: "unknown", Generation: 1}}); err == nil || got != 0 {
		t.Fatalf("未知 event 必須拒絕，got=%d err=%v", got, err)
	}
	assertConsumed(t, consumer, nil)

	partial := []ManualPresentationEvent{
		{Step: 10, Kind: ManualPresentationBegin, Generation: 1},
		{Step: 20, Kind: ManualPresentationClear, Generation: 2},
	}
	if got, err := consumer.Consume(partial); err == nil || got != 1 {
		t.Fatalf("部分失敗須只提交 begin，got=%d err=%v", got, err)
	}
	assertConsumed(t, consumer, partial[:1])
	if overlay.ActiveGeneration() != 1 || len(overlay.ActiveKeys()) != 0 || len(overlay.Actions()) != 0 {
		t.Fatalf("部分失敗 state generation=%d keys=%d actions=%d", overlay.ActiveGeneration(), len(overlay.ActiveKeys()), len(overlay.Actions()))
	}
	if got, err := consumer.Consume(partial); err == nil || got != 0 {
		t.Fatalf("失敗 event 重試不得越過 cursor，got=%d err=%v", got, err)
	}
	assertConsumed(t, consumer, partial[:1])
}

func TestManualPresentationConsumerCopiesHistoryAndRejectsNil(t *testing.T) {
	consumer, _, catalog := newManualPresentationConsumerFixture(t)
	events := manualPresentationEvents(catalog)
	if got, err := consumer.Consume(events); err != nil || got != len(events) {
		t.Fatalf("consume=%d,%v", got, err)
	}
	history, err := consumer.Consumed()
	if err != nil {
		t.Fatal(err)
	}
	history[0].Step = 999
	assertConsumed(t, consumer, events)

	if got, err := NewManualPresentationConsumer(nil); got != nil || err == nil {
		t.Fatalf("nil overlay constructor=%#v,%v", got, err)
	}
	var nilConsumer *ManualPresentationConsumer
	if got, err := nilConsumer.Consume(nil); err == nil || got != 0 {
		t.Fatalf("nil consumer consume=%d,%v", got, err)
	}
	if got, err := nilConsumer.Consumed(); err == nil || got != nil {
		t.Fatalf("nil consumer history=%#v,%v", got, err)
	}
}
