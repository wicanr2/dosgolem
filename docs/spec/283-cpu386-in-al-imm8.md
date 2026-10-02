# 283 — 從立即埠編號輸入 byte 至 AL

狀態：**CONFORMED**
日期：2026-10-02
範圍：cpu386 的裸 E4 imm8 與既有受控 PortIn 回呼；不建立硬體 IOPL／TSS 權限、VM86 或其他輸入寬度。

## 原始定位與公開 CPU 契約

工具基線 9b8d1a07121fab929fc94a7f529449beb0d49742，固定官方 1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f、417 原檔／MOX.SET／映像沿 [281-moo2-protected-irq0-end-chain.md](281-moo2-protected-irq0-end-chain.md)。[282-pit0-mode2-count-latch.md](282-pit0-mode2-count-latch.md) 的標準介面測試揭露保護 CPU E4 未支援，而實模式支援；其原始自然停點 bytes E6 43 EB 00 E4 40 88 C4 E4 40 86 C4 已證實。第一筆候選 **dosgolem 高位 LE 線性** 0x239B3E 的 E4 40，尚待 282 自然重跑實際命中，不以候選地址當動態證據。只處理標準 CPU 埠讀取，不分析 timer driver／ISR／busy-wait。

[原廠 Intel 80386 IN 契約鏡像](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/IN.htm)：E4 ib 選 8 位立即埠編號、補零至 16 位，讀一個 byte 寫入 AL，旗標不變。受限工具沿既有平台明示 PortIn 接受／拒絕，不聲稱硬體 I/O permission bitmap 或精確例外。

## 候選型別、轉移與拒絕

- fetch 一個 imm8，port=uint16(imm8)，不使用 EDX，也不將 80h..FFh 符號擴展；平台僅呼叫一次 byte PortIn。只有平台回 true 才寫 AL，EAX 高 24 位、全部其他暫存器／段／旗標／FPU／記憶體保持。
- 只接裸 E4；所有 66／67／segment／REP／LOCK 前綴與 E5／ED 等其他寬度不因此放寬。沿既有 cpu386 解碼的前綴拒絕規則；VM=1 另拒絕，不推定完整 x86 模式／權限。
- 缺立即值／PortIn nil／平台拒絕時保持 CPU 資料狀態，不呼叫多次、不修改 AL；解碼 EIP 可前進，平台已造成的副作用不倒帶，沿既有 I/O 模型。
- 垂直鏈為原版 E4 40 → CPU typed port → 282 凍結低高計數 → AL → 原版黑箱自然繼續；不改主庫玩法或猜寫等待來源。
- 測全部 imm8／輸入 byte、高半部／完整狀態保持、EDX 故意不同／回呼次數、缺立即值／無回呼／平台拒絕／前綴與 VM 拒絕；全部 CPU／固定 EXE 全套及兩個自然排程收據後才限定 CONFORMED。自然時鐘不同，不把 IN 數值一致當跨執行器逐值對拍。

## READY 前提與停止線

公開 CPU 契約與實際候選命中／bytes 審查後才 READY。平台計數由 282 規格近似，不深入 timer driver／忙碌等待、不追實機時序。主選單、正常玩家操作、255 完整座標／游標、音效／受控亂數與整款 remake 不由本項推定完成。


## 原版自行停點與 READY 審查

已證實：282 的固定 EXE 全套通過；兩個自然排程在第 20,634,820 步越過 OUT 43h←00h／短 JMP，實際停 **高位 LE 線性** 0x239B3E，bytes E4 40 88 C4 E4 40 86 C4 25 FF FF 00 00 3B 15 48，EAX=0、EDX=1、flags=246h；PIT readCount=5681／readPending=true／readHighNext=false，尚未消費第一讀。Micros=21092408、原版 IRQ0 started=completed=1595、等待值1、事件條件已注入。這是直接自然命中而非候選或遊戲狀態注入。

固定 EXE 全套／兩自然 gzip SHA-256：f5fe6a43116c4138f75a3348e151d8f151064bf9f3b1fbb20625fe8705141d7a／1a929e805bbf6f95dd1f3b5c990990c56abede7ed967505b1d479a4328d7325a／c24408b3ab822f01e771b505e7cd143876669c8722eeae753a4bcaff7370329b。原始有限 E4 與已證實平台缺件、公開 byte／立即埠補零／旗標保持契約足夠，審查為 READY。未借此重證 PIT 時序或逆向 driver；實作尚待 CPU 全套與原版自行繼續收據。


## 受限實作、回歸與目前自然前沿

限定 CONFORMED：裸 E4 imm8，立即 byte 埠編號補零、平台 PortIn 一次回 true 才提交 AL；全部 EAX 高位／其他暫存器／段／旗標／FPU／記憶體保持。前綴、VM、E5／ED 等其他寬度拒絕，既有 EC DX 輸入保持。不建 IOPL／TSS，平台讀取的外部副作用不倒帶，不冒稱精確硬體 exception。

Go 1.24.13／映像沿 281；GOMAXPROCS=2 go test -buildvcs=false ./internal/cpu386 -count=1 -v 全部通過，moo2-283-cpu-tests.txt SHA-256 59daba763e0352594d9246d230ed7622474f53e0d177ae9b5970afc114c64057。所有 256 埠×256 byte 輸入、EDX 故意不同、回呼一次及完整狀態保持、缺立即值／平台／平台拒絕／11 種前綴與 VM／未知寬度拒絕。固定官方 EXE 全套 DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1 通過，full-test-283.txt SHA-256 a853240f01a710fae882b28f42ea795a07208e77795c76e19b3bd2afe3501154。

沿 277 的 50M／分離 DOS／417 原檔與固定 MOX.SET 自然命令重跑，另條件加 DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1，輸出名稱 283。兩條第 20,634,818 步 OUT 前相位 credit=74946000000，OUT 自然步進後 credit=75261000000、鎖存 count=5681（1631h）。**高位 LE 線性** 0x239B3E 的第一 E4 40 後 AL=31h、EIP=239B40h、readHighNext=true；0x239B42 的第二 E4 40 後 AL=16h、EIP=239B44h、pending=false。兩次 IN 旗標246h／其他暫存器與段保持，EDX=1 並沒有誤當埠號；共享 credit 仍增加。後續公開 byte 組字自然得到 EAX=1631h，再兩次 latch 為 5302／5180。這是本工具受限近似下原版自然消費，不是跨執行器同時鐘／同數值實測；沒有注入遊戲計時／亂數。

兩個排程均第 20,637,037 步轉停 **dosgolem 高位 LE 線性** 0x254249，bytes 80 2D C0 26 27 00 08 C1 ED 08 8A C3 EB 26 8A 0D；opcode 80 的 ModRM 2D 未支援，是 byte 記憶體目的／imm8 SUB 的標準 CPU 缺件。EAX=80000000h、EBX=31488h、ECX=6BBC7Ch、EDX=272610h，DS／ES／SS=188h、flags=212h。只保存定位／bytes，不推定該遊戲欄位用途；下一步按公開 SUB 契約建立新窄規格，不追 runtime／timer driver／ISR。

IRQ0 started=completed=1595、active=false／failed=false，等待來源 DS:00271148=01 00 00 00；PIT Mode=2／Reload=5966／Generation=5、readPending=false、Micros=21094625、Deliveries=1615、Pending=false／InService=false。事件條件 requested=true／injected=true，第 1,612,067 步 x=657／y=189，仍不算 255 完整座標／游標或正常玩家操作驗收。

自然 gzip SHA-256 a4cadecfd90e37dafc30fb8cd472169cd216039956a479f5ea45a933fc77f468／9a3554fc9a7023fa375c814b6685574c5fa03aa83b4b3069df06a657a0efb181。兩 PNG SHA-256 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622，indexed 7818f5542a0404157573be6cffc0e0c8e68ce3c0f5d17d07ccdd9313fb700baf、RGB 0b150fd32588b1daca5569992ebe559c0102c837306b1af4c44d35128ec58366，同已檢視黑圖；VBE Writes=4046164 不代表目前主選單已呈現。原版完整資料／終端／gzip／PNG 留本機，只有自製原始碼／測試與文字證據公開。主選單、正常玩家路徑、音效／受控亂數及 Go remake 玩法同狀態未完成，255 READY。
