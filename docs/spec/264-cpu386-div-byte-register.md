# 264 — byte 暫存器的無號除法

狀態：**CONFORMED（商餘與資料保持；旗標／例外近似明示）**
日期：2026-10-01
範圍：32 位預設碼段、無前綴 `F6 /6` 的暫存器形式；只補原版啟動的平台依賴，不改 remake 玩法。

## 證據

- **已證實，自生停點**：dosgolem `3cfd84be85738450b65102f46450d46ae91429f1`、Go 1.24.13，官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`。固定完整 ZIP 根層 417 檔與 MOX.SET，無事件第 6,728,365 步、受控事件第 6,728,400 步，均在 **dosgolem 高位 LE 線性** `0x222CCB` 的 `F6 F3 EE AC F6 E7 B3 64 F6 F3 EE AC F6 E7 B3 64` 拒絕。EAX=0、EBX=`64h`、ECX=`80h`、EDX=`3C9h`、EFLAGS=`6h`；ModRM=`F3h` 是 `/6` 暫存器 BL，不是錯誤字串所稱的記憶體。輸入／工具／診斷雜湊見 [規格 263](263-vga-dac-pel-mask.md)。
- **公開 CPU 契約**：[Intel 80386 DIV 指令頁](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/DIV.htm) 定義 `F6 /6`：AX 無號除以 byte，商放 AL、餘數放 AH。除數 0 或商超過 `FFh` 觸發除法錯誤；OF／SF／ZF／AF／PF／CF 未定義。不藉樣本重新證明 CPU 標準。
- 原版輔助使用點以 `startup_probe_131.py --div-byte-register` 有界取得；候選 **DOSBox-X CS:EIP** `0180:00356CCB` 在實際 bytes 核對前只是假說。

## 擬議契約

沿既有 F6 解碼新增 mod=3、group=6：先讀指定的 AL／CL／DL／BL／AH／CH／DH／BH，再取 AX 計算 uint16 商與餘數。除數可與輸出別名，必須在寫 AX 前捕捉值。成功只改 EAX 低 16 位，商 AL、餘 AH；高 16 位與其餘暫存器、段、堆疊及記憶體保持，EIP 前進兩 bytes。

未定義算術旗標沿既有 dword DIV 的保留策略，控制旗標保持；只稱工程策略，不宣稱算術旗標在所有硬體或原版樣本相等。除數 0／商溢位沿既有 CPU `Error` 拒絕、保持資料與旗標，不模擬 `#DE` 中斷／fault EIP。這項例外停止策略是既有平台近似，不能稱完整 CPU exception parity。

無號 byte 除法的所有八種暫存器均接通，不硬編 BL 或遊戲位址。記憶體、前綴、其他未實作 F6 group 與截短仍拒絕；既有 byte MUL／TEST 與 dword DIV 不改。未知 group 的診斷應保留實際 ModRM，不把暫存器誤稱記憶體。

## 驗收計畫

依公開契約與固定原檔形狀審查 READY 後實作。覆蓋八個除數暫存器、別名、商 0／255／溢位、非零餘數、除以 0、高 16 位保留、旗標策略、資料／段／記憶體不變與拒絕邊界；檢查原版同次商／餘與後續第一個 `OUT` 消費，不深挖顯示 driver。固定 EXE 全套測試與無事件／受控事件兩條自然路徑自行重生；規格回填與缺定位／缺舊標記必拒絕護欄一起驗證。

正常玩家畫面、音效、受控亂數與 Go remake 玩法同狀態仍是獨立未完成閘門，255 不升格。最小 CPU bytes 與架構輸入可留測試；原版檔案、記憶體及完整終端不入 Git。

READY 審查：Intel 已定義 byte 無號商／餘數、隱含 AX 與除法錯誤；固定 EXE 實際指令形狀證明暫存器依賴。既有 `reg8` 包含高 byte，dword DIV 在寫資料前檢查錯誤並保留旗標，故可沿相同策略接通全部八種 byte 暫存器。別名、溢位與錯誤時資料保持已列驗收；未定義旗標與缺 `#DE` 明示為工程近似，沒有推論 driver 或遊戲規則。由 DRAFT 轉 READY 後實作。

## 原版輔助樣本

**已證實，同次使用點與消費**：DOSBox-X 2026.07.02 SDL2 heavy debugger，映像 `fd2-dosbox-x:debug-0d7b272b` ID=`sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`。同一固定 EXE／完整資料／MOX.SET 的 `--div-byte-register` 命中 **CS:EIP** `0180:00356CCB`，16 bytes=`F6 F3 EE AC F6 E7 B3 64 F6 F3 EE AC F6 E7 B3 64`，與 dosgolem 高位 LE `0x222CCB` 原始窗口一致。

EAX=0、EBX=`64h`、ECX=`80h`、EDX=`3C9h`、ESI=`3D135Ah`、EDI=`3A2090h`、EBP=`3EBC06h`、ESP=`3EBBA0h`；DS／ES／SS=`188h`、FS=0、GS=`20h`。下一指令 `0180:00356CCD` 的一般暫存器及段保持，商 AL=0、餘 AH=0；後續 LOG 第一個 `OUT DX,AL` 寫 `03C9h` 的值 0，再至 `LODSB`。此樣本不含除法錯誤或其他除數，其他形式依公開 CPU 契約測試。

**未定義旗標差異**：原版 EFLAGS=`46h → 6h`，ZF 被清；執行器對此相同架構輸入沿既有 DIV 策略保持 `46h`。算術旗標無定義，保留這個差異，不拿旗標相等當作商餘驗收，也不把差異猜補到正式邏輯。dosgolem 自然停點的輸入旗標本來是 `6h`，兩個執行器不宣稱整段同狀態。

私有 JSON SHA-256 `35f4e556aedda3256a0b1b509515cfd07e25b888639e68db231e45d842cd18a7`、第一個消費端 LOG `de3b1aeafcba322d51f39cb73f75aa4451b6031e0f7695cbb6bf5f5a9447766c`、終端 `bcf95c8e21a007893978e4672515577c5e5bbe73fc80fe0169ab364b44d4f667`。原版輸入與完整記憶體／終端留本機，最小 bytes／具名架構樣本加入測試。

解析回填不可變鍵：固定 EXE 雜湊＋dosgolem 高位 LE `0x222CCB`／DOSBox-X `0180:00356CCB`＋`F6 F3`。規格 263 必須保留「byte DIV 停點已由規格 264 接通」及本檔連結；`startup_probe_131.py --check-div-byte-register-spec-backlinks` 自動核對來源與 READY／CONFORMED 狀態，缺定位或舊標記必拒絕。

## 實作與驗收

`internal/cpu386/cpu.go` 依 READY 接入八種 byte 暫存器的 `F6 /6`，先讀來源再寫 AX；未知 F6 形狀的診斷保留實際 ModRM。`div_byte_register_test.go` 覆蓋低／高 byte、AL／AH 別名、零／非零餘數、商 255 邊界與溢位、除以零、高半部及資料保持、旗標策略、前綴／記憶體／未知 group／截短拒絕、既有 MUL／TEST 及原版具名架構樣本與第一個 OUT。商餘以重建被除數與餘數範圍核對，邊界另有固定預期值。

Go 1.24.13、映像 `golang:1.24-bookworm` ID=`sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac`。`go test -p 2 -buildvcs=false ./internal/cpu386 -count=1` 通過，加入實際樣本前的輸出 SHA-256 `81f16209cfdab3d775e6650e4765700ec5c9c4ef4fa7709ffdf448850f93dc00`。加入原版樣本後最終 `DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE GOMAXPROCS=2 go test -p 2 -buildvcs=false ./... -count=1` 全套通過，`workplace/full-test-264-final.txt` SHA-256 `5e5a0f83ccfc9296e5748774c79dcc94d8798ac7644fdb51a64d518d3ec50742`；先前全套輸出亦保留，SHA-256 `fd5c92a847ba902e03261b05480b448e761eca1798846320c3b6c6a3a5ab87d3`。

**已證實，自生下一停點**：同一完整資料、MOX.SET 與分離 DOS arena，無事件第 6,738,645 步、設定後受控事件第 6,738,680 步，自行越過 DIV 與後續調色盤寫入，均在 **dosgolem 高位 LE 線性** `0x228C54` 的 `CD 10 61 C3 60 25 FF FF 00 00 33 D2 BB 00 00 00` 拒絕。AX=`4F05h`、BX=0、CX=0、DX=5、DS／ES／SS=`188h`、EFLAGS=`246h`、DOS 呼叫 6；拒絕後 EIP=`0x228C56`，受控回呼 started=1／completed=1。

`DOSGOLEM_MOO2_SEPARATE_DOS=1 go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game` 及增加 `DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1` 的正式診斷自行重生，兩份 gzip SHA-256 `1ba0ba7a2b7ee444757fd7e737ecd8a1d93501d1c5bb94150472cc3e83580500`／`fd6e7ba30851994f24da214fbdd147cb5bde144d092322a967c486dcc3371a45`。Python 語法、索引、既有及新增回填護欄正常／缺定位／缺舊標記必拒絕通過。規格 264 僅在商餘、資料保持與第一個消費範圍 CONFORMED；未定義旗標、缺完整除法例外及跨執行器平台配置差異明示。255 仍 READY，正常玩家畫面／音效／亂數及 Go remake 玩法同狀態未完成。

下一最小行動：核對公開 VBE `4F05h` 視窗控制契約、固定原版使用點與既有模式資訊／顯存模型；保存實際 bank 設定及消費端，不能只返回成功或把工具啟動切片算成玩法完成。
