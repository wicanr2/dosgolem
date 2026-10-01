# 240 — 以立即數加到 AL

狀態：**CONFORMED（無前綴 04 ib，非玩法對拍）**
日期：2026-10-01
範圍：通用 `internal/cpu386` 的無前綴 `04 ib`（`ADD AL,imm8`）；不改遊戲規則、畫面或存檔。

## 問題與 RE 證據

- **已證實，執行器停點**：固定官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、正版 ZIP SHA-256 `3a28a52f5953ff6d8fc251548940500236752ee19b52b51581e71ec1a3373c2f` 根層 417 檔及 `MOX.SET` SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`。明示高位 LE 載入的 dosgolem 於第 1,151,730 步、**高位重定位 LE 線性位址** `0x25025F` 停在 bytes `04 20 31 DB 88 E3 83 FB 41 7C 08 83 FB 5A 7F 03`；進入 EAX=`002B4444h`、EFLAGS=`0293h`。私有 `workplace/moo2-probe-239-full-game.txt` SHA-256 `96af49b38b75df1cfd62c08ea9d30ba2f57624841bd9db6851e0b78f13507da6`。這是合成 PSP／環境路徑，非原版同狀態。
- **已證實，原版同指令形狀的輔助樣本**：DOSBox-X 2026.07.02 SDL2 heavy debugger，映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，版控 `apps/moo2/tools/startup_probe_131.py --add-al-imm8` 命中 **DOSBox-X CS:EIP** `0180:0038425F → 0180:00384261`，同次 LOG 顯示 `add al,20`。原版輸入 EAX=`003E4150h`、EFLAGS=`0297h`；執行後 EAX=`003E4170h`、EFLAGS=`0202h`，其他擷取暫存器與段不變。私有 `add-al-imm8-registers.json` SHA-256 `e116d1761ae34b83b18f970059290fa06d64264de6c65bd4f027e9e9cb75a557`、同次 `add-al-imm8-logcpu.txt` SHA-256 `1a13f3c836b94373807964886df093d2034e1d9af8f330850d0a9f4bec71b652`、終端 SHA-256 `833baa62abfd4be1c8a8d29a68fb2bdc75fc1eac5cf37dd0885e280249a66f19`。原版與合成狀態不同，不能逐值合併成一筆收據。
- **已證實，平台契約**：[Intel IA-32 指令手冊，ADD 條目](https://cdrdv2-public.intel.com/843829/325383-sdm-vol-2abcd-dec-24.pdf)定義 `04 ib` 為 `ADD AL,imm8`，只回寫 AL 並更新 CF、PF、AF、ZF、SF、OF。這是標準 CPU 語意，不再要求遊戲逐條反組譯。既有 `add8` 已在其他 byte ADD 形狀使用，但尚未解碼 `04 ib`。

## 擬議契約與 READY 審查

無前綴 `04 ib` 讀下一 byte 作立即數，計算 `AL + imm8` 的 8 位結果，只替換 EAX 低 8 位；高 24 位、其他暫存器、記憶體及段保持。沿用 `add8` 更新六個算術旗標並保留 IF 等其餘旗標。截短立即數及尚未審查的前綴形狀失敗即關閉，且不改運算結果或旗標。

原版 `50h+20h→70h` 驗證本形狀與旗標；合成停點預期 `44h+20h→64h` 是依 Intel 契約的預測，需由 dosgolem 自生重跑確認。測試須另覆蓋高位保留、8 位 carry、signed overflow、half carry、parity、截短與前綴拒絕。此切片可標 READY；原版輸入狀態未與 dosgolem 對齊，也未證明玩家畫面已可到達。

## 驗收

1. `internal/cpu386` 測試核對原版樣本、合成停點、邊界旗標及失敗路徑；現有 CPU／機器測試不退步。
2. 含固定 1.31 EXE 的全套 `go test -buildvcs=false ./... -count=1` 通過。
3. 相同正版資料由 dosgolem 自 LE entry 越過第 1,151,730 步，記錄下一自然結果；只限通用 CPU 指令驗收，不宣稱原版畫面或玩法同狀態。

## 實作與驗收

- `internal/cpu386/cpu.go` 的 `04 ib` 分支沿用既有 `add8`，只回寫 AL；未知前綴與截短輸入拒絕。`internal/cpu386/cpu_test.go` 核對原版 `50h+20h`、合成停點 `44h+20h`、carry、signed overflow、half carry、zero/parity、高 24 位不變及拒絕路徑。
- 含固定 1.31 EXE 的全套 `go test -buildvcs=false ./... -count=1` 通過；私有 `workplace/full-test-240.txt` SHA-256 `a9ceacbb357b48b98070cb4548aa2dde31c17d68a44d87af3b7960b963b49b86`。
- 相同正版資料及明示高位 LE 載入由 dosgolem 自入口越過第 1,151,730 步；自然抵達第二筆 `0100h` 的 176 段落成功配置，於第 1,166,995 步停在 `DPMI 0300h → INT 66h` 底層 `OUT 0226h,01h` 未支援。私有 `workplace/moo2-probe-240-full-game.txt` SHA-256 `0f00b6396e07eece1707f25932eb5450c46b385add27c4a8d5637171ab88d064`。此結果滿足通用 CPU 指令及自然越過停點的驗收；硬體埠與 GUI 尚待獨立處理。
