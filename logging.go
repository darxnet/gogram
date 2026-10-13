package gogram

import (
	"context"
	"log/slog"
	"time"
)

// Logging returns a [Transformer] that logs every API call with slog:
// successful calls at level, failed ones at [slog.LevelError]. Each record
// carries the method, the duration and, on failure, the error.
// getUpdates is skipped: it is one long-poll per Timeout and only noise.
//
//	gogram.WithTransformers(gogram.Logging(slog.Default(), slog.LevelDebug))
//
// For tracing, write a Transformer of the same shape that opens a span
// around next.Call; Request.Method and Request.Params carry the attributes.
func Logging(logger *slog.Logger, level slog.Level) Transformer {
	return func(next Caller) Caller {
		return CallerFunc(func(ctx context.Context, req Request, result any) error {
			if req.Method == "getUpdates" {
				return next.Call(ctx, req, result)
			}

			start := time.Now()
			err := next.Call(ctx, req, result)
			elapsed := time.Since(start)

			if err != nil {
				logger.LogAttrs(ctx, slog.LevelError, "gogram: api call failed",
					slog.String("method", req.Method), slog.Duration("elapsed", elapsed), slog.Any("err", err))

				return err
			}

			if logger.Enabled(ctx, level) {
				logger.LogAttrs(ctx, level, "gogram: api call",
					slog.String("method", req.Method), slog.Duration("elapsed", elapsed))
			}

			return nil
		})
	}
}
