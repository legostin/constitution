package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/legostin/constitution/internal/config"
	"github.com/legostin/constitution/internal/server"
)

func main() {
	configPath := flag.String("config", "", "path to constitution YAML config")
	addr := flag.String("addr", ":8081", "listen address")
	dbPath := flag.String("db", "constitution.db", "SQLite database path for audit log")
	token := flag.String("token", "", "bearer token for auth (or CONSTITUTION_TOKEN env var)")
	flag.Parse()

	// Load config
	cfgPath := *configPath
	if cfgPath == "" {
		cfgPath = os.Getenv("CONSTITUTION_CONFIG")
	}
	if cfgPath == "" {
		cwd, _ := os.Getwd()
		cfgPath = config.FindConfigPath("", cwd)
	}
	if cfgPath == "" {
		slog.Error("no configuration found")
		os.Exit(1)
	}

	policy, err := config.Load(cfgPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	authToken := *token
	if authToken == "" {
		authToken = os.Getenv("CONSTITUTION_TOKEN")
	}

	srv, err := server.New(server.Config{
		Addr:   *addr,
		Policy: policy,
		DBPath: *dbPath,
		Token:  authToken,
	})
	if err != nil {
		slog.Error("failed to create server", "error", err)
		os.Exit(1)
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.ListenAndServe(*addr); err != nil {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down...")
	srv.Shutdown(context.Background())
}
