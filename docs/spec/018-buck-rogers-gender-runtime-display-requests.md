# Buck Rogers 性別選擇執行期繁中顯示請求

狀態：**CONFORMED**

本規格核准把專案正式 `gender-events.tsv`／`gender.zh-TW.tsv` 接到既有 exact-match catalog
核心與 `MenuRequestWatcher`，由已完成的 `0763:0424` guarded post-call 產生繁中
`DisplayRequest`。它不載入字型、不繪圖、不清除原文、不送輸入、不修改原版狀態，也不選
2×／3×。

## 證據與用語來源

- 初始性別畫面四筆 identity：專案 `text/post-race-events.tsv`。
- Down／Up／Escape identity：專案 `text/gender-selection-events.tsv`。
- 正常玩家路徑收據：專案第 35、36 階段研究紀錄。
- 「性別」由中文說明書掃描 `SCAN0352_005.jpg` 的角色資料欄原圖核對；提示採介面動詞
  「選擇性別」。男性／女性由已核對的原版選項語意採標準繁中介面譯詞，維持 DRAFT 譯文
  等級，不冒稱手冊逐字摘錄。
- runtime 位址皆為 dosgolem `segment:offset`，不是 IDA 線性位址或檔案偏移。

## catalog 契約

`gender-events.tsv` 沿用 `menu-events.tsv` 的十欄 schema、型別、連續 sequence、唯一
event key、唯一完整 identity 與無 BOM UTF-8 契約。完整 identity 仍是：

```text
original_length + original_sha256 + caller + background + foreground + row + column
```

`gender.zh-TW.tsv` 沿用三欄翻譯 schema。事件表與翻譯表必須雙向完整，不得缺鍵或有孤兒鍵。
正式性別 inventory 必須恰有七個唯一 identity：提示、兩筆初始 normal 選項、selected male、
short normal male、selected female、short normal female。Up 的 selected male 重用既有 identity，
不得建立重複列。

載入實作必須共用既有解析器，不得複製第二套 identity 驗證或 Resolve。`LoadMenuCatalog` 與
`LoadGenderCatalog` 可以保留各自檔名錯誤語境；合併 catalog 時任一 identity 衝突都要失敗。

## 命令與 watcher 契約

`buckrogers-text-receipt` 新增成對的 `-gender-events`／`-gender-translations`。只給其中之一必須
拒絕。menu 與 gender 可各自省略；兩者同時提供時先各自嚴格載入，再合併成一個 resolver
交給既有 `MenuRequestWatcher`。不得新增第二個 recorder 或 watcher。

命中時仍只輸出 `event_key`、`text_key`、`translation_runes`；不輸出英／中文全文。未知事件
只保留 content-safe event、增加 catalog miss 且不產生 request，不能沿用前一筆結果。

## 真實路徑驗收

### Enter→Enter→Down→Up

- 固定第 36 階段排程與 #101,000,000 終點。
- 18 個完成事件全部精確命中，產生 18 個 request；零 pending、drop、catalog miss。
- 性別請求依序為提示、normal male、normal female、selected male、normal male、selected female、
  normal female、selected male。
- 兩次完整 JSON 收據逐 byte 相同。

### Enter→Enter→Escape

- 固定第 36 階段排程與 #101,000,000 終點。
- 前 14 筆共同事件及第 15 筆取消男性反白共產生 15 個 request。
- 其後七筆尚未登錄的返回功能選單 identity 必須各計一次 miss，且不得產生空白、重複或沿用
  的性別 request；總數固定為 15 requests、7 misses、零 pending／drop。
- 兩次完整 JSON 收據逐 byte 相同。

## 測試與停止線

- 測試涵蓋 malformed gender TSV、完整 identity 逐欄失配、catalog 合併衝突、成對旗標、nil／
  單 catalog 相容性、切片隔離及真實收據。
- 專案 verifier 必須交叉驗證第 35、36 階段 inventory，不能只驗新表自身一致。
- 全部正式 packages test／vet、`apps/buckrogers` 與 receipt command race detector 通過；兩條
  真實路徑重播符合後才能升為 CONFORMED。
- 本規格完成不代表畫面已有中文。renderer、文字安全矩形、失效世代與倍率仍需獨立 READY
  規格及玩家可見同狀態驗證。

## 符合性紀錄（2026-09-21）

- `gender-events.tsv`／`gender.zh-TW.tsv` SHA-256：
  `a8c96c8edc393c26717a5d05bc46fc031d25a8d0c269f3fe1c6b617ee0dda297`／
  `8fd64b9a15fc94aff73a8f7d06ff100b406763b09ac562989a82344eeeac6857`。
- 共用 catalog 核心／測試 SHA-256：
  `2880e93813a5c0ddfb196071d7f1783c78a0a82ad2cc6b708d2a92a1cc89c778`／
  `4776fb6f4ff9c6164675318ace465dd2fb8f1af1fd8565452134763eb02ae936`。
- receipt command／測試 SHA-256：
  `ed6e7b6537a9be3fe332f49821adee0d91c51a1f4510ca32648ed1b0f8fad5c9`／
  `0578ec0c9a333cefd31f8d00fa125fc1fc1c57dc769d66e8d18632604d4e6edc`。
- Down→Up 兩份 18-event／18-request 收據逐 byte 相同，SHA-256 均為
  `d5bafaf03cd1fb3c5f54346239aab7754b7ceb828789436d73957cca9854faa4`，零 catalog miss。
- Escape 兩份 22-event／15-request 收據逐 byte 相同，SHA-256 均為
  `0b7518bc5ea5583954f33b153c77d900b77ca09f7584de9e21325f618890628b`；返回選單七筆
  未登錄 identity 精確形成七次 miss，沒有沿用性別請求。
- 專案 55 項資料／收據測試與真實 verifier 通過；全部正式 dosgolem packages test／vet、
  `apps/buckrogers` 與 receipt command race detector 通過。
