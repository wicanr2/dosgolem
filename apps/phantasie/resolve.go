package phantasie

import (
	"strings"
)

// Resolver 把 EventRecord 解析成譯文（docs/spec/003 §5.2）。純函式：只讀 EventRecord 與 catalog。
type Resolver struct {
	Cat  Lookup
	Wide WideFunc
}

// pieceResult 是 resolvePiece 的內部結果。
type pieceResult struct {
	zh       string
	center   bool
	replaced bool
	ok       bool
	why      Reason
	key      string
	hits     []string
	missed   []string
}

func hasLetter(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			return true
		}
	}
	return false
}

// literalHasLetter 回格式字串去掉轉換規格後的字面文字是否含英文字母。
func literalHasLetter(segs []FmtSeg) bool {
	for _, s := range segs {
		if s.Conv == nil && hasLetter(s.Lit) {
			return true
		}
	}
	return false
}

// printable 回字串是否全在 20h 至 7Eh。
func printable(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7E {
			return false
		}
	}
	return true
}

func dedupe(in []string) []string {
	if len(in) < 2 {
		return in
	}
	seen := make(map[string]struct{}, len(in))
	out := in[:0:0]
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// Resolve 是 001 §5 的 Resolve。
func (r *Resolver) Resolve(rec *EventRecord) Result {
	if r.protectedRec(rec) {
		return Result{Why: WhyProtected}
	}
	var p *Piece
	if rec.Composed != nil {
		p = rec.Composed
	} else {
		p = &Piece{Fmt: rec.Format, FmtKind: rec.FmtKind, Literal: rec.Text, Args: rec.Args, ArgStrs: rec.ArgStrs}
	}
	pr := r.resolvePiece(p)
	return Result{
		Zh: []rune(pr.zh), Center: pr.center, Key: pr.key,
		Hits: dedupe(pr.hits), Missed: dedupe(pr.missed), OK: pr.ok, Why: pr.why,
	}
}

// protectedRec 檢查事件文字、格式字串、組句與所有引數 Piece（遞迴）的格式字串是否在保護清單內（003 §9）。
func (r *Resolver) protectedRec(rec *EventRecord) bool {
	if r.Cat == nil {
		return false
	}
	hit := func(s string) bool { return r.Cat.Protected(NormalizeText(s)) }
	if hit(rec.Text) || hit(rec.Format) {
		return true
	}
	var walk func(p *Piece) bool
	walk = func(p *Piece) bool {
		if p == nil {
			return false
		}
		if hit(p.Fmt) || (p.Literal != "" && hit(p.Literal)) {
			return true
		}
		for _, a := range p.ArgStrs {
			if walk(a.Piece) {
				return true
			}
		}
		return false
	}
	if walk(rec.Composed) {
		return true
	}
	for _, a := range rec.ArgStrs {
		if walk(a.Piece) {
			return true
		}
	}
	return false
}

func (r *Resolver) lookupUI(key string) (string, bool) {
	if r.Cat == nil {
		return "", false
	}
	return r.Cat.UI(key)
}

func (r *Resolver) lookupProse(norm string) (string, string, bool) {
	if r.Cat == nil {
		return "", "", false
	}
	dk := Digest(norm)
	tr, ok := r.Cat.Prose(dk)
	return tr, dk, ok
}

func stripCenter(s string) (string, bool) {
	if strings.HasPrefix(s, string(CenterMark)) {
		return s[len(string(CenterMark)):], true
	}
	return s, false
}

func (r *Resolver) resolvePiece(p *Piece) pieceResult {
	segs, err := ParseFormat(p.Fmt)
	if err != nil {
		return pieceResult{why: WhyBadFormat}
	}
	if !HasConversions(segs) {
		return r.resolveLiteral(p)
	}

	k := strings.TrimRight(p.Fmt, " ")
	var res pieceResult
	var tpl string
	identity := false
	hasEng := literalHasLetter(segs)
	if p.FmtKind != KindStatic {
		// 以名字緩衝區等資料當格式字串時，名字含 % 會被解析成模板，不得去查 ui（003 §5.2）。
		if hasEng {
			return pieceResult{why: WhyNoKey}
		}
		tpl, identity = p.Fmt, true
	} else {
		raw, ok := r.lookupUI(k)
		if ok && raw != "" {
			var c bool
			tpl, c = stripCenter(raw)
			res.center = c
			res.hits = append(res.hits, k)
			identity = tpl == p.Fmt // 003 §5.2：恆等判準看解析後的 tpl 與格式字串相等，不論 tpl 來自 catalog 或後備
		} else if !hasEng {
			tpl, identity = p.Fmt, true
		} else {
			return pieceResult{why: WhyNoKey, key: k}
		}
	}

	nS := CountS(segs)
	if len(p.ArgStrs) < nS {
		return pieceResult{why: WhyBadArg}
	}
	strs := make([]TargetStr, 0, nS)
	for i := 0; i < nS; i++ {
		a := p.ArgStrs[i]
		if !printable(a.Content) {
			return pieceResult{why: WhyBadArg}
		}
		ts, rep, ctr, bad := r.argText(&res, a, identity && nS == 1 && strings.TrimSpace(p.Fmt) == "%s")
		if bad {
			return pieceResult{why: WhyBadArg}
		}
		if ctr {
			res.center = true
		}
		if rep {
			res.replaced = true
		}
		strs = append(strs, ts)
	}
	var appends strings.Builder
	for _, a := range p.Appends {
		if !printable(a.Content) {
			return pieceResult{why: WhyBadArg}
		}
		ts, rep, _, bad := r.argText(&res, a, false)
		if bad {
			return pieceResult{why: WhyBadArg}
		}
		if rep {
			res.replaced = true
		}
		appends.WriteString(ts.Text)
	}
	zh, err := FormatTarget(tpl, p.Args[:], strs, r.Wide)
	if err != nil {
		return pieceResult{why: WhyBadArg}
	}
	res.zh = nulCut(zh + appends.String()) // %c 為 0 時原版輸出含 NUL；比對與顯示都截到第一個 NUL（003 §5.2）
	if identity {
		if nS == 0 {
			res.why, res.zh = WhyPassthrough, ""
			return res
		}
		if !res.replaced {
			res.why, res.zh = WhyIdentity, ""
			return res
		}
	}
	res.ok = true
	return res
}

// resolveLiteral 處理沒有轉換的 Piece（003 §5.2 的「字面」分支）。
func (r *Resolver) resolveLiteral(p *Piece) pieceResult {
	t := NormalizeText(p.Literal)
	if !hasLetter(t) {
		return pieceResult{why: WhyPassthrough}
	}
	var raw, hitKey string
	var ok bool
	switch p.FmtKind {
	case KindStatic:
		raw, ok = r.lookupUI(t)
		hitKey = t
	case KindBuffer:
		raw, hitKey, ok = r.lookupProse(t)
	}
	if !ok {
		out := pieceResult{why: WhyNoKey}
		if p.FmtKind == KindStatic {
			out.key = t
		}
		return out
	}
	if raw == "" {
		out := pieceResult{why: WhyBlankMissing}
		if p.FmtKind == KindStatic {
			out.key = t
		}
		return out
	}
	body, center := stripCenter(raw)
	if body == "<blank>" && !center {
		return pieceResult{ok: true, hits: []string{hitKey}}
	}
	return pieceResult{ok: true, zh: body, center: center, hits: []string{hitKey}}
}

// argText 依 a.Kind 查表（003 §5.2 的 lookupArg 與組句 Piece）。回傳顯示用的字串、是否換成譯文、是否要求整句置中、是否資料錯誤。
func (r *Resolver) argText(res *pieceResult, a ArgStr, singleS bool) (ts TargetStr, replaced, center, bad bool) {
	orig := TargetStr{Text: a.Content}
	if a.Piece != nil {
		inner := r.resolvePiece(a.Piece)
		if inner.ok {
			res.hits = append(res.hits, inner.hits...)
			res.missed = append(res.missed, inner.missed...)
			return TargetStr{Text: inner.zh, Translated: true}, true, false, false
		}
		if inner.key != "" {
			res.missed = append(res.missed, inner.key)
		}
		res.missed = append(res.missed, inner.missed...)
		return orig, false, false, false
	}
	if strings.TrimRight(a.Content, " ") == "" {
		return orig, false, false, false
	}
	norm := NormalizeText(a.Content)
	var v, hitKey string
	var ok bool
	switch a.Kind {
	case KindStatic, KindMonster, KindTown:
		v, ok = r.lookupUI(norm)
		hitKey = norm
	case KindBuffer:
		v, hitKey, ok = r.lookupProse(norm)
	default:
		return orig, false, false, false
	}
	if !ok || v == "" {
		if a.Kind != KindBuffer {
			res.missed = append(res.missed, norm)
		}
		return orig, false, false, false
	}
	body, c := stripCenter(v)
	if c {
		if !singleS {
			return orig, false, false, true // 資料引數帶 \c 只允許卷軸行的單一 %s 恆等模板
		}
		center = true
	}
	res.hits = append(res.hits, hitKey)
	return TargetStr{Text: body, Translated: true}, true, center, false
}
