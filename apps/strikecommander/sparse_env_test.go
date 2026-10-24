package strikecommander

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// 規格 460 的測試開關（只在測試讀環境變數）：
//
//	DOSGOLEM_SC_SPARSE=1|0           稀疏觀察開／關（覆寫 sparseDefault 與 ReceiptConfig.sparse）
//	DOSGOLEM_SC_SPARSE_AUDIT=1       §5.2 稽核（session 照逐步模式；摘要附加到 $DOSGOLEM_SC_OUT/sparse-audit.txt）
//	DOSGOLEM_SC_SPARSE_AUDIT_RATE=r  稽核快照的隨機取樣率（窗外）；_WINDOW=n 觀察步之後全取的步數；_SEED=s 種子
//	DOSGOLEM_SC_SPLITS=檔案          §5.1 第 4 條的切分點（每列第一欄為絕對步數；# 開頭略過）
//	DOSGOLEM_SC_SPLITS_RANDOM=種子:起:訖  在 (起, 訖) 內以固定種子產生隨機切分點，每段 1 到 2,000 萬步
//
// 實際使用的切分點寫到 $DOSGOLEM_SC_OUT/splits-used.tsv。
func init() {
	switch os.Getenv("DOSGOLEM_SC_SPARSE") {
	case "1":
		sparseOverride = 1
	case "0":
		sparseOverride = -1
	}
	out := os.Getenv("DOSGOLEM_SC_OUT")
	appendOut := func(name, s string) {
		if out == "" {
			fmt.Print(s)
			return
		}
		f, err := os.OpenFile(filepath.Join(out, name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		if _, err := f.WriteString(s); err != nil {
			panic(err)
		}
	}
	if os.Getenv("DOSGOLEM_SC_SPARSE_AUDIT") == "1" {
		sparseAuditOn = true
		if s := os.Getenv("DOSGOLEM_SC_SPARSE_AUDIT_RATE"); s != "" {
			r, err := strconv.ParseFloat(s, 64)
			if err != nil {
				panic(err)
			}
			sparseAuditRate = r
		}
		if s := os.Getenv("DOSGOLEM_SC_SPARSE_AUDIT_WINDOW"); s != "" {
			w, err := strconv.ParseUint(s, 10, 64)
			if err != nil {
				panic(err)
			}
			sparseAuditWindow = w
		}
		if s := os.Getenv("DOSGOLEM_SC_SPARSE_AUDIT_SEED"); s != "" {
			n, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				panic(err)
			}
			sparseAuditSeed = n
		}
		sparseAuditSink = func(s string) { appendOut("sparse-audit.txt", s) }
	}
	var splits []uint64
	if p := os.Getenv("DOSGOLEM_SC_SPLITS"); p != "" {
		b, err := os.ReadFile(p)
		if err != nil {
			panic(err)
		}
		for _, line := range strings.Split(string(b), "\n") {
			f := strings.Fields(line)
			if len(f) == 0 || strings.HasPrefix(f[0], "#") {
				continue
			}
			n, err := strconv.ParseUint(f[0], 10, 64)
			if err != nil {
				panic(fmt.Sprintf("DOSGOLEM_SC_SPLITS：%q：%v", line, err))
			}
			splits = append(splits, n)
		}
	}
	if s := os.Getenv("DOSGOLEM_SC_SPLITS_RANDOM"); s != "" {
		f := strings.Split(s, ":")
		if len(f) != 3 {
			panic("DOSGOLEM_SC_SPLITS_RANDOM 要 種子:起:訖")
		}
		seed, err1 := strconv.ParseInt(f[0], 10, 64)
		from, err2 := strconv.ParseUint(f[1], 10, 64)
		to, err3 := strconv.ParseUint(f[2], 10, 64)
		if err1 != nil || err2 != nil || err3 != nil {
			panic("DOSGOLEM_SC_SPLITS_RANDOM 格式錯")
		}
		rng := rand.New(rand.NewSource(seed))
		for c := from + 1 + uint64(rng.Int63n(20_000_000)); c < to; c += 1 + uint64(rng.Int63n(20_000_000)) {
			splits = append(splits, c)
		}
	}
	if splits != nil {
		sort.Slice(splits, func(i, j int) bool { return splits[i] < splits[j] })
		u := splits[:0]
		for i, c := range splits {
			if i == 0 || c != splits[i-1] {
				u = append(u, c)
			}
		}
		receiptSplitsOverride = u
		var b strings.Builder
		for _, c := range u {
			fmt.Fprintf(&b, "%d\n", c)
		}
		appendOut("splits-used.tsv", b.String())
	}
}
