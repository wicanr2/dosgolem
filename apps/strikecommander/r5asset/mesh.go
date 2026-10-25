package r5asset

import (
	"encoding/binary"
	"math"
)

func validateMesh(ar *jsonArena, b []byte, p profile) (meshData, error) {
	if p.Count < 1 || p.Count > 6 || uint64(len(b)) != p.BinBytes {
		return meshData{}, bad("profile", "bin/count")
	}
	if ar == nil || ar.rawN > uint32(len(ar.raw)) {
		return meshData{}, reject(Capacity, 0, 102)
	}
	root, e := ar.parse(ar.raw[:ar.rawN])
	if e != nil {
		return meshData{}, e
	}
	v := validator{}
	m := v.obj(root, "asset scene scenes nodes meshes materials textures samplers images buffers bufferViews accessors", "")
	if v.err != nil {
		return meshData{}, v.err
	}
	a := v.obj(m.get("asset"), "version generator", "")
	v.eq(a.get("version"), "2.0")
	v.eq(a.get("generator"), "m6asset r3d（Strike Commander 3D 素材試做，程式化）")
	v.num(m.get("scene"), 0)
	scenes := v.arr(m.get("scenes"), 1)
	nodes := v.arr(m.get("nodes"), 1)
	meshes := v.arr(m.get("meshes"), 1)
	buffers := v.arr(m.get("buffers"), 1)
	images := v.arr(m.get("images"), 3)
	textures := v.arr(m.get("textures"), 3)
	samplers := v.arr(m.get("samplers"), 1)
	if v.err != nil {
		return meshData{}, v.err
	}
	scene := v.obj(scenes.at(0), "nodes", "")
	ns := v.arr(scene.get("nodes"), 1)
	if v.err != nil {
		return meshData{}, v.err
	}
	v.num(ns.at(0), 0)
	node := v.obj(nodes.at(0), "mesh name", "")
	v.num(node.get("mesh"), 0)
	v.eq(node.get("name"), "model")
	buffer := v.obj(buffers.at(0), "uri byteLength", "")
	if !buffer.get("uri").equalString("model.bin") {
		return meshData{}, bad("uri", "bin basename")
	}
	v.num(buffer.get("byteLength"), p.BinBytes)
	for i := 0; i < 3; i++ {
		im := v.obj(images.at(i), "uri", "")
		if !im.get("uri").equalString(p.Images[i].Name) {
			return meshData{}, bad("uri", "image basename")
		}
		tx := v.obj(textures.at(i), "source sampler", "")
		v.num(tx.get("source"), uint64(i))
		v.num(tx.get("sampler"), 0)
	}
	sampler := v.obj(samplers.at(0), "magFilter minFilter wrapS wrapT", "")
	for i, key := range [4]string{"magFilter", "minFilter", "wrapS", "wrapT"} {
		v.num(sampler.get(key), [4]uint64{9729, 9987, 33071, 33071}[i])
	}
	material := v.arr(m.get("materials"), p.Count)
	if v.err != nil {
		return meshData{}, v.err
	}
	tex := func(x value, w uint64) { o := v.obj(x, "index", ""); v.num(o.get("index"), w) }
	for i := 0; i < p.Count; i++ {
		name := p.Materials[i]
		var mt value
		switch name {
		case "glass":
			mt = v.obj(material.at(i), "name alphaMode doubleSided pbrMetallicRoughness", "")
			v.eq(mt.get("alphaMode"), "BLEND")
			v.eq(mt.get("doubleSided"), false)
			pb := v.obj(mt.get("pbrMetallicRoughness"), "baseColorFactor metallicFactor roughnessFactor", "")
			v.floats(pb.get("baseColorFactor"), []float64{134.0 / 255, 134.0 / 255, 134.0 / 255, 0.5})
			if v.f(pb.get("metallicFactor")) != 0 || v.f(pb.get("roughnessFactor")) != 0.08 {
				v.fail("profile", "glass factors")
			}
		case "exhaust":
			mt = v.obj(material.at(i), "name emissiveTexture emissiveFactor pbrMetallicRoughness", "")
			tex(mt.get("emissiveTexture"), 0)
			v.floats(mt.get("emissiveFactor"), []float64{1, 1, 1})
			pb := v.obj(mt.get("pbrMetallicRoughness"), "baseColorFactor metallicFactor roughnessFactor", "")
			v.floats(pb.get("baseColorFactor"), []float64{0, 0, 0, 1})
			if v.f(pb.get("metallicFactor")) != 0 || v.f(pb.get("roughnessFactor")) != 1 {
				v.fail("profile", "exhaust factors")
			}
		default:
			req := "name normalTexture pbrMetallicRoughness"
			if name == "cockpit" {
				req = "name normalTexture pbrMetallicRoughness doubleSided"
			}
			mt = v.obj(material.at(i), req, "")
			if name == "cockpit" {
				v.eq(mt.get("doubleSided"), true)
			}
			tex(mt.get("normalTexture"), 1)
			pb := v.obj(mt.get("pbrMetallicRoughness"), "baseColorTexture metallicRoughnessTexture metallicFactor roughnessFactor", "")
			tex(pb.get("baseColorTexture"), 0)
			tex(pb.get("metallicRoughnessTexture"), 2)
			if v.f(pb.get("metallicFactor")) != 1 || v.f(pb.get("roughnessFactor")) != 1 {
				v.fail("profile", "opaque factors")
			}
		}
		v.eq(mt.get("name"), name)
	}
	if v.err != nil {
		return meshData{}, v.err
	}
	n := p.Count
	views := v.arr(m.get("bufferViews"), n*5)
	accessors := v.arr(m.get("accessors"), n*5)
	mesh := v.obj(meshes.at(0), "primitives", "")
	prims := v.arr(mesh.get("primitives"), n)
	if v.err != nil {
		return meshData{}, v.err
	}
	out := meshData{Count: n, Materials: p.Descriptors}
	cursor := uint64(0)
	for i := 0; i < n; i++ {
		pr := v.obj(prims.at(i), "attributes indices material mode", "")
		v.num(pr.get("mode"), 4)
		v.num(pr.get("material"), uint64(i))
		v.num(pr.get("indices"), uint64(5*i+4))
		attrs := v.obj(pr.get("attributes"), "POSITION NORMAL TANGENT TEXCOORD_0", "")
		for j, key := range [4]string{"POSITION", "NORMAL", "TANGENT", "TEXCOORD_0"} {
			v.num(attrs.get(key), uint64(5*i+j))
		}
		if v.err != nil {
			return meshData{}, v.err
		}
		var rec Primitive
		rec.Vertices = p.Vertices[i]
		rec.Indices = p.Indices[i]
		rec.Material = uint8(i)
		for role := 0; role < 5; role++ {
			ai := 5*i + role
			ac := v.obj(accessors.at(ai), "bufferView componentType count type", func() string {
				if role == 0 {
					return "min max"
				}
				return ""
			}())
			vw := v.obj(views.at(ai), "buffer byteOffset byteLength target", "")
			if v.err != nil {
				return meshData{}, v.err
			}
			v.num(ac.get("bufferView"), uint64(ai))
			v.num(vw.get("buffer"), 0)
			ct := v.u(ac.get("componentType"))
			count := v.u(ac.get("count"))
			off := v.u(vw.get("byteOffset"))
			length := v.u(vw.get("byteLength"))
			arity := [5]uint64{3, 3, 4, 2, 1}[role]
			typ := [5]string{"VEC3", "VEC3", "VEC4", "VEC2", "SCALAR"}[role]
			if v.err != nil {
				return meshData{}, v.err
			}
			size, ok := addSpan(off, count, arity, uint64(len(b)))
			if !ok || off%4 != 0 || size != length || off != cursor {
				return meshData{}, bad("span", "offset/count/view bounds")
			}
			cursor = off + size
			v.eq(ac.get("type"), typ)
			wantCount := p.Vertices[i]
			wantCT := uint64(5126)
			target := uint64(34962)
			if role == 4 {
				wantCount = p.Indices[i]
				wantCT = 5125
				target = 34963
			}
			if ct != wantCT || count != wantCount {
				v.fail("profile", "component/count")
			}
			v.num(vw.get("target"), target)
			if v.err != nil {
				return meshData{}, v.err
			}
			rec.Offset[role] = off
			var lo, hi [4]float32
			for k := range lo {
				lo[k] = float32(math.Inf(1))
				hi[k] = float32(math.Inf(-1))
			}
			for j := uint64(0); j < count; j++ {
				for k := uint64(0); k < arity; k++ {
					at := off + (j*arity+k)*4
					bits := binary.LittleEndian.Uint32(b[int(at) : int(at)+4])
					if role == 4 {
						if uint64(bits) >= p.Vertices[i] {
							return meshData{}, bad("index", "index>=vertices")
						}
						continue
					}
					x := math.Float32frombits(bits)
					if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
						return meshData{}, bad("finite", "binary float32")
					}
					if role == 3 && (x < 0 || x > 1) {
						return meshData{}, bad("uv", "outside accepted range")
					}
					if role == 2 && k == 3 && x != 1 && x != -1 {
						return meshData{}, bad("tangent-w", "not +/-1")
					}
					if x < lo[k] {
						lo[k] = x
					}
					if x > hi[k] {
						hi[k] = x
					}
				}
			}
			if role == 0 {
				mn := v.arr(ac.get("min"), 3)
				mx := v.arr(ac.get("max"), 3)
				if v.err != nil {
					return meshData{}, v.err
				}
				for k := 0; k < 3; k++ {
					if float32(v.f(mn.at(k))) != lo[k] || float32(v.f(mx.at(k))) != hi[k] {
						return meshData{}, bad("bounds", "min/max disagree")
					}
				}
			}
		}
		out.Prim[i] = rec
	}
	if cursor != uint64(len(b)) {
		return meshData{}, bad("span", "tail")
	}
	if v.err != nil {
		return meshData{}, v.err
	}
	return out, nil
}
