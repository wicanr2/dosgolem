// chunktest：重現前端「分塊 oracle.Run」的執行模式，驗證是否卡在
// credits→主選單的轉場（wizardry7 docs/re/009 的卡關）。
//
//	go run ./cmd/chunktest -exe <DS.EXE> -root <素材目錄>
//
// 每 chunk 166,666 道指令（≈ 前端 60fps × 10M ips），每 60 個 chunk
// （≈ 1 秒遊戲時間）印一次步數與畫面非零像素。
package main

import (
	"flag"
	"fmt"

	"github.com/wicanr2/dosgolem/oracle"
)

func main() {
	exe := flag.String("exe", "", "")
	root := flag.String("root", "", "")
	chunks := flag.Int("chunks", 7200, "總 chunk 數（7200 ≈ 1.2B 指令）")
	flag.Parse()
	o, err := oracle.LoadWith(*exe, *root, oracle.Options{Adlib: true})
	if err != nil {
		panic(err)
	}
	const chunk = 166_666
	nz := func() int {
		n := 0
		for _, v := range o.Indexed() {
			if v != 0 {
				n++
			}
		}
		return n
	}
	for i := 0; i < *chunks; i++ {
		if err := o.Run(chunk); err != nil {
			fmt.Printf("chunk %d（%d 步）錯誤：%v\n", i, o.Steps(), err)
			return
		}
		if i%60 == 0 {
			fmt.Printf("chunk %d steps=%d 非零=%d\n", i, o.Steps(), nz())
		}
	}
	fmt.Printf("完成 %d chunks，steps=%d 非零=%d\n", *chunks, o.Steps(), nz())
}
