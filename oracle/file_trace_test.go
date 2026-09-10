package oracle

import (
	"github.com/wicanr2/dosgolem/internal/dos"
	"reflect"
	"testing"
)

// 公開軌跡不能把失敗定位與成功定位混為一談。
func TestFileOpsPreservesSeekFailure(t *testing.T) {
	o := &Oracle{d: &dos.DOS{FileOps: []dos.FileOp{
		{Step: 11, Op: "seek", Fn: 0x42, Handle: 5, Name: "DATA", Arg: -3, Pos: 17, Whence: 2},
		{Step: 12, Op: "seek", Fn: 0x42, Handle: 99, Arg: 1, Len: -6, Whence: 1, Failed: true},
	}}}
	want := []FileOp{
		{Step: 11, Op: "seek", Fn: 0x42, Handle: 5, Name: "DATA", Arg: -3, Pos: 17, Whence: 2},
		{Step: 12, Op: "seek", Fn: 0x42, Handle: 99, Arg: 1, Len: -6, Whence: 1, Failed: true},
	}
	if got := o.FileOps(); !reflect.DeepEqual(got, want) {
		t.Fatalf("檔案軌跡資訊遺失：got=%+v want=%+v", got, want)
	}
}
