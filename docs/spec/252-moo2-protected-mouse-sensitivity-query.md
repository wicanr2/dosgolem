# 252 — MOO2 保護模式滑鼠敏感度查詢

狀態：**CONFORMED（受限平台狀態查詢）**
日期：2026-10-01
範圍：受限 `INT 33h/AX=001Bh` 平台服務，不改 Go remake 玩法或正式滑鼠操作。

## 證據

- **已證實，dosgolem 自生停點**：隔離 `fb541129530206f16fda6edc7630b307e0d21ceb` 加 [251-moo2-protected-mouse-driver-reset.md](251-moo2-protected-mouse-driver-reset.md) 已驗實作，Go 1.24.13／`golang:1.24-bookworm`。官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、正版 ZIP SHA-256 `3a28a52f5953ff6d8fc251548940500236752ee19b52b51581e71ec1a3373c2f` 根層 417 檔及 `MOX.SET` SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`，高位 LE／低位 DOS arena 和合成 PSP／環境。第 1,545,515 步在 **dosgolem 高位 LE 線性** `0x24C31B` 的 `CD 33` 拒絕，EAX=`001Bh`、EBX／ECX／EDX=`0`、EFLAGS=`0016h`；私有輸出 `workplace/moo2-probe-251-full-game.txt.gz` SHA-256 `e8d6b4527a940ea1aeb0c4391e7931cd41ed6f8e6e763b951d21bacfa0961c43`。
- **公開平台契約**：[DOSBox-X 滑鼠來源](https://dosbox-x.com/doxygen/html/mouse_8cpp_source.html) 的 `INT33_Handler case 1Bh` 以 BX／CX／DX 回目前保存的水平／垂直／倍速敏感度。既有 [230-moo2-protected-mouse-zero-sensitivity.md](230-moo2-protected-mouse-zero-sensitivity.md) 已保存受限零設定；原版同次返回與消費端見下方收據，不以預設值代替當前狀態。
- **已證實，原版同次樣本**：DOSBox-X 2026.07.02 SDL2 heavy debugger，映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`；版控 `apps/moo2/tools/startup_probe_131.py --mouse-sensitivity` 在 **DOSBox-X CS:EIP** `0180:0038031B` 直接取得 `CD 33 C3`，輸入 AX=`001Bh`、BX／CX／DX=`0`、EFLAGS=`0012h`。同次下一指令 `0180:0038031D` 回 BX／CX／DX=`0032h`（50），AX、其餘擷取暫存器／段／堆疊及旗標不變。後續 `0180:003801B8..003801C0` 寫入 record `003D18E0h +0/+4/+8/+0Ch`；caller 接著重用該 record 準備 `AX=3` 位置查詢。私有 JSON SHA-256 `3becc83160dd11465db3a11e647f31703c8a1b4750639ee601d6edffd9b14e05`、caller LOG SHA-256 `9122014bd929cb6ed0caa21ad8d174d150d847c987a45e1a2212f255dda7b049`、終端 SHA-256 `4b8d8343ce60dc87743da49c610f201de76b05fe3a2da7acf5d430819a2cdbd1`。
- **已證實，正式完整資料序列**：版控 `--mouse-sequence` 從固定 DPMI 零基底分支逐筆擷取，前兩筆為 `0000h → 001Bh`，第二筆回 `50/50/50`；私有 JSON SHA-256 `a39e0dbb15d77c67a531c0f652c40234a45c8f4e7af1f843f9a60a7ef529c6a5`、caller LOG SHA-256 `9122014bd929cb6ed0caa21ad8d174d150d847c987a45e1a2212f255dda7b049`、終端 SHA-256 `530142ef79e2de1b0b9c34c44f8d3b807d02d9eb124c2056e82dae5026145755`。dosgolem 最新完整資料探針同樣先執行 `0000h → 001Bh` 並回三個 50，沒有先行零設定。PSP／環境、旗標等完整狀態仍不同，僅平台服務效果可比。

## 擬議契約與驗收

只在 MOO2 設定接受 `001Bh`；從既有狀態回低 16 位 BX／CX／DX，其高位、AX、其餘通用暫存器、段、旗標及敏感度狀態保持。涵蓋初始 `50/50/50` 與自然先行 `1Ah` 零設定後的查詢，兩種重設不改敏感度；不是固定回常數。一般 FD2 與其他功能維持拒絕。移動換算係數、驅動完整狀態與實體裝置仍未建模，僅平台設定查詢，不稱滑鼠正常玩家路徑完成。

原版同次入口／返回／record 消費端須有收據；合成測試核對狀態往返與欄位保留、固定 EXE 全套測試、正版資料從 LE entry 自然越過該服務並記錄後續。原版完整狀態與 dosgolem 不可比，無玩法同狀態宣稱；原始資料與終端留私有工作區。

## READY 審查

原版返回 50 的樣本與公開平台「讀目前狀態」契約一致；既有 `1Ah` setter 與重設保留欄位足以形成可測的狀態往返。原版只驗初始 50，零狀態讀取依平台契約及合成測試驗收；整段狀態不冒稱已對齊。據此 DRAFT 轉 READY，CONFORMED 僅適用該查詢功能，不代表完整滑鼠玩家路徑驗收。

## 勘誤與 CONFORMED 收據

DRAFT／READY 初稿曾把舊缺檔探針的 `3 → 21h → 1Ah` 零設定誤套到現行完整資料，並推定目前為零；這是未查現行狀態的錯誤。最新版版控探針逐筆印出滑鼠服務輸入／返回，證明完整資料先走 `0 → 1Bh`，實際返回為 50；上方原版序列同樣如此。舊缺檔證據仍見規格 229／230，本次以完整資料收據否定「目前零值」斷言，不改 setter／重設語意，也不固定回常數掩蓋差異。

初始 50、零設定往返、兩種重設保留敏感度、重複讀不改狀態、高位／段／旗標保持、一般 FD2 拒絕通過；固定 EXE 全套 `go test -buildvcs=false ./... -count=1` 私有 `workplace/full-test-252.txt` SHA-256 `37ba0e0c1c59433302b0079af252c6b479945ff760b49f9a9d7e466ebf89cdb0`。正版資料從 LE entry 自然越過查詢，第 1,545,719 步在 **dosgolem 高位 LE 線性** `0x24C31B` 的 `INT 33h/AX=0007h` 拒絕，CX=`0`、DX=`04FEh`（1278）、EFLAGS=`0012h`。私有 `workplace/moo2-probe-252-full-game.txt.gz` SHA-256 `d630b6db5fc796dc91164b3e15219083ada2f8aed8bf0a7e263eb6b64708e666`。下一範圍設定及完整玩家畫面待驗，無玩法同狀態宣稱。
