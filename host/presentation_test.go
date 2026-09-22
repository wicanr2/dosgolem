package host

import (
	"errors"
	"strings"
	"testing"
)

type testFrameSource struct {
	frame IndexedFrame
	err   error
	calls int
}

func (s *testFrameSource) ReadPresentationFrame() (IndexedFrame, error) {
	s.calls++
	if s.err != nil {
		return IndexedFrame{}, s.err
	}
	return s.frame, nil
}

func TestPresentationSnapshotOwnsIndexedPixels(t *testing.T) {
	source := &testFrameSource{frame: IndexedFrame{
		Canvas:  Canvas{Width: 2, Height: 2},
		Indexed: []uint8{1, 2, 3, 4},
		Palette: [256][3]uint8{1: {9, 8, 7}},
	}}
	provider, err := NewPresentationSnapshotProvider(source)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := provider.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if source.calls != 1 || snapshot.Canvas != source.frame.Canvas || snapshot.Palette != source.frame.Palette {
		t.Fatalf("snapshot=%#v calls=%d", snapshot, source.calls)
	}
	source.frame.Indexed[0] = 99
	if snapshot.Indexed[0] != 1 {
		t.Fatalf("source mutation leaked into snapshot: %v", snapshot.Indexed)
	}
	snapshot.Indexed[1] = 88
	if source.frame.Indexed[1] != 2 {
		t.Fatalf("snapshot mutation leaked into source: %v", source.frame.Indexed)
	}
	again, err := provider.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if again.Indexed[0] != 99 || again.Indexed[1] != 2 {
		t.Fatalf("next snapshot did not reflect only source state: %v", again.Indexed)
	}
}

func TestPresentationSnapshotRejectsBadSources(t *testing.T) {
	if provider, err := NewPresentationSnapshotProvider(nil); provider != nil || err == nil {
		t.Fatalf("nil source provider=%#v err=%v", provider, err)
	}
	var typedNil *testFrameSource
	if provider, err := NewPresentationSnapshotProvider(typedNil); provider != nil || err == nil {
		t.Fatalf("typed-nil source provider=%#v err=%v", provider, err)
	}
	maxInt := int(^uint(0) >> 1)
	for name, source := range map[string]*testFrameSource{
		"source error":   {err: errors.New("read failed")},
		"zero width":     {frame: IndexedFrame{Canvas: Canvas{Width: 0, Height: 1}}},
		"zero height":    {frame: IndexedFrame{Canvas: Canvas{Width: 1, Height: 0}}},
		"bad length":     {frame: IndexedFrame{Canvas: Canvas{Width: 2, Height: 2}, Indexed: []uint8{1}}},
		"pixel overflow": {frame: IndexedFrame{Canvas: Canvas{Width: maxInt, Height: 2}}},
	} {
		t.Run(name, func(t *testing.T) {
			provider, err := NewPresentationSnapshotProvider(source)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := provider.Snapshot(); err == nil {
				t.Fatal("Snapshot 必須失敗")
			} else if name == "pixel overflow" && !strings.Contains(err.Error(), "溢位") {
				t.Fatalf("overflow err=%v", err)
			}
		})
	}
	var nilProvider *PresentationSnapshotProvider
	if _, err := nilProvider.Snapshot(); err == nil || !strings.Contains(err.Error(), "不得為 nil") {
		t.Fatalf("nil provider err=%v", err)
	}
}
