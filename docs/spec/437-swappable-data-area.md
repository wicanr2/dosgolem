# 437 — `INT 21h AX=5D06h`：DOS 可交換資料區（SDA）

狀態：**READY**
日期：2026-10-18
前置：[`004-dos-bios-services`](004-dos-bios-services.md)（`AH=34h` InDOS）
案例：三國志V（san5）`MAIN.EXE` 的 CD 模組

---

## 1. 為什麼要做

`AX=5D06h` 沒有實作，DS:SI 原樣返回。三國志V 的 CD 初始化 `main:131D9` 把回傳的 DS:SI 存起來，
它的計時器 callback `main:13155` 每次先讀那裡的 word：0 才送 MSCDEX 的狀態請求（`main:13137`）。
沒實作時那個指標指著呼叫端的程式碼，讀到的值與 DOS 狀態無關。

## 2. 語意

依據 DOSBox-X `src/dos/dos.cpp` 的 `AH=5Dh AL=06h`（`:2613` 一帶）與 `include/dos_inc.h` 的 `sSDA`，
commit `262a304f768d5d8e93ef4abcc479ea7175aef183`。

- 回 DS:SI = SDA（`StubSeg:04D0`，`machine.SDAOff`），CX = DX = 1Ah，清 CF。DOSBox-X 的 DX 是 1Ah、
  CX 預設 560h；這裡 SDA 只有 1Ah bytes，CX 也回 1Ah（DOSBox-X 的另一個設定值）。
- 版面照 `sSDA`：+0 critical error 旗標、+1 InDOS、+2 critical error 的磁碟機（FFh）、+3 錯誤位置、
  +4 延伸錯誤碼、+6 建議動作、+7 錯誤類別、+8 錯誤的 ES:DI、+0Ch 目前 DTA、+10h 目前 PSP、
  +12h INT 23h 的 SP、+14h 上一個行程的離開碼、+16h 目前磁碟機、+17h 延伸 break 旗標、+18h 保留。
- 每次呼叫時寫入：旗標與錯誤欄為 0、+2 為 FFh、DTA、目前 PSP（`AH=62h` 的同一個值）、離開碼、目前磁碟機。
- **InDOS 旗標移到 SDA+1**（`machine.InDOSOff` = `SDAOff + 1`），`AH=34h` 回的就是這一格，前一格是
  critical error 旗標，與真 DOS 相同。以前在 `StubSeg:0450`，SDA 的 1Ah bytes 從那裡起算會蓋到
  `0460` 的 DBCS 表。

## 3. 已知差異

| 項目 | 本實作 | 來源 | 分類 |
|---|---|---|---|
| 欄位同步 | 只在 `5D06h` 呼叫時寫入；之後 DTA、PSP 改變不會更新 SDA | DOSBox-X 在 `DOS_SetPSP` 等處同步 | remake-owned。到期條件：遇到讀 SDA 欄位當即時狀態的程式 |
| InDOS 的值 | 恆為 0（`int 21h` 一次做完，沒有可被打斷的中間狀態） | 真 DOS 在 `int 21h` 期間加 1 | 沿用 `004-dos-bios-services` |
| `AH=34h` 回傳的位址 | 改成 `StubSeg:04D1` | 以前 `StubSeg:0450` | 行為改變：記下 InDOS 位址的既有收據會不同 |

## 4. 驗收

1. `internal/dos/xms_sda_contract_test.go` 的 `TestSwappableDataAreaMatchesInDOS`：CF 清、DX = 1Ah、SDA+0 的 word 為 0、
   +2 為 FFh、+10h 為目前 PSP；`AH=34h` 指到 SDA+1；SDA 範圍不撞國別表與環境區塊的 MCB。
2. 負對照：InDOS 放回 `0450`，第 1 條必須失敗（2026-10-18 手動確認後還原）。

上層收據：三國志V 的未實作服務清單不再有 `int 21h AH=5D AL=06`。
