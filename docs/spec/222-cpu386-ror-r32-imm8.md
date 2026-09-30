# 222 — 32 位暫存器 ROR 立即數 8

狀態：**CONFORMED**  
日期：2026-10-01  
用途：固定 MOO2 1.31 原檔的啟動指令缺口。

## 原版證據與邊界

輸入 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`。DOSBox-X 2026.07.02 SDL2 heavy debugger 映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，在 **DOSBox-X CS:EIP** `0180:0036C22D → 0036C230` 的同次 `LOG 2` 顯示 `ror edx,08 → dec ecx`；進出 EDX=`0`、EFLAGS 的可見值均為 `0206h`。原版只實測了零輸入，不能以此證明非零旋轉或 OF 規則。版控 `apps/moo2/tools/startup_probe_131.py --ror-imm8` 可重生私有 `ror-imm8-logcpu.txt` SHA-256 `2b49614290d647c1ed8ead3336ee49a705d5eb2581d2a9fd5213a79b23136bcc`、`ror-imm8-registers.json` SHA-256 `391c4afc435f8aac41c7316003e2208c1114e7a6748a99ee5feab4c19b9dfd41`。

dosgolem 合成 PSP／環境的固定原檔第 5392 步停在**重定位 LE 線性位址** `0x14822D`，bytes `C1 CA 08`；與 DOSBox-X 的 CS:EIP 是不同位址空間。兩側環境未達完整同狀態，不能把此處當成玩家路徑收據。

## 擬議通用 CPU 契約

依 [Intel 軟體開發手冊第 2B 卷的 RCL／RCR／ROL／ROR 指令表](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2b-manual.pdf)，僅新增 `C1 /1`、32 位暫存器目的、立即數固定 `08h`；結果為向右循環 8 位，CF 為結果 bit 31。SF／ZF／AF／PF 不變；count 大於 1 時 OF 未定義，實作保留原值作決定性約定，**不宣稱原版非零輸入的 OF 對拍**。拒絕其他立即數、記憶體 ModRM、operand-size／segment／repeat 前綴與其他群組形狀；既有 `C1` 形狀維持原契約。

## READY 審查與驗收

審查原版連續 LOG、固定 EXE 雜湊、Intel 指令契約與位址基準；只把零輸入樣本列為原版已證實。合成測試至少驗證零與非零輸入、CF 的兩個值、其他旗標保留、非 8 count 及前綴／記憶體形狀拒絕。固定原檔從 LE entry 自然越過第 5392 步並記錄下一停點；完整 `go test -buildvcs=false ./... -count=1` 須通過。

READY 審查結論：同次 LOG 的指令文字、位址與零輸入結果互相吻合；`C1 CA 08` 的 ModRM 為 register-direct `/1`、目的 EDX。非零結果與 CF 採 Intel 手冊定義，OF 的保留僅是未定義旗標的決定性約定，均不升格為原版實測。上述邊界足以實作這個有限執行器切片。

本切片只補原版執行器的有限 CPU 指令，**不修改 MOO2 remake 玩法**，也不代表 dosgolem 已到正常玩家畫面。

## 實作與驗收收據

`internal/cpu386/cpu.go` 僅為無前綴、register-direct `C1 /1 08` 新增此路徑；向右循環 8 位，CF 取結果 bit 31，保留其他旗標。合成測試驗證零輸入、非零輸入與 CF 兩種結果，以及非 8 count、記憶體目的、截短輸入和前綴拒絕。固定原檔 `TestMOO2RORImmediateEightCheckpointWhenProvided` 從 LE entry 在合成 PSP／環境下到第 5392 步、**重定位 LE 線性位址** `0x14822D`，核對 EDX=`0`、EFLAGS=`0206h`，單步後到 `0x148230` 且其餘暫存器與旗標不變。這是 dosgolem 自行重生的有限啟動指令收據，與上方 DOSBox-X 原版輔助 LOG 分別記錄。

同一固定原檔繼續執行到第 5529 步，停於**重定位 LE 線性位址** `0x14701B` 的 `08 E0`；此新停點尚未由原版獨立核對。私有 `workplace/moo2-probe-222.txt` SHA-256 `3801d8c21e2a3f9251b766817de4946c953340aa6e5cac74ec449b43bf123eca`。含固定原檔的 `go test -buildvcs=false ./... -count=1` 全通過，私有 `workplace/full-test-223.txt` SHA-256 `e885b326c78ac7c279c7021f0a56202ead68326f3ead9b6d2d055eb2d4cb25ca`；另以 `-v` 確認新整合測試確實執行並通過。原版 EXE 與私有完整輸出不入 Git。仍無 dosgolem 正常玩家畫面或 MOO2 玩法同狀態對拍。
