package sbpcm

import (
	"github.com/wicanr2/dosgolem/internal/machine"
	"math/big"
	"reflect"
	"testing"
)

func start(at, due uint64, data ...byte) machine.SBPCMEvent {
	return machine.SBPCMEvent{Kind: "start", Clock: "steps", Command: 0x14, Start: at, Step: at, Due: due, Length: uint32(len(data)), Data: data}
}

func TestFormula(t *testing.T) {
	s := New(48000 * 5)
	s.Apply(start(0, 10, 0, 64, 128, 255))
	// j=0: k=0、f=0 → -32768；j=1: k=2、f=0 → 0。
	for j, want := range []int32{-32768, 0, 0} {
		if got := s.Sample(uint64(j)); got != want {
			t.Fatalf("j=%d: %d want %d", j, got, want)
		}
	}
	// sampleOf(1)=0，首個輸出時刻比 start 早，d 必須夾成 0。
	s.Reset()
	s.Apply(start(1, 11, 127, 128, 129, 130))
	if got := s.Sample(0); got != -256 {
		t.Fatalf("d < 0: %d", got)
	}
	// j=1: d=192000、N=4、den=480000 → k=1、f=39321。
	// (0*26215+1*39321)*256>>16 = 153。
	if got := s.Sample(1); got != 153 {
		t.Fatalf("fraction: %d want 153", got)
	}
	// 負數內插採算術右移，-256*26215>>16 = -103，不能截成 -102。
	s.Reset()
	s.Apply(start(1, 11, 128, 127, 128, 128))
	if got := s.Sample(1); got != -103 {
		t.Fatalf("negative floor: %d want -103", got)
	}
	s = New(48000)
	s.Apply(start(0, 10, 1, 255))
	if got := s.Sample(9); got != 32512 {
		t.Fatalf("last held: %d", got)
	}
}

func TestPauseResumeAndCancel(t *testing.T) {
	s := New(48000)
	s.Apply(start(0, 100, 0, 32, 64, 96, 128, 160, 192, 224, 255, 255))
	s.Apply(machine.SBPCMEvent{Kind: "pause", Clock: "steps", Step: 35})
	s.Apply(machine.SBPCMEvent{Kind: "resume", Clock: "steps", Step: 55, Start: 20, Due: 120, Transferred: 3})
	s.Apply(machine.SBPCMEvent{Kind: "cancel", Clock: "steps", Step: 85})
	if s.Sample(0) != -32768 || s.Sample(34) == 0 {
		t.Fatal("resume 改壞先前區段")
	}
	for j := uint64(35); j < 55; j++ {
		if s.Sample(j) != 0 {
			t.Fatalf("pause j=%d", j)
		}
	}
	// resume 第 1 點 k=3、f=32768，(-32+0)/2*256=-4096。
	if got := s.Sample(55); got != -4096 {
		t.Fatalf("resume: %d want -4096", got)
	}
	if s.Sample(84) == 0 || s.Sample(85) != 0 {
		t.Fatal("cancel 訖點")
	}
	s.Prune(85)
	if s.Stats().Voices != 0 {
		t.Fatal("已結束聲部未釋放")
	}
}

func TestTwoTransfersAndImplicitCancel(t *testing.T) {
	s := New(48000)
	s.Apply(start(0, 10, 255))
	s.Apply(machine.SBPCMEvent{Kind: "complete", Clock: "steps", Step: 10})
	s.Apply(start(12, 22, 0))
	for j := uint64(0); j < 22; j++ {
		want := int32(32512)
		if j >= 10 {
			want = 0
		}
		if j >= 12 {
			want = -32768
		}
		if got := s.Sample(j); got != want {
			t.Fatalf("two transfers j=%d: %d want %d", j, got, want)
		}
	}
	s.Apply(start(15, 25, 129))
	if s.Sample(14) != -32768 || s.Sample(15) != 256 {
		t.Fatal("隱式 cancel")
	}
}

func TestStatsResetEmptyAndPrune(t *testing.T) {
	s := New(48000)
	s.Apply(machine.SBPCMEvent{Kind: "start", Clock: "steps", Command: 0x74})
	s.Apply(machine.SBPCMEvent{Kind: "start", Clock: "cycles", Command: 0x14})
	s.Apply(machine.SBPCMEvent{Kind: "pause", Clock: "steps"})
	s.Apply(start(1, 2, 128))
	s.Apply(machine.SBPCMEvent{Kind: "complete", Clock: "steps", Step: 3})
	if got := s.Stats(); got.ADPCMSkipped != 1 || got.CycleClockSkipped != 1 || got.OrphanEvents != 1 || got.CompleteMismatch != 1 {
		t.Fatalf("stats: %+v", got)
	}
	if s.Sample(1) != 0 {
		t.Fatal("計數夾具應無聲")
	}
	s.Reset()
	if got := s.Stats(); got.Voices != 0 || got.ADPCMSkipped != 1 {
		t.Fatalf("reset: %+v", got)
	}
	s = New(240000)
	s.Apply(start(1, 2, 255))
	s.Apply(machine.SBPCMEvent{Kind: "cancel", Clock: "steps", Step: 1})
	if s.voices[0].data != nil {
		t.Fatal("空聲部仍持有樣本")
	}
	s.Prune(0)
	if s.Stats().Voices != 0 {
		t.Fatal("空聲部未移除")
	}
}

func TestSamplePure(t *testing.T) {
	s := New(240000)
	s.Apply(start(1, 101, 0, 50, 100, 150, 200, 255))
	before := s.Stats()
	a, b := s.Sample(7), s.Sample(3)
	if !reflect.DeepEqual(before, s.Stats()) || a != s.Sample(7) || b != s.Sample(3) {
		t.Fatal("Sample 改狀態")
	}
}

// 讓 d*N 與 remainder*65536 都超過 uint64，商仍在來源長度內。
func TestWideInterpolation(t *testing.T) {
	s := New(11550000)
	data := make([]byte, 65536)
	for i := range data {
		data[i] = byte(i)
	}
	const a, b = uint64(100000003), uint64(100100000003)
	s.Apply(start(a, b, data...))
	j := s.sampleOf((a + b) / 2)
	// 獨立的任意精度算式，確認 128 位乘積與餘數不會截斷。
	d := new(big.Int).Sub(new(big.Int).SetUint64(j*s.sps), new(big.Int).SetUint64(a*48000))
	num := new(big.Int).Mul(d, big.NewInt(65536))
	den := new(big.Int).SetUint64(48000 * (b - a))
	k, r := new(big.Int), new(big.Int)
	k.QuoRem(num, den, r)
	f := new(big.Int).Quo(new(big.Int).Lsh(r, 16), den).Int64()
	x0, x1 := int64(data[k.Uint64()])-128, int64(data[min(k.Uint64()+1, 65535)])-128
	want := int32((x0*(65536-f) + x1*f) * 256 >> 16)
	if got := s.Sample(j); got != want {
		t.Fatalf("wide: %d want %d", got, want)
	}
}
