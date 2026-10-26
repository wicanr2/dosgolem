//go:build !frontendlinked

package main

import (
	"fmt"
	sc "github.com/wicanr2/dosgolem/apps/strikecommander"
	"github.com/wicanr2/dosgolem/audio/mixer"
)

// This temporary overlay-only dependency guard is removed when root R4 and the
// app's actual R1/R2/R6/R8 constructor are linked. It never boots a writable
// original-tree copy and is not a renderer or normal-GUI completion claim.
func openRuntime(originalRoot, saveRoot string, mix *mixer.Mixer, fonts *sc.TextFonts, stateFile string, rebase []string) (runtimeGame, error) {
	return nil, fmt.Errorf("016 R4分根啟動與app Runtime接點尚未連接；未啟動遊戲")
}
