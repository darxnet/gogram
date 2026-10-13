package gogram

import (
	"html"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// markdownV2Escaper escapes every character MarkdownV2 reserves.
// Reference: https://core.telegram.org/bots/api#markdownv2-style
var markdownV2Escaper = strings.NewReplacer(
	`\`, `\\`, `_`, `\_`, `*`, `\*`, `[`, `\[`, `]`, `\]`, `(`, `\(`, `)`, `\)`,
	`~`, `\~`, "`", "\\`", `>`, `\>`, `#`, `\#`, `+`, `\+`, `-`, `\-`, `=`, `\=`,
	`|`, `\|`, `{`, `\{`, `}`, `\}`, `.`, `\.`, `!`, `\!`,
)

// EscapeMarkdownV2 escapes s for use as literal text in a MarkdownV2 message.
func EscapeMarkdownV2(s string) string {
	return markdownV2Escaper.Replace(s)
}

// EscapeHTML escapes s for use as literal text in an HTML message.
func EscapeHTML(s string) string {
	return html.EscapeString(s)
}

// EntityText returns the part of text an entity refers to. Telegram counts
// entity offsets and lengths in UTF-16 code units, not bytes or runes, so a
// plain slice of text is wrong as soon as it contains an emoji.
func EntityText(text string, e *MessageEntity) string {
	start := utf16Index(text, int(e.Offset))
	end := start + utf16Index(text[start:], int(e.Length))

	return text[start:end]
}

// EntityText returns the message text the entity refers to.
func (m *Message) EntityText(e *MessageEntity) string {
	if m.Text != "" {
		return EntityText(m.Text, e)
	}

	return EntityText(m.Caption, e)
}

// utf16Index returns the byte index in s of the character at UTF-16 unit
// offset n, or len(s) when n is beyond the end.
func utf16Index(s string, n int) int {
	i := 0

	for n > 0 && i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])
		n -= utf16.RuneLen(r)
		i += size
	}

	return i
}

// UTF16Len returns the length of s in UTF-16 code units: the unit Telegram
// uses for entity offsets and for the 4096-character message limit.
func UTF16Len(s string) int {
	n := 0

	for _, r := range s {
		n += utf16.RuneLen(r)
	}

	return n
}
