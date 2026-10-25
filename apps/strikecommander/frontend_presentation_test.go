package strikecommander

import (
	"crypto/sha256"
	"testing"
)

func TestFrontendPreparationNoImageRawSafetyAndCallerBuffer(t *testing.T) {
	p := &Presenter{}
	f, e := NewFrontendPresentation(p, [32]byte{1})
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	frame := make([]byte, 64000)
	frame[0] = 7
	var pal [256][3]byte
	pal[7] = [3]byte{9, 19, 29}
	f.onVerdict(FrameVerdict{Class: VerdictNoImage, FrameComplete: true}, frame, pal)
	req := FrontendPrepareRequest{Scale: 2, Language: "en", SettingsVersion: 1}
	if _, ok := f.Compose(req); ok {
		t.Fatal("unprepared returned pixels")
	}
	p.frontend.prepared.enterCallback()
	if _, e = f.Prepare(req); e == nil {
		t.Fatal("callback prepared")
	}
	p.frontend.prepared.leaveCallback()
	ready, e := f.Prepare(req)
	if e != nil {
		t.Fatal(e)
	}
	info, e := ready.Info()
	if e != nil || info.Kind != FrontendSafetyFallback {
		t.Fatal(info, e)
	}
	dst := make([]byte, 640*400*4)
	if e = ready.CopyRGBA(dst); e != nil {
		t.Fatal(e)
	}
	if dst[0] != 9 || dst[1] != 19 || dst[2] != 29 || dst[3] != 255 {
		t.Fatal("raw source failed")
	}
	if sha256.Sum256(dst) != info.SHA256 {
		t.Fatal("hash differs")
	}
	ready.Release()
	if !f.IsCurrent(req, info) {
		t.Fatal("window must retain same prepared key")
	}
	if _, ok := f.Compose(req); ok {
		t.Fatal("consumed engine RGBA retained as third window copy")
	}
	req.Scale = 3
	if f.IsCurrent(req, info) {
		t.Fatal("scale changed without prepare")
	}
}
func TestFrontendEnglishUsesImmutablePresentation(t *testing.T) {
	idx := make([]byte, 64000)
	idx[0] = 7
	var pal [256][3]byte
	pal[7] = [3]byte{17, 23, 41}
	p := &Presenter{cur: &presentImage{info: PresentInfo{Seq: 1, CaptureSteps: 100}, indexed: idx, palette: pal}}
	f, e := NewFrontendPresentation(p, [32]byte{1})
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	req := FrontendPrepareRequest{Scale: 3, Language: "en", HD: true, SettingsVersion: 1}
	q, e := f.Prepare(req)
	if e != nil {
		t.Fatal(e)
	}
	info, _ := q.Info()
	if info.Kind != FrontendFixedFallback {
		t.Fatal("missing3b source silently claimed HD")
	}
	dst := make([]byte, 960*600*4)
	q.CopyRGBA(dst)
	q.Release()
	if dst[0] != 17 || dst[1] != 23 || dst[2] != 41 {
		t.Fatal("english did not use frozen indexed/palette")
	}
	// Neither physical Frame time nor a new overlay state is consulted by an
	// unchanged current presentation; a genuine safety change invalidates it.
	f.onVerdict(FrameVerdict{Seq: 1, Class: VerdictEligible, FrameComplete: true}, idx, pal)
	if f.IsCurrent(req, info) {
		t.Fatal("new FrameComplete safety identity ignored")
	}
}

func TestFrontendPrepareDoesNotPublishSupersededCapture(t *testing.T) {
	p := &Presenter{cur: &presentImage{info: PresentInfo{Seq: 1}, indexed: make([]byte, 64000)}}
	f, e := NewFrontendPresentation(p, [32]byte{1})
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	f.composeHD = func(req FrontendPrepareRequest, dst []byte) (hdPreparedKind, string, error) {
		p.cur = &presentImage{info: PresentInfo{Seq: 2}, indexed: make([]byte, 64000)}
		f.safety.capture(2, true)
		f.invalidate()
		return hdReady, "", nil
	}
	req := FrontendPrepareRequest{Scale: 2, Language: "en", HD: true, SettingsVersion: 1}
	if q, e := f.Prepare(req); e == nil || q != nil {
		t.Fatal("superseded completed image was published")
	}
	if _, ok := f.Compose(req); ok {
		t.Fatal("superseded buffer remained current")
	}
}
