# 414：MOO2 保護模式 DOS 檔案屬性查詢

狀態：**CONFORMED，限定平台 AH43／AL0工程驗證**
日期：2026-10-05

來源是[412正常SAVE GAME](412-moo2-menu-save-normal-input.md)的原238069860拒絕，與Microsoft出版《The MS-DOS Encyclopedia》[Section V，Function43H](https://www.pcjs.org/documents/books/mspl13/msdos/encyclopedia/section5/)。沿[008](008-ems-paging.md)§4與現有findFirstExact的普通檔archive20h平台模型，不宣稱原FAT屬性逐值對拍。主庫玩法RE-first保持。

## 已證實的缺口

固定官方1.31 EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，runtime dosgolem_high_le。原412在runtime219E75 bytes CD21呼叫INT21h，EAX4300、DS188h、EDX2BD904、ECX0，拒絕後EIP219E77；檔名未取樣，不能猜成SAVE10或某個LBX。原服務的欠缺在FD2StartupDOS保護模式路由，不是CD指令解碼；16位dos已有AH43，不能拿它作保護模式已接通的證據。

Microsoft原契約：AL0、ASCIIZ路徑，成功CF0、CX屬性；失敗CF1、AX錯誤碼。具體路徑與返回仍要在下一原guest以同狀態取樣。本規格只補這個公開平台契約，不逆向DOS／Windows檔案wrapper內部。

## 限定實作契約

只在MOO2 startup profile、INT21／AH43／AL0接通。FD2 profile、AL1及其他子功能保持原拒絕，沒有假成功或檔案寫入。實模式橋本輪不擴張。使用當次DS:完整EDX讀NUL路徑，最大260bytes，與既有openReadOnly相同；溢位、descriptor越界或未終止回AX3／CF1，CX保持。不把32位pointer截成DX。

以既有ReadOnlyFileProvider.OpenRead查詢實際檔案並關閉；不配置DOS handle，不改任何檔案。nil provider／permission或其他provider錯誤回AX5；fs.ErrNotExist回AX2。普通虛擬檔成功只把CX低16bit設20h並清CF，EAX與其他暫存器／segment／EIP、RAM及CF以外flags保持。錯誤只改AX低16bit及CF，其他保持。20h是既有普通檔平台近似，不從主機chmod推論原FAT的hidden／system／read-only。

## 驗證與邊界

測試涵蓋實際原DS188h／EDX2BD904及完整32位pointer、既有檔與case-insensitive provider、缺檔、nil provider、越界、溢位、260bytes未終止、拒絕AL1及非MOO2 profile；成功／錯誤的非輸出狀態、RAM、handle與來源檔bytes保持。另跑internal/machine、internal/dos與internal/dosfile相關套件及建置。

只在工程測試通過後標示平台實作驗證；412正常原玩家路徑仍待新guest，不以測試綠宣稱已越過拒絕。保留412原Go、完整15phase與cpu_stop terminal；下一觀察器凍結此前成功phase，不把已拒絕的INT收據改寫成成功。原檔／私有JSON／PNG／Go不公開。

## 來源與契約審查

原412服務拒絕、現有16／32位路由差異、Microsoft公開契約與既有普通檔平台模型已核對。new-game-414-draft-review.json保存原稿；new-game-414-ready-review.json保存審查。只授權上述平台查詢與必要測試，不解除主庫RE-first，不把未取樣路徑猜補成已知檔名。已實作，工程驗證見下節。

## 工程實作與證據限制

internal/machine/le_startup.go接通MOO2 profile的AH43／AL0，保留FD2、AL1及實模式拒絕；沒有新CPU解碼或玩法。新增兩組測試共12個案例通過，含原高位EDX、缺檔、拒絕與非輸出狀態；internal/machine、internal/dos及internal/dosfile套件通過。go build -p2 ./internal/... ./cmd/...通過。平台測試不能證明原412已越過拒絕，下一原guest見[415](415-moo2-save-attributes-continue.md)。

首輪測試把ECX索引與已初始化handle表錯設，兩份fixture／輸出及manifest保存；修正測試，服務實作不變。首次go build ./...誤包含忽略的workplace私有Go與不同package，失敗輸出保存，改用核心與命令程式建置範圍通過；不把它列成產品缺陷，也沒有因此重跑原guest。本機忽略收據new-game-414-target-tests.txt、new-game-414-package-tests.txt、new-game-414-build-tests.txt。屬性20h仍是普通檔平台近似，當次原檔名與保存頁未驗。
