package shadesource

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"reflect"
	"sort"
	"strconv"
)

// CanonicalSourceV1 uses only the closed 013b Go record types; it is not a public
// arbitrary-value encoder. Byte counts are checked before any encoding buffer.
type canonicalWriter struct {
	w   io.Writer
	n   uint64
	err error
}

func (w *canonicalWriter) put(b []byte) {
	if w.err != nil {
		return
	}
	if w.n+uint64(len(b)) > 32<<20 {
		w.err = failure(ResourceLimit, "canonical intermediate limit")
		return
	}
	w.n += uint64(len(b))
	_, w.err = w.w.Write(b)
}
func (w *canonicalWriter) size(v uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	w.put(b[:])
}
func (w *canonicalWriter) value(v reflect.Value) {
	if w.err != nil {
		return
	}
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			w.put([]byte{0})
		} else {
			w.value(v.Elem())
		}
	case reflect.Bool:
		if v.Bool() {
			w.put([]byte{1, 1})
		} else {
			w.put([]byte{1, 0})
		}
	case reflect.Uint16, reflect.Uint32, reflect.Uint64:
		var b [9]byte
		b[0] = 2
		binary.LittleEndian.PutUint64(b[1:], v.Uint())
		w.put(b[:])
	case reflect.String:
		w.put([]byte{4})
		w.size(uint32(len(v.String())))
		w.put([]byte(v.String()))
	case reflect.Array, reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			w.put([]byte{5})
			w.size(uint32(v.Len()))
			if v.Kind() == reflect.Slice {
				w.put(v.Bytes())
			} else {
				var b [32]byte
				for i := 0; i < v.Len(); i++ {
					b[i] = byte(v.Index(i).Uint())
				}
				w.put(b[:v.Len()])
			}
			return
		}
		w.put([]byte{6})
		w.size(uint32(v.Len()))
		for i := 0; i < v.Len(); i++ {
			w.value(v.Index(i))
		}
	case reflect.Struct:
		t := v.Type()
		fields := make([]int, t.NumField())
		for i := range fields {
			fields[i] = i
		}
		sort.Slice(fields, func(i, j int) bool { return t.Field(fields[i]).Name < t.Field(fields[j]).Name })
		w.put([]byte{7})
		w.size(uint32(len(fields)))
		for _, i := range fields {
			w.value(reflect.ValueOf(t.Field(i).Name))
			w.value(v.Field(i))
		}
	default:
		w.err = failure(InvalidSchema, "unsupported canonical type")
	}
}
func packetShape(p *Packet) error {
	if p.Proofs == nil || p.Builds == nil || p.Calls == nil {
		return failure(InvalidSchema, "required array is nil")
	}
	if encodedSize(reflect.ValueOf(*p), false) > 32<<20 {
		return failure(ResourceLimit, "canonical intermediate preflight")
	}
	if p.SchemaVersion != SchemaVersion || p.RuleVersion != RuleVersion || p.Status > H4 || p.Reason > Bounds {
		return failure(InvalidSchema, "packet schema/enum")
	}
	if len(p.Proofs) > 32768 || len(p.Builds) > 32768 || len(p.Calls) > 65536 {
		return failure(Bounds, "packet cardinality")
	}
	check := func(s Status, r Reason) error {
		if s > H4 || r > Bounds {
			return failure(InvalidSchema, "status/Reason enum")
		}
		return nil
	}
	read := func(r *Read) error {
		if r == nil {
			return nil
		}
		if r.Spans == nil {
			return failure(InvalidSchema, "required Spans is nil")
		}
		if r.State > NotApplicable || r.Requested > 4096 || len(r.Spans) > 2 {
			return failure(InvalidSchema, "Read schema")
		}
		for _, s := range r.Spans {
			if s.Data == nil {
				return failure(InvalidSchema, "required Data is nil")
			}
			if s.Requested > 4096 || len(s.Data) > 4096 {
				return failure(Bounds, "Span bound")
			}
		}
		return nil
	}
	for _, x := range p.Proofs {
		if x.Reads == nil {
			return failure(InvalidSchema, "required Reads is nil")
		}
		if e := check(x.Status, x.Reason); e != nil {
			return e
		}
		if len(x.Reads) > 168 {
			return failure(Bounds, "proof records")
		}
		for _, v := range x.Reads {
			if v.SignatureIndex >= 168 {
				return failure(InvalidSchema, "proof index")
			}
			if e := read(&v.Capture); e != nil {
				return e
			}
		}
	}
	for _, x := range p.Builds {
		if e := check(x.Status, x.Reason); e != nil {
			return e
		}
		for _, r := range []*Read{x.Method24, x.Raw5} {
			if e := read(r); e != nil {
				return e
			}
		}
	}
	for _, x := range p.Calls {
		if e := check(x.Status, x.Reason); e != nil {
			return e
		}
		if x.RowOrigin > ExtraRead {
			return failure(InvalidSchema, "row origin")
		}
		for _, r := range []*Read{x.Row, x.Table} {
			if e := read(r); e != nil {
				return e
			}
		}
	}
	return nil
}
func sourceHash(p Packet, h hash.Hash) ([32]byte, error) {
	var out [32]byte
	if e := packetShape(&p); e != nil {
		return out, e
	}
	w := canonicalWriter{w: h}
	w.value(reflect.ValueOf(p))
	if w.err != nil {
		return out, w.err
	}
	copy(out[:], h.Sum(nil))
	return out, nil
}
func SourceDigest(p Packet) ([32]byte, error) { return sourceHash(p, sha256.New()) }

// MarshalPacket produces normalized JSON for receipts, outside an observer callback.
func MarshalPacket(p Packet) ([]byte, error) {
	if e := packetShape(&p); e != nil {
		return nil, e
	}
	if encodedSize(reflect.ValueOf(p), true) > 8<<20 {
		return nil, failure(ResourceLimit, "receipt scratch preflight")
	}
	b, e := json.Marshal(p)
	if len(b) > 8<<20 {
		return nil, failure(ResourceLimit, "receipt scratch limit")
	}
	return b, e
}
func UnmarshalPacket(b []byte) (Packet, error) {
	var p Packet
	if len(b) > 8<<20 {
		return p, failure(ResourceLimit, "receipt scratch limit")
	}
	if e := checkJSONEnvelope(b); e != nil {
		return p, e
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e := d.Decode(&p); e != nil {
		return p, e
	}
	var tail any
	if e := d.Decode(&tail); e != io.EOF {
		return p, fmt.Errorf("trailing packet JSON")
	}
	return p, packetShape(&p)
}

// encodedSize walks the fixed schema without allocating an output buffer. The
// limit is applied while walking, so hostile slice counts cannot cause growth.
func encodedSize(v reflect.Value, jsonMode bool) uint64 {
	const stop = uint64(32<<20) + 1
	var n uint64
	add := func(k uint64) {
		n += k
		if n > stop {
			n = stop
		}
	}
	var walk func(reflect.Value)
	walk = func(v reflect.Value) {
		if n >= stop {
			return
		}
		switch v.Kind() {
		case reflect.Pointer:
			if v.IsNil() {
				if jsonMode {
					add(4)
				} else {
					add(1)
				}
			} else {
				walk(v.Elem())
			}
		case reflect.Bool:
			if jsonMode {
				if v.Bool() {
					add(4)
				} else {
					add(5)
				}
			} else {
				add(2)
			}
		case reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if jsonMode {
				add(uint64(len(strconv.FormatUint(v.Uint(), 10))))
			} else {
				add(9)
			}
		case reflect.String:
			if jsonMode {
				add(uint64(len(v.String()) + 2))
			} else {
				add(uint64(len(v.String()) + 5))
			}
		case reflect.Array, reflect.Slice:
			if v.Type().Elem().Kind() == reflect.Uint8 && !jsonMode {
				add(uint64(v.Len()) + 5)
				return
			}
			if jsonMode && v.Kind() == reflect.Slice && v.Type().Elem().Kind() == reflect.Uint8 {
				add(2 + 4*((uint64(v.Len())+2)/3))
				return
			}
			if jsonMode {
				add(2)
				if v.Len() > 0 {
					add(uint64(v.Len() - 1))
				}
			} else {
				add(5)
			}
			for i := 0; i < v.Len() && n < stop; i++ {
				if jsonMode && v.Type().Elem().Kind() == reflect.Uint8 {
					add(uint64(len(strconv.FormatUint(v.Index(i).Uint(), 10))))
				} else {
					walk(v.Index(i))
				}
			}
		case reflect.Struct:
			if jsonMode {
				add(2)
				if v.NumField() > 0 {
					add(uint64(v.NumField() - 1))
				}
			} else {
				add(5)
			}
			for i := 0; i < v.NumField() && n < stop; i++ {
				if jsonMode {
					add(uint64(len(v.Type().Field(i).Name) + 3))
				} else {
					add(uint64(len(v.Type().Field(i).Name) + 5))
				}
				walk(v.Field(i))
			}
		}
	}
	walk(v)
	return n
}

// Preflight record/cardinality bounds and duplicate keys before typed JSON allocs.
// The private JSON transport has the exact twelve record types. Preflight every
// field before encoding/json can invent a zero value or accept a case alias.
func checkJSONEnvelope(b []byte) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var metadata uint64
	var read func(reflect.Type, string, int) error
	read = func(t reflect.Type, key string, depth int) error {
		if depth > 16 {
			return failure(InvalidSchema, "JSON depth")
		}
		tok, e := d.Token()
		if e != nil {
			return e
		}
		if t.Kind() == reflect.Pointer {
			if tok == nil {
				return nil
			}
			t = t.Elem()
		} else if tok == nil {
			return failure(InvalidSchema, "required value is null")
		}
		switch t.Kind() {
		case reflect.Struct:
			if tok != json.Delim('{') {
				return failure(InvalidSchema, "expected record")
			}
			metadata += uint64(t.Size()) + 256
			if metadata > 32<<20 {
				return failure(ResourceLimit, "JSON metadata preflight")
			}
			seen := map[string]bool{}
			for d.More() {
				v, e := d.Token()
				if e != nil {
					return e
				}
				name, ok := v.(string)
				if !ok || seen[name] {
					return failure(InvalidSchema, "duplicate record field")
				}
				f, ok := t.FieldByName(name)
				if !ok || f.Name != name {
					return failure(InvalidSchema, "unknown/exact-case field")
				}
				seen[name] = true
				if e = read(f.Type, name, depth+1); e != nil {
					return e
				}
			}
			if len(seen) != t.NumField() {
				return failure(InvalidSchema, "missing required record field")
			}
			end, e := d.Token()
			if e != nil {
				return e
			}
			if end != json.Delim('}') {
				return failure(InvalidSchema, "record end")
			}
		case reflect.Array, reflect.Slice:
			if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
				v, ok := tok.(string)
				if !ok || len(v) > base64.StdEncoding.EncodedLen(4096) {
					return failure(InvalidSchema, "byte string transport")
				}
				n, e := base64.StdEncoding.Decode(make([]byte, base64.StdEncoding.DecodedLen(len(v))), []byte(v))
				if e != nil || n > 4096 {
					return failure(InvalidSchema, "base64 bytes")
				}
				return nil
			}
			if tok != json.Delim('[') {
				return failure(InvalidSchema, "required array transport")
			}
			bound := 65536
			switch key {
			case "Proofs", "Builds":
				bound = 32768
			case "Reads":
				bound = 168
			case "Spans":
				bound = 2
			}
			if t.Kind() == reflect.Array {
				bound = t.Len()
			}
			n := 0
			for d.More() {
				n++
				if n > bound {
					return failure(Bounds, "array count")
				}
				if e = read(t.Elem(), "", depth+1); e != nil {
					return e
				}
			}
			if t.Kind() == reflect.Array && n != t.Len() {
				return failure(InvalidSchema, "fixed byte array length")
			}
			end, e := d.Token()
			if e != nil {
				return e
			}
			if end != json.Delim(']') {
				return failure(InvalidSchema, "array end")
			}
		case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			n, ok := tok.(json.Number)
			if !ok {
				return failure(InvalidSchema, "unsigned transport")
			}
			if _, e = strconv.ParseUint(string(n), 10, t.Bits()); e != nil {
				return failure(InvalidSchema, "unsigned range")
			}
		case reflect.Bool:
			if _, ok := tok.(bool); !ok {
				return failure(InvalidSchema, "bool transport")
			}
		case reflect.String:
			v, ok := tok.(string)
			if !ok || v != RuleVersion {
				return failure(InvalidSchema, "RuleVersion transport")
			}
		default:
			return failure(InvalidSchema, "unsupported transport type")
		}
		return nil
	}
	if e := read(reflect.TypeFor[Packet](), "", 0); e != nil {
		return e
	}
	if _, e := d.Token(); e != io.EOF {
		return failure(InvalidSchema, "JSON trailing value")
	}
	return nil
}
