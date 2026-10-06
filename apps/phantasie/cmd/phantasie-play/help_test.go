package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/apps/phantasie"
	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/dosgolem/xlate"
)

func helpFixture() []byte {
	return []byte("key\ttranslation\tsource\n" +
		"title\tHelp\tHelp\npause\tPaused\tPaused\nopen\tOpen\tOpen\nclose\tClose\tClose\n" +
		"language\tLanguage\tLanguage\ntheme\tTheme\tTheme\nfullscreen\tFullscreen\tFullscreen\n" +
		"arrows\tArrows\tArrows\nconfirm\tConfirm\tConfirm\nletters\tLetters\tLetters\n" +
		"context\tContext\tContext\nfooter\tReturn\tReturn\n")
}

func TestHelpRejectsBrokenCatalogAndMissingGlyph(t *testing.T) {
	valid := helpFixture()
	if _, err := parseHelp(valid, nil, nil, true); err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{append([]byte{0xff}, valid...), bytes.Replace(valid, []byte("title\t"), []byte("unknown\t"), 1),
		bytes.Replace(valid, []byte("pause\t"), []byte("title\t"), 1), valid[:len(valid)-1],
		bytes.Replace(valid, []byte("Help\tHelp"), []byte(strings.Repeat("A", 73)+"\tHelp"), 1),
		bytes.Replace(valid, []byte("Paused"), []byte("Pause\r"), 1), bytes.Replace(valid, []byte("Help\t"), []byte("說明\t"), 1)} {
		if _, err := parseHelp(data, nil, nil, true); err == nil {
			t.Fatalf("accepted invalid Help: %q", data[:32])
		}
	}
	font := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
	for _, c := range string(valid) {
		font.Glyphs[c] = make([]byte, 32)
	}
	if _, err := parseHelp(valid, font, func(rune) bool { return false }, false); err != nil {
		t.Fatal(err)
	}
	delete(font.Glyphs, 'H')
	if _, err := parseHelp(valid, font, func(rune) bool { return false }, false); err == nil {
		t.Fatal("missing H silently accepted")
	}
}

func TestHelpRealFreezeAndResume(t *testing.T) {
	root := os.Getenv("PHANTASIE_HELP_ROOT")
	if root == "" {
		t.Skip("缺原版，SKIP不算驗收")
	}
	s, err := phantasie.StartSession(phantasie.SessionOptions{Root: root, TextDir: os.Getenv("PHANTASIE_HELP_TEXT"),
		FontDir: os.Getenv("PHANTASIE_HELP_FONT"), Langs: []string{"zh-TW", "zh-CN", "ja", "ko"}, Scratch: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if len(s.Failed) != 0 {
		t.Fatal(s.Failed)
	}
	if err := s.O.Run(6500000); err != nil {
		t.Fatal(err)
	}
	g := &game{s: s, stepsPer: 100000, theme: "original", help: loadHelpPages(s, os.Getenv("PHANTASIE_HELP_TEXT"))}
	if len(g.help) != 5 {
		t.Fatalf("five Help pages not enabled: %v", g.help)
	}
	for lang, p := range g.help {
		if p == nil || len(p.lines) != 12 {
			t.Fatal(lang)
		}
	}
	type snap struct {
		Steps, Reads   uint64
		Pending, Gated int
		Memory, VRAM   [32]byte
	}
	state := func() snap {
		return snap{s.O.Steps(), s.Gate.Reads, s.Gate.Pending(), s.Gate.Gated,
			sha256.Sum256(s.O.Bytes(oracle.Far(0, 0), 1<<20)), sha256.Sum256(s.O.Bytes(oracle.Far(0xb800, 0), 0x4000))}
	}
	// 原先排入的鍵須保留；模態內新增鍵不能排入。
	s.Gate.Press(0, "Down")
	before := state()
	if err := g.tick(frameInput{f1: true, keys: []string{"Return", "X"}}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 120; i++ {
		in := frameInput{keys: []string{"Down", "Return", "Q"}}
		if i%24 == 0 {
			in.f12 = true
		}
		if i == 60 {
			in.f12, in.shift = true, true
		}
		if err := g.tick(in); err != nil {
			t.Fatal(err)
		}
		if got := state(); got != before {
			t.Fatalf("Help changed original state at tick %d: before=%+v after=%+v", i, before, got)
		}
	}
	if !g.helpOn || g.theme != "amber" {
		t.Fatal("modal controls failed")
	}
	if err := g.tick(frameInput{esc: true, keys: []string{"Esc", "Return"}}); err != nil {
		t.Fatal(err)
	}
	if g.helpOn || state() != before {
		t.Fatal("closing Help changed original")
	}
	if err := g.tick(frameInput{}); err != nil {
		t.Fatal(err)
	}
	if s.O.Steps() <= before.Steps || s.Gate.Gated <= before.Gated {
		t.Fatal("resume did not consume existing key normally")
	}
	if err := g.tick(frameInput{f1: true}); err != nil {
		t.Fatal(err)
	}
	resume := state()
	if err := g.tick(frameInput{f1: true}); err != nil {
		t.Fatal(err)
	}
	if g.helpOn || state() != resume {
		t.Fatal("F1 close changed original")
	}
	if dir := os.Getenv("PHANTASIE_HELP_OUT"); dir != "" {
		data, _ := json.MarshalIndent(map[string]any{"result": "PASS", "modal_updates": 120, "before": before, "after_close": before, "after_resume": resume, "languages": []string{"zh-TW", "zh-CN", "en", "ja", "ko"}}, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, "freeze.json"), append(data, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
	}
}
