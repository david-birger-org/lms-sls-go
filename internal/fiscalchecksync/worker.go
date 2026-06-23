package fiscalchecksync

import (
	"context"
	"log/slog"
	"time"
)

type WorkerConfig struct {
	Interval time.Duration
	Limit    int
	Timeout  time.Duration
}

func (c WorkerConfig) withDefaults() WorkerConfig {
	if c.Interval <= 0 {
		c.Interval = 10 * time.Minute
	}
	if c.Limit <= 0 {
		c.Limit = DefaultBatchLimit
	}
	if c.Timeout <= 0 {
		c.Timeout = 45 * time.Second
	}
	return c
}

func StartWorker(ctx context.Context, store Store, client Client, config WorkerConfig) {
	cfg := config.withDefaults()
	go func() {
		runWorkerOnce(ctx, store, client, cfg)

		ticker := time.NewTicker(cfg.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runWorkerOnce(ctx, store, client, cfg)
			}
		}
	}()
}

func runWorkerOnce(ctx context.Context, store Store, client Client, config WorkerConfig) {
	runCtx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()

	result, err := SyncMissing(runCtx, store, client, config.Limit)
	attrs := []any{
		"scanned", result.Scanned,
		"synced", result.Synced,
		"empty", result.Empty,
		"failed", result.Failed,
	}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
		slog.WarnContext(runCtx, "fiscal check sync finished with errors", attrs...)
		return
	}
	if result.Scanned > 0 {
		slog.InfoContext(runCtx, "fiscal check sync finished", attrs...)
	}
}
