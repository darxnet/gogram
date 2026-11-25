package gogram_test

import (
	"testing"

	"github.com/darxnet/gogram"
)

func TestInlineKeyboardBuilder(t *testing.T) {
	t.Parallel()

	rows := gogram.NewInlineKeyboardBuilder().
		Text("Yes", "c 1").Text("No", "c 0").Row().
		Row(). // empty row is skipped
		URL("Docs", "https://example.com").
		WebApp("App", "https://app.example.com").
		SwitchInline("Share", "q").
		Rows()

	if len(rows) != 2 || len(rows[0]) != 2 || len(rows[1]) != 3 {
		t.Fatalf("rows = %+v", rows)
	}

	if rows[0][1].CallbackData != "c 0" || rows[1][0].URL == "" || rows[1][1].WebApp == nil ||
		rows[1][2].SwitchInlineQueryCurrentChat != "q" {
		t.Fatalf("rows = %+v", rows)
	}

	var zero gogram.InlineKeyboardBuilder
	if m := zero.Text("a", "b").ReplyMarkup(); m.InlineKeyboardMarkup == nil || len(m.InlineKeyboard) != 1 {
		t.Fatalf("zero value builder: %+v", m)
	}
}

func TestInlineKeyboardAliases(t *testing.T) {
	t.Parallel()

	kb := gogram.InlineKeyboard{
		gogram.InlineKeyboardRow{{Text: "a", CallbackData: "a"}},
		{{Text: "b", CallbackData: "b"}, {Text: "c", CallbackData: "c"}},
	}

	// The aliases are the field type itself, so no conversion is needed either way.
	markup := gogram.InlineKeyboardMarkup{InlineKeyboard: kb}

	if len(markup.InlineKeyboard) != 2 || len(markup.InlineKeyboard[1]) != 2 {
		t.Fatalf("markup = %+v", markup)
	}
}
