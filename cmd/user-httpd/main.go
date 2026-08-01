package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx := context.Background()
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	srv := http.Server{
		Addr: ":8080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			w.Write([]byte(`{"status": "ok"}`))
		}),
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
	}
	go func() {
		slog.DebugContext(ctx, "serving http...", "addr", srv.Addr)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			slog.ErrorContext(ctx, "http listener error", slog.String("error", context.Cause(ctx).Error()))
		}
	}()

	<-ctx.Done()
	stop()
	slog.DebugContext(ctx, "interrupt caught, shutting down process")

	{ // Shutdown
		slog.Debug("BEGIN: shutdown")
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		go func() {
			if err := srv.Shutdown(ctx); err != nil {
				slog.DebugContext(ctx, "http server shutdown error", slog.String("error", err.Error()))
			}
		}()
		slog.Debug("END: shutdown")
		os.Exit(0)
	}

}
