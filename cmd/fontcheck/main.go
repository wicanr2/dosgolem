// fontcheck validates a GOLEMFNT file with dosgolem's production loader.
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"unicode/utf8"

	"github.com/wicanr2/dosgolem/xlate"
)

func main() {
	text := flag.String("text", "", "required runes; fail when any glyph is absent")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: fontcheck [-text string] font.golemfnt")
		os.Exit(2)
	}
	if !utf8.ValidString(*text) {
		fmt.Fprintln(os.Stderr, "fontcheck: -text is not valid UTF-8")
		os.Exit(2)
	}

	font, err := xlate.LoadFont(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	missingSet := make(map[rune]struct{})
	for _, r := range *text {
		if _, ok := font.Glyphs[r]; !ok {
			missingSet[r] = struct{}{}
		}
	}
	missing := make([]rune, 0, len(missingSet))
	for r := range missingSet {
		missing = append(missing, r)
	}
	sort.Slice(missing, func(i, j int) bool { return missing[i] < missing[j] })
	if len(missing) != 0 {
		for _, r := range missing {
			fmt.Fprintf(os.Stderr, "fontcheck: missing U+%04X %q\n", r, r)
		}
		os.Exit(1)
	}
	fmt.Printf("GOLEMFNT %dx%d glyphs=%d coverage=%d\n", font.W, font.H, len(font.Glyphs), utf8.RuneCountInString(*text))
}
