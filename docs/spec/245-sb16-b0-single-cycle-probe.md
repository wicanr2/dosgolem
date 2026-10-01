# 245 — MOO2 啟動時的 SB16 `B0h` 單次 16 位元 DMA

狀態：**CONFORMED（受限硬體規格近似）**
日期：2026-10-01
範圍：dosgolem 在固定 MOO2 1.31 啟動狀態接受 `B0 30 00 00`，以公開 Sound Blaster 16 契約近似一個 16 位元 word 的單次輸出、完成閘門與 IRQ。無音效播放設備、逐波形或 DAC／PIT／DMA 逐週期驗證。

## 證據與分級

- **已證實，dosgolem 固定原檔的輸入**：官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、正版 ZIP SHA-256 `3a28a52f5953ff6d8fc251548940500236752ee19b52b51581e71ec1a3373c2f`、`MOX.SET` SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`。明示高位 LE 載入、合成 PSP／環境，自 LE entry 到第 1,166,995 步 **dosgolem 高位 LE 線性位址** `0x2454AE`，`DPMI 0300h → INT 66h` 的 **dosgolem 實模式 segment:offset** `1201:05D9` 先在第 457 步拒絕 `OUT 022Ch,B0h`。規格 244 的正式自生收據為私有 `workplace/moo2-probe-244-full-game.txt.gz` SHA-256 `337e94af41018d907487430fc80bab5db6a965ce93972048bc1a19152b33e96b`，工具版本為隔離工作樹 `7281a38fe2c7872fe59827b4ed40551def5d6f48` 加本輪未提交的 243／244 平台實作。
- **已證實，明示可丟棄的參數探針**：未提交的 `DOSGOLEM_MOO2_B0_PROBE=1` 包裝器只攔截這一次 `22Ch` 命令與後三筆參數，最後一筆強制拒絕，仍呼叫既有實模式時鐘。記錄 `B0h,30h,00h,00h`，B0 前第二 DMA 相對通道 1／PC 通道 5 的 mode=`48h`、mask=`0Dh`（該通道解除遮罩）、base address=`92AFh`、count=`0001h`、page=`00h`，第一 DMA mask=`0Fh`。私有 `workplace/moo2-probe-245-b0-state.txt.gz` SHA-256 `151a0edf2db27e3f7df8d5fc386e2c2bd7be751258fd7cf14b106960c9800485`。探針已移除；收據是合成執行器觀測，不是 DOSBox-X 同狀態原版實機。
- **已證實，公開硬體契約**：[Creative《Sound Blaster Series Hardware Programming Guide》第 6 章 Bxh](https://www.ardent-tool.com/sound/Sound_Blaster_HW_Programming_Guide_1st.pdf)定義命令後接 mode、長度低／高 byte；B0h 為 16 位元單次 D/A，`30h` 表示立體聲、有號，長度 `0000h` 表示一個 16 位元 sample。該章也區分 16 位元 DMA 的 IRQ 來源與確認；平台採 **hardware-spec approximation**，不冒稱原版 wall-clock、聲波或人耳聽感。
- **未知**：MOO2 驅動選此命令的上游條件與正式遊戲是否沿用同樣音效模式；不反組譯 DSP driver 或 busy-wait 時序。合成 `VirtualMicros` 與原版實際時間未對齊。

## 擬議契約

DSP 只接受 `B0h` 加精確 `mode=30h`、`wLength=0000h`；其他 Bx 命令、mode 或長度仍失敗即關閉。參數在最後一 byte 才啟動受限傳輸；在此之前不修改 DMA。啟動前驗證第二 DMA 通道 5 已解除遮罩、mode=`48h`、位址／計數兩 byte 均已知、page 已知且為 0、剩餘 word 至少 1、無另一筆 DMA，以及 DSP 有有效正取樣率；來源實際讀取時再核對物理位址落在可讀記憶體範圍，越界立即拒絕並記下原因。任何不符不產生成功回覆或 IRQ。

受限傳輸按 [IBM PC AT 技術參考手冊的 16 位元 DMA 位址契約](https://www.minuszerodegrees.net/manuals/IBM_5170_Technical_Reference_6280070_SEP85.pdf) word 化：`physical=((page&FEh)<<16)|(current_address<<1)`；本次只接受 page `00h`。以既有 `1 µs／指令` 虛擬時鐘和 DSP 取樣率，立體聲模式每 word 累積 `2×rate` 的進度，滿一個 sample 期間後讀一個 little-endian 16 位元 word 到僅供驗證的 PCM 收據、位址 word+1、count word-1，並設 16 位元 DSP IRQ 來源與 PIC 待派送。`wLength+1=1` 是 DSP 的完成門檻；DMA count 可大於此門檻，不因此謊稱 DMA terminal count。16 位元 IRQ 由 `22Fh` 確認，8 位元 IRQ 的 `22Eh` 既有行為保持。PIC 走既有設定的 IRQ7／向量 0Fh，但仍需有效 IVT、未遮罩與 IF，否則待派送不會憑空完成。`reset` 取消未完成傳輸及 16 位元 IRQ 狀態。這只影響 dosgolem 平台層；MOO2 remake 玩法、UI 與存檔不變。

## 驗收

1. 合成測試覆蓋命令四 byte 序列、拒絕未知 mode／長度、DMA 未設定或遮罩拒絕、單一 word 的來源／位址／計數效果、16 位元 IRQ／22Fh 確認、8 位元 IRQ 互不污染、reset 取消與有界 sample duration。
2. 固定官方 EXE 全套 Go 測試通過；正版資料由 dosgolem LE entry 自行越過 `B0h`，記錄下一個自然結果。若原檔暴露本規格未描述的必要行為，回到 DRAFT，不在程式裡默默猜補。
3. 這不是音訊聽感或正常玩家路徑驗收；DOSBox-X 如用於交叉檢查只作輔助基準。

## READY 證據審查

Creative 原廠指南直接定義 `B0h` 四 byte 形狀、mode、長度、DSP 取樣率與 16 位元 IRQ 來源／確認埠；IBM PC AT 手冊直接定義通道 5 的 word 位址與計數。dosgolem 可丟棄探針提供固定原檔的精確四 byte 與 DMA 前態，但沒有原版 wall-clock，故完成時間僅採既有虛擬時鐘的 **hardware-spec approximation**。本切片將可接受輸入收窄為已觀測的 `B0 30 00 00`，其他命令不猜補；固定初態足以審查失敗邊界與驗收。據此由 DRAFT 轉 READY，若原檔後續揭示新的必要效果再回到證據審查。

## CONFORMED 收據與限制

合成測試已覆蓋四 byte 命令、未知輸入拒絕、DMA 字址與計數、獨立 16 位元 IRQ、`22Fh` 確認及 reset；固定 1.31 EXE 全套測試通過，輸出 SHA-256 `2eca390bbd4ef2b35f6436c49c6293c7b7e624ea622eef2aa03ed8de405d4`。正版資料由 LE entry 自然越過 `B0 30 00 00`，下一停點 `1201:0317` 因 BIOS 資料區未初始化錯讀埠 `0006h`；私有自生收據 `workplace/moo2-probe-245-full-game.txt.gz` SHA-256 `ba5aff0b0052eb66e77fb28ca43def2a0f33dd0b7cd8257a95d0a3d0b09a54a4`。後續規格 246 已修正該平台初態，下一停點變成 `IN 0225h`。本 CONFORMED 僅涵蓋受限 B0 命令及平台近似，不代表原版逐週期、波形、聽感或玩家路徑對拍。
