# 436 — XMS move（`AH=0Bh`）先驗證再搬

狀態：**READY**
日期：2026-10-18
前置：[`011-xms`](011-xms.md)
案例：三國志V（san5）`MAIN.EXE` 的 overlay manager

---

## 1. 為什麼要做

`xmsMove` 不檢查描述子：不存在的 handle 讀到 0、寫入丟掉，搬移一律回 AX = 1。三國志V 的 overlay
manager 在 overlay 表被蓋掉時（`433-last-fit-allocates-at-top` 修正前），以來源 handle `FFFF`、位移
`A8CC6F08` 搬了 576 bytes，dosgolem 回成功並把 576 個 0 寫進常規記憶體 `0094:0000`（落在 StubSeg 的
向量 stub 區）。真 XMS 驅動會回錯誤，manager 走 `__amsg_exit` 印訊息結束，不會默默寫壞記憶體
（量測：`wicanr2/san5-remake` 的 `docs/re/012-dosgolem-san5-branch.md`、`013-main-to-title-menu.md`）。

## 2. 語意

依據 DOSBox-X `src/ints/xms.cpp` 的 `XMS_MoveMemory`（`:272`），commit `262a304f768d5d8e93ef4abcc479ea7175aef183`。
依序檢查，第一個不成立的回 AX = 0、BL = 錯誤碼，記憶體不動：

| 順序 | 條件 | BL |
|---:|---|---|
| 1 | 長度是奇數（XMS 3.0「Length must be even」） | A7h |
| 2 | 來源 handle 非 0 而且不存在 | A3h |
| 3 | 來源位移不小於區塊大小 | A4h |
| 4 | 長度大於區塊大小減來源位移 | A7h |
| 5 | 來源 handle 為 0：far 指標換成線性位址加長度超過 10FFF0h | A7h |
| 6 到 9 | 目的端同 2 到 5，錯誤碼 A5h、A6h、A7h、A7h | |

全部成立才搬，回 AX = 1。`XMSMove.Status` 記下失敗的 BL，`probe` 的 XMS move 列表印出來。

## 3. 已知差異

| 項目 | 本實作 | 來源 | 分類 |
|---|---|---|---|
| 來源與目的重疊 | 逐 byte 由前往後搬（沿用） | XMS 3.0 只保證來源在前時正確；DOSBox-X 用 `MEM_BlockCopy` | 沿用既有行為 |

## 4. 驗收

1. `internal/dos/xms_sda_contract_test.go` 的 `TestXMSMoveRejectsInvalidRequests`：上表七種情形各回 AX = 0 與對應 BL，
   常規記憶體的哨兵不變；合法搬移仍成功。
2. `xms_contract_test.go` 的 `TestEMBInfoAndReallocKeepContents` 原本搬 1 byte，改成 2 bytes（奇數長度依規格回 A7h）。
3. 負對照：拿掉驗證，第 1 條必須失敗（2026-10-18 手動確認後還原）。
