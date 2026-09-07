#!/usr/bin/env bash
# 本機 CI：**收工前這一支要全綠。**
#
#   tools/ci.sh
#   DOSGOLEM_CPUS=2 tools/ci.sh    # 這台機器上還有別人在跑時
#
# 全部跑在 docker 裡（走 tools/go.sh），不裝任何東西到系統環境。
#
# 四步，每一步各自回報，最後一行才是結論。**跳過不等於通過**——
# 缺語料的那一步會明講自己沒驗到什麼。
set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

fail=0
step() { printf '\n\033[1m=== %s\033[0m\n' "$1"; }
bad() { echo "  ✗ $1"; fail=1; }

# ---- 1. 格式 -------------------------------------------------------------
# 整個 repo 都要乾淨。十條分支合併的時候順手格式化過一輪，所以這裡不必
# 再只挑「本分支動過的檔」——那個做法會在合併進 master 之後變成「零個檔案」，
# 於是這一步永遠是綠的而且什麼都沒檢查。
step "gofmt（整個 repo）"
out=$(DOSGOLEM_GO_CMD=gofmt tools/go.sh -l . 2>&1)
if [[ -n "$out" ]]; then
  bad "沒有格式化："
  echo "$out" | sed 's/^/      /'
else
  echo "  ✓"
fi

# ---- 2. 靜態檢查 ---------------------------------------------------------
step "go vet"
tools/go.sh vet ./... && echo "  ✓" || bad "go vet 有問題"

# ---- 3. 單元測試（不含語料）----------------------------------------------
# **語料留給下一步**：`./...` 已經含 internal/cpu，不加 `-short` 的話同一份
# 727 MB 語料會在這一輪跑兩次（實測序列 182–287 秒一次），一輪 CI 平白多花
# 三分多鐘做同一件事。
step "go test（不含 CPU 語料）"
tools/go.sh test -short ./... || bad "測試沒過"

# ---- 4. CPU 語料 ---------------------------------------------------------
# 判準是**全部通過**（`docs/spec/002` §5）：CPU 的錯不會報錯，只會讓上層
# 在幾百萬道指令之後畫錯一個像素。
#
# 路徑要與 `internal/cpu/singlestep_test.go` 的 testDir 一致
# （`testdata/8088/`，底下直接是 `00.json.gz`…）。看錯一層的話這一步會
# 一直「跳過」，而跳過在輸出裡不長得像失敗——CPU 就這樣一路沒被驗過。
step "CPU 語料（SingleStepTests）"
corpus=(testdata/8088/*.json.gz)
if [[ -e "${corpus[0]}" ]]; then
  echo "  語料 ${#corpus[@]} 檔"
  if [[ ${#corpus[@]} -lt 300 ]]; then
    bad "語料只有 ${#corpus[@]} 檔（完整的一份是 323）——先跑 tools/fetch_cputests.sh"
  fi
  tools/go.sh test ./internal/cpu -run TestSingleStep && echo "  ✓" || bad "語料沒全綠"
else
  echo "  跳過：testdata/8088/ 底下沒有語料。用 tools/fetch_cputests.sh 抓。"
  echo "  ⚠ **跳過不等於通過**：CPU 的驗收判準是「全部通過」，這一輪沒有驗到 CPU。"
fi

printf '\n'
if [[ $fail -eq 0 ]]; then
  echo "全部通過。"
else
  echo "有項目沒過，見上面的 ✗。"
fi
exit $fail
