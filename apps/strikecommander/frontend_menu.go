package strikecommander

import (
	"bytes"
	"fmt"
	"unsafe"
)

// FrontendMenuConfig selects the limited 016a profile. Zero limits use the
// fixed profile limits; callers may lower either limit, never raise it.
type FrontendMenuConfig struct {
	RetainedBytes, WorkspaceBytes uint64
}

type FrontendMenuBoot uint8

const (
	FrontendMenuColdOriginal FrontendMenuBoot = iota + 1
	FrontendMenuFromState
)

// FrontendMenuProgram is supplied only after the cmd's unique physical MZ
// comparison. Neither a PSP number nor a load count is a program identity.
type FrontendMenuProgram uint8

const (
	FrontendMenuSCCD FrontendMenuProgram = iota + 1
	FrontendMenuMENU
	FrontendMenuOther // one of the four verified non-SCCD/MENU candidates
	FrontendMenuStopped
)

type FrontendMenuBootstrap struct {
	Boot    FrontendMenuBoot
	Program FrontendMenuProgram
}

const (
	frontendMenuRetainedLimit  = 64 << 10
	frontendMenuWorkspaceLimit = 256 << 10
	menuButtonW                = 166
	menuButtonH                = 26
	menuButtonPoints           = menuButtonW * menuButtonH
	menuBlankW                 = 165
	menuBlankPoints            = menuBlankW * menuButtonH
)

type menuBindingValue uint8

const (
	menuUnbound menuBindingValue = iota
	menuNoCover
	menuCoverK1
)

type menuBinding struct {
	key   uint64
	value menuBindingValue
}

type menuCursorPoint struct {
	dx, dy int8
	color  byte
}

// All retained R8 data lives inline in this one owner. No Frame, image, palette,
// archive, or lease reference is retained. expected is reused for each of the
// at most 112 exact-cursor candidates.
type frontendMenuPolicy struct {
	reference                                                 [2][menuButtonPoints]byte
	blank                                                     [menuBlankPoints]byte
	cursor                                                    [56]menuCursorPoint
	expected                                                  [menuButtonPoints]byte
	image                                                     menuBinding
	frame                                                     menuBinding
	program                                                   FrontendMenuProgram
	ready, touched, period, visible, entered, stopped, closed bool
	retained, workspacePeak                                   uint64
}

// menu is the last field of FrontendPresentation. Its old final field is a
// function value with pointer alignment, so this includes the new pointer and
// any tail padding. The checker also compiles the old header for comparison.
func frontendMenuHeaderBytes() uint64 {
	return uint64(unsafe.Sizeof(FrontendPresentation{}) - unsafe.Offsetof(FrontendPresentation{}.menu))
}

func (r *FrontendRuntime) InitializeMenu(b FrontendMenuBootstrap) error {
	if r == nil || r.closed || r.output == nil || r.output.menu == nil {
		return fmt.Errorf("016a: menu policy not configured or closed")
	}
	return r.output.menu.initialize(b)
}

func (m *frontendMenuPolicy) initialize(b FrontendMenuBootstrap) error {
	if m.closed || m.ready || m.touched {
		return fmt.Errorf("016a: menu bootstrap repeated, late or closed")
	}
	if b.Boot == FrontendMenuColdOriginal && b.Program == FrontendMenuSCCD {
		m.period = true
	} else if b.Boot != FrontendMenuFromState || b.Program != FrontendMenuOther {
		return fmt.Errorf("016a: unsupported menu bootstrap")
	}
	m.program, m.ready = b.Program, true
	return nil
}

func (r *FrontendRuntime) ObserveMenuProgram(p FrontendMenuProgram) error {
	if r == nil || r.closed || r.output == nil || r.output.menu == nil {
		return fmt.Errorf("016a: menu policy not configured or closed")
	}
	return r.output.menu.observeProgram(p)
}

func (m *frontendMenuPolicy) observeProgram(p FrontendMenuProgram) error {
	m.touched = true
	if !m.ready || m.closed || m.stopped || p < FrontendMenuSCCD || p > FrontendMenuStopped {
		return fmt.Errorf("016a: invalid menu program observation")
	}
	if p == FrontendMenuStopped {
		m.program, m.period, m.visible, m.stopped = p, false, false, true
		return nil
	}
	if m.period {
		if !m.entered {
			if p == FrontendMenuOther {
				return fmt.Errorf("016a: cold program chain skipped MENU")
			}
			m.entered = p == FrontendMenuMENU
		} else if p != FrontendMenuMENU {
			m.period, m.visible = false, false
		}
	} else if p == FrontendMenuMENU {
		return fmt.Errorf("016a: unsupported MENU re-entry")
	}
	m.program = p
	return nil
}

func (r *FrontendRuntime) MenuState() (period, visible bool) {
	if r != nil && !r.closed && r.output != nil && r.output.menu != nil {
		m := r.output.menu
		if m.ready && !m.closed {
			return m.period, m.visible
		}
	}
	return false, false
}

func (m *frontendMenuPolicy) observe() error {
	m.touched = true
	if !m.ready || m.closed || m.stopped {
		return fmt.Errorf("016a: observation requires initialized menu policy")
	}
	return nil
}

func (m *frontendMenuPolicy) art(ev ArtEvent) {
	m.touched = true
	if m.ready && !m.closed && m.period && m.program == FrontendMenuMENU && ev.File == ArtTomenuButtons && (ev.K == 0 || ev.K == 1) {
		m.visible = true
	}
}

func (m *frontendMenuPolicy) modeChange() { m.touched, m.visible = true, false }

func (m *frontendMenuPolicy) bindImage(img *presentImage) {
	m.touched = true
	m.image = menuBinding{key: img.info.Seq, value: m.binding(img.indexed)}
}

func (m *frontendMenuPolicy) complete(key uint64, indexed []byte) {
	m.touched = true
	v := m.binding(indexed)
	m.frame = menuBinding{key: key, value: v}
	if m.visible && v != menuCoverK1 {
		m.visible = false
	}
}

func (m *frontendMenuPolicy) binding(indexed []byte) menuBindingValue {
	if m.match(indexed).matched {
		return menuCoverK1
	}
	return menuNoCover
}

type menuMatch struct {
	matched, cursor bool
	reference       uint8
	x, y            int16
	candidates      uint16
}

func menuEqual(indexed []byte, reference []byte) bool {
	for y := 0; y < menuButtonH; y++ {
		at := (33+y)*320 + 89
		if !bytes.Equal(indexed[at:at+menuButtonW], reference[y*menuButtonW:(y+1)*menuButtonW]) {
			return false
		}
	}
	return true
}

func (m *frontendMenuPolicy) match(indexed []byte) menuMatch {
	if len(indexed) != 64000 {
		return menuMatch{}
	}
	for k := range m.reference {
		if menuEqual(indexed, m.reference[k][:]) {
			return menuMatch{matched: true, reference: uint8(k * 2)}
		}
	}
	var result menuMatch
	for k := range m.reference {
		reference := m.reference[k][:]
		first := -1
		for i, c := range reference {
			if indexed[(33+i/menuButtonW)*320+89+i%menuButtonW] != c {
				first = i
				break
			}
		}
		if first < 0 {
			continue
		}
		gx, gy := 89+first%menuButtonW, 33+first/menuButtonW
		for _, point := range m.cursor {
			cx, cy := gx-int(point.dx), gy-int(point.dy)
			if cx < 0 || cx >= 320 || cy < 0 || cy >= 200 {
				continue
			}
			result.candidates++
			copy(m.expected[:], reference)
			for _, p := range m.cursor {
				x, y := cx+int(p.dx)-89, cy+int(p.dy)-33
				if x >= 0 && x < menuButtonW && y >= 0 && y < menuButtonH {
					m.expected[y*menuButtonW+x] = p.color
				}
			}
			if menuEqual(indexed, m.expected[:]) {
				result.matched, result.cursor, result.reference = true, true, uint8(k*2)
				result.x, result.y = int16(cx), int16(cy)
				return result
			}
		}
	}
	return result
}

// The selected binding and palette are frozen local values. In particular the
// finalizer never consults live period/visible or a later raw-frame palette.
type menuFinalizer struct {
	policy  *frontendMenuPolicy
	value   menuBindingValue
	palette [256][3]byte
	scale   int
}

func (f *FrontendPresentation) selectMenuFinalizer(img *presentImage, safety HDSafetyState, scale int) (menuFinalizer, error) {
	m := f.menu
	if m == nil {
		return menuFinalizer{}, nil
	}
	m.touched = true
	if !m.ready || m.closed {
		return menuFinalizer{}, fmt.Errorf("016a: Prepare requires initialized menu policy")
	}
	v := menuFinalizer{policy: m, value: menuNoCover, scale: scale}
	if safety.Eligibility == HDEligible && img != nil {
		if m.image.key != img.info.Seq || m.image.value == menuUnbound {
			return menuFinalizer{}, fmt.Errorf("016a: unbound immutable image")
		}
		v.value, v.palette = m.image.value, img.palette
	} else if f.safety.hasFrame {
		if m.frame.key != safety.FrameCompleteKey || m.frame.value == menuUnbound {
			return menuFinalizer{}, fmt.Errorf("016a: unbound complete fallback")
		}
		v.value, v.palette = m.frame.value, f.safety.palette
	}
	return v, nil
}

func (v menuFinalizer) finish(dst []byte, kind hdPreparedKind, reason string, err error) (hdPreparedKind, string, error) {
	if err != nil || v.policy == nil {
		return kind, reason, err
	}
	if v.policy.closed || (v.scale != 2 && v.scale != 3) || len(dst) != 320*200*v.scale*v.scale*4 || v.value < menuNoCover || v.value > menuCoverK1 {
		return hdNotPrepared, "", fmt.Errorf("016a: invalid finalizer source, scale or destination")
	}
	if v.value == menuCoverK1 {
		for y := 0; y < menuButtonH; y++ {
			for x := 0; x < menuBlankW; x++ {
				c := v.palette[v.policy.blank[y*menuBlankW+x]]
				for dy := 0; dy < v.scale; dy++ {
					for dx := 0; dx < v.scale; dx++ {
						at := 4 * (((71+y)*v.scale+dy)*320*v.scale + (90+x)*v.scale + dx)
						dst[at], dst[at+1], dst[at+2], dst[at+3] = c[0], c[1], c[2], 255
					}
				}
			}
		}
	}
	return kind, reason, nil
}

func (m *frontendMenuPolicy) close() {
	// The prepared owner has already waited for a running producer. RGBA leases
	// are independent and never refer to this policy or its inline assets.
	*m = frontendMenuPolicy{closed: true}
}
