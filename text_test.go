package gogram_test

import (
	"testing"

	"github.com/darxnet/gogram"
)

func TestEscape(t *testing.T) {
	t.Parallel()

	if got := gogram.EscapeMarkdownV2(`a_b*c[d]e(f)g~h` + "`" + `i>j#k+l-m=n|o{p}q.r!s\t`); got !=
		`a\_b\*c\[d\]e\(f\)g\~h\`+"`"+`i\>j\#k\+l\-m\=n\|o\{p\}q\.r\!s\\t` {
		t.Fatalf("EscapeMarkdownV2 = %q", got)
	}

	if got := gogram.EscapeHTML(`<b>&"'`); got != `&lt;b&gt;&amp;&#34;&#39;` {
		t.Fatalf("EscapeHTML = %q", got)
	}
}

func TestEntityText_UTF16(t *testing.T) {
	t.Parallel()

	// "😀" is one rune but two UTF-16 units; "é" is one unit, two bytes.
	text := "😀 /start é @user"

	if n := gogram.UTF16Len(text); n != 17 {
		t.Fatalf("UTF16Len = %d, want 17", n)
	}

	m := &gogram.Message{Text: text}

	cmd := &gogram.MessageEntity{Type: gogram.MessageEntityBotCommand, Offset: 3, Length: 6}
	if got := m.EntityText(cmd); got != "/start" {
		t.Fatalf("command = %q", got)
	}

	mention := &gogram.MessageEntity{Type: gogram.MessageEntityMention, Offset: 12, Length: 5}
	if got := m.EntityText(mention); got != "@user" {
		t.Fatalf("mention = %q", got)
	}

	// Out-of-range entities clamp instead of panicking.
	if got := gogram.EntityText(text, &gogram.MessageEntity{Offset: 100, Length: 5}); got != "" {
		t.Fatalf("out of range = %q", got)
	}
}

func TestStartLink(t *testing.T) {
	t.Parallel()

	client, err := gogram.NewClient(testToken,
		gogram.WithCaller(cannedCaller(`{"id":1,"is_bot":true,"first_name":"b","username":"my_bot"}`)))
	if err != nil {
		t.Fatal(err)
	}

	for param, want := range map[string]string{
		"":       "https://t.me/my_bot",
		"ref-1_": "https://t.me/my_bot?start=ref-1_",
	} {
		got, linkErr := client.StartLink(t.Context(), param)
		if linkErr != nil || got != want {
			t.Fatalf("StartLink(%q) = %q, %v; want %q", param, got, linkErr, want)
		}
	}

	got, err := client.StartGroupLink(t.Context(), "x")
	if err != nil || got != "https://t.me/my_bot?startgroup=x" {
		t.Fatalf("StartGroupLink = %q, %v", got, err)
	}

	for _, bad := range []string{"has space", "ünï", string(make([]byte, 65))} {
		if _, err = client.StartLink(t.Context(), bad); err == nil {
			t.Fatalf("StartLink(%q) must fail", bad)
		}
	}
}
