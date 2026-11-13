package gogram_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/darxnet/gogram"
)

// A batch with an update whose message_id has an unexpected type: the
// update is skipped and reported, the rest is handled, the offset moves on.
const brokenBatch = `[
	{"update_id":10,"message":{"message_id":"oops","date":1,"chat":{"id":7,"type":"private"},"text":"bad"}},
	{"update_id":11,"message":{"message_id":2,"date":1,"chat":{"id":7,"type":"private"},"text":"good"}}
]`

func TestPoll_SkipsUndecodableUpdate(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		offsets []int64
		handled []string
		errs    []error
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	caller := gogram.CallerFunc(func(_ context.Context, req gogram.Request, result any) error {
		params, _ := req.Params.(*gogram.GetUpdatesParams)

		mu.Lock()
		offsets = append(offsets, params.Offset)
		n := len(offsets)
		mu.Unlock()

		switch n {
		case 1:
			return json.Unmarshal([]byte(brokenBatch), result)
		case 2:
			cancel() // the second poll proves the offset moved past both updates
		}

		return json.Unmarshal([]byte(`[]`), result)
	})

	router := gogram.NewRouter()
	router.HandleOnMessage(func(_ *gogram.Context, m *gogram.Message) error {
		mu.Lock()
		handled = append(handled, m.Text)
		mu.Unlock()

		return nil
	})
	router.SetHandlerErr(func(_ *gogram.Context, err error) {
		mu.Lock()
		errs = append(errs, err)
		mu.Unlock()
	})

	client, err := gogram.NewClient(testToken, gogram.WithCaller(caller), gogram.WithRouter(router), gogram.WithWorkers(1))
	if err != nil {
		t.Fatal(err)
	}

	if err = client.Start(ctx, nil); err != nil {
		t.Fatalf("Start = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(offsets) < 2 || offsets[0] != 0 || offsets[1] != 12 {
		t.Fatalf("offsets = %v, want [0 12 …]", offsets)
	}

	if len(handled) != 1 || handled[0] != "good" {
		t.Fatalf("handled = %v", handled)
	}

	var skipped *gogram.UndecodableUpdateError
	if len(errs) != 1 || !errors.As(errs[0], &skipped) || skipped.UpdateID != 10 || len(skipped.Raw) == 0 {
		t.Fatalf("errs = %v", errs)
	}

	var typeErr *json.UnmarshalTypeError
	if !errors.As(errs[0], &typeErr) {
		t.Fatalf("cause lost: %v", errs[0])
	}
}

func TestPoll_MalformedBatchIsAnError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var gotErr error

	router := gogram.NewRouter()
	router.SetHandlerErr(func(_ *gogram.Context, err error) {
		gotErr = err
		cancel()
	})

	client, err := gogram.NewClient(testToken,
		gogram.WithCaller(gogram.CallerFunc(func(_ context.Context, _ gogram.Request, result any) error {
			// Not an array at all: the batch fails as before.
			return json.Unmarshal([]byte(`{"update_id":1}`), result)
		})),
		gogram.WithRouter(router), gogram.WithWorkers(1))
	if err != nil {
		t.Fatal(err)
	}

	if err = client.Start(ctx, nil); err != nil {
		t.Fatalf("Start = %v", err)
	}

	var typeErr *json.UnmarshalTypeError
	if !errors.As(gotErr, &typeErr) {
		t.Fatalf("error = %v", gotErr)
	}
}
