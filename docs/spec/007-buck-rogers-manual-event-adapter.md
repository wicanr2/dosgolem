# 007 — Buck Rogers 手冊題目事件 adapter

狀態：**READY**  
日期：2026-09-20  
適用程式：DOS《Buck Rogers: Countdown to Doomsday》固定版本

## 1. 範圍

本規格只定義 `apps/buckrogers` 的純狀態核心：消費已由觀測層取得的原版可見字串事件，
建立題目 generation，以原版序數表橋接數字，並從外部 UTF-8 TSV 精確產生繁中顯示請求。

本規格**不**授權：掛接 `oracle.OnCall`、讀取執行中暫存器／堆疊、繪圖、2×／3× 選擇、
分頁、按鍵輸入、自動作答、答案解碼、DOS 記憶體寫入、原版返回值修改或存檔變更。
上述項目必須由後續 READY 規格個別授權。

## 2. 證據與位址空間

所有 `segment:offset` 均為 dosgolem 執行期 16-bit 實模式位址，不是 IDA linear EA。

| 證據 | 結論 | 等級 |
| --- | --- | --- |
| 兩次固定狀態錯答重播 | 題首、六筆 fragment、局部 clear 與終態 VRAM 決定性一致 | 已證實 |
| `0763:0424` dispatcher trace | 題首及六筆 fragment 的 caller 與可見字串 | 已證實 |
| `2A33:01ED` 題首 entry | 新 generation 最早可辨識邊界，先於局部 clear | 已證實於觀測路徑 |
| `2A33:0309` 題尾 guarded post-call | `word?` 完成後題目三欄已齊 | 已證實於觀測路徑 |
| `026F:029C` | 題目建立中途會發生的 Mode 13h 局部矩形清除 | 已證實 |
| `2A33:02B7..02D2` consumer | `record[+14] * 0x13 + DS:339B` 取得序數 slot | 已證實 |
| `0EC0:33AE..3459` | 1–10 的 19-byte 長度前綴 ASCII 序數表 | 已證實 |
| 題首是否涵蓋所有可能手冊入口 | 未命中時不得顯示；不能宣稱完整覆蓋 | 強推論 |

原版輸入雜湊：

- `START.EXE`：`58a34a38b1db455202d2d30daa82915982d7d905932b46bdc7371cb466226cf1`
- `GAME.OVR`：`3a4ad4856c08fe5973179f1d907feed1d870af99d08abd1cb884b316324f3cc0`
- runtime `0EC0:0000` 64 KiB：
  `28a0563b647ebe75a70e19648502e3fb9dc9379ada35bd397d99b104a4ce7c60`
- runtime `2A33:0000` 8 KiB：
  `7a2d9a3806d6ae49e5f8aa47a36767a38a0c1d1cbe744400e0525a98c4c2682c`

外部資料雜湊：

- `manual-events.tsv`：`bbf9c9058363b475858171b66048be6e2bb388eb73fa0fc7134e7092963efb6f`
- `manual-ordinals.tsv`：`fbf643ff2eecb1d5747f6d07d8845d2f67ed2e908ea060b8152dcb10bf0b538e`
- `manual.zh-TW.tsv`：`cea5474ebcf41eab7a8c64c5d2ca7639c871931fd4cd18ba3fa85549fc97d1bb`

## 3. Typed inputs 與 outputs

```text
Caller       = { Segment uint16, Offset uint16 }
Question     = { Page int, Heading string, OrdinalWord string }
DisplayRequest = { Generation uint64, EventKey string,
                   TextKey string, Translation string }
```

Collector 只接受三種呼叫：

1. `BeginEntry(caller, text)`：dispatcher entry 的 caller 與原始可見字串。
2. `PostCall(generation, caller, text)`：已由呼叫端完成 guarded post-call 驗證的事件。
3. `ClearEntry(caller)`：矩形清除 entry。

核心不自行讀取 CPU、stack、原版記憶體或答案。`DisplayRequest` 不得加入 answer、input、
memory mutation、save 或 renderer 欄位。

## 4. Collector 狀態機

狀態為 `idle | pending(generation, stage, page?, heading?, ordinal?) | visible(question)`。

1. 只有 `(2A33:01ED, "In the Log Book on page")` 精確命中才建立新 generation；generation
   單調加一，既有 pending 與 visible 立即清除。
2. 同 generation 的 post-call 必須依序精確命中：

   ```text
   2A33:021B page
   2A33:0231 "following the heading"
   2A33:027A heading
   2A33:02A5 "what is the"
   2A33:02E2 ordinal word
   2A33:0309 "word?"
   ```

3. 可見字串只正規化連續空白與首尾空白；page 仍須是 ASCII 十進位 1–999，heading 與
   ordinal word 不得為空。固定文句大小寫必須精確相符。
4. caller、順序、固定文句或欄位無效時，當前 generation poisoned；後續事件不得提交，
   只有下一個精確 begin 可復原。
5. generation 不符的延遲事件直接忽略，不得污染或 poison 現行 pending。
6. `026F:029C` 清除 visible，但保留 pending；其他 caller 不產生作用。
7. 只有 `word?` 完成且 page、heading、ordinal 齊全才回傳 Question；提交後 pending 清除，
   visible 設為該 Question。

## 5. TSV 與 resolver 契約

輸入採嚴格 UTF-8、無 BOM、tab 分隔；標頭必須逐欄相同且至少一筆資料。任何欄位空白、
欄數錯誤、解析錯誤或重複鍵都使整份 catalog 載入失敗，不做部分載入。

- ordinal：`number, ordinal_ascii, runtime_address, slot_hex`；必須恰有 1–10、數字與英文詞
  各自唯一，英文詞只接受 ASCII 小寫字母。
- event：`event_key, record_index, page, heading_ascii, ordinal, text_key`；題目身分、event key
  與 text key 各自唯一，page／ordinal 為正整數。
- catalog：`key, translation, source`；key 唯一，三欄皆非空，且不得有 event 未引用的孤兒。

解析順序固定為：generation 必須為正且等於 current generation → ordinal word 精確命中 →
`(page, heading, ordinal)` 精確唯一命中 → text key 存在 → 回傳 DisplayRequest。不得大小寫
折疊、模糊標題、相鄰頁碼、相近序數或 fallback。

## 6. 驗收矩陣

正式 Go 測試至少涵蓋：

- 第十八階段真實 `41 / Technical Skills / second` 事件序列，含中途 clear；
- 精確題首使舊 visible 失效、非精確題首不失效；
- 缺尾、亂序、重複、未知 caller、錯誤固定文句、非 ASCII／0 頁碼；
- stale generation 不污染新題，poisoned 只由下一個精確 begin 復原；
- 正式三份 TSV 載入後 `34 / Deimos Prison / tenth` 唯一命中；
- `Technical Skills / second` 未收錄時不顯示；heading 大小寫差異與未知 ordinal 不顯示；
- 重複 identity／event key／text key／catalog key、孤兒、BOM、無效 UTF-8、ordinal 缺號或
  多對一全部拒絕；
- 以 reflection 或等價檢查證明 DisplayRequest 只有四個顯示欄位。

所有 dosgolem 正式 packages 必須通過；這些內部測試只可把本規格標成 implemented，不能
宣稱畫面或正常玩家路徑 CONFORMED。

## 7. 玩家路徑、存檔與權利邊界

- 本核心沒有玩家輸入與存檔副作用；只把呼叫端已觀測的可見字串轉成顯示請求。
- 翻譯只從外部 TSV 載入，不內嵌手冊全文；dosgolem 儲存庫不加入原版 EXE、dump、掃描、
  字型或繁中 catalog。
- 正常玩家路徑、hook guard、畫面位置、倍率、分頁與失效 A/B 均是後續規格的必要工作。
- 發現新 caller、不同事件順序或無法以本狀態機表示的手冊入口時，本規格退回 DRAFT，先補
  原版證據；不得在程式加入特例猜補。

## 8. READY 審查結論

本規格的輸入、狀態轉移、輸出、失敗模式與內部測試均有第 18–21 階段證據或明示的
失敗即關閉設計支撐；未證實的「所有入口覆蓋」已由未命中不顯示與後續 player-path gate
隔離。因此只批准純核心實作，不批准 runtime／renderer 接線。
