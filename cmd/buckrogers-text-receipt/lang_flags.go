package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/apps/buckrogers"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// langPathFlag is a repeatable `代碼=路徑` flag (Buck repo spec 041 §3.9).
type langPathFlag map[string]string

func (f langPathFlag) String() string {
	keys := make([]string, 0, len(f))
	for k := range f {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + f[k]
	}
	return strings.Join(parts, ",")
}

func (f langPathFlag) Set(v string) error {
	code, path, ok := strings.Cut(v, "=")
	if !ok || code == "" || path == "" {
		return fmt.Errorf("需要 `代碼=路徑`：%q", v)
	}
	if !buckrogers.KnownLang(code) {
		return fmt.Errorf("不認得的語言代碼 %q", code)
	}
	if _, dup := f[code]; dup {
		return fmt.Errorf("語言 %s 重複指定", code)
	}
	f[code] = path
	return nil
}

// receiptLangs decides which extra lanes the receipt loads (spec 041 §3.9):
// every language named by -lang, -lang-switch, -lang-dir or -lang-font,
// except zh-TW (always loaded) and en (no lane).  A language's directory
// defaults to -live-text-dir; its font to -lang-font, else
// <font-root>/font/buckrogers-<lang>.golemfnt when that file exists (root
// defaults to the parent of -live-text-dir), and for the test language zz
// to the zh-TW overlay font as before.
func receiptLangs(textDir, fontRoot, overlayFont, start string, switches []string, dirs, fonts langPathFlag) ([]string, map[string]string, map[string]string) {
	if fontRoot == "" {
		fontRoot = filepath.Dir(filepath.Clean(textDir))
	}
	want := map[string]bool{}
	add := func(code string) {
		if code != "" && code != buckrogers.LangZhTW && code != buckrogers.LangEn {
			want[code] = true
		}
	}
	add(start)
	for _, c := range switches {
		add(c)
	}
	for c := range dirs {
		add(c)
	}
	for c := range fonts {
		add(c)
	}
	var langs []string
	for _, c := range append(append([]string{}, buckrogers.LangCycle...), buckrogers.LangTest) {
		if want[c] {
			langs = append(langs, c)
		}
	}
	if len(langs) == 0 {
		return nil, nil, nil
	}
	outDirs, outFonts := map[string]string{}, map[string]string{}
	for _, c := range langs {
		outDirs[c] = textDir
		if d, ok := dirs[c]; ok {
			outDirs[c] = d
		}
		switch {
		case fonts[c] != "":
			outFonts[c] = fonts[c]
		case c == buckrogers.LangTest:
			outFonts[c] = overlayFont
		default:
			if p := buckrogers.LangFontPath(fontRoot, c); fileExists(p) {
				outFonts[c] = p
			}
		}
	}
	return langs, outDirs, outFonts
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

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
