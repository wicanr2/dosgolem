# 432 — `INT 21h AH=51h／62h` 回目前行程的 PSP

狀態：**READY**
日期：2026-10-17
前置：[`009-exec`](009-exec.md)、[`008-tsr-resident`](008-tsr-resident.md)
案例：三國志V（san5）`GRP480C.EXE`

---

## 1. 為什麼要做

`AH=51h` 與 `AH=62h` 一律回常數 `machine.PSPSeg`（`0x0100`）。第一支程式不受影響，
EXEC 的子行程卻拿到第一支程式的 PSP。

`GRP480C.EXE` 的常駐碼以 `AH=62h` 取得的 BX 當自己的 PSP，算
`DX = (SS - BX) + (SP >> 4) + 1` 後 `AH=31h` 常駐。三次實跑的 DX 都符合
`0x599 + (實際 PSP - 0x100)`：子行程載得越高，常駐區多留越多，之後每支程式的
載入段都和真 DOS 不同（量測：`wicanr2/san5-remake` 的
`docs/re/010-dosgolem-coverage.md`，`GRP480C.EXE` 常駐大小的備註）。

## 2. 語意

依據 DOSBox-X `src/dos/dos.cpp:2431`（`case 0x51`）與 `:2734`（`case 0x62`），
兩者都回 `dos.psp()`，commit `262a304f768d5d8e93ef4abcc479ea7175aef183`。

- `BX` = 目前行程的 PSP（`curPSP`）。`curPSP` 為 0（開機階段、測試直接呼叫）時回
  `PSPSeg`，與配置器的擁有者規則（`blockOwner`）一致。
- 清 CF，其他暫存器不動。

## 3. 影響範圍

第一支程式的 `curPSP` 就是 `PSPSeg`，結果不變。只有 EXEC 子行程（以及監督佇列推出的
下一支）看到的值改變。

## 4. 驗收

1. `internal/dos/psp_query_test.go`：EXEC 一支子程式後 `AH=62h` 與 `AH=51h` 都回子行程
   PSP；子行程結束後回到父行程的。
2. 負對照：改回常數，第 1 條必須失敗。

上層收據：三國志V `GRP480C.EXE` 的 `AH=31h` DX 為 `0x599`，與載入段無關。
