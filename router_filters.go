package gogram

import (
	"regexp"
	"strings"
)

type Filter func(ctx *Context) bool

func FilterText(s string) Filter {
	return func(ctx *Context) bool {
		return ctx.Text() == s
	}
}

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

func FilterRegexp(pattern string) Filter {
	re := regexp.MustCompile(pattern)
	return func(ctx *Context) bool {
		return re.MatchString(ctx.Text())
	}
}

func FilterMessageDocument() Filter {
	return func(ctx *Context) bool {
		m := ctx.Message()
		return m != nil && m.Document != nil
	}
}

func FilterMessageVideo() Filter {
	return func(ctx *Context) bool {
		m := ctx.Message()
		return m != nil && m.Video != nil
	}
}

func FilterMessageContact() Filter {
	return func(ctx *Context) bool {
		m := ctx.Message()
		return m != nil && m.Contact != nil
	}
}
