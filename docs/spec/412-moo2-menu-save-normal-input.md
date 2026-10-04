# 412：原GAME選單的正常SAVE GAME輸入

狀態：**DRAFT，正常輸入已驗，原存檔入口未到**
日期：2026-10-05

來源是[411](411-moo2-game-frontier-continue.md)實際原正常輸入、[407](407-moo2-menu-control-input-source.md)控件writer與完整mode0，以及[395](395-moo2-player-return-options-source.md)原case3。官方1.31 EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；IDA9.4 linear EA、runtime＝EA＋F0000h與原file offset分列。本契約只建立私有原版觀察器，不修改主庫玩法或解除RE-first。

## 當次已證實來源

411原236253170 reader的11項／605bytes控件表，pointer298848、bias0，SHA-256 c3b06da11b742fa587e4ee45f9fcfb71484b9962ef5b276805132f2e8b1f3a2f。SAVE GAME為index3、kind0、signed矩形184,68..274,95；GUI229,81對應裝置458,81。四角與中心均first-hit3，四個相鄰界外點均first-hit9。原PNG人工確認文字，與數值驗證分開。

原7D1BB將DX寫入物件+2Ah；當次物件4516E0的+2Ah確實為3。原7DF12比較輸入DX與+2Ah，7DF18比較word_1919E4與0，7DF20非零轉提示分支；零才經7DF29把word_191830寫3。原802C9取EBP−Ch，802CC CALL7E154，返回802D1。啟用word的當次值未知，必須在任何新press前只讀runtime2819E4，非零即拒絕，不改RAM補成0。

## 固定前置與輸入契約

固定411 Go SHA-256 f789844c77cc8cc0e6651d6ec4726647300b4590fdf1e480d540d79e763c49a2，完整12phase SHA-256 dd32a754b6fe38627153053e365a65ec7ba6e6865a2f3b672d4ffb8a26180e4c，terminal SHA-256 ec239a46d0c0bea9edfbdeb968e11d717a5a8798776ccf7d280bf9b7b209a1f0。所有舊完整前置保持。新旗標只接受1，依賴完整411旗標及所有祖先、state與MAX_STEPS185M；讀EXE前拒絕。關閉必須逐bytes反轉到固定411及原236253170停止契約。

在411原終止點、任何新CPU.Step前重生並凍結完整terminal及12phase，只略既定RAM hash鍵。核對11表、實際binding3、啟用word0、零按鍵、IF、callback idle及trueSS188h。通過後只呼叫正常裝置press458,81,1，不直接派送3、不寫mode或guest RAM。

沿既有selector原113FB9真SS返回與唯一下一Step取得實際結果3，確認新的GUI229,81。只有已消費、elapsed至少20000µs及callback idle後，才用正常裝置release458,81,0；最多兩個新InjectMouseEvent。按下與放開前後CPU／RAM不變。未達安全條件則保存當次狀態並拒絕，不縮短時間或代寫cache。

只讀觀察原7DF12／7DF18／7DF29實際分支與writer，原1171AB正常返回16DD7C及原802CC→7E154的真CALL／下一Step、同CS／SS、ESP−4與trueSS return1702D1。到case3 CALL前若正常release未完成，即停止拒絕；不讓held狀態繼續執行存檔子流程。正常release及真CALL完成後，在16E154入口立即停止，不推定存檔頁、slot選取或檔案寫入完成。

仍以240M為有界上限，不延長。觀察最多40新phase，保留唯一Step、原getter及所有舊收據；無CPU補丁、正式RNG變更、直接入口或RAM注入。失敗保存原bytes／真SS／原末態，不把觀察器或環境失敗列成產品缺陷。

## 驗證與範圍

245CLI保留235前綴，再加8拒絕與2正對照。生成器反轉至固定411、唯一Step、原getter、兩個新裝置呼叫；獨立重建原LE bytes／fixups、全部新只讀phase、裝置press／release、完整舊前置與trueSS握手。九個當次熱區案例與來源bytes已通過new-game-412-source-verify.py；來源結果new-game-412-source-result.json不含新guest。

原418檔與來源SAVE10／MOX唯讀保持，隔離覆蓋層的實際變動另存；不得假定後續檔案不變。原PNG人工與數值分開，正式保存／讀取及remake同狀態仍未驗，不把固定日期當seed。

本機忽略入口workplace/new-game-412-source-verify.py；後續私有觀察器沿既有workplace/new-game-412-generator.py、run.sh、verify.py命名。原EXE／Go／PNG／JSON／LOG不公開。沿Go1.24.13、UID/GID1000、network none、原ZIP／patch唯讀；guest900s／state850s／3GiB／2CPU／128pids、GOMEMLIMIT1GiB、cgroup收據、owned PID trap與Docker清理保持。412私有Go及唯一原guest已執行，實際收據見下節。

## 來源與契約審查

完整411當次前置、原控件writer／啟用條件／case3 CALL、九個熱區與唯一Step邊界已核對。new-game-412-draft-review.json保存原稿hash；new-game-412-ready-review.json保存來源審查。未知啟用值保留為執行前只讀guard，不能猜補。READY只授權此私有原版觀察器；412私有Go及唯一原guest已執行，實際收據見下節。

## 實作前置驗證

私有Go SHA-256 6ad43493b3adc0ff46669e1299e755f243f57ef1e8d138cb1137ec4a0f609a67；9個精確可反轉patch、唯一Step／原getter、兩個正常裝置呼叫與245CLI含201拒絕／44正對照通過，完整235前綴保持。六份產物在容器暫存目錄逐bytes再生一致，沒有額外guest。實作來源驗證入口為workplace/new-game-412-implementation-source-verify.py，保留已凍結的412來源／熱區收據不覆寫。執行期收據完成後才判定CONFORMED；正式存讀與remake同狀態仍未驗。

## 原正常SAVE輸入與平台拒絕

245CLI、9個精確反轉patch、唯一Step／原getter與兩個新裝置呼叫通過；完整411 terminal／12phase、409 terminal／15phase、406七phase與全部較早前置保持。原236253170 actual enable word1919E4讀到0後正常press458,81,1；236263377原selector入口，236263779 C3真SS槽20E1AC，236263780唯一下一Step／ESP＋4與EAX3通過。新GUI229,81已取樣。

236264659 elapsed43132µs、callback24／24且idle，正常release458,81,0；CPU／RAM前後保持。236282811原207261 C3、真SS return16DD7C，236282812唯一下一Step返回EAX3／ESP＋4；236282825原7DF12、236282827原7DF18、236282831原7DF29 writer及下一Step寫mode3已核對。15個新phase全只讀；正常SAVE輸入消費及writer已驗。

238069860原runtime219E75 bytes CD21、AX4300／DS188h／EDX2BD904未支援，拒絕後EIP219E77；當次檔名未知，沒有到7E154。獨立數值驗證通過限定結果，不能將它寫成完整存檔入口成功。原PNG人工仍可見GAME面板、游標在SAVE GAME，沒有存檔頁。原418檔與SAVE10／MOX保持，覆蓋層沒有新差異；cgroup峰值1518264320bytes、oom／oom_kill增量0。

唯一原session26701 outer exit124；原probe exit0但明確記錄cpu_stop／step_error，resource-after、gzip與state manifest完成，Docker容器已刪除。外層900s時間邊界與原平台拒絕分列，不把probe exit0當玩家完成。389份當次產物以failed1-412前綴及manifest保存，沒有第二個guest。412完整契約回DRAFT，原已成功14phase另給後續凍結，不改原15phase或cpu_stop terminal。

來源[413](413-moo2-save-entry-input-source.md)只補下一子頁的控件與reader；平台缺口由[414](414-moo2-protected-file-attributes.md)限定工程補驗，原版續行見[415](415-moo2-save-attributes-continue.md)。正式存讀與remake同狀態仍未驗。
