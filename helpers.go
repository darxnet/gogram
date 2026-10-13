package gogram

import "strings"

// extractArgs splits the text by spaces and returns the arguments (excluding the command).
func extractArgs(text string) []string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return nil
	}

	return fields[1:]
}

// extractPayload returns the substring after the first space.
func extractPayload(text string) string {
	_, after, _ := strings.Cut(text, " ")
	return after
}

// LargestPhoto returns the PhotoSize with the largest area, or nil for an
// empty list. Telegram sends sizes in ascending order, but the order is not
// documented, so the area is compared instead of taking the last element.
func LargestPhoto(list []PhotoSize) *PhotoSize {
	var (
		v    *PhotoSize
		area int64
	)

	for i := range list {
		if a := list[i].Width * list[i].Height; v == nil || a > area {
			v, area = &list[i], a
		}
	}

	return v
}
