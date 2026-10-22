package session

import (
	"bytes"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// docs/spec/458-session-indexed-into-and-videowrite-ctx §3.1 第 6 項與第 9 項的 session 部分。

// frame458COM：int 10h 設 mode 13h，之後迴圈寫像素（ES=A000，BL 遞增）；0180h 是回呼目標 RETF。
//
//	0100 B8 13 00 mov ax,13h   0103 CD 10 int 10h   0105 B8 00 A0 mov ax,A000   0108 8E C0 mov es,ax
//	010A 31 FF xor di,di       010C 26 88 1D mov es:[di],bl   010F 43 inc bx   0110 47 inc di   0111 EB F9 jmp 010C
func frame458COM() []byte {
	code := make([]byte, 0x81)
	copy(code, []byte{0xB8, 0x13, 0x00, 0xCD, 0x10, 0xB8, 0x00, 0xA0, 0x8E, 0xC0, 0x31, 0xFF,
		0x26, 0x88, 0x1D, 0x43, 0x47, 0xEB, 0xF9})
	code[0x80] = 0xCB
	return code
}

const every458 = 997

// frameTimingObserver 每個觀察點以 IndexedInto 存進固定緩衝；Frame 比對收到的 indexed 與最近一次的緩衝。
// cbAfter > 0 時，在第 cbAfter 次回掃之後、下一次回掃應發生的那一步的觀察點排一個回呼，讓回掃延後一步。
type frameTimingObserver struct {
	o          *Owner
	buf        []byte
	obsStep    uint64
	frames     int
	bad        int
	stale      int
	lastFrame  uint64
	cbAfter    int
	cbAt       uint64
	delayed    bool
	frameSteps []uint64
}

func (f *frameTimingObserver) BeforeStep(v StepView) error {
	vp := &v
	if f.buf == nil {
		l := vp.IndexedLayout()
		f.buf = make([]byte, l.W*l.H)
	}
	if _, ok := vp.IndexedInto(f.buf); !ok {
		f.bad++
	}
	f.obsStep = vp.Steps()
	if f.cbAfter > 0 && f.frames == f.cbAfter && f.cbAt == 0 && f.obsStep == f.lastFrame+every458 {
		f.cbAt = f.obsStep
		f.o.machine.QueueCallback(machine.QueuedCall{Seg: machine.PSPSeg, Off: 0x0180})
	}
	return nil
}
func (f *frameTimingObserver) VideoWrite(machine.VideoWrite) {}
func (f *frameTimingObserver) Frame(indexed []byte, _ [256][3]uint8) {
	f.frames++
	s := f.o.machine.Steps
	f.frameSteps = append(f.frameSteps, s)
	if s != f.obsStep {
		f.stale++
	}
	if !bytes.Equal(indexed, f.buf) {
		f.bad++
	}
	if f.cbAt != 0 && s == f.cbAt+1 {
		f.delayed = true
	}
	f.lastFrame = s
}

func TestIndexedInto458FrameTiming(t *testing.T) {
	for _, withCB := range []bool{false, true} {
		o := startSyntheticOwner(t, frame458COM())
		o.machine.VGAFrameEvery = every458
		f := &frameTimingObserver{o: o}
		if withCB {
			f.cbAfter = 10
		}
		o.observer = f
		if _, err := runTurn(t, o, machine.DefaultVGAFrameEvery+70*every458); err != nil {
			t.Fatal(err)
		}
		if o.machine.VideoMode() != 0x13 || len(f.buf) != 64000 {
			t.Fatalf("前提：mode 13h、緩衝 64000（mode %02X、%d）", o.machine.VideoMode(), len(f.buf))
		}
		if f.frames < 50 || f.bad != 0 || f.stale != 0 {
			t.Errorf("回呼=%v：%d 次回掃，%d 次與觀察點的 IndexedInto 不同，%d 次步數不同", withCB, f.frames, f.bad, f.stale)
		}
		if withCB && (f.cbAt == 0 || !f.delayed || o.machine.CallbacksMade() != 1) {
			t.Errorf("回呼案：應在回掃那一步排入回呼並讓回掃延後一步（排入步 %d、延後 %v、回呼 %d）",
				f.cbAt, f.delayed, o.machine.CallbacksMade())
		}
	}
}

// staleIndexedObserver 第 1 步保留視圖；第 2 步以舊視圖呼叫 IndexedInto／IndexedLayout（或在 Frame 內以舊視圖呼叫）。
type staleIndexedObserver struct {
	keep    *StepView
	n       int
	inFrame bool
	dst     []byte
	gotOK   bool
	gotL    machine.IndexedLayout
	gotL2   machine.IndexedLayout
	allocs  float64
	called  bool
}

func (s *staleIndexedObserver) BeforeStep(v StepView) error {
	s.n++
	if s.keep == nil {
		kept := v
		s.keep = &kept
		vp := &v
		buf := make([]byte, 64000)
		s.allocs = testing.AllocsPerRun(10, func() { vp.IndexedInto(buf) })
		return nil
	}
	if s.n == 2 && !s.inFrame {
		s.call()
	}
	return nil
}

func (s *staleIndexedObserver) call() {
	s.called = true
	s.dst = bytes.Repeat([]byte{0xA5}, 64000)
	s.gotL, s.gotOK = s.keep.IndexedInto(s.dst)
	s.gotL2 = s.keep.IndexedLayout()
}

func (s *staleIndexedObserver) VideoWrite(machine.VideoWrite) {}
func (s *staleIndexedObserver) Frame([]byte, [256][3]uint8) {
	if s.inFrame && !s.called {
		s.call()
	}
}

func TestIndexedInto458Misuse(t *testing.T) {
	for _, inFrame := range []bool{false, true} {
		o := startSyntheticOwner(t, frame458COM())
		o.machine.SetVideoMode(0x13)
		s := &staleIndexedObserver{inFrame: inFrame}
		o.observer = s
		r, err := runTurn(t, o, machine.DefaultVGAFrameEvery+200) // 第一次回掃在第 165000 步
		if err == nil || !strings.Contains(err.Error(), "觀測器在回呼外使用了 StepView") || r.Reason != StopReasonObserverFault {
			t.Fatalf("inFrame=%v：應回觀測器誤用：%+v %v", inFrame, r, err)
		}
		if !s.called || s.gotOK || s.gotL != (machine.IndexedLayout{}) || s.gotL2 != (machine.IndexedLayout{}) ||
			!bytes.Equal(s.dst, bytes.Repeat([]byte{0xA5}, 64000)) {
			t.Errorf("inFrame=%v：保留的視圖應不寫 dst、回零值與 ok=false（called=%v ok=%v）", inFrame, s.called, s.gotOK)
		}
		if s.allocs != 0 {
			t.Errorf("StepView.IndexedInto（以 *StepView 呼叫）每次配置 %v 次", s.allocs)
		}
	}
}
