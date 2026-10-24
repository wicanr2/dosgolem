package strikecommander

import (
	"reflect"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

func TestFSourceDrainOracleAndSkip(t *testing.T) {
	r := newPresRig(t, 0)
	v := fBoot(r)
	base := r.v.steps
	r.p.VideoWrite(machine.VideoWrite{Step: base + 1, CS: 0x1234, IP: 0x1111, Offset: 5, Ctx: machine.VWV86})
	r.v.steps = base + 1
	before := sparseAuditHash(r.so)
	r.p.Observe(v)
	after := sparseAuditHash(r.so)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("a skipped idle observation changed source representation")
	}
	a := newSparseAudit(r.so)
	a.oracle(r.so, base+4096, 0x1234)
	if !strings.Contains(strings.Join(a.fails, "\n"), "來源drain期限到期") {
		t.Fatal("independent source deadline missed", a.fails)
	}
	r.v.steps = base + 4096
	r.p.Observe(v)
	if len(r.p.sourceWrites.rows) != 0 || len(r.p.unattr) != 1 || r.p.nextWake() != base+8192 {
		t.Fatal("due source drain failed")
	}
	a = newSparseAudit(r.so)
	a.oracle(r.so, r.v.steps, 0x1234)
	if strings.Contains(strings.Join(a.fails, "\n"), "來源drain期限到期") {
		t.Fatal("source oracle fires before next deadline", a.fails)
	}
}
