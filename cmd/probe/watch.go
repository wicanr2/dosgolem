package main

import (
	"fmt"
	"strconv"
	"strings"
)

// watchRange 是 -watch 的一段線性位址（含兩端）。
type watchRange struct{ lo, hi uint32 }

// parseWatchRanges 解 -watch：逗號分隔的 `<lo>-<hi>` 或單一 `<位址>`，十六進位。
//
// ⚠ **每一段都要吃進去，吃不進去就報錯。** 舊版用 `Sscanf("%x-%x")`，
// 逗號後面整串被安靜地丟掉：`-watch A-B,C-D` 只看 A-B，報告照印，
// 於是 C-D「沒有人寫」看起來是結論（`~/cht/logh3` issue #54）。
func parseWatchRanges(s string) ([]watchRange, error) {
	var out []watchRange
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		loS, hiS, isRange := strings.Cut(item, "-")
		lo, err := strconv.ParseUint(strings.TrimSpace(loS), 16, 32)
		if err != nil {
			return nil, fmt.Errorf("-watch 的 %q 不是 16 進位的 <lo>-<hi> 或 <位址>", item)
		}
		hi := lo
		if isRange {
			if hi, err = strconv.ParseUint(strings.TrimSpace(hiS), 16, 32); err != nil {
				return nil, fmt.Errorf("-watch 的 %q 不是 16 進位的 <lo>-<hi>", item)
			}
		}
		if hi < lo {
			return nil, fmt.Errorf("-watch 的 %q 上限小於下限", item)
		}
		out = append(out, watchRange{uint32(lo), uint32(hi)})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("-watch 沒有任何範圍：%q", s)
	}
	return out, nil
}

// skipRegsHit 判斷 -regs-at 這一次命中要不要跳過。只有開了 -regs-skip-blit，
// 而且 DS:SI 比同一個位置的上一筆往前 1–16 時才跳（blit 迴圈逐列走）。
// 位置完全相同的重複呼叫一律要記——那是不同的一次事件。
func skipRegsHit(skipBlit, seen bool, prev, cur uint32) bool {
	return skipBlit && seen && cur > prev && cur-prev <= 16
}
