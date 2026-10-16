package main

import (
	"flag"
	"testing"

	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// docs/spec/441-exec-reclaim-nil-path-child-block §3 第 2 項：-exec-reclaim 預設 false，開啟後欄位為 true。
func TestExecReclaimFlag(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"-exec-reclaim"}, true},
	} {
		fs := flag.NewFlagSet("probe", flag.ContinueOnError)
		on := execReclaimFlag(fs)
		if err := fs.Parse(tc.args); err != nil {
			t.Fatal(err)
		}
		if *on != tc.want {
			t.Errorf("%v：-exec-reclaim＝%v，預期 %v", tc.args, *on, tc.want)
		}
		d := dos.New(machine.New(), "/anchor")
		if d.ExecReclaim {
			t.Fatal("dos.New 之後 ExecReclaim 應預設為 false")
		}
		d.ExecReclaim = *on // 與 main 相同的賦值
		if d.ExecReclaim != tc.want {
			t.Errorf("%v：ExecReclaim＝%v，預期 %v", tc.args, d.ExecReclaim, tc.want)
		}
	}
}
