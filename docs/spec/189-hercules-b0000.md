# 189 — Hercules 的畫面在 `B0000`：第三塊顯示記憶體

狀態：**READY**
日期：2026-09-15
動機：softworld_san（三國演義）的裝置選單有 Hercules／EGA 兩個選項；
選了 Hercules 之後 probe 報「A0000 全零、B8000 全零」，看起來像程式沒畫，
其實是看錯地方（softworld_san `docs/re/00` 第三輪；該專案 Issue #4）

---

## 1. 事實

| 顯示卡 | 顯示記憶體 | dosgolem 原本看得到嗎 |
|---|---|---|
| VGA mode 13h、EGA 平面模式 | `A0000` | `Indexed()`／`IndexedEGA*` |
| CGA 圖形、所有文字模式 | `B8000` | `-dump-cga`、probe 的「B8000 非零 bytes」|
| **Hercules（HGC）** | **`B0000`**（頁 0；頁 1 在 `B8000`）| **看不到** |

Hercules 是 1 bpp，四個 bank 交錯：第 r 列在 `bank(r%4)*0x2000 + (r/4)*stride`。
**幾何不是常數**：標準是 720×348（每列 90 bytes），但程式寫 6845 CRTC
（`3B4h`／`3B5h`）就能改——三國演義寫 R1＝40、R6＝102、R9＝3，開成
**640×408、每列 80 bytes**，與它的 EGA 畫面同一個幾何；`3B8h` 在 `0Ah`／`8Ah`
之間切換是雙頁翻頁（bit 7 選頁 1 ＝ `B8000`），兩頁內容逐位元組相同。
拿 720×348 去解會得到一張有規律的雜訊（試過），不會報錯。

`B0000` 是線性記憶體，不走平面路徑（`planarOn` 只接管 `A0000`–`AFFFF`），
所以 `WatchWrites`／`Bytes` 本來就看得到它——缺的是「當畫面解出來」與
「probe 報那一塊有沒有東西」。

## 2. 加了什麼

- `machine.Hercules()`／`oracle.Hercules()`：顯示中那一頁解成 0／1 陣列，
  寬高與每列位元組數由 `HerculesGeometry()` 從 CRTC 讀（R1×16、R6×(R9+1)、
  R1×2），沒寫過就退回 720×348／90；`3B8h` bit 7 選頁。不看圖形／文字位元。
- `machine.HerculesNonZero()`／`oracle.HerculesNonZero()`：頁 0 非零位元組數。
  **0 才是「沒畫」**——這是 probe 判斷用的數字。
- `cmd/probe`：每次都印 `B0000 非零 bytes N / 32768（Hercules 圖形頁 0）`，
  與 `B8000` 那一行並排；`-dump-herc <png>` 存 720×348 灰階圖。

## 3. 測試

`internal/machine/hercules_test.go`：

- **有畫面不得回假零**：往 `B0000` 寫兩個位元組（第 5 列最左、第 347 列最右），
  非零計數 ＝ 2，解碼後亮點 ＝ 2 且落在 (0,5)／(719,347)。
- **幾何跟著 CRTC**：寫 R1＝40、R6＝102、R9＝3 之後是 640×408／80，最後一個
  像素解得到；`3B8h←8Ah` 之後改讀頁 1；非零計數固定看頁 0。
- **EGA／VGA 不退步**：寫進 `A0000`／`B8000` 的位元組不會被算進 Hercules。

softworld_san 那一側的收據：`TestZZHerculesBootDrawsB0000`（`-tags oracle`）
用裝置答案 `112` 跑 15 億道指令，`B0000` 非零 30,309／32,768、解成 640×408
之後是三英圖的抖色版；同一段指令用 EGA（`122`）跑 `B0000` 是 0。

## 4. 邊界

- 頁 1（`B8000`）與 CGA 撞位址；`Hercules()` 照 `3B8h` bit 7 選頁，
  `HerculesNonZero()` 固定看頁 0。
- 文字模式下 Hercules 的文字頁也在 `B0000`，解出來像雜訊——判斷「有沒有畫」
  用非零計數，判斷「畫的是圖還是字」要看 `3B8h` bit 1，這一版沒做。
- CRTC 只認 R1／R6／R9，其餘暫存器（水平同步位置等）不影響解碼。
