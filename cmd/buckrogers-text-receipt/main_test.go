package main

import "testing"

func TestMenuCatalogFlagsArePaired(t *testing.T) {
	for _, tc := range []struct {
		events, translations string
		valid                bool
	}{
		{"", "", true},
		{"events.tsv", "translations.tsv", true},
		{"events.tsv", "", false},
		{"", "translations.tsv", false},
	} {
		if got := validateMenuCatalogFlags(tc.events, tc.translations) == nil; got != tc.valid {
			t.Fatalf("events=%q translations=%q valid=%v，要 %v", tc.events, tc.translations, got, tc.valid)
		}
	}
}
