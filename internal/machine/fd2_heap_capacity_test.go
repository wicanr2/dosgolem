package machine

import "testing"

func TestFD2WatcomRuntimeExplicitHeapCapacity(t *testing.T) {
	for _, capacity := range []uint32{1024 * 1024, 32 * 1024 * 1024} {
		m, _ := watcomHeapFixture(t, 16)
		h, err := InstallFD2WatcomRuntimeWithHeapCapacity(m, capacity)
		if err != nil {
			t.Fatal(err)
		}
		first, ok := h.allocate(1000000)
		if !ok || first < 0x100000 {
			t.Fatal("初始配置失敗")
		}
		m.Mem[first] = 0xa7
		scratch, ok := h.allocate(153216)
		if capacity == 1024*1024 {
			if ok || scratch != 0 {
				t.Fatal("小容量不得假裝配置成功")
			}
		} else {
			if !ok || scratch < first+1000000 {
				t.Fatal("明示較大容量未提供獨立區塊")
			}
			if err := h.release(scratch); err != nil {
				t.Fatal(err)
			}
			reused, ok := h.allocate(153216)
			if !ok || reused != scratch {
				t.Fatal("容量選項破壞既有回收契約")
			}
		}
		if m.Mem[first] != 0xa7 {
			t.Fatal("配置改寫既有內容")
		}
	}
}

func TestFD2WatcomRuntimeHeapCapacityBounds(t *testing.T) {
	for _, capacity := range []uint32{0, 64*1024*1024 + 1} {
		m, _ := watcomHeapFixture(t, 16)
		if _, err := InstallFD2WatcomRuntimeWithHeapCapacity(m, capacity); err == nil {
			t.Fatal("接受越界容量")
		}
	}
	m, _ := watcomHeapFixture(t, 16)
	h, err := InstallFD2WatcomRuntime(m)
	if err != nil || h.limit-h.base != 1024*1024 {
		t.Fatal("舊API預算不相容")
	}
}
