package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/wicanr2/dosgolem/internal/machine"
)

func main() {
	exe := flag.String("exe", "", "要檢查的 MZ／LE 執行檔（必填）")
	flag.Parse()
	if *exe == "" {
		flag.Usage()
		os.Exit(2)
	}
	b, err := os.ReadFile(*exe)
	if err != nil {
		die(err)
	}
	h, err := machine.InspectLE(b)
	if err != nil {
		die(err)
	}
	fmt.Printf("format=LE header_offset=0x%X cpu=%d os=%d pages=%d page_size=0x%X objects=%d\n", h.Offset, h.CPUType, h.OSType, h.ModulePages, h.PageSize, h.ObjectCount)
	fmt.Printf("entry=object:%d+0x%X stack=object:%d+0x%X execution_supported=false\n", h.EIPObject, h.EIP, h.ESPObject, h.ESP)
	for i, o := range h.Objects {
		fmt.Printf("object[%d] virtual_size=0x%X relocation_base=0x%X flags=0x%X page_index=%d page_count=%d reserved=0x%X\n", i+1, o.VirtualSize, o.RelocationBase, o.Flags, o.PageTableIndex, o.PageCount, o.Reserved)
	}
}

func die(err error) { fmt.Fprintln(os.Stderr, "leprobe:", err); os.Exit(1) }
