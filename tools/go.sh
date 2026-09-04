#!/usr/bin/env bash
# Go 走 docker，不裝到系統環境。
#
#   tools/go.sh build ./...
#   tools/go.sh test ./internal/cpu -run TestSingleStep -v
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
for v in GOOS GOARCH CGO_ENABLED; do
  [[ -n "${!v:-}" ]] && PASS+=(-e "$v=${!v}")
done

exec timeout "${DOSGOLEM_TIMEOUT:-30m}" docker run --rm --network none \
  --memory "${DOSGOLEM_MEM:-4g}" --cpus "${DOSGOLEM_CPUS:-4}" --pids-limit 256 \
  --log-opt max-size=10m --log-opt max-file=3 \
  -u "$RUN_UID:$RUN_GID" \
  -v "$ROOT:/src" \
  -v "$ROOT/workplace/gocache:/gocache" \
  -v "$ROOT/workplace/gomodcache:/gomodcache" \
  -e GOCACHE=/gocache -e GOMODCACHE=/gomodcache \
  -e HOME=/tmp -e GOFLAGS=-mod=mod \
  "${PASS[@]}" -w /src "$IMAGE" go "$@"
