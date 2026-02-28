package gogram

import (
	"regexp"
	"strings"
)

// Filter is a function that checks if the update matches the condition.
type Filter func(ctx *Context) bool

// FilterText creates a filter that matches the message text exactly.
func FilterText(s string) Filter {
	return func(ctx *Context) bool {
		return ctx.Text() == s
	}
}

// FilterCommand creates a filter that matches a command.
// It supports commands with arguments and bot mentions (e.g., /command, /command@bot, /command arg).
func FilterCommand(s string) Filter {
	return func(ctx *Context) bool {
		if s == "" {
			return false
		}

		text := ctx.Text()
		if text == "" || text[0] != '/' {
			return false
		}

		// full path
		if text == s {
			return true
		}

		// /command@bot
		if before, _, found := strings.Cut(text, "@"); found && before == s {
			return true
		}

		// /command payload
		if before, _, found := strings.Cut(text, " "); found && before == s {
			return true
		}

		return false
	}
}

// FilterRegexp creates a filter that matches the message text against a regular expression.
// Note: It panics if the pattern is invalid.
func FilterRegexp(pattern string) Filter {
	re := regexp.MustCompile(pattern)
	return func(ctx *Context) bool {
		return re.MatchString(ctx.Text())
	}
}

// FilterMessageDocument creates a filter that matches messages with a document.
func FilterMessageDocument() Filter {
	return func(ctx *Context) bool {
		m := ctx.Message()
		return m != nil && m.Document != nil
	}
}

// FilterMessageVideo creates a filter that matches messages with a video.
func FilterMessageVideo() Filter {
	return func(ctx *Context) bool {
		m := ctx.Message()
		return m != nil && m.Video != nil
	}
}

// FilterMessageContact creates a filter that matches messages with a contact.
func FilterMessageContact() Filter {
	return func(ctx *Context) bool {
		m := ctx.Message()
		return m != nil && m.Contact != nil
	}
}
