# 211 — 保護模式 AND r/m32,r32 暫存器形狀

狀態：**CONFORMED**（僅通用 CPU 指令形狀）
日期：2026-10-01

## 證據與邊界

固定 MOO2 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`，dosgolem 已綁定 DPMI 的合成環境診斷在第 762 步、**重定位 LE 線性位址** `0x1515CD` 以原始 bytes `21 C8` 失敗即關閉。DOSBox-X 2026.07.02 SDL2 heavy debugger、映像 `fd2-dosbox-x:debug-0d7b272b` ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，在其 **CS:EIP** `0180:003755CD → 0180:003755CF` 以同一次有界 `LOG 2` 記錄：進入 `EAX=00000080h`、`ECX=FFFFFFFFh`、CF=1／ZF=0／SF=1／PF=1，離開時 `EAX=00000080h`、`ECX=FFFFFFFFh`、CF=0／ZF=0／SF=0／PF=0。私有 `and-logcpu.txt` SHA-256 `1bdb7c5df2801e62c4bd7aee1a7b7bb533b71fe8c1aa19fb92afa9263536755d`，由版控 `apps/moo2/tools/startup_probe_131.py --and` 重生。

先前兩個獨立 `BP` 加 `EV` 快照顯示 `ECX=0Fh`、前後 `EAX=80h`，與 AND 預期矛盾；有界 `LOG 2` 在**同一次指令序列**內記錄 `ECX=FFFFFFFFh`，因此撤回以 `EV` 暫存器文字作此點的規則證據，只保留其地址定位。這段程式在原版有界指令記錄中重複進入，分開的前後斷點可能跨不同迴圈次。`LOG 2` 的連續紀錄顯示此原版樣本的 `80h & FFFFFFFFh = 80h`。這只證明此樣本，不聲明整段合成環境與原版同狀態。

修正版私有 `and-registers.json` SHA-256 `c5131b8aea15512c7c1deeb42964c1261e4ea8c182a07dd125a818a50c0d7796`，明分 EV 位址欄位與權威 LOG 前後行；舊 JSON SHA-256 `955d273f733e19d92994fead6adfd3acf3eb65ff6b79a1817ad0bfac9c06d018` 只保留作勘誤追溯，不可當值對拍。

[Intel® 64 and IA-32 架構軟體開發手冊第 2A 卷](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2a-manual.pdf) 的 `AND` 條目定義 `21 /r` 為 `AND r/m32,r32`：將目的與來源逐位相與、寫回目的，清除 CF／OF，依結果設定 SF／ZF／PF，AF 未定義。`C8` 的 ModRM 為 `mod=11, reg=ECX, r/m=EAX`，因此此處是 `AND EAX,ECX`。證據等級：指令語意已由 Intel 規格與原版同次紀錄證實；玩家路徑與 PSP／環境可比性未知。

## 擬議實作與驗收

只支援無前綴的 `21 /r`、`mod=11`、32 位暫存器目的與來源；目的取 `r/m`，來源取 `reg`。沿用現有 `setLogicFlags` 方法，對 AF 採工具既有清除慣例，不冒稱 Intel 定義其值。段覆寫、repeat、16 位前綴與記憶體形狀仍失敗即關閉。

合成測試核對來源／目的方向、零與非零結果、CF／OF 清除、SF／ZF／PF、EIP 增量與失敗形狀不改目的；固定原檔合成環境診斷越過此指令並記錄下一停點；`go test ./internal/cpu386 ./internal/machine -count=1` 與 `go test ./... -count=1` 通過。正式 dosgolem 玩家對拍仍需另行重生。

## 證據審查

固定輸入雜湊、原始 `21 C8`、兩個位址、同次 `LOG 2` 的原版暫存器與旗標，以及 Intel 的 `21 /r` 編碼相互吻合。`EV` 值誤導已在收據欄位與規格中明確隔離，未用作驗收真值。核准上述無前綴暫存器指令切片為 READY；未核准記憶體形狀、完整 DOS 啟動或玩家玩法對拍。

## 驗收結果

`go test ./internal/cpu386 ./internal/machine -count=1` 與固定原檔輸入的 `go test ./... -count=1` 通過。測試驗證原版 `80h & FFFFFFFFh = 80h`、不同值歸零與 SF/PF 情形、目的方向、未支援的 16 位與記憶體形式拒絕。已綁定 DPMI 的固定原檔合成環境診斷越過第 762 步，至第 803 步在 dosgolem 重定位 LE 線性 `0x151648` 的原始 bytes `83 0E 01` 失敗即關閉。這是新的通用 CPU 工具停點，不代表玩家畫面或玩法同狀態對拍。
