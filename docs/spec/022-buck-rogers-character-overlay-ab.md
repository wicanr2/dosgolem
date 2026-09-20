# Buck Rogers 性別與職業繁中覆繪倍率 A/B

狀態：**CONFORMED**

本規格核准擴充離線 `buckrogers-overlay-prototype`，以第三十五至四十階段已證實的性別／職業
exact identity、正式繁中 catalog、logical text-safe rectangles 與真實 framebuffer，產生
2×／3× 診斷 PNG 及 content-safe 幾何收據。它不授權 runtime renderer 或產品倍率預設。

## 輸入與畫面集合

- `gender-steady`：提示、selected male、normal female。
- `gender-down`：提示、normal male、selected female。
- `class-steady`：提示、selected Rocket Jock、其餘四個 normal 職業。
- `class-down`：提示、normal Rocket Jock、selected Warrior、其餘三個 normal 職業。
- framebuffer 必須由 dosgolem 正常 BIOS Enter／Down 路徑產生；palette 使用既有 8-bit RGB
  `machine.Palette()` 輸出，不能再作 6→8 位元轉換。

## 幾何與失敗即關閉契約

- 每個安全矩形固定為 `(column×8,row×8,original_length×8,8)`；draw anchor 等於該 exact
  identity 的起點，不得跨列、擴張或自動縮字。
- 只允許單列、容量等於矩形 cell 數、`single-line-reject`；譯文超容量即拒絕。
- 每批只能包含該終態實際可見的 exact variant；同列 normal／selected 不得同時覆繪。
- `BuildMenuOverlay` 仍是唯一倍率中立核心；每筆都須零缺字、ink contained，整批須零矩形
  重疊、安全矩形外零像素差異。
- selected 事件沿用原版 palette 色號，黑底黑字仍不可見；診斷工具不得自行美化。

## 驗收

- 四個狀態 × 2×／3× 各獨立重生兩批，JSON、繁中 PNG 與 base PNG 逐檔相同。
- 每份 JSON 固定輸入 SHA-256、canvas、事件次序、色號、clear／ink rect、containment、缺字、
  overlap、outside diff 與 PNG SHA-256。
- 專案 geometry validator、正反例測試、全部正式 dosgolem 套件測試、`go vet`、相關 race
  detector 通過後才能升為 CONFORMED。
- 本規格不選定 2×／3×，不接 runtime Layer，不驗證轉場、連續幀或其他角色分支。

## 符合性紀錄（2026-09-21）

- 四個原版 framebuffer 各重播兩次逐 byte 相同；八種倍率輸出各重生兩批逐檔相同。
- 八份收據皆為零缺字、零 overlap、零 outside diff，所有 event 均 contained。
- 專案 70 項測試與真實 verifier 通過；全部正式套件測試、`go vet`、相關 race detector 通過。
