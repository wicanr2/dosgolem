package buckrogers

// StepCursor lets a caller hand the live runtime one long-lived StepReader
// while swapping the per-step view underneath.  Converting a fresh value view
// to the StepReader interface on every instruction allocates; converting this
// one pointer does not.  A runtime that retains the cursor past the callback
// reads the stale inner view, whose own validity check reports the misuse.
type StepCursor[T StepReader] struct{ cur T }

func (c *StepCursor[T]) Set(v T)                { c.cur = v }
func (c *StepCursor[T]) Steps() uint64          { return c.cur.Steps() }
func (c *StepCursor[T]) CS() uint16             { return c.cur.CS() }
func (c *StepCursor[T]) IP() uint16             { return c.cur.IP() }
func (c *StepCursor[T]) SS() uint16             { return c.cur.SS() }
func (c *StepCursor[T]) SP() uint16             { return c.cur.SP() }
func (c *StepCursor[T]) ES() uint16             { return c.cur.ES() }
func (c *StepCursor[T]) DI() uint16             { return c.cur.DI() }
func (c *StepCursor[T]) CX() uint16             { return c.cur.CX() }
func (c *StepCursor[T]) Read8(a uint32) uint8   { return c.cur.Read8(a) }
func (c *StepCursor[T]) Read16(a uint32) uint16 { return c.cur.Read16(a) }
func (c *StepCursor[T]) Palette() [256][3]uint8 { return c.cur.Palette() }
