package phantasie

import (
	"fmt"
	"sort"
	"strings"
)

// Counters 是診斷計數器與鍵集合（docs/spec/001 §9）。只記數，不影響機器。
type Counters struct {
	n    map[string]uint64
	keys map[string]map[string]struct{}
}

func NewCounters() *Counters {
	return &Counters{n: map[string]uint64{}, keys: map[string]map[string]struct{}{}}
}

// Inc 計一次。
func (c *Counters) Inc(name string) { c.n[name]++ }

// Add 加 v。
func (c *Counters) Add(name string, v uint64) { c.n[name] += v }

// Get 讀值。
func (c *Counters) Get(name string) uint64 { return c.n[name] }

// Key 把一個鍵記進名為 name 的鍵集合。呼叫端負責不放玩家輸入或資料型引數內容（001 §9）。
func (c *Counters) Key(name, key string) {
	m := c.keys[name]
	if m == nil {
		m = map[string]struct{}{}
		c.keys[name] = m
	}
	m[key] = struct{}{}
}

// KeySet 回鍵集合（排序）。
func (c *Counters) KeySet(name string) []string {
	var out []string
	for k := range c.keys[name] {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Snapshot 回所有計數器的複本。
func (c *Counters) Snapshot() map[string]uint64 {
	out := make(map[string]uint64, len(c.n))
	for k, v := range c.n {
		out[k] = v
	}
	return out
}

// String 回一行可讀的摘要（名稱=值，只列非 0）。
func (c *Counters) String() string {
	names := make([]string, 0, len(c.n))
	for k, v := range c.n {
		if v != 0 {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	parts := make([]string, len(names))
	for i, k := range names {
		parts[i] = fmt.Sprintf("%s=%d", k, c.n[k])
	}
	return strings.Join(parts, " ")
}
