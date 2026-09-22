package host

import "fmt"

// Canvas 描述交給 host presenter 的未縮放索引色畫布。
// 它刻意不依賴任何特定 DOS 顯示模式或視窗後端。
type Canvas struct {
	Width  int
	Height int
}

func (c Canvas) valid() bool { return c.Width > 0 && c.Height > 0 }

// IndexedFrame 是一份未縮放的畫面輸入。Palette 是值；
// PresentationSnapshot 一律持有自己的 Indexed bytes。兩者都不讓 host
// 後端取得修改 DOS machine 的路徑。
type IndexedFrame struct {
	Canvas  Canvas
	Indexed []uint8
	Palette [256][3]uint8
}

// FrameSource 供應一致的一組 indexed framebuffer 與 palette。frontend 自行決定
// 何時呼叫 ReadPresentationFrame；不得從另一個同時 Step machine 的 goroutine 呼叫。
// source 可以重用回傳 bytes：PresentationSnapshotProvider 會建立自己的複本。
type FrameSource interface {
	ReadPresentationFrame() (IndexedFrame, error)
}

// PresentationSnapshot 是依契約唯讀的 host 輸入。每次 Snapshot 均會新建 Indexed
// 複本，因此 renderer 的修改不會影響 source 或後續 snapshot。
type PresentationSnapshot struct {
	Canvas  Canvas
	Indexed []uint8
	Palette [256][3]uint8
}

// PresentationSnapshotProvider 將唯讀 FrameSource 轉接給 host presenter。
// 它不含輸入轉送、DOS 狀態轉移、存檔狀態或任何遊戲專屬知識。
type PresentationSnapshotProvider struct {
	source FrameSource
}

func NewPresentationSnapshotProvider(source FrameSource) (*PresentationSnapshotProvider, error) {
	if source == nil {
		return nil, fmt.Errorf("host: presentation frame source 不得為 nil")
	}
	return &PresentationSnapshotProvider{source: source}, nil
}

// Snapshot 取得並驗證剛好一份畫面輸入，接著取得 indexed pixels 複本的擁有權。
// 它不會對 machine 寫入。
func (p *PresentationSnapshotProvider) Snapshot() (PresentationSnapshot, error) {
	if p == nil || p.source == nil {
		return PresentationSnapshot{}, fmt.Errorf("host: presentation snapshot provider 不得為 nil")
	}
	frame, err := p.source.ReadPresentationFrame()
	if err != nil {
		return PresentationSnapshot{}, fmt.Errorf("host: 讀取 presentation frame: %w", err)
	}
	if !frame.Canvas.valid() {
		return PresentationSnapshot{}, fmt.Errorf("host: presentation canvas 必須有正寬高，得到 %dx%d", frame.Canvas.Width, frame.Canvas.Height)
	}
	want := frame.Canvas.Width * frame.Canvas.Height
	if len(frame.Indexed) != want {
		return PresentationSnapshot{}, fmt.Errorf("host: presentation indexed 長度=%d，預期 %d (%dx%d)", len(frame.Indexed), want, frame.Canvas.Width, frame.Canvas.Height)
	}
	indexed := make([]uint8, len(frame.Indexed))
	copy(indexed, frame.Indexed)
	return PresentationSnapshot{Canvas: frame.Canvas, Indexed: indexed, Palette: frame.Palette}, nil
}
