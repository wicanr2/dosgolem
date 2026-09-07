# 007 — VGA 16 色平面模式（mode 12h）

日期：2026-09-05
狀態：**READY**

---

## 1. 為什麼要做

第二個案例是《臥龍傳》（松崗 DOS/V 版，`KI.EXE`）。它開機第一件事是
`mov ax, 12h / int 10h` ——**640×480、16 色、四平面**，
不是第一個案例那種 mode 13h 的線性 256 色。

現況：`internal/machine` 把 `A0000` 之後當成一塊線性緩衝區，
`Indexed()` 直接複製 320×200 個 byte 出來。拿它去看平面模式的畫面，
得到的是**四個平面交錯壓在一起的 bit 圖**——看起來像雜訊，
而且不會有任何錯誤。

## 2. 證據

`KI.EXE` 的 `sub_1EB6C`（IDA linear `0x1EB6C`，檔案位移 `0xED6C`）：

```asm
mov  ax, 12h
int  10h                 ; 640×480 16 色平面
mov  dx, 3DAh / in al,dx ; 重設 3C0 的 flip-flop
mov  dx, 3C0h
mov  al, 14h / out / al=0 / out          ; 色彩選擇 = 0
16 次：out index / out value             ; 調色盤暫存器 0..0Fh = 0..0Fh（identity）
mov  al, 11h / out / al=0 / out          ; 邊框色 = 0
mov  al, 20h / out                       ; 開顯示
retn
```

緊接著的清畫面常式（`0x1ED93`）：

```asm
es = A000h
3C0: index 11h = ah                       ; 邊框色
3CE: index 05h = 03h                      ; ← 寫入模式 3
3CE: index 00h = ah                       ; Set/Reset = 顏色
3CE: index 03h = 00h                      ; 功能選擇 = 取代、不旋轉
3CE: index 08h = FFh                      ; 位元遮罩 = 全開
di = 0 / cx = 4B00h / rep stosw           ; 19200 words = 38400 bytes = 640×480/8
```

文字繪製（`docs/re/28` §1，臥龍傳專案）：

```asm
mov dx, 3CFh / mov al,<顏色> / out dx,al  ; Set/Reset 資料
mov al, es:[di]                            ; ← dummy read，載入 latch
movsb                                      ; 寫字型 byte
add di, 4Fh                                ; 下一列（列距 50h = 80 bytes）
```

三件事因此定案：

1. **寫入模式 0 與 3 都會用到。** 清畫面用模式 3，文字用模式 0 ＋ Set/Reset。
2. **latch 是行為的一部分。** 那個 dummy read 不是多餘的——
   沒有 latch 的話「沒被字型位元覆蓋的平面會被清掉」。
   模型少了 latch，畫出來的字會只剩一個平面的顏色。
3. **畫面是 640×480，但遊戲的內容只有 640×400。**
   文字常式的 VRAM 段是 `A0C8h` ＝ `A000h` ＋ `0xC80` bytes ＝ 40 列，
   所以遊戲的 y 原點是螢幕第 40 列。這與臥龍傳專案 `tools/parity_crop.py`
   在 DOSBox-X 截圖上**量到**的 y 偏移 40 是同一件事，兩個獨立來源對上。

## 3. 要做什麼

### 3.1 記憶體：`A0000`–`AFFFF` 改走平面

`Read8`／`Write8` 在平面模式時把 `A0000`–`AFFFF` 導到 `internal/machine/vga.go`，
其餘位址不變。四個平面各 64 KB（`[4][65536]uint8`），
**不佔用 `Mem` 那 1 MB**——平面模式下 `Mem[A0000:]` 不再有意義。

判斷「現在是不是平面模式」用 BDA 的視訊模式（`SetVideoMode` 已經在記）：
`0Dh`／`0Eh`／`0Fh`／`10h`／`11h`／`12h` 是平面，其餘不是。
**不要用「程式有沒有寫過 3CE」判斷**——那會讓同一支程式在切模式前後
落進不同的行為，而且切回去時不會恢復。

### 3.2 暫存器

| 埠 | 索引 | 名稱 | 用途 |
|---|---|---|---|
| `3C4`/`3C5` | 02 | Map Mask | 哪幾個平面吃這次寫入 |
| `3CE`/`3CF` | 00 | Set/Reset | 模式 0（開啟時）與模式 3 的顏色來源 |
| | 01 | Enable Set/Reset | 哪幾個平面改用 Set/Reset 的值 |
| | 02 | Color Compare | 讀取模式 1 的比較值 |
| | 03 | Data Rotate / Function | bit 0–2 ＝ 右旋位數、bit 3–4 ＝ 00 取代／01 AND／10 OR／11 XOR |
| | 04 | Read Map Select | 讀取模式 0 讀哪一個平面 |
| | 05 | Mode | bit 0–1 ＝ 寫入模式、bit 3 ＝ 讀取模式 |
| | 07 | Color Don't Care | 讀取模式 1 忽略哪幾個平面 |
| | 08 | Bit Mask | 哪幾個 bit 由 CPU 資料決定，其餘取 latch |
| `3C0` | 00–0F | 調色盤暫存器 | 4 bit 像素值 → 6 bit DAC 索引 |
| | 10 | Mode Control | bit 7 ＝ P5/P4 來自色彩選擇 |
| | 14 | Color Select | 高兩位／高四位 |

`3C0` 是**同一個埠先寫索引再寫資料**，由一個 flip-flop 決定這次是哪一種；
讀 `3DA` 會把 flip-flop 重設回「下一次寫的是索引」。
索引 bit 5 是「調色盤位址來源」，不影響我們要算的值。

### 3.3 寫入的四種模式

令 `latch[p]` 是四個平面的 latch，`data` 是 CPU 寫出去的 byte，
`mm` 是 Map Mask，`bm` 是 Bit Mask，`esr` 是 Enable Set/Reset，
`sr` 是 Set/Reset，`rot`／`fn` 來自 Data Rotate。

- **模式 0**：`v = ror(data, rot)`；每個平面 `p`：
  `src = esr[p] ? (sr[p] ? FF : 00) : v`，再套 `fn` 與 `latch[p]`，
  最後 `out = (alu & bm) | (latch[p] & ^bm)`。
- **模式 1**：直接把 latch 原封不動寫回去（`bm` 與 `fn` 都不參與）。
  這是「搬一整塊」的快路徑。
- **模式 2**：`src = (data>>p)&1 ? FF : 00`，其餘同模式 0（`rot` 不參與）。
- **模式 3**：`bm' = bm & ror(data, rot)`，
  `src = sr[p] ? FF : 00`，`out = (src & bm') | (latch[p] & ^bm')`。
  ⚠ **模式 3 不看 Enable Set/Reset**，四個平面一律用 Set/Reset 的值。

四種模式最後都再過一次 Map Mask：`mm` 沒開的平面不寫。

### 3.4 讀取

- **模式 0**：回 `plane[readMapSelect][off]`，同時四個平面全部載進 latch。
- **模式 1**：回「四個平面在 Color Don't Care 之下等於 Color Compare」的位元圖。
  同樣載 latch。

**latch 一定要在讀取時載入**，包括程式只是為了載 latch 而做的 dummy read。

### 3.5 取畫面

```go
func (m *Machine) Planar() (w, h int, px []uint8)   // 4 bit 像素值 0–15
```

寬固定 640；高由視訊模式決定（12h ＝ 480、10h ＝ 350、0Eh ＝ 200）。
列距 80 bytes，**不讀 CRTC**——本專案的目標程式不改 CRTC
（實測 `KI.EXE` 一次都沒寫過 `3D4`）。

`Indexed()` 維持 mode 13h 的語意不動；平面模式下改叫 `Planar()`。
**兩支分開，不要讓 `Indexed()` 依模式改回傳大小**——
呼叫端拿到長度不對的切片時多半是安靜地畫錯，不是報錯。

色號 → RGB 走 `Palette()`：4 bit 像素值先經 `3C0` 的調色盤暫存器
變成 6 bit DAC 索引，再查 DAC。`KI.EXE` 把調色盤設成 identity，
所以實務上就是 0–15 直接查 DAC 的前 16 格；**但公式要寫對**，
不能寫死成 identity——寫死的話換一支程式會得到「顏色全錯」而查不出來源。

## 4. 怎麼驗

| 項目 | 驗收 |
|---|---|
| 四種寫入模式 | 單元測試，逐項對照本規格 §3.3 的算式（含 latch 未被覆蓋的平面不變）|
| latch | 「dummy read 之後寫一個 byte」要保住其他平面：漏掉 latch 的實作會把它們清成 0 |
| 讀取模式 1 | Color Compare／Don't Care 的四種組合 |
| 清畫面 | 跑 `KI.EXE` 到 `sub_1EB6C` 之後，四個平面應該全 0 而不是全 `FF` |
| 整體 | `Planar()` 出來的 640×480 裁掉上下各 40 列，與臥龍傳專案的原版截圖逐點比 |

⚠ **單元測試綠不代表接對了。** 這一層的接線點是 `Read8`／`Write8` 的分支：
測試直接呼叫 `vga.Write()` 的話，把 `Read8` 的分支拿掉照樣全綠。
接線要另外測（給機器設 mode 12h，用 `Write8` 寫，用 `Planar()` 讀）。
