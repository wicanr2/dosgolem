# 308：dword 指標的間接遠呼叫

狀態：**CONFORMED**
日期：2026-10-03
範圍：cpu386裸FF 1D disp32；只接已知平坦CS的同權限遠呼叫。

## 定位與證據

[307](307-moo2-protected-keyboard-irq1.md)的可丟棄IRQ1診斷使用固定官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f、Go1.24.13 linux/amd64與4eb97f6277121e528a2d3ce594dbb1967d16382e工具基線。兩正常啟動先到50M且PNG沿306，然後診斷第13步在dosgolem高位LE0x21C4EE原始bytes FF 1D DC 42 2A 00拒絕，回傳EIP21C4F0。此前PUSHFD、ESP=FB8h、SS=150h／158h、flags12h，尚無鍵盤I/O與遊戲RAM改寫。

已證實原始指令／停點，未證實鍵盤流程。[Intel SDM Vol.2A 253666-089US，CALL](https://cdrdv2-public.intel.com/868140/253666-089-sdm-vol-2a.pdf)印刷頁3-121至3-128記載FF /3 m16:32、同權限code descriptor、CS及下一EIP入堆疊，不影響旗標。僅依公開CPU契約，不反組譯ISR內部。模型沿[280](280-cpu386-far-ret32.md)的平坦descriptor／同RPL近似，沒有完整code/type/DPL／gate／TSS權限驗證。CS堆疊slot高word寫零是模型選擇，不宣稱原版核心的未指定padding。

## 輸入、轉移與失敗

- 裸FF 1D disp32；66、地址大小、段、LOCK、REP前綴及其他FF /3形狀仍拒絕。
- 完整解碼disp32，再以實際DS descriptor讀六byte offset32＋selector16。六byte必須完整在段內，不能以另一段／回呼拼接來源。DS可以非平坦。
- 目前與目的CS均已知、非null、平坦、同RPL，VM=0；下一EIP須在目前CS範圍內，目的offset32須在目的段範圍且可讀。目標未知或不可讀，拒絕。
- ESP至少8；先驗證整個8byte SS寫入範圍及可讀RAM，再寫下一EIP與零延伸舊CS，ESP減8，CS／EIP切目的。讀取來源必須在寫堆疊之前，允許來源與堆疊重疊。
- 保持其他R、六段中除CS之外、全部旗標與FPU。錯誤時不提交核心寄存器，EIP維持既有解碼進度。descriptor／讀取拒絕時不寫RAM；Bus.Write8中途失敗沿既有Bus契約可能留下部分寫入，必須停止，不宣稱任意MMIO交易回滾。

## 驗收

自製稀疏Bus：四RPL、完整32位目標、非平坦DS／SS、所有必要來源byte、堆疊下溢／段界／線性溢位／不可寫、未知CS／不同權限／VM／不可讀、截短disp32／前綴／其他FF /3拒絕；檢查完整核心及8byte寫回，另用已完成CB真實返回消費，不鏡像decode控制流。

全部CPU與固定EXE測試，然後兩自然307診斷。原版FF 1D須實際讀DS:2A42DC、寫8byte返回框架並抵達AH3509保存的108:326009，正式50M基線須沿306。default IRQ1服務仍未知時停在邊界，不以虛構handler或BIOS直接入隊稱鍵盤完成。

主庫玩法／UI／存檔無改動；垂直影響限CPU→原始遠指標→實際IRQ1入口。原始RAM與收據不提交。CONFORMED只涵蓋裸指令及此診斷的遠呼叫消費，不能稱正常玩家流程／整款remake完成。


## 驗證與回填

狀態：限定**CONFORMED**。四RPL／六個完整32位目標、非平坦DS／SS與來源／堆疊重疊、六來源byte／八堆疊byte缺項、截短disp32／11前綴／其他FF /3／權限／描述子／段界／線性溢位均通過。每個成功案例實際執行既有CB讀取寫回的8byte遠返回框架，恢復下一EIP=6、舊CS與原ESP，沒有直接改PC代替消費。

完整CPU／固定EXE全套通過，收據599bcf5266ca68853dd7fb1d1b43da4046b32b26ef4d652ae39e6558e709b5a4。兩自然先完成306的50M逐列基線，再進307診斷：原始DS:2A42DC六byte 09 60 32 00 08 01，FF 1D DC 42 2A 00在高位LE0x21C4EE真正遠CALL，ESP FB8h→FB0h、CS 8→108、EIP326009、其他R／段／flags12h保持；8byte實際堆疊F4 C4 21 00 08 00 00 00，接前一PUSHFD 12 00 00 00。第14步抵達尚未建模default09後停止，沒有I/O／遊戲RAM寫入；不冒稱此收據驗證完整IRQ1。

兩診斷gzip SHA-256 bced307f87cc1c3afb96409fcfaf247627b583ab50f80f2761c1256de3d1ca43／089b1a4ad44da3302dadedaaa610e269778b58b34cb9a8e63d3336566e6166b1。當時觀測來源40bd71baeba579370377e4229de8534e8c781a0342205a0f5592e96f9d24cd6f；後續307追加受控BIOS診斷及正式鍵盤排程另列，不覆寫此邊界。CPU SHA-256 6dffa60e7bb5b5c15c1b669a1b683dd8a2c2eb379cdcc6fc92e5a5bdb5cca09b；測試a911e37b1884d2d4ef72a9898d12fab2ef421b3a5424783fcb0448cb1a9f74bb，窄測試收據79f1afea2ac64204bb0344bfdd14f198367f6807c101a279471dd681201af659。正式307全套再次涵蓋此CPU來源。

不可變鍵：固定1.31 EXE雜湊＋高位LE0x21C4EE＋FF 1D DC 42 2A 00。307舊診斷追加「FF 1D遠呼叫停點已由規格 308 接通」及本檔連結。入口apps/moo2/tools/startup_probe_131.py --check-far-call-indirect-spec-backlinks；缺原始bytes、六bytepointer／8byteframe／flags及其他外層保持、真正CB測試消費、兩收據或舊標記必須拒絕。
