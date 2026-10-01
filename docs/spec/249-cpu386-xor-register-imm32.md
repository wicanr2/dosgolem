# 249 — 32 位元暫存器 XOR 完整立即數

狀態：**CONFORMED（無前綴 32 位元暫存器形狀）**
日期：2026-10-01
範圍：通用 `internal/cpu386` 無前綴 `81 /6 id mod=11`。只擴充 CPU 解碼，MOO2 音訊波形、遊戲規則、介面與存檔不在本切片。

## 證據與原始輸入

- **已證實，dosgolem 自生停點**：官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、正版 ZIP SHA-256 `3a28a52f5953ff6d8fc251548940500236752ee19b52b51581e71ec1a3373c2f`、`MOX.SET` SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`。隔離 dosgolem `8f6bf14` 加規格 248 已驗實作，Go 1.24.13／`golang:1.24-bookworm`，高位 LE／低位 DOS arena 及合成 PSP／環境，第 1,170,076 步在 **dosgolem 高位 LE 線性位址** `0x250722` 拒絕 `81 F2 00 80 00 00`，EDX=`0`、EFLAGS=`0206h`。私有 `workplace/moo2-probe-248-full-game.txt.gz` SHA-256 `5ca332efc5cc54d6ebf0fb8921ac8bad10cc38c577cf0f3576fdd764084c8731`。
- **已證實，公開 CPU 契約**：[Intel《80386 Programmer's Reference Manual》XOR 條目，第 411 頁](https://read.seas.harvard.edu/~kohler/class/aosref/i386.pdf)定義 `81 /6 id` 為 `XOR r/m32,imm32`，回寫完整 32 位元目的，CF／OF 清除，SF／ZF／PF 依結果，AF 未定義。`F2h` 的 ModRM 為 mod=`11`、group=`/6`、目的=`EDX`，little-endian 立即數為 `00008000h`。
- **已證實，原版同次輔助樣本**：DOSBox-X 2026.07.02 SDL2 heavy debugger，映像 `fd2-dosbox-x:debug-0d7b272b` ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`；版控 `apps/moo2/tools/startup_probe_131.py --xor-register-imm32` 在 **DOSBox-X CS:EIP** `0180:00384722` 直接讀到 `81 F2 00 80 00 00`。同次 LOG 的下一指令 `0180:00384728`，EDX=`0 → 00008000h`，其餘通用暫存器與段不變。停在該下一指令後，以 `EV CS EIP EDX EFLAGS` 讀得 `0180 00384728 00008000 0206`；再啟動連續 LOG 的第一行仍在該位址，PF=1，並確認後續 `MOV DL,AH` 不改旗標。後續資料格式與音效用途未知。
- **已證實，除錯顯示差異**：首次與重跑的 LOG 2 皆將 XOR 後 PF 印成 0；同次實際 EFLAGS 和續取 LOG 則為 PF=1。初次 JSON SHA-256 `4e49b5b650d4923192389da79fea9a0c51a7a840bf19badb29aa9f491ec82335` 另存私有 `xor-register-imm32-initial-registers.json`。重跑 JSON `xor-register-imm32-registers.json` SHA-256 `368364c1b18d62c58ba6a381858b44fbd69d56edd32cfefb6c881e91dd10a68e`、首段 LOG SHA-256 `bec46426929c6ff31d4ec525563dbf7b972b779fe1af4664a82b3ea949bf575c`、續段 LOG SHA-256 `cc608d6504ee24b1b001a6980c34b5b6e301135c76343f712642b9fbc7fb693e`、重跑終端 SHA-256 `865232034711bc2f978812c70dd5972a15299b82d44275162517c117fd1b9ba3`。不以首段 LOG 的 PF 當 CPU 契約。
- **強推論，差異來源**：[DOSBox-X 公開 flags.cpp](https://raw.githubusercontent.com/joncampbell123/dosbox-x/master/src/cpu/flags.cpp) 的 `get_PF()` 對 `t_XORd` 使用整個 dword 的 `PARITY32`，而 `FillFlags()` 使用低 byte 的 `DOFLAG_PF`；`8000h` 因而分別得到 0／1。此來源讀於 2026-10-01，尚未將映像二進位與該來源逐值比對，故只作顯示差異解釋，不升格為映像內部已證實。原版程式呼叫／CPU ISA 的證據已足，不深挖除錯器內部。

## 擬議契約

只接受無前綴、32 位元、`81 /6 mod=11`；以既有 `fetch32` 讀完整 little-endian 立即數，與 ModRM r/m 指定的暫存器 XOR，僅回寫該暫存器並呼叫 `setLogicFlags`。其餘暫存器、段、記憶體與 IF 等旗標保持；AF 沿用清除策略，不宣稱未定義旗標原版逐值一致。16 位元、記憶體目的、repeat／段覆寫等未審查形狀維持拒絕。截短立即數拒絕且不改暫存器／旗標；EIP 取指進度不回滾。

## 驗收

合成測試覆蓋原版同次樣本、八個目的暫存器、完整立即數高位、zero／sign／parity、CF／OF 清除、IF 保持、截短與未授權形狀拒絕。固定 EXE 全套 Go 測試通過；正版資料由 dosgolem 自 LE entry 自然越過停點並記錄後續。原版完整狀態仍不可與合成初態等同，無玩家畫面或 remake 玩法同狀態宣稱；原檔、完整記憶體及終端留私有工作區。

## READY 證據審查

原版 raw bytes、目的暫存器結果、實際 EFLAGS 與續取 LOG 均符合 Intel CPU 契約。首段 LOG 的旗標顯示差異已可重播並明示，不採錯誤的 PF=0 作測試預期。AF 未定義，沿用既有清除策略；原版只核對 EDX／立即數 `8000h`，八個暫存器及其他立即數依 CPU 契約驗收。據此 DRAFT 轉 READY；完整狀態與正常玩家路徑另驗。

## CONFORMED 收據

原版樣本、八個目的暫存器、完整立即數高位、旗標及未授權形狀／截短拒絕測試均通過。固定 EXE 全套 `go test -buildvcs=false ./... -count=1` 輸出 `workplace/full-test-249.txt` SHA-256 `42bba15ca1049b603dc409a74635cd8a94e0287c543b341de0a2329d7ecc2732`。同一正版 417 檔由 dosgolem LE entry 自然越過舊停點，第三筆 DOS 配置 512 段落亦成功；在探針八百萬步上限停止，**dosgolem 高位 LE 線性位址** `0x222AB5`，未實作 DPMI 清單為空。私有 `workplace/moo2-probe-249-full-game.txt.gz` SHA-256 `a0e0c714e5cf33bcf612becf50e5c7186494b44259ab0c8d789e68f5890a947b`。此收據僅驗收 CPU 形狀與自然越過停點，尾端 BIOS tick 等待與接線收據見 [規格 250](250-moo2-bios-clock-attach.md)，正常玩家畫面仍待驗，不稱完整啟動或玩法同狀態通過。
