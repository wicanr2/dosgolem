# 217 — Buck Rogers 手冊多行 presenter 核心

狀態：**CONFORMED（純核心；非原版畫面 A/B）**  
日期：2026-09-21  
前置：[`216-buck-rogers-manual-presentation-lifecycle.md`](216-buck-rogers-manual-presentation-lifecycle.md)、
[`202-translation-overlay.md`](202-translation-overlay.md)、專案 specs 002／005 與第 83、85、86 階段收據。

## 目的與邊界

建立 `apps/buckrogers` 的純 presentation core，將已驗證的 `ManualPresentationEvent` request 依
正式手冊 layout 繪製為 2×／3× RGBA。它只讀 indexed framebuffer、palette、layout、catalog、font
及 lifecycle values；不得建立 oracle hook、寫 DOS VRAM／記憶體、送鍵、改答案、檔案、存檔或 host UI。

本規格只涵蓋 core 與 synthetic-frame 測試，不授權 command、正常遊戲 runtime、正式 GOLEMFNT、
原版畫面 A/B 或玩家可見完成聲明。

## 已證實幾何與資料邊界

- `manual-overlay-layout.tsv` 的唯一正文 layout 是 clear `[7,312)×[72,184)`、文字 anchor `(16,72)`、
  36 欄×14 行、capacity 504、line height 8、spacing 0、`single-page-reject`。這是保留原版頁碼、
  英文標題與序數的 confirmed 契約。
- `ManualPresentationEvent`（spec 216 CONFORMED）只在 exact begin、active-context clear、exact
  catalog-hit request 發射；request 含 generation、event key、text key、translation，不含原文或答案。
- `xlate.Layer.Draw` 只填滿 stamp 的格子。文字格是 `[16,304)`，比 clear rect 的 `[7,312)` 窄，故
  單一文字 layer 不足以清除兩側原版墨跡；必須另以背景 layer 覆蓋每列完整 305×8 clear band，再以
  文字 layer 覆蓋 36 個 8×8 格。

原版位址、fixed state 與題目事件仍由 specs 008／216 固定；本 core 沒有新增原版位址或語意。

## 擬定 typed 契約

```go
type ManualOverlayLayout struct { /* 唯一正式 TSV 的已驗證幾何 */ }

func LoadManualOverlayLayout(name string, data []byte) (*ManualOverlayLayout, error)
func NewRuntimeManualOverlay(layout *ManualOverlayLayout, catalog *Catalog,
    font *xlate.Font, scale int) (*RuntimeManualOverlay, error)

func (o *RuntimeManualOverlay) Apply(event ManualPresentationEvent) error
func (o *RuntimeManualOverlay) Frame(indexed []byte, palette [256][3]uint8)
func (o *RuntimeManualOverlay) Draw(indexed []byte, palette [256][3]uint8) (rgba []byte, missing []rune, drew bool)
```

constructor 必須驗證唯一 layout、non-nil／non-empty catalog、16×16 font、scale 為 2 或 3，並對
catalog 全部 translation 的每個 rune 檢查 32-byte glyph；任一缺字使整個 constructor 失敗。
request 也必須在 catalog 內以 event key、text key、translation 完整相等，不能接受外部合成值。

## 擬定 lifecycle 與圖層規則

1. `begin(generation)` 只接受嚴格遞增的非零 generation，建立 pending state 並移除 core 自己的
   background／text 兩個 layer 的所有上一代 stamp。
2. `clear(generation)` 只接受目前 generation。若仍 pending，保持 pending；若 visible，移除兩個
   layer 並轉為 cleared。cleared 後同 generation request、stale generation、零 generation、duplicate
   begin／clear／request 全部失敗即關閉。
3. `request(generation, request)` 只接受目前 pending generation 及 exact catalog request。translation
   以 rune row-major 分成 14 行、每行最多 36；建立 14 個文字 stamp（x=16、y=72+8×row、36 cells）及
   14 個無字背景 stamp（x=7、同 y、1 cell、width=305），再原子替換為 visible。
4. 每次 `Frame` 對兩個 layer 分別以同一 raw indexed framebuffer／palette 定色與檢查；`Draw` 先
   `ScaleIndexedRGBA`，再畫背景 layer、最後畫文字 layer。未用文字格仍由 36-cell stamp 填背景，以
   清除同列原版英文；兩側 margin 則由背景 layer 處理。

## READY 證據審查與失敗即關閉

- loader 拒絕 UTF-8／BOM／schema／row count／空欄、任一 geometry／capacity／policy／evidence drift，
  並重新證明正文格線完整包含於 clear rect。
- core 拒絕 nil／錯誤 layout、font、catalog、scale、缺 glyph、非 UTF-8／空／超 504 translation、
  catalog 不相符 request 及所有非法 lifecycle transition；錯誤不得留下部分 stamp。
- 純核心測試必須用 fixture font／frame 驗證 14 行、full clear rect containment、2×／3× RGBA、
  begin／pending-clear／request／visible-clear／新 generation、catalog miss（沒有 request）與
  defensive-copy。這些不是原版 A/B 收據。

第 83 階段 formal TSV 已將每個欄位與 504／505 邊界 fail-close；第 86 階段 lifecycle queue 已
CONFORM，沒有需要猜測的 begin／clear 順序。`xlate.Layer` 的 `Frame`／`Draw` 已證實每個 stamp
只在自己 cells 內填 background：用獨立 background layer 正好能覆蓋 confirmed 305-pixel clear band，
而 text layer 保持 36 個 8-pixel cells，不需要改動既有通用 renderer。typed input、state、失敗模式、
synthetic 內部驗收及權利停止線均已明確，因此本規格升為 **READY**，只授權本節的純核心實作。

## CONFORMED 純核心收據

2026-09-21，`apps/buckrogers/manual_overlay_runtime.go` 與對應單元測試已依本規格完成：

- `LoadManualOverlayLayout` 只接受正式 14 欄、唯一一列及全部 confirmed 幾何；schema、列數、
  clear rect、504 容量與 overflow/evidence 任一漂移皆拒絕。
- `NewRuntimeManualOverlay` 在繪製前驗證完整 manual catalog、16×16／32-byte glyph、2×或3×倍率；
  空、超過 504、非 UTF-8 或缺字均不能建立 presenter。
- `Apply` 已測得 begin → pending clear → request → visible clear → new generation，並拒絕 catalog
  miss、stale request、重複或錯誤 generation 及未知事件。`Actions` 回傳 defensive copy。
- synthetic indexed frame 的測試已量到兩種倍率各有 14 個背景 stamp 與 14 個 36-cell text stamp，
  504 rune 逐列分為 14×36，RGBA 差異只在批准 clear rect；兩側 margin 由背景 layer 清除。
- 隔離 Docker 中的 `go vet ./apps/buckrogers` 與 `go test -race ./apps/buckrogers` 均以 exit 0
  完成。本收據僅證實純核心；尚未接入 command、正式字型、正常玩家路徑或原版／繁中 A/B。

因此本規格升為 **CONFORMED（純核心）**。後續接線仍須另建 DRAFT，並取得同狀態正常玩家
收據後才可聲稱該手冊畫面已中文化。
