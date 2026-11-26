package gogram_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/darxnet/gogram"
)

func TestInlineKeyboardBuilder(t *testing.T) {
	t.Parallel()

	rows := gogram.NewInlineKeyboardBuilder().
		Callback("Yes", "c 1").Callback("No", "c 0").Row().
		Row(). // empty row is skipped
		URL("Docs", "https://example.com").
		WebApp("App", "https://app.example.com").
		SwitchInlineCurrent("Share", "q").
		Rows()

	if len(rows) != 2 || len(rows[0]) != 2 || len(rows[1]) != 3 {
		t.Fatalf("rows = %+v", rows)
	}

	if rows[0][1].CallbackData != "c 0" || rows[1][0].URL == "" || rows[1][1].WebApp == nil ||
		rows[1][2].SwitchInlineQueryCurrentChat != gogram.Some("q") {
		t.Fatalf("rows = %+v", rows)
	}

	var zero gogram.InlineKeyboardBuilder
	if m := zero.Callback("a", "b").ReplyMarkup(); m.InlineKeyboardMarkup == nil || len(m.InlineKeyboard) != 1 {
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

func TestInlineKeyboardBuilder_AddColumnRowsKeyboards(t *testing.T) {
	t.Parallel()

	a := gogram.InlineKeyboardButton{Text: "a", CallbackData: "a"}
	b := gogram.InlineKeyboardButton{Text: "b", CallbackData: "b"}
	c := gogram.InlineKeyboardButton{Text: "c", CallbackData: "c"}

	rows := gogram.NewInlineKeyboardBuilder().
		Add(a). // current row is finished by AddColumn
		AddColumn(b, c).
		AddRows(gogram.InlineKeyboardRow{a, b}, nil, gogram.InlineKeyboardRow{c}).
		Add(c).
		AddKeyboards(gogram.InlineKeyboard{{a}}, nil, gogram.InlineKeyboard{{}, {b, c}}).
		Rows()

	want := []string{"a", "b", "c", "ab", "c", "c", "a", "bc"}
	if len(rows) != len(want) {
		t.Fatalf("len(rows) = %d, want %d: %+v", len(rows), len(want), rows)
	}

	for i, row := range rows {
		var got strings.Builder
		for _, btn := range row {
			got.WriteString(btn.CallbackData)
		}

		if got.String() != want[i] {
			t.Fatalf("row %d = %q, want %q", i, got.String(), want[i])
		}
	}

	var empty gogram.InlineKeyboardBuilder
	if got := empty.AddColumn().AddRows().AddKeyboards().Rows(); len(got) != 0 {
		t.Fatalf("empty builder rows = %+v", got)
	}
}

func TestInlineKeyboardBuilder_ButtonKinds(t *testing.T) {
	t.Parallel()

	rows := gogram.NewInlineKeyboardBuilder().
		Pay("Pay 100 ⭐").Row().
		Game("Play").
		SwitchInlineChat("Share", "q").
		CopyText("Copy", "code").
		Disabled("1/5").
		Rows()

	if len(rows) != 2 || len(rows[1]) != 4 {
		t.Fatalf("rows = %+v", rows)
	}

	if !rows[0][0].Pay || rows[0][0].Text != "Pay 100 ⭐" {
		t.Fatalf("pay = %+v", rows[0][0])
	}

	game, chat, copyText, disabled := rows[1][0], rows[1][1], rows[1][2], rows[1][3]
	if game.CallbackGame == nil || chat.SwitchInlineQuery != gogram.Some("q") ||
		copyText.CopyText == nil || copyText.CopyText.Text != "code" || disabled.Disabled == nil {
		t.Fatalf("rows[1] = %+v", rows[1])
	}
}

func TestInlineKeyboardBuilder_StyleIcon(t *testing.T) {
	t.Parallel()

	kb := gogram.InlineKeyboard{{{Text: "a", CallbackData: "a"}}}

	rows := gogram.NewInlineKeyboardBuilder().
		Callback("x", "x").Style(gogram.ButtonStyleDanger).Icon("123").
		Callback("y", "y").Row().Style(gogram.ButtonStyleSuccess). // after Row: last button of the last row
		AddKeyboards(kb).Style(gogram.ButtonStylePrimary).
		Rows()

	if x := rows[0][0]; x.Style != gogram.ButtonStyleDanger || x.IconCustomEmojiID != "123" {
		t.Fatalf("x = %+v", x)
	}

	if y := rows[0][1]; y.Style != gogram.ButtonStyleSuccess || y.IconCustomEmojiID != "" {
		t.Fatalf("y = %+v", y)
	}

	if rows[1][0].Style != gogram.ButtonStylePrimary {
		t.Fatalf("added row = %+v", rows[1][0])
	}

	if kb[0][0].Style != "" {
		t.Fatalf("caller's keyboard modified: %+v", kb[0][0])
	}
}

func TestInlineKeyboardBuilder_StyleWithoutButtonPanics(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("Style on an empty builder must panic")
		}
	}()

	gogram.NewInlineKeyboardBuilder().Style(gogram.ButtonStyleDanger)
}

func TestInlineKeyboardBuilder_EmptySwitchInlineQuery(t *testing.T) {
	t.Parallel()

	// An empty query is valid and must still be sent: it inserts only the
	// bot's username. Without it the button would be a bare text button.
	rows := gogram.NewInlineKeyboardBuilder().
		SwitchInlineCurrent("Search", "").
		SwitchInlineChat("Share", "").
		Rows()

	b, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}

	const want = `[[{"text":"Search","switch_inline_query_current_chat":""},{"text":"Share","switch_inline_query":""}]]`
	if string(b) != want {
		t.Fatalf("json = %s, want %s", b, want)
	}
}

func TestInlineKeyboardBuilder_LoginURLChosenChat(t *testing.T) {
	t.Parallel()

	rows := gogram.NewInlineKeyboardBuilder().
		LoginURL("Log in", gogram.LoginUrl{URL: "https://example.com/auth", RequestWriteAccess: true}).
		SwitchInlineChosenChat("Share", gogram.SwitchInlineQueryChosenChat{Query: "q", AllowGroupChats: true}).
		Rows()

	login, chosen := rows[0][0], rows[0][1]
	if login.LoginUrl == nil || login.LoginUrl.URL != "https://example.com/auth" || !login.LoginUrl.RequestWriteAccess {
		t.Fatalf("login = %+v", login)
	}

	if chosen.SwitchInlineQueryChosenChat == nil || chosen.SwitchInlineQueryChosenChat.Query != "q" ||
		!chosen.SwitchInlineQueryChosenChat.AllowGroupChats {
		t.Fatalf("chosen = %+v", chosen)
	}
}
