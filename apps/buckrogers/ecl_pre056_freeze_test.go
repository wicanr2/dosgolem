package buckrogers

// Spec 056 §5.3: the placeText method and layoutPlayerName function of dosgolem
// 72ce242, copied as they were (only the names changed).  The differential tests
// compare the current ones against them, and a digest of what they return on the
// synthetic corpus guards the copies themselves.  Nothing here may be edited.

// placeText lays out the text of one call of the ECL catalog, the engine
// catalog or the passthrough.  Every tier starts from the same cursor and
// continuation state; the layout is pure, so a failed try changes nothing
// (spec 036 §3.3).  Whether the call owes a space is decided on txt as it is;
// the leading particle mark is replaced after that (spec 054 §3.2), or the
// glue test would see a bare 이.
func (w *EclTextWatcher) placeTextPre056(txt, key string, isPlayer, fullStop, fresh bool, p *EclTextPage, lead rune, spaceNeeded bool, prevRune rune, row, col uint8, e EclTextEntry) (pl eclPlacement) {
	body := txt
	if lead != 0 {
		body, pl.marker = koResolveLeading(txt, lead)
	}
	attempt := func(t string) (ls []EclTextLine, er, ec uint8, ok bool, tier NameTier) {
		variants := []AnnotatedText{{Tier: NameTierNone, Text: []rune(t)}}
		if key != "engine" && key != "passthrough" && w.names != nil {
			variants = w.names.Variants(t, key, NameCaseUpper)
		}
		for i, v := range variants {
			if ls, er, ec, ok = layoutEclTextP(w.layout, v.Text, v.Units, row, col, eclUnitLeft(e.Left), eclUnitRight(e.Right), e.Bottom); ok {
				if i > 0 {
					tier = v.Tier
				}
				return
			}
		}
		return
	}
	done := false
	if fullStop {
		// Spec 047 §3.2: 。 is two units wide.  The layout moves a lone
		// two-unit character to the next row when one unit is left
		// instead of failing, and a full stop never starts a row, so
		// "does not fit" is a failed layout or an end row other than
		// the start row; the original text is then laid out as before.
		pl.fullStop++
		if ls, er, ec, ok, tr := attempt("。"); ok && er == row {
			pl.lines, pl.endRow, pl.endCol, pl.fits, pl.tier, done = ls, er, ec, true, tr, true
		} else {
			pl.fullStopDropped++
		}
	}
	if done {
		return
	}
	spaced := spaceNeeded && txt != "" && txt[0] != ' ' && !koGlue(prevRune, txt)
	switch {
	case spaced:
		// Spec 046 §3.4 (5): the space is never the reason a window turns
		// into English; without it the text is tried again.
		pl.lines, pl.endRow, pl.endCol, pl.fits, pl.tier = attempt(" " + body)
		if !pl.fits {
			if pl.lines, pl.endRow, pl.endCol, pl.fits, pl.tier = attempt(body); pl.fits {
				pl.spaceDropped++
			}
		}
	case !isPlayer && !fresh && w.catalog.zhDeckSpace() && zhDeckDigitCall(p, txt, col, eclUnitLeft(e.Left)):
		// Spec 048: a number right after a deck prompt (甲板) gets a
		// space.  The result counts only when the first row drawn is
		// the start row and begins with the space; a space that pushes
		// the number to the next row is dropped, as is one that does
		// not fit.
		pl.lines, pl.endRow, pl.endCol, pl.fits, pl.tier = attempt(" " + body)
		if !pl.fits || len(pl.lines) == 0 || pl.lines[0].Row != row || len(pl.lines[0].Text) == 0 || pl.lines[0].Text[0] != ' ' {
			if pl.lines, pl.endRow, pl.endCol, pl.fits, pl.tier = attempt(body); pl.fits {
				pl.spaceDropped++
			}
		}
	default:
		pl.lines, pl.endRow, pl.endCol, pl.fits, pl.tier = attempt(body)
	}
	return
}

// layoutPlayerName picks the first tier of a player-name call that fits
// (spec 038 §3.3).  With space set (the call continues text and owes a
// space, spec 045 §3.4) the order is: full with space, Chinese only with
// space, full, Chinese only, English with space, English; without it the
// first two (and the English with space) are not tried, which is the order
// spec 038 had before.  The space is a token of its own: it is not inside a
// name unit, so the units shift by one.  When nothing fits, the result is
// the last try (the plain English text) with fits false.
func layoutPlayerNamePre056(prof *LayoutProfile, player []AnnotatedText, original []byte, space bool, row, col, left, right, bottom uint8) playerLayout {
	spaced := func(v AnnotatedText) AnnotatedText {
		out := AnnotatedText{Tier: v.Tier, Text: append([]rune{' '}, v.Text...)}
		for _, u := range v.Units {
			out.Units = append(out.Units, NameUnit{u.Start + 1, u.End + 1})
		}
		return out
	}
	try := func(kind int, hasSpace bool, text []rune, units []NameUnit) (playerLayout, bool) {
		ls, er, ec, ok := layoutEclTextP(prof, text, units, row, col, left, right, bottom)
		return playerLayout{kind: kind, spaced: hasSpace, lines: ls, endRow: er, endCol: ec, fits: ok}, ok
	}
	if space {
		for i, v := range player {
			sv := spaced(v)
			if c, ok := try(i, true, sv.Text, sv.Units); ok {
				return c
			}
		}
	}
	for i, v := range player {
		if c, ok := try(i, false, v.Text, v.Units); ok {
			return c
		}
	}
	if space {
		if c, ok := try(playerEnglish, true, append([]rune{' '}, []rune(string(original))...), nil); ok {
			return c
		}
	}
	c, _ := try(playerEnglish, false, []rune(string(original)), nil)
	return c
}
