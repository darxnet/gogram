package gogram

import "strings"

type Identifier interface {
	Identifier() string
}

func ExtractArgs(text string) []string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return nil
	}

	return fields[1:]
}

func ExtractPayload(text string) string {
	_, after, _ := strings.Cut(text, " ")
	return after
}

func PhotoBiggest(list []PhotoSize) PhotoSize {
	var v PhotoSize

	for _, photo := range list {
		if photo.Width > v.Width {
			v = photo
		}
	}

	return v
}
