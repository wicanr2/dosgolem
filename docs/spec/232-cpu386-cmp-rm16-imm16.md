# 232 — 16 位元記憶體與立即數比較

狀態：**CONFORMED**
日期：2026-10-01
用途：固定 MOO2 1.31 在明示空 `MOX.SET` 輸入下的啟動比較指令；不解釋該欄位的玩法語意。

## 原版與工具證據

輸入為原版 ZIP 中 `ORION2.EXE`，SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`，另在一次性環境明示提供零位元組 `MOX.SET`，SHA-256 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`、修改時間 1996-01-01 00:00:00 UTC。輔助執行器 DOSBox-X 2026.07.02 SDL2 heavy debugger，映像 `fd2-dosbox-x:debug-0d7b272b` ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`。版控 `apps/moo2/tools/startup_probe_131.py --empty-mox-cmp` 自 LE entry 走過已核對的檔案首次搜尋，在 **DOSBox-X CS:EIP** `0180:002340CF → 0180:002340D8` 擷取同次 `LOG 2`，進入來源 DS:`003AFCBE`=`0000h`、立即數 `0082h`、EFLAGS=`0202h`；離開時六個算術旗標 CF=1、ZF=0、SF=1、OF=0、AF=1、PF=1，通用暫存器與段不變。私有 `empty-mox-cmp-logcpu.txt` SHA-256 `60a5d3e3e3ff7e138dd762698e93280d4f6af81f4acf764f573adb6d2daef1c6`，暫存器 JSON SHA-256 `c6d90860f417e3d8d6a1a095df9aff3fe44abcb856b3c63a1d9d50c6e1146134`。

dosgolem 以合成 PSP／環境、相同明示空檔，在第 6504 步、**重定位 LE 線性位址** `0x100CF` 遇到 bytes `66 81 3D BE 1C 19 00 82 00`；DS=`0188h`、來源線性 `0x191CBE`=`0000h`、EFLAGS=`0202h`。私有 `workplace/moo2-probe-232-empty-mox.txt` SHA-256 `c576678611c50c6c77bdbee720044138ef976c4abf43e904d89e36c6a5707255`。兩工具位址及堆疊不同，只以原始 bytes、受控輸入與比較運算元作局部核對。

DOSBox-X LOG 把指令印成 `cmp dword`，但 `66` operand-size 前綴、`81 /7 iw` 編碼及下一指令位址 `+9` 均指向 16 位元來源與 2-byte 立即數。以原始 bytes 與 [Intel IA-32 指令手冊第 2A 卷 `CMP` 條目](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2a-manual.pdf) 作解碼契約；不以除錯器顯示字樣推論 32 位元寬度。原版只觀測來源為零的樣本，欄位用途仍未知。

## 擬議受限 CPU 契約

對無 segment／repeat 前綴的 `66 81 /7 iw` 記憶體形狀，使用既有 `decodeAddress32` 依 DS／SS 預設段解碼 32 位元有效位址，讀 16 位元來源、取 16 位元立即數，以既有 `sub16` 更新 CF、PF、AF、ZF、SF、OF；不寫回記憶體或通用暫存器。截短指令、段越界、來源不可讀時失敗即關閉，不改記憶體或旗標。其他 `81` 形狀與前綴維持既有限制。

## 驗收

合成測試涵蓋原版零值與 `0082h`、非零相等／大小關係、不同段基址、記憶體與暫存器不變，以及截短／越界拒絕。固定原檔以相同空檔輸入自行抵達第 6504 步並單步，核對旗標與下一個自然停點；Go 全套測試仍需通過。這是執行器 CPU 工具切片，不是 MOO2 玩法已對齊或正常玩家畫面。

READY 審查：原版同次連續 LOG 的地址、來源與六旗標，以及 dosgolem 的原始 bytes／相同來源值，已足以界定此一 CPU 比較形狀；`decodeAddress32`、`readSegment16`、`sub16` 均為既有已測通用工具。非零輸入與其他記憶體編碼依 Intel 契約、須由合成測試覆蓋，不能冒稱原版多狀態實測。原版程序在空設定檔下後續如何進入玩家畫面仍未知。

CONFORMED 驗收：`internal/cpu386/cpu.go` 在既有 `81` 解碼器加入 operand-size 前綴的記憶體 `/7` 形狀，以 16 位元來源及立即數呼叫 `sub16`，不寫回。合成測試核對零值對 `0082h` 的六旗標、相等／較大來源、DS／SS 基址及截短／越界拒絕。固定原檔整合測試使用明示零位元組 `MOX.SET`，在第 6504 步、**dosgolem 重定位 LE 線性位址** `0x100CF` 自行讀出來源 `0000h`，單步後 EIP=`0x100D8`、EFLAGS=`0297h`，與 DOSBox-X 輔助收據的算術旗標一致。含原檔 `go test -buildvcs=false ./... -count=1` 全通過；最終乾淨重跑私有 `workplace/full-test-232-final.txt` SHA-256 `a6c8319d03cbfbe8cfe6cef27011b170e03d6f2bffe64bbc42a45816bf8cf69e`。空檔分支最後仍因設定資料不足而以代碼 1 結束，不是可玩收據。

另以本機正版壓縮檔根層資料 417 個檔案及官方 1.31 EXE 在一次性容器內診斷：真正的 `MOX.SET` 大小 553、SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`，其比較來源為 `0082h`；第 288215 步停於 **dosgolem 重定位 LE 線性位址** `0x151A21` 的 `66 F7 05 52 1C 1C 00 F0 FF`，尚未取得此新指令的原版同次收據。私有 `workplace/moo2-probe-232-full-game.txt` SHA-256 `c032e24493aef6d936a810d10dfe109b9884de5a78cb5527fdebcaeec63cfb9e`。完整檔案僅在容器暫存區供唯讀服務使用，不入版控；合成 PSP／環境與玩家輸入仍未對齊。
