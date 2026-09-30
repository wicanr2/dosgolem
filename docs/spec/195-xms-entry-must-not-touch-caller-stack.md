# 195 — XMS driver entry 不能改呼叫端的堆疊

狀態：**READY**
日期：2026-09-30
前置：[`011-xms.md`](011-xms.md)（XMS 服務）、
[`004-dos-bios-services.md`](004-dos-bios-services.md)（trampoline 與 `fixStackedCF`）

---

## 1. 問題

服務層處理 `int F5h`（XMS driver entry 的 trampoline，`CD F5 CB`）之後呼叫
`fixStackedCF`，把 CF 寫進 `SS:SP+4` 那個 word 的 bit 0。

`fixStackedCF` 的前提是堆疊上有一個**中斷框**（IP、CS、FLAGS），`SP+4` 就是
被中斷處的 FLAGS。`int 21h`／`10h`／`15h` 的 trampoline 都是經 INT 進來，前提成立。

**XMS entry 是 far call**（XMS 3.0：`call far [entry]`），堆疊上只有 CS:IP。
`SP+4` 是呼叫端自己 push 的資料。每呼叫一次 XMS，那個 word 的 bit 0 就被改成
CF（通常是清掉）。XMS 本來就不用 CF 回報結果（`AX=1`／`AX=0＋BL`），
這一步沒有任何正當用途。

## 2. 量到的證據（已證實）

《魔眼殺機二》中文版 `START.EXE` 的 Borland overlay 管理員（dosgolem 段 `1E1A`）：

- `1E1A:11F1`–`1217`：`push es`（ES=`2C57`，某個 overlay 的 stub 段）後
  `call far` XMS entry（`0080:040C`）做 XMS move。
- 回來後 `pop es` 得到 **`2C56`**：`2C57` 的 bit 0 被清掉。
- `1E1A:0626` 的 `mov word es:[0010h],0`（卸載：清 stub 的載入段欄位）因此寫到
  `2C56:0010` ＝ `2C57:0000`，把 stub 的 `CD 3F` 清成 `00 00`，載入段欄位保持 `3DE6`。
- 之後片頭 overlay 覆蓋那塊記憶體，管理員照 stub 以為舊 overlay 還在，跳進被覆蓋的
  程式碼，執行不該執行的影像編碼，最後常規記憶體不足
  （`Insufficient memory error!`）。
- 對照：DOSBox-X（記憶體寫入斷點）在同一流程裡，同一條 `mov` 寫的是 stub `+10h`，
  欄位依序 `0 → 43DF → 44FA → 0 → 46EF`；dosgolem 在第四步寫錯位置。

## 3. 修法

`int F5h` 分派只呼叫 `xmsCall`，不呼叫 `fixStackedCF`。

`int F6h`（EMS）原本就不呼叫，理由相同。

## 4. 驗收

- 單元：堆疊上放「far call 回傳位址＋一個 bit 0 為 1 的 word」，經 `int F5h`
  分派做一次會失敗（CF 會被設成 0）的 XMS 呼叫後，該 word 不變。
- 實跑：EOB2 中文版不再在片頭 `T1.CPS` 那一幕記憶體不足。
