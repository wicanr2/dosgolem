package shadesource

// CaptureRead preserves actual ReadBlock request boundaries and failed destination
// buffers. It never interprets failed bytes as an authenticated memory prefix.
func CaptureRead(read func(uint32, []byte) bool, start Far, n uint32) *Read {
	r := &Read{Start: start, Requested: n, State: Complete, Spans: []Span{}}
	off := start.Off
	left := n
	// Requested is fixed by the caller to 4,5,256,4096 or a bounded proof length.
	if n == 0 || n > 4096 {
		r.State = NotAttempted
		return r
	}
	buf := make([]byte, n)
	pos := 0
	for left > 0 {
		k := min(left, uint32(0x10000)-uint32(off))
		dst := buf[pos : pos+int(k)]
		ok := read(uint32(start.Seg)*16+uint32(off), dst)
		r.Spans = append(r.Spans, Span{Off: off, Requested: k, OK: ok, Data: dst})
		if !ok {
			r.State = Failed
			return r
		}
		left -= k
		pos += int(k)
		off = 0
	}
	return r
}
func readBytes(r *Read) []byte {
	if r == nil || r.State != Complete {
		return nil
	}
	if len(r.Spans) == 1 {
		return r.Spans[0].Data
	}
	b := make([]byte, 0, r.Requested)
	for _, s := range r.Spans {
		b = append(b, s.Data...)
	}
	return b
}
func ReadBytes(r *Read) []byte { return append([]byte(nil), readBytes(r)...) }
func CheckRead(r *Read, start Far, n uint32) Reason {
	if r == nil {
		return ReadFailed
	}
	if r.Start != start || r.Requested != n || n == 0 || n > 4096 {
		return SpanMismatch
	}
	if r.State == Budget {
		if len(r.Spans) != 0 {
			return SpanMismatch
		}
		return ResourceLimit
	}
	if r.State == NotAttempted || r.State == NotApplicable {
		if len(r.Spans) != 0 {
			return SpanMismatch
		}
		return ReadFailed
	}
	if r.State != Complete && r.State != Failed {
		return SpanMismatch
	}
	off := start.Off
	left := n
	for i, s := range r.Spans {
		k := min(left, uint32(0x10000)-uint32(off))
		if left == 0 || s.Off != off || s.Requested != k || len(s.Data) != int(k) {
			return SpanMismatch
		}
		if !s.OK {
			if i != len(r.Spans)-1 || r.State != Failed {
				return SpanMismatch
			}
			return ReadFailed
		}
		left -= k
		off = 0
	}
	if left != 0 || r.State != Complete {
		return SpanMismatch
	}
	return None
}
