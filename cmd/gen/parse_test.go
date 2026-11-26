package main

import (
	"strings"
	"testing"
)

func TestParseDescription_EmojiImg(t *testing.T) {
	t.Parallel()

	const page = `<p>Substrings “<img class="emoji" src="//telegram.org/img/emoji/40/E2AD90.png" ` +
		`alt="⭐" />” and “XTR”</p>`

	root := parseHTML(strings.NewReader(page))

	got := parseDescription(findNodes(root, "p")...)
	if want := "\n// Substrings “⭐” and “XTR”"; got != want {
		t.Fatalf("parseDescription = %q, want %q", got, want)
	}
}
