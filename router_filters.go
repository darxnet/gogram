package gogram

import (
	"regexp"
	"strings"
)

// Filter is a function that checks if the update matches the condition.
type Filter func(ctx *Context) bool

// FilterAny creates a filter that always matches.
func FilterAny() Filter {
	return func(*Context) bool {
		return true
	}
}

// FilterAnd creates a filter that matches only when all provided filters match.
func FilterAnd(filters ...Filter) Filter {
	return func(ctx *Context) bool {
		for _, f := range filters {
			if !f(ctx) {
				return false
			}
		}
		return true
	}
}

// FilterOr creates a filter that matches when at least one of the provided filters matches.
func FilterOr(filters ...Filter) Filter {
	return func(ctx *Context) bool {
		for _, f := range filters {
			if f(ctx) {
				return true
			}
		}
		return false
	}
}

// FilterNot creates a filter that inverts the result of the provided filter.
func FilterNot(f Filter) Filter {
	return func(ctx *Context) bool {
		return !f(ctx)
	}
}

// FilterText creates a filter that matches the message text exactly.
func FilterText(s string) Filter {
	return func(ctx *Context) bool {
		return ctx.Text() == s
	}
}

// FilterPrefix creates a filter that matches when the message text starts with the given prefix.
func FilterPrefix(prefix string) Filter {
	return func(ctx *Context) bool {
		return strings.HasPrefix(ctx.Text(), prefix)
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

		// full match
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

// FilterFromUser creates a filter that matches updates sent by the user with the given ID.
func FilterFromUser(id int64) Filter {
	return func(ctx *Context) bool {
		u := ctx.User()
		return u != nil && u.ID == id
	}
}

// FilterChat creates a filter that matches updates originating from the chat with the given ID.
func FilterChat(id int64) Filter {
	return func(ctx *Context) bool {
		c := ctx.Chat()
		return c != nil && c.ID == id
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
