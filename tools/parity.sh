#!/usr/bin/env bash
# MVP-B 的驗收：跑到防拷密碼畫面，與原版逐點比對（`docs/spec/001` §4）。
#
#   DOSGOLEM_ORIG=~/cht/rich2/workplace tools/parity.sh <oracle.png>
#
# oracle 怎麼產：在 rich2 那邊跑
#
#   tools/pyx.sh tools/dosbox_pw_indexed.py 2
#
# 它用 DOSBox 的 Ctrl+F5 存索引 PNG，像素值就是執行期色號。
#
# ⚠ **本專案不含任何原版檔案。** 素材由玩家自備，經 DOSGOLEM_ORIG 唯讀掛載。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ORACLE="${1:?用法：tools/parity.sh <oracle.png>}"
EXE="${DOSGOLEM_EXE:-/orig/ida/RUN_full.EXE}"
GAME="${DOSGOLEM_GAME:-/orig/orig/大宇大富翁2/DOS/game/RICH2}"

exec "$ROOT/tools/go.sh" run ./cmd/parity \
  -exe "$EXE" -root "$GAME" -oracle "$ORACLE" "${@:2}"
