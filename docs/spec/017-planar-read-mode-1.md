# 017 — planar read mode 1（color compare）

日期：2026-09-07
狀態：**READY**
前置：[`009`](009-planar-vga.md) §1（planar 讀寫路徑）、§4（缺口表把
read mode 1 列為「回 plane 值＋記錄」，觸發條件是「有程式用」）、
[`010`](010-exec-memory-reclaim-and-planar-write-modes.md) §2（write mode 3）。
動機：《銀河英雄傳說III SP》的五張清單畫面（提督、艦隊、敵情等）
底色畫得出來、框線畫得出來，**字一個都畫不出來**
（`~/cht/logh3/docs/re/49`）。

---

## 1. 證據

| 事實 | 證據 | 等級 |
|---|---|---|
| 遊戲的字元平面（`ds:D81E`，40×20 格）確實含 11 筆提督資料 | `~/cht/logh3` 收據 `s3plane` | confirmed |
| 字形常式 `2376:0E0D` 對每一格都被呼叫 | 收據 `s3jin` 的 watch（格子髒位元被 `2376:0456` 清掉） | confirmed |
| 畫字前設 `GC[5]=0x0B`（write mode 3 ＋ **read mode 1**）、`GC[7]=0`（color don't care 全 0）、`GC[8]=0xFF` | `2376:1835` 反組譯 | confirmed |
| 畫字的指令是 `and es:[di], al`——先讀（裝 latch ＋ 取回傳值）再寫 | `2376:10E3`／`2376:110A` 反組譯 | confirmed |
| 底色色號 9（plane 0 ＝ 1）的格子字畫得出來，底色 8（plane 0 ＝ 0）畫不出來 | 收據 `s3sortie` 逐像素統計 | confirmed |

`and es:[di], al` 的算式是 `讀回值 AND 字形位元組`，結果在 write mode 3
裡當有效遮罩用。硬體上 read mode 1 配 color don't care ＝ 0 表示
「哪個 plane 都不比」，於是**每個像素都算相符，讀回 `0xFF`**，
遮罩就等於字形本身。回 plane 值的話遮罩變成
`plane[gc[4]] AND 字形`——底色的那個 plane 是 0 的時候整個遮罩歸零，
字就一個像素都不寫。這正是「底色 9 有字、底色 8 沒字」的成因。

## 2. 語意（VGA 標準）

`gc[5]` bit3 選讀模式：0 ＝ read mode 0（回 `gc[4]&3` 選的 plane），
1 ＝ read mode 1（color compare）。read mode 1 的回傳值逐位元算：

```
color compare  cc = gc[2] & 0x0F
color don't care dc = gc[7] & 0x0F

result = 0xFF
for p in 0..3:
    if dc bit p == 0: continue          # 這個 plane 不參與比較
    if cc bit p == 1: result &= plane[p]
    else:             result &= ^plane[p]
```

也就是「回傳位元 i ＝ 1」代表像素 i 在所有**參與比較**的 plane 上都與
`gc[2]` 相符。`dc` 全 0 時沒有 plane 參與，結果恆為 `0xFF`。

讀取仍然照舊先把四個 plane 的位元組裝進 latch——latch 與讀模式無關，
read-modify-write 靠的是它。

## 3. 驗收

- 單元測試 `TestPlanarReadMode1ColorCompare`：
  - don't care ＝ 0 → 回 `0xFF`；
  - 只比 plane 0 且 compare ＝ 1 → 回 plane 0 的位元組；
  - 比 plane 0 與 1、compare ＝ `0b01` → 回 `plane0 AND ^plane1`；
  - 不論哪種模式，讀完 latch 都等於四個 plane 的內容。
- `009` §4 缺口表把這一列改成「已實作（`017`）」。
- 對拍：`~/cht/logh3` 的清單畫面在改動後要畫出提督名字；
  **執行器一改，所有收據重跑**（`~/cht/logh3/CLAUDE.md` 的硬規則）。
