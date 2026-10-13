package gogram

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultWebhookMaxBodyBytes      int64 = 4 << 20
	defaultWebhookReadHeaderTimeout       = 5 * time.Second
)

var (
	// ErrInvalidWebhookURL indicates that the public webhook URL is not HTTPS.
	ErrInvalidWebhookURL = errors.New("gogram: invalid webhook url")
	// ErrInvalidWebhookAddr indicates that no local webhook listen address was supplied.
	ErrInvalidWebhookAddr = errors.New("gogram: invalid webhook listen address")
)

// StartWebhook registers params with setWebhook, then serves the webhook
// on addr until ctx is cancelled and shuts down gracefully. The webhook
// stays registered: a later [Client.Start] removes it when getUpdates
// reports the conflict.
func (c *Client) StartWebhook(ctx context.Context, addr string, params *SetWebhookParams) error {
	if addr == "" {
		return ErrInvalidWebhookAddr
	}
	if params == nil || params.URL == "" {
		return ErrInvalidWebhookURL
	}

	webhookURL, err := url.Parse(params.URL)
	if err != nil {
		return err
	}

	if webhookURL.Host == "" {
		return ErrInvalidWebhookURL
	}

	pattern := webhookURL.EscapedPath()
	if pattern == "" {
		pattern = params.URL
	}

	if pattern == "" || pattern[0] != '/' {
		return ErrInvalidWebhookURL
	}

	innerCtx, state, err := c.beginRun(ctx)
	if err != nil {
		return err
	}
	defer c.finishRun(state)

	if _, err = c.SetWebhook(ctx, params); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST "+pattern, c.webhookHandler(params.SecretToken))

	readHeaderTimeout := defaultWebhookReadHeaderTimeout
	if c.cfg.timeout > 0 {
		readHeaderTimeout = min(readHeaderTimeout, c.cfg.timeout)
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       max(c.cfg.timeout, readHeaderTimeout), // the body is bounded too
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err = <-errCh:
	case <-innerCtx.Done():
		shutdownTimeout := c.cfg.timeout
		if shutdownTimeout <= 0 {
			shutdownTimeout = defaultTimeout
		}

		shutdownCtx, shutdownCancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer shutdownCancel()
		shutdownErr := srv.Shutdown(shutdownCtx)
		if shutdownErr != nil {
			_ = srv.Close()
		}
		err = <-errCh
		if shutdownErr != nil {
			return shutdownErr
		}
	}

	if errors.Is(err, http.ErrServerClosed) {
		if ctxErr := ctx.Err(); !errors.Is(ctxErr, context.Canceled) {
			return ctxErr
		}
		return nil
	}
	return err
}

func (c *Client) webhookHandler(secretToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if secretToken != "" && !validSecretToken(r.Header.Get("X-Telegram-Bot-Api-Secret-Token"), secretToken) {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		buffer := acquireBuffer()
		defer releaseBuffer(buffer)

		if _, copyErr := io.Copy(buffer, http.MaxBytesReader(w, r.Body, defaultWebhookMaxBodyBytes)); copyErr != nil {
			status := http.StatusBadRequest
			var tooLarge *http.MaxBytesError
			if errors.As(copyErr, &tooLarge) {
				status = http.StatusRequestEntityTooLarge
			}
			http.Error(w, http.StatusText(status), status)
			return
		}

		var update Update
		if unmarshalErr := json.Unmarshal(buffer.B, &update); unmarshalErr != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		c.processUpdate(c.acquireContext(r.Context(), &update))
		w.WriteHeader(http.StatusOK)
	}
}

// validSecretToken compares the webhook secret in constant time.
func validSecretToken(got, want string) bool {
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
