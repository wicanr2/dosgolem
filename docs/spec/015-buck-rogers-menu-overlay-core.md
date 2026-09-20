# Buck Rogers 倍率中立的功能選單覆繪核心

狀態：**CONFORMED**

本規格只核准把 `013-buck-rogers-menu-overlay-scale-prototype` 已驗證的單列選單覆繪邏輯，
從診斷命令移入 `apps/buckrogers` 的純核心。核心由呼叫端明示倍率，建立 `xlate.Layer`、
驗證字模墨跡與安全矩形，並回傳 content-safe 幾何結果；它不掛 runtime hook、不送輸入、
不讀寫原版記憶體，也不替產品選擇 2× 或 3×。

## 玩家可見範圍與排除項目

核准範圍只有已證實的 `PICK RACE` steady／Down 畫面所需單列事件。呼叫端仍負責用
`MenuCatalog` 精確解析事件、挑選當前可見 event key、載入正式譯文、palette 與字型；核心
只接受 typed entry，不自行猜測 variant。

本規格不核准：正常玩家路徑接線、預設倍率、手冊段落、分頁、動態失效、存讀檔重建、
自訂反白色、原版 EXE／資料修改，或讓譯文進入比較、查找、輸入與序列化路徑。

## 證據、版本與位址空間

- steady／Down indexed framebuffer SHA-256：
  `d0f70a73b80b1998c0744ae2bb2903dba4783104fbfccc3ded41d70e8eacc1cd`／
  `efaa3d88ea3c1f05aff308b86f796c4634845304c63eb5c92cc7e1d0a1a278d0`。
- 8-bit RGB palette SHA-256：
  `045796505f7ec3115cec8632ca7a29e6391687a2a013198e38dd68dd5b3564eb`。
- GOLEMFNT SHA-256：
  `553df09b6accd101995a2781819ba9bee84a6220f0db5bba4437df1ccc458fe7`。
- `menu-events.tsv`／`menu-text-safe-rects.tsv`／`menu.zh-TW.tsv` SHA-256：
  `973a6a1e247e7d9e16518a1a66266f340666d32e785f3f6f6652a890e830da2e`／
  `3e37efcae4222ffb9ea11b19ea93dc5b016e11df974253d58da797365f6c982f`／
  `ca3319830adb7b34a048b498d8d0466b38b8fb6f418e5244a3a467e77ea68077`。
- 原版畫面是 dosgolem 320×200 Mode 13h indexed framebuffer；event caller 若出現在上游
  inventory，使用 dosgolem runtime `segment:offset`。本純核心不接受或產生位址。
- 工具版本基線為 dosgolem commit `41917c85007efe17154cb92422cba3fe6539ad88`。

上述雜湊、兩畫面事件集合、2×／3× 幾何與 selected 色彩均為**已證實**；正式產品倍率仍為
**未知／待使用者決定**。本規格不得把兩種倍率皆通過測試升格成產品選擇。

## Typed input 與輸出

每筆輸入必須明示：event key、text key、繁中譯文、原版背景／前景色號、logical 清除矩形
`x/y/width/height`、logical draw anchor、容量、行數與 overflow policy。整批另明示：

- 非 nil 的 16×16 GOLEMFNT；
- 256 色 8-bit RGB palette；
- 正整數 `scale`，沒有預設值；
- 供 `xlate.Layer.Draw` 使用的 RGBA buffer 由呼叫端建立，核心不得持有原版狀態。

核心輸出 `xlate.Layer` 與逐事件 content-safe 幾何：scale 後 clear rectangle、draw anchor、
ink rectangle 與 containment。輸出不得保存英文／繁中全文。

## 規則與狀態轉移

每筆目前只接受一列 8×8 logical cell：`height=8`、`width` 為 8 的倍數、`draw_y=y`、
`draw_x>=x` 且差為 8 的倍數、`line_count=1`、`overflow=single-line-reject`。前置縮排以全形
空白加入 `Stamp.Text`，讓 stamp 從 `x` 清除完整原文，但可見繁中從 `draw_x` 起畫。

- `cells=width/8`，`prefix=(draw_x-x)/8`；必須有 `cells-prefix=capacity` 且整段文字不超容。
- 2× 時 glyph offset 為 0；3× 時為 4。其他正整數倍率使用
  `max(0, (8*scale-16)/2)` 置中 16×16 字模，不建立產品預設值。
- `xlate.Stamp` 使用原版 `background`／`foreground` 索引同一 palette 的 RGB，狀態為
  `Shown`。色號 0 與 15 同為黑色時，selected row 必須維持黑底黑字，不得增補游標。
- 所有非空白字模 ink 必須落在該筆 scale 後 clear rectangle；各 clear rectangle 不得重疊。
- 建立期間不修改傳入 framebuffer；只有呼叫端顯式呼叫回傳 layer 的 `Draw` 才覆繪。

任一 entry 通過後加入 layer；整批任一 entry 無效時整體回錯，呼叫端不得使用部分結果。
核心是純建構流程，沒有 generation、frame clock、save 或 input 狀態。

## 失敗模式

nil／非 16×16 font、非正整數倍率、空 event／text key、空譯文、負座標、超出 320×200、
不符單列格線、容量不符、overflow policy 不符、缺字、空 ink、ink 越界或矩形重疊，一律
回傳錯誤與 nil 結果。不得靜默截斷、替換字元、改色、縮字或只畫可用的前半批。

## 垂直鏈與權利邊界

正式鏈仍是：原版 guarded post-call → `MenuCatalog` exact identity → `DisplayRequest` →
本核心 typed entry → `xlate.Layer.Draw` → 輸出端 RGBA。這一輪只實作後兩節之間的純核心；
正常玩家路徑接線仍受專案 DRAFT 與倍率決策阻擋。

核心與測試不含原版 framebuffer、palette、手冊、字型二進位或譯文全文。真實輸入與 PNG／
JSON 收據只留在被忽略的專案 `workplace/`，不得加入 dosgolem Git 或公開發行物。

## 驗收與停止線

- 單元測試覆蓋 2×／3×、selected 黑底黑字、prefix anchor、缺字、倍率、字型尺寸、容量、
  越界、重疊及整批原子失敗。
- `cmd/buckrogers-overlay-prototype` 改用本核心；移除命令內相同行為，不保留第二套規則。
- 第 32 階段四組真實輸入重生後，PNG／JSON 必須逐 byte 等於既有基線：JSON SHA-256
  `ae796fc1…ed13`、`06c342ab…97140`、`0e9ee862…6d3`、`2c0dd1d0…758`。
- 全部正式 packages test／vet 與 `apps/buckrogers`、診斷命令 race detector 通過。

通過後本規格可標為 CONFORMED。這只證明倍率中立核心與既有離線收據一致；不外推為
runtime 中文已接通、2×／3× 已決定，或整款遊戲完成中文化。

## 符合性紀錄（2026-09-21）

- 正式核心／測試 SHA-256：
  `f7b442e6d3d234b946a629e2c889e090ecc22eb346f53116322422d48b45ddee`／
  `75fafe6115aabe5525dfd39238fdafc6b7491ad4399fe158bb20731abc7eac52`。
- 診斷命令已改用正式核心，主程式 SHA-256 為
  `c2e2e1e2e385c86a2b70224777d315319a7e44d544dd117033a4623ef57310bd`；命令內原有 stamp
  建構、ink 計算與安全矩形 containment 的第二套邏輯已移除。
- steady／Down × 2×／3× 以固定原版 framebuffer、palette、GOLEMFNT 與三份正式 TSV
  重生；四組繁中 PNG、base PNG 與 JSON 全部逐 byte 等於第 32 階段基線。四份 JSON
  SHA-256 仍為 `ae796fc1…ed13`、`06c342ab…97140`、`0e9ee862…6d3`、`2c0dd1d0…758`。
- 單元測試覆蓋 2×／3×、prefix anchor、selected 黑底黑字、缺字、倍率、字型尺寸、容量、
  越界、overflow、重疊、重複事件與整批 nil/error；核心不保存產品預設倍率。
- 排除既有非正式 `workplace/` 草稿後，全部正式 packages test／vet，以及
  `apps/buckrogers`／`cmd/buckrogers-overlay-prototype` race detector 全數通過。
- 本符合性仍不包含正常玩家路徑 renderer、frame invalidation、倍率決策或手冊覆繪。
