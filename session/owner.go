// Package session owns one sealed DOS session lifecycle.
//
// It intentionally exposes phase and fault values only.  The machine, DOS,
// panel, and bridges stay private so an open frontend configuration cannot
// rebind one half of a turn to another target.
package session

import (
	"errors"
	"fmt"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/presentation"
)

// Phase is the monotonic lifecycle of one sealed session.
type Phase uint8

const (
	PhaseBooting Phase = iota + 1
	PhaseRunning
	PhaseStopped
	PhaseFailed
	PhaseClosed
)

// Config contains presentation-only construction choices.  In particular it
// does not accept an existing machine, DOS, panel, or bridge.
type Config struct {
	InitialScale host.OutputScale
}

// Status is a value-only lifecycle receipt.  It deliberately exposes neither
// an owned resource nor a mutable routing capability.
type Status struct {
	Phase      Phase
	FirstFault error
	CloseError error
}

// Owner privately owns the resources which must share a session lifetime.
// All methods are for the one frontend goroutine named by spec 019.
type Owner struct {
	machine  *machine.Machine
	dos      *dos.DOS
	panel    *host.PanelController
	mouse    *host.MouseBridge
	keyboard *presentation.KeyboardBridge
	closer   resourceCloser

	phase      Phase
	firstFault error
	closeErr   error
	closed     bool
}

// New creates a new sealed, synthetic-ready owner.  It neither loads an
// original executable nor accepts one; later boot work must remain within this
// owner rather than exposing these resources.
func New(cfg Config) (*Owner, error) {
	panel, err := host.NewPanelController(cfg.InitialScale)
	if err != nil {
		return nil, err
	}
	m := machine.New()
	d := dos.New(m, "")
	// DOS.Install must follow LoadEXE.  This narrow owner has no original
	// boot yet, so Booting deliberately does not install DOS or permit steps.
	// A future private boot must LoadEXE, then Install, then enter Running.
	keepDOS := false
	defer func() {
		if !keepDOS {
			d.Close()
		}
	}()
	mouse, err := host.NewMouseBridge(d)
	if err != nil {
		return nil, err
	}
	keyboard, err := presentation.NewKeyboardBridgeWithBIOS(panel, m, d)
	if err != nil {
		return nil, err
	}
	o, err := newOwner(m, d, panel, mouse, keyboard, dosCloser{dos: d})
	if err != nil {
		return nil, err
	}
	keepDOS = true
	return o, nil
}

// ReportDrawFault synchronously latches the first inspectable Draw fault.
// It is intentionally independent of any later Update call.
func (o *Owner) ReportDrawFault(err error) error {
	if o == nil {
		return fmt.Errorf("session: Owner 不得為 nil")
	}
	if err == nil {
		err = errors.New("session: ReportDrawFault 需要非 nil error")
	}
	return o.fail(err)
}

// Close performs the owned resource close at most once.  A close after a
// fault preserves that first fault; a normal close moves directly to Closed.
func (o *Owner) Close() error {
	if o == nil {
		return fmt.Errorf("session: Owner 不得為 nil")
	}
	if !o.closed {
		o.closed = true
		if o.closer == nil {
			o.closeErr = errors.New("session: 缺少私有資源關閉器")
		} else {
			o.closeErr = o.closer.Close()
		}
		if o.phase != PhaseFailed {
			o.phase = PhaseClosed
		}
	}
	return o.closeErr
}

// Status returns immutable lifecycle state; it cannot be used to reach an
// owned machine, DOS, panel, or bridge.
func (o *Owner) Status() Status {
	if o == nil {
		return Status{}
	}
	return Status{Phase: o.phase, FirstFault: o.firstFault, CloseError: o.closeErr}
}

func (o *Owner) fail(err error) error {
	if o.phase == PhaseFailed {
		return errors.Join(o.firstFault, o.closeErr)
	}
	if o.phase == PhaseClosed {
		return errors.Join(err, o.closeErr)
	}
	o.firstFault = err
	o.phase = PhaseFailed
	return errors.Join(err, o.Close())
}

type resourceCloser interface {
	Close() error
}

type dosCloser struct{ dos *dos.DOS }

func (c dosCloser) Close() error {
	if c.dos == nil {
		return errors.New("session: 私有 DOS 不得為 nil")
	}
	c.dos.Close()
	return nil
}

func newOwner(m *machine.Machine, d *dos.DOS, panel *host.PanelController, mouse *host.MouseBridge, keyboard *presentation.KeyboardBridge, closer resourceCloser) (*Owner, error) {
	if m == nil || d == nil || panel == nil || mouse == nil || keyboard == nil || closer == nil {
		return nil, errors.New("session: 封閉 owner 的私有資源不得為 nil")
	}
	return &Owner{machine: m, dos: d, panel: panel, mouse: mouse, keyboard: keyboard, closer: closer, phase: PhaseBooting}, nil
}
