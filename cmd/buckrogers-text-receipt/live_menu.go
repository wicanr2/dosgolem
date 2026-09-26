package main

import (
	"os"

	"github.com/wicanr2/dosgolem/apps/buckrogers"
	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/xlate"
)

// machineReader adapts the runner's machine to buckrogers.StepReader so the
// live runtime sees exactly what a sealed session.StepView would show.
type machineReader struct{ m *machine.Machine }

func (r machineReader) Steps() uint64          { return r.m.Steps }
func (r machineReader) CS() uint16             { return r.m.CPU.Seg[cpu.CS] }
func (r machineReader) IP() uint16             { return r.m.CPU.IP }
func (r machineReader) SS() uint16             { return r.m.CPU.Seg[cpu.SS] }
func (r machineReader) SP() uint16             { return r.m.CPU.R[cpu.SP] }
func (r machineReader) ES() uint16             { return r.m.CPU.Seg[cpu.ES] }
func (r machineReader) DI() uint16             { return r.m.CPU.R[cpu.DI] }
func (r machineReader) CX() uint16             { return r.m.CPU.R[cpu.CX] }
func (r machineReader) Read8(a uint32) uint8   { return r.m.Peek8(a) }
func (r machineReader) Read16(a uint32) uint16 { return r.m.Peek16(a) }
func (r machineReader) Palette() [256][3]uint8 { return r.m.Palette() }

type liveRectInput struct{ name, path string }

func liveMenuRectInputs(menu, gender, class, roster, name, career, technical string) []liveRectInput {
	return []liveRectInput{
		{"menu-text-safe-rects.tsv", menu},
		{"gender-text-safe-rects.tsv", gender},
		{"class-text-safe-rects.tsv", class},
		{"save-roster-join-text-safe-rects.tsv", roster},
		{"name-prompt-text-safe-rects.tsv", name},
		{"career-skill-screen-text-safe-rects.tsv", career},
		{"technical-skill-screen-text-safe-rects.tsv", technical},
	}
}

// newLiveMenuFromFlags loads the same rect catalogs as the runner's own menu
// presenter, so the two differ only in how they are driven.
func newLiveMenuFromFlags(catalog *buckrogers.MenuCatalog, fontPath string, inputs []liveRectInput, characterSheet string) (*buckrogers.LiveMenuRuntime, error) {
	var rectCatalogs []*buckrogers.MenuOverlayRects
	for _, input := range inputs {
		if input.path == "" {
			continue
		}
		data, err := os.ReadFile(input.path)
		if err != nil {
			return nil, err
		}
		rc, err := buckrogers.LoadMenuOverlayRects(input.name, data)
		if err != nil {
			return nil, err
		}
		rectCatalogs = append(rectCatalogs, rc)
	}
	if characterSheet != "" {
		data, err := os.ReadFile(characterSheet)
		if err != nil {
			return nil, err
		}
		rc, err := buckrogers.LoadCharacterSheetOverlayRects("character-sheet-text-safe-rects.tsv", data)
		if err != nil {
			return nil, err
		}
		rectCatalogs = append(rectCatalogs, rc)
	}
	rects, err := buckrogers.MergeMenuOverlayRects(rectCatalogs...)
	if err != nil {
		return nil, err
	}
	font, err := xlate.LoadFont(fontPath)
	if err != nil {
		return nil, err
	}
	return buckrogers.NewLiveMenuRuntime(catalog, rects, font)
}
