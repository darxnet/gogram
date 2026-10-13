package gogram_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/darxnet/gogram"
)

func TestInMemoryStorer_SetGetFinish(t *testing.T) {
	t.Parallel()

	s := gogram.NewInMemoryStorer()
	defer s.Close() //nolint:errcheck

	ctx := t.Context()

	if err := s.SetState(ctx, 1, 2, "a"); err != nil {
		t.Fatalf("SetState: %v", err)
	}

	state, err := s.GetState(ctx, 1, 2)
	if err != nil || state != "a" {
		t.Fatalf("GetState = %q, %v; want a, nil", state, err)
	}

	if err = s.Finish(ctx, 1, 2); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	if state, _ = s.GetState(ctx, 1, 2); state != "" {
		t.Fatalf("GetState after Finish = %q, want empty", state)
	}
}

func TestInMemoryStorer_TTLExpires(t *testing.T) {
	t.Parallel()

	const ttl = 20 * time.Millisecond

	s := gogram.NewInMemoryStorer(gogram.WithTTL(ttl))
	defer s.Close() //nolint:errcheck

	ctx := t.Context()

	if err := s.SetState(ctx, 1, 2, "a"); err != nil {
		t.Fatalf("SetState: %v", err)
	}

	time.Sleep(2 * ttl)

	if state, _ := s.GetState(ctx, 1, 2); state != "" {
		t.Fatalf("GetState after ttl = %q, want empty", state)
	}
}

// countingStorer counts GetState calls to verify FilterState memoization.
type countingStorer struct {
	gets  atomic.Int32
	state gogram.State
}

func (s *countingStorer) SetState(context.Context, int64, int64, gogram.State) error { return nil }

func (s *countingStorer) GetState(context.Context, int64, int64) (gogram.State, error) {
	s.gets.Add(1)
	return s.state, nil
}

func (s *countingStorer) Finish(context.Context, int64, int64) error { return nil }
func (s *countingStorer) Close() error                               { return nil }

func TestFilterState_MemoizesPerUpdate(t *testing.T) {
	t.Parallel()

	storer := &countingStorer{state: "c"}

	r := gogram.NewRouter()

	var handled gogram.State

	for _, state := range []gogram.State{"a", "b", "c"} {
		r.HandleOnMessage(func(*gogram.Context, *gogram.Message) error {
			handled = state
			return nil
		}, gogram.FilterState(storer, state))
	}

	client, err := gogram.NewClient(testToken)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	update := &gogram.Update{Message: &gogram.Message{From: &gogram.User{ID: 7}, Chat: gogram.Chat{ID: 7}}}
	r.Process(gogram.NewTestContext(t.Context(), client, update))

	if handled != "c" {
		t.Fatalf("handled state = %q, want c", handled)
	}

	if gets := storer.gets.Load(); gets != 1 {
		t.Fatalf("GetState calls = %d, want 1 (memoized across 3 routes)", gets)
	}
}

func TestContext_FSM_SetInvalidatesMemo(t *testing.T) {
	t.Parallel()

	storer := &countingStorer{state: "a"}

	client, err := gogram.NewClient(testToken)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	update := &gogram.Update{Message: &gogram.Message{From: &gogram.User{ID: 7}, Chat: gogram.Chat{ID: 7}}}
	ctx := gogram.NewTestContext(t.Context(), client, update)

	filter := gogram.FilterState(storer, "a")

	filter(ctx)
	filter(ctx)

	if gets := storer.gets.Load(); gets != 1 {
		t.Fatalf("GetState calls = %d, want 1", gets)
	}

	fsm, ok := ctx.FSM(storer)
	if !ok {
		t.Fatal("FSM: expected ok")
	}

	if err = fsm.Set(ctx, "b"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	filter(ctx)

	if gets := storer.gets.Load(); gets != 2 {
		t.Fatalf("GetState calls after Set = %d, want 2 (memo invalidated)", gets)
	}
}
