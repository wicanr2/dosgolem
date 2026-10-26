//go:build frontendlinked

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	sc "github.com/wicanr2/dosgolem/apps/strikecommander"
	"github.com/wicanr2/dosgolem/apps/strikecommander/player"
	"github.com/wicanr2/dosgolem/session"
	"os"
	"path/filepath"
)

// This file uses the READY463 API only in the explicit temporary frontendlinked
// source combination. The exact root implementation is recorded by the build
// source freeze; no original files are copied into a writable SaveRoot for boot.
func bootFrontendSession(obs session.StepObserver, originalRoot, saveRoot, stateFile string, rebase []string) (*session.Owner, error) {
	owner, e := session.New(sc.SessionConfig(obs))
	if e != nil {
		return nil, e
	}
	fail := func(err error) (*session.Owner, error) { owner.Close(); return nil, err }
	if stateFile != "" {
		st, e := os.Stat(stateFile)
		if e != nil {
			return fail(e)
		}
		if st.Size() <= 0 || st.Size() > 64<<20 {
			return fail(fmt.Errorf("state file size must be 1..64MiB"))
		}
		raw, e := os.ReadFile(stateFile)
		if e != nil {
			return fail(e)
		}
		if _, e = owner.BootFromState(session.StateInput{State: raw, ExpectedStateSHA256: sha256.Sum256(raw), OriginalRoot: originalRoot, SaveRoot: saveRoot, RebaseFrom: append([]string(nil), rebase...)}); e != nil {
			return fail(e)
		}
		return owner, nil
	}
	var expected [32]byte
	found := false
	for _, f := range player.RequiredFiles {
		if f.Name == "SCCD.EXE" {
			b, e := hex.DecodeString(f.SHA256)
			if e != nil {
				return fail(e)
			}
			copy(expected[:], b)
			found = true
			break
		}
	}
	if !found {
		return fail(fmt.Errorf("missing fixed SCCD.EXE identity"))
	}
	exe, e := os.ReadFile(filepath.Join(originalRoot, "SCCD.EXE"))
	if e != nil {
		return fail(e)
	}
	if _, e = owner.BootOriginal(session.BootInput{EXE: exe, ExpectedEXESHA256: expected, OriginalRoot: originalRoot, SaveRoot: saveRoot}); e != nil {
		return fail(e)
	}
	return owner, nil
}
