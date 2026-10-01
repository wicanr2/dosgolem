# 251 — MOO2 保護模式滑鼠驅動重設

狀態：**CONFORMED（受限 MOO2 平台服務）**
日期：2026-10-01
範圍：MOO2 啟動的 `INT 33h/AX=0000h` 平台服務；不改 Go remake 玩法或輸入。

## 證據

- **已證實，dosgolem 自生停點**：隔離 dosgolem `fb541129530206f16fda6edc7630b307e0d21ceb`，Go 1.24.13／`golang:1.24-bookworm`，官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、正版 ZIP SHA-256 `3a28a52f5953ff6d8fc251548940500236752ee19b52b51581e71ec1a3373c2f` 根層 417 檔及 `MOX.SET` SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`。明示高位 LE／低位 DOS arena、合成 PSP／環境，第 1,545,396 步在 **dosgolem 高位 LE 線性位址** `0x24C31B` 的 `CD 33` 拒絕；EAX／EBX／ECX／EDX=`0`、EFLAGS=`0012h`。私有 `workplace/moo2-probe-250-full-game.txt.gz` SHA-256 `74b73985b73a1f077f2005343fef0168df1d05dbe49d03d8f824369f5105a1b8`。
- **公開平台契約**：[DOSBox-X 滑鼠來源](https://dosbox-x.com/doxygen/html/mouse_8cpp_source.html) 的 `INT33_Handler` 將功能 `00h` 經硬體重設導向軟體重設，回 AX=`FFFFh`、BX=按鍵數。`Mouse_Reset` 清按鍵與移動／歷史狀態、清回呼，座標設為目前範圍中心；`Mouse_AfterNewVideoMode(false)` 按當前模式重設範圍。不是原版 MOO2 對所有實機驅動的逐值契約。已有軟體重設見 [229-moo2-protected-mouse-software-reset.md](229-moo2-protected-mouse-software-reset.md)，VBE 模式設定見 [239-moo2-vbe-set-mode-0101.md](239-moo2-vbe-set-mode-0101.md)。
- **已證實，原版同次返回及消費端**：DOSBox-X 2026.07.02 SDL2 heavy debugger，映像 `fd2-dosbox-x:debug-0d7b272b` ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`。版控 `apps/moo2/tools/startup_probe_131.py --mouse-reset` 在 **DOSBox-X CS:EIP** `0180:0038031B` 直接取得 `CD 33 C3`，EAX／EBX／ECX／EDX／ESI／EDI=`0`、DS／ES／SS=`0188h`、ESP=`003EBB58h`、EFLAGS=`0016h`。下一指令 `0180:0038031D` 的同次返回 AX=`FFFFh`、BX=`3`，其餘擷取欄位及旗標不變。64 行連續紀錄確認 `0180:003801B8..003801C0` 把 EAX／EBX／ECX／EDX 寫入 record `003D18E0h +0/+4/+8/+0Ch`；`0180:00347527` 比較該 AX 輸出是否為零，成功後於 `0180:003473F4` 準備功能 `1Bh`。私有 JSON SHA-256 `2be392ca0015dbf77cf73de2fb3881daee97d0c71428deb5d0b519acab859b18`、caller LOG SHA-256 `d6030bb492e573e64ec239faeb92dff7701b273e19dba12b5fc46c9cb8116327`、終端 SHA-256 `8cea3130bbf2dfdd8e7bf6a271c91411e5c3f65ffbaa46fa946f29a5a2a00085`。兩側旗標、PSP／環境、堆疊與完整狀態不同，僅比對該平台功能的可比效果。

## 擬議契約

只在 MOO2 設定支援功能 `0000h`；回低 16 位 AX=`FFFFh`、BX=`0003h`，其餘暫存器高位、段與旗標保持。清受控按鍵；未設定 VBE 時回既有初始中心 `320,100`，已設定模式 `0101h` 時回 `320,240`，由平台來源與 640×480 模式導出，標為 **platform-spec approximation**。既有 `21h` 重設亦使用目前模式中心，避免兩功能狀態不對稱；敏感度沿既有狀態，不猜補驅動內部。未建模硬體 IRQ、回呼、歷史與相對位移保持未知，後續若消費則另開窄規格；其他功能及一般 FD2 設定維持拒絕。

## 驗收

原版同次入口／返回／record 消費端、重設後受控 `AX=3` 狀態、兩種模式中心、`00h／21h` 共用狀態、高位／旗標保持、一般 FD2 拒絕均須核對；固定原檔全套測試及 LE entry 自然越過停點。原版完整 PSP／環境與 dosgolem 不同，無正常玩家畫面或玩法同狀態宣稱。原檔、記憶體、完整終端留私有工作區。

## READY 審查

原版同次位元組、返回與 record 消費端證實 AX／BX 與保留欄位；平台來源足以界定受控按鍵清除及模式中心。高位樣本為零，非零高位只依既有保留策略與合成測試驗收；模式中心未有原版重設後位置查詢，只標近似，不冒稱逐值原版。據此 DRAFT 轉 READY；後續功能 `1Bh` 另核對，不在本切片猜補。

## CONFORMED 收據

功能 `00h／21h` 在未設 VBE／已設 `0101h` 的中心與後續查詢、高位／旗標保留、敏感度不改、一般 FD2 拒絕均通過。固定 EXE 全套 `go test -buildvcs=false ./... -count=1` 輸出 `workplace/full-test-251.txt` SHA-256 `f7ac2730974ecee63a4b2e4146f9d5850e7aa829d903a14525c62c3c731b16b5`。相同正版資料從 LE entry 自然越過 `AX=0000h`，第 1,545,515 步在 **dosgolem 高位 LE 線性** `0x24C31B` 的下一筆 `INT 33h/AX=001Bh` 拒絕，EBX／ECX／EDX=`0`、EFLAGS=`0016h`；DPMI 未實作清單為空。私有 `workplace/moo2-probe-251-full-game.txt.gz` SHA-256 `e8d6b4527a940ea1aeb0c4391e7931cd41ed6f8e6e763b951d21bacfa0961c43`。驗收限平台重設，後續敏感度查詢與正常玩家畫面另驗。
