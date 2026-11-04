package main

// 批次 080 自給自足的載入（不依賴 apps/koukai2 套件，免得別批的檔案編到一半連帶編不過）。
// 設定與 apps/koukai2/koukai2.go 的 Load 相同。

import "github.com/wicanr2/dosgolem/oracle"

func Load(root string) (*oracle.Oracle, error) {
	return oracle.LoadWith(root+"/KOUKAI2.COM", root, oracle.Options{
		Adlib: true, MouseXScale: 1, COM: true,
	})
}
