# 206 — Buck Rogers 姓名提示執行期繁中顯示請求

狀態：CONFORMED

## 範圍與證據

由修正後 #99,999,999 state 經四次 Enter 與 `N` 的正常 BIOS 路徑，原版於
#101,919,217 進入 `0763:0424`，並於 #101,931,640 完成姓名提示事件：

- caller `0763:0826`；長度 16；SHA-256
  `246a64eabf869f90773d848870a32766bc838b7e766d88fce33dfa4e99d06676`；
- 背景 0、前景 13、row 24、column 0；
- 本機一次性探針以完整 SHA-256 匹配後取得原文 `Character name: `，探針已刪除且原文不納入
  正式 repository catalog。

第四十四階段另證實後續 `A`／`B` 是玩家輸入的單 byte 回顯，Backspace 直接清像素而沒有文字
事件。姓名提示是唯一核准的靜態 identity；玩家姓名不是譯文。

## 契約

1. 正式 events TSV 只收錄上述一筆 exact identity，對應繁中「角色姓名：」。
2. resolver 必須同時比對長度、SHA-256、caller、背景、前景、row 與 column；任何漂移皆 miss。
3. request 只含 event key、text key 與譯文，不得回寫原版記憶體、鍵盤、姓名 buffer 或存檔。
4. 玩家姓名回顯、空字串、Backspace 與相同 caller 的其他內容一律不得由此 catalog 命中。
5. 本規格只建立 `DisplayRequest`；沒有 text-safe rectangle 時不得啟用 overlay 繪製。

## 驗收

- 專案資料驗證器拒絕 BOM、欄位錯誤、重複 key／identity、來源 inventory 漂移、漏譯、孤兒 key、
  控制碼與非 NFC 譯文。
- Go 正反例固定 exact hit，並改動 identity 每一欄確認 miss。
- 正常五鍵路徑須維持 183 events，只產生一筆姓名提示 request、其餘 182 筆 miss；輸入 `A` 的
  路徑須維持該一筆 request，單 byte 回顯仍為 miss。
- 原版 indexed framebuffer、BIOS keys 與終點狀態不得因 catalog 存在而改變。

## READY 審查

原版事件、完整內容、正常路徑與動態姓名邊界均已有已證實證據；exact resolver 與 content-free
收據已有 CONFORMED 先例。本規格不新增玩家規則、座標推論或 renderer，故可進入實作。

## CONFORMED 收據

- `LoadNamePromptCatalog` 重用完整 identity resolver；命令列以成對旗標載入並合併 catalog，
  未提供安全矩形時不啟用 overlay。
- 五鍵正常路徑為 183 events、1 request、182 misses；加送 `A` 後為 184 events、1 request、
  183 misses，玩家回顯仍為 miss。
- 兩條路徑各重跑一次，JSON 與 indexed framebuffer 均逐位元相同。正式 catalog SHA-256：
  events `52fc88bafb3dec5a4cb6becfa54104e4bf741303cb0b7106ad1c39c52546d4fd`；譯文
  `2e45bffd335e3a5546507e99d010a9d2a8425b29e9b2cea7a74fe324e5a4f0d2`。
