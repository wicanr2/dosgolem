package buckrogers

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash"
	"strings"
	"unicode/utf8"

	"github.com/wicanr2/dosgolem/presentation"
	"github.com/wicanr2/dosgolem/xlate"
)

const (
	manualE1Rows           = 14
	manualE1PixelTextLeft  = 48
	manualE1PixelTextRight = 912
	manualE1PixelTextTop   = 216
	manualE1PixelLineH     = 24
	manualE1PixelCJK       = 24
	manualE1PixelSpace     = 8
)

// The token and line rules below are the approved 3× E1 rules from spec 005.
func manualE1ASCIIAlpha(r rune) bool {
	return r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z'
}
func manualE1InkBounds(glyph []byte, width, height int) (minX, minY, maxX, maxY int, ok bool) {
	if len(glyph) != height*((width+7)/8) {
		return 0, 0, 0, 0, false
	}
	minX, minY, maxX, maxY = width, height, -1, -1
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if glyph[y*((width+7)/8)+x/8]&(0x80>>uint(x%8)) == 0 {
				continue
			}
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	return minX, minY, maxX, maxY, maxX >= minX
}

type manualE1PlanASCIIRun struct {
	text   string
	offset int
	width  int
}

type manualE1PlanToken struct {
	kind  string
	runes []rune
	width int
	ascii []manualE1PlanASCIIRun
}

type manualE1PlanPlacedToken struct {
	token manualE1PlanToken
	x     int
}

type manualE1PlanLine struct {
	row    int
	tokens []manualE1PlanPlacedToken
	used   int
}

func manualE1ASCIIAlnum(r rune) bool {
	return manualE1ASCIIAlpha(r) || r >= '0' && r <= '9'
}

func manualE1ASCIIConnector(r rune) bool {
	return r == '.' || r == '/' || r == '+' || r == '-'
}

func manualE1PlanWordEnd(runes []rune, start int) int {
	i := start
	for i < len(runes) && manualE1ASCIIAlnum(runes[i]) {
		i++
	}
	for i+1 < len(runes) && manualE1ASCIIConnector(runes[i]) && manualE1ASCIIAlnum(runes[i+1]) {
		i++
		for i < len(runes) && manualE1ASCIIAlnum(runes[i]) {
			i++
		}
	}
	if i < len(runes) && runes[i] == '%' {
		i++
	}
	return i
}

func manualE1PlanPairClose(r rune) (rune, bool) {
	switch r {
	case '（':
		return '）', true
	case '(':
		return ')', true
	case '「':
		return '」', true
	case '『':
		return '』', true
	case '【':
		return '】', true
	case '《':
		return '》', true
	case '〈':
		return '〉', true
	default:
		return 0, false
	}
}

func manualE1PlanIsClose(r rune) bool {
	for _, candidate := range []rune{'）', ')', '」', '』', '】', '》', '〉'} {
		if r == candidate {
			return true
		}
	}
	return false
}

func manualE1PlanTerminalPunctuation(r rune) bool {
	for _, candidate := range []rune{'、', '，', '。', '！', '？', '：', '；', '…'} {
		if r == candidate {
			return true
		}
	}
	return false
}

func manualE1PlanFindPair(runes []rune, start int) (int, error) {
	want, ok := manualE1PlanPairClose(runes[start])
	if !ok {
		return 0, fmt.Errorf("manualE1 plan: U+%04X is not an opener", runes[start])
	}
	stack := []rune{want}
	for i := start + 1; i < len(runes); i++ {
		if close, opens := manualE1PlanPairClose(runes[i]); opens {
			stack = append(stack, close)
			continue
		}
		if !manualE1PlanIsClose(runes[i]) {
			continue
		}
		if runes[i] != stack[len(stack)-1] {
			return 0, fmt.Errorf("manualE1 plan: mismatched closing U+%04X", runes[i])
		}
		stack = stack[:len(stack)-1]
		if len(stack) == 0 {
			return i, nil
		}
	}
	return 0, fmt.Errorf("manualE1 plan: unmatched opening U+%04X", runes[start])
}

func manualE1PlanNarrowGlyphAdvance(derived *xlate.Font, r rune) (int, error) {
	glyph, ok := derived.Glyphs[r]
	if !ok {
		return 0, fmt.Errorf("manualE1 plan: missing punctuation glyph U+%04X", r)
	}
	minX, _, maxX, _, ok := manualE1InkBounds(glyph, 22, 22)
	if !ok {
		return 0, fmt.Errorf("manualE1 plan: empty punctuation glyph U+%04X", r)
	}
	advance := maxX - minX + 1 + 4
	if advance < 8 {
		advance = 8
	}
	if advance > 20 {
		return 0, fmt.Errorf("buckrogers: E1 parenthesis U+%04X ink width %d exceeds safe advance", r, maxX-minX+1)
	}
	return advance, nil
}

// manualE1PlanASCIIWordWidth measures the READY E1 identifier alphabet.
func manualE1PlanASCIIWordWidth(runes []rune, advance int) (int, error) {
	if len(runes) == 0 {
		return 0, fmt.Errorf("manualE1 plan: empty ASCII identifier")
	}
	for _, r := range runes {
		if !manualE1ASCIIAlnum(r) && !manualE1ASCIIConnector(r) && r != '%' {
			return 0, fmt.Errorf("manualE1 plan: unsupported identifier rune U+%04X", r)
		}
	}
	return 16 + (len(runes)-1)*advance, nil
}

func manualE1PlanMeasure(runes []rune, derived *xlate.Font, advance int) (manualE1PlanToken, error) {
	if len(runes) == 0 {
		return manualE1PlanToken{}, fmt.Errorf("manualE1 plan: empty token")
	}
	token := manualE1PlanToken{runes: append([]rune(nil), runes...)}
	for i := 0; i < len(runes); {
		if manualE1ASCIIAlnum(runes[i]) {
			end := manualE1PlanWordEnd(runes, i)
			word := runes[i:end]
			width, err := manualE1PlanASCIIWordWidth(word, advance)
			if err != nil {
				return manualE1PlanToken{}, err
			}
			token.ascii = append(token.ascii, manualE1PlanASCIIRun{text: string(word), offset: token.width, width: width})
			token.width += width
			i = end
			continue
		}
		switch runes[i] {
		case ' ':
			token.width += manualE1PixelSpace
		case '（', '）', '(', ')':
			width, err := manualE1PlanNarrowGlyphAdvance(derived, runes[i])
			if err != nil {
				return manualE1PlanToken{}, err
			}
			token.width += width
		default:
			if manualE1ASCIIConnector(runes[i]) || runes[i] == '%' {
				return manualE1PlanToken{}, fmt.Errorf("buckrogers: E1 connector U+%04X is outside an identifier", runes[i])
			}
			token.width += manualE1PixelCJK
		}
		i++
	}
	return token, nil
}

func manualE1PlanAppendTerminal(previous *manualE1PlanToken, punctuation manualE1PlanToken, derived *xlate.Font, advance int) error {
	if previous == nil {
		return fmt.Errorf("manualE1 plan: punctuation cannot begin paragraph")
	}
	combined := append(append([]rune(nil), previous.runes...), punctuation.runes...)
	measured, err := manualE1PlanMeasure(combined, derived, advance)
	if err != nil {
		return err
	}
	measured.kind = "cluster"
	*previous = measured
	return nil
}

func manualE1PlanAppendPrefix(next *manualE1PlanToken, prefix []rune, derived *xlate.Font, advance int) error {
	if next == nil {
		return fmt.Errorf("manualE1 plan: opening bracket has no following token")
	}
	combined := append(append([]rune(nil), prefix...), next.runes...)
	measured, err := manualE1PlanMeasure(combined, derived, advance)
	if err != nil {
		return err
	}
	measured.kind = "cluster"
	*next = measured
	return nil
}

func manualE1PlanShortBracketIdentifier(runes []rune) bool {
	if len(runes) < 3 || (runes[0] != '(' && runes[0] != '（') {
		return false
	}
	if want, ok := manualE1PlanPairClose(runes[0]); !ok || runes[len(runes)-1] != want {
		return false
	}
	return manualE1PlanWordEnd(runes[1:len(runes)-1], 0) == len(runes)-2
}

// manualE1PlanTokens makes bracket pairs and ASCII identifiers atomic. Terminal
// Chinese punctuation is folded into its preceding token so it cannot become a
// line-start orphan; unmatched brackets are rejected before a plan exists.
func manualE1PlanTokens(text string, derived *xlate.Font, advance int) ([]manualE1PlanToken, error) {
	if text == "" {
		return nil, fmt.Errorf("manualE1 plan: empty translation")
	}
	runes := []rune(text)
	if runes[0] == ' ' || runes[len(runes)-1] == ' ' {
		return nil, fmt.Errorf("manualE1 plan: source text has leading or trailing ASCII space")
	}
	for i := 1; i < len(runes); i++ {
		if runes[i-1] == ' ' && runes[i] == ' ' {
			return nil, fmt.Errorf("manualE1 plan: source text has consecutive ASCII spaces")
		}
	}
	tokens := make([]manualE1PlanToken, 0, len(runes))
	for i := 0; i < len(runes); {
		var segment []rune
		kind := "cjk"
		if _, opens := manualE1PlanPairClose(runes[i]); opens {
			end, err := manualE1PlanFindPair(runes, i)
			if err != nil {
				return nil, err
			}
			pair := runes[i : end+1]
			if manualE1PlanShortBracketIdentifier(pair) {
				segment, kind, i = pair, "paired_identifier", end+1
			} else {
				inner, err := manualE1PlanTokens(string(pair[1:len(pair)-1]), derived, advance)
				if err != nil {
					return nil, err
				}
				if len(inner) == 0 {
					segment, kind, i = pair, "paired_empty", end+1
				} else {
					if err := manualE1PlanAppendPrefix(&inner[0], pair[:1], derived, advance); err != nil {
						return nil, err
					}
					close, err := manualE1PlanMeasure(pair[len(pair)-1:], derived, advance)
					if err != nil {
						return nil, err
					}
					if err := manualE1PlanAppendTerminal(&inner[len(inner)-1], close, derived, advance); err != nil {
						return nil, err
					}
					tokens = append(tokens, inner...)
					i = end + 1
					continue
				}
			}
		} else if manualE1PlanIsClose(runes[i]) {
			return nil, fmt.Errorf("manualE1 plan: unmatched closing U+%04X", runes[i])
		} else if manualE1ASCIIAlnum(runes[i]) {
			end := manualE1PlanWordEnd(runes, i)
			segment, kind, i = runes[i:end], "identifier", end
		} else {
			if manualE1ASCIIConnector(runes[i]) || runes[i] == '%' {
				return nil, fmt.Errorf("buckrogers: E1 connector U+%04X is outside an identifier", runes[i])
			}
			segment, i = runes[i:i+1], i+1
			if segment[0] == ' ' {
				kind = "space"
			} else if manualE1PlanTerminalPunctuation(segment[0]) {
				kind = "terminal"
			}
		}
		token, err := manualE1PlanMeasure(segment, derived, advance)
		if err != nil {
			return nil, err
		}
		token.kind = kind
		if kind == "terminal" {
			if len(tokens) == 0 {
				return nil, fmt.Errorf("manualE1 plan: terminal punctuation begins paragraph")
			}
			if err := manualE1PlanAppendTerminal(&tokens[len(tokens)-1], token, derived, advance); err != nil {
				return nil, err
			}
			continue
		}
		tokens = append(tokens, token)
	}
	return tokens, nil
}

func manualE1PlanLines(text string, derived *xlate.Font, advance int) ([]manualE1PlanLine, error) {
	tokens, err := manualE1PlanTokens(text, derived, advance)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("manualE1 plan: no tokens")
	}
	lines := []manualE1PlanLine{{row: 0}}
	cursor := manualE1PixelTextLeft
	for _, original := range tokens {
		token := original
		// Boundary ASCII spaces remain in the token stream for exact rune
		// round-trip, but have no ink or advance at a physical line boundary.
		if token.kind == "space" && cursor == manualE1PixelTextLeft {
			token.width = 0
		}
		if token.width > manualE1PixelTextRight-manualE1PixelTextLeft {
			return nil, fmt.Errorf("manualE1 plan: token %q exceeds text width", string(token.runes))
		}
		if cursor+token.width > manualE1PixelTextRight {
			if len(lines) == manualE1Rows {
				return nil, fmt.Errorf("manualE1 plan: paragraph exceeds %d rows", manualE1Rows)
			}
			previous := &lines[len(lines)-1]
			for i := len(previous.tokens) - 1; i >= 0 && previous.tokens[i].token.kind == "space"; i-- {
				cursor -= previous.tokens[i].token.width
				previous.tokens[i].token.width = 0
			}
			previous.used = cursor - manualE1PixelTextLeft
			lines = append(lines, manualE1PlanLine{row: len(lines)})
			cursor = manualE1PixelTextLeft
			if token.kind == "space" {
				token.width = 0
			}
		}
		lines[len(lines)-1].tokens = append(lines[len(lines)-1].tokens, manualE1PlanPlacedToken{token: token, x: cursor})
		cursor += token.width
	}
	lastLine := &lines[len(lines)-1]
	for i := len(lastLine.tokens) - 1; i >= 0 && lastLine.tokens[i].token.kind == "space"; i-- {
		cursor -= lastLine.tokens[i].token.width
		lastLine.tokens[i].token.width = 0
	}
	lastLine.used = cursor - manualE1PixelTextLeft
	for _, line := range lines {
		if len(line.tokens) == 0 || line.used > manualE1PixelTextRight-manualE1PixelTextLeft {
			return nil, fmt.Errorf("manualE1 plan: empty or oversized line")
		}
		firstIndex, lastIndex := 0, len(line.tokens)-1
		for firstIndex <= lastIndex && line.tokens[firstIndex].token.kind == "space" {
			if line.tokens[firstIndex].token.width != 0 {
				return nil, fmt.Errorf("manualE1 plan: visible line-start ASCII space")
			}
			firstIndex++
		}
		for lastIndex >= firstIndex && line.tokens[lastIndex].token.kind == "space" {
			if line.tokens[lastIndex].token.width != 0 {
				return nil, fmt.Errorf("manualE1 plan: visible line-end ASCII space")
			}
			lastIndex--
		}
		if firstIndex > lastIndex {
			return nil, fmt.Errorf("manualE1 plan: line contains only boundary spaces")
		}
		first := line.tokens[firstIndex].token.runes[0]
		lastRunes := line.tokens[lastIndex].token.runes
		last := lastRunes[len(lastRunes)-1]
		if manualE1PlanIsClose(first) || manualE1PlanTerminalPunctuation(first) {
			return nil, fmt.Errorf("manualE1 plan: forbidden line-start punctuation U+%04X", first)
		}
		if _, opens := manualE1PlanPairClose(last); opens {
			return nil, fmt.Errorf("manualE1 plan: forbidden line-end opening U+%04X", last)
		}
	}
	return lines, nil
}
func manualE1PlanValidateTokenPixels(lines []manualE1PlanLine, source *xlate.Font, advance int) (int, error) {
	maxInk := 0
	for _, line := range lines {
		for _, placed := range line.tokens {
			if placed.x < manualE1PixelTextLeft || placed.x+placed.token.width > manualE1PixelTextRight {
				return 0, fmt.Errorf("manualE1 plan: token outside text rect")
			}

			previousMax := -1
			for _, run := range placed.token.ascii {
				for i, r := range []rune(run.text) {
					glyph, ok := source.Glyphs[r]
					if !ok {
						return 0, fmt.Errorf("manualE1 plan: missing ASCII glyph U+%04X", r)
					}
					minX, _, maxX, _, hasInk := manualE1InkBounds(glyph, 16, 16)
					if !hasInk {
						return 0, fmt.Errorf("manualE1 plan: empty ASCII glyph U+%04X", r)
					}
					inkWidth := maxX - minX + 1
					if inkWidth > maxInk {
						maxInk = inkWidth
					}
					left, right := placed.x+run.offset+i*advance+minX, placed.x+run.offset+i*advance+maxX
					if left < placed.x+run.offset || right >= placed.x+run.offset+run.width {
						return 0, fmt.Errorf("manualE1 plan: glyph U+%04X escapes token pixel range", r)
					}
					if previousMax >= left {
						return 0, fmt.Errorf("manualE1 plan: 14px cross-run ink collision in token %q", string(placed.token.runes))
					}
					previousMax = right
				}
			}
		}
	}
	return maxInk, nil
}

// ManualE1Plan is an output-only, privately held value plan. Its slices are
// copied from the tokenizer and never exposed to callers.
type ManualE1Plan struct {
	generation                     uint64
	eventKey, textKey, translation string
	runeCount                      int
	baseName, derivedName          string
	baseSeal, derivedSeal          [sha256.Size]byte
	lines                          [manualE1Rows]manualE1SealedLine
	digest                         [sha256.Size]byte
}

type manualE1SealedLine struct {
	row, y, used int
	tokens       []manualE1SealedToken
}

type manualE1SealedToken struct {
	kind       string
	start, end int
	runes      []rune
	x, advance int
	glyphs     []manualE1Glyph
}

type manualE1Glyph struct {
	r                      rune
	role, name             string
	seal                   [sha256.Size]byte
	srcX, srcY, srcW, srcH int
	x, y                   int
}

// BuildManualE1Plan accepts one exact catalog request and the already verified
// session fonts. It does not modify either font or any existing presenter.
func BuildManualE1Plan(layout *ManualOverlayLayout, catalog *Catalog, request DisplayRequest, base, derived *xlate.Font) (*ManualE1Plan, error) {
	if !layout.confirmed() || request.Generation == 0 || request.EventKey == "" || request.TextKey == "" ||
		request.Translation == "" || !utf8.ValidString(request.Translation) ||
		catalog == nil || !catalog.containsManualRequest(request) {
		return nil, fmt.Errorf("buckrogers: invalid E1 manual request")
	}
	if base == nil || derived == nil || base.W != 16 || base.H != 16 || derived.W != 22 || derived.H != 22 ||
		base.Name == "" || derived.Name == "" || base.Name == derived.Name {
		return nil, fmt.Errorf("buckrogers: E1 requires distinct named 16x16 and 22x22 fonts")
	}
	baseSeal, err := presentation.FontFingerprint(base)
	if err != nil {
		return nil, err
	}
	derivedSeal, err := presentation.FontFingerprint(derived)
	if err != nil {
		return nil, err
	}
	lines, err := manualE1PlanLines(request.Translation, derived, 14)
	if err != nil {
		return nil, err
	}
	if _, err := manualE1PlanValidateTokenPixels(lines, base, 14); err != nil {
		return nil, err
	}
	plan := &ManualE1Plan{
		generation: request.Generation, eventKey: request.EventKey, textKey: request.TextKey,
		translation: request.Translation, runeCount: utf8.RuneCountInString(request.Translation),
		baseName: base.Name, derivedName: derived.Name, baseSeal: baseSeal, derivedSeal: derivedSeal,
	}
	for row := range plan.lines {
		plan.lines[row].row, plan.lines[row].y = row, manualE1PixelTextTop+row*manualE1PixelLineH
	}
	source := 0
	for row, line := range lines {
		sealed := &plan.lines[row]
		sealed.used = line.used
		for _, placed := range line.tokens {
			runes := append([]rune(nil), placed.token.runes...)
			token := manualE1SealedToken{
				kind: placed.token.kind, start: source, end: source + len(runes),
				runes: runes, x: placed.x, advance: placed.token.width,
			}
			source = token.end
			cursor := token.x
			for i := 0; i < len(runes); {
				r := runes[i]
				if manualE1ASCIIAlnum(r) {
					end := manualE1PlanWordEnd(runes, i)
					word := runes[i:end]
					width, err := manualE1PlanASCIIWordWidth(word, 14)
					if err != nil {
						return nil, err
					}
					for n, ascii := range word {
						if _, ok := base.Glyphs[ascii]; !ok {
							return nil, fmt.Errorf("buckrogers: missing E1 ASCII glyph U+%04X", ascii)
						}
						token.glyphs = append(token.glyphs, manualE1Glyph{r: ascii, role: "base16", name: base.Name, seal: baseSeal, srcW: 16, srcH: 16, x: cursor + n*14, y: sealed.y + 4})
					}
					cursor += width
					i = end
					continue
				}
				if r == ' ' {
					if token.advance != 0 {
						cursor += manualE1PixelSpace
					}
					i++
					continue
				}
				glyph, ok := derived.Glyphs[r]
				if !ok || len(glyph) != 22*3 {
					return nil, fmt.Errorf("buckrogers: missing E1 derived glyph U+%04X", r)
				}
				advance, srcX, srcY, srcW, srcH, x, y := 24, 0, 0, 22, 22, cursor+1, sealed.y+1
				if r == '（' || r == '）' || r == '(' || r == ')' {
					minX, minY, maxX, maxY, ink := manualE1InkBounds(glyph, 22, 22)
					if !ink || maxX-minX+1+4 > 20 {
						return nil, fmt.Errorf("buckrogers: E1 parenthesis U+%04X exceeds safe advance", r)
					}
					srcX, srcY, srcW, srcH = minX, minY, maxX-minX+1, maxY-minY+1
					advance, err = manualE1PlanNarrowGlyphAdvance(derived, r)
					if err != nil {
						return nil, err
					}
					x, y = cursor+(advance-srcW)/2, sealed.y+1+srcY
				}
				token.glyphs = append(token.glyphs, manualE1Glyph{r: r, role: "derived22", name: derived.Name, seal: derivedSeal,
					srcX: srcX, srcY: srcY, srcW: srcW, srcH: srcH, x: x, y: y})
				cursor += advance
				i++
			}
			if cursor != token.x+token.advance {
				return nil, fmt.Errorf("buckrogers: E1 token measurement changed")
			}
			sealed.tokens = append(sealed.tokens, token)
		}
	}
	if source != plan.runeCount {
		return nil, fmt.Errorf("buckrogers: E1 source span incomplete")
	}
	plan.digest = plan.hash()
	if _, err := plan.TextLayer(base, derived); err != nil {
		return nil, err
	}
	return plan, nil
}

func manualE1U32(h hash.Hash, value int) {
	var raw [4]byte
	binary.LittleEndian.PutUint32(raw[:], uint32(value))
	_, _ = h.Write(raw[:])
}

func manualE1String(h hash.Hash, value string) {
	manualE1U32(h, len(value))
	_, _ = h.Write([]byte(value))
}

func (p *ManualE1Plan) hash() [sha256.Size]byte {
	h := sha256.New()
	_, _ = h.Write([]byte("buckrogers-manual-e1-layout-v1\x00"))
	for _, v := range []int{3, 14, 21, 216, 915, 336, 48, 216, 864, 336, manualE1Rows} {
		manualE1U32(h, v)
	}
	for _, line := range p.lines {
		for _, v := range []int{line.row, line.y, line.used, len(line.tokens)} {
			manualE1U32(h, v)
		}
		for _, token := range line.tokens {
			manualE1String(h, token.kind)
			for _, v := range []int{token.start, token.end, token.x, token.advance, len(token.runes)} {
				manualE1U32(h, v)
			}
			for _, r := range token.runes {
				manualE1U32(h, int(r))
			}
			manualE1U32(h, len(token.glyphs))
			for _, glyph := range token.glyphs {
				manualE1U32(h, int(glyph.r))
				manualE1String(h, glyph.role)
				manualE1String(h, glyph.name)
				_, _ = h.Write(glyph.seal[:])
				for _, v := range []int{glyph.srcX, glyph.srcY, glyph.srcW, glyph.srcH, glyph.x, glyph.y} {
					manualE1U32(h, v)
				}
			}
		}
	}
	var generation [8]byte
	binary.LittleEndian.PutUint64(generation[:], p.generation)
	_, _ = h.Write(generation[:])
	manualE1String(h, p.eventKey)
	manualE1String(h, p.textKey)
	manualE1U32(h, p.runeCount)
	manualE1String(h, p.translation)
	manualE1String(h, p.baseName)
	_, _ = h.Write(p.baseSeal[:])
	manualE1String(h, p.derivedName)
	_, _ = h.Write(p.derivedSeal[:])
	var out [sha256.Size]byte
	copy(out[:], h.Sum(nil))
	return out
}

// TextLayer creates an independent 14-row text layer after rechecking the
// plan's source, hash, font identity, and every physical glyph boundary.
func (p *ManualE1Plan) TextLayer(base, derived *xlate.Font) (*xlate.Layer, error) {
	if p == nil || p.generation == 0 || p.eventKey == "" || p.textKey == "" || p.translation == "" ||
		p.runeCount != utf8.RuneCountInString(p.translation) || p.digest != p.hash() ||
		base == nil || derived == nil || base.Name != p.baseName || derived.Name != p.derivedName || base.Name == derived.Name {
		return nil, fmt.Errorf("buckrogers: invalid or changed E1 plan")
	}
	baseSeal, err := presentation.FontFingerprint(base)
	if err != nil || baseSeal != p.baseSeal {
		return nil, fmt.Errorf("buckrogers: E1 base font identity mismatch")
	}
	derivedSeal, err := presentation.FontFingerprint(derived)
	if err != nil || derivedSeal != p.derivedSeal {
		return nil, fmt.Errorf("buckrogers: E1 derived font identity mismatch")
	}
	fonts := map[string]*xlate.Font{base.Name: base, derived.Name: derived}
	layer := &xlate.Layer{W: 320, H: 200, FontRegistry: fonts}
	var source strings.Builder
	nextSource := 0
	for row, line := range p.lines {
		if line.row != row || line.y != manualE1PixelTextTop+row*manualE1PixelLineH || line.used < 0 || line.used > 864 {
			return nil, fmt.Errorf("buckrogers: E1 row geometry mismatch")
		}
		stamp := &xlate.Stamp{Key: fmt.Sprintf("manual.e1.%d.%s.%02d", p.generation, p.textKey, row),
			X: 16, Y: 72 + row*8, Cells: 36, CellW: 8, CellH: 8, State: xlate.Pending}
		cursor := manualE1PixelTextLeft
		for _, token := range line.tokens {
			if token.start != nextSource || token.end-token.start != len(token.runes) || token.x != cursor ||
				token.advance < 0 || token.x+token.advance > manualE1PixelTextRight ||
				(token.kind == "space" && token.advance == 0 && len(token.glyphs) != 0) {
				return nil, fmt.Errorf("buckrogers: E1 token source or geometry mismatch")
			}
			nextSource = token.end
			source.WriteString(string(token.runes))
			for _, glyph := range token.glyphs {
				font := fonts[glyph.name]
				if font == nil || glyph.role == "base16" && font != base || glyph.role == "derived22" && font != derived ||
					glyph.role != "base16" && glyph.role != "derived22" {
					return nil, fmt.Errorf("buckrogers: E1 glyph font role mismatch")
				}
				seal := p.derivedSeal
				if font == base {
					seal = p.baseSeal
				}
				if glyph.seal != seal || glyph.x < token.x || glyph.x+glyph.srcW > token.x+token.advance {
					return nil, fmt.Errorf("buckrogers: E1 glyph seal or token bounds mismatch")
				}
				stamp.PixelGlyphs = append(stamp.PixelGlyphs, xlate.PixelGlyph{Rune: glyph.r, Font: font,
					SrcX: glyph.srcX, SrcY: glyph.srcY, SrcW: glyph.srcW, SrcH: glyph.srcH, X: glyph.x, Y: glyph.y})
			}
			cursor += token.advance
		}
		if cursor-manualE1PixelTextLeft != line.used {
			return nil, fmt.Errorf("buckrogers: E1 line used width mismatch")
		}
		if len(line.tokens) != 0 {
			if len(stamp.PixelGlyphs) == 0 {
				return nil, fmt.Errorf("buckrogers: E1 visible row has no glyphs")
			}
			stamp.PixelScale = 3
		}
		layer.Stamps = append(layer.Stamps, stamp)
	}
	if nextSource != p.runeCount || source.String() != p.translation {
		return nil, fmt.Errorf("buckrogers: E1 source does not round trip")
	}
	if err := layer.ValidatePixelGlyphPlan(3); err != nil {
		return nil, err
	}
	return layer, nil
}

// CanonicalHash returns only the sealed digest; it does not expose plan slices.
func (p *ManualE1Plan) CanonicalHash() [sha256.Size]byte {
	if p == nil {
		return [sha256.Size]byte{}
	}
	return p.digest
}
