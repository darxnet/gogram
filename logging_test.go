package gogram_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/darxnet/gogram"
)

func TestLogging(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	client, err := gogram.NewClient(testToken,
		gogram.WithCaller(cannedCaller(`true`)),
		gogram.WithTransformers(gogram.Logging(logger, slog.LevelDebug)),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	params := &gogram.DeleteMessageParams{ChatID: gogram.ChatID{ID: 7}, MessageID: 1}
	if _, err = client.DeleteMessage(t.Context(), params); err != nil {
		t.Fatal(err)
	}

	// A bool body does not decode into a Message: logged as an error.
	_, _ = client.GetMe(t.Context(), nil)

	out := buf.String()

	if !strings.Contains(out, "level=DEBUG") || !strings.Contains(out, "method=deleteMessage") {
		t.Fatalf("missing debug record: %s", out)
	}

	if !strings.Contains(out, "level=ERROR") || !strings.Contains(out, "method=getMe") || !strings.Contains(out, "err=") {
		t.Fatalf("missing error record: %s", out)
	}
}
