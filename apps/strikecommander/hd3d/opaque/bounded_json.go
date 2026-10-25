package opaque

// This restricted JSON writer is for the frozen app observation/value schemas.
// It has no encoding/json encodeState/pool or whole-value return buffer. It
// preserves Go JSON's field order, byte-slice base64, map-key order, nil and HTML
// escaping. Custom marshalers and unsupported/cyclic/deep shapes fail preflight.
import (
	"bufio"
	"crypto/sha256"
	"encoding"
	"encoding/base64"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/model"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
)

const boundedJSONFixedWorkspace uint64 = 256 << 10
const boundedJSONBuffer = 16 << 10
const boundedJSONDepth = 32

var jsonMethodType = reflect.TypeFor[interface{ MarshalJSON() ([]byte, error) }]()
var textMethodType = reflect.TypeFor[encoding.TextMarshaler]()
var sceneValueType = reflect.TypeFor[model.Input]()
var verdictValueType = reflect.TypeFor[realspace.ObjVerdict]()
var emptyVerdictValue = reflect.Zero(verdictValueType)

type boundedJSONField struct {
	index int
	name  string
	omit  bool
}
type boundedJSONSchema struct {
	fields map[reflect.Type][]boundedJSONField
	count  int
}

func (s *boundedJSONSchema) lookup(t reflect.Type) ([]boundedJSONField, error) {
	if f, ok := s.fields[t]; ok {
		return f, nil
	}
	if len(s.fields) >= 128 || t.NumField() > 1024-s.count {
		return nil, gateFailure(H3, -1, -1, "bounded JSON schema capacity")
	}
	f := make([]boundedJSONField, 0, t.NumField())
	s.count += t.NumField()
	for i := 0; i < t.NumField(); i++ {
		x := t.Field(i)
		if x.Anonymous {
			return nil, gateFailure(H3, -1, -1, "embedded field outside bounded JSON schema")
		}
		if x.PkgPath != "" {
			continue
		}
		name, option, _ := strings.Cut(x.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = x.Name
		}
		if option != "" && option != "omitempty" {
			return nil, gateFailure(H3, -1, -1, "unsupported bounded JSON tag")
		}
		f = append(f, boundedJSONField{i, name, option == "omitempty"})
	}
	s.fields[t] = f
	return f, nil
}

type boundedJSONPlan struct{ EncodedBytes, WorkspaceBytes uint64 }
type boundedJSONEmitter struct {
	out                             *bufio.Writer
	canonical                       bool
	limit, n, mapBytes, maxMapBytes uint64
	mapBudget                       uint64
	err                             error
	scratch                         [4096]byte
	schema                          *boundedJSONSchema
}

func (e *boundedJSONEmitter) fail(s string) {
	if e.err == nil {
		e.err = gateFailure(H3, -1, -1, s)
	}
}
func (e *boundedJSONEmitter) raw(s string) {
	if e.err != nil {
		return
	}
	if uint64(len(s)) > e.limit-e.n {
		e.fail("bounded JSON encoded capacity")
		return
	}
	e.n += uint64(len(s))
	if e.out != nil {
		_, e.err = e.out.WriteString(s)
	}
}
func (e *boundedJSONEmitter) data(b []byte) {
	if e.err != nil {
		return
	}
	if uint64(len(b)) > e.limit-e.n {
		e.fail("bounded JSON encoded capacity")
		return
	}
	e.n += uint64(len(b))
	if e.out != nil {
		_, e.err = e.out.Write(b)
	}
}
func (e *boundedJSONEmitter) quote(s string) {
	e.raw("\"")
	start := 0
	for i := 0; i < len(s) && e.err == nil; {
		c := s[i]
		if c >= 0x20 && c < utf8.RuneSelf && c != '"' && c != '\\' && c != '<' && c != '>' && c != '&' {
			i++
			continue
		}
		if c >= utf8.RuneSelf {
			r, n := utf8.DecodeRuneInString(s[i:])
			if !(r == utf8.RuneError && n == 1) && r != '\u2028' && r != '\u2029' {
				i += n
				continue
			}
			e.raw(s[start:i])
			if r == utf8.RuneError && n == 1 {
				e.raw("\\ufffd")
			} else if r == '\u2028' {
				e.raw("\\u2028")
			} else {
				e.raw("\\u2029")
			}
			i += n
			start = i
			continue
		}
		e.raw(s[start:i])
		switch c {
		case '"':
			e.raw("\\\"")
		case '\\':
			e.raw("\\\\")
		case '\b':
			e.raw("\\b")
		case '\f':
			e.raw("\\f")
		case '\n':
			e.raw("\\n")
		case '\r':
			e.raw("\\r")
		case '\t':
			e.raw("\\t")
		default:
			const hex = "0123456789abcdef"
			b := [6]byte{'\\', 'u', '0', '0', hex[c>>4], hex[c&15]}
			e.data(b[:])
		}
		i++
		start = i
	}
	e.raw(s[start:])
	e.raw("\"")
}
func jsonEmpty(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64, reflect.Interface, reflect.Pointer:
		return v.IsZero()
	}
	return false
}
func (e *boundedJSONEmitter) key(v reflect.Value) string {
	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(v.Uint(), 10)
	}
	e.fail("unsupported bounded JSON map key")
	return ""
}

type boundedJSONMapEntry struct {
	text string
	key  reflect.Value
}

func (e *boundedJSONEmitter) value(v reflect.Value, depth int) {
	if e.err != nil {
		return
	}
	if depth > boundedJSONDepth {
		e.fail("bounded JSON depth")
		return
	}
	if !v.IsValid() {
		e.raw("null")
		return
	}
	t := v.Type()
	if t.Implements(jsonMethodType) || t.Implements(textMethodType) || reflect.PointerTo(t).Implements(jsonMethodType) || reflect.PointerTo(t).Implements(textMethodType) {
		e.fail("custom marshaler outside bounded JSON schema")
		return
	}
	if e.canonical && t == verdictValueType {
		v = emptyVerdictValue
	}
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer:
		if v.IsNil() {
			e.raw("null")
		} else {
			e.value(v.Elem(), depth+1)
		}
	case reflect.Bool:
		if v.Bool() {
			e.raw("true")
		} else {
			e.raw("false")
		}
	case reflect.String:
		e.quote(v.String())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		e.data(strconv.AppendInt(e.scratch[:0], v.Int(), 10))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		e.data(strconv.AppendUint(e.scratch[:0], v.Uint(), 10))
	case reflect.Slice:
		if v.IsNil() {
			e.raw("null")
			return
		}
		if v.Type().Elem().Kind() == reflect.Uint8 {
			n := uint64(v.Len())
			if n > e.limit {
				e.fail("bounded JSON byte capacity")
				return
			}
			encoded := 4 * ((n + 2) / 3)
			if e.out == nil {
				if encoded+2 > e.limit-e.n {
					e.fail("bounded JSON encoded capacity")
				} else {
					e.n += encoded + 2
				}
				return
			}
			e.raw("\"")
			b := v.Bytes()
			for len(b) > 0 && e.err == nil {
				n := min(len(b), 3072)
				base64.StdEncoding.Encode(e.scratch[:], b[:n])
				e.data(e.scratch[:base64.StdEncoding.EncodedLen(n)])
				b = b[n:]
			}
			e.raw("\"")
			return
		}
		fallthrough
	case reflect.Array:
		if uint64(v.Len()) > e.limit {
			e.fail("bounded JSON array cardinality")
			return
		}
		e.raw("[")
		for i := 0; i < v.Len() && e.err == nil; i++ {
			if i != 0 {
				e.raw(",")
			}
			e.value(v.Index(i), depth+1)
		}
		e.raw("]")
	case reflect.Struct:
		fields, err := e.schema.lookup(t)
		if err != nil {
			e.err = err
			return
		}
		e.raw("{")
		first := true
		for _, f := range fields {
			if e.err != nil {
				break
			}
			x := v.Field(f.index)
			if f.omit && jsonEmpty(x) {
				continue
			}
			if !first {
				e.raw(",")
			}
			first = false
			e.quote(f.name)
			e.raw(":")
			// Only canonicalScene's Objects append normalizes an empty slice.
			if e.canonical && t == sceneValueType && f.name == "Objects" && x.Len() == 0 {
				e.raw("null")
			} else {
				e.value(x, depth+1)
			}
		}
		e.raw("}")
	case reflect.Map:
		if v.IsNil() {
			e.raw("null")
			return
		}
		if t.Elem().Size() > 64 {
			e.fail("map value outside bounded JSON schema")
			return
		}
		n := uint64(v.Len())
		if n == 0 {
			e.raw("{}")
			return
		}
		if n > (^uint64(0)-e.mapBytes)/256 {
			e.fail("bounded JSON map workspace")
			return
		}
		e.mapBytes += n * 256
		if e.mapBytes > e.mapBudget {
			e.fail("bounded JSON map workspace")
			return
		}
		if e.mapBytes > e.maxMapBytes {
			e.maxMapBytes = e.mapBytes
		}
		// Cumulative, not only simultaneously active: no reliance on a GC cycle
		// between maps. 256 bytes covers both traversals and sorted entries.
		e.raw("{")
		if e.out == nil {
			iter := v.MapRange()
			i := 0
			for iter.Next() && e.err == nil {
				if i != 0 {
					e.raw(",")
				}
				e.quote(e.key(iter.Key()))
				e.raw(":")
				e.value(iter.Value(), depth+1)
				i++
			}
		} else {
			entries := make([]boundedJSONMapEntry, v.Len())
			iter := v.MapRange()
			i := 0
			for iter.Next() {
				key := iter.Key()
				entries[i] = boundedJSONMapEntry{e.key(key), key}
				i++
			}
			sort.Slice(entries, func(i, j int) bool { return entries[i].text < entries[j].text })
			for i, x := range entries {
				if i != 0 {
					e.raw(",")
				}
				e.quote(x.text)
				e.raw(":")
				e.value(v.MapIndex(x.key), depth+1)
			}
			clear(entries)
		}
		e.raw("}")
	default:
		e.fail("type outside bounded JSON schema")
	}
}
func planBoundedJSON(v any, canonical bool, limit, workspace uint64) (boundedJSONPlan, error) {
	p, _, err := prepareBoundedJSON(v, canonical, limit, workspace)
	return p, err
}
func prepareBoundedJSON(v any, canonical bool, limit, workspace uint64) (boundedJSONPlan, *boundedJSONSchema, error) {
	if limit == 0 || workspace < boundedJSONFixedWorkspace || workspace > DefaultLimits().IntermediateBytes {
		return boundedJSONPlan{}, nil, gateFailure(H3, -1, -1, "invalid bounded JSON limit")
	}
	e := boundedJSONEmitter{canonical: canonical, limit: limit, mapBudget: workspace - boundedJSONFixedWorkspace, schema: &boundedJSONSchema{fields: make(map[reflect.Type][]boundedJSONField)}}
	e.value(reflect.ValueOf(v), 0)
	if e.err != nil {
		return boundedJSONPlan{}, nil, e.err
	}
	p := boundedJSONPlan{e.n, boundedJSONFixedWorkspace + e.maxMapBytes}
	if p.WorkspaceBytes > workspace {
		return boundedJSONPlan{}, nil, gateFailure(H3, -1, -1, "bounded JSON workspace capacity")
	}
	return p, e.schema, nil
}

// Expose only Write to bufio: optional StringWriter must not bypass strict
// short-write handling or let bufio retry short+nil. WriteString then copies
// into the existing fixed buffer; it never converts a whole long string.
type strictBoundedJSONWriter struct{ dst io.Writer }

func (w strictBoundedJSONWriter) Write(p []byte) (n int, err error) {
	n, err = w.dst.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return
}

func writeBoundedJSON(dst io.Writer, v any, canonical, newline bool, limit, workspace uint64) error {
	if dst == nil {
		return gateFailure(H3, -1, -1, "missing bounded JSON writer")
	}
	p, schema, e := prepareBoundedJSON(v, canonical, limit, workspace)
	if e != nil {
		return e
	}
	b := bufio.NewWriterSize(strictBoundedJSONWriter{dst}, boundedJSONBuffer)
	em := boundedJSONEmitter{out: b, canonical: canonical, limit: limit, mapBudget: workspace - boundedJSONFixedWorkspace, schema: schema}
	em.value(reflect.ValueOf(v), 0)
	if em.err != nil {
		return em.err
	}
	if em.n != p.EncodedBytes {
		return gateFailure(H3, -1, -1, "immutable JSON input changed during serialization")
	}
	if newline {
		if e = b.WriteByte('\n'); e != nil {
			return e
		}
	}
	return b.Flush()
}

// WriteBoundedJSON requires the caller's single pre-reserved workspace to be
// held. It emits default Go JSON plus newline with no untracked whole-value copy.
// The actual observation/packet schemas contain no floating point or custom JSON.
func WriteBoundedJSON(dst io.Writer, v any, limit, workspace uint64) error {
	if limit > DefaultLimits().IntermediateBytes {
		return gateFailure(H3, -1, -1, "invalid bounded JSON limit")
	}
	return writeBoundedJSON(dst, v, false, true, limit, workspace)
}
func BoundedInputDigest(s model.Input, limit uint64) ([32]byte, error) {
	if limit > DefaultLimits().IntermediateBytes {
		return [32]byte{}, gateFailure(H3, -1, -1, "invalid bounded JSON digest limit")
	}
	if e := inputCounts(s, DefaultLimits()); e != nil {
		return [32]byte{}, e
	}
	h := sha256.New()
	if e := writeBoundedJSON(h, s, true, false, limit, limit); e != nil {
		return [32]byte{}, e
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out, nil
}

// A material key is the original sequence of default JSON values plus newlines.
// Reuse one buffer/schema for the entire sequence rather than allocating per call.
type boundedJSONSequence struct {
	em    boundedJSONEmitter
	plan  boundedJSONEmitter
	b     *bufio.Writer
	count uint64
}

func newBoundedJSONSequence(w io.Writer) *boundedJSONSequence {
	b := bufio.NewWriterSize(strictBoundedJSONWriter{w}, boundedJSONBuffer)
	return &boundedJSONSequence{em: boundedJSONEmitter{out: b, limit: 32 << 20, mapBudget: (32 << 20) - boundedJSONFixedWorkspace, schema: &boundedJSONSchema{fields: make(map[reflect.Type][]boundedJSONField)}}, b: b}
}
func (s *boundedJSONSequence) add(v any) error {
	p := &s.plan
	*p = boundedJSONEmitter{limit: (32 << 20) - s.count, mapBudget: (32 << 20) - boundedJSONFixedWorkspace, schema: s.em.schema}
	p.value(reflect.ValueOf(v), 0)
	if p.err != nil {
		return p.err
	}
	if p.n+1 > (32<<20)-s.count {
		return gateFailure(H3, -1, -1, "material key encoded capacity")
	}
	// Material-key schemas contain no maps. Refuse a new map-bearing extension
	// until it has a sequence-wide allocation plan.
	if p.maxMapBytes != 0 {
		return gateFailure(H3, -1, -1, "material key map outside frozen schema")
	}
	s.em.n = 0
	s.em.value(reflect.ValueOf(v), 0)
	if s.em.err != nil {
		return s.em.err
	}
	if s.em.n != p.n {
		return gateFailure(H3, -1, -1, "material key changed during write")
	}
	if err := s.b.WriteByte('\n'); err != nil {
		return err
	}
	s.count += p.n + 1
	return nil
}
func (s *boundedJSONSequence) finish() error { return s.b.Flush() }
func boundedJSONDigest(v any, limit uint64) (out [32]byte, err error) {
	h := sha256.New()
	if err = writeBoundedJSON(h, v, false, false, limit, limit); err == nil {
		copy(out[:], h.Sum(nil))
	}
	return
}

// WriteBoundedInputJSON is the controlled full legacy receipt path. Its schema
// bytes remain default Go JSON; observation cardinalities keep013a's ceilings.
func WriteBoundedInputJSON(dst io.Writer, s model.Input) error {
	if err := inputCounts(s, DefaultLimits()); err != nil {
		return err
	}
	// Preserve the canonical digest ceiling. A streamed full receipt has no old
	// encoded-output ceiling: output length is not resident workspace.
	if _, err := planBoundedJSON(s, true, 32<<20, 32<<20); err != nil {
		return err
	}
	return writeBoundedJSON(dst, s, false, true, ^uint64(0), 32<<20)
}
