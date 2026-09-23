# 202 — x87 探測語意（`FNINIT`／`FNSTCW` 控制字）

狀態：**READY**（行為照 DOSBox-X `src/fpu/fpu.cpp` 的 `FINIT` 與控制字初值；
觸發案例見 §2；實作另開條目，本輪只定規格）
日期：2026-09-23
前置：無（CPU 層；`internal/cpu` 的 ESC 目前「只做記憶體讀取」）

---

## 1. 規則

只做**探測語意**，不做浮點運算。目標是讓「有沒有 8087」這道檢查回出
跟實機／DOSBox 一樣的答案。

- `FNINIT`（`DB E3`）：把模擬的控制字設為 `037Fh`（初值：例外全遮罩、
  就近捨入、64 位元精度）。依據是 DOSBox-X `FPU_FINIT`
 （`src/fpu/fpu_instructions.h`：`fpu.cw.init()` 等，初值即 `037Fh`）。
- `FNSTCW m16`（`D9 /7`，記憶體運算元）：把控制字存進去。
  1988 年工具的標準探測手法就是 `fninit；fnstcw mem；cmp 高位, 03h`
  （Watcom 6.5 的 `WCC`／`WCG` 啟動碼 `sub_1007F` 即此形；高位 `03h`＝有，
  否則無）。

## 2. 觸發案例（Watcom C 6.5，`retro-runtime-study-private#32` R27–R37）

- `WCC.EXE` 前端 `EXEC WCG.EXE`（chained compile）後必 `E142 ***FATAL***
  Stack Overflow；同樣的 `HELLO.C`／`EMPTY.C` 在 DOSBox（0.74-3，`fpu`
  開與關皆可）編過（`Code size: 11／21`）。
- 自寫探針（`fninit；fnstcw；印控制字`，見該 issue R37）：
  DOSBox 兩種設定皆回 `037Fh`；dosgolem 回 `0000h`（ESC 只做記憶體讀取，
  控制字槽維持初值零）。
- 後果鏈（已證實到合成者）：無 8087 判定→數學初始化走 index 0
 （`loc_12FA9`）→閂鎖 `ds:0x2AC＝0xFF`→整數程式無浮點 op 清不了→迴圈尾
  `sub_12E14` 檢查引爆 E142（父 `func#0xC` case 4 即時印出）。

## 3. 範圍外（明寫，不假裝有）

- x87 算術（`FADD／FMUL／FLD` 等的數值語意）：不做。整數程式走不到，
  本規格只修 chained compile 的探測分歧。
- `int 34h`–`3Dh` 模擬器陷入：不做（`WCG` 沒有 `AH=25h` 裝模擬器的碼，
  本案例用不到）。
- ⚠ 風險（寫給實作者）：探測回「有」之後，含浮點常數的程式會真的執行
  FP 運算——在算術未實作前結果是靜默錯誤。實作本規格時，要嘛連同
  「無算術」的限制一起公告，要嘛另開算術條目；不要讓「探測說有、
  算出來是垃圾」安靜地發生。
