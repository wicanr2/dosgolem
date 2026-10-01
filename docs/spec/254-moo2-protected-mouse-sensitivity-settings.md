# 254 — MOO2 保護模式滑鼠非零敏感度設定

狀態：**CONFORMED（受控設定／查詢平台模型）**
日期：2026-10-01
範圍：MOO2 完整資料啟動的 `INT 33h/AX=001Ah` 設定狀態；不改 Go remake 玩法。

## 證據

- **已證實，自生停點**：隔離 dosgolem `792b8d077d1d4611ae8246ddc9d61129e316b67a` 加 [253-moo2-protected-mouse-coordinate-ranges.md](253-moo2-protected-mouse-coordinate-ranges.md) 已驗實作，Go 1.24.13／`golang:1.24-bookworm`。官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、正版 ZIP SHA-256 `3a28a52f5953ff6d8fc251548940500236752ee19b52b51581e71ec1a3373c2f` 根層 417 檔及 `MOX.SET` SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`。高位 LE／低位 DOS arena、合成 PSP／環境，第 1,545,911 步在 **dosgolem 高位 LE 線性** `0x24C31B` 的 `CD 33` 拒絕功能 `001Ah`，私有 `workplace/moo2-probe-253-full-game.txt.gz` SHA-256 `76521df20677fe92c685fe909a1fd13fdd41b25ffc2955051d4c9359d8ddd04d`。
- **已證實，原版準備參數**：前一規格垂直服務的 caller LOG 於 **DOSBox-X CS:EIP** `0180:003475F7..00347619` 寫 record：AX=`001Ah`、BX／CX=`0064h`（100）、DX=`01DFh`（479）。
- **已證實，原版同次樣本**：DOSBox-X 2026.07.02 SDL2 heavy debugger，映像 `fd2-dosbox-x:debug-0d7b272b` ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`；版控探針 `apps/moo2/tools/startup_probe_131.py --mouse-set-sensitivity`。**DOSBox-X CS:EIP** `0180:0038031B` 原始 `CD 33 C3`，AX=`001Ah`、BX／CX=`0064h`、DX=`01DFh`、ESI／EDI=`0`、DS／ES／SS=`0188h`、ESP=`003EBB58h`、EFLAGS=`0016h`。下一指令 `0180:0038031D` 的擷取欄位完全保持；`0180:003801B8..003801C0` 寫 EAX／EBX／ECX／EDX 到 record `003D18E0h +0/+4/+8/+0Ch`，後續啟動 caller 回到滑鼠狀態查詢鏈。私有 JSON SHA-256 `6fcbabf3b96c68eda2bea2032090f5e2a2fa25e098a16999229b8d407eb0d57f`、caller LOG SHA-256 `01d24378d6ffa0b5c7fda1532177ab6f3e623f64994907aa383cce5f96d06268`、終端 SHA-256 `a829f04c341c922bdbc8fc46ed01b83a7aeef983c107c243f8a6c423b8385826`。兩側 PSP／環境、堆疊與完整旗標不同，只驗限定服務效果。
- **公開平台契約**：[DOSBox-X 滑鼠來源](https://dosbox-x.com/doxygen/html/mouse_8cpp_source.html) 的 `INT33_Handler case 1Ah`／`Mouse_SetSensitivity` 將各低 16 位輸入上限裁切為 100，保存三值供 `1Bh` 讀回；零值可保存。移動係數與實體滑鼠不在現行受控絕對座標模型內，維持未知，不能因設定成功宣稱移動速度還原。

## 擬議契約與驗收

只在明示 MOO2 設定接受 `001Ah`，把低 BX／CX／DX 各取 `min(value,100)` 保存；所有暫存器、高位、段、旗標、受控位置、按鍵及座標範圍保持。由既有 `001Bh` 查詢驗證狀態往返，不改注入的絕對座標或永久鎖成測試值。一般 FD2 及其他功能仍拒絕。屬 **platform-spec approximation**，不是所有 DOS 滑鼠驅動或原版實機精確敏感度契約。

原版同次入口／返回／record 消費端，合成 `100/100/479`、零、混合、上限與 `FFFFh`，讀回、重複設定及兩種重設保留、受控座標／按鍵／範圍不改、高位／段／旗標保持，一般 FD2 拒絕，固定 EXE 全套測試與正版資料從 LE entry 自然越過服務。沒有原版非零設定後的敏感度讀回或移動實驗，不將合成讀回冒稱原版動態收據。

## 舊規格邊界與停止線

[230-moo2-protected-mouse-zero-sensitivity.md](230-moo2-protected-mouse-zero-sensitivity.md) 的缺檔零值樣本與雜湊仍有效；本規格擴充設定狀態的輸入域，不推翻零值結果。其「非零拒絕」只描述歷史實作，READY／CONFORMED 後須回填解析入口與替換舊拒絕測試。沒有正常玩家畫面、完整驅動、回呼、IRQ 或移動速度收據；範圍足以讓原版前進便停止，不重做平台驅動考古。原始資料與完整終端留本機私有工作區，Go remake UI／存檔不受影響。

## READY 審查

原版實際輸入、同次欄位保持與 record 消費端已取得；平台來源直接定義裁切與可讀狀態，足以實作受控絕對座標模型的設定／查詢。非零設定後移動係數依舊未知，不用常數或未驗係數猜補。舊拒絕邊界限制的是歷史啟動能力，本規格僅解除可依平台契約驗收的狀態設定。據此 DRAFT 轉 READY，不把平台狀態驗收升格成完整輸入還原。

## CONFORMED 收據與解析回填

實際 `100/100/479`、零、混合、99／100／101／`FFFFh` 的裁切與讀回、重複設定、兩種重設保留敏感度、受控位置／按鍵／範圍保持、高位／段／旗標保持及一般 FD2 拒絕均通過。第一次回歸的舊零值測試在位置查詢後留下 CX=417／DX=122，卻期待下一次設定仍為零；修正為明示輸入 `1/0/0` 後，以同一容器映像及命令乾淨重跑。保留私有 `workplace/full-test-254-first.txt`，不改平台契約迎合錯誤預期。

固定 EXE 全套 `go test -buildvcs=false ./... -count=1` 最終私有 `workplace/full-test-254.txt` SHA-256 `fe9cf5242d01be373fdea1c67d9869165446d18bdf267831524cb6af4cf0fd01`。完整正版資料自 LE entry 自然越過非零設定與後續位置查詢，第 1,546,160 步在 **dosgolem 高位 LE 線性** `0x24C31B` 的 `INT 33h/AX=000Ch` 拒絕；私有 `workplace/moo2-probe-254-full-game.txt.gz` SHA-256 `3f3af2c4aeef4fc15686b2a281ab35d8ac6b7c432c3016205277db6e85f9dfe7`。回呼服務仍待證據，沒有正常玩家畫面或玩法同狀態宣稱。

| 不可變鍵 | 附加語意／等級 | 新證據 | 舊規格／必備標記 |
|---|---|---|---|
| DOS 1.31／EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`／DOSBox-X CS:EIP `0180:0038031B`／`INT 33h/AX=001Ah` | 實際非零參數與同次返回已證實；可讀裁切狀態為平台規格近似 | 本規格 | 規格 230／「非零拒絕邊界已由規格 254 取代」 |

版控 `apps/moo2/tools/startup_probe_131.py --check-mouse-spec-backlinks` 檢查原始鍵、READY／CONFORMED 狀態與舊規格勘誤；缺任何一項即失敗。`--mouse-set-sensitivity` 擷取前也自動執行同一護欄。此解析解除平台啟動阻塞，沒有改 Go remake 玩家規則。
