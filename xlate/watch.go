package xlate

// 以畫面內容當觸發點的疊字（spec 203）：畫面上某塊像素等於已知的原版圖塊時就蓋中文。
//
// 用途是「畫在圖檔裡的文字」——圖被畫到畫面的路徑不只一條，攔繪圖太貴；
// 比對「那一塊長什麼樣」是通用的，原版圖塊從哪來（解圖檔、事先存好）由呼叫端決定。

// Watcher 是一塊要盯著的畫面區域（spec 203 §2.1）。
type Watcher struct {
	Key   string  // 紀錄用；也是它加出來的疊字的 Owner
	X, Y  int     // 畫面座標（原版像素）
	W, H  int     // 區塊大小
	Want  []uint8 // 期望的色號，長度 W*H
	Probe []int   // 先比這幾個位置（Want 的索引）；空的話自己均勻取 8 點
	// Make 在比對到時呼叫，回傳要加的疊字（呼叫端決定文字、字型、格數）。
	Make func() []*Stamp
}

// probes 回實際要先比的位置。
func (w *Watcher) probes() []int {
	if len(w.Probe) > 0 {
		return w.Probe
	}
	n := len(w.Want)
	if n == 0 {
		return nil
	}
	out := make([]int, 0, 8)
	for i := 0; i < 8; i++ {
		out = append(out, i*n/8)
	}
	return out
}

// Watch 登記一個 watcher；同一個 Key 只留最後一個。
func (l *Layer) Watch(w *Watcher) {
	l.Unwatch(w.Key)
	l.watchers = append(l.watchers, w)
}

// Unwatch 移除 Key 對應的 watcher（已經加上去的疊字不動）。
func (l *Layer) Unwatch(key string) {
	keep := l.watchers[:0]
	for _, w := range l.watchers {
		if w.Key != key {
			keep = append(keep, w)
		}
	}
	l.watchers = keep
}

// Watchers 回目前登記的數量（測試與紀錄用）。
func (l *Layer) Watchers() int { return len(l.watchers) }

// alive 回這個 Key 目前有沒有活著的疊字。
func (l *Layer) alive(key string) bool {
	for _, s := range l.Stamps {
		if s.Owner == key {
			return true
		}
	}
	return false
}

// match 比對畫面上的區塊是否等於 Want。
func (w *Watcher) match(indexed []uint8, sw, sh int) bool {
	if len(w.Want) != w.W*w.H || w.W <= 0 || w.H <= 0 {
		return false
	}
	if w.X < 0 || w.Y < 0 || w.X+w.W > sw || w.Y+w.H > sh {
		return false
	}
	at := func(i int) uint8 { return indexed[(w.Y+i/w.W)*sw+w.X+i%w.W] }
	for _, i := range w.probes() {
		if i < 0 || i >= len(w.Want) || at(i) != w.Want[i] {
			return false
		}
	}
	for i := range w.Want {
		if at(i) != w.Want[i] {
			return false
		}
	}
	return true
}

// checkWatchers 由 Frame 呼叫：沒有活著的疊字的 watcher 比對成功就加疊字。
func (l *Layer) checkWatchers(indexed []uint8) {
	w, h := l.width(), l.height()
	for _, wa := range l.watchers {
		if wa.Make == nil || l.alive(wa.Key) {
			continue
		}
		if !wa.match(indexed, w, h) {
			continue
		}
		for _, s := range wa.Make() {
			if s == nil {
				continue
			}
			s.Owner = wa.Key
			l.Add(s)
		}
	}
}
