#!/usr/bin/env python3
"""產生 CGA 模式 04h 的 INT 10h AH=06h／07h 測試向量（docs/spec/250-cga-int10-scroll-and-palette §5 第 1 項）。

依 DOSBox-X `src/ints/int10_char.cpp` 的 `INT10_ScrollWindow`（行 1032 起）、`CGA4_CopyRow`（行 73）、
`CGA4_FillRow`（行 428）的語意，以**與 Go 實作無關**的邏輯座標（第 y 條掃描線、第 x 個位元組，y 優先）重現，
輸出每個案例執行後整張 200×80 位元組畫面的 SHA-256。Go 測試讀同一份 TSV 當字面期望值。

  docker run --rm --network none -u "$(id -u):$(id -g)" -v "$PWD:/src" -w /src python:3.13-bookworm \
    python -B tools/cga_scroll_vectors.py > internal/dos/testdata/cga_scroll_vectors.tsv

初始畫面：位元組 (y, x) = (y*7 + x*3 + 1) & 0xFF，使每條掃描線互異，複製方向、每格寬度、bank 位移錯誤都不會因填充值相同而通過。
"""
import hashlib

ROWS, COLS, H, W = 25, 40, 200, 80


def initial():
    return [[(y * 7 + x * 3 + 1) & 0xFF for x in range(W)] for y in range(H)]


def scroll(fb, ah, al, bh, ch, cl, dh, dl):
    """回傳執行後的畫面；INT10_ScrollWindow 的語意（先比較後夾邊；AL=0 或 AL 不小於視窗列數視為清除）。"""
    rul, cul, rlr, clr = ch, cl, dh, dl
    if rul > rlr or cul > clr:
        return fb
    if rlr >= ROWS:
        rlr = ROWS - 1
    if clr >= COLS:
        clr = COLS - 1
    if cul > clr:
        return fb
    rows = rlr - rul + 1
    fill = (bh & 3) * 0x55
    x0, x1 = cul * 2, (clr + 1) * 2

    def fill_row(r):
        for y in range(r * 8, r * 8 + 8):
            for x in range(x0, x1):
                fb[y][x] = fill

    def copy_row(src, dst):
        for i in range(8):
            for x in range(x0, x1):
                fb[dst * 8 + i][x] = fb[src * 8 + i][x]

    if al == 0 or al >= rows:
        for r in range(rul, rlr + 1):
            fill_row(r)
        return fb
    n = al
    if ah == 0x06:  # 上捲：rul+n..rlr 移到 rul..rlr-n，由上往下複製；底部 n 列填色
        for r in range(rul + n, rlr + 1):
            copy_row(r, r - n)
        for r in range(rlr - n + 1, rlr + 1):
            fill_row(r)
    else:  # 下捲：rul..rlr-n 移到 rul+n..rlr，由下往上複製；頂部 n 列填色
        for r in range(rlr - n, rul - 1, -1):
            copy_row(r, r + n)
        for r in range(rul, rul + n):
            fill_row(r)
    return fb


def digest(fb):
    return hashlib.sha256(bytes(b for row in fb for b in row)).hexdigest()


CASES = [
    ("clear", 0x06, 0, 0, 2, 3, 5, 10),
    ("up1", 0x06, 1, 0, 2, 3, 5, 10),
    ("up3", 0x06, 3, 0, 2, 3, 5, 10),
    ("down1", 0x07, 1, 0, 2, 3, 5, 10),
    ("down2", 0x07, 2, 0, 2, 3, 5, 10),
    ("up_equal_height", 0x06, 4, 0, 2, 3, 5, 10),
    ("up_over_height", 0x06, 9, 0, 2, 3, 5, 10),
    ("down_equal_height", 0x07, 4, 0, 2, 3, 5, 10),
    ("fill3", 0x06, 0, 3, 2, 3, 5, 10),
    ("fill1", 0x06, 0, 1, 2, 3, 5, 10),
    ("fill_bh85", 0x06, 0, 0x85, 2, 3, 5, 10),
    ("clamp_right_bottom", 0x06, 0, 0, 20, 30, 40, 79),
    ("clamp_scroll", 0x06, 2, 0, 20, 30, 40, 79),
    ("top_gt_bottom", 0x06, 0, 0, 5, 3, 3, 10),
    ("left_gt_right", 0x06, 0, 0, 2, 12, 5, 10),
    ("ch30_dh24", 0x06, 0, 0, 30, 3, 24, 10),
    ("full_up1", 0x06, 1, 0, 0, 0, 24, 39),
    ("full_down1", 0x07, 1, 0, 0, 0, 24, 39),
    ("one_row_up1", 0x06, 1, 0, 7, 0, 7, 39),
    ("one_col_up2", 0x06, 2, 0, 3, 5, 12, 5),
    ("bottom_down3", 0x07, 3, 2, 18, 0, 24, 39),
    ("top_up2", 0x06, 2, 0, 0, 0, 5, 39),
]

if __name__ == "__main__":
    print("name\tah\tal\tbh\tch\tcl\tdh\tdl\tsha256")
    for name, ah, al, bh, ch, cl, dh, dl in CASES:
        fb = scroll(initial(), ah, al, bh, ch, cl, dh, dl)
        print(f"{name}\t{ah:02X}\t{al}\t{bh}\t{ch}\t{cl}\t{dh}\t{dl}\t{digest(fb)}")
