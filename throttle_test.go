package gogram_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/darxnet/gogram"
	"golang.org/x/time/rate"
)

func TestThrottle_PerChat(t *testing.T) {
	t.Parallel()

	var (
		mu    sync.Mutex
		times = map[string][]time.Time{}
	)

	record := gogram.CallerFunc(func(_ context.Context, req gogram.Request, _ any) error {
		chat := ""
		if ct, ok := req.Params.(gogram.ChatTargeter); ok {
			chat = ct.ChatTarget()
		}

		mu.Lock()
		times[req.Method+" "+chat] = append(times[req.Method+" "+chat], time.Now())
		mu.Unlock()

		return nil
	})

	const spacing = 50 * time.Millisecond

	client, err := gogram.NewClient(testToken,
		gogram.WithCaller(record),
		gogram.WithTransformers(gogram.Throttle(gogram.ThrottleConfig{
			PrivateRate: rate.Every(spacing),
			GroupRate:   rate.Every(spacing),
			GroupBurst:  1,
		})),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx := t.Context()

	var wg sync.WaitGroup

	// Three sends to one private chat are spaced; a group and another
	// private chat are independent; sendChatAction and getMe are free.
	run := func(f func()) {
		wg.Add(1)

		go func() {
			defer wg.Done()
			f()
		}()
	}

	for range 3 {
		run(func() { _, _ = client.SendMessage(ctx, &gogram.SendMessageParams{ChatID: "7", Text: "x"}) })
		run(func() { _, _ = client.SendMessage(ctx, &gogram.SendMessageParams{ChatID: "-100", Text: "x"}) })
		run(func() { _, _ = client.SendMessage(ctx, &gogram.SendMessageParams{ChatID: "8", Text: "x"}) })
		run(func() { _, _ = client.SendChatAction(ctx, &gogram.SendChatActionParams{ChatID: "7", Action: "typing"}) })
		run(func() { _, _ = client.GetMe(ctx, nil) })
	}

	start := time.Now()

	wg.Wait()

	elapsed := time.Since(start)

	mu.Lock()
	defer mu.Unlock()

	for _, key := range []string{"sendMessage 7", "sendMessage -100", "sendMessage 8"} {
		ts := times[key]
		if len(ts) != 3 {
			t.Fatalf("%s: %d calls, want 3", key, len(ts))
		}

		for i := 1; i < len(ts); i++ {
			if gap := ts[i].Sub(ts[i-1]); gap < spacing-5*time.Millisecond {
				t.Fatalf("%s: calls %d and %d only %v apart, want >= %v", key, i-1, i, gap, spacing)
			}
		}
	}

	// Independent chats run in parallel: total time is ~2 spacings, not 8.
	if elapsed > 5*spacing {
		t.Fatalf("chats were serialized across each other: %v", elapsed)
	}

	if len(times["sendChatAction 7"]) != 3 || len(times["getMe "]) != 3 {
		t.Fatalf("unthrottled methods must all run: %v", times)
	}
}

func TestThrottle_ContextDeadline(t *testing.T) {
	t.Parallel()

	noop := gogram.CallerFunc(func(context.Context, gogram.Request, any) error { return nil })

	client, err := gogram.NewClient(testToken,
		gogram.WithCaller(noop),
		gogram.WithTransformers(gogram.Throttle(gogram.ThrottleConfig{PrivateRate: rate.Every(time.Hour)})),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	params := &gogram.DeleteMessageParams{ChatID: "7", MessageID: 1}

	if _, err = client.DeleteMessage(t.Context(), params); err != nil {
		t.Fatalf("delete must not be throttled: %v", err)
	}

	if _, err = client.SendMessage(t.Context(), &gogram.SendMessageParams{ChatID: "7", Text: "a"}); err != nil {
		t.Fatalf("first send: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()

	if _, err = client.SendMessage(ctx, &gogram.SendMessageParams{ChatID: "7", Text: "b"}); err == nil {
		t.Fatal("second send within the hour must fail on the context deadline")
	}
}
