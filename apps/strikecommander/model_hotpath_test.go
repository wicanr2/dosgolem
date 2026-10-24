package strikecommander

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
)

type modelCountingView struct {
	*modelTestView
	reads map[string]int
}

func (v *modelCountingView) Steps() uint64      { v.reads["Steps"]++; return v.steps }
func (v *modelCountingView) IP() uint16         { v.reads["IP"]++; return v.ip }
func (v *modelCountingView) Deliveries() uint64 { v.reads["Deliveries"]++; return v.deliveries }
func (v *modelCountingView) CS() uint16         { v.reads["CS"]++; return v.cs }
func (v *modelCountingView) SS() uint16         { v.reads["SS"]++; return v.ss }
func (v *modelCountingView) SP() uint16         { v.reads["SP"]++; return v.sp }
func (v *modelCountingView) BP() uint16         { v.reads["BP"]++; return v.bp }
func (v *modelCountingView) DS() uint16         { v.reads["DS"]++; return v.ds }
func (v *modelCountingView) AX() uint16         { v.reads["AX"]++; return v.ax }
func (v *modelCountingView) BX() uint16         { v.reads["BX"]++; return v.bx }
func (v *modelCountingView) CX() uint16         { v.reads["CX"]++; return v.cx }
func (v *modelCountingView) DX() uint16         { v.reads["DX"]++; return v.dx }
func (v *modelCountingView) SI() uint16         { v.reads["SI"]++; return v.si }
func (v *modelCountingView) Flags() uint16      { v.reads["Flags"]++; return 0 }
func (v *modelCountingView) Read8(a uint32) uint8 {
	v.reads["Read8"]++
	return v.modelTestView.Read8(a)
}
func (v *modelCountingView) ReadBlock(a uint32, b []byte) bool {
	v.reads["ReadBlock"]++
	return v.modelTestView.ReadBlock(a, b)
}

func TestModelDormantKnownAndUnknownBase(t *testing.T) {
	for _, known := range []bool{false, true} {
		for _, raster := range []bool{false, true} {
			r := NewModelRecorder(nil)
			if raster {
				r.raster = &rasterRecorder{}
			}
			r.base, r.baseOK, r.lastSig = 0x241, known, 100
			v := &modelCountingView{modelTestView: newModelTestView(), reads: map[string]int{}}
			v.ip, v.steps = 0xdead, 101
			r.Observe(v)
			for method, n := range v.reads {
				if method != "IP" && method != "Steps" && n != 0 {
					t.Fatalf("base=%t raster=%t dormant read %s=%d", known, raster, method, n)
				}
			}
			if v.reads["IP"] != 1 || v.reads["Steps"] > 1 {
				t.Fatalf("dispatch reads: %+v", v.reads)
			}
			if r.pend != nil || r.open != nil || r.counts.SignatureFailures != 0 {
				t.Fatal("dormant path changed recorder")
			}
		}
	}
}

// R6 200-byte 表不放入測試原始碼；正測必須提供 oracle EXE。
func TestModelSignatureDeadlineOriginalTable(t *testing.T) {
	root := os.Getenv("SC_MODEL_ROOT")
	if root == "" {
		t.Skip("requires original oracle table")
	}
	exe, err := os.ReadFile(filepath.Join(root, "STRIKE.EXE"))
	if err != nil {
		t.Fatal(err)
	}
	tables, err := hd3d.TablesFromEXE(exe)
	if err != nil {
		t.Fatal(err)
	}
	header := int(binary.LittleEndian.Uint16(exe[8:])) * 16
	for _, bad := range []string{"", "code", "table"} {
		t.Run("bad-"+bad, func(t *testing.T) {
			r := NewSceneRecorder(nil, tables)
			r.base, r.baseOK, r.lastSig = 0x241, true, 100
			v := &modelCountingView{modelTestView: newModelTestView(), reads: map[string]int{}}
			v.ip, v.steps = 0xdead, 100+SigCheckSteps
			for _, sig := range modelSigs {
				n := len(sig.b)
				if sig.b == nil {
					n = len(sigJemm)
				}
				image := int(sig.seg)*16 + int(sig.off)
				b := append([]byte(nil), exe[header+image:header+image+n]...)
				// 套用原始 MZ 重定位表，涵蓋此簽章窗口內的 word。
				for i := 0; i < int(binary.LittleEndian.Uint16(exe[6:])); i++ {
					at := int(binary.LittleEndian.Uint16(exe[0x18:])) + i*4
					pos := int(binary.LittleEndian.Uint16(exe[at+2:]))*16 + int(binary.LittleEndian.Uint16(exe[at:]))
					if pos >= image && pos+2 <= image+n {
						off := pos - image
						binary.LittleEndian.PutUint16(b[off:], binary.LittleEndian.Uint16(b[off:])+r.base)
					}
				}
				v.put(r.base+sig.seg, sig.off, b)
			}
			v.put(r.base+0x54b0, 8, tables.R6Table)
			if !r.verify(v, r.base, true) {
				t.Fatal("original signature fixture invalid")
			}
			v.reads = map[string]int{}
			r.open, r.await, r.shown = &ModelGroup{}, &ModelGroup{}, &ModelGroup{}
			if bad == "code" {
				v.mem[lin(r.base+0x1d5e, 0x000c)] ^= 1
			}
			if bad == "table" {
				v.mem[lin(r.base+0x54b0, 8)] ^= 1
			}
			r.Observe(v)
			if v.reads["ReadBlock"] == 0 {
				t.Fatal("deadline skipped complete signature check")
			}
			if bad == "" {
				if !r.baseOK || r.lastSig != v.steps || r.open == nil || r.await == nil || r.shown == nil {
					t.Fatal("valid deadline recheck did not preserve groups")
				}
			} else if r.baseOK || r.open != nil || r.await != nil || r.shown != nil || r.counts.SignatureFailures != 1 {
				t.Fatal("invalid deadline recheck did not disable and clear all groups")
			}
		})
	}
}
