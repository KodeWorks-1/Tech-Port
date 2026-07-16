// Package handler adapts the TechPort server to Vercel's Go serverless
// runtime: one catch-all function wrapping the full chi router.
// vercel.json rewrites every path here; views/ and static/ ship with the
// function via includeFiles.
package handler

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/KodeWorks-1/techport/internal/config"
	"github.com/KodeWorks-1/techport/internal/db"
	"github.com/KodeWorks-1/techport/internal/handlers"
	"github.com/KodeWorks-1/techport/internal/services"
)

var (
	once    sync.Once
	router  http.Handler
	initErr error
)

func setup() {
	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		initErr = err
		return
	}
	if initErr = db.Migrate(ctx, pool); initErr != nil {
		return
	}
	if initErr = db.SeedIfEmpty(ctx, pool); initErr != nil {
		return
	}

	catalog := services.NewCatalog(pool)
	cart := services.NewCart(pool)
	settings := services.NewSettings(pool)
	orders := services.NewOrders(pool, settings)
	users := services.NewUsers(pool)
	admin := services.NewAdmin(pool)
	adminAuth := services.NewAdminAuth(pool)
	renderer := handlers.NewRenderer(cfg.Dev(), handlers.NavFuncs(catalog, settings))
	router = handlers.New(catalog, cart, orders, users, settings, admin, adminAuth, renderer).Router()
}

// Handler is the Vercel entrypoint.
func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(setup)
	if initErr != nil {
		slog.Error("bootstrap failed", "err", initErr)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	router.ServeHTTP(w, r)
}
