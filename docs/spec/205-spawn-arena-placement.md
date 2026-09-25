# 205 — spawn 走 arena 落子（EXEC 不壓活塊）

狀態：**READY**（只定規則，不碰實作；觸發案例見 §3；實作另開條目）
日期：2026-09-25
前置：[`009` 的 arena 與 `seg+1` 慣例](009-scratch-writes.md)（按該 repo 實際檔名調整）

---

## 1. 規則

`spawn`（`AH=4Bh AL=00h`）與 supervisor queue 落子目前都用 `freeSeg+1` 當子 PSP。
但 `splitBlock`（`AH=48h`）只標 arena、不推 `freeSeg`——app 只要先配置再 EXEC，
子 PSP＋映像就落在 arena 首活塊上，蓋掉活的堆（真 DOS 的 MCB arena 絕不允許）。

- 落子改走 arena：找 ≥ `need` 的自由段（沿用 `pickBlock` 策略與 `seg+1`／假 MCB
  慣例，`WriteMCB` 照寫），標已用。
- arena 還是空（nil、沒人配過）維持 `freeSeg+1` 快速路——行為與以前逐位元組一致。
- `freeSeg` 高水位：只升不降（`max(舊值, PlacedEnd)`）；子退出照既有回收路
 （含 arena 塊釋放＋`freeSeg` 還原——實作時先確認回收已涵蓋 arena 塊，沒有才補）。
- TSR／queue 落子同一修法（同一 `freeSeg+1` 模式）。

## 2. 驗收

- `dosrun` 跑 chained `WCC EMPTY.C／HELLO.C`：E142 消失（出乾淨 `Code size`＋`.OBJ`），
  且全測試綠（含既有 `TestChildExitReclaimsMemory` 系）。
- 迴歸：`arena` 為空的老路徑位元組一致（既有測試已釘）。

## 3. 觸發案例（Watcom 6.5 E142，`retro-runtime-study-private#32` R55）

EXEC 前零釋放 → 父堆 `0x25D7／0x2958` 全活 → 子映像（`0x25E7–0x49A7`）蓋住
`0x2958` 塊（`-watch` 步 38489 實證）→ 父 54842 拷被蓋位元組 → 路徑走岔 →
`sub_2CE17` 守衛 E142。參照（FreeDOS）子 `DS＝0x6CC0` 高位載入，無重疊。

## 4. 範圍外

- 改配置策略（`AH=58h`）、MCB 客方發布、TSR 常駐語意：不動。
- WCC／WCG 側：不動（錯在 dosgolem 主體，不在客體程式）。
