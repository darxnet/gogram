package gogram

import "regexp"

// Constants for Telegram limits.
const (
	// CommandMaxLen is the maximum command length accepted by Telegram.
	CommandMaxLen = 32
	// StartParamMaxLen is the maximum deep-link start parameter length.
	StartParamMaxLen = 64

	// MessageMaxLen is the maximum text message length.
	MessageMaxLen = 4096
	// CaptionMaxLen is the maximum media caption length.
	CaptionMaxLen = 1024
	// MediaGroupMaxLen is the maximum number of media items in an album.
	MediaGroupMaxLen = 10
)

// StartParamRegexp is a regular expression for validating start parameters.
var StartParamRegexp = regexp.MustCompile("[A-Za-z0-9_-]{1,64}")
