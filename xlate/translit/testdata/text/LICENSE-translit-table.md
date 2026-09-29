# 英語人名譯音表（text/translit-table.tsv）授權與出處

`text/translit-table.tsv` 改作自中文維基百科頁面「Wikipedia:外語譯音表/英語」的「人名」表。

- 頁面：<https://zh.wikipedia.org/wiki/Wikipedia:外語譯音表/英語>
- 所用版本：revision id 86761742（2025-04-09T13:39:43Z），
  <https://zh.wikipedia.org/w/index.php?oldid=86761742>
- 下載日期：2026-09-29
- 作者：中文維基百科該頁面的貢獻者（完整名單見頁面歷史）
- 授權：創用 CC 姓名標示－相同方式分享 4.0 國際（CC BY-SA 4.0），
  <https://creativecommons.org/licenses/by-sa/4.0/deed.zh-hant>
- 頁面註明其內容整理自《世界人名翻譯大辭典》（新華社譯名室編，2007）。

## 修改說明

1. 只收「人名」表；地名表與其專用規則不收。
2. 改寫成 TSV：每個「元音列 × 輔音欄」一列；展開 rowspan，列標頭與欄標頭只保留國際音標，
   第一列記為「單獨輔音」，第一欄記為「無輔音」；空白格不收。
3. 一律轉為繁體字（頁面簡繁混用；轉換字見 `tools/translit.py` 的 `S2T`）。
4. 每格只保留一個用字；以夾注標示的女名用字移到 `zh_female`，詞首用字（「弗」）移到 `zh_initial`。
5. 儲存格、列標頭與欄標頭上的註腳名稱合併到 `note` 欄。
6. 頁面「說明」段落提到的附註用字「思」「茜」不在表格內，由 `tools/translit.py` 另行加入允許字集。

轉錄可用 `python3 tools/translit.py table --check --src <wikitext>` 以同一 revision 的 wikitext 重跑比對。

本檔與 `translit-table.tsv` 依 CC BY-SA 4.0 散布，與本專案程式的授權分開。
