package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Ar52015/beacon/internal/api"
	"github.com/Ar52015/beacon/internal/config"
	"github.com/Ar52015/beacon/internal/store"

	_ "net/http/pprof"
)

func main() {
	// logger
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// config import
	conf, err := config.Load()
	if err != nil {
		slog.Error("BEACON TOKEN cannot be empty", "err", err)
		os.Exit(1)
	}

	st := store.NewStore()
	ser := api.NewServer(st, conf.Token)
	srv := &http.Server{
		Addr:     conf.Addr,
		Handler:  ser.Routes(),
		ErrorLog: slog.NewLogLogger(slog.Default().Handler(), slog.LevelError),
	}

	// pprof
	go func() {
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			slog.Error("pprof server failed", "err", err)
		}
	}()

	// graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server closed unexpectedly", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	// shutdown fired
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Shutdown error", "err", err)
	}
}
