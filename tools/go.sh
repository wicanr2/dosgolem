#!/usr/bin/env bash
# Go 走 docker，不裝到系統環境。
#
#   tools/go.sh build ./...
#   tools/go.sh test ./internal/cpu -run TestSingleStep -v
#
# 要跑原版執行檔時，用 DOSGOLEM_ORIG 指到玩家自己的素材目錄，
# 它會**唯讀**掛到容器裡的 /orig：
#
#   DOSGOLEM_ORIG=~/cht/rich2/workplace tools/go.sh run ./cmd/probe \
#       -exe /orig/ida/RUN_full.EXE -root /orig/orig/.../RICH2
#
# 本專案不含任何原版檔案，掛載一律唯讀（`ro`），容器也不會把它們帶出去。
#
# module cache 放 workplace/（gitignore），避免每次重抓。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="${DOSGOLEM_GO_IMAGE:-golang:1.24-bookworm}"
RUN_UID="${DOSGOLEM_RUN_UID:-$(id -u)}"
RUN_GID="${DOSGOLEM_RUN_GID:-$(id -g)}"

mkdir -p "$ROOT/workplace/gocache" "$ROOT/workplace/gomodcache"

# docker 預設不繼承 shell 的環境；交叉編譯要靠這幾個。
PASS=()
for v in GOOS GOARCH CGO_ENABLED DOSGOLEM_TEST_EXE DOSGOLEM_TEST_ROOT; do
  [[ -n "${!v:-}" ]] && PASS+=(-e "$v=${!v}")
done

# 原版素材唯讀掛載。沒設就不掛——容器預設看不到任何原版檔案。
MOUNTS=()
if [[ -n "${DOSGOLEM_ORIG:-}" ]]; then
  MOUNTS+=(-v "$(cd "$DOSGOLEM_ORIG" && pwd):/orig:ro")
fi

exec timeout "${DOSGOLEM_TIMEOUT:-30m}" docker run --rm --network none \
  --memory "${DOSGOLEM_MEM:-4g}" --cpus "${DOSGOLEM_CPUS:-4}" --pids-limit 256 \
  --log-opt max-size=10m --log-opt max-file=3 \
  -u "$RUN_UID:$RUN_GID" \
  -v "$ROOT:/src" \
  -v "$ROOT/workplace/gocache:/gocache" \
  -v "$ROOT/workplace/gomodcache:/gomodcache" \
  -e GOCACHE=/gocache -e GOMODCACHE=/gomodcache \
  -e HOME=/tmp -e GOFLAGS=-mod=mod \
  "${PASS[@]}" "${MOUNTS[@]}" -w /src "$IMAGE" go "$@"
