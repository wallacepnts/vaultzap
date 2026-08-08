// VaultZap — local, browsable archive of exported WhatsApp conversations.
// Copyright (C) 2026 Wallace Pontes
//
// This program is free software: you can redistribute it and/or modify it under the
// terms of the GNU Affero General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later version.
// It is distributed WITHOUT ANY WARRANTY. See the LICENSE file, or
// <https://www.gnu.org/licenses/>.

// Command vaultzap: serves the reading UI by default, or runs utility subcommands.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/wallacepnts/vaultzap/internal/config"
	"github.com/wallacepnts/vaultzap/internal/ingest"
	"github.com/wallacepnts/vaultzap/internal/store"
	"github.com/wallacepnts/vaultzap/internal/web"
)

// Set via -ldflags "-X main.version=..." at build time (see Makefile and
// deploy/Dockerfile); "dev" is what `go run`/`make dev` leaves it at.
var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "healthcheck":
			if err := runHealthcheck(); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return
		case "reset-password":
			if err := runResetPassword(); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return
		case "ingest":
			if err := runIngest(os.Args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return
		}
	}

	if err := runServer(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Imports export files synchronously, printing the parser's report for each. Same code
// the watched folder's scan uses.
func runIngest(paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("usage: vaultzap ingest <file.txt>...")
	}

	cfg, err := config.LoadFromEnv()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	ctx := context.Background()
	db, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	for _, path := range paths {
		report, err := ingest.ImportFile(ctx, db, path, path, cfg.MediaDir, cfg.DefaultDateOrder)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: error: %v\n", path, err)
			continue
		}
		if report.AlreadyDone {
			fmt.Printf("%s: already imported, no change\n", path)
			continue
		}
		fmt.Printf("%s: chat %q (id=%d) — %d added, %d skipped, %d warnings\n",
			path, report.ChatName, report.ChatID, report.Added, report.Skipped, report.Warnings)
	}
	return nil
}

func runServer() error {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	level := slog.LevelInfo
	if cfg.LogLevel == "debug" {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	db, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := warnIfUnclaimed(ctx, db, cfg); err != nil {
		return err
	}

	// The scan runs on a single serial goroutine, from startup to graceful shutdown.
	scanner := ingest.NewScanner(db, cfg)
	scanDone := make(chan struct{})
	go func() {
		defer close(scanDone)
		scanner.Scan(ctx)
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthzHandler)
	mux.Handle("/", web.NewHandler(db, cfg, scanner, version).Routes())

	server := &http.Server{
		Addr:    cfg.Addr,
		Handler: mux,
	}

	errs := make(chan error, 1)
	go func() {
		slog.Info("server started", "addr", cfg.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- err
		}
	}()

	var runErr error
	select {
	case <-ctx.Done():
		slog.Info("shutting down server")
	case err := <-errs:
		runErr = fmt.Errorf("server: %w", err)
	}
	// Also stops the scan when the exit came from the server failing, not from a signal.
	cancel()

	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := server.Shutdown(ctxShutdown); err != nil && runErr == nil {
		runErr = err
	}

	// The scan holds an open transaction while importing and db.Close() is deferred above,
	// so returning without waiting would pull the connection out from under it.
	select {
	case <-scanDone:
	case <-time.After(shutdownScanWait):
		slog.Warn("varredura não terminou a tempo; encerrando assim mesmo")
	}
	return runErr
}

const shutdownScanWait = 15 * time.Second

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func runHealthcheck() error {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return err
	}

	url := "http://127.0.0.1" + healthcheckPort(cfg.Addr) + "/healthz"
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("healthcheck: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck: status %d", resp.StatusCode)
	}
	return nil
}

func healthcheckPort(addr string) string {
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		return addr[i:]
	}
	return ":8927"
}
