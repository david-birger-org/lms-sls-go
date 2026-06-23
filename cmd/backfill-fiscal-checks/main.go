package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"

	"github.com/apexwoot/lms-sls-go/internal/db"
	"github.com/apexwoot/lms-sls-go/internal/fiscalchecksync"
	"github.com/apexwoot/lms-sls-go/internal/monobank"
)

func main() {
	limit := flag.Int("limit", fiscalchecksync.DefaultBatchLimit, "maximum paid registration payments to inspect")
	timeout := flag.Duration("timeout", 2*time.Minute, "maximum runtime for this backfill run")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))
	for _, name := range []string{".env.local", ".env"} {
		if err := godotenv.Load(name); err == nil {
			slog.Info("loaded env file", "file", name)
			break
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	defer db.Close()

	result, err := fiscalchecksync.SyncMissing(
		ctx,
		fiscalchecksync.DBStore{},
		monobank.NewClient(),
		*limit,
	)
	attrs := []any{
		"scanned", result.Scanned,
		"synced", result.Synced,
		"empty", result.Empty,
		"failed", result.Failed,
	}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
		slog.Error("fiscal check backfill failed", attrs...)
		os.Exit(1)
	}
	slog.Info("fiscal check backfill finished", attrs...)
}
