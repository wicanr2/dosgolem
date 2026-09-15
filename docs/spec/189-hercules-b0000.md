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

Hercules 是 720×348、1 bpp、每列 90 個位元組，四個 bank 交錯：
第 r 列在 `bank(r%4)*0x2000 + (r/4)*90`。`B0000` 是線性記憶體，不走平面
路徑（`planarOn` 只接管 `A0000`–`AFFFF`），所以 `WatchWrites`／`Bytes` 本來
就看得到它——缺的是「當畫面解出來」與「probe 報那一塊有沒有東西」。

## 2. 加了什麼

- `machine.Hercules()`／`oracle.Hercules()`：`B0000` 頁 0 解成 720×348 的
  0／1 陣列。只解碼，不看 `3B8h`／`3BFh` 的模式位元。
- `machine.HerculesNonZero()`／`oracle.HerculesNonZero()`：頁 0 非零位元組數。
  **0 才是「沒畫」**——這是 probe 判斷用的數字。
- `cmd/probe`：每次都印 `B0000 非零 bytes N / 32768（Hercules 圖形頁 0）`，
  與 `B8000` 那一行並排；`-dump-herc <png>` 存 720×348 灰階圖。

## 3. 測試

`internal/machine/hercules_test.go`：

- **有畫面不得回假零**：往 `B0000` 寫兩個位元組（第 5 列最左、第 347 列最右），
  非零計數 ＝ 2，解碼後亮點 ＝ 2 且落在 (0,5)／(719,347)。
- **EGA／VGA 不退步**：寫進 `A0000`／`B8000` 的位元組不會被算進 Hercules。

## 4. 邊界

- 頁 1（`B0000+0x8000`）與 CGA 撞位址，這裡固定取頁 0；程式用 `3B8h` 切頁時
  要另外處理。
- 文字模式下 Hercules 的文字頁也在 `B0000`，解出來像雜訊——判斷「有沒有畫」
  用非零計數，判斷「畫的是圖還是字」要看模式埠，這一版沒做。
