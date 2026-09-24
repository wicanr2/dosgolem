//go:build draft_session_manual_checkpoint_owner_pair

package session

// Local-only DRAFT evidence for the manual checkpoint owner boundary.  This
// file is intentionally self-contained: it does not export an Owner resource,
// change its lifecycle, or turn the test-only Oracle alias into an API.

import (
	"crypto/sha256"
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"unsafe"

	"github.com/wicanr2/dosgolem/apps/buckrogers"
	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/internal/state"
	"github.com/wicanr2/dosgolem/oracle"
)

const (
	draftOwnerPairCheckpointHash = "8cbc27f568057fbf3ce2f91d407953ec94836f2b723f50b7b73e56100e859269"
	draftOwnerPairOverlayHash    = "3a4ad4856c08fe5973179f1d907feed1d870af99d08abd1cb884b316324f3cc0"
	draftOwnerPairStartStep      = uint64(266399999)
	draftOwnerPairHardStop       = uint64(266557247)
)

// draftOwnerPairOracle aliases only one test-owned machine/DOS pair. Oracle
// deliberately has no public constructor for an Owner-owned machine;
// reflection/unsafe stays confined to this DRAFT test to preserve that seam.
func draftOwnerPairOracle(t *testing.T, m *machine.Machine, d *dos.DOS) *oracle.Oracle {
	t.Helper()
	if m == nil || d == nil {
		t.Fatal("DRAFT setup needs one live machine/DOS pair")
	}
	o := new(oracle.Oracle)
	fields := reflect.ValueOf(o).Elem()
	set := func(name string, value any) {
		t.Helper()
		field := fields.FieldByName(name)
		if !field.IsValid() || !field.CanAddr() {
			t.Fatalf("DRAFT Oracle alias field %q is unavailable", name)
		}
		reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(value))
	}
	set("m", m)
	set("d", d)
	set("onCall", map[uint32][]func(*oracle.Oracle){})
	return o
}

func draftOwnerPairLocalInput(t *testing.T) (project, checkpoint string) {
	t.Helper()
	project = os.Getenv("BUCK_OWNER_PROJECT")
	if project == "" {
		t.Skip("local original manual checkpoint is not supplied")
	}
	checkpoint = filepath.Join(project, "workplace/probe/phase12-before-question.state")
	bytes, err := os.ReadFile(checkpoint)
	if err != nil {
		t.Skip("local original manual checkpoint is unavailable")
	}
	// The sealed checkpoint records its original overlay as /orig/GAME.OVR.
	// It must be supplied through the caller's read-only /orig mount; this
	// test neither synthesizes the file nor rewrites the restored DOS path.
	overlay, err := os.ReadFile("/orig/GAME.OVR")
	if err != nil {
		t.Skip("local original overlay is unavailable at read-only /orig")
	}
	if got := fmtDigest(sha256.Sum256(overlay)); got != draftOwnerPairOverlayHash {
		t.Fatalf("original overlay SHA-256=%s", got)
	}
	// Only the digest enters diagnostics; original state bytes stay private.
	if got := fmtDigest(sha256.Sum256(bytes)); got != draftOwnerPairCheckpointHash {
		t.Fatalf("checkpoint SHA-256=%s", got)
	}
	return project, checkpoint
}

func fmtDigest(sum [sha256.Size]byte) string {
	const hex = "0123456789abcdef"
	out := make([]byte, 0, sha256.Size*2)
	for _, b := range sum {
		out = append(out, hex[b>>4], hex[b&15])
	}
	return string(out)
}

func draftOwnerPairCatalog(t *testing.T, project string) *buckrogers.Catalog {
	t.Helper()
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join(project, "text", name))
		if err != nil {
			t.Fatalf("local catalog input unavailable: %s", name)
		}
		return data
	}
	catalog, err := buckrogers.LoadCatalog(read("manual-events.tsv"), read("manual-ordinals.tsv"), read("manual.zh-TW.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

// draftOwnerPairEventSummary intentionally stores no original string,
// translation, glyph, request, or complete event.  The digest commits to the
// exact event sequence and content-safe request metadata for equality only.
type draftOwnerPairEventSummary struct {
	Count, Begins, Clears, Requests int
	Digest                          [sha256.Size]byte
}

func draftOwnerPairEvents(events []buckrogers.ManualPresentationEvent) draftOwnerPairEventSummary {
	h := sha256.New()
	s := draftOwnerPairEventSummary{Count: len(events)}
	var word [8]byte
	put := func(v uint64) { binary.LittleEndian.PutUint64(word[:], v); _, _ = h.Write(word[:]) }
	for _, event := range events {
		put(event.Step)
		put(event.Generation)
		_, _ = h.Write([]byte(event.Kind))
		_, _ = h.Write([]byte{0})
		switch event.Kind {
		case buckrogers.ManualPresentationBegin:
			s.Begins++
		case buckrogers.ManualPresentationClear:
			s.Clears++
		case buckrogers.ManualPresentationRequest:
			s.Requests++
			// The identity and rune count are committed as a digest, never logged.
			_, _ = h.Write([]byte(event.Request.EventKey))
			_, _ = h.Write([]byte{0})
			_, _ = h.Write([]byte(event.Request.TextKey))
			_, _ = h.Write([]byte{0})
			put(uint64(len([]rune(event.Request.Translation))))
		}
	}
	copy(s.Digest[:], h.Sum(nil))
	return s
}

// draftOwnerPairTerminalSummary has hashes and scalar DOS exit status only. It
// never returns memory, pixels, palette entries, register contents, or text.
type draftOwnerPairTerminalSummary struct {
	Steps                         uint64
	Exited                        bool
	ExitCode                      uint8
	Memory, CPU, Indexed, Palette [sha256.Size]byte
}

func draftOwnerPairCPUHash(c *cpu.CPU) [sha256.Size]byte {
	if c == nil {
		return [sha256.Size]byte{}
	}
	h := sha256.New()
	var word [8]byte
	put := func(v uint64) { binary.LittleEndian.PutUint64(word[:], v); _, _ = h.Write(word[:]) }
	for _, value := range c.R {
		put(uint64(value))
	}
	for _, value := range c.Seg {
		put(uint64(value))
	}
	put(uint64(c.IP))
	put(uint64(c.Flags))
	put(uint64(c.Model))
	put(c.Cycles)
	if c.Halted {
		put(1)
	} else {
		put(0)
	}
	put(uint64(c.EAXHi))
	for _, value := range c.DivErrors {
		put(uint64(value.CS))
		put(uint64(value.IP))
	}
	var sum [sha256.Size]byte
	copy(sum[:], h.Sum(nil))
	return sum
}

func draftOwnerPairPaletteHash(palette [256][3]uint8) [sha256.Size]byte {
	h := sha256.New()
	for _, rgb := range palette {
		_, _ = h.Write(rgb[:])
	}
	var sum [sha256.Size]byte
	copy(sum[:], h.Sum(nil))
	return sum
}

func draftOwnerPairTerminal(m *machine.Machine, d *dos.DOS) draftOwnerPairTerminalSummary {
	return draftOwnerPairTerminalSummary{
		Steps:    m.Steps,
		Exited:   d.Exited,
		ExitCode: d.ExitCode,
		Memory:   sha256.Sum256(m.Mem),
		CPU:      draftOwnerPairCPUHash(m.CPU),
		Indexed:  sha256.Sum256(m.Indexed()), Palette: draftOwnerPairPaletteHash(m.Palette()),
	}
}

// TestDraftManualCheckpointOwnerPair compares two independent continuations:
// one starts with the same private Owner machine/DOS loaded by this test and
// the other with a separately restored machine/DOS pair. Both use the formal
// Buck Rogers Watcher.  It is local-only evidence, not a production session
// route or an assertion that Owner.Advance already has this behavior.
func TestDraftManualCheckpointOwnerPair(t *testing.T) {
	project, checkpoint := draftOwnerPairLocalInput(t)
	owner, err := New(Config{InitialScale: host.OutputScale2})
	if err != nil {
		t.Fatal("DRAFT Owner setup failed")
	}
	defer owner.Close()
	owner.dos.Install()
	if err := state.Load(checkpoint, owner.machine, owner.dos); err != nil {
		t.Fatal("DRAFT Owner checkpoint load failed")
	}
	if owner.machine.Steps != draftOwnerPairStartStep {
		t.Fatalf("checkpoint start step=%d", owner.machine.Steps)
	}

	ownerOracle := draftOwnerPairOracle(t, owner.machine, owner.dos)
	ownerWatcher := buckrogers.NewWatcher(draftOwnerPairCatalog(t, project))
	ownerWatcher.Install(ownerOracle)
	delta := draftOwnerPairHardStop - ownerOracle.Steps()
	if delta == 0 {
		t.Fatal("DRAFT replay interval is empty")
	}
	if err := ownerOracle.RunUntil(oracle.Steps(delta), oracle.Budget(delta+1)); err != nil {
		t.Fatal("DRAFT Owner replay failed before hard stop")
	}
	ownerEvents := draftOwnerPairEvents(ownerWatcher.PresentationEvents())
	ownerTerminal := draftOwnerPairTerminal(owner.machine, owner.dos)

	independentMachine := machine.New()
	independentDOS := dos.New(independentMachine, ".")
	defer independentDOS.Close()
	independentDOS.Install()
	if err := state.Load(checkpoint, independentMachine, independentDOS); err != nil {
		t.Fatalf("DRAFT independent checkpoint load failed: %v", err)
	}
	independent := draftOwnerPairOracle(t, independentMachine, independentDOS)
	if independent.Steps() != draftOwnerPairStartStep {
		t.Fatalf("independent start step=%d", independent.Steps())
	}
	independentWatcher := buckrogers.NewWatcher(draftOwnerPairCatalog(t, project))
	independentWatcher.Install(independent)
	if err := independent.RunUntil(oracle.Steps(delta), oracle.Budget(delta+1)); err != nil {
		t.Fatal("DRAFT independent replay failed before hard stop")
	}
	independentEvents := draftOwnerPairEvents(independentWatcher.PresentationEvents())
	independentTerminal := draftOwnerPairTerminal(independentMachine, independentDOS)

	if ownerOracle.Steps() != draftOwnerPairHardStop || independent.Steps() != draftOwnerPairHardStop {
		t.Fatalf("hard-stop steps=%d/%d", ownerOracle.Steps(), independent.Steps())
	}
	if ownerEvents.Begins != 1 || ownerEvents.Clears != 1 || ownerEvents.Requests != 1 ||
		independentEvents.Begins != 1 || independentEvents.Clears != 1 || independentEvents.Requests != 1 {
		t.Fatalf("event counts begin/clear/request=%d/%d/%d vs %d/%d/%d", ownerEvents.Begins, ownerEvents.Clears, ownerEvents.Requests, independentEvents.Begins, independentEvents.Clears, independentEvents.Requests)
	}
	if ownerEvents != independentEvents {
		t.Fatalf("event sequence differs: count=%d/%d digest=%x/%x", ownerEvents.Count, independentEvents.Count, ownerEvents.Digest, independentEvents.Digest)
	}
	if ownerTerminal != independentTerminal {
		t.Fatalf("terminal summary differs: steps=%d/%d exited=%t/%t exit-code=%d/%d memory=%x/%x cpu=%x/%x indexed=%x/%x palette=%x/%x", ownerTerminal.Steps, independentTerminal.Steps, ownerTerminal.Exited, independentTerminal.Exited, ownerTerminal.ExitCode, independentTerminal.ExitCode, ownerTerminal.Memory, independentTerminal.Memory, ownerTerminal.CPU, independentTerminal.CPU, ownerTerminal.Indexed, independentTerminal.Indexed, ownerTerminal.Palette, independentTerminal.Palette)
	}
	// The only receipt deliberately exposed by this test is safe metadata.
	t.Logf("DRAFT owner/checkpoint pair: events=%d begin/clear/request=%d/%d/%d event-hash=%x terminal-hash(memory/cpu/indexed/palette)=%x/%x/%x/%x equal=true", ownerEvents.Count, ownerEvents.Begins, ownerEvents.Clears, ownerEvents.Requests, ownerEvents.Digest, ownerTerminal.Memory, ownerTerminal.CPU, ownerTerminal.Indexed, ownerTerminal.Palette)
}
