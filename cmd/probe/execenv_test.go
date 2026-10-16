package main

import (
	"flag"
	"testing"

	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// docs/spec/439-exec-child-environment §3 第 2 項：-exec-env 預設 false，開啟後欄位為 true。
func TestExecEnvFlag(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"-exec-env"}, true},
	} {
		fs := flag.NewFlagSet("probe", flag.ContinueOnError)
		on := execEnvFlag(fs)
		if err := fs.Parse(tc.args); err != nil {
			t.Fatal(err)
		}
		if *on != tc.want {
			t.Errorf("%v：-exec-env＝%v，預期 %v", tc.args, *on, tc.want)
		}
		d := dos.New(machine.New(), "/anchor")
		if d.ExecEnvCopy {
			t.Fatal("dos.New 之後 ExecEnvCopy 應預設為 false")
		}
		d.ExecEnvCopy = *on // 與 main 相同的賦值
		if d.ExecEnvCopy != tc.want {
			t.Errorf("%v：ExecEnvCopy＝%v，預期 %v", tc.args, d.ExecEnvCopy, tc.want)
		}
	}
}
