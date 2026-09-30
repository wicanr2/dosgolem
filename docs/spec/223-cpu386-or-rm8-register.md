# 223 — 8 位元暫存器目的的 OR r/m8,r8

狀態：**CONFORMED**  
日期：2026-10-01  
用途：固定 MOO2 1.31 原檔的啟動指令缺口。

## 原版證據與界線

輸入固定 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`。DOSBox-X 2026.07.02 SDL2 heavy debugger 映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，於 **DOSBox-X CS:EIP** `0180:0036B01B → 0036B01D` 的同次 `LOG 2` 顯示 `or al,ah → mov edi,FFFFFFFF`。本次原版 EAX=`00000001h`、EFLAGS=`0202h`，執行後均不變；AH 為零，這個樣本沒有實測非零來源或記憶體目的。版控 `apps/moo2/tools/startup_probe_131.py --or-al-ah` 可重生私有 `or-al-ah-logcpu.txt` SHA-256 `7dc04fd5696dc56a37915d397cd758b0067c75c2b3ff64f5434077ef4a747a8b`、`or-al-ah-registers.json` SHA-256 `a7006a6055ecebffb66c8dc6f989a48b3ecbd5e7191c03a5a8346cd4e407fbfa`。候選 EV 前態只供定位，指令前後態以同次連續 LOG 為準。

dosgolem 使用合成 PSP／環境及已綁定 DPMI，固定原檔第 5529 步停在**重定位 LE 線性位址** `0x14701B` 的 `08 E0`。兩工具位址基準不同；這不是正常玩家路徑或完整同狀態收據。

## 擬議通用 CPU 契約

依 [Intel 軟體開發手冊 OR 指令表](https://cdrdv2-public.intel.com/835752/253667-sdm-vol-2b.pdf)，僅新增無前綴 opcode `08 /r`、ModRM `mod=11` 的 8 位元暫存器目的與來源：`r/m8 ← r/m8 OR r8`。低／高 byte 暫存器均使用既有 `reg8`／`setReg8`，先讀雙方再寫回目的。CF、OF 清零；SF、ZF、PF 隨 8 位結果更新。AF 由 Intel 定義為未定義，沿用既有 `setLogicFlags8` 清零的決定性約定，**不宣稱原版非零輸入的 AF 對拍**。記憶體 ModRM、operand-size／segment／repeat 前綴與截短輸入均失敗即關閉；既有 `0A`、`0C`、`80 /1` 不改。

## READY 審查與驗收

審查同次 LOG、EXE 雜湊、Intel 契約與兩種位址基準；原版證實範圍只限 `08 E0` 且 AH=`0` 的一次執行。合成測試驗證 AL／AH 同底層 EAX 的高低 byte、非零 OR、目的以外暫存器不變、旗標與拒絕形狀。固定原檔須由 LE entry 自然越過第 5529 步並記錄下一停點；完整 `go test -buildvcs=false ./... -count=1` 須通過。

READY 審查結論：`08 E0` 的 ModRM 為 register-direct、來源 AH、目的 AL；同次 LOG 的 EAX／旗標與 `AL=1 OR AH=0` 相容。Intel 指令表與既有 8 位元暫存器、邏輯旗標實作足以限定其他暫存器組合；這些組合只算手冊加合成測試，不升格為原版實測。原版零樣本不支持記憶體形式，故保留失敗即關閉。

此切片只補 dosgolem 的 CPU 執行能力；**不修改 MOO2 remake 玩法**，也不代表正常玩家畫面已可重生。

## 實作與驗收收據

`internal/cpu386/cpu.go` 只為無前綴、register-direct 的 `08 /r` 新增 8 位元 OR；沿用 `reg8`、`setReg8` 與 `setLogicFlags8`。合成測試涵蓋原版 AL／AH 零樣本、同一 EAX 的非零高低 byte、不同暫存器、零結果及前綴／記憶體／截短輸入拒絕。固定原檔 `TestMOO2OrALAHCheckpointWhenProvided` 從 LE entry 在合成 PSP／環境下自然執行到第 5529 步、**重定位 LE 線性位址** `0x14701B`，EAX=`1`、EFLAGS=`0202h`；單步到 `0x14701D`，所列狀態不變。這是 dosgolem 自行重生的有限啟動指令收據，不能與 DOSBox-X 的 CS:EIP 當作同一位址空間。

固定原檔有界診斷前進到第 5806 步，停在**重定位 LE 線性位址** `0x15C1DF` 的 `2E 8D 86 82 C2 15 00`；處理器報錯時 EIP 已取過前綴及 opcode，顯示 `0x15C1E1`。此新停點原版對應尚未獨立核對。私有 `workplace/moo2-probe-223.txt` SHA-256 `7bd4203641b216f68fd8117a1772a24cca26441d1937033127f5b88bcbed16bc`。含固定原檔的 `go test -buildvcs=false ./... -count=1` 全通過，私有 `workplace/full-test-224.txt` SHA-256 `a7e03d6798acdf2df438d0bd94ed99827b2f7c858f3efec2338ae0eec4fe8f61`；新整合測試與 CPU 測試另以 `-v` 確認執行。原版 EXE、完整終端及測試輸出不入 Git；仍沒有 dosgolem 正常玩家畫面或 MOO2 玩法同狀態收據。
