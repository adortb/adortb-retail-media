// Package main 启动 Retail Media Network HTTP 服务（端口 8095）。
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"github.com/adortb/adortb-retail-media/internal/api"
	"github.com/adortb/adortb-retail-media/internal/auction"
	"github.com/adortb/adortb-retail-media/internal/campaign"
	"github.com/adortb/adortb-retail-media/internal/catalog"
	"github.com/adortb/adortb-retail-media/internal/reporting"
)

func main() {
	dsn := envOrDefault("DATABASE_URL", "postgres://localhost/adortb_rmn?sslmode=disable")
	port := envOrDefault("PORT", "8095")

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(30)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	// 初始化存储层
	productStore := catalog.NewProductStore(db)
	categoryStore := catalog.NewCategoryStore(db)
	inventoryStore := catalog.NewInventoryStore(db)
	spStore := campaign.NewSPStore(db)
	sbStore := campaign.NewSBStore(db)
	reporter := reporting.NewReporter(db)

	// 预算守卫（16 分片）
	budgetGuard := campaign.NewBudgetGuard(16)

	// 拍卖引擎
	searchAuction := auction.NewSearchAuction(spStore, sbStore, productStore, inventoryStore, budgetGuard)
	catAuction := auction.NewCategoryAuction(productStore, inventoryStore, budgetGuard)

	// HTTP 路由
	mux := http.NewServeMux()
	handler := api.NewHandler(
		productStore, categoryStore, inventoryStore,
		spStore, sbStore,
		searchAuction, catAuction,
		reporter,
	)
	handler.RegisterRoutes(mux)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("retail-media listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutCancel()
	log.Println("shutting down...")
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
