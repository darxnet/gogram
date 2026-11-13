package gogram_test

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/darxnet/gogram"
)

func TestOptionalJSON(t *testing.T) {
	t.Parallel()

	type params struct {
		A gogram.Optional[bool]  `json:"a,omitzero"`
		N gogram.Optional[int64] `json:"n,omitzero"`
	}

	for _, tc := range []struct {
		in   params
		want string
	}{
		{params{}, `{}`},
		{params{A: gogram.Some(false), N: gogram.Some[int64](0)}, `{"a":false,"n":0}`},
		{params{A: gogram.Some(true), N: gogram.Some[int64](300)}, `{"a":true,"n":300}`},
	} {
		got, err := json.Marshal(tc.in)
		if err != nil || string(got) != tc.want {
			t.Errorf("Marshal(%+v) = %s, %v; want %s", tc.in, got, err, tc.want)
		}

		var back params
		if err := json.Unmarshal(got, &back); err != nil || back != tc.in {
			t.Errorf("Unmarshal(%s) = %+v, %v; want %+v", got, back, err, tc.in)
		}
	}

	var p params
	if err := json.Unmarshal([]byte(`{"a":null,"n":"x"}`), &p); err == nil {
		t.Errorf("Unmarshal of a string into Optional[int64] succeeded: %+v", p)
	}

	if p.A.Set {
		t.Error("null set the Optional")
	}

	if b, err := (gogram.Optional[bool]{}).MarshalJSON(); err != nil || string(b) != "null" {
		t.Errorf("unset MarshalJSON = %s, %v; want null", b, err)
	}
}

func TestOptionalOr(t *testing.T) {
	t.Parallel()

	if got := (gogram.Optional[bool]{}).Or(true); !got {
		t.Error("unset Or(true) = false")
	}

	if got := gogram.Some(false).Or(true); got {
		t.Error("Some(false).Or(true) = true")
	}
}

func TestOptionalFormat(t *testing.T) {
	t.Parallel()

	perms := gogram.ChatPermissions{CanSendMessages: true, CanManageTopics: gogram.Some(false)}

	for _, tc := range []struct {
		format string
		arg    any
		want   string
	}{
		{"%v", gogram.Some(false), "false"},
		{"%v", gogram.Optional[bool]{}, "<unset>"},
		{"%5d", gogram.Some[int64](42), "   42"},
		{"%q", gogram.Some("x"), `"x"`},
		{"%#v", gogram.Some(false), "gogram.Some(false)"},
		{"%#v", gogram.Optional[int64]{}, "gogram.Optional[int64]{}"},
		{"%+v", struct{ A, B gogram.Optional[bool] }{A: gogram.Some(true)}, "{A:true B:<unset>}"},
	} {
		if got := fmt.Sprintf(tc.format, tc.arg); got != tc.want {
			t.Errorf("Sprintf(%q) = %q, want %q", tc.format, got, tc.want)
		}
	}

	got := fmt.Sprintf("%+v", perms)
	if !strings.Contains(got, "CanManageTopics:false") || !strings.Contains(got, "CanEditTag:<unset>") {
		t.Errorf("ChatPermissions prints as %s", got)
	}
}

// requestFields returns a client whose requests record their fields, from a
// JSON or a multipart body, into fields.
func requestFields(t *testing.T, fields map[string]string) *gogram.Client {
	t.Helper()

	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		clear(fields)

		mediaType, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
		if err != nil {
			return nil, err
		}

		if mediaType == "multipart/form-data" {
			form, err := multipart.NewReader(req.Body, params["boundary"]).ReadForm(1 << 20)
			if err != nil {
				return nil, err
			}

			for name, values := range form.Value {
				fields[name] = strings.TrimSpace(values[0])
			}
		} else {
			body, _ := io.ReadAll(req.Body)

			var raw map[string]json.RawMessage
			if err := json.Unmarshal(body, &raw); err != nil {
				return nil, err
			}

			for name, value := range raw {
				fields[name] = string(value)
			}
		}

		result := []byte(`true`)
		if strings.HasSuffix(req.URL.Path, "/sendPoll") {
			result = []byte(`{"message_id":1,"date":1,"chat":{"id":7,"type":"private"}}`)
		}

		resp := testResponse{OK: true, Result: result}

		return jsonHTTPResponse(t, &resp), nil
	})

	client, err := gogram.NewClient(testToken, gogram.WithHTTPClient(&http.Client{Transport: transport}))
	if err != nil {
		t.Fatal(err)
	}

	return client
}

// TestOptionalSent checks that an Optional field set to its zero value is
// sent, and an unset one is not, so that Telegram's default applies.
func TestOptionalSent(t *testing.T) {
	t.Parallel()

	fields := map[string]string{}
	client := requestFields(t, fields)
	chat := gogram.ChatID{ID: 7}

	poll := func(opts ...gogram.SendPollOption) {
		t.Helper()

		params := &gogram.SendPollParams{ChatID: chat, Question: "q",
			Options: []gogram.InputPollOption{{Text: "a"}, {Text: "b"}}}
		params.Option(opts...)

		if _, err := client.SendPoll(t.Context(), params); err != nil {
			t.Fatal(err)
		}
	}

	poll(gogram.WithSendPollIsAnonymous(false), gogram.WithSendPollAllowsRevoting(false))

	if fields["is_anonymous"] != "false" || fields["allows_revoting"] != "false" {
		t.Errorf("explicit false: fields = %v, want is_anonymous and allows_revoting false", fields)
	}

	poll()

	if _, ok := fields["is_anonymous"]; ok {
		t.Errorf("unset is_anonymous was sent: %v", fields)
	}

	if _, err := client.AnswerInlineQuery(t.Context(), &gogram.AnswerInlineQueryParams{
		InlineQueryID: "q", CacheTime: gogram.Some[int64](0),
	}); err != nil {
		t.Fatal(err)
	}

	if fields["cache_time"] != "0" {
		t.Errorf("cache_time 0: fields = %v", fields)
	}

	if _, err := client.SetChatPermissions(t.Context(), &gogram.SetChatPermissionsParams{ChatID: chat,
		Permissions: gogram.ChatPermissions{CanPinMessages: true, CanManageTopics: gogram.Some(false)},
	}); err != nil {
		t.Fatal(err)
	}

	if want := `{"can_pin_messages":true,"can_manage_topics":false}`; fields["permissions"] != want {
		t.Errorf("permissions = %s, want %s", fields["permissions"], want)
	}
}
