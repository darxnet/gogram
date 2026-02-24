package gogram

import "regexp"

const (
	CommandMaxLen    = 32
	StartParamMaxLen = 64

	MessageMaxLen    = 4096
	CaptionMaxLen    = 1024
	MediaGroupMaxLen = 10
)

var StartParamRegexp = regexp.MustCompile("[A-Za-z0-9_-]{1,64}")
