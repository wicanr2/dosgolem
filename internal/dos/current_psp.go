package dos

// CurrentPSP reports the actual current process segment without a fallback.
// It is a scalar read and does not inspect guest memory (spec 464).
func (d *DOS) CurrentPSP() uint16 { return d.curPSP }
