# 223 — Buck Rogers 劇情 content-safe 診斷

狀態：**CONFORMED（診斷限定；非 runtime hook）**
日期：2026-09-22
前置：`011-buck-rogers-diagnostic-key-schedule.md`、專案第一百零五階段的原版追蹤收據。

## 目的與邊界

本規格只授權 `cmd/buckrogers-text-receipt` 量測首屏劇情的低階 glyph run 與下一頁的第一筆
story-region framebuffer 改寫，讓 project adapter 取得可重播、content-safe identity／lifecycle
證據。它不建立 catalog、顯示 request、xlate layer、production watcher 或任何機器狀態改寫。

尤其不得將此診斷當作正式劇情覆繪的實作許可；後者須先通過專案 `010-story-opening-overlay-draft.md`
的 READY gate。

## 命令列與輸出契約

可選 flags：

- `-clear-trace`：只記錄 `026F:029C` entry 的 step 與四個 rectangle byte 參數。
- `-glyph-trace`：只在 `0763:026B` entry 與同 caller、同 `SS`、`SP + 0x12` guarded return 都
  成立時，輸出 caller、mode、repeat、bg／fg、row、column、entry／post-call step。glyph byte
  不可輸出。
- `-glyph-trace-from STEP`：零值不過濾；非零時只開始追蹤不早於該絕對 step 的 entry。
- `-story-pixel-trace`：從啟動 state 的 logical rows 17–21 建立只讀 320×40 baseline，輸出第一筆
  變化的 executed instruction address、step、`REP STOSB` 前的 `ES:DI`／`CX` destination metadata
  與變化 bounding box；不輸出 pixel data。

相鄰、相同 caller／mode／repeat／color／row 且 column 嚴格連續的 completed glyph 只在程序內
暫存 bytes 以計算 SHA-256；receipt 只可輸出 length／digest 與上述 metadata。任何 guard 失敗或
重入未完成 glyph 都增加 `glyph_drops`，不能佯裝完成 run。clear、glyph 與 pixel trace 都必須是
沒有對應 flag 時的零副作用選項；既有 receipt fields、BIOS 注入與 machine 迴圈語意不得變更。

## READY 審查

- 已有首屏五行兩次重播的 content-safe length／hash／caller／色號／位置收據，且第一個第二頁
  story-region write 已獨立量到為 `0CF4:1B3A`；診斷只補足可重生 observer，沒有猜測遊戲語意。
- 所有捕獲資料不是英文原文、手冊、答案、VRAM bytes、存檔資料或鍵盤內容；原始 bytes 在 hash
  後即丟棄。
- 既有 `buckrogers-text-receipt` 有唯一正常 state loop，將 observer 置於其指令前後不新增任何
  DOS／BIOS call；所以可保證它是 read-only diagnostics。

因此本規格只授權這四個 diagnostics 與其收據欄位。任何將 glyph run 轉為繁中、或將 pixel
變化轉為 overlay invalidation 的行為仍屬 project spec 010，未在此授權範圍。

## 驗收與停止線

1. `go test ./apps/buckrogers ./cmd/buckrogers-text-receipt`、`go vet` 與 race detector 通過。
2. 相同 state／BIOS schedule 的兩次 trace receipt 必須逐 byte 相同；receipt 不能含原文 glyph
   byte、原始 indexed bytes、答案或 screenshot。
3. 關閉全部新 flag 的 normal command 與改動前既有 output contract 相容；此規格不對 production
   overlay、host UI、玩家輸入或存讀檔作任何完成宣稱。

## Conformance 收據

2026-09-22，在使用者本機、唯讀 `GAME.OVR` 與既有私有成功返回 state 上，以同一 BIOS Enter
排程重播兩次。兩份 content-safe receipt 逐 byte 相等，SHA-256 為
`ff1ac573b81cc8175469de92a80e0b46093b4c5bbf3859d9e01ad344cff21345`。它們只留在
project `workplace/phase105-story-page-clear/`。

第二頁的第一筆 story-region 改寫固定為 step `281020572`、`0CF4:1B3A`；pre-execution
metadata 是 `ES:DI=A000:AB48`、`CX=304`，即 320-byte scanline row 137 的 x=`8..311`。這證明
未來 runtime 可採有界的 video-span 相交判斷，不必在每道指令後掃描整個 story region；它仍不是
runtime hook 本身。

Docker 無網路環境下 `go test ./apps/buckrogers ./cmd/buckrogers-text-receipt`、`go vet`、以及
`go test -race` 均通過；單元測試另鎖定 glyph／run／pixel-write JSON 的允許 metadata keys，拒絕
序列化 glyph bytes、original bytes、indexed pixels 或答案欄位。因此本規格升為
**CONFORMED（診斷限定）**；其結果只可作為後續 project DRAFT 的證據。
