package main

// 批次 078 自給自足的位址換算與載入（不依賴 apps/koukai2 套件，免得別批的檔案編到一半連帶編不過）。
// 內容與 apps/koukai2/koukai2.go、boot.go 的對應函式相同。

import (
	"fmt"
	"strings"

	"github.com/wicanr2/dosgolem/oracle"
)

const (
	MainDGroup uint32 = 0x48150
	MainPSP    uint16 = 0x02CF
	KeyGap            = 2_000_000
)

func Load(root string) (*oracle.Oracle, error) {
	return oracle.LoadWith(root+"/KOUKAI2.COM", root, oracle.Options{
		Adlib: true, MouseXScale: 1, COM: true,
	})
}

type Bind struct {
	Name    string
	PSP     uint16
	Running bool
}

func MainAt() Bind { return Bind{Name: "MAIN.EXE", PSP: MainPSP, Running: true} }

func Current(o *oracle.Oracle) (Bind, bool) {
	log := o.ExecLog()
	for i := len(log) - 1; i >= 0; i-- {
		r := log[i]
		if r.TSR || r.Ended {
			continue
		}
		return Bind{Name: strings.ToUpper(r.Base), PSP: r.PSP, Running: true}, true
	}
	return Bind{}, false
}

func IsMain(o *oracle.Oracle) bool {
	b, ok := Current(o)
	return ok && b.Name == "MAIN.EXE"
}

func (b Bind) loadSeg() uint16 { return b.PSP + 0x10 }

func (b Bind) Code(linear uint32) oracle.Addr {
	off := linear - 0x10000
	return oracle.Addr{Seg: b.loadSeg() + uint16(off>>4), Off: uint16(off & 0xF)}
}

func (b Bind) Seg(idaSeg uint16) uint16 { return idaSeg - 0x1000 + b.loadSeg() }

func (b Bind) DS(off uint16) oracle.Addr {
	return oracle.Addr{Seg: b.Seg(uint16(MainDGroup >> 4)), Off: off}
}

func (b Bind) ToIDA(a oracle.Addr) uint32 {
	return a.Linear() - uint32(b.loadSeg())*16 + 0x10000
}

func CurrentOrMain(o *oracle.Oracle) (Bind, bool) {
	if len(o.ExecLog()) == 0 {
		return MainAt(), true
	}
	return Current(o)
}

func Describe(o *oracle.Oracle) string {
	b, ok := CurrentOrMain(o)
	if !ok {
		return fmt.Sprintf("程式 無  CS:IP %04X:%04X", o.IP().Seg, o.IP().Off)
	}
	return fmt.Sprintf("程式 %s PSP=%04X  CS:IP %04X:%04X（IDA %05X）",
		b.Name, b.PSP, o.IP().Seg, o.IP().Off, b.ToIDA(o.IP()))
}
