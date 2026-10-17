# 435 — `INT 21h AX=4408h`：區塊裝置可不可抽換

狀態：**READY**
日期：2026-10-18
前置：[`004-dos-bios-services`](004-dos-bios-services.md)、[`434-mscdex-minimal`](434-mscdex-minimal.md)
案例：三國志V（san5）`MAIN.EXE` 的磁碟機掃描（`main:3D61C`）

---

## 1. 為什麼要做

`AH=44h` 的子功能 08h 原本落在「沒實作」的分支，一律回 AX = 0、清 CF，也就是每個磁碟機代號都
「存在而且可抽換」。三國志V 啟動時以它掃 A: 到 Z:（`main:3D61C` 經 `main:1D530`），
建 26 格的磁碟機表；舊行為讓每一格都像一台軟碟機。

## 2. 語意

依據 DOSBox-X `src/dos/dos_ioctl.cpp`：`DOS_IOCTL` 的磁碟機檢查（`:640-641`）與 case 0x08（`:731`），
commit `262a304f768d5d8e93ef4abcc479ea7175aef183`。

BL = 0 表示目前磁碟機，否則 BL − 1 是代號。

| 磁碟機 | 回傳 |
|---|---|
| 目前磁碟機（dosgolem 唯一的硬碟，預設 C:） | AX = 1（固定式），清 CF |
| MSCDEX 的光碟機（`434-mscdex-minimal`） | AX = 01h（function number invalid），設 CF。參考實作對可抽換的非 A:／B: 磁碟機回這個 |
| 其他代號（含 A:、B:，dosgolem 沒有軟碟機） | AX = 0Fh（invalid drive），設 CF |

## 3. 已知差異

| 項目 | 本實作 | 來源 | 分類 |
|---|---|---|---|
| 軟碟機 | 沒有，A:、B: 回 invalid drive | DOSBox-X 有掛軟碟時回 AX = 0 | 沿用 dosgolem 只有一台硬碟的模型 |

## 4. 驗收

1. `internal/dos/mscdex_contract_test.go` 的 `TestIOCTLRemovableMedia`：BL = 0、3 回 AX = 1；BL = 4（光碟）CF 與 01h；
   BL = 1、26 CF 與 0Fh。
2. 負對照：改回一律 AX = 0，第 1 條必須失敗（2026-10-18 手動確認後還原）。
3. `tools/ci.sh` 全過（原本依賴「一律成功」的既有測試會在這裡現形）。
