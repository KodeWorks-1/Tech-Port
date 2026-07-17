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

	"github.com/KodeWorks-1/techport/internal/config"
	"github.com/KodeWorks-1/techport/internal/db"
	"github.com/KodeWorks-1/techport/internal/handlers"
	"github.com/KodeWorks-1/techport/internal/services"
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		slog.Error("db connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Migrate(context.Background(), pool); err != nil {
		slog.Error("migrate failed", "err", err)
		os.Exit(1)
	}
	if err := db.SeedIfEmpty(context.Background(), pool); err != nil {
		slog.Error("seed failed", "err", err)
		os.Exit(1)
	}

	catalog := services.NewCatalog(pool)
	cart := services.NewCart(pool)
	settings := services.NewSettings(pool)
	orders := services.NewOrders(pool, settings)
	users := services.NewUsers(pool)
	admin := services.NewAdmin(pool)
	adminAuth := services.NewAdminAuth(pool)
	var demoUserID int64
	if cfg.Demo {
		id, err := users.EnsureDemoUser(context.Background())
		if err != nil {
			slog.Error("demo user seed failed", "err", err)
			os.Exit(1)
		}
		demoUserID = id
		slog.Info("DEMO MODE: admin login bypassed, visitors auto-logged-in (set DEMO_MODE=0 to disable)")
	}

	renderer := handlers.NewRenderer(cfg.Dev(), cfg.Demo, handlers.NavFuncs(catalog, settings))
	h := handlers.New(catalog, cart, orders, users, settings, admin, adminAuth, renderer, cfg.Demo, demoUserID)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           h.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("techport listening", "port", cfg.Port, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}
