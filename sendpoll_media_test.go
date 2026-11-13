package gogram_test

import (
	"net/http"
	"testing"

	"github.com/darxnet/gogram"
)

// TestSendPoll_WithoutMedia checks that a poll without media can be sent:
// the multipart writer must not reach into the unset media fields.
func TestSendPoll_WithoutMedia(t *testing.T) {
	t.Parallel()

	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		//nolint:gosec // a test transport parsing the client's own request
		if err := req.ParseMultipartForm(1 << 20); err != nil {
			return nil, err
		}

		if got := req.FormValue("question"); got != "q" {
			t.Errorf("question = %q, want q", got)
		}

		resp := testResponse{OK: true, Result: []byte(`{"message_id":1,"date":1,"chat":{"id":7,"type":"private"}}`)}

		return jsonHTTPResponse(t, &resp), nil
	})

	client, err := gogram.NewClient(testToken, gogram.WithHTTPClient(&http.Client{Transport: transport}))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := client.SendPoll(t.Context(), &gogram.SendPollParams{
		ChatID: gogram.ChatID{ID: 7}, Question: "q",
		Options: []gogram.InputPollOption{{Text: "a"}, {Text: "b"}},
	})
	if err != nil || msg == nil || msg.MessageID != 1 {
		t.Fatalf("SendPoll = %+v, %v", msg, err)
	}
}
