package r5asset

import (
	"crypto/sha256"
	"encoding/binary"
)

type fileIdentity struct {
	Name  string
	Bytes uint64
	SHA   [32]byte
	Color byte
}
type profile struct {
	Name               string
	Model              uint8
	Count              int
	BinBytes           uint64
	Vertices, Indices  [6]uint64
	Materials          [6]string
	Images             [3]fileIdentity
	Files              [5]fileIdentity
	Descriptors        [6]Material
	Origin, Provenance [32]byte
}

var profiles = [2]profile{
	{Name: "f16", Model: 1, Count: 6, BinBytes: 2520096, Vertices: [6]uint64{26103, 1386, 3315, 268, 1105, 4389}, Indices: [6]uint64{135600, 7200, 18432, 1056, 6144, 22800}, Materials: [6]string{"paint", "cockpit", "metal", "intake", "exhaust", "glass"}, Files: [5]fileIdentity{{Name: "model.gltf", Bytes: 9840, SHA: [32]byte{214, 187, 173, 165, 93, 36, 173, 40, 106, 71, 243, 33, 43, 16, 184, 49, 154, 234, 56, 207, 205, 155, 157, 246, 134, 167, 224, 134, 81, 134, 237, 62}, Color: 2}, {Name: "model.bin", Bytes: 2520096, SHA: [32]byte{156, 201, 89, 204, 47, 218, 120, 59, 241, 197, 191, 58, 197, 117, 122, 74, 224, 210, 134, 233, 46, 136, 254, 29, 117, 144, 166, 118, 177, 221, 87, 182}, Color: 2}, {Name: "albedo.png", Bytes: 3186168, SHA: [32]byte{158, 65, 139, 133, 41, 55, 243, 57, 240, 44, 180, 39, 200, 43, 10, 15, 152, 148, 90, 27, 178, 153, 129, 106, 82, 86, 139, 3, 188, 133, 74, 8}, Color: 6}, {Name: "normal.png", Bytes: 1501881, SHA: [32]byte{135, 107, 228, 208, 107, 71, 41, 222, 63, 37, 162, 120, 178, 95, 153, 89, 103, 143, 182, 24, 178, 1, 150, 36, 188, 152, 57, 164, 123, 75, 29, 212}, Color: 2}, {Name: "metal_rough.png", Bytes: 60071, SHA: [32]byte{140, 80, 182, 33, 59, 27, 80, 241, 37, 58, 163, 228, 112, 10, 12, 227, 150, 198, 42, 11, 91, 114, 202, 152, 140, 94, 139, 131, 41, 218, 184, 54}, Color: 2}}, Images: [3]fileIdentity{{Name: "albedo.png", Bytes: 3186168, SHA: [32]byte{158, 65, 139, 133, 41, 55, 243, 57, 240, 44, 180, 39, 200, 43, 10, 15, 152, 148, 90, 27, 178, 153, 129, 106, 82, 86, 139, 3, 188, 133, 74, 8}, Color: 6}, {Name: "normal.png", Bytes: 1501881, SHA: [32]byte{135, 107, 228, 208, 107, 71, 41, 222, 63, 37, 162, 120, 178, 95, 153, 89, 103, 143, 182, 24, 178, 1, 150, 36, 188, 152, 57, 164, 123, 75, 29, 212}, Color: 2}, {Name: "metal_rough.png", Bytes: 60071, SHA: [32]byte{140, 80, 182, 33, 59, 27, 80, 241, 37, 58, 163, 228, 112, 10, 12, 227, 150, 198, 42, 11, 91, 114, 202, 152, 140, 94, 139, 131, 41, 218, 184, 54}, Color: 2}}, Origin: [32]byte{156, 48, 247, 71, 17, 168, 80, 33, 175, 14, 74, 212, 175, 206, 134, 17, 66, 127, 182, 202, 196, 38, 223, 141, 23, 2, 159, 155, 54, 175, 154, 131}, Provenance: [32]byte{207, 228, 45, 77, 162, 87, 75, 148, 162, 20, 33, 51, 93, 233, 0, 126, 39, 98, 52, 87, 199, 136, 89, 253, 101, 225, 54, 45, 129, 146, 217, 170}, Descriptors: [6]Material{{Kind: 1, Presence: 964, DoubleSided: false, AlphaMode: 0, BaseColor: [4]float64{0, 0, 0, 0}, Emissive: [3]float64{0, 0, 0}, Metallic: 1, Roughness: 1, BaseTexture: 0, NormalTexture: 1, MetalRoughTexture: 2, EmissiveTexture: -1, MagFilter: 9729, MinFilter: 9987, WrapS: 33071, WrapT: 33071}, {Kind: 2, Presence: 965, DoubleSided: true, AlphaMode: 0, BaseColor: [4]float64{0, 0, 0, 0}, Emissive: [3]float64{0, 0, 0}, Metallic: 1, Roughness: 1, BaseTexture: 0, NormalTexture: 1, MetalRoughTexture: 2, EmissiveTexture: -1, MagFilter: 9729, MinFilter: 9987, WrapS: 33071, WrapT: 33071}, {Kind: 3, Presence: 964, DoubleSided: false, AlphaMode: 0, BaseColor: [4]float64{0, 0, 0, 0}, Emissive: [3]float64{0, 0, 0}, Metallic: 1, Roughness: 1, BaseTexture: 0, NormalTexture: 1, MetalRoughTexture: 2, EmissiveTexture: -1, MagFilter: 9729, MinFilter: 9987, WrapS: 33071, WrapT: 33071}, {Kind: 4, Presence: 964, DoubleSided: false, AlphaMode: 0, BaseColor: [4]float64{0, 0, 0, 0}, Emissive: [3]float64{0, 0, 0}, Metallic: 1, Roughness: 1, BaseTexture: 0, NormalTexture: 1, MetalRoughTexture: 2, EmissiveTexture: -1, MagFilter: 9729, MinFilter: 9987, WrapS: 33071, WrapT: 33071}, {Kind: 5, Presence: 824, DoubleSided: false, AlphaMode: 0, BaseColor: [4]float64{0, 0, 0, 1}, Emissive: [3]float64{1, 1, 1}, Metallic: 0, Roughness: 1, BaseTexture: -1, NormalTexture: -1, MetalRoughTexture: -1, EmissiveTexture: 0, MagFilter: 9729, MinFilter: 9987, WrapS: 33071, WrapT: 33071}, {Kind: 6, Presence: 803, DoubleSided: false, AlphaMode: 1, BaseColor: [4]float64{0.5254901960784314, 0.5254901960784314, 0.5254901960784314, 0.5}, Emissive: [3]float64{0, 0, 0}, Metallic: 0, Roughness: 0.08, BaseTexture: -1, NormalTexture: -1, MetalRoughTexture: -1, EmissiveTexture: -1, MagFilter: 9729, MinFilter: 9987, WrapS: 33071, WrapT: 33071}}},
	{Name: "mig29", Model: 2, Count: 5, BinBytes: 2702256, Vertices: [6]uint64{27027, 1155, 6630, 2210, 2079}, Indices: [6]uint64{140400, 6000, 36864, 12288, 10800}, Materials: [6]string{"paint", "cockpit", "metal", "exhaust", "glass"}, Files: [5]fileIdentity{{Name: "model.gltf", Bytes: 8344, SHA: [32]byte{135, 7, 219, 216, 247, 181, 66, 19, 253, 134, 155, 243, 217, 42, 139, 41, 77, 141, 203, 29, 172, 190, 29, 53, 117, 228, 110, 109, 77, 128, 107, 8}, Color: 2}, {Name: "model.bin", Bytes: 2702256, SHA: [32]byte{26, 227, 144, 125, 181, 117, 209, 12, 2, 145, 130, 130, 90, 84, 18, 113, 62, 131, 210, 244, 91, 121, 97, 21, 191, 5, 239, 72, 113, 34, 235, 213}, Color: 2}, {Name: "albedo.png", Bytes: 3190236, SHA: [32]byte{52, 62, 25, 147, 186, 90, 152, 88, 104, 178, 92, 92, 245, 29, 31, 65, 136, 106, 57, 72, 154, 210, 214, 41, 177, 85, 105, 78, 165, 21, 58, 86}, Color: 6}, {Name: "normal.png", Bytes: 1181242, SHA: [32]byte{67, 199, 101, 125, 48, 197, 92, 213, 120, 190, 134, 195, 85, 4, 31, 157, 71, 160, 193, 184, 40, 227, 201, 247, 49, 197, 46, 106, 217, 174, 122, 83}, Color: 2}, {Name: "metal_rough.png", Bytes: 59864, SHA: [32]byte{231, 134, 183, 102, 13, 53, 172, 208, 146, 102, 88, 17, 92, 200, 215, 153, 49, 140, 151, 103, 85, 236, 246, 55, 48, 173, 153, 190, 195, 75, 35, 7}, Color: 2}}, Images: [3]fileIdentity{{Name: "albedo.png", Bytes: 3190236, SHA: [32]byte{52, 62, 25, 147, 186, 90, 152, 88, 104, 178, 92, 92, 245, 29, 31, 65, 136, 106, 57, 72, 154, 210, 214, 41, 177, 85, 105, 78, 165, 21, 58, 86}, Color: 6}, {Name: "normal.png", Bytes: 1181242, SHA: [32]byte{67, 199, 101, 125, 48, 197, 92, 213, 120, 190, 134, 195, 85, 4, 31, 157, 71, 160, 193, 184, 40, 227, 201, 247, 49, 197, 46, 106, 217, 174, 122, 83}, Color: 2}, {Name: "metal_rough.png", Bytes: 59864, SHA: [32]byte{231, 134, 183, 102, 13, 53, 172, 208, 146, 102, 88, 17, 92, 200, 215, 153, 49, 140, 151, 103, 85, 236, 246, 55, 48, 173, 153, 190, 195, 75, 35, 7}, Color: 2}}, Origin: [32]byte{228, 153, 128, 83, 140, 246, 185, 147, 214, 105, 108, 12, 74, 22, 81, 87, 200, 255, 108, 35, 57, 73, 19, 184, 49, 236, 172, 163, 109, 199, 145, 118}, Provenance: [32]byte{44, 28, 192, 150, 127, 64, 143, 39, 173, 166, 181, 62, 67, 63, 140, 11, 85, 148, 232, 125, 26, 153, 18, 2, 211, 84, 45, 195, 254, 91, 69, 254}, Descriptors: [6]Material{{Kind: 1, Presence: 964, DoubleSided: false, AlphaMode: 0, BaseColor: [4]float64{0, 0, 0, 0}, Emissive: [3]float64{0, 0, 0}, Metallic: 1, Roughness: 1, BaseTexture: 0, NormalTexture: 1, MetalRoughTexture: 2, EmissiveTexture: -1, MagFilter: 9729, MinFilter: 9987, WrapS: 33071, WrapT: 33071}, {Kind: 2, Presence: 965, DoubleSided: true, AlphaMode: 0, BaseColor: [4]float64{0, 0, 0, 0}, Emissive: [3]float64{0, 0, 0}, Metallic: 1, Roughness: 1, BaseTexture: 0, NormalTexture: 1, MetalRoughTexture: 2, EmissiveTexture: -1, MagFilter: 9729, MinFilter: 9987, WrapS: 33071, WrapT: 33071}, {Kind: 3, Presence: 964, DoubleSided: false, AlphaMode: 0, BaseColor: [4]float64{0, 0, 0, 0}, Emissive: [3]float64{0, 0, 0}, Metallic: 1, Roughness: 1, BaseTexture: 0, NormalTexture: 1, MetalRoughTexture: 2, EmissiveTexture: -1, MagFilter: 9729, MinFilter: 9987, WrapS: 33071, WrapT: 33071}, {Kind: 5, Presence: 824, DoubleSided: false, AlphaMode: 0, BaseColor: [4]float64{0, 0, 0, 1}, Emissive: [3]float64{1, 1, 1}, Metallic: 0, Roughness: 1, BaseTexture: -1, NormalTexture: -1, MetalRoughTexture: -1, EmissiveTexture: 0, MagFilter: 9729, MinFilter: 9987, WrapS: 33071, WrapT: 33071}, {Kind: 6, Presence: 803, DoubleSided: false, AlphaMode: 1, BaseColor: [4]float64{0.5254901960784314, 0.5254901960784314, 0.5254901960784314, 0.5}, Emissive: [3]float64{0, 0, 0}, Metallic: 0, Roughness: 0.08, BaseTexture: -1, NormalTexture: -1, MetalRoughTexture: -1, EmissiveTexture: -1, MagFilter: 9729, MinFilter: 9987, WrapS: 33071, WrapT: 33071}}},
}
var decoderProfile = [32]byte{150, 137, 244, 39, 62, 45, 79, 223, 147, 106, 110, 125, 159, 29, 175, 46, 28, 36, 29, 154, 36, 174, 1, 42, 99, 131, 177, 65, 204, 234, 109, 111}

func assetCanonical(p profile) [328]byte {
	var out [328]byte
	copy(out[:], "SC-R5-ASSET-v1\n")
	i := 15
	put := func(n uint32) { binary.LittleEndian.PutUint32(out[i:i+4], n); i += 4 }
	put(1)
	out[i] = p.Model
	i++
	put(1)
	put(1)
	put(1)
	copy(out[i:i+32], decoderProfile[:])
	i += 32
	for _, f := range p.Files {
		binary.LittleEndian.PutUint64(out[i:i+8], f.Bytes)
		i += 8
		copy(out[i:i+32], f.SHA[:])
		i += 32
	}
	copy(out[i:i+32], p.Origin[:])
	i += 32
	copy(out[i:i+32], p.Provenance[:])
	return out
}
func assetHash(p profile) [32]byte { x := assetCanonical(p); return sha256.Sum256(x[:]) }
func batchHash() [32]byte {
	var b [77]byte
	copy(b[:], "SC-R5-SET-v1\n")
	a, c := assetHash(profiles[0]), assetHash(profiles[1])
	copy(b[13:45], a[:])
	copy(b[45:], c[:])
	return sha256.Sum256(b[:])
}
