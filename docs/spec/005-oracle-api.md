# 005 — 對外的 Go API

狀態：**DRAFT**（規格未定案，**不要照這一份動手**）
日期：2026-09-04

---

## 1. 這一層存在的理由

整個專案的價值在這裡：**讓對拍的兩邊活在同一個 Go 行程裡**。

```go
ora := oracle.Load("…/RICH2")            // 玩家自備的原版
ora.RunUntil(oracle.CallOf(0x25BF6))     // 跑到收租常式
rent := ora.Word("ds:1BE")               // 直接讀原版的變數
shot := ora.Indexed()                    // 320×200 色號，不經過 X

parity.Compare(t, shot, remakeFrame)
```

## 2. 要有的能力（草案）

| 能力 | 為什麼 | 對應到 `rich2` 的哪個痛點 |
|---|---|---|
| `RunUntil(cond)` | 跑到某個位址／某個呼叫／某一幀 | 取代 `time.sleep(2.2)`（`rich2/docs/lessons.md` F48）|
| `Word/Byte("ds:1BE")` | 直接讀原版變數 | 取代「從像素反推」（D34 誤中）|
| `Indexed()` | 320×200 色號陣列 | 取代 Ctrl+F5 ＋ 檔案交換 |
| `OnCall(addr, fn)` | 攔繪製常式，**印出原版自己的呼叫參數** | 判準從像素變成參數 |
| `Save()/Restore()` | 從同一個狀態展開多個變體 | 取代「改存檔碰運氣」（D33／D34）|
| `Trace(RND)` | 記錄每一次亂數 | `rich2/WORKLIST.md` P1.1 |
| `Key()/Mouse()` | 幀對齊的輸入 | 取代 xdotool ＋ 猜的按住時間 |

## 3. 未定案

- 位址怎麼寫（`"ds:1BE"` 字串？型別化的 `Seg/Off`？）。
  字串好讀但會在執行期才失敗；**傾向型別化 ＋ 一支 helper**。
- `RunUntil` 的上限與逾時語意：**跑不到要是明確的錯誤，不是靜靜地回來**。
- 呈現層要不要在這裡就做調色盤（回索引還是回 RGB）。
  **傾向回索引**——`rich2` 那邊的比對本來就在色號空間做。
