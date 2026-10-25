package r5asset

import (
	"math"
	"strconv"
	"unicode/utf8"
	"unsafe"
)

const JSONReservation uint64 = 131072
const JSONBodyLimit uint64 = 126976
const numberReservation uint64 = 4096

type tokenKind uint8

const (
	tObject tokenKind = iota + 1
	tArray
	tString
	tNumber
	tTrue
	tFalse
	tNull
	tEndObject
	tEndArray
	tColon
	tComma
)

type token struct {
	Start, End, Next uint32
	Kind             tokenKind
	_                [3]byte
}
type frame struct {
	Start, Count, Seen, LastKey uint32
	State, Kind                 uint8
	_                           [14]byte
}

// All owned parsing backing is fixed. Scalar call controls fit the remaining
// body slack; strconv's single bounded conversion is reserved separately.
type jsonArena struct {
	raw            [9840]byte
	tokens         [4096]token
	frames         [32]frame
	strings        [2][256]byte
	number         [64]byte
	typed          [16384]byte
	errorRecord    [256]byte
	hashControl    [512]byte
	scanner        [1024]byte
	n, rawN, nodes uint32
}

const arenaBytes = uint64(unsafe.Sizeof(jsonArena{}))

type value struct {
	a     *jsonArena
	i     uint32
	valid bool
}

func (a *jsonArena) add(k tokenKind, start, end int) error {
	if a.n == uint32(len(a.tokens)) {
		return reject(Capacity, start, 1)
	}
	a.tokens[a.n] = token{Start: uint32(start), End: uint32(end), Next: a.n + 1, Kind: k}
	a.n++
	return nil
}
func hexDigit(c byte) (uint32, bool) {
	if c >= '0' && c <= '9' {
		return uint32(c - '0'), true
	}
	if c >= 'a' && c <= 'f' {
		return uint32(c - 'a' + 10), true
	}
	if c >= 'A' && c <= 'F' {
		return uint32(c - 'A' + 10), true
	}
	return 0, false
}
func quad(s []byte) (uint32, bool) {
	if len(s) < 4 {
		return 0, false
	}
	var n uint32
	for _, c := range s[:4] {
		v, ok := hexDigit(c)
		if !ok {
			return 0, false
		}
		n = n*16 + v
	}
	return n, true
}
func decodeString(raw []byte, dst *[256]byte) (int, error) {
	if len(raw) > 256 {
		return 0, reject(Capacity, 0, 2)
	}
	n := 0
	for i := 0; i < len(raw); {
		if raw[i] < 0x20 {
			return 0, reject(Structure, i, 3)
		}
		if raw[i] != '\\' {
			r, size := utf8.DecodeRune(raw[i:])
			if r == utf8.RuneError && size == 1 {
				return 0, reject(Structure, i, 4)
			}
			if n+size > len(dst) {
				return 0, reject(Capacity, i, 5)
			}
			copy(dst[n:n+size], raw[i:i+size])
			n += size
			i += size
			continue
		}
		i++
		if i == len(raw) {
			return 0, reject(Structure, i, 6)
		}
		c := raw[i]
		i++
		var r rune
		switch c {
		case '"', '\\', '/':
			r = rune(c)
		case 'b':
			r = '\b'
		case 'f':
			r = '\f'
		case 'n':
			r = '\n'
		case 'r':
			r = '\r'
		case 't':
			r = '\t'
		case 'u':
			v, ok := quad(raw[i:])
			if !ok {
				return 0, reject(Structure, i, 7)
			}
			i += 4
			if v >= 0xd800 && v <= 0xdbff {
				if i+6 > len(raw) || raw[i] != '\\' || raw[i+1] != 'u' {
					return 0, reject(Structure, i, 8)
				}
				lo, ok := quad(raw[i+2:])
				if !ok || lo < 0xdc00 || lo > 0xdfff {
					return 0, reject(Structure, i, 9)
				}
				i += 6
				v = 0x10000 + (v-0xd800)*1024 + (lo - 0xdc00)
			} else if v >= 0xdc00 && v <= 0xdfff {
				return 0, reject(Structure, i, 10)
			}
			r = rune(v)
		default:
			return 0, reject(Structure, i, 11)
		}
		size := utf8.RuneLen(r)
		if size < 0 || n+size > len(dst) {
			return 0, reject(Capacity, i, 12)
		}
		n += utf8.EncodeRune(dst[n:], r)
	}
	return n, nil
}

func (a *jsonArena) lex(raw []byte) error {
	if arenaBytes > JSONBodyLimit || uint64(len(raw)) > uint64(len(a.raw)) {
		return reject(Capacity, 0, 13)
	}
	copy(a.raw[:], raw)
	a.rawN = uint32(len(raw))
	a.n, a.nodes = 0, 0
	for i := 0; i < len(raw); {
		start := i
		c := raw[i]
		if c == ' ' || c == '\n' || c == '\r' || c == '\t' {
			i++
			continue
		}
		var k tokenKind
		switch c {
		case '{':
			k = tObject
			i++
		case '[':
			k = tArray
			i++
		case '}':
			k = tEndObject
			i++
		case ']':
			k = tEndArray
			i++
		case ':':
			k = tColon
			i++
		case ',':
			k = tComma
			i++
		case '"':
			i++
			for i < len(raw) && raw[i] != '"' {
				if i-start-1 >= 256 {
					return reject(Capacity, start, 14)
				}
				if raw[i] == '\\' {
					i++
					if i == len(raw) {
						return reject(Structure, start, 15)
					}
				}
				i++
			}
			if i == len(raw) {
				return reject(Structure, start, 16)
			}
			if _, e := decodeString(raw[start+1:i], &a.strings[0]); e != nil {
				return e
			}
			i++
			k = tString
		case 't', 'f', 'n':
			word := "true"
			k = tTrue
			if c == 'f' {
				word = "false"
				k = tFalse
			}
			if c == 'n' {
				word = "null"
				k = tNull
			}
			if len(raw)-i < len(word) || !bytesStringEqual(raw[i:i+len(word)], word) {
				return reject(Structure, i, 17)
			}
			i += len(word)
		default:
			k = tNumber
			if c == '-' {
				i++
				if i == len(raw) {
					return reject(Structure, i, 18)
				}
			}
			if raw[i] == '0' {
				i++
			} else if raw[i] >= '1' && raw[i] <= '9' {
				for i < len(raw) && raw[i] >= '0' && raw[i] <= '9' {
					i++
					if i-start > 64 {
						return reject(Capacity, start, 19)
					}
				}
			} else {
				return reject(Structure, i, 20)
			}
			if i < len(raw) && raw[i] == '.' {
				i++
				j := i
				for i < len(raw) && raw[i] >= '0' && raw[i] <= '9' {
					i++
					if i-start > 64 {
						return reject(Capacity, start, 21)
					}
				}
				if i == j {
					return reject(Structure, i, 22)
				}
			}
			if i < len(raw) && (raw[i] == 'e' || raw[i] == 'E') {
				i++
				if i < len(raw) && (raw[i] == '+' || raw[i] == '-') {
					i++
				}
				j := i
				for i < len(raw) && raw[i] >= '0' && raw[i] <= '9' {
					i++
					if i-start > 64 {
						return reject(Capacity, start, 23)
					}
				}
				if i == j {
					return reject(Structure, i, 24)
				}
			}
			if i-start > 64 {
				return reject(Capacity, start, 25)
			}
		}
		if e := a.add(k, start, i); e != nil {
			return e
		}
	}
	return nil
}

// grammar is iterative. Counts are checked before consuming each child.
func (a *jsonArena) grammar() error {
	depth := 0
	rootDone := false
	for i := uint32(0); i < a.n; i++ {
		t := &a.tokens[i]
		if depth > 0 {
			f := &a.frames[depth-1]
			if f.Kind == uint8(tObject) {
				switch f.State {
				case 0, 4:
					if t.Kind == tEndObject && f.State == 0 {
						a.tokens[f.Start].Next = i + 1
						depth--
						continue
					}
					if t.Kind != tString {
						return reject(Structure, int(t.Start), 26)
					}
					if f.Count == 32 {
						return reject(Capacity, int(t.Start), 27)
					}
					f.Count++
					f.State = 1
					continue
				case 1:
					if t.Kind != tColon {
						return reject(Structure, int(t.Start), 28)
					}
					f.State = 2
					continue
				case 3:
					if t.Kind == tEndObject {
						a.tokens[f.Start].Next = i + 1
						depth--
						continue
					}
					if t.Kind != tComma {
						return reject(Structure, int(t.Start), 29)
					}
					f.State = 4
					continue
				case 2:
					f.State = 3
				}
			} else {
				switch f.State {
				case 0, 2:
					if t.Kind == tEndArray && f.State == 0 {
						a.tokens[f.Start].Next = i + 1
						depth--
						continue
					}
					if f.Count == 64 {
						return reject(Capacity, int(t.Start), 30)
					}
					f.Count++
					f.State = 1
				case 1:
					if t.Kind == tEndArray {
						a.tokens[f.Start].Next = i + 1
						depth--
						continue
					}
					if t.Kind != tComma {
						return reject(Structure, int(t.Start), 31)
					}
					f.State = 2
					continue
				}
			}
		} else {
			if rootDone {
				return reject(Structure, int(t.Start), 32)
			}
			rootDone = true
		}
		if t.Kind < tObject || t.Kind > tNull {
			return reject(Structure, int(t.Start), 33)
		}
		if depth+1 > 32 || a.nodes == 2048 {
			return reject(Capacity, int(t.Start), 34)
		}
		a.nodes++
		if t.Kind == tObject || t.Kind == tArray {
			a.frames[depth] = frame{Start: i, Kind: uint8(t.Kind)}
			depth++
		}
	}
	if !rootDone || depth != 0 {
		return reject(Structure, int(a.rawN), 35)
	}
	return nil
}
func (a *jsonArena) parse(raw []byte) (value, error) {
	if e := a.lex(raw); e != nil {
		return value{}, e
	}
	if e := a.grammar(); e != nil {
		return value{}, e
	}
	return value{a, 0, true}, nil
}
func bytesStringEqual(b []byte, s string) bool {
	if len(b) != len(s) {
		return false
	}
	for i := range b {
		if b[i] != s[i] {
			return false
		}
	}
	return true
}
func (x value) kind() tokenKind {
	if !x.valid {
		return 0
	}
	return x.a.tokens[x.i].Kind
}
func (x value) equalString(s string) bool {
	if x.kind() != tString {
		return false
	}
	t := x.a.tokens[x.i]
	n, e := decodeString(x.a.raw[t.Start+1:t.End-1], &x.a.strings[0])
	return e == nil && bytesStringEqual(x.a.strings[0][:n], s)
}
func (x value) at(n int) value {
	if x.kind() != tArray || n < 0 {
		return value{}
	}
	i := x.i + 1
	end := x.a.tokens[x.i].Next - 1
	for j := 0; i < end; j++ {
		if j == n {
			return value{x.a, i, true}
		}
		i = x.a.tokens[i].Next
		if i < end {
			i++
		}
	}
	return value{}
}
func (x value) length() int {
	if x.kind() != tArray {
		return -1
	}
	n := 0
	i := x.i + 1
	end := x.a.tokens[x.i].Next - 1
	for i < end {
		n++
		i = x.a.tokens[i].Next
		if i < end {
			i++
		}
	}
	return n
}
func (x value) get(key string) value {
	if x.kind() != tObject {
		return value{}
	}
	i := x.i + 1
	end := x.a.tokens[x.i].Next - 1
	for i < end {
		v := i + 2
		if (value{x.a, i, true}).equalString(key) {
			return value{x.a, v, true}
		}
		i = x.a.tokens[v].Next
		if i < end {
			i++
		}
	}
	return value{}
}
func (x value) uint() (uint64, bool) {
	if x.kind() != tNumber {
		return 0, false
	}
	t := x.a.tokens[x.i]
	var n uint64
	for _, c := range x.a.raw[t.Start:t.End] {
		if c < '0' || c > '9' {
			return 0, false
		}
		d := uint64(c - '0')
		if n > (^uint64(0)-d)/10 {
			return 0, false
		}
		n = n*10 + d
	}
	return n, true
}
func (x value) float() (float64, bool) {
	if x.kind() != tNumber {
		return 0, false
	}
	t := x.a.tokens[x.i]
	if t.End-t.Start > 64 {
		return 0, false
	}
	f, e := strconv.ParseFloat(string(x.a.raw[t.Start:t.End]), 64)
	return f, e == nil && !math.IsNaN(f) && !math.IsInf(f, 0)
}
