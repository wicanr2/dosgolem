# Buck Rogers 選單執行期顯示請求 watcher

狀態：**CONFORMED**

本規格只核准把既有 `TextRecorder` 完成事件送入既有 `MenuCatalog`，在原版
`0763:0424` guarded post-call 後產生繁體中文 `DisplayRequest`。它不繪圖、不清除原文、
不決定倍率、不送輸入，也不修改原版狀態。

## 已證實前置與固定輸入

- 原版 dispatcher 與 far-return／SS／SP 完成閘門：
  `docs/spec/008-buck-rogers-manual-runtime-watcher.md`。
- 選單 exact identity 與 catalog：
  `docs/spec/009-buck-rogers-menu-display-request.md`。
- 正式事件表 SHA-256：
  `38bc0fa693fa5ee4bed3209548a0dc67c1270b28657f0801e42645b70fd370e9`。
- 正式翻譯表 SHA-256：
  `ca3319830adb7b34a048b498d8d0466b38b8fb6f418e5244a3a467e77ea68077`。
- 所有位址都是 DOS runtime `segment:offset`；不得當成檔案偏移或 IDA 線性位址。

本 watcher 不另行解釋原文、caller 或畫面欄位。事件與 catalog 的 typed schema、唯一性、
大小寫、數值界線及失敗模式完全繼承上述兩份 READY 規格。

## 狀態轉移

watcher 擁有一個 `TextRecorder`、一個可為 nil 的唯讀 `MenuCatalog`、已提交請求序列與
catalog miss 計數。

1. dispatcher entry 必須把 caller、`SS:SP`、六個原始參數、原文字節與 entry step 原樣交給
   `TextRecorder.ObserveDispatchEntry`。entry 當下不得解析或提交請求。
2. 每一道候選 instruction 都交給 `TextRecorder.ObserveInstruction`。只有 recorder 新增一筆
   完成事件，watcher 才能進行一次 catalog lookup。
3. catalog 精確命中時追加一筆 `DisplayRequest`；次序必須與完成事件一致，每筆事件最多一筆。
4. catalog 為 nil 時只記錄事件，不算 miss；catalog 非 nil 而 identity 未命中時增加 miss，
   不得建立空白請求或沿用前一筆請求。
5. recorder 的 pending、drop 與完成事件是唯一生命週期權威。重疊 entry、字串長度超過 255、
   return address／SS／SP guard 失敗，均沿用 recorder 的失敗即關閉行為。

`Requests()` 與 `Events()` 必須回傳切片副本。呼叫者修改副本不得污染 watcher。watcher 不得
保存第二份英文原文，也不得把譯文回寫 recorder、原版資料、比較、查找或序列化路徑。

## 收據契約

`buckrogers-text-receipt` 保留既有純事件模式。只有 `-menu-events` 與
`-menu-translations` 同時提供時才載入 catalog 並輸出 `requests`；只提供其中一個必須拒絕。
可選 `-want-requests` 以失敗即關閉驗證數量。

每筆 request 收據只含：

```text
event_key, text_key, translation_runes
```

不得輸出英文原文、繁中全文、原版 pointer、答案、輸入資料、renderer 或記憶體寫入內容。
既有 `events` content-free metadata 保留，供逐筆回查完成事件。

## 固定狀態驗收

以第 27 階段同一個 #99,999,999 狀態，在 #100,010,000 排入正常 BIOS Enter，跑至既有終點：

- recorder 恰有九筆完成事件、零 drop、無 pending；
- watcher 恰有九筆 request、零 miss；
- request 順序逐筆等於正式 `menu-events.tsv` 的 event／text key；
- 第 3、9 筆分別是 `race.option.terran` 與 `race.heading.terran`，但都使用
  `race.terran`；
- 收據 schema 不含任何全文欄位。

套件測試須覆蓋 entry 不提交、完整 guard 提交、無關 instruction、錯誤 SS／SP、重疊 entry、
未知 identity、nil catalog、請求切片隔離及命令列成對參數。全部正式 packages test／vet 與
Buck Rogers race detector 通過後，本子規格可標為 CONFORMED；玩家可見覆繪規格仍維持 DRAFT。

## Conformance 收據

2026-09-20 以本規格固定狀態與正常 Enter 重跑，得到九筆事件、九筆 request、零 drop、
零 pending、零 catalog miss；正式 request verifier 逐筆核對 event／text key 與譯文字數。
全部正式 packages test／vet、`apps/buckrogers` 與 receipt command race detector 通過。
