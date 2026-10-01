# 248 — 32 位元暫存器 OR 符號擴展立即數

狀態：**CONFORMED（無前綴 32 位元暫存器形狀）**
日期：2026-10-01
範圍：通用 `internal/cpu386` 無前綴 `83 /1 ib mod=11`；只擴充 CPU 解碼，不改 MOO2 Go remake 玩法、介面或存檔。

## 證據、輸入與位址基準

- **已證實，dosgolem 自生停點**：官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、正版 ZIP SHA-256 `3a28a52f5953ff6d8fc251548940500236752ee19b52b51581e71ec1a3373c2f`、`MOX.SET` SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`。隔離 dosgolem `8f6bf14c0ac65c53f72d77d1439945151a2916c0`，Go 1.24.13／`golang:1.24-bookworm`，明示高位 LE／低位 DOS arena 及合成 PSP／環境，在第 1,170,054 步、**dosgolem 高位 LE 線性位址** `0x25221E` 拒絕 `83 C8 10`，EAX=`0`、EFLAGS=`0246h`。私有 `workplace/moo2-probe-247-full-game.txt.gz` SHA-256 `ad3568f83e7a85f70e682484bf392bac3bcd66dce42fd474e1d061b6af3083ca`。後續 bytes `83 7B 18 01` 不在本規格推論玩法。
- **已證實，公開 CPU 契約**：[Intel IA-32 指令手冊第 2B 卷 OR 條目，4-163–4-164](https://cdrdv2-public.intel.com/868141/253667-089-sdm-vol-2b.pdf)定義 `83 /1 ib` 的 32 位元形式為 `OR r/m32,imm8`，先符號擴展立即數，再回寫目的；CF／OF 清除，SF／ZF／PF 依結果，AF 未定義。`C8h` 是 mod=`11`、group=`/1`、r/m=`EAX`。既有記憶體形式見 `docs/spec/212-cpu386-or-rm32-imm8-memory.md`；16 位元暫存器形式已有獨立處理。
- **已證實，原版同次輔助樣本**：DOSBox-X 2026.07.02 SDL2 heavy debugger 映像 `fd2-dosbox-x:debug-0d7b272b` ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，版控 `apps/moo2/tools/startup_probe_131.py --or-register-imm8`。在 **DOSBox-X CS:EIP** `0180:0038621E` 直接擷取原始 bytes `83 C8 10`；同次 `LOG 2` 的下一行為 `0180:00386221`，EAX=`00000000h → 00000010h`、CF=OF=SF=AF=0、ZF/PF=`1 → 0`、IF=1，其餘通用暫存器與段不變。下一指令為 `CMP dword [EBX+18h],1`，原版此次來源值為 `2`；該結構玩法語意未知。私有 `or-register-imm8-registers.json` SHA-256 `19e379e706dc80f1100c254f68f145b11555da57796525d772b3e8abbfebda36`、`or-register-imm8-logcpu.txt` SHA-256 `dd2e0788f4e33ccbbf32886ab62faecace8468362056eaccad0f368887ffb047`、終端 SHA-256 `4ea3bdd4270bac47809074bdb6e99c06241422fa749b90504f3c06321a6bcb7b`。兩工具指令形狀相符，但 EBX、ESP 等不同，不宣稱整段同狀態。

## 擬議契約

無前綴 32 位元 `83 /1 mod=11` 讀一個立即數 byte，以 `int8 → int32 → uint32` 符號擴展，與 ModRM r/m 指定的完整 32 位元暫存器 OR，僅回寫該暫存器並以既有 `setLogicFlags` 更新旗標。IF 等其他旗標、其餘暫存器、段及記憶體保持；AF 沿用既有清除近似，不稱未定義旗標原版逐情形一致。未知 repeat／段覆寫形狀仍拒絕；16 位元與記憶體的既有受審查路徑維持。截短立即數拒絕且不改暫存器／旗標；EIP 已取出的進度不作交易式回滾。

## 驗收與未知邊界

合成測試須覆蓋原版輔助樣本、dosgolem 停點預測 `0|10h=10h`、八個目的暫存器、高位保留／負立即數符號擴展、zero／sign／parity 與 CF／OF 清除、IF 保存、截短及未授權前綴拒絕。固定官方 EXE 全套 Go 測試通過；同一正版資料由 dosgolem 自 LE entry 越過原停點並記錄下一自然結果。原版 PSP／環境、混音器設定及完整狀態與 dosgolem 尚不可比，只有 CPU 形狀可獨立驗收；尚無玩家畫面或 remake 玩法同狀態對拍。原檔與完整記憶體／終端資料留在私有工作區。

## READY 證據審查

原版同次 raw bytes、LOG 的目的暫存器與旗標效果，以及 Intel 對 `83 /1 ib` 的編碼／符號擴展契約一致。原版只實測 EAX／正立即數 `10h`；八個目的暫存器和負立即數依公開 CPU 契約與合成測試驗收。AF 的未定義狀態與現行清除策略已明示，原版完整狀態差異不作此 CPU 切片阻塞項。據此 DRAFT 轉 READY；正常玩家路徑仍另驗。

## CONFORMED 收據

八個目的暫存器、原版樣本、符號擴展、旗標及拒絕測試通過；固定 EXE 的全套 `go test -buildvcs=false ./... -count=1` 輸出 SHA-256 `fa42fdfa526f73f528f57088473c79af840a19e1a5ca245bd850fe11a2da89d2`。同一正版資料由 LE entry 自然越過舊停點，第 1,170,076 步停在 **dosgolem 高位 LE 線性位址** `0x250722` 的 `81 F2 00 80 00 00`；私有 `workplace/moo2-probe-248-full-game.txt.gz` SHA-256 `5ca332efc5cc54d6ebf0fb8921ac8bad10cc38c577cf0f3576fdd764084c8731`。驗收限本 CPU 形狀，後續 XOR 原版樣本與收據見 [規格 249](249-cpu386-xor-register-imm32.md)，尚無玩家畫面或玩法同狀態對拍。
