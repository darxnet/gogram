package gogram

import (
	"context"
	"sync"
	"time"

	"github.com/darxnet/goshard"
)

// State represents a specific state in a state machine.
type State string

// StateStore is the interface for state persistence.
// Implementations must be safe for concurrent use.
type StateStore interface {
	SetState(ctx context.Context, userID, chatID int64, state State) error
	GetState(ctx context.Context, userID, chatID int64) (State, error)
	Finish(ctx context.Context, userID, chatID int64) error
	Close() error
}

// storageKey uniquely identifies a user in a chat for state management.
type storageKey struct {
	UserID int64
	ChatID int64
}

// stateEntry holds a state value alongside an optional expiry timestamp.
// It is comparable, which lets expiry use CompareAndDelete: a concurrent
// SetState that replaced the entry is never deleted by mistake.
type stateEntry struct {
	state     State
	expiresAt time.Time // zero means no expiry
}

func (e stateEntry) expired(now time.Time) bool {
	return !e.expiresAt.IsZero() && !now.Before(e.expiresAt)
}

// MemoryStateStoreOption configures an MemoryStateStore.
type MemoryStateStoreOption func(*MemoryStateStore)

// WithTTL sets a time-to-live for inactive FSM states.
// Entries that have not been updated within ttl are purged automatically.
// A cleanup goroutine is started; call Close() to stop it.
func WithTTL(ttl time.Duration) MemoryStateStoreOption {
	return func(s *MemoryStateStore) {
		s.ttl = ttl
	}
}

// MemoryStateStore is a sharded, thread-safe, in-memory state storage.
// It is the default storer if no other is provided.
type MemoryStateStore struct {
	states    goshard.ComparableMap[storageKey, stateEntry]
	ttl       time.Duration
	closeOnce sync.Once
	done      chan struct{}
}

// NewMemoryStateStore creates a new in-memory state storer.
func NewMemoryStateStore(opts ...MemoryStateStoreOption) *MemoryStateStore {
	s := &MemoryStateStore{done: make(chan struct{})}
	for _, opt := range opts {
		opt(s)
	}

	if s.ttl > 0 {
		go s.cleanupLoop()
	}

	return s
}

// cleanupLoop sweeps expired entries. All() copies a shard's entries out
// under a read lock and yields with no lock held, so the sweep never
// blocks SetState/GetState for the whole map.
func (s *MemoryStateStore) cleanupLoop() {
	ticker := time.NewTicker(s.ttl)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case now := <-ticker.C:
			for key, entry := range s.states.All() {
				if entry.expired(now) {
					s.states.CompareAndDelete(key, entry)
				}
			}
		}
	}
}

// SetState saves the state for a user-chat pair.
func (s *MemoryStateStore) SetState(_ context.Context, userID, chatID int64, state State) error {
	entry := stateEntry{state: state}
	if s.ttl > 0 {
		entry.expiresAt = time.Now().Add(s.ttl)
	}

	s.states.Store(storageKey{UserID: userID, ChatID: chatID}, entry)

	return nil
}

// GetState retrieves the state for a user-chat pair.
func (s *MemoryStateStore) GetState(_ context.Context, userID, chatID int64) (State, error) {
	key := storageKey{UserID: userID, ChatID: chatID}

	entry, ok := s.states.Load(key)
	if !ok {
		return "", nil
	}

	if entry.expired(time.Now()) {
		s.states.CompareAndDelete(key, entry)
		return "", nil
	}

	return entry.state, nil
}

// Finish removes the state for a user-chat pair.
func (s *MemoryStateStore) Finish(_ context.Context, userID, chatID int64) error {
	s.states.Delete(storageKey{UserID: userID, ChatID: chatID})
	return nil
}

// Close stops the background cleanup goroutine if TTL was configured.
func (s *MemoryStateStore) Close() error {
	s.closeOnce.Do(func() { close(s.done) })
	return nil
}

// FSM provides methods to control the state machine within a handler.
// It is a small value; copying it is cheap and intended.
type FSM struct {
	store  StateStore
	userID int64
	chatID int64
}

// Set changes the state for the current user/chat.
func (f FSM) Set(ctx context.Context, state State) error {
	invalidateState(ctx)
	return f.store.SetState(ctx, f.userID, f.chatID, state)
}

// Finish clears the state for the current user/chat.
func (f FSM) Finish(ctx context.Context) error {
	invalidateState(ctx)
	return f.store.Finish(ctx, f.userID, f.chatID)
}

// State returns the current state for the user/chat.
func (f FSM) State(ctx context.Context) (State, error) {
	return f.store.GetState(ctx, f.userID, f.chatID)
}

// invalidateState drops the per-update state memo after a write.
func invalidateState(ctx context.Context) {
	if c, ok := ctx.(*Context); ok {
		c.fsm.loaded = false
	}
}

// FSM returns a state machine controller for the current context, backed
// by the client's state store, see [WithStateStore].
// The ok result is false when the update carries no user.
func (ctx *Context) FSM() (f FSM, ok bool) {
	user := ctx.User()
	if user == nil {
		return FSM{}, false
	}

	// A chat is not always present (e.g., inline query), use 0 as a placeholder.
	var chatID int64
	if chat := ctx.Chat(); chat != nil {
		chatID = chat.ID
	}

	return FSM{store: ctx.client.cfg.store, userID: user.ID, chatID: chatID}, true
}

// fsmMemo caches the state looked up while routing one update, so N
// FilterState routes cost one StateStore round-trip instead of N.
type fsmMemo struct {
	state  State
	loaded bool
}

// loadState returns the user's state, memoized per update.
func (ctx *Context) loadState() (State, bool) {
	if ctx.fsm.loaded {
		return ctx.fsm.state, true
	}

	f, ok := ctx.FSM()
	if !ok {
		return "", false
	}

	state, err := f.State(ctx.context)
	if err != nil {
		return "", false
	}

	ctx.fsm = fsmMemo{state: state, loaded: true}

	return state, true
}

// FilterState creates a filter that matches when the user is in the expected
// state in the client's state store, see [WithStateStore].
// This is the primary way to integrate the FSM with the router.
func FilterState(expected State) Filter {
	return func(ctx *Context) bool {
		state, ok := ctx.loadState()
		return ok && state == expected
	}
}
