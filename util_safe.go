//go:build purego

package gogram

func ConvertBytesToString(b []byte) (s string) {
	return string(b)
}

func ConvertStringToBytes(s string) []byte {
	return []byte(s)
}
