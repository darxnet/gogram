package gogram

import (
	"regexp"
	"slices"
	"strings"
)

// Filter reports whether an update matches a condition. Filters compose:
//
//	gogram.FilterPrivate().And(gogram.FilterCommand("/start").Not())
type Filter func(ctx *Context) bool

// And returns a filter that matches when f and every one of others match.
func (f Filter) And(others ...Filter) Filter {
	return func(ctx *Context) bool {
		if !f(ctx) {
			return false
		}

		for _, o := range others {
			if !o(ctx) {
				return false
			}
		}

		return true
	}
}

// Or returns a filter that matches when f or any one of others matches.
func (f Filter) Or(others ...Filter) Filter {
	return func(ctx *Context) bool {
		if f(ctx) {
			return true
		}

		for _, o := range others {
			if o(ctx) {
				return true
			}
		}

		return false
	}
}

// Not returns a filter that matches when f does not.
func (f Filter) Not() Filter {
	return func(ctx *Context) bool {
		return !f(ctx)
	}
}

// FilterAny creates a filter that always matches.
func FilterAny() Filter {
	return func(*Context) bool {
		return true
	}
}

// FilterAnd creates a filter that matches only when all provided filters match.
func FilterAnd(filters ...Filter) Filter {
	return FilterAny().And(filters...)
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
	return f.Not()
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
	if s != "" && s[0] != '/' {
		s = "/" + s
	}

	return func(ctx *Context) bool {
		return s != "" && retrieveCommand(ctx.Text()) == s
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

// FilterUsers creates a filter that matches updates sent by any of the given users.
func FilterUsers(ids ...int64) Filter {
	ids = slices.Clone(ids)

	return func(ctx *Context) bool {
		u := ctx.User()
		return u != nil && slices.Contains(ids, u.ID)
	}
}

// FilterChat creates a filter that matches updates originating from the chat with the given ID.
func FilterChat(id int64) Filter {
	return func(ctx *Context) bool {
		c := ctx.Chat()
		return c != nil && c.ID == id
	}
}

// FilterChatType creates a filter that matches updates from chats of the given types,
// see ChatPrivate, ChatGroup, ChatSupergroup and ChatChannel.
func FilterChatType(types ...string) Filter {
	types = slices.Clone(types)

	return func(ctx *Context) bool {
		c := ctx.Chat()
		return c != nil && slices.Contains(types, c.Type)
	}
}

// FilterPrivate matches updates from private chats.
func FilterPrivate() Filter {
	return FilterChatType(ChatPrivate)
}

// FilterGroup matches updates from groups and supergroups.
func FilterGroup() Filter {
	return FilterChatType(ChatGroup, ChatSupergroup)
}

// FilterChannel matches updates from channels.
func FilterChannel() Filter {
	return FilterChatType(ChatChannel)
}

// FilterMessage creates a filter from a predicate on the current message;
// it does not match when there is no message.
func FilterMessage(pred func(m *Message) bool) Filter {
	return func(ctx *Context) bool {
		m := ctx.Message()
		return m != nil && pred(m)
	}
}

// FilterMessagePhoto matches messages with a photo.
func FilterMessagePhoto() Filter {
	return FilterMessage(func(m *Message) bool { return len(m.Photo) != 0 })
}

// FilterMessageDocument creates a filter that matches messages with a document.
func FilterMessageDocument() Filter {
	return FilterMessage(func(m *Message) bool { return m.Document != nil })
}

// FilterMessageVideo creates a filter that matches messages with a video.
func FilterMessageVideo() Filter {
	return FilterMessage(func(m *Message) bool { return m.Video != nil })
}

// FilterMessageAudio matches messages with an audio file.
func FilterMessageAudio() Filter {
	return FilterMessage(func(m *Message) bool { return m.Audio != nil })
}

// FilterMessageVoice matches messages with a voice note.
func FilterMessageVoice() Filter {
	return FilterMessage(func(m *Message) bool { return m.Voice != nil })
}

// FilterMessageSticker matches messages with a sticker.
func FilterMessageSticker() Filter {
	return FilterMessage(func(m *Message) bool { return m.Sticker != nil })
}

// FilterMessageContact creates a filter that matches messages with a contact.
func FilterMessageContact() Filter {
	return FilterMessage(func(m *Message) bool { return m.Contact != nil })
}

// FilterMessageLocation matches messages with a location.
func FilterMessageLocation() Filter {
	return FilterMessage(func(m *Message) bool { return m.Location != nil })
}

// FilterMessageMedia matches messages carrying any media: photo, video,
// animation, audio, voice, video note, document or sticker.
func FilterMessageMedia() Filter {
	return FilterMessage(func(m *Message) bool {
		return len(m.Photo) != 0 || m.Video != nil || m.Animation != nil || m.Audio != nil ||
			m.Voice != nil || m.VideoNote != nil || m.Document != nil || m.Sticker != nil
	})
}

// FilterMessageForwarded matches forwarded messages.
func FilterMessageForwarded() Filter {
	return FilterMessage(func(m *Message) bool { return m.ForwardOrigin != nil })
}

// FilterMessageReply matches messages that reply to another message.
func FilterMessageReply() Filter {
	return FilterMessage(func(m *Message) bool { return m.ReplyToMessage != nil })
}

// FilterCallbackPrefix matches callback queries whose data equals prefix or
// starts with prefix followed by a space. See [Callback.Filter] for typed data.
func FilterCallbackPrefix(prefix string) Filter {
	return func(ctx *Context) bool {
		cq := ctx.Update().CallbackQuery
		if cq == nil {
			return false
		}

		return cq.Data == prefix ||
			len(cq.Data) > len(prefix) && cq.Data[len(prefix)] == ' ' && cq.Data[:len(prefix)] == prefix
	}
}
