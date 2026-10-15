package main

import "testing"

// TestValidateDOSFiles 是 docs/spec/429-dos-sft-chain §5 第 8 項。
func TestValidateDOSFiles(t *testing.T) {
	if err := validateDOSFiles(30, "x.state"); err == nil {
		t.Error("(30, \"x.state\") 應回錯誤")
	}
	if err := validateDOSFiles(30, ""); err != nil {
		t.Errorf("(30, \"\") 不應回錯誤：%v", err)
	}
	if err := validateDOSFiles(0, "x.state"); err != nil {
		t.Errorf("(0, \"x.state\") 不應回錯誤：%v", err)
	}
}
