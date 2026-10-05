# 422：原存檔callee的有界續行

狀態：**READY，限定私有原版有界存檔觀察**
日期：2026-10-05

依[421 原存檔交易來源](421-moo2-save-callee-file-source.md)，由[420 正常輸入完整末態](420-moo2-save-release-guard-correction.md)接續原CPU。目標是取得原SAVE1.GAM的實際檔案請求、寫入或真實拒絕，並觀察原交易返回。不增加玩家輸入、不代寫名稱、不直接呼叫保存函式，不改主庫玩法／公開CPU／DOS／probe。

## 固定前置與停止契約

- 固定官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；完整正版417檔、MOX.SET、419受控SAVE10元資料及所有原輸入保持。
- 固定420 private Go、26個只讀phase／terminal、四個正常裝置操作、source419完整失敗九phase／terminal／PNG，以及完整417／416／所有祖先。比對只略既定三個RAM hash；日期、名稱、局部值、裝置、原碼與PNG不略。
- 新旗標DOSGOLEM_MOO2_SAVE_CALLEE_CONTINUE只接受1，要求完整420與所有祖先、state及舊MAX_STEPS185M；讀EXE前拒絕。維持295CLI前綴，新305CLI預定249拒絕／56正對照。
- 旗標關閉逐bytes反轉到420，保持238505421停在10160Bh、callee未執行的舊契約與245M上限。
- 旗標開啟先重生並獨立保存完整420收據，比對通過後只解除私有停止。沿原CPU.Step自然續行，不改EIP、暫存器、guest RAM、時鐘、狀態或檔案內容。
- 新後段上限250M；3GiB／2CPU／128pids／UID1000／network none／GOMEMLIMIT1GiB、1200s外層與1150s owned state capture。原420停止收據不覆寫。新增snapshot上限256，到上限即保存結果停止，不調閾值追成功。

## 只讀觀察

在玩家owner的原CALL／下一Step與真RET邊界記錄true SS stack、實際參數／返回、裝置狀態、CPU與RAM不變、原VBE frame／PNG。

- runtime1016A0h CALL fopen_→21685Dh，真返回1016A5h。觀察原EAX filename、EDX mode與NUL bytes，不能只用來源導出字串代替實際buffer。
- fopen返回0／非0沿原TEST／分支保存，失敗訊息與末態照實記錄。
- 53個fwrite CALL只在owner邊界觀察，附實際EAX／EDX／EBX／ECX、原可讀buffer範圍、返回值與原序列。第一次E0000000／第二次37-byte名稱是待驗來源配置，不預先改成測試成功真值。
- 原DOS檔案請求附AX、DS／EDX、handle、長度、可讀資料與前後返回；只看實際INT21h或既有服務觀察，沒有新平台API、C runtime內部語意或強制成功。
- 101BCBh fclose CALL及真返回；原尾端1015FEh／raw10160Ah近RET到420的真SS返回16E3F9h／ESP＋4。若未到，不補造返回。
- 原callee返回後等待SAVE owner的下一正常16E1FDh CALL→2071ABh，原表／裝置pending核對後停止，不自動點選或輸入鍵盤。返回後畫面與名稱是否刷新必須用原PNG另驗。

不解fopen／fwrite／fclose內部。若現有工具在檔案或CPU服務拒絕，保存原請求／原碼／呼叫鏈、所有收據及拒絕，回到獨立來源與平台契約；本規格不授權增加DOS支援或放寬拒絕。

## 驗收

先完成私有生成器、CLI、唯一Step／原getter、零新輸入／CPU或guest RAM寫入、完整六份逐bytes重生、獨立verifier與來源釘選，再跑唯一新原guest。失敗不重擲，另存manifest，不覆寫祖先。

實際檔案差異只來自原EXE在容器內既有可寫overlay。原ZIP／patch／base輸入唯讀；不在原檔上寫入或預建SAVE1.GAM。原SAVE10／MOX副本若有差異照實記錄，不能用還原副本掩蓋原動作。

數值verifier、PNG人工檢視與檔案驗證分開。要宣稱本次保存完成，必須同時驗證原filename與mode、原寫入資料序列／實際file bytes、原close／返回與正常owner路徑；有檔案不代表完整保存，正常讀回仍另驗。來源與觀察器測試通過不能替代原guest。

本機入口預定new-game-422-generator.py／run.sh／implementation-source-verify.py／verify.py與moo2-colony-return-422.go。尚無422 Go／guest。正式存讀、鍵盤命名與remake同狀態保持未知，主庫RE-first不變。
