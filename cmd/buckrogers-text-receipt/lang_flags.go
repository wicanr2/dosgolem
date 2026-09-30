package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// langSwitch is one -lang-switch entry (spec 040 §5.3).
type langSwitch struct {
	step uint64
	code string
}

// parseLangSwitches reads `步數:代碼[,步數:代碼…]`; steps are absolute and
// strictly increasing.
func parseLangSwitches(s string) ([]langSwitch, error) {
	if s == "" {
		return nil, nil
	}
	var out []langSwitch
	for _, part := range strings.Split(s, ",") {
		stepText, code, ok := strings.Cut(part, ":")
		step, err := strconv.ParseUint(stepText, 10, 64)
		if !ok || err != nil || code == "" {
			return nil, fmt.Errorf("lang-switch 項目無效 %q（`步數:代碼`）", part)
		}
		out = append(out, langSwitch{step, code})
	}
	if !sort.SliceIsSorted(out, func(i, j int) bool { return out[i].step < out[j].step }) {
		return nil, fmt.Errorf("lang-switch 的步數必須遞增")
	}
	for i := 1; i < len(out); i++ {
		if out[i].step == out[i-1].step {
			return nil, fmt.Errorf("lang-switch 的步數必須嚴格遞增")
		}
	}
	return out, nil
}

// cpuDigestHex is session.StateDigest's CPU fingerprint of the machine.
func cpuDigestHex(m *machine.Machine) string {
	cpu := make([]byte, 0, 2*(len(m.CPU.R)+len(m.CPU.Seg)+2))
	for _, v := range m.CPU.R {
		cpu = binary.LittleEndian.AppendUint16(cpu, v)
	}
	for _, v := range m.CPU.Seg {
		cpu = binary.LittleEndian.AppendUint16(cpu, v)
	}
	cpu = binary.LittleEndian.AppendUint16(cpu, m.CPU.IP)
	cpu = binary.LittleEndian.AppendUint16(cpu, m.CPU.Flags)
	sum := sha256.Sum256(cpu)
	return hex.EncodeToString(sum[:])
}

func startCPUProfile(path string) (func(), error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		f.Close()
		return nil, err
	}
	return func() { pprof.StopCPUProfile(); f.Close() }, nil
}
