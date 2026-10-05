# 423：MOO2 保護模式普通檔建立

狀態：**CONFORMED，限定平台 AH3C／CX0工程驗證**
日期：2026-10-05

來源是[422原存檔請求與拒絕](422-moo2-save-callee-continue.md)，及Microsoft出版《The MS-DOS Encyclopedia》[Section V，Function3CH](https://www.pcjs.org/documents/books/mspl13/msdos/encyclopedia/section5/)。標準DOS API按公開契約實作，不追fopen／runtime內部。主庫玩法RE-first保持。

## 原版實際請求

固定官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；位址空間dosgolem_high_le。422在238506393、runtime237107h、原碼CD21請求AX3C80、CX0、DS188h／完整EDX2BD894h；原NUL bytes 53415645312E47414D00是SAVE1.GAM。此前AH3D寫入開啟因缺檔回AX2／CF1；AH3C工具拒絕。請求與未建立檔案已證實，原fopen返回、後續53個fwrite及close未知。

現有FD2StartupDOS只有AH3D開啟；DirectoryOverlayFiles.OpenWrite只開既有overlay或從base複製，不能建立缺檔。ReadOnlyFileProvider與WriteFileProvider不改既有契約。缺口是保護模式檔案服務，與CPU CD21解碼無關。

## 限定平台契約

Microsoft Function3CH規定AH3C、CX屬性與NUL路徑，成功CF0／AX handle，已有檔案截成零長且位置0；失敗CF1／AX3、4或5。AL不是此服務的子功能，不能把原AL80h錯當成拒絕條件。

- 只對MOO2 profile的INT21／AH3C及CX低16bit＝0接通。FD2、非零屬性與實模式橋維持未處理；不猜hidden／system／readonly等FAT屬性。
- 路徑由當次DS:完整EDX讀取，最多260bytes；overflow、descriptor越界或未終止回AX3／CF1。平坦目錄沿既有exactDOSName的8.3限制，字元限英數、底線與連字號；不支援路徑分隔、萬用字元或巢狀目錄，CON／PRN／AUX／NUL／COM1..9／LPT1..9等裝置名稱不當普通檔建立。非法普通檔名稱回AX5／CF1。
- 新增可選CreateFileProvider.CreateFile(name)介面，回io.ReadSeekCloser與error；唯讀provider或nil provider沒有能力時回AX5／CF1。與OpenWrite分開，保持原開啟不建立檔案。
- DirectoryOverlayFiles只在既有os.Root的state目錄建立或截斷普通檔，使用可讀寫handle、位置0。case-insensitive既有名稱沿stateName；symlink、目錄、重複大小寫名稱或非法名稱拒絕。不存在的state檔可直接建立，base只讀且不複製原內容；原base同名檔保持。
- 原普通檔屬性20h的查詢近似沿414；本輪不模擬FAT metadata、分享、繼承或主機chmod。
- 建立前依既有NewReusingTable範圍5..FFFE確認空閒handle，沒有空位回AX4且provider不被呼叫。成功以既有Add接入同一表，AH40／AH3E自然寫入／close；nil file或provider錯誤回AX5，不配置handle。provider若回非nil file加error須關閉檔案。
- 成功或失敗只改EAX低16bit及CF；其他暫存器／segment／EIP、guest RAM、時鐘與裝置保持。檔案改動僅在明示可寫overlay，不能預建SAVE1.GAM或固定回成功。

這是公開DOS契約與受限overlay的現代平台近似，不宣稱實機FAT逐值對齊。正式玩家存檔完成仍由後續唯一原guest另驗。

## 驗收與停止線

工程測試使用真實暫存base／state：建立缺檔、大小寫既有檔截斷、base同名檔不動、寫入／close後實際bytes、唯讀或nil provider、provider錯誤／nil file、非法名稱／symlink／目錄、32位高位pointer、越界／overflow／260未終止、handle滿不截檔、AL80與其他AL不影響、非MOO2／非零屬性／實模式仍拒絕。每案核對非輸出暫存器／RAM與handle狀態。另跑internal/machine、dos、dosfile及核心／命令程式建置。

READY後才修改public platform Go，必要範圍為internal/machine/le_startup.go、overlay_files.go與各自測試。不改CPU、公開probe或主庫玩法。工程通過只稱工程CONFORMED；422完整拒絕收據保持，下一原觀察器另建READY，成功phase凍結後自然執行AH3C，不把422的未處理末態改名為成功。

本機原輸入入口new-game-422-verification-result.json、conformance-review.json、one-guest-ready.json與failed1-422-manifest.json。423原稿與READY review另存workplace/，索引由000-index進入。423平台接線已實作；沒有新原guest。

## 來源與範圍審查

原422完整實際請求、Microsoft Function3CH、現有provider／handle／路徑政策與主庫RE-first已核對。原稿與new-game-423-ready-review.json另存；只授權上述平台接線及工程測試，不包含新原guest、CPU解碼或主庫玩法。平台接線已實作，工程結果見下節。

## 工程實作結果

internal/machine/le_startup.go只接MOO2 AH3C／CX0，AL80／00／FF不被當成子功能；overlay_files.go新增可選CreateFileProvider，建立／截斷只落state。建立前確認handle容量；無能力、nil／錯誤檔案、非法名稱與裝置名稱、指標越界／未終止均回限定錯誤，錯誤附檔案會關閉。FD2、非零屬性及實模式維持拒絕，不改CPU解碼。

兩份測試驗證實際overlay新檔、大小寫截斷、base同名檔不複製或改寫、原AH40寫入／AH3E關閉與handle重用、33個子案例及三種AL。原高位EDX2BD894h、非輸出暫存器／RAM、handle滿表不呼叫provider、symlink／目錄／大小寫碰撞拒絕均通過。只提供工程證據，不把合成檔當成原版存檔。

既有Go1.24.13 image、UID1000／network none／2GiB／2CPU／128pids／240s，go test -p2 ./internal/machine -run 'TestMOO2(ProtectedCreateFile|OverlayCreateFile)' -v，session43951 exit0；go test -p2 ./internal/machine ./internal/dos ./internal/dosfile與go build -p2 ./internal/... ./cmd/...，session95786 exit0。原422完整500份失敗與執行前輸入SHA保持，沒有新guest。

本機new-game-423-engineering-result.json釘選READY原稿、四份source與三份測試輸出。CONFORMED僅限公開DOS契約與平坦overlay平台近似；原SAVE1.GAM保存／讀回尚未驗。後續依[424原正常保存續行](424-moo2-save-create-continue.md)另驗實際請求返回、寫入、close／owner返回；422原拒絕收據不覆寫。
