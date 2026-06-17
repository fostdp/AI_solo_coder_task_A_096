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
	"tashan-weir-seepage/internal/mqtt"
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

	log.Println("[INIT] Initializing MQTT service...")
	var mqttSvc *mqtt.MQTTService
	mqttSvc, err = mqtt.NewMQTTService()
	if err != nil {
		log.Printf("[WARN] MQTT service initialization failed (running in fallback mode): %v", err)
		log.Println("[INFO] Alarms will be logged but not pushed via MQTT")
		mqttSvc = nil
	} else {
		defer mqttSvc.Close()
		log.Println("[OK] MQTT service connected")

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		mqttSvc.PublishStatus(ctx, map[string]interface{}{
			"status":  "started",
			"version": "1.0.0",
			"service": "tashan-weir-seepage-backend",
		})
	}

	log.Println("[INIT] Setting up HTTP API server...")
	server := api.NewServer(store, mqttSvc)
	log.Println("[OK] API routes configured")

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      server.Router(),
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Println()
		log.Printf("[SERVER] API Server listening on :%s", port)
		log.Printf("[SERVER] Frontend available at http://localhost:%s/", port)
		log.Printf("[SERVER] API docs available at http://localhost:%s/api/v1/health", port)
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if mqttSvc != nil {
		mqttCtx, mqttCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer mqttCancel()
		mqttSvc.PublishStatus(mqttCtx, map[string]interface{}{
			"status":  "stopped",
			"version": "1.0.0",
			"service": "tashan-weir-seepage-backend",
		})
	}

	log.Println("[SHUTDOWN] Stopping HTTP server...")
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("[ERROR] Server shutdown failed: %v", err)
	}

	log.Println("[SHUTDOWN] Server exited gracefully")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
