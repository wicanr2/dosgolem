# 431 — 移位量遮罩：80186 以上取 `CL & 31`

狀態：**READY**
日期：2026-10-17
前置：[`002-cpu-8086`](002-cpu-8086.md) §4 第 2 點、[`012-cpu-386-subset`](012-cpu-386-subset.md)
案例：三國志V（san5）`BGMSEQ.COM`

---

## 1. 為什麼要做

`002-cpu-8086` §4 第 2 點寫「移位／旋轉的位移量不遮罩成 5 位元」，那是 8086 的行為，
由 8086 語料支持。實作把它套到所有機型，而跑 DOS 軟體的 `machine.New()` 是
`Model80386`。

`BGMSEQ.COM` 的 CPU 偵測是 `mov ax,2; mov cl,21h; shr ax,cl`：8086 得 0，286 以上
因為 `21h & 31 = 1` 得 1。dosgolem 得 0，判成 8086，以 exit 7 結束，`FMOPL2.COM`
連帶 exit 11，整場沒有 BGM（量測：`wicanr2/san5-remake` 的
`docs/re/010-dosgolem-coverage.md`，實跑 `q1`；只改這一處的補丁副本 `snd`、`san5snd`
三支都 exit 0 並常駐）。

## 2. 語意

依據 DOSBox-X `src/cpu/instructions.h:863` 的 `CPU_SHIFTOP_MASK`（8086 核心 `& 0xff`，
其他 `& 0x1f`），commit `262a304f768d5d8e93ef4abcc479ea7175aef183`。

| 機型 | `D2`／`D3`（CL） | `C0`／`C1`（imm8） | 32 位元（`66` 前綴） |
|---|---|---|---|
| `Model8086` | 整個 CL（0–255） | 不存在（`RET` 別名） | 不存在 |
| `Model80186` | `CL & 31` | `imm8 & 31` | 不存在 |
| `Model80386` | `CL & 31` | `imm8 & 31` | `& 31`（`ops386.go` 原本就遮） |

遮罩後為 0 時與原本相同：不動運算元，也不動旗標。

## 3. 已知差異

| 項目 | 本實作 | 來源 | 分類 |
|---|---|---|---|
| 80186 的遮罩 | 與 386 相同取 5 位元 | Intel 80186 手冊同此；沒有 80186 語料可驗 | 照參考實作，假說等級：本 repo 沒有 80186 語料 |

## 4. 驗收

1. `tools/go.sh test ./internal/cpu` 全綠，8086 語料路徑沒動（`cpu.New()` 仍是 `Model8086`）。
2. `internal/cpu/shift_mask_test.go`：同一段 `mov ax,2; mov cl,21h; shr ax,cl`
   在 `Model8086` 得 0、`Model80186` 與 `Model80386` 得 1；`C1 /5 imm8=21h` 在
   `Model80186` 得 1。
3. 負對照：拿掉遮罩，第 2 條的 186／386 兩例必須失敗。

上層收據：三國志V `BGMSEQ.COM` 未修補即 exit 0。
