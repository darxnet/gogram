//go:build !purego

package gogram

import "unsafe"

func ConvertBytesToString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b)) //nolint: gosec // G103
}

func ConvertStringToBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s)) //nolint: gosec // G103
}
