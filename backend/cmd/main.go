package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tashan-weir-seepage/internal/api"
	"tashan-weir-seepage/internal/database"
)

func main() {
	log.Println("================================================")
	log.Println("  古代它山堰坝体结构渗流仿真与防渗优化系统")
	log.Println("  Tashan Weir Seepage Simulation System")
	log.Println("================================================")
	log.Println()

	port := getEnv("PORT", "8080")

	log.Println("[INIT] Connecting to TimescaleDB...")
	db, err := database.NewDatabase()
	if err != nil {
		log.Fatalf("[ERROR] Database connection failed: %v", err)
	}
	defer db.Close()
	log.Println("[OK] Database connected")

	store := database.NewDataStore(db)

	log.Println("[INIT] Initializing modular services (dtu_receiver + alarm_mqtt + seepage_simulator + anti_seepage_optimizer)...")
	server := api.NewServer(store)
	defer server.Stop()
	log.Println("[OK] Modular services started")

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      server.Router(),
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 600 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Println()
		log.Printf("[SERVER] API Server listening on :%s", port)
		log.Printf("[SERVER] Frontend available at http://localhost:%s/", port)
		log.Printf("[SERVER] API health at http://localhost:%s/api/v1/health", port)
		log.Printf("[SERVER] API configs at http://localhost:%s/api/v1/configs", port)
		log.Println()

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[ERROR] Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Println()
	log.Printf("[SHUTDOWN] Received signal: %v", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Println("[SHUTDOWN] Stopping HTTP server...")
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("[WARN] HTTP server shutdown error: %v", err)
	}

	log.Println("[SHUTDOWN] Server exited gracefully")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
