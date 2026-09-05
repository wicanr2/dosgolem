#!/usr/bin/env bash
# 本機 CI。**送 MR 之前這一支要全綠。**
#
#   tools/ci.sh
#   BASE=origin/master tools/ci.sh     # 換一個比較基準
#
# 全部跑在 docker 裡（走 tools/go.sh），不裝任何東西到系統環境。
#
# gofmt 只檢查「這個分支動過的檔案」。整個 repo 對 gofmt 並不乾淨——
# 那是 CJK 註解對齊的既有分歧，跟這個分支無關；一次全格式化會把 MR
# 埋進幾百行雜訊裡，反而看不到真正的改動。
set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

BASE="${BASE:-origin/master}"
fail=0
step() { printf '\n\033[1m=== %s\033[0m\n' "$1"; }
bad() { echo "  ✗ $1"; fail=1; }

step "gofmt（本分支動過的 .go）"
mapfile -t changed < <(git diff --name-only --diff-filter=d "$BASE"...HEAD -- '*.go' 2>/dev/null)
mapfile -t untracked < <(git ls-files --others --exclude-standard -- '*.go')
# 已經刪掉的檔案不能餵給 gofmt——分支裡改過又刪掉的檔會落進 diff 清單。
files=()
for f in "${changed[@]}" "${untracked[@]}"; do
  [[ -f "$f" ]] && files+=("$f")
done
if [[ ${#files[@]} -eq 0 ]]; then
  echo "  （沒有動過任何 .go）"
else
  out=$(DOSGOLEM_GO_CMD=gofmt tools/go.sh -l "${files[@]}" 2>&1)
  if [[ -n "$out" ]]; then
    bad "沒有格式化："
    echo "$out" | sed 's/^/      /'
  else
    echo "  ✓ ${#files[@]} 個檔案"
  fi
fi

step "go vet"
tools/go.sh vet ./... && echo "  ✓" || bad "go vet 有問題"

step "go test"
tools/go.sh test ./... || bad "測試沒過"

step "CPU 語料（SingleStepTests）"
if [[ -d testdata/8088/v2 ]]; then
  tools/go.sh test ./internal/cpu -run TestSingleStep && echo "  ✓" || bad "語料沒全綠"
else
  echo "  跳過：testdata/8088/v2 不在。用 tools/fetch_cputests.sh 抓。"
  echo "  ⚠ CPU 的驗收判準是「全部通過」，跳過不等於通過。"
fi

printf '\n'
if [[ $fail -eq 0 ]]; then
  echo "全部通過。"
else
  echo "有項目沒過，見上面的 ✗。"
fi
exit $fail
