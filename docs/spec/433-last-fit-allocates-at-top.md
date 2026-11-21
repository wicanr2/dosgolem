# 433 — last fit 從自由區塊的尾端切

狀態：**READY**
日期：2026-10-17
前置：[`009-memory-allocator`](009-memory-allocator.md)、[`430-exec-child-environment`](430-exec-child-environment.md)
案例：三國志V（san5）`MAIN.EXE` 的 overlay manager

---

## 1. 為什麼要做

`AH=58h` 設成 last fit（策略低兩位為 2）時，`pickBlock` 挑位址最高的自由區塊，但 `carveBlock`
仍從那塊的**開頭**切。真 DOS 從尾端切。

Microsoft C 的 overlay manager（`__movesetheap`，三國志V `main:1FC7A`）先設 last fit，再用 `AH=48h`
配 overlay 表與 overlay 堆，之後才把程式的 PSP 區塊撐到最大。它預期表在記憶體頂端，PSP 區塊與
far heap 都在表的下方。dosgolem 把表切在 PSP 區塊正後方（段 `5B04`），far heap（`main:12654`
`__searchseg`）之後就寫進表裡，overlay 記錄的旗標與快取位置被蓋掉。下一次載入 overlay 時，manager
照被蓋掉的旗標走 XMS 路徑，拿到不存在的 handle `FFFF`，以 exit 4 結束（量測：`wicanr2/san5-remake`
的 `docs/re/012-dosgolem-san5-branch.md`，`-watch` 看到 far heap 寫進表的範圍）。

## 2. 語意

依據 DOSBox-X `src/dos/dos_memory.cpp` 的 `DOS_AllocateMemory`（lastfit 分支，「allocate block at the
end of the MCB」），commit `262a304f768d5d8e93ef4abcc479ea7175aef183`。

策略低兩位為 2（last fit）時，從挑中的自由區塊切 `want` 段：

1. 區塊大小剛好等於 `want`：整塊給出去。
2. 否則：給出去的區塊在尾端，資料段 = 原 MCB 段 + 1 + 原大小 − `want`，它的 MCB 在前一段；
   原區塊留在前面，大小 = 原大小 − `want` − 1，仍是自由。

first fit 與 best fit 不變（從開頭切）。適用所有經 `carveBlock` 的配置：`AH=48h`、EXEC 的程式區塊
與環境區塊（DOSBox-X 的 EXEC 也走 `DOS_AllocateMemory`，同受策略影響）。

## 3. 已知差異

| 項目 | 本實作 | 來源 | 分類 |
|---|---|---|---|
| best fit 剛好一段之差 | 沿用既有規則：剩不到 2 段就整塊給 | DOSBox-X 一律切，剩 0 段的空 MCB 也留著 | 既有行為，不在本規格 |
| 高位記憶體（UMB）優先位元 | 不處理，沿用 `009-memory-allocator` | DOSBox-X 先掃 UMB 鏈 | 既有行為 |

## 4. 驗收

1. `internal/dos/alloc_lastfit_test.go`：last fit 配 `0x10` 段，回傳段 = 最高自由區塊的尾端 − `0x10`；
   剩下的前段仍可用 first fit 配到；大小剛好時整塊給出。
2. `dosinfo_contract_test.go` 既有三個策略測試不變。
3. 負對照：改回從開頭切，第 1 條必須失敗。

上層收據：三國志V overlay 表落在記憶體頂端，far heap 不再寫進表，`MAIN.EXE` 越過 exit 4。
