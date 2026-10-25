// Fixed 013b §4 original-MZ anchors. Generated from the frozen source-only evidence.
package shadesource

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
)

type Signature struct {
	Seg, Off uint16
	Hex      string
	Reloc    []int
}

var signatures = []Signature{
	{0x1ad5, 0x0a72, "ff769e", []int{}},
	{0x1ad5, 0x0a75, "52", []int{}},
	{0x1ad5, 0x0a7c, "ff5f18", []int{}},
	{0x1ad5, 0x0a7c, "ff5f1883c4080ac074578d8634ff5066", []int{}},
	{0x1ad5, 0x0a7f, "83c4080ac074578d8634ff5066ff769c", []int{}},
	{0x1ad5, 0x0a8b, "66ff769c", []int{}},
	{0x1ad5, 0x0a95, "ff5f24", []int{}},
	{0x1ad5, 0x0a9b, "66ff769c", []int{}},
	{0x1ad5, 0x0aa5, "ff5f1c", []int{}},
	{0x1ad5, 0x0aa5, "ff5f1c83c40489569a89469866837e98", []int{}},
	{0x1ad5, 0x0aab, "89569a89469866837e98007425668b46", []int{}},
	{0x1d5e, 0x0ea8, "c86800001e5657fcf7c40300742eb912", []int{}},
	{0x20a8, 0x0002, "c80e000056571ec57606c474028976f2", []int{}},
	{0x20a8, 0x006b, "c57406", []int{}},
	{0x20a8, 0x006e, "26c45f1a", []int{}},
	{0x20a8, 0x025f, "658b740a", []int{}},
	{0x20a8, 0x0266, "648b5f1e", []int{}},
	{0x20a8, 0x0454, "658b740e", []int{}},
	{0x20a8, 0x045b, "648b5f22", []int{}},
	{0x2579, 0x0405, "6626894706", []int{}},
	{0x2579, 0x040e, "662689470a", []int{}},
	{0x2579, 0x0417, "662689470e", []int{}},
	{0x2579, 0x0c02, "9a60042b28", []int{3}},
	{0x2579, 0x0c07, "83c406", []int{}},
	{0x2579, 0x0c0a, "c45e06", []int{}},
	{0x2579, 0x0c0d, "26884704", []int{}},
	{0x2579, 0x10ec, "558bec66ff7606666a00900ee8050083c4085dcb", []int{}},
	{0x2579, 0x10ef, "66ff7606", []int{}},
	{0x2579, 0x10f3, "666a00", []int{}},
	{0x2579, 0x1151, "26c45f06", []int{}},
	{0x2579, 0x115b, "26894706", []int{}},
	{0x2579, 0x1174, "26c45f0a", []int{}},
	{0x2579, 0x117e, "2689471c", []int{}},
	{0x2579, 0x1197, "26c45f0e", []int{}},
	{0x2579, 0x11a1, "26894732", []int{}},
	{0x2579, 0x12eb, "9a3d085e1d", []int{3}},
	{0x2579, 0x1633, "55", []int{}},
	{0x2579, 0x1634, "8bec", []int{}},
	{0x2579, 0x1636, "ff760a", []int{}},
	{0x2579, 0x1639, "66ff7606", []int{}},
	{0x2579, 0x163d, "8b4606", []int{}},
	{0x2579, 0x1640, "051200", []int{}},
	{0x2579, 0x1643, "ff7608", []int{}},
	{0x2579, 0x1646, "50", []int{}},
	{0x2579, 0x1647, "0e", []int{}},
	{0x2579, 0x1648, "e8e7f4", []int{}},
	{0x2579, 0x164b, "83c40a", []int{}},
	{0x2579, 0x1664, "55", []int{}},
	{0x2579, 0x1665, "8bec", []int{}},
	{0x2579, 0x1667, "83ec0e", []int{}},
	{0x2579, 0x166a, "56", []int{}},
	{0x2579, 0x166b, "c45e06", []int{}},
	{0x2579, 0x166e, "268a4716", []int{}},
	{0x2579, 0x1672, "b400", []int{}},
	{0x2579, 0x1674, "8946fe", []int{}},
	{0x2579, 0x1677, "66a1da51", []int{}},
	{0x2579, 0x167b, "8b16d851", []int{}},
	{0x2579, 0x167f, "0316ca51", []int{}},
	{0x2579, 0x1683, "8946fc", []int{}},
	{0x2579, 0x1686, "8956fa", []int{}},
	{0x2579, 0x1689, "8b46fe", []int{}},
	{0x2579, 0x168c, "fecc", []int{}},
	{0x2579, 0x168e, "34ff", []int{}},
	{0x2579, 0x1690, "22c4", []int{}},
	{0x2579, 0x1692, "b400", []int{}},
	{0x2579, 0x1694, "c1e804", []int{}},
	{0x2579, 0x1697, "c1e008", []int{}},
	{0x2579, 0x169a, "0146fa", []int{}},
	{0x2579, 0x169d, "8b46fc", []int{}},
	{0x2579, 0x16a0, "8b56fa", []int{}},
	{0x2579, 0x16a3, "a34e51", []int{}},
	{0x2579, 0x16a6, "89164c51", []int{}},
	{0x2579, 0x1887, "6626c7474a00000000", []int{}},
	{0x2579, 0x1890, "8a460e", []int{}},
	{0x2579, 0x1893, "b400", []int{}},
	{0x2579, 0x1895, "2689470a", []int{}},
	{0x2579, 0x1899, "66a14c51", []int{}},
	{0x2579, 0x189d, "662689474a", []int{}},
	{0x2579, 0x18f5, "6626ff774a", []int{}},
	{0x2579, 0x18fa, "268b5f48", []int{}},
	{0x2579, 0x18fe, "8a4705", []int{}},
	{0x2579, 0x1901, "b400", []int{}},
	{0x2579, 0x1903, "50", []int{}},
	{0x2579, 0x1904, "6a00", []int{}},
	{0x2579, 0x1906, "8b5e06", []int{}},
	{0x2579, 0x1909, "268b5f48", []int{}},
	{0x2579, 0x190d, "8a4704", []int{}},
	{0x2579, 0x1910, "50", []int{}},
	{0x2579, 0x1911, "8b5e06", []int{}},
	{0x2579, 0x1914, "268b5f48", []int{}},
	{0x2579, 0x1918, "66ff37", []int{}},
	{0x2579, 0x191b, "68d859", []int{}},
	{0x2579, 0x191e, "9af901e85a", []int{3}},
	{0x2579, 0x1923, "83c40a", []int{}},
	{0x2579, 0x1926, "52", []int{}},
	{0x2579, 0x1927, "50", []int{}},
	{0x2579, 0x1928, "8b4606", []int{}},
	{0x2579, 0x192b, "050600", []int{}},
	{0x2579, 0x192e, "ff7608", []int{}},
	{0x2579, 0x1931, "50", []int{}},
	{0x2579, 0x1932, "6a03", []int{}},
	{0x2579, 0x1934, "ff361a3f", []int{}},
	{0x2579, 0x1938, "9aa80e5e1d", []int{3}},
	{0x2579, 0x193d, "83c412", []int{}},
	{0x2579, 0x1b70, "558bec66ff7606666a00900ee8050083c4085dcb", []int{}},
	{0x2579, 0x1b73, "66ff7606", []int{}},
	{0x2579, 0x1b77, "666a00", []int{}},
	{0x2579, 0x1bda, "26c45f06", []int{}},
	{0x2579, 0x1be4, "26894706", []int{}},
	{0x2579, 0x1bfd, "26c45f0a", []int{}},
	{0x2579, 0x1c07, "2689471c", []int{}},
	{0x2579, 0x1c20, "26c45f0e", []int{}},
	{0x2579, 0x1c2a, "26894732", []int{}},
	{0x2579, 0x1dd7, "c45e0a", []int{}},
	{0x2579, 0x1dda, "26c45f06", []int{}},
	{0x2579, 0x1dde, "268a4710", []int{}},
	{0x2579, 0x1de2, "b400", []int{}},
	{0x2579, 0x1de4, "c45e0a", []int{}},
	{0x2579, 0x1de7, "26c45f0a", []int{}},
	{0x2579, 0x1deb, "268a5710", []int{}},
	{0x2579, 0x1def, "b600", []int{}},
	{0x2579, 0x1df1, "03c2", []int{}},
	{0x2579, 0x1df3, "c45e0a", []int{}},
	{0x2579, 0x1df6, "26c45f0e", []int{}},
	{0x2579, 0x1dfa, "268a5710", []int{}},
	{0x2579, 0x1dfe, "b600", []int{}},
	{0x2579, 0x1e00, "03c2", []int{}},
	{0x2579, 0x1e02, "8bd8", []int{}},
	{0x2579, 0x1e04, "d1e0", []int{}},
	{0x2579, 0x1e06, "03d8", []int{}},
	{0x2579, 0x1e08, "c1eb03", []int{}},
	{0x2579, 0x1e0b, "895ecc", []int{}},
	{0x2579, 0x1e0e, "66a1da51", []int{}},
	{0x2579, 0x1e12, "8b16d851", []int{}},
	{0x2579, 0x1e16, "0316ca51", []int{}},
	{0x2579, 0x1e1a, "8946ca", []int{}},
	{0x2579, 0x1e1d, "8956c8", []int{}},
	{0x2579, 0x1e20, "8b46cc", []int{}},
	{0x2579, 0x1e23, "fecc", []int{}},
	{0x2579, 0x1e25, "34ff", []int{}},
	{0x2579, 0x1e27, "22c4", []int{}},
	{0x2579, 0x1e29, "b400", []int{}},
	{0x2579, 0x1e2b, "c1e804", []int{}},
	{0x2579, 0x1e2e, "c1e008", []int{}},
	{0x2579, 0x1e31, "0146c8", []int{}},
	{0x2579, 0x1e8d, "9a3d085e1d", []int{3}},
	{0x2579, 0x1f1d, "68d859", []int{}},
	{0x2579, 0x1f20, "9af901e85a", []int{3}},
	{0x2579, 0x1f25, "83c40a", []int{}},
	{0x2579, 0x1f28, "52", []int{}},
	{0x2579, 0x1f29, "50", []int{}},
	{0x2579, 0x1f2a, "8b4606", []int{}},
	{0x2579, 0x1f2d, "050600", []int{}},
	{0x2579, 0x1f30, "ff7608", []int{}},
	{0x2579, 0x1f33, "50", []int{}},
	{0x2579, 0x1f34, "6a03", []int{}},
	{0x2579, 0x1f36, "ff361a3f", []int{}},
	{0x2579, 0x1f3a, "9aa80e5e1d", []int{3}},
	{0x2579, 0x1f3f, "83c412", []int{}},
	{0x2579, 0x1f42, "5d", []int{}},
	{0x2579, 0x1f43, "cb", []int{}},
	{0x2579, 0x1f44, "55", []int{}},
	{0x2579, 0x1f45, "8bec", []int{}},
	{0x6c4c, 0x33a5, "0200a820", []int{2}},
	{0x6c4c, 0x33a9, "701b7925", []int{2}},
	{0x6c4c, 0x33d1, "0200a820", []int{2}},
	{0x6c4c, 0x33d5, "64167925", []int{2}},
	{0x6c4c, 0x33dd, "33167925", []int{2}},
}

func Signatures() []Signature {
	out := append([]Signature(nil), signatures...)
	for i := range out {
		out[i].Reloc = append([]int(nil), out[i].Reloc...)
	}
	return out
}
func SignatureBytes(sig Signature, base uint16) []byte {
	b, _ := hex.DecodeString(sig.Hex)
	for _, i := range sig.Reloc {
		binary.LittleEndian.PutUint16(b[i:], binary.LittleEndian.Uint16(b[i:])+base)
	}
	return b
}
func SignatureSetDigest() [32]byte {
	var b strings.Builder
	for _, s := range signatures {
		fmt.Fprintf(&b, "%04x:%04x:%s:", s.Seg, s.Off, s.Hex)
		for i, v := range s.Reloc {
			if i > 0 {
				b.WriteByte(',')
			}
			fmt.Fprintf(&b, "%d", v)
		}
		b.WriteByte('\n')
	}
	return sha256.Sum256([]byte(b.String()))
}
func CheckProof(p *Proof, id Identity) Reason {
	if p == nil {
		return MissingProof
	}
	if p.Base != id.Base || p.ModelsGeneration != id.ModelsGeneration || p.ShadeEpoch != id.ShadeEpoch {
		return GenerationMismatch
	}
	reason := None
	prev := -1
	if len(p.Reads) != len(signatures) {
		reason = ProofUnavailable
	}
	for _, x := range p.Reads {
		i := int(x.SignatureIndex)
		if i <= prev || i >= len(signatures) {
			return ProofUnavailable
		}
		prev = i
		s := signatures[i]
		r := CheckRead(&x.Capture, Far{id.Base + s.Seg, s.Off}, uint32(len(s.Hex)/2))
		if r != None && r != ResourceLimit {
			r = ProofUnavailable
		}
		reason = MergeReason(reason, r)
		if r == None && string(readBytes(&x.Capture)) != string(SignatureBytes(s, id.Base)) {
			reason = MergeReason(reason, ProofMismatch)
		}
	}
	return reason
}
func MergeReason(a, b Reason) Reason {
	if statusFor(b) > statusFor(a) || statusFor(b) == statusFor(a) && b != None && (a == None || b < a) {
		return b
	}
	return a
}
