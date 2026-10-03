#!/usr/bin/env python3
"""同狀態收據（docs/spec/250-cga-int10-scroll-and-palette §5 第 4 項）：把原版程式實際發出的
INT 10h AH=06h／07h 呼叫，以與 Go 實作無關的 Python 模型（tools/cga_scroll_vectors.py，依 DOSBox-X
INT10_ScrollWindow 語意）重現，與 dosgolem 執行後的視訊記憶體逐位元組比對。

輸入：phantasie-receipt 的 -dump-scroll 目錄，內有 <路線>.<序號>.bin：
  8 bytes（ax、bx、cx、dx，各 16 位元小端）＋ 入口時 B800:0000 起 4000h bytes ＋ 完成時同範圍 4000h bytes。

  docker run --rm --network none -u "$(id -u):$(id -g)" -v "$PWD/tools:/t:ro" -v "<dump 目錄>:/d:ro" \
    python:3.13-bookworm python -B /t/cga_scroll_replay.py /d

輸出每個呼叫的參數、窗口列數與 PASS／FAIL，最後印摘要；有任何 FAIL 就以非零離開。
"""
import glob
import os
import struct
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import cga_scroll_vectors as model  # noqa: E402

PAGE = 0x4000


def to_rows(mem):
    """CGA 模式 04h 的交錯記憶體 → 200 條掃描線，每條 80 bytes（y 優先）。"""
    rows = []
    for y in range(200):
        off = (y >> 1) * 80 + (0x2000 if y & 1 else 0)
        rows.append(list(mem[off:off + 80]))
    return rows


def main(d):
    files = sorted(glob.glob(os.path.join(d, "*.bin")))
    if not files:
        print("沒有 .bin 檔：路線沒有 INT 10h AH=06h／07h 呼叫，這份收據是空洞的")
        return 1
    bad = 0
    kinds = {}
    for f in files:
        data = open(f, "rb").read()
        if len(data) != 8 + 2 * PAGE:
            print(f"{f}: 長度 {len(data)} 不是 {8 + 2 * PAGE}")
            bad += 1
            continue
        ax, bx, cx, dx = struct.unpack("<4H", data[:8])
        ah, al, bh = ax >> 8, ax & 0xFF, bx >> 8
        ch, cl, dh, dl = cx >> 8, cx & 0xFF, dx >> 8, dx & 0xFF
        pre, post = to_rows(data[8:8 + PAGE]), to_rows(data[8 + PAGE:])
        want = model.scroll([r[:] for r in pre], ah, al, bh, ch, cl, dh, dl)
        ok = want == post
        rows = min(dh, 24) - ch + 1
        kind = ("清除" if (al == 0 or al >= rows) else ("上捲" if ah == 6 else "下捲")) + f" AH={ah:02X} AL={al}"
        kinds[kind] = kinds.get(kind, 0) + 1
        print(f"{os.path.basename(f)}\tAH={ah:02X} AL={al} BH={bh} 視窗(列{ch}-{dh} 欄{cl}-{dl})\t{'PASS' if ok else 'FAIL'}")
        if not ok:
            bad += 1
            diff = [(y, x) for y in range(200) for x in range(80) if want[y][x] != post[y][x]]
            print(f"  不同位元組 {len(diff)} 個，前 5 個 (y,x)：{diff[:5]}")
    print(f"共 {len(files)} 個呼叫；失敗 {bad}；種類 {kinds}")
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1]))
