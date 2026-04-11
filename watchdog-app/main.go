package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"
)

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	primaryURL := envOrDefault("PRIMARY_URL", "http://victim-a:9995/status")
	backupURL := envOrDefault("BACKUP_URL", "http://victim-b:9995/status")
	backupContainer := envOrDefault("BACKUP_CONTAINER", "victim-b")

	checkInterval, err := time.ParseDuration(envOrDefault("CHECK_INTERVAL", "5s"))
	if err != nil {
		log.Fatalf("invalid CHECK_INTERVAL: %v", err)
	}

	failureThreshold, err := strconv.Atoi(envOrDefault("FAILURE_THRESHOLD", "3"))
	if err != nil || failureThreshold < 1 {
		log.Fatalf("invalid FAILURE_THRESHOLD: must be a positive integer")
	}

	log.Printf("Watchdog started. primary=%s backup=%s interval=%s threshold=%d",
		primaryURL, backupURL, checkInterval, failureThreshold)

	// cancel on SIGTERM or Ctrl-C
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var backupStarted atomic.Bool
	failureCount := 0
	client := &http.Client{Timeout: 2 * time.Second}

	// HTTP status server
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		if backupStarted.Load() {
			fmt.Fprintln(w, "watchdog: victim-a is DOWN, victim-b is running")
		} else {
			fmt.Fprintln(w, "watchdog: victim-a is UP")
		}
	})
	server := &http.Server{Addr: ":9999", Handler: mux}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-ticker.C:
			// after failover: poll victim-b and keep logging its liveness
			if backupStarted.Load() {
				resp, err := client.Get(backupURL)
				if err != nil {
					log.Printf("[DOWN] victim-b is unreachable: %v", err)
				} else {
					resp.Body.Close()
					log.Printf("[ALIVE] victim-b responded: %s", resp.Status)
				}
				continue
			}

			resp, err := client.Get(primaryURL)
			if err != nil {
				failureCount++
				log.Printf("[DOWN] victim-a is unreachable: %v (failure %d/%d)", err, failureCount, failureThreshold)

				if failureCount >= failureThreshold {
					log.Printf("[ACTION] Starting backup container: %s", backupContainer)
					cmd := exec.Command("docker", "start", backupContainer)
					out, err := cmd.CombinedOutput()
					if err != nil {
						log.Printf("[ERROR] Failed to start %s: %v\n%s", backupContainer, err, out)
					} else {
						log.Printf("[OK] %s started successfully: %s", backupContainer, out)
						backupStarted.Store(true)
					}
				}
			} else {
				if failureCount > 0 {
					log.Printf("[RECOVERED] victim-a is back after %d failure(s), resetting counter", failureCount)
					failureCount = 0
				}
				resp.Body.Close()
				log.Printf("[ALIVE] victim-a responded: %s", resp.Status)
			}
		}
	}

	log.Println("Shutting down watchdog...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
	log.Println("Watchdog stopped.")
}
