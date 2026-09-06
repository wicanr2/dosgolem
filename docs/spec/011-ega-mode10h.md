# 011：EGA mode 10h 的平面式 VRAM

狀態：**可用**（Map Mask 寫入路徑已驗；圖形控制器的寫入模式未做，見 §4）
日期：2026-09-06
基礎：`docs/spec/007`（mode 0Dh，`pool-of-radiance-oracle` 分支）

## 1. 為什麼需要

智冠《三國演義》設 EGA mode `10h`（640×350、16 色）畫它的畫面。
四個位元平面疊在 `A0000` 同一段位址，由序列器的 Map Mask（索引 `02`）
決定哪些平面吃得到寫入。

**沒有平面式 VRAM 時症狀不是壞掉，是「一張正常但錯的圖」**：
四個平面互相蓋掉只剩最後一個，看起來仍是一張圖。

## 2. 與 `007` 的差別：尺寸要由呼叫端給

`007` 的 `IndexedEGA` 寫死 320×200。**平面式 VRAM 本身不記解析度**——
同一份平面資料在 mode `0Dh` 是 320×200、在 `10h` 是 640×350，
猜錯不會報錯，只會得到一張錯位但看起來像圖的東西。

所以改成 `IndexedEGASize(w, h)`，`IndexedEGA()` 保留為 320×200 的別名。
mode `10h` 一列 80 bytes、350 列 ＝ 28,000 bytes，放得進 64 KB 的平面。

## 3. 驗收

`AA.EXE` 答完裝置選單（`122` ＝ 無音樂／EGA／硬碟）跑到標題畫面：

| | 值 |
|---|---|
| 視訊模式 | `10h` |
| 寫過的 I/O 埠 | `3C4 3C5 3CE 3CF 3D4 3D5` |
| 平面式非零像素 | **180,715 / 224,000** |
| `Map Mask` 曾動過 | `true` |

`-dump-ega 640x350=out.png` 產出的圖是可辨識的完整畫面
（發行商識別畫面，中文字與圖形都正確）。


## 4. 圖形控制器（埠 `3CE`／`3CF`）

實作的暫存器：

| 索引 | 名稱 | 作用 |
|---|---|---|
| `00` | Set/Reset | Enable Set/Reset 打開的平面要填的顏色位元 |
| `01` | Enable Set/Reset | 哪些平面改用 Set/Reset 的顏色，不用 CPU 寫進去的值 |
| `02` | Color Compare | read mode 1 的比對顏色 |
| `03` | Data Rotate／Function Select | bit0–2 右旋次數、bit3–4 邏輯運算（換／AND／OR／XOR）|
| `04` | Read Map Select | read mode 0 讀哪一個平面 |
| `05` | Mode | bit0–1 寫入模式、bit3 讀取模式 |
| `07` | Color Don't Care | read mode 1 比對時忽略哪些平面 |
| `08` | Bit Mask | 這一次寫入動得了哪幾個位元 |

### latch

`[HARD]` **從 A0000 段讀一個 byte 會把四個平面同時鎖進 latch。**
之後的寫入只動 Bit Mask 打開的位元，其餘位元從 latch 補回去。
把讀取當成沒有副作用的話，`read-modify-write` 的那個 read 就白做了
——而那正是 EGA 畫圖的標準寫法。

少了 latch 的症狀**不是壞掉，是一張有規律雜訊的圖**：遮罩外的位元
被歸零，畫面上是一條一條的直線。看起來像時序問題，不像少了一個暫存器。

`Bit Mask` 的重置值是 `FFh`。預設 0 的話畫面一片空白，
而空白看起來像「還沒畫」不像暫存器沒初始化。

### 寫入模式

| 模式 | 資料來源 |
|---|---|
| 0 | CPU 的值（可右旋）；Enable Set/Reset 打開的平面改用 Set/Reset 的顏色 |
| 1 | latch 原封不動寫回去（搬圖形用，CPU 的值完全不參與）|
| 2 | CPU 值的第 n 位決定平面 n 整個 byte 是 `FF` 還是 `00`（畫單色圖形）|
| 3 | **EGA 沒有，VGA 才有。不實作。** |

四種模式都在 Map Mask 之後才落地。

驗收在 `internal/machine.TestEGABitMaskKeepsUntouchedBits`／
`TestEGASetResetPicksColour`／`TestEGAWriteMode1CopiesLatches`，
以及智冠《三國演義》的君主選擇畫面（地圖、六位君主頭像、
「中平六年元月」的直排年號）。

## 5. 沒做的部分



  ⚠ **已經觀測到它畫錯了。** 標題畫面之後的開場插圖 dump 出來是
  **單色點狀**——16 色模式的插圖不該長這樣。成因是那張圖走
  Set/Reset 或寫入模式 2，四個平面的顏色資訊沒有進來，只剩一個平面
  有資料。

  這一次錯得看得出來（一眼就知道不對）。**下一次不一定**——
  寫入模式錯在某些圖上只會讓顏色偏掉，那種錯會安靜地通過對拍。
  要做畫面對拍就得先補這一段。
- **調色盤暫存器（`3C0` Attribute Controller）。** `-dump-ega` 用 EGA
  預設的 16 色，遊戲自己設的顏色沒攔。要對拍顏色得補這一段。
- CRTC（`3D4`／`3D5`）：只記埠寫入，不影響取樣。
