package gogram_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/darxnet/gogram"
)

var errDenied = errors.New("denied")

// cannedCaller answers every call with body decoded into result.
func cannedCaller(body string) gogram.Caller {
	return gogram.CallerFunc(func(_ context.Context, _ gogram.Request, result any) error {
		return json.Unmarshal([]byte(body), result)
	})
}

func TestWithCaller_ReplacesTransport(t *testing.T) {
	t.Parallel()

	var seen gogram.Request

	caller := gogram.CallerFunc(func(_ context.Context, req gogram.Request, result any) error {
		seen = req
		return json.Unmarshal([]byte(`{"message_id":9,"date":1,"chat":{"id":7,"type":"private"},"text":"hi"}`), result)
	})

	client, err := gogram.NewClient(testToken, gogram.WithCaller(caller))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	msg, err := client.SendMessage(t.Context(), &gogram.SendMessageParams{ChatID: "7", Text: "hi"})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	if msg == nil || msg.MessageID != 9 {
		t.Fatalf("result = %+v, want message 9", msg)
	}

	if seen.Method != "sendMessage" || seen.ContentType != "application/json" {
		t.Fatalf("request = %+v", seen)
	}

	params, ok := seen.Params.(*gogram.SendMessageParams)
	if !ok || params.Text != "hi" {
		t.Fatalf("Params = %T %+v, want *SendMessageParams{Text: hi}", seen.Params, seen.Params)
	}

	// Bool results decode into the typed pointer too.
	ok, err = client.DeleteMessage(t.Context(), &gogram.DeleteMessageParams{ChatID: "7", MessageID: 9})
	if err == nil && ok {
		t.Fatal("canned message body must not decode as true")
	}
}

func TestWithTransformers_Order(t *testing.T) {
	t.Parallel()

	var trace []string

	tag := func(name string) gogram.Transformer {
		return func(next gogram.Caller) gogram.Caller {
			return gogram.CallerFunc(func(ctx context.Context, req gogram.Request, result any) error {
				trace = append(trace, "before "+name)
				err := next.Call(ctx, req, result)
				trace = append(trace, "after "+name)
				return err
			})
		}
	}

	client, err := gogram.NewClient(testToken,
		gogram.WithCaller(cannedCaller(`true`)),
		gogram.WithTransformers(tag("outer"), tag("inner")),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ok, err := client.DeleteMessage(t.Context(), &gogram.DeleteMessageParams{ChatID: "7", MessageID: 1})
	if err != nil || !ok {
		t.Fatalf("DeleteMessage = %v, %v", ok, err)
	}

	want := []string{"before outer", "before inner", "after inner", "after outer"}
	if !slices.Equal(trace, want) {
		t.Fatalf("trace = %v, want %v", trace, want)
	}
}

func TestTransformer_ShortCircuit(t *testing.T) {
	t.Parallel()

	deny := func(gogram.Caller) gogram.Caller {
		return gogram.CallerFunc(func(context.Context, gogram.Request, any) error { return errDenied })
	}

	client, err := gogram.NewClient(testToken, gogram.WithCaller(cannedCaller(`true`)), gogram.WithTransformers(deny))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if _, err = client.GetMe(t.Context(), nil); !errors.Is(err, errDenied) {
		t.Fatalf("err = %v, want errDenied", err)
	}
}

// TestEnvelope_NullResult pins the encoding/json behaviour the transport
// relies on: a pointer stored in an interface field is decoded into, and
// a JSON null leaves the typed destination untouched.
func TestEnvelope_NullResult(t *testing.T) {
	t.Parallel()

	client, err := gogram.NewClient(testToken, gogram.WithHTTPClient(nil), gogram.WithCaller(cannedCaller(`null`)))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	msg, err := client.GetMe(t.Context(), nil)
	if err != nil || msg != nil {
		t.Fatalf("GetMe = %+v, %v; want nil, nil", msg, err)
	}
}
