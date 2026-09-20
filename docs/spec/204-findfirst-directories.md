# 204 — FindFirst 回傳目錄項目

狀態：**READY**  
日期：2026-09-20  
前置：[`009-scratch-writes`](009-scratch-writes.md)、
[`184-mvp-scope-review`](184-mvp-scope-review.md)

## 1. 動態缺口

《Buck Rogers: Countdown to Doomsday》的正常功能選單進入存檔／冒險路徑後，會反覆執行：

1. `int 21h AH=39h` 建立 `C:\BUCK\SAVE`；
2. `int 21h AH=1Ah` 設 DTA；
3. `int 21h AH=4Eh` 搜尋 `C:\BUCK\SAVE`。

未設定 `DOS.Scratch` 時，`AH=39h` 依既有相容行為宣告成功但不落地，原版因此反覆嘗試。
設定可寫 Scratch 後，`/scratch/SAVE` 已確實建立且未實作服務為 0，但 `AH=4Eh` 仍每次回
「沒有更多檔案」。根因是 `searchFor` 對 `DirEntry.IsDir()` 無條件 `continue`，而
`emitFind` 又把所有結果的 DTA 屬性固定寫成 `0x20`（archive）。

以上位址是 DOS 中斷 API，不是遊戲 executable 位址。玩家路徑由相同 #99,999,999 快照
依序輸入 `Down, Down, Enter` 到達，沒有 memory poke 或 direct-entry。

證據審查補充：末端 trace 在 `0C72:013F` 設定 `CX=0010`，`0C72:0141` 以
`AX=4E00, CX=0010` 呼叫 DOS，`0C72:0143` 立即取得 `AX=0012`。因此原版確實要求把目錄
納入搜尋；本規格沒有以推測補上未知屬性。

## 2. DOS 契約

`AH=4Eh` 的 `CX` 是搜尋屬性遮罩。目錄不是一般檔案；只有遮罩包含 bit 4（`0x10`）時，
搜尋結果才可包含相符目錄。回傳目錄時，DTA `+15h` 必須是目錄屬性 `0x10`，長度為 0；
一般檔案維持 `0x20`。時間與日期仍由 host metadata 決定。

## 3. 實作契約

1. `findState` 每筆結果保存實際路徑、8.3 大寫名稱與屬性，不再由 `emitFind` 猜成 archive。
2. `findFirst` 把 `CX` 低位元組傳給搜尋；`CX & 0x10 == 0` 時維持現行「跳過目錄」。
3. `CX & 0x10 != 0` 時，Root 與 Scratch 中符合 8.3 樣式的目錄可列入；同名項目仍由後掃描
   的 Scratch 覆蓋 Root，排序仍以大寫名稱決定。
4. DTA 的目錄 size 固定為 0；一般檔案使用 `FileInfo.Size()`。
5. 不放寬 path traversal，也不寫 Root；建目錄仍只透過既有 Scratch 契約。

## 4. 驗收

- `CX=0` 搜尋 `SAVE` 不得回傳目錄。
- `CX=0x10` 搜尋 `SAVE` 應成功，DTA 名稱為 `SAVE`、屬性 `0x10`、大小 0。
- 由 `AH=39h` 在 Scratch 建立 `SAVE` 後，`AH=4Eh/CX=0x10` 必須可立即找到。
- 既有檔案 FindFirst／FindNext、排序、Scratch 覆蓋與錯誤路徑測試不得退步。
- Buck Rogers 的相同正常輸入不再停在 `AH=39h → AH=1Ah → AH=4Eh` 迴圈，才算跨層驗收。

## 5. 不做什麼

- 不實作磁碟機樹、真正 DOS 目前目錄或長檔名；現行 basename flattening 不在本片改寫。
- 不建立 `.`／`..` 虛擬項目，不加入 hidden／system／volume label 語意。
- 不把可丟棄 Scratch 或任何原版素材納入版控。
