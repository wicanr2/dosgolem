package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/wicanr2/dosgolem/apps/buckrogers"
)

type bodyIconOverlayJSON struct {
	Scale                  int      `json:"scale"`
	ActiveKeys             []string `json:"active_keys"`
	MissingGlyphs          []string `json:"missing_glyphs"`
	Drew                   bool     `json:"drew"`
	BaselineSHA256         string   `json:"baseline_rgba_sha256"`
	OverlaySHA256          string   `json:"overlay_rgba_sha256"`
	DiffOutsideSafeRects   int      `json:"diff_outside_safe_rects"`
	DiffInsideSafeRects    int      `json:"diff_inside_safe_rects"`
	DynamicIconDiff        int      `json:"dynamic_icon_diff"`
	AddedNonBaselinePixels int      `json:"added_nonbaseline_pixels"`
}

type bodyIconOverlaySampleJSON struct {
	Step            uint64              `json:"step"`
	Kind            string              `json:"kind"`
	Generation      uint64              `json:"generation"`
	Group           string              `json:"group,omitempty"`
	InvalidatedKeys []string            `json:"invalidated_keys,omitempty"`
	Metrics         bodyIconOverlayJSON `json:"metrics"`
}

func makeBodyIconOverlayReceipt(p *buckrogers.RuntimeBodyIconOverlay, indexed []byte, palette [256][3]uint8, baselinePath, overlayPath string) (*bodyIconOverlayJSON, error) {
	r, baseline, overlay, err := inspectBodyIconOverlay(p, indexed, palette)
	if err != nil {
		return nil, err
	}
	if baselinePath != "" {
		if err := os.WriteFile(baselinePath, baseline, 0600); err != nil {
			return nil, err
		}
	}
	if overlayPath != "" {
		if err := os.WriteFile(overlayPath, overlay, 0600); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func inspectBodyIconOverlay(p *buckrogers.RuntimeBodyIconOverlay, indexed []byte, palette [256][3]uint8) (*bodyIconOverlayJSON, []byte, []byte, error) {
	if p == nil {
		return nil, nil, nil, fmt.Errorf("body icon presenter missing")
	}
	baseline := buckrogers.ScaleIndexedRGBA(indexed, palette, p.Scale())
	overlay, missing, drew := p.Draw(indexed, palette)
	if len(overlay) != len(baseline) {
		return nil, nil, nil, fmt.Errorf("body icon RGBA length mismatch")
	}
	r := &bodyIconOverlayJSON{Scale: p.Scale(), ActiveKeys: p.ActiveKeys(), MissingGlyphs: make([]string, len(missing)), Drew: drew, BaselineSHA256: hashBodyRGBA(baseline), OverlaySHA256: hashBodyRGBA(overlay)}
	for i, g := range missing {
		r.MissingGlyphs[i] = fmt.Sprintf("U+%04X", g)
	}
	allowed := p.SafeRects()
	w := 320 * p.Scale()
	for i := 0; i < len(baseline); i += 4 {
		if string(baseline[i:i+4]) == string(overlay[i:i+4]) {
			continue
		}
		pixel := i / 4
		x, y := pixel%w, pixel/w
		inside := false
		for _, rect := range allowed {
			if x >= rect.X && x < rect.X+rect.Width && y >= rect.Y && y < rect.Y+rect.Height {
				inside = true
				break
			}
		}
		if inside {
			r.DiffInsideSafeRects++
		} else {
			r.DiffOutsideSafeRects++
		}
		if overlay[i] != baseline[i] || overlay[i+1] != baseline[i+1] || overlay[i+2] != baseline[i+2] {
			r.AddedNonBaselinePixels++
		}
	}
	r.DynamicIconDiff = r.DiffOutsideSafeRects
	if r.DiffOutsideSafeRects != 0 || r.DynamicIconDiff != 0 {
		return nil, nil, nil, fmt.Errorf("body icon overlay escaped text safe rectangles")
	}
	return r, baseline, overlay, nil
}
func hashBodyRGBA(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
