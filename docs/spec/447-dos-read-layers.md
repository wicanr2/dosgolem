# 447 — 唯讀疊層目錄

狀態：**READY**（呼叫端契約；觸發案例見 §2）
日期：2026-10-22
前置：[`009-scratch-writes`](009-scratch-writes.md)（暫存層）、[`195-subdirectory-path-resolution`](195-subdirectory-path-resolution.md)、[`198-dos-relative-directories`](198-dos-relative-directories.md)

---

## 1. 規則

`DOS.Layers []string`：

| 項目 | 行為 |
|---|---|
| 讀檔查找順序 | `Scratch`、`Layers[0]`、`Layers[1]`…、`Root`，第一個找到的為準；空字串略過 |
| 涵蓋的解析 | basename、完整路徑（195）、相對子目錄（198）三條解析都用同一順序 |
| 寫入 | 永遠不寫疊層。以寫入模式開疊層裡的檔時，照 009 的規則先複製到暫存層再開 |
| 狀態檔 | 疊層是呼叫端設定，不進狀態檔；從狀態檔接著跑時由呼叫端重新設定 |

`Layers` 為 nil 時行為與之前完全相同。

## 2. 觸發案例

智冠中文版《魔法門 IV＋V》合併執行。名詞修正會改寫 `DARK.CC`、`XEEN.CC` 裡的文字資源，長度會變，
只能產生新的封包檔。衍生檔不能放進暫存層（那是玩家存檔的位置，版本更新時難以替換），也不能覆寫原版目錄。
前端把衍生檔放在自己的目錄，設成疊層即可讓遊戲讀到，原版檔與存檔都不動。

## 3. 驗收

1. `internal/dos/read_layers_test.go`：疊層蓋過 Root、疊層沒有的檔讀 Root、暫存層蓋過疊層；
   以讀寫模式開疊層裡的檔時複製到暫存層，疊層內容不變。
2. `go test ./...` 全綠（Xvfb）。
