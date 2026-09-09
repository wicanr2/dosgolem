# FD2 主線接手與操作對拍驗證（2026-09-07）

## 目前狀態

| 項目 | 實跑結果 |
|---|---|
| 上游主線 | 遠端預設分支為 `master`，不存在 `main`；已 fetch 並以 pull --ff-only 更新 |
| 新分支 | `feat/fd2-input-parity-20260907` |
| 分支起點 | `fd746405cf0ef9e535121e215b935b0bf59cd1de`，與本輪 origin/master 相同 |
| 固定原版素材回歸 | 121 項 TestFD2 通過，沒有以缺素材 skip 代替驗證 |
| 其他相關回歸 | apps/fd2/parity、internal/machine、internal/cpu386、internal/dosfile 通過 |
| LE entry 到 main | 1,094 步，抵達 0x25BF4 |
| 自然執行至下一缺口 | 18,969 步，指令起點 0x3CC20，未支援 36h 段覆寫前綴 |
| 遊戲畫面、操作感對拍 | **BLOCKED**：尚未產生 dosgolem 原版畫面；未呼叫圖片比較工具冒稱完成 |

這是最新主線的重新量測，不以歷史 DOSBox 圖片代替 dosgolem 執行。
[機器收據](../evidence/fd2-main-input-parity-20260907.json)由本輪探針及測試日誌整理；
121 項通過只證明已列啟動切片，不能外推為標題、對話或第一關可玩。

## 輸入與方法

原版 FD2.EXE 為 357,074 bytes，SHA-256
`222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`。
工具為 Go 1.24 容器內的本分支程式；位址空間為 dosgolem 重定位後的 LE 線性位址，
不是檔案偏移，也不是這次重新匯出的 IDA 註記。

探針從 `LoadLE` 正常入口逐步執行，沿用主線
`InstallFD2WatcomRuntime` 與 `FD2StartupDOS`；原版根目錄唯讀。
沒有指定遊戲入口、角色、回合、座標或亂數。既有啟動服務／runtime 攔截仍存在，
因此此結果不能冒稱真實 DOS/4GW extender 的完整執行。

**已證實：**錯誤發生前指令起點為 `0x3CC20`，
bytes 為 `66 36 89 07`；CPU 錯誤文字報未支援 `36h`，
錯誤後 EIP 已移至 `0x3CC22`。收據分別保存兩者，避免把解碼器前進後的位置
誤認成原始指令起點。這與 [081 能力矩陣](../spec/081-dos4gw-capability-matrix.md)
記錄的下一缺口相同，沒有新證據可宣稱該缺口已被最新合併消除。

目前沒有修改 CPU、DOS、遊戲素材或重製引擎；新增的
[bootprobe](../../apps/fd2/cmd/bootprobe/main.go)只作有界量測。
正式對拍的下一前置工作是依現有證據／READY 規格流程補足該指令能力，
再自然執行至第一個玩家可見畫面。不能只略過指令或注入成功值。

## 重跑

在主機只啟動下列 Docker 控制命令；`原版目錄` 換成本機合法資料目錄。
容器輸出只寫到已檢查為目前使用者擁有的 `workplace/`。

```sh
timeout 90 docker run --rm --network none --memory 2g --cpus 2 --pids-limit 128 \
  -u "$(id -u):$(id -g)" -v /home/anr2/cht/dosgolem:/src \
  -v 原版目錄:/orig:ro -e HOME=/tmp \
  -e GOCACHE=/src/workplace/gocache -e GOMODCACHE=/src/workplace/gomodcache \
  -w /src golang:1.24-bookworm \
  go run ./apps/fd2/cmd/bootprobe -exe /orig/FD2.EXE -root /orig
```

目前預期探針非零退出並輸出上述缺口，不是畫面对拍成功。
測試命令為 `go test -count=1 -json ./apps/fd2/... ./internal/machine ./internal/cpu386 ./internal/dosfile`，
在相同容器另外設定 `DOSGOLEM_FD2_EXE=/orig/FD2.EXE`、
`DOSGOLEM_FD2_ROOT=/orig`。
完整日誌在本機 `workplace/fd2-input-parity-20260907/`，不含於公開庫。

本輪有界 Docker 容器均已結束並移除；未新增工具映像。
