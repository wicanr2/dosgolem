package session

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/dos"
)

// This test consumes the pre-464 offline raw-Mem oracle. No guest instruction,
// Deliver, Advance, new game probe, or production reader computes its expected.
func TestBootPSPSixFixedStatesZeroGuest(t *testing.T) {
	if os.Getenv("SC_R8_REAL") != "1" {
		t.Skip("explicit six-state fixture opt-in")
	}
	var oracle struct {
		Rows []struct {
			State, StateSHA256 string
			CurPSP             uint16
			Steps              uint64
			Matches            []string
			Candidates         []struct {
				Name                                  string
				EntryOffset                           uint32
				ActualPrefixSHA256, ActualEntrySHA256 string
				PrefixMatch, EntryMatch               bool
			}
		}
	}
	b, e := os.ReadFile("/project/workplace/impl016/session-r8/raw-mem-result-001.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &oracle); e != nil {
		t.Fatal(e)
	}
	if len(oracle.Rows) != 6 {
		t.Fatal("not fixed six cases")
	}
	var results []any
	for _, row := range oracle.Rows {
		t.Run(filepath.Base(row.State), func(t *testing.T) {
			raw, e := os.ReadFile(row.State)
			if e != nil {
				t.Fatal(e)
			}
			if fmt.Sprintf("%x", sha256.Sum256(raw)) != row.StateSHA256 {
				t.Fatal("state source drift")
			}
			// Materialize only the files named by original saved handles. Existing
			// RebasePath supplies the legacy routing, and all source files stay RO.
			z, e := gzip.NewReader(bytes.NewReader(raw))
			if e != nil {
				t.Fatal(e)
			}
			var n uint64
			if e = binary.Read(z, binary.LittleEndian, &n); e != nil {
				t.Fatal(e)
			}
			if n > 256<<20 {
				t.Fatal("section limit")
			}
			if _, e = io.CopyN(io.Discard, z, int64(n)); e != nil {
				t.Fatal(e)
			}
			if e = binary.Read(z, binary.LittleEndian, &n); e != nil {
				t.Fatal(e)
			}
			if n > 256<<20 {
				t.Fatal("section limit")
			}
			var saved struct {
				Root    string
				Handles []struct {
					Path string
					Size int64
				}
			}
			if e = gob.NewDecoder(io.LimitReader(z, int64(n))).Decode(&saved); e != nil {
				t.Fatal(e)
			}
			if e = z.Close(); e != nil {
				t.Fatal(e)
			}
			root := t.TempDir()
			for _, h := range saved.Handles {
				target, e := dos.RebasePath(h.Path, []string{saved.Root, "/scratch"}, root)
				if e != nil {
					t.Fatal(e)
				}
				rel, e := filepath.Rel(root, target)
				if e != nil || strings.HasPrefix(rel, "..") {
					t.Fatal("unsafe fixture path")
				}
				source := filepath.Join("/project/workplace/root", rel)
				data, e := os.ReadFile(source)
				if e != nil {
					t.Fatalf("saved handle %q source %q: %v", h.Path, source, e)
				}
				if h.Size != 0 && int64(len(data)) != h.Size {
					t.Fatal("saved handle source size")
				}
				if e = os.MkdirAll(filepath.Dir(target), 0700); e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(target, data, 0600); e != nil {
					t.Fatal(e)
				}
			}
			cfg := stateConfig()
			cfg.SoundBlaster = true
			cfg.SBDMA8 = true
			cfg.ExtendedMemoryKB = 8192
			cfg.NoEMS = true
			cfg.ProtectedModeProfile = true
			cfg.ExecEnvCopy = true
			cfg.ExecReclaim = true
			cfg.KbdIntercept = true
			cfg.Mode13Phase = true
			cfg.ProgramPath = "C:\\SCCD.EXE"
			cfg.DOSFiles = 30
			o := newStateOwner(t, cfg)
			boot, e := o.BootFromState(StateInput{State: raw, ExpectedStateSHA256: sha256.Sum256(raw), SaveRoot: root, RebaseFrom: []string{"/scratch"}})
			if e != nil {
				t.Fatal(e)
			}
			if boot.Steps != row.Steps {
				t.Fatal("boot steps mismatch")
			}
			var ranges []BootPSPRange
			for _, c := range row.Candidates {
				ranges = append(ranges, BootPSPRange{0x100, make([]byte, 256)}, BootPSPRange{0x100 + c.EntryOffset, make([]byte, 64)})
			}
			if len(ranges) != 12 {
				t.Fatal("not all six candidates")
			}
			before := bootPSPSnap(t, o)
			frames, deliveries := o.machine.Frames, o.machine.Deliveries()
			receipt, e := o.ReadBootPSP(ranges)
			if e != nil {
				t.Fatal(e)
			}
			if receipt.PSP != row.CurPSP {
				t.Fatal("actual PSP differs from independent raw")
			}
			after := bootPSPSnap(t, o)
			after.Used = before.Used
			if !reflect.DeepEqual(before, after) || frames != o.machine.Frames || deliveries != o.machine.Deliveries() {
				t.Fatal("bootstrap read changed original state/counters")
			}
			for i, c := range row.Candidates {
				gotPrefix := fmt.Sprintf("%x", sha256.Sum256(ranges[2*i].Dst))
				gotEntry := fmt.Sprintf("%x", sha256.Sum256(ranges[2*i+1].Dst))
				if gotPrefix != c.ActualPrefixSHA256 || gotEntry != c.ActualEntrySHA256 {
					t.Fatalf("raw oracle mismatch %s", c.Name)
				}
				// No reader output is used to choose the expected candidate.
				if _, e = hex.DecodeString(c.ActualPrefixSHA256); e != nil {
					t.Fatal(e)
				}
			}
			results = append(results, map[string]any{"State": row.State, "SHA256": row.StateSHA256, "Steps": boot.Steps, "PSP": receipt.PSP, "Ranges": len(ranges), "Bytes": 1920, "MatchesFromIndependentRaw": row.Matches, "All12RawHashesEqual": true, "StateDigestUnchanged": true, "FullDigestUnchanged": true, "MemoryUnchanged": true, "Frames": frames, "Deliveries": deliveries, "ZeroGuestSteps": true})
		})
	}
	if dest := os.Getenv("SC_R8_RESULT"); dest != "" {
		data, e := json.MarshalIndent(results, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(dest, append(data, '\n'), 0600); e != nil {
			t.Fatal(e)
		}
	}
}
