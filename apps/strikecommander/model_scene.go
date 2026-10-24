package strikecommander

// 013 第二期只輸出診斷線框與覆蓋度量；正式 HD 合成仍依第三期。
import (
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/model"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
)

// SceneInput 只轉接已封存群組，資料切片仍屬原群組的唯讀複本。
// model.Render 不得修改這些資料，也不持有 StepView 或遊戲狀態。
func (g *ModelGroup) SceneInput() model.Input {
	if g == nil {
		return model.Input{}
	}
	elements := make([]model.Element, len(g.Elements))
	for i, e := range g.Elements {
		elements[i] = model.Element{Pointer: e.Pointer, Header: e.Header}
	}
	original := g.Raster
	if !g.ReplayReady || g.ReadFailed || g.UnmatchedFrames != 0 || len(g.CameraReadFailed) != len(g.Cameras) {
		original = nil
	}
	for _, failed := range g.CameraReadFailed {
		if failed {
			original = nil
		}
	}
	return model.Input{
		Key: g.Key, Cameras: g.Cameras, Objects: g.Objects,
		Raster:   hd3d.Group{Key: g.Key, Calls: g.Calls, List: g.List, Buf3D: g.Buf3D},
		Original: original, Elements: elements,
	}
}

// RenderModelDiagnostics 在合法觀察回呼之外對資料複本執行。
// 呼叫端需要的唯讀原版模型庫與表格均於載入時建立，不在渲染內讀檔。
func (g *ModelGroup) RenderModelDiagnostics(lib *realspace.Library, tables hd3d.Tables, scale int) (model.Result, error) {
	return model.Render(g.SceneInput(), lib, tables, scale)
}
