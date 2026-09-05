#!/usr/bin/env bash
# 在既有 FD2 DOSBox 映像中建立可拋棄遊戲複本並執行有界抓圖。
set -euo pipefail

if [[ $# -lt 2 || $# -gt 4 ]]; then
  echo "用法：$0 原版目錄 輸出目錄 [timeline] [cycles]" >&2
  exit 2
fi

orig=$(cd "$1" && pwd)
mkdir -p "$2"
out=$(cd "$2" && pwd)
timeline=${3:-wait:2;key:Escape;wait:8;shot:title-original}
cycles=${4:-fixed 12000}
image=${DOSGOLEM_FD2_DOSBOX_IMAGE:-fd2-dosbox-screenshot-local:latest}
uid=$(id -u)
gid=$(id -g)

test -f "$orig/FD2.EXE" || { echo "缺少 $orig/FD2.EXE" >&2; exit 2; }

exec timeout "${DOSGOLEM_FD2_CAPTURE_TIMEOUT:-90s}" docker run --rm \
  --network none --memory 1g --cpus 2 --pids-limit 256 \
  --log-opt max-size=10m --log-opt max-file=2 \
  -u "$uid:$gid" \
  --tmpfs "/game:rw,uid=$uid,gid=$gid,mode=0700,size=256m" \
  -v "$orig:/orig:ro" -v "$out:/shots:rw" \
  -e "FD2_TIMELINE=$timeline" -e "FD2_CYCLES=$cycles" \
  --entrypoint bash "$image" -lc '
set -euo pipefail
cp -a /orig/. /game/
actual_size=$(stat -c %s /game/FD2.EXE)
actual_md5=$(md5sum /game/FD2.EXE | cut -d" " -f1)
actual_sha256=$(sha256sum /game/FD2.EXE | cut -d" " -f1)
if [[ "$actual_size" != 357074 ||
      "$actual_md5" != b97caf2239a27a896069d03549d96e1e ||
      "$actual_sha256" != 222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f ]]; then
  echo "FD2.EXE 身分不符：size=$actual_size md5=$actual_md5 sha256=$actual_sha256" >&2
  exit 3
fi
/usr/local/bin/fd2-dosbox-screenshot "$FD2_TIMELINE" "$FD2_CYCLES"
for shot in /shots/*.png; do
  [[ -e "$shot" ]] || continue
  geometry=$(identify -format "%wx%h" "$shot")
  if [[ "$geometry" != 1024x768 ]]; then
    echo "DOSBox 擷取視窗尺寸不符：$shot 是 $geometry，預期 1024x768" >&2
    exit 4
  fi
  convert "$shot" -crop 320x200+0+0 +repage "$shot"
done
'
