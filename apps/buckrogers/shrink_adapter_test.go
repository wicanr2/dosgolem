package buckrogers

// The per-commit part of the digest tests of spec 057: how the current code
// is called.  (workplace/phase324/adapter_bba48d9_test.go is the same three
// functions for dosgolem bba48d9, where the baseline constants come from.)

func adapterPlace(w *EclTextWatcher, c pre056PlaceCase) eclPlacement {
	return w.placeText(c.txt, "ecl.1", false, false, false, c.page(), c.lead, c.spaceNeeded, c.prevRune, c.row, c.col, c.entry())
}

func adapterPlayer(lang string, prof *LayoutProfile, c pre056PlayerCase) playerLayout {
	return layoutPlayerName(prof, shrinkLevelsFor(lang), c.player(), []byte(c.en), c.space, c.row, c.col, c.left, c.right, c.b)
}

func adapterLevels(lang string) []shrinkSpec { return shrinkLevelsFor(lang) }
