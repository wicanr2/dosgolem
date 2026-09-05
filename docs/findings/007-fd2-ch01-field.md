# FD2 第一戰同狀態近似對拍

本切片以 dosgolem 管理的 DOSBox 輔助場景 `ch01-field` 從固定雜湊 `FD2.EXE` 及既有
`FD2.SAV` 出發，只用一般標題輸入進入第一戰。原版擷取在一次有界 Docker
工作階段命中目標，沒有修改路由或反覆刷關：

```text
wait:10;repeat:8,Escape,500;wait:2;key:Down;key:Down;key:Return;wait:5;shot:ch01-original
```

重製端輸入是提交 `bb83e82e` 的 `640×400` 同狀態畫面。dosgolem 提交
`8032baa` 以內建 `nearest_2x` 正規化到 `320×200`，不再依賴外部
ImageMagick 隱式縮放。比較結果為 63,960／64,000 像素相同（99.9375%），
RGB 平均絕對誤差 0.00625；40 個差異全部位於左下邊界
`x=4..45, y=185..195`，內容區沒有差異。

這次原版擷取與 2026-08-10 舊錨點的 PNG 位元組及邊界相位不同，因此誠實標為
`near-state`，不可宣稱逐像素一致或外推至其他章節。機器可讀結果見
`docs/evidence/fd2-ch01-field.json`；原版 PNG、重製 PNG 與差異圖只留在
`workplace/fd2-parity/` 或來源重製庫，不把原作素材加入 dosgolem 版控。

2026-09-06 勘誤：此結果屬 `dosbox-bootstrap`，用途是補足 dosgolem 的第一戰
顯示與輸入能力；在 dosgolem 自行重生同狀態畫面前，不是正式對拍完成證據。
