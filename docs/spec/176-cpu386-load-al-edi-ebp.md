# 176 — CPU386 從 EDI+EBP 載入 AL

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 118`](../re/118-fd2-ini-input-byte-sib-load.md)

- 擴充無前綴 `8A 04 2F`：`mov al,byte ptr [edi+ebp]`。
- 有效位址以 32 位加法計算 `EDI+EBP`，並透過 DS descriptor 讀取一個 byte。
- 成功時只覆寫 EAX 的低八位 AL；EAX 高 24 位、EDI、EBP、來源記憶體及旗標
  必須保持不變。
- 讀取越界時失敗，且不得修改 EAX、EDI、EBP、記憶體或旗標。
- operand-size、segment、repeat prefix、其他 ModRM `04` SIB 與其他尚未列入的
  `8A` SIB 形狀維持失敗即關閉（fail-closed）。

## 驗收條件

- CPU 測試覆蓋有效讀取、AL 局部寫入、32 位位址回繞、狀態不變及 descriptor
  越界失敗。
- 固定雜湊 FD2 從現有啟動路徑自然執行 `0x3F2AC`，確認 AL 等於
  `DS:[EDI+EBP]` 並抵達 `0x3F2AF`。
- 有界自然執行收據記錄下一個失敗即關閉位址；不得順帶放寬該指令。

## 驗收收據

- `go test -p=1 -v ./internal/cpu386 ./internal/machine -run
  'TestLoadALFromEDIPlusEBP|TestFD2LoadsCurrentINIInputByte' -count=1` 通過；
  固定雜湊 FD2 測試未略過。
- CPU 測試覆蓋一般相加、32 位回繞、descriptor 越界與錯誤 SIB 拒絕；成功路徑
  保留 EAX 高 24 位、EDI、EBP、來源與旗標。
- 固定 FD2 自然執行 13,272 步後抵達下一個失敗即關閉位置 `0x3F2C1`；錯誤
  回報時 EIP 為 `0x3F2C4`，opcode `8B`、ModRM `04`、SIB `24`。該指令不屬於
  本規格，未被順帶放寬。
