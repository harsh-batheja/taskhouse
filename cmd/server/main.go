package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/harsh-batheja/taskhouse/internal/server"
	"github.com/harsh-batheja/taskhouse/internal/store"
	"github.com/harsh-batheja/taskhouse/internal/webhook"
)

func main() {
	dbPath := envOr("TASKHOUSE_DB", "./taskhouse.db")
	port := envOr("TASKHOUSE_PORT", "8080")
	authToken := os.Getenv("TASKHOUSE_AUTH_TOKEN")

	s, err := store.New(dbPath)
	if err != nil {
		log.Fatalf("failed to open store: %v", err)
	}
	defer s.Close()

	d := webhook.NewDispatcher(s)
	srv := server.New(s, d, authToken)

	httpSrv := &http.Server{
		Addr:    ":" + port,
		Handler: srv,
	}

	go func() {
		log.Printf("taskhouse server listening on :%s", port)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}
	log.Println("server stopped")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
