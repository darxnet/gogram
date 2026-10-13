// Package tgtest tests gogram handlers without a network: a [Recorder]
// replaces the client's transport, records every API call and answers
// with canned results, and the update constructors build the incoming
// side.
//
//	rec := tgtest.NewRecorder()
//	client, _ := gogram.NewClient("123:abc", gogram.WithCaller(rec))
//
//	router.Process(tgtest.NewContext(t, client, tgtest.Text(7, "/start")))
//
//	rec.AssertCalled(t, "sendMessage", func(p *gogram.SendMessageParams) bool {
//		return p.Text == "Hello"
//	})
package tgtest

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/darxnet/gogram"
)

// Call is one recorded API call.
type Call struct {
	Method string
	// Params is the typed *<Method>Params, nil for raw calls.
	Params any
}

// Recorder is a [gogram.Caller] that records calls and answers them with
// canned results. It is safe for concurrent use.
type Recorder struct {
	mu      sync.Mutex
	calls   []Call
	results map[string][]json.RawMessage // per method, consumed in order
	errs    map[string]error             // per method, returned on every call
}

var _ gogram.Caller = (*Recorder)(nil)

// NewRecorder returns an empty recorder: every call succeeds with a zero
// result until [Recorder.WillReturn] or [Recorder.WillFail] say otherwise.
func NewRecorder() *Recorder {
	return &Recorder{
		results: make(map[string][]json.RawMessage),
		errs:    make(map[string]error),
	}
}

// WillReturn queues result (a Go value, marshalled to JSON) for the next
// call of method. Queued results are consumed in order; when the queue is
// empty the call succeeds with a zero result.
func (r *Recorder) WillReturn(method string, result any) *Recorder {
	raw, err := json.Marshal(result)
	if err != nil {
		panic(fmt.Sprintf("tgtest: marshal result for %s: %v", method, err))
	}

	r.mu.Lock()
	r.results[method] = append(r.results[method], raw)
	r.mu.Unlock()

	return r
}

// WillFail makes every call of method return err until reset with a nil err.
func (r *Recorder) WillFail(method string, err error) *Recorder {
	r.mu.Lock()
	if err == nil {
		delete(r.errs, method)
	} else {
		r.errs[method] = err
	}
	r.mu.Unlock()

	return r
}

// Call implements [gogram.Caller].
func (r *Recorder) Call(_ context.Context, req gogram.Request, result any) error {
	r.mu.Lock()
	r.calls = append(r.calls, Call{Method: req.Method, Params: req.Params})

	if err := r.errs[req.Method]; err != nil {
		r.mu.Unlock()
		return err
	}

	var raw json.RawMessage
	if queue := r.results[req.Method]; len(queue) != 0 {
		raw, r.results[req.Method] = queue[0], queue[1:]
	}
	r.mu.Unlock()

	if raw == nil {
		return nil
	}

	return json.Unmarshal(raw, result)
}

// Calls returns a copy of the recorded calls in order.
func (r *Recorder) Calls() []Call {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]Call(nil), r.calls...)
}

// Reset forgets recorded calls, queued results and failures.
func (r *Recorder) Reset() {
	r.mu.Lock()
	r.calls = nil
	clear(r.results)
	clear(r.errs)
	r.mu.Unlock()
}

// AssertCalled fails the test unless method was called with params that
// satisfy match (nil matches any call of method).
func (r *Recorder) AssertCalled(tb testing.TB, method string, match any) {
	tb.Helper()

	if !r.called(method, match) {
		tb.Fatalf("tgtest: expected a %s call matching the predicate; calls: %v", method, r.methods())
	}
}

// AssertNotCalled fails the test if method was called at all.
func (r *Recorder) AssertNotCalled(tb testing.TB, method string) {
	tb.Helper()

	if r.called(method, nil) {
		tb.Fatalf("tgtest: unexpected %s call; calls: %v", method, r.methods())
	}
}

// AssertSequence fails the test unless the recorded methods are exactly methods, in order.
func (r *Recorder) AssertSequence(tb testing.TB, methods ...string) {
	tb.Helper()

	got := r.methods()
	if len(got) != len(methods) {
		tb.Fatalf("tgtest: calls %v, want %v", got, methods)
	}

	for i := range methods {
		if got[i] != methods[i] {
			tb.Fatalf("tgtest: calls %v, want %v", got, methods)
		}
	}
}

// called reports whether a call of method matches match, a func(P) bool
// where P is the method's params pointer type, or nil for any.
func (r *Recorder) called(method string, match any) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, call := range r.calls {
		if call.Method != method {
			continue
		}

		if match == nil || matches(match, call.Params) {
			return true
		}
	}

	return false
}

func (r *Recorder) methods() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]string, len(r.calls))
	for i, call := range r.calls {
		out[i] = call.Method
	}

	return out
}

// Matcher is implemented by typed predicates, see [Match].
type Matcher interface {
	Match(params any) bool
}

// Match wraps a typed predicate for [Recorder.AssertCalled]: it reports
// false for params of another type.
func Match[P any](f func(P) bool) Matcher {
	return matcher[P](f)
}

type matcher[P any] func(P) bool

func (m matcher[P]) Match(params any) bool {
	p, ok := params.(P)
	return ok && m(p)
}

func matches(match, params any) bool {
	switch m := match.(type) {
	case Matcher:
		return m.Match(params)
	case func(any) bool:
		return m(params)
	default:
		panic(fmt.Sprintf("tgtest: match must be a tgtest.Matcher or func(any) bool, got %T", match))
	}
}

// NewContext returns a handler context for update, released when the test ends.
func NewContext(tb testing.TB, client *gogram.Client, update *gogram.Update) *gogram.Context {
	tb.Helper()

	return gogram.NewTestContext(tb.Context(), client, update)
}

// Default identities used by the update constructors.
var (
	// User is the sender of constructed updates.
	User = gogram.User{ID: 100, FirstName: "Test", Username: "tester"}
	// PrivateChat is the chat of constructed updates; its id equals User.ID.
	PrivateChat = gogram.Chat{ID: 100, Type: gogram.ChatPrivate, FirstName: "Test", Username: "tester"}
)

var nextID = struct {
	sync.Mutex

	update, message int64
}{update: 1000, message: 1}

func ids() (update, message int64) {
	nextID.Lock()
	defer nextID.Unlock()

	nextID.update++
	nextID.message++

	return nextID.update, nextID.message
}

// Message returns an update with a text message in chat from [User].
func Message(chat *gogram.Chat, text string) *gogram.Update {
	updateID, messageID := ids()
	user := User

	return &gogram.Update{
		UpdateID: updateID,
		Message: &gogram.Message{
			MessageID: messageID,
			From:      &user,
			Chat:      *chat,
			Date:      1700000000,
			Text:      text,
			Entities:  commandEntities(text),
		},
	}
}

// Text returns an update with a text message in a private chat with chatID.
func Text(chatID int64, text string) *gogram.Update {
	chat := PrivateChat
	chat.ID = chatID

	return Message(&chat, text)
}

// Callback returns an update with a callback query carrying data, pressed
// on a message in a private chat with chatID.
func Callback(chatID int64, data string) *gogram.Update {
	updateID, messageID := ids()
	user := User
	chat := PrivateChat
	chat.ID = chatID

	return &gogram.Update{
		UpdateID: updateID,
		CallbackQuery: &gogram.CallbackQuery{
			ID:   fmt.Sprintf("cb%d", updateID),
			From: user,
			Data: data,
			Message: &gogram.MaybeInaccessibleMessage{
				Message: &gogram.Message{MessageID: messageID, Chat: chat, Date: 1700000000},
			},
		},
	}
}

// commandEntities marks a leading "/command" as a bot_command entity, as Telegram does.
func commandEntities(text string) []gogram.MessageEntity {
	if text == "" || text[0] != '/' {
		return nil
	}

	end := len(text)
	for i := 1; i < len(text); i++ {
		if text[i] == ' ' {
			end = i
			break
		}
	}

	return []gogram.MessageEntity{{Type: gogram.MessageEntityBotCommand, Offset: 0, Length: int64(end)}}
}
