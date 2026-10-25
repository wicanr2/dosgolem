package hd3d

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

type r1RawExpectedWrite struct {
	IP       uint16
	Space    uint8
	Offset   uint32
	Old, New uint8
}
type r1RawMapping struct {
	Linear, Length, Offset uint32
	Space                  uint8
}
type r1RawEffectCase struct {
	Name                                                          string
	Input                                                         R1EffectsInput
	ExpectedCPU                                                   R1EffectsCPU
	ExpectedN, ExpectedCycles                                     uint64
	LastCS, LastIP                                                uint16
	LastOpcode                                                    uint8
	Writes                                                        []r1RawExpectedWrite
	Mapping                                                       []r1RawMapping
	BeforeMem, AfterMem, BeforeExt, AfterExt, InputStepConversion string
}
type r1RawSource struct {
	SHA256 string
	Bytes  int
}
type r1RawEffectOracle struct {
	Kind    string
	Cases   []r1RawEffectCase
	Sources map[string]r1RawSource
}

func r1RawSHA(p []byte) string { x := sha256.Sum256(p); return hex.EncodeToString(x[:]) }
func r1RawRead(t *testing.T, root, rel string) []byte {
	t.Helper()
	p, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("explicit fixture path %s: %v", filepath.Join(root, rel), err)
	}
	return p
}
func r1RawAddress(t *testing.T, m []r1RawMapping, linear uint32) (space uint8, offset uint32) {
	t.Helper()
	found := false
	for _, r := range m {
		if linear >= r.Linear && linear-r.Linear < r.Length {
			s, o := r.Space, r.Offset+linear-r.Linear
			if found && (s != space || o != offset) {
				t.Fatal("ambiguous captured mapping", linear)
			}
			found = true
			space, offset = s, o
		}
	}
	if !found {
		t.Fatalf("missing captured mapping %x", linear)
	}
	return
}
func TestR1EffectsRawOracle(t *testing.T) {
	root := os.Getenv("SC_R1_EFFECT_FIXTURE_ROOT")
	if root == "" {
		t.Skip("set SC_R1_EFFECT_FIXTURE_ROOT to the original direct-render evidence root")
	}
	raw := r1RawRead(t, root, "impl-pure-r1-001/oracle-raw-001.json")
	if r1RawSHA(raw) != "b1d8df3751a49b897a5964b724687929f5834854197235b13a549eaa25b37753" {
		t.Fatal("raw adapter identity changed")
	}
	var oracle r1RawEffectOracle
	if err := json.Unmarshal(raw, &oracle); err != nil {
		t.Fatal(err)
	}
	if len(oracle.Cases) != 9 {
		t.Fatal("normal plus fixed8 raw required")
	}
	for rel, pin := range oracle.Sources {
		p := r1RawRead(t, root, rel)
		if len(p) != pin.Bytes || r1RawSHA(p) != pin.SHA256 {
			t.Fatal("original fixture SHA/size changed", rel)
		}
	}
	for _, c := range oracle.Cases {
		t.Run(c.Name, func(t *testing.T) {
			p, reject := BuildR1Effects(c.Input)
			if reject != R1EffectsOK {
				t.Fatal("raw case rejected", reject)
			}
			if p.CPU != c.ExpectedCPU {
				t.Fatalf("full CPU mismatch\ngot %+v\nraw %+v", p.CPU, c.ExpectedCPU)
			}
			if p.Instructions != c.ExpectedN || p.CyclesDelta != c.ExpectedCycles || p.LastCS != c.LastCS || p.LastIP != c.LastIP || p.LastOpcode != c.LastOpcode {
				t.Fatal("cost/last original instruction mismatch", p.Instructions, p.CyclesDelta)
			}
			var xy [3][2]uint16
			for i := range xy {
				xy[i] = [2]uint16{binary.LittleEndian.Uint16(c.Input.Vertices[i*22:]), binary.LittleEndian.Uint16(c.Input.Vertices[i*22+2:])}
			}
			var clip [4]uint16
			for i := range clip {
				clip[i] = binary.LittleEndian.Uint16(c.Input.Descriptor[4+i*2:])
			}
			command := R1FlatCommand{XY: xy, Clip: clip, DestinationSegment: binary.LittleEndian.Uint16(c.Input.Descriptor[:]), Color: c.Input.Vertices[4]}
			if p.Command != command {
				t.Fatal("typed flat command lost raw source")
			}
			mem := r1RawRead(t, root, c.BeforeMem)
			afterMem := r1RawRead(t, root, c.AfterMem)
			var ext, afterExt []byte
			if c.BeforeExt != "" {
				ext = r1RawRead(t, root, c.BeforeExt)
				afterExt = r1RawRead(t, root, c.AfterExt)
			}
			pos := 0
			for _, e := range p.Effects[:p.EffectCount] {
				if e.Width != 1 && e.Width != 2 && e.Width != 4 {
					t.Fatal("invalid effect width")
				}
				for iteration := uint16(0); iteration < e.Count; iteration++ {
					for b := uint8(0); b < e.Width; b++ {
						address := uint32(e.Offset) + uint32(iteration)*uint32(e.Width) + uint32(b)
						if address > 65535 {
							t.Fatal("effect16bit wrap")
						}
						space, off := r1RawAddress(t, c.Mapping, uint32(e.Segment)*16+address)
						var dst []byte
						switch space {
						case 1:
							dst = mem
						case 2:
							dst = ext
						default:
							t.Fatal("unexpected raw space", space)
						}
						if uint64(off) >= uint64(len(dst)) {
							t.Fatal("mapped byte outside raw buffer")
						}
						w := r1RawExpectedWrite{e.IP, space, off, dst[off], uint8(e.Value >> (8 * b))}
						if pos >= len(c.Writes) || w != c.Writes[pos] {
							var want any = "end"
							if pos < len(c.Writes) {
								want = c.Writes[pos]
							}
							t.Fatalf("ordered raw write %d got %+v raw %+v", pos, w, want)
						}
						dst[off] = w.New
						pos++
					}
				}
			}
			if pos != len(c.Writes) || uint32(pos) != p.ByteCommits {
				t.Fatal("byte commits dropped", pos, len(c.Writes), p.ByteCommits)
			}
			if !bytes.Equal(mem, afterMem) || !bytes.Equal(ext, afterExt) {
				t.Fatal("complete Mem/Ext mismatch")
			}
			t.Logf("original raw writes=%d all RAM/CPU/Steps/Cycles equal (%s)", pos, c.InputStepConversion)
		})
	}
}
func ExampleR1EffectsReject() {
	// The zero input is never a source identity or native permit.
	p, r := BuildR1Effects(R1EffectsInput{})
	fmt.Println(r == R1EffectsProgram, p.EffectCount)
	// Output: true 0
}
