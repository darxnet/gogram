package gogram

import "strings"

// Identifier is an interface for types that have a string identifier.
type Identifier interface {
	Identifier() string
}

// ExtractArgs splits the text by spaces and returns the arguments (excluding the command).
func ExtractArgs(text string) []string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return nil
	}

	return fields[1:]
}

// ExtractPayload returns the substring after the first space.
func ExtractPayload(text string) string {
	_, after, _ := strings.Cut(text, " ")
	return after
}

// PhotoBiggest returns the largest photo from a list of PhotoSize.
func PhotoBiggest(list []PhotoSize) PhotoSize {
	var v PhotoSize

	for _, photo := range list {
		if photo.Width > v.Width {
			v = photo
		}
	}

	return v
}
