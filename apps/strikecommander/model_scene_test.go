package strikecommander

import (
	"testing"

	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
)

func TestModelSceneRetainsGroupFailureBoundary(t *testing.T) {
	for _, tc := range []struct {
		name       string
		invalidate func(*ModelGroup)
	}{
		{"valid", nil},
		{"not-replay-ready", func(g *ModelGroup) { g.ReplayReady = false }},
		{"read-failed", func(g *ModelGroup) { g.ReadFailed = true }},
		{"unmatched-frame", func(g *ModelGroup) { g.UnmatchedFrames = 1 }},
		{"camera-read-failed", func(g *ModelGroup) { g.CameraReadFailed[0] = true }},
		{"camera-results-missing", func(g *ModelGroup) { g.CameraReadFailed = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := &hd3d.Result{Valid: true, Checked: true, Repaintable: true}
			g := &ModelGroup{ReplayReady: true, Raster: original, Cameras: []realspace.Camera{{}}, CameraReadFailed: []bool{false}}
			if tc.invalidate != nil {
				tc.invalidate(g)
			}
			in := g.SceneInput()
			if (in.Original == nil) != (tc.invalidate != nil) {
				t.Fatal("group failure did not gate C evidence")
			}
			if g.Raster != original || !original.Valid || !original.Checked || !original.Repaintable {
				t.Fatal("adapter modified the sealed replay")
			}
		})
	}
}
