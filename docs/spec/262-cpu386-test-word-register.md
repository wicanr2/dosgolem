# 262 — 16 位元暫存器間的 TEST

狀態：**CONFORMED（暫存器形式與定義旗標）**
日期：2026-10-01
範圍：32 位預設碼段下 `66 85 /r` 的暫存器形式，依 CPU 公開契約擴充，不改 remake 玩法。

## 證據

- **已證實，自生停點**：dosgolem `f044744b17821d429275e73c4ebc7b477a957638`、Go 1.24.13，以官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、正版 ZIP 根層 417 檔與固定 MOX.SET 重播；無事件第 6,725,985 步、受控事件第 6,726,020 步在 **dosgolem 高位 LE 線性** `0x114F43` 的 `66 85 C0 0F 85 F5 00 00 00 B8 17 0C 26 00 31 D2` 拒絕。EAX=0、EFLAGS=`246h`。工具映像／ZIP／MOX.SET／測試與診斷雜湊見 [261-moo2-dos-findfirst-current-directory.md](261-moo2-dos-findfirst-current-directory.md) 及其來源規格。
- **公開 CPU 契約**：[Intel 80386 原始 TEST 指令頁](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/TEST.htm) 列 `85 /r` 的 word／dword 形式；TEST 對操作數做 AND，只更新旗標，丟棄結果。ZF／SF／PF 依指定寬度結果，CF／OF 清零。AF 未定義；沿既有邏輯旗標策略清 AF，不能拿未定義位元宣稱全硬體相等。
- 原版同次樣本由 `apps/moo2/tools/startup_probe_131.py --test-word-register` 輔助擷取；候選位址必須以實際原始 bytes 及同次返回核對，不以位址換算冒充定位。

## 擬議契約

擴充既有 `85 /r` 暫存器路徑的運算元大小：`operand16` 時只取兩個暫存器低 16 位，重用 `setLogicFlags16`；沒有前綴的 32 位形式沿用既有邏輯。一般暫存器的高／低位、段、堆疊與記憶體都保持，EIP 正常推進三 bytes。支援所有 mod=3 的合法暫存器配對，不硬編 AX 或遊戲位址。

新的 16 位形式仍拒絕記憶體、段覆寫、重複前綴及截短；未讀完整 ModRM 不改旗標。AF 策略為工程近似，不推論所在 caller 或 runtime helper 的用途。

## 驗收計畫

先依公開 CPU 契約與固定原檔形狀審查轉 READY。覆蓋所有暫存器配對、零／符號／低 byte parity／高 16 位干擾／不同遮罩與旗標保持，並驗證既有 dword 形式、未知前綴／記憶體／截短拒絕。原版樣本值、定義旗標與後續第一個分支作有限交叉核對；固定 EXE 全套 Go 測試與兩條自然路徑自行重生。正常玩家畫面、音效、受控亂數及 Go remake 玩法同狀態仍是獨立未完成閘門。

READY 審查：公開 CPU 規格已定義暫存器讀取、寬度、旗標及結果不保存，固定 EXE 原始 bytes 證明這條自然路徑使用此形式；既有 `setLogicFlags16` 與暫存器配對解碼可重用。因此由 DRAFT 轉 READY 後實作，原版樣本只作該使用點的有限交叉檢查，不重證 CPU 標準或猜測 helper 語意。

## 原版樣本與驗收

**已證實，原版同次樣本**：DOSBox-X 2026.07.02 SDL2 heavy debugger，映像 `fd2-dosbox-x:debug-0d7b272b` ID=`sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，**CS:EIP** `0180:00248F43` 實際 16 bytes=`66 85 C0 0F 85 F5 00 00 00 B8 17 EC 38 00 31 D2`。與 dosgolem 原始序列的前三 bytes 與分支一致；後面的絕對立即數有重定位差異，不宣稱整個窗口相同。EAX=0、EBX=`A0h`、ECX=0、EDX=`00508044h`、ESI=0、EDI=`003A2090h`、EBP=`003EBC06h`、ESP=`003EBBC4h`；DS／ES／SS=`0188h`、GS=`20h`、FS=0。下一指令 `0180:00248F46` 的一般暫存器／段／堆疊保持，EFLAGS=`246h → 246h`。`JNZ` 未跳轉，抵達 `0180:00248F4C`。這個樣本 AF=0 與執行器策略相同，但 [Intel 旗標附錄](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/appc.htm) 將 TEST 的 AF 列為未定義，仍不外推其他樣本。

私有 JSON SHA-256 `67c6888c34e674e54b614556765617356fff274ac5d217b4ee3a6bebdc2a45ae`、第一個分支 LOG `70d2a992af8a687163af0a98f9a7c3b8d36d996da4477f25c3d0727b54bb50ed`、終端 `c1880f5f784129c43d4376490e610e76676f2d24f40a5a1f484624663b1bca68`；ZIP 與 MOX.SET 雜湊沿用規格 261／255。原版 EXE 與完整資料留本機，CPU 樣本測試只保存最小指令與架構輸入。

`internal/cpu386/cpu.go` 接入 word 暫存器形式，`test_word_register_test.go` 覆蓋 64 種暫存器配對及零／符號／parity／高位干擾／遮罩，核對資料不變與控制旗標保留；32 位結果另測，未知前綴／記憶體／截短仍拒絕。具名暫存器／段索引的原版 CPU 樣本與第一個 JNZ 路徑通過。固定 EXE、Go 1.24.13，最終 `DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE GOMAXPROCS=2 go test -p 2 -buildvcs=false ./... -count=1` 全套通過，`workplace/full-test-262-final.txt` SHA-256 `869b2683fc5a8f0f400d2f2b6a5134bfc48e0e2e4d4caeecedec342e70aa2a79`；加入實際樣本前的全套輸出也保留。

**已證實，新的自生停點**：無事件第 6,728,298 步、設定後受控事件第 6,728,333 步自行越過 TEST，均在 **dosgolem 高位 LE 線性** `0x222C9E`、原始 `EE E8 9D FE FF FF 66 BA C8 03 8B 35 80 38 2A 00` 的 `OUT DX,AL` 拒絕，DX=`03C6h`、AL=`FFh`、EFLAGS=`246h`。受控回呼 started=1／completed=1。兩份診斷 SHA-256 分別 `75c166c25246e0c8a79edfc5f4586f07c73dccccf5aca9ef82428d792577acb3`／`803916a40f99759906ac59bb207fe7528ba6848a861cd106ebb315ac17b39b79`。

解析回填：不可變鍵為固定 EXE 雜湊＋dosgolem 高位 LE `0x114F43`／DOSBox-X `0180:00248F43`＋`66 85 C0`。規格 261 必須保存「word TEST 停點已由規格 262 接通」與本檔連結；`startup_probe_131.py --check-test-word-register-spec-backlinks` 自動核對原始定位與狀態，缺舊標記必拒絕。仍無正常玩家畫面、音效、受控亂數及 Go remake 玩法同狀態收據，規格 255 維持 READY。

後續解析：**VGA 像素遮罩停點已由規格 263 接通**，見 [263-vga-dac-pel-mask.md](263-vga-dac-pel-mask.md) 的公開平台契約、色彩消費端與保存／還原驗證。上述 `0x222C9E` 是本規格完成時的歷史收據；目前自然路徑與未知邊界以規格 263 及專案活表為準，不把接通埠寫等同正常畫面完成。
