# 255 — MOO2 保護模式滑鼠回呼

狀態：**READY**
日期：2026-10-01
範圍：MOO2 明示平台的 `INT 33h/AX=000Ch` 註冊與受控事件派送；不改 remake 玩法。

## 已取得證據

- **已證實，自生拒絕點**：dosgolem `438d6cc5971c3e212e0ce949e1ddd61de307f794`，Go 1.24.13，固定官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`，正版 ZIP 根層 417 檔、`MOX.SET` SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`。高位 LE／低位 DOS arena 自 LE entry 第 1,546,160 步在 **dosgolem 高位 LE 線性** `0x24C31B` 的 `CD 33` 拒絕 AX=`000Ch`、CX=`1`、ES:EDX=`0008:002136D1`。完整歷史收據見 [254-moo2-protected-mouse-sensitivity-settings.md](254-moo2-protected-mouse-sensitivity-settings.md)。
- **已證實，原版註冊樣本**：DOSBox-X 2026.07.02 SDL2 heavy debugger，映像 `fd2-dosbox-x:debug-0d7b272b` ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`。版控 `apps/moo2/tools/startup_probe_131.py --mouse-callback`。**DOSBox-X CS:EIP** `0180:0038031B` 的 `CD 33 C3`：AX=`000Ch`、BX=`0`、CX=`1`、ES:EDX=`0180:003476D1`、DS=`0`、SS:ESP=`0188:003EBB4C`、EFLAGS=`0016h`。同次下一指令 `0180:0038031D` 所擷取暫存器、段、堆疊及旗標保持。私有 JSON SHA-256 `c8d27769a08724015e94ecec234c2f2f1e1238f92a7f8a862f6e397f91f07002`，caller LOG SHA-256 `0bda77ed367c01f167f4895e3180ef0f3ddf37025656ff753fefa40be3413f15`。入口 64 bytes SHA-256 `fa629f9ee324b8ad4629864b88e9f791f1f8955584d534a6c52d2838d0d6c381`。
- **已證實，一般輸入觸發**：同一工具與固定輸入的 `--mouse-callback-event`，在 caller 重新啟用中斷後，由唯一 DOSBox-X 視窗的一般 X11 `xdotool mousemove_relative -- 8 6` 命中 **DOSBox-X CS:EIP** `0180:003476D1`；沒有改寫程式、遊戲狀態或回呼參數。入口 AX=`1`、BX=`0`、CX=`0291h`、DX=`00BDh`、SI／DI=`0`、DS=`00A8h`、ES=`3F78h`、SS:ESP=`3F68:000003B0`、EFLAGS=`2`。原版 `0180:0034772D..00347741` 將 CX 算術右移一位後寫 `003D1A38h`、DX 寫 `003D1A36h`、BX 寫 `003CF21Ah`；只證此樣本的可見狀態消費，不建立玩法規格。
- **公開平台契約**：[DOSBox-X 滑鼠來源](https://dosbox-x.com/doxygen/html/mouse_8cpp_source.html) 的 `INT33_Handler case 0Ch` 保存遮罩與遠指標；事件 AX 為類型、BX 為按鍵、CX／DX 為位置、SI／DI 為位移量。重設清除回呼。既有 [013-mouse-event-callback.md](013-mouse-event-callback.md) 是實模式模型；不能直接推定 DOS/4G 的 32 位元返回框架。
- **已證實，32 位元遠返回**：原版收尾 **DOSBox-X CS:EIP** `0180:003477EC..003477F2` 原始 bytes `89 EC 5D 5F 5E 1F CB`，恢復 ESP／EBP／EDI／ESI／DS，再執行 `RETF`。SS:ESP=`3F68:000003B0` 的前八 bytes 為 `E3 00 00 00 98 00 00 00`；下一步 `0098:000000E3`、ESP=`000003B8`，足以確認 32 位元 offset 與四位元組 selector 槽。橋接層再還原原狀態並 IRET；不追橋接層內部。最終事件 JSON SHA-256 `63ca35948207d9121cdb4726c01622b8ec3c2b1e0a7e1d2d72e5e7032b9764cb`、入口 LOG SHA-256 `e9b59d0464c17c70ae0c9194403ec6f9f30f526d2e99ca49a6521122df0258b2`、返回 LOG SHA-256 `16e96e7f9bee6f281ea8bb94fcc3b2eca786e4dca9496f680099d36c4436df40`、終端 SHA-256 `425ae1ea0cf363602fe14025615f19a8a73400b9ec3f066b65d6e07702a4e296`。

## 擬議受控契約

只在 `MOO2StartupDOS.AttachMachine` 安裝派送器後接受 `000Ch`。低 CX 遮罩只支援公開契約的 `007Fh` 七個事件位；非零遮罩要求已登錄、base=0、可讀入口的 ES:EDX。保存完整 32 位元 offset，不截成 DX。低 CX=0 解除註冊並清除待派送事件，不要求解除時的指標有效；其他暫存器、段及旗標保持。兩種重設也清除註冊與待派送事件。

既有 `SetMouseState` 保持受控查詢用途。另提供明示事件 API，依受控絕對位置、按鍵轉換與明示 mickey 位移判定事件類型；不推算實體速度。先套既有座標範圍，再將匹配遮罩的事件與註冊目標一起排入 FIFO。容量上限 4096，滿時回錯且不改狀態；不靜默丟棄。未匹配的正常輸入仍更新位置／按鍵。

CPU 指令邊界且 IF=1 才派送；回呼不重入。保留通用 CPU 原解碼與既有 StepHook／BIOS 時鐘，只在派送及精確返回框架處理一次步進。入口 AX=事件、BX=按鍵、CX／DX=受控位置、SI／DI=有號 mickey 的低 16 位；不使用亂數。私有有界堆疊由 DPMI 線性配置及獨立可寫 descriptor 提供，避免覆蓋被中斷的遊戲堆疊。原版可能自行換堆疊，返回時必須回到自己框架。錯誤框架、未知指令或越界失敗即關閉。

返回框架為兩個四位元組槽：私有返回標記 `FFFFFFF0h` 與註冊的 CS；派送時 IF／DF 清除。只在活動回呼的 `CB`、私有 SS、原框架 ESP 及兩個槽都吻合時截取返回，不放行其他 CPU 遠返回。完成後恢復被中斷的架構狀態（一般暫存器、段、EIP、旗標與 x87 狀態），保留回呼對遊戲記憶體的正常修改及平台服務效果。這是 **platform-spec approximation**；不還原原版 extender 的臨時 selector、實體 IRQ、指令時間或驅動內部。

## READY 審查與驗收

註冊、一般輸入命中、座標消費及八位元組遠返回已形成最小證據鏈；遮罩與重設依公開平台契約。據此 DRAFT 轉 READY。合成測試須覆蓋完整 EDX、解除／重設、遮罩、FIFO、非重入、IF 等待、容量拒絕、範圍、暫存器／段／旗標／x87 恢復、記憶體效果、既有時鐘 hook、錯誤框架及一般 FD2 拒絕。固定原檔全套回歸與完整資料自然越過註冊後，分開記錄註冊完成、合成派送完成及原版完整回呼尚未完成的邊界；不能因自然註冊成功把實际事件全路徑寫成 CONFORMED。

所有原版記憶體、完整終端與 LOG 留私有 `workplace/`；本規格只是輔助基準，正式驗收須由 dosgolem 自行重生。

## 實作與有限收據（維持 READY）

`internal/machine/le_mouse_callback.go` 已接 `000Ch`、FIFO、IF 閘門、非重入、受限私有堆疊及精確遠返回框架。合成 CPU 真正執行自製回呼，寫出六個事件參數，測試架構狀態恢復、記憶體效果、依序派送、按鍵轉換、範圍、容量拒絕、無效註冊、重設／解除、舊 StepHook／BIOS 時鐘與錯誤返回拒絕均通過。固定 EXE 全套回歸以規格 257 的最後重跑為準，私有 `workplace/full-test-257.txt` SHA-256 `33ca5589160c47f992c5cdeec08b6b65ded02dfc6a03fb705aa698c21a05f26e`。

第一個原檔受控事件進入回呼後，因 CS 段載入形狀未支援而停止；規格 [256-cpu386-cs-absolute-ds-load.md](256-cpu386-cs-absolute-ds-load.md) 補該窄 CPU 形狀。原檔從 LE entry 註冊後首次 IF=1 排入事件，再由本規格派送並返回，started=1／completed=1；私有 `workplace/moo2-probe-256-mouse-event.txt.gz` SHA-256 `2cc16391476f390116582129013b28f784cb0a4c48ced8a530399e454fc98510`。

規格 [257-moo2-protected-mouse-position-setting.md](257-moo2-protected-mouse-position-setting.md) 接線後，另於原檔自行設定座標後首次 IF=1 排入 `657/189/buttons=0/mickey=0/0`，第 1,546,522 步送出，亦由原檔回呼遠返回；最後第 6,217,034 步在 **dosgolem 高位 LE 線性** `0x217888` 的 `INT 2Fh/AX=160Ah` 拒絕，started=1／completed=1。私有 `workplace/moo2-probe-257-mouse-event.txt.gz` SHA-256 `5cc4ec566b82857814f12e271e0dedd88f8f283835e9505cac0f7c6dc88324ba`。這是原檔早期回呼返回收據，沒有核對與 DOSBox-X 相同初態下的完整座標／游標消費支線；不將其升格為正常玩家輸入或完整回呼同狀態 CONFORMED。無事件路徑的 started／completed 皆 0，仍正常越過註冊。
