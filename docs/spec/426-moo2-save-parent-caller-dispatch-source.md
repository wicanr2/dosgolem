# 426：原保存父層返回與主畫面分派來源

狀態：**CONFORMED，限定來源**
日期：2026-10-05

接續[425父層旗標來源](425-moo2-save-parent-return-source.md)。固定官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；IDA Pro9.4 locked-v1 image SHA-256 6f6d59af49d0008c4109a5295b5f374bdc007e2d1ab28cb9de08779584de2780。IDA linear EA、原LE file offset及dosgolem_high_le分別保存，runtime各加F0000h。

## 已證實

- IDA對8012F的唯一直接code caller為104A6 CALL8012F；runtime1004A6到17012F，return1004AB。不從反編譯器名稱推定呼叫用途。
- 呼叫端原sub_1049B共151項；只匯出call局部及10687恢復分派的上下文。104AB原bytes寫byte_191F19=8，104B2跳10687。
- 10687先TEST ESI／條件跳轉；1068F讀原word_191A08，1069E驗0..43；106A7經CS間接JMP讀原103EB的44-entry表。原表每項經LE fixup核對，case8到104A6。它選擇下一場景，沒有直接CALL1171AB。
- 兩次窄IDA共135列／108EA，所有原指令與LE一致，指令fixup差異0。已閉合直接caller與表路由，不跟進各case的callee或runtime內部。

## 未知與下一觀察

SAVE後是否實際以真SS從16DA12返回1004AB、原場景word值、分派是否到106A7及實際target、下一正常reader與GUI可操作性仍未知。原425對上一層caller的留白已由此來源補齊；原415真child stack與parent旗標證據保持。

下一只讀觀察契約見[427](427-moo2-save-parent-return-observation.md)，限制原父層真返回與第一個主分派JMP，不宣稱正常玩家輸入或存讀往返完成。主庫RE-first保持。

## 重生與收據

同locked-v1 image／UID1000／network none／2GiB／2CPU／128pids／180s：sessions73043、36378 wrapper0／idat1，非空JSON／schema／原EXE SHA／5365函式及UID1000核對通過。Go1.24.13容器python3 workplace/new-game-426-source-verify.py退出0，原MZ／LE／51363 fixup及44個表target核對通過。

本機new-game-426-source-result.json保存14份來源SHA；source-plan.json、source-verify.py／tests、兩組moo2-426-ida-save-parent-callers及save-parent-resume腳本／JSON／log／stdout、ida-run.sh／output保存原輸出。公開只提交自撰文件；000-index、424及425同次回鏈，原反組譯／JSON不公開。

後續[427結果](427-moo2-save-parent-return-observation.md)已驗原entry17012F及真SS1004AB，原250M未達parent返回與分派。418獨立PNG及owned生命週期通過；整體仍DRAFT。下一只追parent局部退出旗標與保存後玩家consumer，末態支援候選邊界見[428](428-moo2-save-terminal-support-boundary.md)。
