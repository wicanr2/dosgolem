# 299：C1 dword 單位移的溢位旗標

狀態：**CONFORMED**
日期：2026-10-02
範圍：既有裸C1暫存器SHL／SHR／SAR的遮罩計數1旗標；不擴張解碼形狀。

## 已證實的工具反例與公開前提

工具基線4cdf20e347500a5c996b955831e35ef68ca556e0加298的READY D1實作，CPU SHA-256 0a2d7309b1c2f44f9bd4d5f1c16022286d5814f6cb303429e0492d47d935baab。全部D1新測試通過，但同次CPU回歸的C1 E0 01、EAX80000001h／flagsED7h，工具得到EAX2／flags603h，獨立預期EAX2／flagsE03h；唯一差異是已定義OF。這是自製CPU的反例，沒有MOO2自然OF=1同狀態收據，不能稱原版動態旗標已證實。失敗moo2-298-cpu-tests-failed.txt SHA-256 7c672d84df30bdd041903bc7812b6bb839f000de01222b8af640520eaba52b33；只讀現行C1分支確認setLogicFlags後沒有OF單位移處理。

[Intel80386 SAL／SAR／SHL／SHR](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/SAL.htm)及[Intel SDM2B](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2b-manual.pdf)印刷頁4-585 Flags Affected：遮罩計數1的SHL OF為結果最高bit XOR CF，SHR OF為原最高bit，SAR OF=0。CF／SF／ZF／PF按位移結果，非零AF未定義；計數0全部保持，非1的OF未定義。

## 候選修正與驗收

- 既有裸C1、mod11、group4／5／7取得完整imm8、計數遮罩低五位後，只有計數1新增OF處理。結果／CF與既有地址／前綴／拒絕範圍不變，其他R／段／FPU／記憶體及非算術旗標保持。
- 非零AF清除沿既有工具模型；C1多位OF目前清除，維持並明示為未定義旗標的工具模型，不為統一D3的保留模型改行為。零計數保持，不稱硬體未定義值一致。
- 三個group／八目的／全部256個imm8／每bit及補集／符號邊界／CF與OF及保持旗標初態，以逐步整除／倍增與bit計數的獨立oracle驗結果與五定義旗標，AF／多位OF模型另驗。
- 截短／前綴／未知形狀拒絕與保持、既有word／D1／D3回歸；完整CPU及固定官方EXE全套須通過，298兩自然consumer須重跑。若原版自然未命中這個OF=1條件，完成聲明限公開CPU契約修正，不假造原版oracle。

## 停止線與來源範圍

只修實際回歸找到的標準CPU錯誤，不逆向driver／ISR、不注入遊戲狀態。主庫玩法RE閘門不變。MOO2原始輸入與兩自然命令／雜湊沿 [298-cpu386-shl-dword-register-one.md](298-cpu386-shl-dword-register-one.md)。兩項須完成適用驗證，全部CPU／固定EXE及適用原版驗證通過後才各自限定CONFORMED；298仍須真實寫回consumer。原始素材與完整終端／記憶體／gzip／PNG留本機。

## READY 證據審查

2026-10-02：反例的80000001h倍增取低32位為2，移出最高bit為1，結果最高bit為0，故OF=1。低byte 2只有一個bit，PF=0；SF／ZF=0。保留非算術旗標602h得到E03h。這個計算不使用CPU移位或旗標函式。三種單位移OF已由公開Intel契約定義，AF與多位OF沿工具模型；不新增未證實遊戲語意。允許實作上述窄修正。


## 限定公開 CPU 契約修正

限定CONFORMED：既有C1 /4／/5／/7、mod11只在遮罩計數1補OF，SHL取結果符號 XOR CF、SHR取原符號、SAR清除。沒有新增解碼形狀；C1多位OF清除與非零AF清除仍只是工具模型，零計數全部保持，word／D1／D3／其他group範圍保持。

三group×八目的×256個imm8×72個值×八個初旗標組合，以逐步倍增／整除／低byte bit計數的獨立oracle驗五定義旗標與全部目的外R／段／FPU／記憶體／非算術旗標；未定義AF與多位OF模型另驗。全部CPU乾淨重跑、298既有C1反例與固定官方EXE全套及兩自然SHL寫回通過。CPU SHA-256 7cb0cade0c7b66adc37e01d458f9f22a1a57e2112afa03e62c91417d3a8a2f7c，299測試7023724891cb9865589d28f7b9488392aed471ae70818c89585f528abd39e71f；完整命令／CPU與全套收據／兩原版輸入及版本雜湊見298。

沒有MOO2自然OF=1同狀態收據，這項完成只表示公開CPU契約及獨立反例修正，不宣稱原版動態OF已對齊。自然仍停IRQ0內ADC缺件，不能稱完整IRQ0、音訊或正常玩家路徑完成。

## 解析回填

既有186的C1 dword SAR／多位未定義清除模型保持；298的回歸失敗有獨立規格修正。兩份須保存「裸C1單位移OF契約由規格 299 補齊」及本檔連結，避免把298測試預期錯改。驗證入口 apps/moo2/tools/startup_probe_131.py --check-c1-single-shift-overflow-spec-backlinks；缺反例、公開契約、未定義模型、原版限制或任一回填須拒絕。第一次CPU失敗收據保留為證據，不重寫歷史。

dword ADC停點已由規格 300 接通，見 [300-cpu386-adc-dword-register.md](300-cpu386-adc-dword-register.md)。兩自然三組完整ADC／六旗標與索引ADD真實dword消費已驗，後續word XOR停高位LE0x24678C。原有CPU／平台證據範圍不擴張，完整IRQ0返回與正常玩家路徑仍未完成。
