package strikecommander

// 收據截圖點與停止點共用的逐倍率合成（strike_commander 規格 012 §3.8.3，分期 (e)）。

// hdSource 是 HD 3D 合成的來源（012 §3.8.1 第 2 步：已升格群組、N 倍畫布與閘門）。
// 分期 (g) 才填欄位與建構子；目前呼叫端一律傳 nil（HD 關）。
type hdSource struct{}

// HDShot 是一個倍率的 HD 結果（012 §3.8.3：升格鑰匙、閘門為真的像素數、失效原因）。分期 (g) 才填欄位。
type HDShot struct{}

// scaleIn 是 composeScale 的輸入。
type scaleIn struct {
	Overlay *Overlay
	Flight  *FlightOverlay // 可為 nil
	FlightV []FlightText   // flight.Visible()；Flight 為 nil 時不用
	FlightG uint64         // flight.Generation()
	Labels  *LabelObserver
	Scale   int
	Frame   []byte        // 截圖點：so.LastIndexed；停止點：res.Frame
	Palette [256][3]uint8 // 截圖點：so.LastPalette；停止點：res.Palette
	HD      *hdSource     // nil 為 HD 關
}

// scaleOut 是 composeScale 的輸出；呼叫端依 §3.8.3 的表取用。
type scaleOut struct {
	Comp, Plain []byte
	Check       CompositeCheck
	Covers      []FlightCover     // fov.Covers()
	FlightHide  []string          // fov.Hidden 的複本
	FlightRep   []FlightEntry     // fov.Report()
	ArtKeys     []string          // ov.ArtKeys()
	Hidden      []string          // ov.Hidden 的複本
	BG          []BGEntry         // ov.BGReport()
	StampCovers map[string][4]int // Font == nil 的 stamp 的框（1 倍，[x0, y0, x1, y1)）；一律非 nil
	Arts        []ArtReportEntry  // ov.ArtReport()
	HDShot      HDShot            // HD 為 nil 時零值
}

// composeScale 出一個倍率的合成圖、原版放大圖與比對（012 §3.8.3 的函式內順序）：
// ov.Sync → ov.Draw → BG → ScaleIndexedRGBA → HD 3D → fov.Sync、fov.Draw → 比對 → ArtKeys、Hidden、StampCovers、Arts。
// HD 3D 與 B-HD 在分期 (g) 實作；in.HD 非 nil 時 panic，避免未實作的路徑默默當成 HD 關。
func composeScale(in scaleIn) scaleOut {
	if in.HD != nil {
		panic("strikecommander: HD 合成在規格 012 分期 (g) 實作")
	}
	ov := in.Overlay
	ov.Sync(in.Labels.Visible(), in.Labels.Generation())
	var out scaleOut
	out.Comp, _ = ov.Draw(in.Frame, in.Palette)
	out.BG = ov.BGReport()
	out.Plain = ScaleIndexedRGBA(in.Frame, in.Palette, in.Scale)
	boxes := ov.TextBoxes()
	if fov := in.Flight; fov != nil {
		fov.Sync(in.FlightV, in.FlightG)
		fov.Draw(out.Comp, in.Frame, in.Palette)
		out.Covers = fov.Covers()
		boxes = append(boxes, fov.TextBoxes()...)
		out.FlightHide, out.FlightRep = append([]string(nil), fov.Hidden...), fov.Report()
	}
	out.Check = CheckCompositeMask(out.Comp, out.Plain, in.Scale, ov.Stamps(), ov.Art(), boxes, nil, out.Covers...)
	out.ArtKeys = ov.ArtKeys()
	out.Hidden = append([]string(nil), ov.Hidden...)
	out.StampCovers = map[string][4]int{}
	for _, s := range ov.Stamps() {
		if s.Font == nil {
			out.StampCovers[s.Key] = [4]int{s.X, s.Y, s.X + s.Cells*s.CellW, s.Y + s.CellH}
		}
	}
	out.Arts = ov.ArtReport() // 011 §3.8 arts.tsv
	return out
}

// storeShotScale 把一個倍率的 composeScale 輸出存進截圖點的 ShotResult（012 §3.8.3 呼叫端表）。
// hasFlight 為 false（Flight 為 nil）時不寫 FlightHidden、FlightCovers、FlightReport 的該倍率鍵（缺鍵，不寫空值）。
// HDShot 對應的 ShotResult.HD 在分期 (g) 加。
func storeShotScale(sr *ShotResult, sc int, o scaleOut, hasFlight bool) {
	sr.Composites[sc], sr.Plain[sc] = o.Comp, o.Plain
	if hasFlight {
		sr.FlightHidden[sc], sr.FlightCovers[sc], sr.FlightReport[sc] = o.FlightHide, o.Covers, o.FlightRep
	}
	sr.Check[sc] = o.Check
	sr.ArtKeys[sc] = o.ArtKeys
	sr.Hidden[sc] = o.Hidden
	sr.BG[sc] = o.BG
	sr.Covers[sc] = o.StampCovers
	if sr.Arts == nil {
		sr.Arts = map[int][]ArtReportEntry{}
	}
	sr.Arts[sc] = o.Arts
}

// storeStopScale 把一個倍率的 composeScale 輸出存進停止點的 ReceiptResult（012 §3.8.3 呼叫端表）。
// hasFlight 為 false 時不寫 FlightReport、FlightHidden 的該倍率鍵。
func storeStopScale(res *ReceiptResult, sc int, o scaleOut, hasFlight bool) {
	res.Composites[sc], res.Plain[sc] = o.Comp, o.Plain
	res.BG[sc] = o.BG
	if hasFlight {
		res.FlightReport[sc], res.FlightHidden[sc] = o.FlightRep, o.FlightHide
	}
	res.CompositeCheck[sc] = o.Check
	if res.StopArts == nil {
		res.StopArts = map[int][]ArtReportEntry{}
	}
	res.StopArts[sc] = o.Arts
}
