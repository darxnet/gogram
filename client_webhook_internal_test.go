package gogram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWebhookHandler_QueuesAndAnswersImmediately(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		handled []string
		started = make(chan struct{}, 1)
		release = make(chan struct{})
	)

	router := NewRouter()
	router.HandleOnMessage(func(_ *Context, m *Message) error {
		started <- struct{}{}
		<-release // a slow handler must not delay the HTTP answer

		mu.Lock()
		handled = append(handled, m.Text)
		mu.Unlock()

		return nil
	})

	client, err := NewClient("1:a", WithRouter(router))
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	pool := client.startWorkers(2, 1)
	handler := client.webhookHandler(ctx, pool, "secret")

	post := func(body, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/hook", strings.NewReader(body))
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", token)

		rec := httptest.NewRecorder()
		handler(rec, req)

		return rec
	}

	if rec := post(`{"update_id":1}`, "wrong"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong secret: %d", rec.Code)
	}

	if rec := post(`not json`, "secret"); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad json: %d", rec.Code)
	}

	answered := time.Now()

	const update = `{"update_id":2,"message":{"message_id":1,"date":1,"chat":{"id":7,"type":"private"},"text":"slow"}}`

	rec := post(update, "secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	if time.Since(answered) > 100*time.Millisecond {
		t.Fatal("the answer waited for the handler")
	}

	<-started // the handler is running after the request was answered
	close(release)

	pool.stop()

	if len(handled) != 1 || handled[0] != "slow" {
		t.Fatalf("handled = %v", handled)
	}
}
