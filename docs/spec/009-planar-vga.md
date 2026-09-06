# 009 — VGA/EGA planar 寫讀路徑（mode 10h/12h）、滑鼠事件登錄、AH=0Dh

日期：2026-09-06
狀態：**READY**（§1 planar、§2 int 33h AX=000Ch 登錄、§3 AH=0Dh）／
**DRAFT**（§4 已知缺口）
動機：`GIN3PS.EXE` 已進入 mode 12h（VGA 640×480×16）且到達互動畫面
（滑鼠輪詢開始），但機器沒有 planar 語意，無法倒出畫面
（`~/cht/logh3/docs/re/03` §4）。
前置：[`008`](008-ems-paging.md) §5 的判定（planar ＝ 渲染語意，要 spec）。

---

## 1. planar 寫讀路徑（READY）

### 證據

| 行為 | 證據 | 等級 |
|---|---|---|
| 本體切 mode 12h；開頭 OPEN.EXE 用 mode 10h | probe 報告「視訊模式 12h」；re/02 §3（mode 10h）| confirmed（軌跡） |
| 程式寫 3C4/3C5（sequencer）、3CE/3CF（GC）、3D4/3D5（CRTC） | re/02/re/03 埠清單 | confirmed |
| OPEN.EXE 用 `xor al, es:[di]` 做 read-modify-write 繪圖（mode 10h，檔案偏移 0x1405） | objdump（re/02 §3）| confirmed（bytes） |
| 調色盤走 int 10h AH=10h（×37）＋既有 DAC 3C8/3C9 | 軌跡；machine.go DAC | confirmed |

### 語意（VGA GC／sequencer 標準模型，與程式無關）

- 機器新增 **4 個 plane × 64 KB** 的 VRAM（`vram [4][0x10000]`），
  與平坦 Mem 並存。平坦 Mem 的 A0000 段**照樣寫**（給 -watch／-dump-linear
  這些偵錯工具用），但 planar 模式下畫面的真相在 plane。
- planar 生效時機：`SetVideoMode` 切到 0Dh/0Eh/0Fh/10h/11h/12h。
  切回文字或 13h 就關。**切換不清 plane**（真機 BIOS 會清，但那要模擬
  整套 mode-set；程式切完都會自己畫滿，有差再說——DRAFT §4）。
- 埠狀態：3C4 索引 → 3C5 寫 seq 檔（用得到的是 seq[2] ＝ map mask）；
  3CE 索引 → 3CF 寫 gc 檔（gc[3] 旋轉/函數、gc[4] 讀 plane 選擇、
  gc[5] 寫模式、gc[8] 位元遮罩）。
- **調色盤**：16 色模式的色彩鏈是「4 位元色號 → 屬性調色盤（AttrPal，
  預設 identity）→ DAC」。`int 10h AH=10h` 實作 AL=00h/10h（設單一
  屬性暫存器）、AL=02h/12h（設 DAC 整份／一段）——證據：鏈內 ×37，
  實測 AL 用到 00/02/10/12 四種（其餘記 unimplemented；實測 AL=09/17
  各 1 次，語意未知，先記）。
  直接寫 3C8/3C9 的既有 DAC 路徑不變。
- **寫**（CPU 寫 A0000–AFFFF，位址對 plane 內偏移）：
  - write mode 0：CPU 位元組旋轉（gc[3] 低 3 位）→ 依 gc[3] bit3-4 與
    latch 做 replace/AND/OR/XOR → 依 gc[8] 位元遮罩與 latch 混合 →
    寫進 map mask（seq[2]）開著的 plane。
  - write mode 1：latch 直接寫進 map mask 開著的 plane（CPU 資料不看）。
  - write mode 2：CPU 位元組的 bit n 展開成 plane n 的 0x00/0xFF，
    其餘同 mode 0（含位元遮罩）。
  - write mode 3：旋轉後的 CPU 位元組 AND gc[8] 當遮罩，CPU 資料由
    set/reset 展開——**先照 mode 0 實作並記一筆**（未見證據，§4）。
- **讀**：任何 CPU 讀都先把四個 plane 該位址的位元組裝進 latch
  （read-modify-write 就靠它）；read mode 0（gc[5] bit3=0）回
  gc[4] 選的 plane 的位元組。read mode 1（color compare）**不實作**，
  回 plane 值並記一筆（§4）。
- latch 初始值 0。

### probe：`-dump-ega <png>`

依 BDA 的目前模式選尺寸（12h ＝ 640×480、10h ＝ 640×350），
每像素從四個 plane 取 bit 拼 4 位元色號，過既有 `Palette()`（DAC 6 位元
→ 8 位元）輸出 PNG。色號陣列也存 `.bin`（640×480），對拍在色號空間做
（比照 mode 13h 的判準）。

## 2. int 33h AX=000Ch：登錄事件 handler（READY）

- 證據：GIN3PS 鏈內呼叫 1 次（re/03 缺口表 #2）。
- 最小語意：**記下 mask 與 handler 位址**（Mouse 加欄位），
  AX 不動、視為成功。無頭執行器的滑鼠是注入式（直接改 Mouse 狀態），
  目前不產生事件回呼；真的需要事件驅動時（程式只掛 handler 不輪詢）
  再實作「注入事件時 far call handler」。
- 依據：鏈內滑鼠輪詢（AX=3）有發生且有回報（re/03 §1），
  所以「只登錄不回呼」目前不擋路——這個判斷是**強證據**，不是假說。

## 3. int 21h AH=0Dh（flush disk buffers）（READY）

- 證據：鏈內 ×3（re/02 §4 #4）。
- 語意：我們的檔案寫根本不做（Wrote 清單），沒有可 flush 的東西；
  **靜默收下**（清 CF），不再記 unimplemented——它不是「沒實作」，
  是「沒事可做」。

## 4. 已知缺口（DRAFT）

| 項 | 狀態 | 觸發條件 |
|---|---|---|
| read mode 1（color compare） | 回 plane 值＋記錄 | 有程式用 |
| write mode 3 | 照 mode 0 實作 | 有程式用 |
| CRTC（3D4/3D5）視窗位移、雙倍掃描 | 不實作（dump 假設線性 plane、原尺寸）| 畫面對拍發現位移/縮放 |
| 模式切換清 plane | 不清 | 殘影造成對拍差異 |
| 屬性控制器（3C0）與 16 色 palette 對映 | 不實作，DAC 直接當 256 色用 | 16 色畫面顏色錯 |

## 5. 驗收

1. 單元測試：map mask 只寫選中的 plane；位元遮罩與 latch 混合；
   write mode 2 展開；讀取裝 latch、write mode 1 用 latch；
   旋轉＋XOR 函數；mode 13h 行為不變（既有測試全綠）。
2. 整合（缺檔 skip）：原版 GIN3.COM 鏈跑到 mode 12h 互動畫面，
   `-dump-ega` 出 PNG，內容不為全黑（A0000 有 38 KB 非零的
   事實要反映在色號陣列上）。
3. `tools/go.sh build ./...` 與 `test ./...` 全綠。
