package session

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// 每圈寫 OPL 20h←01h、PIT 通道 2 分頻 1234h、喇叭 61h←03h 再 ←00h，然後跳回開頭。
var audioLoopCOM = []byte{
	0xBA, 0x88, 0x03, // mov dx,0388
	0xB0, 0x20, 0xEE, // mov al,20 / out dx,al
	0x42, 0xB0, 0x01, 0xEE, // inc dx / mov al,01 / out dx,al
	0xB0, 0xB6, 0xE6, 0x43, // mov al,B6 / out 43,al
	0xB0, 0x34, 0xE6, 0x42, // mov al,34 / out 42,al
	0xB0, 0x12, 0xE6, 0x42, // mov al,12 / out 42,al
	0xB0, 0x03, 0xE6, 0x61, // mov al,03 / out 61,al
	0xB0, 0x00, 0xE6, 0x61, // mov al,00 / out 61,al
	0xEB, 0xE0, // jmp 0100
}

const audioLoopSteps = 17 // 一圈的指令數

type audioRecorder struct {
	recordingObserver
	opl     []machine.OPLWrite
	speaker []machine.SpeakerSample
	pit2    []machine.PIT2Change
	panicAt int
}

func (a *audioRecorder) OPLWrite(w machine.OPLWrite) {
	a.opl = append(a.opl, w)
	if a.panicAt == len(a.opl) {
		panic("opl boom")
	}
}
func (a *audioRecorder) SpeakerSample(s machine.SpeakerSample) { a.speaker = append(a.speaker, s) }
func (a *audioRecorder) PITChannel2(c machine.PIT2Change)      { a.pit2 = append(a.pit2, c) }

func adlibOwner(t *testing.T, obs StepObserver) *Owner {
	t.Helper()
	o := startSyntheticOwner(t, audioLoopCOM)
	o.machine.SetAdLib(true)
	o.muteAudio()
	o.observer = obs
	return o
}

func TestAudioObserverReceivesWritesInTurnOnly(t *testing.T) {
	obs := &audioRecorder{}
	o := adlibOwner(t, obs)
	for i := 0; i < 3; i++ {
		if r, err := runTurn(t, o, audioLoopSteps); err != nil || r.Steps != audioLoopSteps {
			t.Fatalf("%+v %v", r, err)
		}
	}
	// OPL：每圈兩筆（選暫存器不算寫入，只有 389h 算一筆）。
	if len(obs.opl) != 3 {
		t.Fatalf("OPL %d 筆，要 3", len(obs.opl))
	}
	for _, w := range obs.opl {
		if w.Reg != 0x20 || w.Val != 0x01 || w.Bank != 0 {
			t.Fatalf("OPL 寫入 %+v", w)
		}
	}
	if len(obs.pit2) != 3 || obs.pit2[0].Divisor != 0x1234 || obs.pit2[0].Mode != 3 {
		t.Fatalf("PIT2 %+v", obs.pit2)
	}
	// 喇叭每圈兩次變化（11 → 00）。
	if len(obs.speaker) != 6 || obs.speaker[0].Level != 1 || obs.speaker[0].Gate != 1 || obs.speaker[1].Level != 0 {
		t.Fatalf("喇叭 %+v", obs.speaker)
	}
	if len(o.machine.OPL) != 0 || len(o.machine.Speaker) != 0 {
		t.Fatal("觀測器掛上時仍追加序列")
	}
	if d, _ := o.machine.PITChannel2(); d != 0x1234 {
		t.Fatalf("通道 2 分頻 %#x", d)
	}
	// 回合外的 Step 不轉交，也不追加（AdLib 開啟時掛空收集器）。
	n := len(obs.opl)
	for i := 0; i < audioLoopSteps; i++ {
		if err := o.machine.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if len(obs.opl) != n || len(o.machine.OPL) != 0 || len(o.machine.Speaker) != 0 {
		t.Fatal("回合外仍轉交或追加")
	}
}

func TestAudioObserverDoesNotChangeState(t *testing.T) {
	with := adlibOwner(t, &audioRecorder{})
	plain := adlibOwner(t, &recordingObserver{})
	bare := adlibOwner(t, nil)
	for i := 0; i < 4; i++ {
		for _, o := range []*Owner{with, plain, bare} {
			if _, err := runTurn(t, o, 23); err != nil {
				t.Fatal(err)
			}
		}
	}
	a, _ := with.Digest()
	b, _ := plain.Digest()
	c, _ := bare.Digest()
	if a != b || a != c || !a.AdLib {
		t.Fatalf("摘要不同或 AdLib 未反映：%v %v %v", a == b, a == c, a.AdLib)
	}
	for _, o := range []*Owner{plain, bare} {
		if len(o.machine.OPL) != 0 || len(o.machine.Speaker) != 0 {
			t.Fatal("AdLib 開啟卻仍追加序列")
		}
	}
}

func TestDigestAdLibOffByDefault(t *testing.T) {
	o := startSyntheticOwner(t, audioLoopCOM)
	d, err := o.Digest()
	if err != nil || d.AdLib {
		t.Fatalf("%+v %v", d, err)
	}
}

func TestAudioObserverPanicIsObserverFault(t *testing.T) {
	o := adlibOwner(t, &audioRecorder{panicAt: 1})
	r, err := runTurn(t, o, 40)
	// 第 6 步（out dx,al 寫 389h）panic；該步完成，第 7 步前停止。
	assertObserverFault(t, o, r, err, 6)
}
