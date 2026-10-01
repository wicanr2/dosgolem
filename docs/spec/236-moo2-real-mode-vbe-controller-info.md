# 236 — MOO2 實模式 VBE 控制器資訊呼叫

狀態：**CONFORMED**（僅固定 `4F00h` 平台樣本）
日期：2026-10-01
範圍：隔離 dosgolem `NewMOO2StartupDOS` 設定下，DPMI `INT 31h/AX=0300h` 轉呼叫實模式 `INT 10h/AX=4F00h` 的首筆控制器資訊請求。只建模 DOS 視訊 BIOS 公開契約與已觀測的 MOO2 啟動樣本；不改 remake 玩法或宣稱已繪製畫面。

## RE／平台證據

- **已證實，固定原版輔助執行**：官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、正版 ZIP 根層 417 檔與 `MOX.SET` SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`。DOSBox-X 2026.07.02 SDL2 heavy debugger 映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`；版控 `apps/moo2/tools/startup_probe_131.py --real-video-0300` 在 **DOSBox-X CS:EIP** `0180:00380315 → 0180:00380317` 記錄外層 `EAX=0300h、EBX=10h、ECX=0、ES=0188h、EDI=003EBB18h`，外層暫存器與旗標保持。50-byte 封包進入時實模式 AX=`4F00h`、ES=`0FE3h`、DI=0、flags=0；返回 AX=`004Fh`、flags=`0002h`，其餘封包不變。私有 `workplace/dosbox-moo2/real-video-0300-registers.json` SHA-256 `e64bb9d907c18ed54d3939419289b86bf0c4dce3cc30878a91831be98c2fbed5`，終端 SHA-256 `d5e46afe5d9a4f6d24d3c24e0f935290bf9f316e471de9c260bff7aabf9250d3`。
- **已證實，緩衝區與 consumer**：原版實模式 ES:DI=`0FE3:0000`，轉物理低位址 `0xFE30`，由零填的 512-byte 區塊變成前 20 bytes 含 `VESA`、版本 `0200h`、OEM 遠指標 `C000:016C`、能力 `1`、模式清單指標 `C000:0100`、64KiB 顯示記憶體單位數 `32`；其餘 bytes 保持零。私有緩衝前 SHA-256 `076a27c79e5ace2a3d47f9dd2e83e4ff6ea8872b3c2218f66c92b89b55f36560`、後 SHA-256 `35667f00729cc44ab2885d6703bf2527909ebab0fd2c4d43153af129088450d9`。延長的同次 LOG 在 **DOSBox-X CS:EIP** `0180:0036A09F` 比較返回記錄 `004Fh` 並走成功分支；私有 `real-video-0300-caller-logcpu.txt` SHA-256 `1fdf1ff345298aecd936aefa803a2a542595d165f218519d831cd60467543afb`。模式清單 ROM `C000:0100` 前 128 bytes 的第 54 個 word 是 `FFFFh` 結束符；OEM 字串是 DOSBox-X 模擬的 S3 Trio64。私有 mode list SHA-256 `ed7849dbcb11a1b8b6ebc692f932fc7cac00df31706d8708865ba236d1a0c205`、OEM 區 SHA-256 `9718a16232ceeee81c438fa223e9bbe3ea212267bfec571f186dfd33c83bb966`。
- **勘誤，已證實的工具位址錯誤**：第一次 `MEMDUMPBIN 0FE3:0` 在保護模式把實模式段號當 selector，輸出整片 `0x55`，兩側 SHA-256 同為 `f93ac174acd97b23458c571f52c97347dd856ecdb64697e86f71fbe88bdfed19`。舊輸出保留為 `real-video-0300-selector-invalid-*`，不代表原版未寫緩衝。修正為已核對零基底的平坦 selector `0188:0000FE30`，同映像同輸入重跑才取得上述真資料。
- **已證實，平台規格**：[VESA VBE Core Functions 2.0 Rev 1.1，第 12–16 頁](https://www.phatcode.net/res/221/files/vbe20.pdf)定義 `4F00h`、ES:DI 的控制器資訊結構、`VESA` 簽章、`0200h` 版本、遠指標、64KiB 記憶體數與 AX 返回狀態；呼叫前若未放 `VBE2`，不能把 512-byte 延伸欄位視為必填。DOSBox-X 的 OEM、模式清單內容及 2MiB 視訊記憶體是該輔助環境數值，不是原版遊戲規則。
- **已證實，合成停點**：規格 235 的高位 LE／低位 DOS arena 路徑，第 189,580 步首筆 `0100h` 成功，於第 189,582 步 `0300h` 因實模式 `INT 10h` 未註冊而停；`RealModeLast` 封包實模式 AX=`4F00h`，但其 DOS 段 `1000h` 與原版 `0FE3h` 不同。私有 `workplace/moo2-probe-235-real-mode.txt` SHA-256 `5dbf8b22cda56a2f77fd0ff498daa3dd5d1a14f7b194f074fe748013708023e8`。兩側仍非完整同狀態。

## 擬議受限契約

在 MOO2 啟動設定中加入一個受限的實模式 BIOS 服務轉接：只接受 `BL=10h、CX=0、BH=0`，DPMI 封包實模式 AX=`4F00h`，其 ES:DI 必須落於仍活著的 DOS `0100h` 區塊且至少容納 256 bytes。若符合，於同一機器低位記憶體寫入固定的、合成 VBE 2.0 控制器資訊頭；模式清單與 OEM 遠指標須指向機器中可讀、由此設定明示生成的資料，不得留下空指標或未映射地址。返回封包 AX=`004Fh`、flags=`0002h`，其餘暫存器及外層 DPMI 狀態保持；不得聲稱 DOSBox-X 的 53 個模式是所有真機或遊戲資產的固有資料。輸入形狀不符、區塊越界或其他 `INT 10h` 功能維持失敗即關閉，不用 `4F00h` 成功值概括 `4F01h/4F02h`。這是**平台規格近似**。

## 驗收與邊界

1. 合成：相同 DOS 區塊的實模式段與 selector 可讀同一資訊頭；輸出 AX／flags／外層狀態正確；無效段、太小區塊、未知 VBE 功能拒絕且不改寫記憶體。
2. 固定 1.31 EXE／正版資料自 LE entry 自行重生首筆 `0300h`，記錄封包、VBE 頭與下一自然結果；若成功返回後第二筆 `0100h` 可抵達，回到規格 235 完成其原檔驗收。
3. 即使通過上述受限服務，正常玩家畫面、模式資訊／模式設定、視訊記憶體、音訊與 remake 玩法同狀態對拍仍須另驗，不以零填的畫面或 DOSBox-X 收據替代。

## READY 證據審查

已核對 `simulateRealModeInterrupt`：既有路徑先讀 50-byte 封包、驗證目的描述子與 FS／GS，之後才要求實模式向量；可在該邊界掛一個**明示設定**的 BIOS 契約 callback，回傳已驗證的封包和低位 DOS 緩衝，其他呼叫仍落回原有向量／失敗路徑。`NewMOO2StartupDOS` 是唯一設定者，FD2 預設不啟用。`DPMIHost.dosBlocks` 已記住每筆活區塊的 segment、base、paras，可在任何寫入前驗證 ES:DI 目標；高位 LE loader 的 `Mem` 覆蓋低位 DOS、VBE ROM `0xC0000` 與 LE 高位，同一 bus 可供模式清單遠指標讀取。用 DOSBox-X 輔助收據的 53 個數值建立**明示合成顯示配備**的清單和可讀 OEM 字串，標示其硬體配置屬平台近似，避免空指標；先只驗 `4F00h`，若原檔下一步查 `4F01h` 再另立規格。原版正常玩家畫面、視訊 mode info 與其他裝置仍未知，但不影響這一筆服務的受限輸入、輸出、拒絕與測試契約，因此進入實作。

## CONFORMED 驗收與新停點

`internal/machine/moo2_vbe.go` 已在明示 MOO2 設定回應精確 `4F00h`：驗證仍活著且足夠大的 DOS 緩衝，填入受控 `VESA/0200h` 資訊頭、可讀的 ROM 遠指標資料與成功封包。未知功能、無效或太小區塊均拒絕；一般 FD2 的實模式路徑不變。`DOSGOLEM_MOO2_EXE=/tmp/ORION2.EXE go test -buildvcs=false ./... -count=1` 全通過，私有 `workplace/full-test-236.txt` SHA-256 `c34d903a1918a2f3b9c691ca13c8eda0fcbfdc07d5e362d05272b47fbd28e771`。

固定 1.31 EXE／正版資料自高位 LE entry 自然至第 189,582 步，dosgolem 的 `RealModeLast` 顯示輸入實模式 AX=`4F00h`、輸出 AX=`004Fh` 與 flags=`0002h`、`Returned=true`；DOS 段為其合成配置的 `1000h`。接著至第 189,937 步的**dosgolem 高位重定位 LE 線性位址** `0x24C2B2`，停在另一筆保護模式 `INT 10h/AX=4F07h`，`Unimplemented` 為空。私有 `workplace/moo2-probe-236-full-game.txt` SHA-256 `db6c5b63cbcf9297c0e9a8c5c27a74d77e6895dbac435773b955d2e4b162f444`。這只證明受限 VBE 服務自行重生及下一個自然停點；沒有畫面或玩法同狀態收據。規格 235 的第二筆原檔 `0100h` 尚未抵達，仍維持 READY。
