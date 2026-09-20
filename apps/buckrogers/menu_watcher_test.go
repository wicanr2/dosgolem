package buckrogers

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func menuWatcherFixture(t *testing.T) (*MenuRequestWatcher, TextEvent, [6]uint16, []byte) {
	t.Helper()
	original := []byte("fixture!")
	sum := sha256.Sum256(original)
	events := strings.Replace(menuEventFixture,
		"28f962ee4f76bcaea2e7d195fccb11782ecb4aea3c8f78e342118d69968e5d69",
		hex.EncodeToString(sum[:]), 1)
	c, err := LoadMenuCatalog([]byte(events), []byte(menuTextFixture))
	if err != nil {
		t.Fatal(err)
	}
	event := fixtureMenuEvent(t)
	event.OriginalSHA256 = sum
	args := [6]uint16{0x1234, 0x5678, uint16(event.Background), uint16(event.Foreground), uint16(event.Row), uint16(event.Column)}
	return NewMenuRequestWatcher(c), event, args, original
}

func TestMenuRequestWatcherSubmitsOnlyAfterGuardedReturn(t *testing.T) {
	w, event, args, original := menuWatcherFixture(t)
	w.ObserveDispatchEntry(event.Caller, 0x1841, 0x3900, args, original, 100)
	if len(w.Events()) != 0 || len(w.Requests()) != 0 || !w.Pending() {
		t.Fatal("entry 不得提交事件或請求")
	}
	w.ObserveInstruction(Address{0x1111, 0x2222}, 0x1841, 0x3910, 150)
	if len(w.Requests()) != 0 || !w.Pending() {
		t.Fatal("不相關 instruction 不得提交或丟棄")
	}
	w.ObserveInstruction(event.Caller, 0x1841, 0x3910, 200)
	requests := w.Requests()
	if w.Pending() || w.Drops() != 0 || w.Misses() != 0 || len(w.Events()) != 1 || len(requests) != 1 {
		t.Fatalf("events=%d requests=%d pending=%v drops=%d misses=%d", len(w.Events()), len(requests), w.Pending(), w.Drops(), w.Misses())
	}
	if requests[0].EventKey != "race.option.terran" || requests[0].TextKey != "race.terran" || requests[0].Translation != "地球人" {
		t.Fatalf("request = %#v", requests[0])
	}
}

func TestMenuRequestWatcherGuardAndOverlapFailClosed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		ss, sp uint16
	}{
		{"錯誤 SS", 0x1842, 0x3910},
		{"錯誤 SP", 0x1841, 0x390F},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, event, args, original := menuWatcherFixture(t)
			w.ObserveDispatchEntry(event.Caller, 0x1841, 0x3900, args, original, 100)
			w.ObserveInstruction(event.Caller, tc.ss, tc.sp, 200)
			if w.Pending() || w.Drops() != 1 || len(w.Events()) != 0 || len(w.Requests()) != 0 {
				t.Fatal("guard 失敗必須丟棄且不提交")
			}
		})
	}
	w, event, args, original := menuWatcherFixture(t)
	w.ObserveDispatchEntry(event.Caller, 1, 2, args, original, 1)
	w.ObserveDispatchEntry(event.Caller, 1, 2, args, original, 2)
	if w.Pending() || w.Drops() != 1 || len(w.Events()) != 0 || len(w.Requests()) != 0 {
		t.Fatal("重疊 entry 必須失敗即關閉")
	}
}

func TestMenuRequestWatcherCatalogMissAndNilCatalog(t *testing.T) {
	w, event, args, _ := menuWatcherFixture(t)
	w.ObserveDispatchEntry(event.Caller, 1, 2, args, []byte("Unknown!"), 1)
	w.ObserveInstruction(event.Caller, 1, 0x12, 2)
	if len(w.Events()) != 1 || len(w.Requests()) != 0 || w.Misses() != 1 {
		t.Fatal("未知 identity 應保留完成事件、計一次 miss 且不提交")
	}

	nilWatcher := NewMenuRequestWatcher(nil)
	nilWatcher.ObserveDispatchEntry(event.Caller, 1, 2, args, []byte("fixture!"), 1)
	nilWatcher.ObserveInstruction(event.Caller, 1, 0x12, 2)
	if len(nilWatcher.Events()) != 1 || len(nilWatcher.Requests()) != 0 || nilWatcher.Misses() != 0 {
		t.Fatal("nil catalog 應維持純 recorder 模式")
	}
}

func TestMenuRequestWatcherReturnsCopies(t *testing.T) {
	w, event, args, original := menuWatcherFixture(t)
	w.ObserveDispatchEntry(event.Caller, 1, 2, args, original, 1)
	w.ObserveInstruction(event.Caller, 1, 0x12, 2)
	events, requests := w.Events(), w.Requests()
	events[0].Caller = Address{}
	requests[0].EventKey = "mutated"
	if w.Events()[0].Caller != event.Caller || w.Requests()[0].EventKey != "race.option.terran" {
		t.Fatal("回傳切片不得污染 watcher")
	}
}
