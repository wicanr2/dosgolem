package phantasie

import "bytes"

// isScrollRow recognizes only the measured full-page row consumer.
// Image-relative return 99F0; DGROUP format C21D; the argument is in the
// caller's stack buffer. Keep captured text and video writes unmodified.
func isScrollRow(r *EventRecord) bool {
	if r.Overlay != "ov1" || r.Caller != 0x99F0 || r.FmtPtr != 0xC21D ||
		r.FmtKind != KindStatic || r.Format != "%s" || r.Col != 0 ||
		r.Row < 2 || r.Row > 21 || r.Composed != nil || len(r.ArgStrs) != 1 ||
		len(r.Text) < 1 || len(r.Text) > 39 || !printable(r.Text) ||
		!bytes.Equal(r.Cells, []byte(r.Text)) || r.BP == 0 {
		return false
	}
	a := r.ArgStrs[0]
	ptr := int(r.BP) + 12 + 40*(r.Row-2)
	return ptr <= 0xFFFF && int(a.Ptr) == ptr && a.Ptr == r.Args[0] &&
		a.Kind == KindBuffer && a.Piece == nil && a.Content == r.Text
}

// displayCells is the approved visual span, distinct from original writes.
func displayCells(r *EventRecord) int {
	if isScrollRow(r) {
		return 40
	}
	return len(r.Text)
}

// maskCells adds virtual blank cells only in the measured scroll-page tail.
// The returned buffer never aliases or changes captured Cells.
func maskCells(r *EventRecord) []byte {
	if !isScrollRow(r) {
		return r.Cells
	}
	b := bytes.Repeat([]byte{' '}, 40)
	copy(b, r.Cells)
	return b
}
