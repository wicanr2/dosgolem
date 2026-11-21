# 429 — EMS 4.0 `INT 67h AH=57h`（Move／Exchange Memory Region）

狀態：**READY**
日期：2026-10-17
前置：[`008-ems-paging`](008-ems-paging.md)（複製式分頁）、[`014-ems`](014-ems.md)（偵測與 4.0 功能）
案例：三國志V（san5）`MAIN.EXE`

---

## 1. 為什麼要做

三國志V 的 `MAIN.EXE` 帶 37 個 overlay 模組，Microsoft C 的 overlay manager
啟動時把它們從 `MAIN.EXE` 讀出後用 `AX=5700h` 搬進 EMS。目前 `emsCall` 沒有
`57h`，回 `AH=84h`（function not supported），程式印
`run-time error R6808 - error accessing expanded memory` 後 exit 255，
停在第一個 overlay 載入前（量測：`wicanr2/san5-remake` 的
`docs/re/010-dosgolem-coverage.md`，實跑 `mainq2`，dosgolem `81c85be`）。

## 2. 介面

呼叫：`AH=57h`，`AL=00h` 搬移、`AL=01h` 交換，`DS:SI` 指向 18 bytes 描述子：

| 位移 | 大小 | 欄位 |
|---:|---:|---|
| +00h | dword | 長度（bytes） |
| +04h | byte | 來源型態：0 常規記憶體、1 EMS |
| +05h | word | 來源 handle（型態 0 時忽略） |
| +07h | word | 來源位移 |
| +09h | word | 來源段（型態 0）或邏輯頁號（型態 1） |
| +0Bh | byte | 目的型態 |
| +0Ch | word | 目的 handle |
| +0Eh | word | 目的位移 |
| +10h | word | 目的段或邏輯頁號 |

回傳：`AH` 為狀態碼，其他暫存器不動。

## 3. 語意

依據 DOSBox-X `src/ints/ems.cpp` 的 `MemoryRegion`（`:867`）與 `case 0x57`
（`:1088`），commit `262a304f768d5d8e93ef4abcc479ea7175aef183`。

1. `AL > 1`：`AH=84h`，不動任何記憶體。
2. 型態 0 的位址：線性 `段 × 16 + 位移`。
3. 型態 1 的位址：handle 內的線性位移 `頁號 × 16384 + 位移`。位移可以超過
   16 KB，跨頁連續。
4. 檢查順序：長度先（§4），再來源、目的。型態 1 的 handle 不存在回 `83h`；
   `頁號 × 16384 + 位移 + 長度` 超過 handle 的頁數 × 16384 回 `8Ah`。
   任一檢查失敗都不動記憶體。
5. 成功回 `AH=00h`。`AL=00h` 把來源的長度個 bytes 寫到目的；`AL=01h` 交換兩邊。

**複製式分頁下的一致性**（`008-ems-paging` §3）：EMS 頁若此刻映在 page
frame 的某個實體頁，那一頁的現行內容在 frame 裡，不在 Go 端的備份。
所以型態 1 的每個 byte 先看所屬邏輯頁有沒有映著：有就經 `M.Read8／Write8`
存取 frame 位址，沒有才讀寫備份。型態 0 一律經 `M.Read8／Write8`，
落在 frame 範圍內的常規位址自然就是映著的那一頁。這樣搬完不需要
flush 或 reload，寫入監看也看得到 frame 內的寫入。

## 4. 已知差異

| 項目 | 本實作 | 來源 | 分類 |
|---|---|---|---|
| 來源與目的重疊 | 先完整讀出來源再寫目的（`memmove` 語意），回 `00h` | LIM 4.0 規定重疊搬移回 `92h`、重疊交換回 `97h`；DOSBox-X 不檢查，以 4 KB 為單位邊讀邊寫 | 刻意不照做：照參考實作不回錯誤碼，資料結果取一個確定值。到期條件：遇到依賴 `92h`／`97h` 的程式 |
| 長度上限 | 超過 1 MB（1,048,576 bytes）回 `96h`，在 handle 檢查之前 | LIM 4.0 同此；DOSBox-X 不檢查 | 刻意不照參考實作：不擋的話壞描述子會讓 Go 端配置數 GB 記憶體 |
| 型態不是 0 或 1 | 非 0 一律當 EMS | DOSBox-X 同樣以「非 0」判斷 | 照參考實作 |
| 同一邏輯頁映在多個實體頁 | 取編號最小的實體頁 | 真 EMS 是同一塊記憶體的別名 | 假說：沒有觀測到的程式這樣用 |

## 5. 驗收

`internal/dos/ems_contract_test.go` 新增：

1. 常規 → EMS（未映射頁）→ 映射後在 frame 讀到。
2. EMS（映著的頁，先在 frame 寫入、不換頁）→ 常規，讀到 frame 的內容，不是舊備份。
3. 跨頁：位移 16380、長度 8，資料跨兩個邏輯頁。
4. 交換：兩邊內容互換。
5. 錯誤碼：handle 不存在 `83h`、超出範圍 `8Ah`、`AL=02h` `84h`、長度超過 1 MB `96h`，記憶體都不動。
6. 負對照：把型態 1 的位址算成 `頁號 × 16 + 位移`，第 1 條必須失敗（2026-10-17 手動確認後還原；第 3 條用頁 0，這個錯誤抓不到）。

上層收據：三國志V `MAIN.EXE` 不再印 R6808，overlay manager 走到下一個停點
（記在 `wicanr2/san5-remake` 的新證據文件，不改 `010`）。
