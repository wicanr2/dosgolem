# 204 — 疊字層：可恢復的圖面

狀態：**CONFORMED**（限通用圖面層與 §3 的本次驗證）
日期：2026-10-01（8×8 定案後的契約審查）
前置：[`202-translation-overlay.md`](202-translation-overlay.md)、[`203-baked-text-watchers.md`](203-baked-text-watchers.md)
實作與驗收入口：[`../../xlate/art.go`](../../xlate/art.go)、[`../../xlate/art_test.go`](../../xlate/art_test.go)，整合於 `layer.go`／`watch.go`。

## 1. 範圍、證據與舊說訂正

本規格提供預先放大的 RGBA 圖面，與文字分組。原版圖塊、貼圖位置、素材、動畫與前後順序由呼叫端提供。
主專案 `/home/anr2/cht/psychic-war/docs/spec/024-hd-theme.md` 另管載入及玩家驗收；204 READY 不代表整款 HD 完成。

| 等級 | 已有事實及來源 |
|---|---|
| 已證實 | 202 的透明格不恢復；203 同 Owner 多列可能缺列；小比對區不能證明全部繪製格有效。主專案 `docs/re/038-hd-theme-feasibility.md` §23 |
| 已證實，限正常畫面及錯誤模型 | 固定 CAF0h 遭遇狀態，正常 F3 後有 47 個非黑背景像素恢復，永久遮格少畫 423 個放大 HD 像素。研究 §29 |
| 使用者定案 | 8×8 格、原點 `(0,0)`；美女圖在後、框線在前，排版沿用原版。研究 §31、024 §3.6 |
| 規格設計 | 明確原版基準、逐格完整比對、按 Key 補齊整組、資產不進快照；以下 §2 |

舊 DRAFT「沿用全部既有行為」已被 §23 否定。舊來源與 SHA-256 保存於主專案
`workplace/hd/source-before-art-20261001/manifest.json`，共 14 檔；歷史收據不改寫為新來源。
新審查及實作驗證索引為主專案研究 §32。
原版輸入與位址基準見研究 §29／§31：SCREEN／MENU PBL 雜湊、320×200 色號及 960×600 原型。
受控矩陣是 16×16 人工像素座標，不是 IDA 線性位址或原版對拍。本規格不新增遊戲反組譯語意。

## 2. 契約

### 2.1 登記及圖字分組

`Layer.Art []*Stamp` 保存圖面；既有 `Layer.Stamps` 保存文字。Stamp 新欄位：

```go
Art       bool
Pix       []uint8 // 非預乘 RGBA，已放大
PixScale  int
Reference []uint8 // 原版色號，逐列排列，寬 Cells*CellW、高 CellH
Order     int     // 小的先畫，相同時依 Key 排序
```

- `AddArt(s *Stamp) error` 驗證 Art=true、非 nil、非空 Key、正整數 Cells／CellW／CellH／PixScale，
  矩形完整在畫面內，尺寸乘法不溢位；Reference 長度恰為 `Cells*CellW*CellH`，
  Pix 長度恰為 `Cells*CellW*CellH*PixScale*PixScale*4`。不合格回錯且不改已有圖／字。
- 登記時複製 Stamp、Reference、Pix、固定 Transparent，避免輸入切片後續修改污染基準。
- 同 Key 替換原筆；不同 Key 可重疊，依 Order、Key 繪製。舊 DRAFT 的同組重疊移除不適用圖面，
  否則角色會刪掉需要恢復的背景。文字 Add／重疊移除仍依 202／203。
- `Add` 遇 Art=true 時轉交 AddArt；錯誤以 `OnDrop(s,"art")` 通知。
  正式載入器使用能回傳錯誤的 AddArt。`DropArt(key)`／`ClearArt()` 只清圖面。
- 文字捲動與 Frozen 不移動、凍結圖面；原版幀內容是圖面有效性判準。

### 2.2 不可變基準與恢復

基準只來自 Reference；不可從第一次顯示的當前畫面採樣或從未分級筆記猜補。
每個有效 Frame 逐格完整比較原版色號：任一像素不同便本幀不畫該格，全相同便可繪製。
前一幀被遮與否不影響本幀；全部遮格時仍保留登記、基準與資產。
不套用文字錨定格移除或永久 Transparent。呼叫端明示的 Transparent 是固定不畫區，
與本幀可恢復遮格分開保存。首次繪製也驗所有格，Watcher match 只決定整組啟用。

indexed 或 rgb 長度不符畫面尺寸：不更新文字、不執行 watcher，所有圖面本幀停止顯示；
有效幀回來再比較，不能索引越界。
二維圖由呼叫端切成橫排 Stamp；主專案對齊原版 `(0,0)` 的 8×8 格線，必要時補足邊緣。
通用 xlate 不內建 PBL 或遊戲位置。

### 2.3 RGBA、倍率及順序

- alpha=0 不改目標；255 覆蓋；中間值採來源覆於目標（source-over），輸出仍為非預乘 RGBA。
  不可把透明目標當不透明黑底。
- Draw 倍率與 PixScale 不符便不畫，以 `OnDrop(s,"scale")` 每段不符狀態通知一次；保留登記。
  正確倍率恢復繪製，並允許下次新的不符狀態再次通知。
- scale 非正、目標長度不符或乘法溢位：回 false，不寫目標。
- 先依 Order／Key 畫圖面，再畫文字。文字仍沿用 202 的三倍數倍率；圖面使用自己的合法整數倍率。
- draw 回值表示實際寫入過像素，全遮格／全透明不算有繪製。
- 主專案先合成「人物在後、框架在前」背景；sprite 依原版動作提供來源與順序。
  不以 watcher 登記／補列順序推定前後關係。

### 2.4 圖面 Watcher 與多列完整性

Watcher 增加 `Art bool`、`ArtKeys []string`；預設 false 保留 203 文字路徑。
ArtKeys 是 Make 輸出的全部非空、唯一 Key，固定為該組完整集合。

- 每幀完整比對圖面 watcher 的 Want；match 不符便整個 Owner 本幀停用，基準仍保留。
  避免其他畫面剛好有相同黑格時冒出背景裝飾。
- match 相符且 Owner 的 Key 集合完整：不再 Make；逐格比對仍每幀執行。
- match 相符但少列、多列或尚未登記：Make，先驗全部是合法 Art 且 Key 集合等於 ArtKeys，
  成功才原子替換整組，不讓一列存活阻止缺列恢復。
- nil、非 Art、重複／缺／額外 Key、非法資產、或與別的 Owner 同 Key 衝突：整組拒絕，
  不部分更新；本幀 Owner 停用，以 OnDrop 的 `"art"` 通知。其他圖字不變。
- Unwatch／UnwatchAll 後 Owner 沒有啟用依據，其圖面不顯示；呼叫端可再 ClearArt 回收。
  直接 AddArt 且 Owner 空的圖面只用逐格基準，不需 watcher。

### 2.5 快照、還原與切換

快照增加可省略的 Art 中繼資料：Key／Owner／矩形格數／PixScale／Order。
Pix、Reference、可恢復遮格、Watcher 閉包不進快照；舊文字 JSON 欄位及讀取維持 202／203。
沒有 Art 的舊快照仍可還原。還原成功後清除目前 Art 與 Owner 啟用狀態；
呼叫端重新 Watch／AddArt，下一個有效 Frame 驗基準及完整集合，不能從還原畫面猜基準。
還原失敗不得清掉原圖。關閉主題由呼叫端停止繪製或 ClearArt／Unwatch，再啟用依登記流程重建。
這是轉譯層快照，不改原版存檔格式、記憶體或規則。

### 2.6 READY 審查

| 舊缺口 | 明確行為 | 必要反例 |
|---|---|---|
| 永久遮格 | 每幀與 Reference 完整比較，全遮仍保留 | 單像素遮格後恢復、正常 F3 非黑恢復樣本 |
| 同 Owner 缺列 | ArtKeys 完整集合，成功才整組替換 | 刪一列留另一列、快照重建、錯誤 factory |
| 首次基準污染 | 當前幀不能建立基準，每格都驗 Reference | match 僅第一列，第二列已被遮 |
| 像素契約不明 | 精確長度、PixScale、非預乘 alpha、Order | 截短資產、錯倍率、透明及半透明目標、逆序登記 |

輸入、狀態、錯誤與驗收已明確，依已有原版觀察及使用者定案允許實作。
本次先依 READY 契約實作再執行 §3，結果見 §3.1；當時 READY 的完整來源與雜湊
保存於主專案 `workplace/hd/source-art-v2-20261001/manifest.json`。

## 3. 驗收

1. 圖／字重疊互不移除，文字最後；不同圖面逆序登記仍按 Order／Key 繪製。
2. 一像素改動遮整格，其他格保留；下一幀恢復、全遮後也恢復；輸入基準切片修改不污染登記。
3. 僅比第一列的 watcher 不能蓋住已遮的第二列；match 不符停用整組，恢復再顯示。
4. Owner 缺列能補齊、不重複；錯誤 factory 整組拒絕，活著的另一列不能遮掩錯誤。
5. alpha=0 不改目標；半透明分別驗透明與不透明目標；錯倍率不畫、通知一次，正確倍率恢復。
6. 快照無資產；還原清舊 Art，重新登記完整恢復；舊文字測試通過，失敗還原不清圖。
7. 錯尺寸、無基準、越界及壞目標不崩潰／不部分更新；202／203 全部既有測試通過。
8. 主專案保存的正常遭遇→F3／不按鍵畫面：Art 與獨立 8×8 消費者逐像素核對，
   中文、動態、非黑恢復與對照各給數字。只核對已保存正常幀，不取代正式前端玩家路徑。
9. 1,000 格 Art 加文字及文字單獨的 Frame／Draw 成本分別量測，不設門檻；
   工具／版本、輸入及輸出雜湊由主專案研究 §32 收據保存。

### 3.1 本次通過證據（2026-10-01）

Go 1.24.13、既有 `psychicwar-go-ebiten` 映像、CPU 1。39 項 xlate 測試通過
（含子案例 53 筆）；主專案轉譯層 8 項測試通過（含子案例 133 筆）。
保存的正常遭遇／F3／不按鍵共 6 幀，與獨立 PBL＋8×8 消費者的圖面、合成、中文及動態不符均 0；
恢復 47 個非黑背景像素，永久遮格負對照差 423 個放大像素。
25 列始終完整，兩個分支各只建立一次，不靠重建整圖掩蓋逐格恢復。

不透明區段批次複製前後共 19 個輸出檔完整位元組相同；本次 1,000 格加文字的
Frame 約 0.099 ms、Draw 約 0.209 ms；只含文字約 0.003／0.031 ms。
此為共用主機本次成本量測，非跨平台幀率保證；原版、文字及 sprite 完成範圍不因此擴張。

現行收據位於主專案 `workplace/hd/redraw/art-plane-v2-independent-20261001.json`，
Go 來源／輸出收據 `art-plane-v2-20261001.json`；重跑命令、原版來源及歷史快照映射見研究 §32。
快照／恢復等合約項目由 §3.1 所列單元測試核對；保存正常幀不能代替主題的實際前端存讀檔驗收。

## 4. 停止線

圖面層不解 PBL、縮放、旋轉、搜尋座標或猜動作，不處理新遊戲規則。
本規格不授予素材散布權。正式角色、場景及平台接入／美術品質仍由主專案驗收。

## 5. 指定文字採用已繪圖面背景（READY，2026-10-06）

原§1–§4的CONFORMED範圍保持。本節由主專案原版ROOM0.PBL #5與可丟棄合成原型審查，
允許局部實作；入口為[研究038 §175](../../../../docs/re/038-hd-theme-feasibility.md)。
主專案使用者已定案原位文字與自然HD美術，無新增外觀決定、存檔欄位或資料格式。

### 5.1 輸入與行為

新增`Layer.DrawWithBackground(dst, scale, missing, background, useBackground)`。
background為同尺寸RGBA圖面，useBackground為呼叫端提供的`func(*Stamp) bool`。
原`Draw`轉呼叫本方法並傳nil，保持既有輸出。通用層不內建遊戲鍵、PBL或來源位置。

Shown文字且呼叫端明示使用時，文字有效格的每個背景像素採background中同位置的RGB，
但只接受alpha=255。alpha<255、背景長度不符、nil回呼或回呼拒絕時，該像素仍填原Stamp.BG。
字模、前景色、透明格、裁切、定色、Watcher、失效及Snapshot/Restore全部沿既有契約。
background及回呼不進快照、不修改Stamp、輸入圖面、原版RAM或畫面。
dst無效／倍率非法依Draw拒絕；回呼只對Shown文字求值，不處理Layer.Art。

主專案只對已選定的無字重繪來源授權此背景；未重繪／原版文字素材不能全域套用。
圖面須由原版完整Watcher與8×8遮格先繪好；不接受舊幀殘留的圖面作本幀背景。

### 5.2 證據與驗收

Go1.24.13、原版PW.EXE SHA88321206d5400b2276aba0f268e2daaa8355a733843dd9e89f9e7116ae690c49，
ROOM0.PBL SHA2b2f58c9b716a54bf826dbc9c90237a32458fe49abab52869d5353bbff34d111。
原版runtime CS:IP 0161:8588／DS:BX 0161:92BE來源及原位(4,124)已confirmed。
完整64000色號重建差0、44機器欄位及DOS相同；限實際F7/F8輔助起點後普通按鍵。
字型資料獨立核對166字模像素及2480背景像素，固定原位合成期望差0。

驗收含原Draw回歸、回呼拒絕、非法背景尺寸、alpha0／254回退、alpha255採圖、
前景色恰等背景色仍照字模繪製、透明格、Pending、來源背景不可變及快照相同。
主專案另驗HD關閉／重開、英文切換、原版源失效、撤圖與載回；未通過前不稱本節CONFORMED。
