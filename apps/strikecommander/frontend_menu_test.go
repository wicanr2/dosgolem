package strikecommander

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/wicanr2/dosgolem/internal/machine"
)

func menuReal(t *testing.T) *frontendMenuPolicy {
	t.Helper()
	root := os.Getenv("SC_R8_ROOT")
	if root == "" {
		t.Skip("explicit immutable resource root required; no guest")
	}
	m, e := newFrontendMenu(root, FrontendMenuConfig{})
	if e != nil {
		t.Fatal(e)
	}
	return m
}

func TestFrontendMenuRealPreload(t *testing.T) {
	m := menuReal(t)
	root := os.Getenv("SC_R8_ROOT")
	absPrefix := filepath.VolumeName(root) + string(os.PathSeparator)
	if m.retained != uint64(unsafe.Sizeof(*m))+frontendMenuHeaderBytes() || m.retained > frontendMenuRetainedLimit || m.workspacePeak > frontendMenuWorkspaceLimit {
		t.Fatal("preflight accounting", m.retained, m.workspacePeak)
	}
	t.Logf("R8 retained=%d workspacePeak=%d headerDelta=%d afterHeader=%d config=%d runtime=%d presentImage=%d hdPrepareKey=%d", m.retained, m.workspacePeak, frontendMenuHeaderBytes(), unsafe.Sizeof(FrontendPresentation{}), unsafe.Sizeof(FrontendRuntimeConfig{}), unsafe.Sizeof(FrontendRuntime{}), unsafe.Sizeof(presentImage{}), unsafe.Sizeof(hdPrepareKey{}))
	for _, cfg := range []FrontendMenuConfig{{RetainedBytes: m.retained - 1}, {WorkspaceBytes: m.workspacePeak - 1}, {RetainedBytes: frontendMenuRetainedLimit + 1}, {WorkspaceBytes: frontendMenuWorkspaceLimit + 1}} {
		if _, err := newFrontendMenu(absPrefix+strings.Repeat("x", len(root)-len(absPrefix)), cfg); err == nil {
			t.Fatal("budget passed")
		} else if _, isPath := err.(*os.PathError); isPath {
			t.Fatal("read occurred before budget refusal", err)
		} else if !strings.Contains(err.Error(), "allocation budget") {
			t.Fatal("low budget failed for unrelated reason", err)
		}
	}
	if _, err := newFrontendMenu(absPrefix+strings.Repeat("x", frontendMenuWorkspaceLimit), FrontendMenuConfig{}); err == nil {
		t.Fatal("unbounded root escaped workspace preflight")
	} else if _, isPath := err.(*os.PathError); isPath {
		t.Fatal("unbounded root allocated/opened path before refusal", err)
	}
	m.close()
	if m.retained != 0 || m.workspacePeak != 0 || !m.closed {
		t.Fatal("owner not retired")
	}
}

func TestFrontendMenuPlatformPathBudget(t *testing.T) {
	// Pure arithmetic on Linux; these are not Windows filesystem tests.
	for _, s := range []string{`C:\game\tree-v1\BIGTRE.TRE`, `C:\繁中\😀\BIGTRE.TRE`, `\\?\C:\` + strings.Repeat("a", 300) + `\BIGTRE.TRE`} {
		length := uint64(len(s))
		shortOrExtended := len(s) < 248 || menuWindowsExtended(s)
		got, err := menuPathWorkspace("windows", true, length, menuWindowsExtended(s))
		if err != nil {
			t.Fatal(err)
		}
		// UTF16FromString allocates capacity len(UTF8)+1, not just the
		// shorter number of actual UTF16 code units, even for CJK/astral input.
		encoded := utf16.Encode([]rune(s))
		if uint64(len(encoded)) > length {
			t.Fatal("UTF16 input bound")
		}
		if shortOrExtended && got != length+2*(length+1) {
			t.Fatal("UTF16 capacity missing", got, length)
		}
	}
	for _, length := range []uint64{247, 248, 300, 4096} {
		got, err := menuPathWorkspace("windows", true, length, false)
		if err != nil {
			t.Fatal(err)
		}
		want := length + 2*(length+1)
		if length >= 248 {
			prefixUnits := uint64(8)
			inputUTF16 := 2 * (length + 1)
			fullBuffer := 2 * (length + 1 + prefixUnits)
			returnedUTF8 := 3 * (length + 1 + prefixUnits)
			openUTF16 := 2 * (returnedUTF8 + 1)
			want = length + 2*prefixUnits + inputUTF16 + fullBuffer + returnedUTF8 + openUTF16
		}
		if got != want {
			t.Fatal("long-path conversion backing missing", length, got, want)
		}
		_, work, err := menuAllocationBytes("windows", true, length, false, FrontendMenuConfig{})
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err = menuAllocationBytes("windows", true, length, false, FrontendMenuConfig{WorkspaceBytes: work - 1}); err == nil {
			t.Fatal("Windows low-byte budget escaped pure preflight")
		}
		if _, _, err = menuAllocationBytes("windows", true, length, false, FrontendMenuConfig{WorkspaceBytes: work}); err != nil {
			t.Fatal("Windows exact budget refused", err)
		}
	}
	for _, goos := range []string{"linux", "darwin"} {
		got, err := menuPathWorkspace(goos, true, 300, false)
		if err != nil || got != 601 {
			t.Fatal("byte NUL budget", goos, got, err)
		}
	}
	for _, goos := range []string{"linux", "windows"} {
		if _, err := menuPathWorkspace(goos, false, 100, false); err == nil {
			t.Fatal("relative internal cache accepted")
		}
		if _, err := menuPathWorkspace(goos, true, ^uint64(0), false); err == nil {
			t.Fatal("path accounting overflow")
		}
	}
	for _, p := range []string{`\\?\C:\cache`, `//?/C:/cache`, `\??\C:\cache`} {
		if !menuWindowsExtended(p) {
			t.Fatal("known extended prefix missed", p)
		}
	}
	for _, p := range []string{`C:\cache`, `\\server\share\cache`, `\\.\C:\cache`} {
		if menuWindowsExtended(p) {
			t.Fatal("ordinary/device prefix mislabeled extended", p)
		}
	}
	if _, err := newFrontendMenu("relative-cache", FrontendMenuConfig{}); err == nil || !strings.Contains(err.Error(), "must be absolute") {
		t.Fatal("relative cache touched IO before internal precondition", err)
	}
}

func menuCanvas(m *frontendMenuPolicy, k int) []byte {
	frame := bytes.Repeat([]byte{37}, 64000)
	for y := 0; y < menuButtonH; y++ {
		copy(frame[(33+y)*320+89:], m.reference[k][y*menuButtonW:(y+1)*menuButtonW])
	}
	return frame
}

func menuStamp(frame []byte, cursor [56]menuCursorPoint, cx, cy int) {
	for _, p := range cursor {
		x, y := cx+int(p.dx), cy+int(p.dy)
		if x >= 0 && x < 320 && y >= 0 && y < 200 {
			frame[y*320+x] = p.color
		}
	}
}

func TestFrontendMenuWholeCursorMatcher(t *testing.T) {
	m := menuReal(t)
	for k := 0; k < 2; k++ {
		for _, center := range [][2]int{{172, 46}, {89, 46}, {254, 46}, {172, 33}, {172, 58}, {0, 0}, {319, 0}, {0, 199}, {319, 199}, {20, 100}} {
			frame := menuCanvas(m, k)
			menuStamp(frame, m.cursor, center[0], center[1])
			if result := m.match(frame); !result.matched || result.candidates > 112 {
				t.Fatal("whole cursor rejected", k, center, result)
			}
		}
	}
	base := menuCanvas(m, 0)
	full := append([]byte(nil), base...)
	menuStamp(full, m.cursor, 172, 46)
	changed := -1
	for i := range full {
		if full[i] != base[i] {
			changed = i
			break
		}
	}
	if changed < 0 {
		t.Fatal("fixture has no changed stroke")
	}
	negative := make(map[string][]byte)
	b := append([]byte(nil), full...)
	b[changed] = base[changed]
	negative["missing_stroke"] = b
	b = append([]byte(nil), full...)
	b[changed] = 253
	negative["wrong_color"] = b
	b = append([]byte(nil), full...)
	b[35*320+91] = 253
	negative["extra_point"] = b
	b = append([]byte(nil), base...)
	menuStamp(b, m.cursor, 130, 45)
	menuStamp(b, m.cursor, 215, 45)
	negative["two_cursors"] = b
	b = append([]byte(nil), base...)
	for y := 41; y < 52; y++ {
		for x := 167; x < 178; x++ {
			b[y*320+x] = 0
		}
	}
	negative["rectangle"] = b
	b = append([]byte(nil), base...)
	for y := 0; y < 26; y++ {
		copy(b[(33+y)*320+89+83:(33+y)*320+89+166], m.reference[1][y*166+83:(y+1)*166])
	}
	negative["mixed_templates"] = b
	for name, frame := range negative {
		t.Run(name, func(t *testing.T) {
			if result := m.match(frame); result.matched || result.candidates > 112 {
				t.Fatal("REVIEW whole-image matcher accepted incomplete/corrupt cursor", result)
			}
		})
	}
	if m.match(base[:63999]).matched {
		t.Fatal("short frame accepted")
	}
	if n := testing.AllocsPerRun(50, func() { _ = m.match(full) }); n != 0 {
		t.Fatal("callback matcher allocated", n)
	}
}

func TestFrontendMenuBlankMatches008AndDecoderBounds(t *testing.T) {
	m := menuReal(t)
	w := new(frontendMenuWorkspace)
	if err := w.load(os.Getenv("SC_R8_ROOT"), new(frontendMenuPolicy)); err != nil {
		t.Fatal(err)
	}
	entry, err := decodeEntry(w.buttons[:], 1)
	if err != nil {
		t.Fatal(err)
	}
	sp, _ := artSpecFor(ArtEvent{File: ArtTomenuButtons, K: 1})
	band, _ := tomenuBackground(entry.Pix, sp)
	want := append([]byte(nil), entry.Pix...)
	bw := sp.band.x1 - sp.band.x0 + 1
	for y := sp.band.y0; y <= sp.band.y1; y++ {
		copy(want[y*165+sp.band.x0:y*165+sp.band.x1+1], band[(y-sp.band.y0)*bw:(y-sp.band.y0+1)*bw])
	}
	if !bytes.Equal(want, m.blank[:]) {
		t.Fatal("008 whole blank k1 differs")
	}
	for _, raw := range [][]byte{nil, {1, 2, 3, 4}, w.buttons[:12]} {
		if _, err := menuPAKEntry(raw, 4, 1, 0); err == nil {
			t.Fatal("invalid PAK accepted")
		}
	}
	broken := *w
	broken.mouse[609] ^= 1
	if err := broken.parse(new(frontendMenuPolicy)); err == nil {
		t.Fatal("cursor identity corruption accepted")
	}
	broken = *w
	broken.tomenu[12290] ^= 1
	if err := broken.parse(new(frontendMenuPolicy)); err == nil {
		t.Fatal("TOMENU corruption accepted")
	}
	if err := menuDecodeShape([]byte{0, 0, 0, 0, 0, 0, 0, 0, 3, 0, 0, 0, 0, 0, 0}, 1, 1, make([]byte, 1), make([]bool, 1)); err == nil {
		t.Fatal("zero-length subrun accepted")
	}
	if err := w.lzw([]byte{0, 0}, false); err == nil {
		t.Fatal("truncated LZW accepted")
	}
}

func TestFrontendMenuRecordedFramesAndPartialHold(t *testing.T) {
	project := os.Getenv("SC_R8_PROJECT")
	if project == "" {
		t.Skip("explicit fixed frame receipts required")
	}
	m := menuReal(t)
	if err := m.initialize(FrontendMenuBootstrap{FrontendMenuColdOriginal, FrontendMenuSCCD}); err != nil {
		t.Fatal(err)
	}
	if err := m.observeProgram(FrontendMenuMENU); err != nil {
		t.Fatal(err)
	}
	f, err := NewFrontendPresentation(&Presenter{}, [32]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	f.menu = m
	defer f.Close()
	count, matched, partial := 0, 0, 0
	for _, label := range []string{"001", "002"} {
		var analysis struct {
			Rows []struct {
				Frame, IndexedSHA256 string
				Full4316Match        bool
				Verdict              FrameVerdict
			}
		}
		p := filepath.Join(project, "workplace/impl016/logs/menu-frame-analysis-"+label+".json")
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(raw, &analysis); err != nil {
			t.Fatal(err)
		}
		for _, row := range analysis.Rows {
			frame, err := os.ReadFile(filepath.Join(project, "workplace/impl016/logs/menu-frame-probe-"+label, row.Frame))
			if err != nil {
				t.Fatal(err)
			}
			if fmt.Sprintf("%x", sha256.Sum256(frame)) != row.IndexedSHA256 {
				t.Fatal("fixed input SHA changed", row.Frame)
			}
			if got := m.match(frame).matched; got != row.Full4316Match {
				t.Fatal("production differs from independent raw-frame witness", row.Frame, got)
			}
			m.art(ArtEvent{File: ArtTomenuButtons, K: 0})
			if !m.visible {
				t.Fatal("OnArt did not start")
			}
			before := m.frame
			f.onVerdict(row.Verdict, frame, menuPalette(7))
			if row.Verdict.FrameComplete {
				if !m.visible {
					t.Fatal("complete known cursor cleared visible")
				}
				matched++
			} else {
				partial++
				if m.frame != before || !m.visible {
					t.Fatal("partial overwrote state")
				}
			}
			count++
		}
	}
	if count != 30 || matched != 25 || partial != 5 {
		t.Fatal("incomplete fixed corpus", count, matched, partial)
	}
	t.Logf("fixed raw Frame windows %d: complete=%d partial=%d; no guest/GUI", count, matched, partial)
}

func TestFrontendMenuProgramLifecycle(t *testing.T) {
	for _, b := range []FrontendMenuBootstrap{{FrontendMenuFromState, FrontendMenuMENU}, {FrontendMenuFromState, FrontendMenuSCCD}, {FrontendMenuColdOriginal, FrontendMenuOther}, {0, 0}} {
		m := new(frontendMenuPolicy)
		if err := m.initialize(b); err == nil {
			t.Fatal("unsupported bootstrap accepted", b)
		}
	}
	m := new(frontendMenuPolicy)
	m.touched = true
	if err := m.initialize(FrontendMenuBootstrap{FrontendMenuColdOriginal, FrontendMenuSCCD}); err == nil {
		t.Fatal("late bootstrap")
	}
	for _, fromState := range []bool{false, true} {
		m := new(frontendMenuPolicy)
		b := FrontendMenuBootstrap{FrontendMenuColdOriginal, FrontendMenuSCCD}
		if fromState {
			b = FrontendMenuBootstrap{FrontendMenuFromState, FrontendMenuOther}
		}
		if err := m.initialize(b); err != nil {
			t.Fatal(err)
		}
		if err := m.initialize(b); err == nil {
			t.Fatal("repeated bootstrap")
		}
		if !fromState {
			if err := m.observeProgram(FrontendMenuSCCD); err != nil || !m.period {
				t.Fatal("SCCD prelude", err)
			}
			if err := m.observeProgram(FrontendMenuOther); err == nil {
				t.Fatal("skipped MENU chain accepted")
			}
			if err := m.observeProgram(FrontendMenuMENU); err != nil {
				t.Fatal(err)
			}
			m.art(ArtEvent{File: ArtTomenuButtons, K: 0})
			if !m.visible {
				t.Fatal("MENU did not start")
			}
			m.modeChange()
			if m.visible || !m.period {
				t.Fatal("mode change ended period")
			}
			m.art(ArtEvent{File: ArtTomenuButtons, K: 1})
			if !m.visible {
				t.Fatal("MENU next loop not started")
			}
			if err := m.observeProgram(FrontendMenuSCCD); err != nil || m.period || m.visible {
				t.Fatal("MENU return", err)
			}
		}
		for _, p := range []FrontendMenuProgram{FrontendMenuOther, FrontendMenuSCCD, FrontendMenuOther, FrontendMenuSCCD, FrontendMenuOther} {
			if err := m.observeProgram(p); err != nil || m.period || m.visible {
				t.Fatal("REVIEW later parent return opened menu period", fromState, p, err)
			}
		}
		if err := m.observeProgram(FrontendMenuMENU); err == nil {
			t.Fatal("unsupported MENU reentry")
		}
		if err := m.observeProgram(FrontendMenuStopped); err != nil || m.period || m.visible {
			t.Fatal("stopped", err)
		}
		if err := m.observe(); err == nil {
			t.Fatal("observation after stopped")
		}
	}
}

func menuPresentation(t *testing.T) (*Presenter, *FrontendPresentation, *frontendMenuPolicy) {
	t.Helper()
	p := &Presenter{cfg: PresenterConfig{}, counts: PresentCounts{Captures: make(map[PresentKind]int)}}
	f, err := NewFrontendPresentation(p, [32]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	m := menuReal(t)
	f.menu = m
	if err = m.initialize(FrontendMenuBootstrap{FrontendMenuColdOriginal, FrontendMenuSCCD}); err != nil {
		t.Fatal(err)
	}
	if err = m.observeProgram(FrontendMenuMENU); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(f.Close)
	return p, f, m
}

func menuPalette(seed byte) (p [256][3]byte) {
	for i := range p {
		p[i] = [3]byte{byte(i) ^ seed, byte(i+19) ^ seed, byte(i+71) ^ seed}
	}
	return
}

func menuPublish(p *Presenter, idx []byte, pal [256][3]byte, seq uint64) {
	p.cur = &presentImage{info: PresentInfo{Seq: seq, Kind: PresFallback}, indexed: append([]byte(nil), idx...), palette: pal}
	p.seq = seq
	p.publish(p.cur)
}

func menuPrepareBytes(t *testing.T, f *FrontendPresentation, req FrontendPrepareRequest) ([]byte, FrontendCompositeInfo) {
	t.Helper()
	c, err := f.Prepare(req)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Release()
	info, err := c.Info()
	if err != nil {
		t.Fatal(err)
	}
	dst := make([]byte, 320*200*req.Scale*req.Scale*4)
	if err = c.CopyRGBA(dst); err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(dst) != info.SHA256 {
		t.Fatal("REVIEW final menu not included in published SHA")
	}
	other, ok := f.Compose(req)
	if !ok {
		t.Fatal("missing composed lease")
	}
	defer other.Release()
	second := make([]byte, len(dst))
	if err = other.CopyRGBA(second); err != nil || !bytes.Equal(dst, second) {
		t.Fatal("immutable lease copy", err)
	}
	return dst, info
}

func assertMenuOutput(t *testing.T, got, idx []byte, pal [256][3]byte, scale int, blank []byte) {
	t.Helper()
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			c := pal[idx[y*320+x]]
			if blank != nil && x >= 90 && x < 255 && y >= 71 && y < 97 {
				c = pal[blank[(y-71)*165+x-90]]
			}
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					at := 4 * ((y*scale+dy)*320*scale + x*scale + dx)
					if got[at] != c[0] || got[at+1] != c[1] || got[at+2] != c[2] || got[at+3] != 255 {
						t.Fatalf("REVIEW source/palette/whole-button mismatch (%d,%d) got=%v want=%v", x, y, got[at:at+4], c)
					}
				}
			}
		}
	}
}

func TestFrontendMenuFrozenImageFallbackAndFinalizer(t *testing.T) {
	p, f, m := menuPresentation(t)
	a := menuCanvas(m, 0)
	palA, palB := menuPalette(2), menuPalette(153)
	menuPublish(p, a, palA, 1)
	if m.image.value != menuCoverK1 {
		t.Fatal("binding missing before public capture")
	}
	// A complete raw frame can clear live visible and update its own palette,
	// while immutable image A remains selected and must keep its own cover.
	m.visible = true
	b := bytes.Repeat([]byte{44}, 64000)
	f.onVerdict(FrameVerdict{Class: VerdictEligible, FrameComplete: true}, b, palB)
	if m.visible {
		t.Fatal("complete mismatch did not clear visible")
	}
	version := uint64(0)
	for _, scale := range []int{2, 3} {
		for _, hd := range []bool{false, true} {
			for _, subs := range []bool{false, true} {
				version++
				req := FrontendPrepareRequest{Scale: scale, Language: "en", HD: hd, Subtitles: subs, SettingsVersion: version}
				got, info := menuPrepareBytes(t, f, req)
				assertMenuOutput(t, got, a, palA, scale, m.blank[:])
				if (hd && info.Kind != FrontendFixedFallback) || (!hd && info.Kind != FrontendReady) {
					t.Fatal("kind changed", info.Kind)
				}
				if hd {
					rebuilt, again := menuPrepareBytes(t, f, req)
					if !bytes.Equal(got, rebuilt) || info != again {
						t.Fatal("REVIEW fixed rebuild skipped finalizer or changed metadata")
					}
				}
			}
		}
	}
	// Derive has a new Seq and its own palette. It binds before capture callbacks.
	p.OnCapture(func(info PresentInfo) {
		if m.image.key != info.Seq {
			t.Fatal("public capture ran before binding")
		}
	})
	p.derive(PresPalette, palB, nil, false)
	req := FrontendPrepareRequest{Scale: 2, Language: "en", SettingsVersion: 1}
	got, _ := menuPrepareBytes(t, f, req)
	assertMenuOutput(t, got, a, palB, 2, m.blank[:])
	// Stale selects B and its NoCover binding, never A's Seq or live visible.
	m.visible = true
	f.onVerdict(FrameVerdict{Class: VerdictStale, FrameComplete: false}, a, palA)
	got, info := menuPrepareBytes(t, f, req)
	assertMenuOutput(t, got, b, palB, 2, nil)
	if info.Kind != FrontendSafetyFallback {
		t.Fatal(info)
	}
	// Complete TOMENU fallback has its own key and palette, including when a
	// non-TOMENU image becomes current. Raw true is not a source-cover gate.
	f.onVerdict(FrameVerdict{Class: VerdictStale, FrameComplete: true}, a, palA)
	menuPublish(p, b, palB, 3)
	got, _ = menuPrepareBytes(t, f, req)
	assertMenuOutput(t, got, b, palB, 2, nil)
	f.onVerdict(FrameVerdict{Class: VerdictStale, FrameComplete: false}, b, palB)
	got, _ = menuPrepareBytes(t, f, req)
	assertMenuOutput(t, got, a, palA, 2, m.blank[:])
	// Missing or mismatched source bindings are wiring errors, not NoCover.
	m.frame.value = menuUnbound
	req.SettingsVersion++
	if c, err := f.Prepare(req); err == nil || c != nil {
		t.Fatal("REVIEW unbound fallback accepted")
	}
	f.safety.set(HDEligible)
	f.invalidate()
	m.image.key = 999
	req.SettingsVersion++
	if c, err := f.Prepare(req); err == nil || c != nil {
		t.Fatal("REVIEW wrong image key accepted")
	}
}

func TestFrontendMenuLateBootstrapHDGuardAndCloseLease(t *testing.T) {
	cfg := runtimeTestConfig(t)
	cfg.CacheRoot = "/must-not-read"
	cfg.Menu, cfg.HD = &FrontendMenuConfig{}, &FrontendHDConfig{}
	var pathError *os.PathError
	if _, err := NewFrontendRuntime(cfg); err == nil || errors.As(err, &pathError) {
		t.Fatal("joint constructor accepted")
	}
	p, f, m := menuPresentation(t)
	r := &FrontendRuntime{output: f, present: p, so: &SessionObserver{}}
	before := f.prepared
	epoch := f.sourceEpoch
	handler := fmt.Sprintf("%p", f.composeHD)
	if err := r.ConfigureHD(FrontendHDConfig{GameRoot: "/must-not-read", StateSHA: [32]byte{1}, AppGeneration: 1}); err == nil || errors.As(err, &pathError) {
		t.Fatal("joint ConfigureHD accepted")
	}
	if r.hd != nil || f.prepared != before || f.sourceEpoch != epoch || fmt.Sprintf("%p", f.composeHD) != handler {
		t.Fatal("joint refusal changed state")
	}
	idx := menuCanvas(m, 0)
	pal := menuPalette(2)
	menuPublish(p, idx, pal, 1)
	req := FrontendPrepareRequest{Scale: 2, Language: "en", SettingsVersion: 1}
	lease, err := f.Prepare(req)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if f.menu != nil || !m.closed || m.retained != 0 || m.frame != (menuBinding{}) || m.image != (menuBinding{}) {
		t.Fatal("R8 held owner after close")
	}
	got := make([]byte, 320*200*4*4)
	if err = lease.CopyRGBA(got); err != nil {
		t.Fatal("old lease depended on closed menu owner", err)
	}
	// The immutable published RGBA retains the full cover after owner retirement.
	if got[4*(71*2*640+90*2)+3] != 255 {
		t.Fatal("closed lease lost alpha")
	}
	lease.Release()
	if err = r.InitializeMenu(FrontendMenuBootstrap{FrontendMenuColdOriginal, FrontendMenuSCCD}); err == nil {
		t.Fatal("initialize closed policy")
	}
	// A failed Prepare before initialize still makes a later bootstrap too late.
	p2 := &Presenter{}
	f2, err := NewFrontendPresentation(p2, [32]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	defer f2.Close()
	f2.menu = menuReal(t)
	if _, err = f2.Prepare(req); err == nil {
		t.Fatal("uninitialized Prepare")
	}
	if err = f2.menu.initialize(FrontendMenuBootstrap{FrontendMenuColdOriginal, FrontendMenuSCCD}); err == nil {
		t.Fatal("late initialize after Prepare")
	}
	// Close must wait for the active prepared producer before zeroing its owner.
	p3, f3, m3 := menuPresentation(t)
	menuPublish(p3, menuCanvas(m3, 0), pal, 1)
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	f3.composeHD = func(_ FrontendPrepareRequest, dst []byte) (hdPreparedKind, string, error) {
		close(entered)
		<-release
		scaleFrontendInto(dst, p3.cur.indexed, p3.cur.palette, 2)
		return hdReady, "", nil
	}
	req.HD = true
	go func() {
		c, _ := f3.Prepare(req)
		if c != nil {
			c.Release()
		}
		close(done)
	}()
	<-entered
	closed := make(chan struct{})
	go func() { f3.Close(); close(closed) }()
	var releaseOnce sync.Once
	defer func() {
		releaseOnce.Do(func() { close(release) })
		<-done
		<-closed
	}()
	select {
	case <-closed:
		t.Fatal("REVIEW Close retired policy before producer finished")
	case <-time.After(time.Millisecond * 10):
	}
	if m3.closed || m3.retained == 0 {
		t.Fatal("REVIEW ownership deducted while prepare active")
	}
	releaseOnce.Do(func() { close(release) })
	<-done
	<-closed
	if !m3.closed || m3.retained != 0 {
		t.Fatal("owner not released after prepare")
	}
}

func TestFrontendMenuActualRuntimeEventAndChineseFinalOrder(t *testing.T) {
	assets, fontRoot := os.Getenv("SC_FRONTEND_RESOURCE_ROOT"), os.Getenv("SC_FRONTEND_FONT_ROOT")
	if assets == "" || fontRoot == "" {
		t.Skip("explicit immutable text/font assets required")
	}
	cfg := runtimeTestConfig(t)
	cfg.CacheRoot = os.Getenv("SC_R8_ROOT")
	cfg.Menu = &FrontendMenuConfig{}
	var err error
	cfg.Fonts, err = LoadTextFonts(fontRoot, filepath.Join(assets, "font/zh-TW.cjkb.expected.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.GfxCatalog, err = LoadCatalog(filepath.Join(assets, "text/menu_gfx.zh-TW.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewFrontendRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err = r.InitializeMenu(FrontendMenuBootstrap{FrontendMenuColdOriginal, FrontendMenuSCCD}); err != nil {
		t.Fatal(err)
	}
	if err = r.ObserveMenuProgram(FrontendMenuMENU); err != nil {
		t.Fatal(err)
	}
	m := r.output.menu
	w := new(frontendMenuWorkspace)
	if err = w.load(cfg.CacheRoot, new(frontendMenuPolicy)); err != nil {
		t.Fatal(err)
	}
	frame := menuCanvas(m, 0)
	for y := 0; y < 26; y++ {
		copy(frame[(71+y)*320+90:(71+y)*320+255], w.k1[y*165:(y+1)*165])
	}
	for _, ev := range []ArtEvent{{File: ArtTomenuButtons, K: 0, X: 89, Y: 33, W: 166, H: 26, Entry: m.reference[0][:], Base0: m.reference[0][:]}, {File: ArtTomenuButtons, K: 1, X: 90, Y: 71, W: 165, H: 26, Entry: w.k1[:], Base0: w.k1[:]}} {
		r.so.Labels.OnArt(ev)
	}
	if period, visible := r.MenuState(); !period || !visible || r.present.stateSinceCap != 2 {
		t.Fatal("chained OnArt did not reach presenter and policy", period, visible, r.present.stateSinceCap)
	}
	r.present.applyQueue()
	pal := menuPalette(11)
	menuPublish(r.present, frame, pal, 1)
	version := uint64(0)
	for _, scale := range []int{2, 3} {
		for _, lang := range []string{"en", "zh-TW"} {
			for _, hd := range []bool{false, true} {
				version++
				req := FrontendPrepareRequest{Scale: scale, Language: lang, HD: hd, Subtitles: true, SettingsVersion: version}
				baseline := make([]byte, 320*200*scale*scale*4)
				if lang == "en" {
					scaleFrontendInto(baseline, frame, pal, scale)
				} else {
					r.present.composeImageInto(r.present.cur, scale, true, baseline)
				}
				got, _ := menuPrepareBytes(t, r.output, req)
				for y := 0; y < 200*scale; y++ {
					for x := 0; x < 320*scale; x++ {
						at := 4 * (y*320*scale + x)
						if x >= 90*scale && x < 255*scale && y >= 71*scale && y < 97*scale {
							c := pal[m.blank[(y/scale-71)*165+x/scale-90]]
							if got[at] != c[0] || got[at+1] != c[1] || got[at+2] != c[2] || got[at+3] != 255 {
								t.Fatal("REVIEW final k1 cover occurred before Chinese/art/subtitle", scale, lang, hd, x, y)
							}
						} else if !bytes.Equal(got[at:at+4], baseline[at:at+4]) {
							t.Fatal("R8 changed outside approved full k1", scale, lang, hd, x, y)
						}
					}
				}
			}
		}
	}
	// Actual Runtime path forwards mode cleanup to both the original presenter
	// and R8, and retains the MENU period for the next loop.
	r.VideoModeChange(machine.ModeChange{Mode: 0x13, Step: 1})
	if period, visible := r.MenuState(); !period || visible || r.present.counts.ModeChanges != 1 {
		t.Fatal("mode cleanup not chained")
	}
	if err = r.ObserveMenuProgram(FrontendMenuStopped); err != nil {
		t.Fatal(err)
	}
	if period, visible := r.MenuState(); period || visible {
		t.Fatal("stopped input remains menu-clamped")
	}
}
