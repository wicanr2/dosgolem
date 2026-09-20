package main

import (
	"os/exec"
	"testing"
)

func TestUsageRequiresFontPath(t *testing.T) {
	cmd := exec.Command("go", "run", ".")
	err := cmd.Run()
	if err == nil {
		t.Fatal("fontcheck without a font path unexpectedly succeeded")
	}
}
