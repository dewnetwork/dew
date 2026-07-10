package rpc

import "testing"

// FuzzDecodeBytes ensures hex decoding never panics on arbitrary strings.
func FuzzDecodeBytes(f *testing.F) {
	f.Add("0x")
	f.Add("0x00")
	f.Add("0xdeadbeef")
	f.Add("not-hex")
	f.Add("0xZZ")
	f.Add("")
	f.Add("0x0")
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = DecodeBytes(s)
		_, _ = DecodeUint64(s)
		_, _ = DecodeBig(s)
		_, _ = DecodeAddress(s)
		_, _ = DecodeHash(s)
	})
}
